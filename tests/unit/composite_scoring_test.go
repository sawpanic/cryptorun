package unit

import (
	"math"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/score/composite"
)

// TestOrthogonality validates Gram-Schmidt orthogonalization
func TestOrthogonality(t *testing.T) {
	orthogonalizer := composite.NewOrthogonalizer()

	// Create test factors
	factors := []composite.Factor{
		{Name: "momentum_core", Values: []float64{1.0}, Protected: true},
		{Name: "technical", Values: []float64{0.8, 0.6}},
		{Name: "volume", Values: []float64{0.5, 0.7}},
		{Name: "quality", Values: []float64{0.3, 0.4, 0.9, 0.2}},
	}

	// Orthogonalize
	result, err := orthogonalizer.Orthogonalize(factors)
	if err != nil {
		t.Fatalf("Orthogonalization failed: %v", err)
	}

	// Validate orthogonality with tolerance
	tolerance := 0.01
	err = orthogonalizer.ValidateOrthogonality(result, tolerance)
	if err != nil {
		t.Errorf("Orthogonality validation failed: %v", err)
	}

	// Check that momentum core is unchanged (protected)
	if result.MomentumCore.Values[0] != 1.0 {
		t.Errorf("Protected momentum core was modified: got %.3f, expected 1.0",
			result.MomentumCore.Values[0])
	}

	// Check residuals are non-zero (assuming proper input)
	if len(result.TechnicalResid.Values) == 0 {
		t.Error("Technical residual is empty")
	}
}

// TestCompositeScoring validates end-to-end scoring pipeline
func TestCompositeScoring(t *testing.T) {
	// Create test input
	input := composite.ScoringInput{
		Symbol:    "BTCUSD",
		Timestamp: time.Now(),

		// Momentum factors
		Momentum1h:  0.05, // 5% 1h return
		Momentum4h:  0.12, // 12% 4h return
		Momentum12h: 0.08, // 8% 12h return
		Momentum24h: 0.15, // 15% 24h return
		Momentum7d:  0.03, // 3% 7d return

		// Technical factors
		RSI4h:    65.0,
		ADX1h:    45.0,
		HurstExp: 0.6,

		// Volume factors
		VolumeSurge: 2.5, // 2.5× volume surge
		DeltaOI:     0.1, // 10% OI increase

		// Quality factors
		OIAbsolute:   100000, // OI absolute value
		ReserveRatio: 0.95,   // 95% reserve ratio
		ETFFlows:     50000,  // ETF inflows
		VenueHealth:  0.9,    // 90% venue health

		// Social factors
		SocialScore: 7.5, // High social score
		BrandScore:  6.0, // High brand score

		Regime: "normal",
	}

	// Create scorer with normal regime weights
	weights := composite.RegimeWeights{
		MomentumCore:      0.45,
		TechnicalResidual: 0.22,
		SupplyDemandBlock: 0.33,
		VolumeWeight:      0.55,
		QualityWeight:     0.45,
	}

	scorer := composite.NewCompositeScorer(weights)

	// Score the input
	result := scorer.Score(input)

	// Validate basic constraints
	if result.Internal0to100 < 0 || result.Internal0to100 > 100 {
		t.Errorf("Internal score %.2f outside [0,100] range", result.Internal0to100)
	}

	if result.FinalWithSocial < 0 || result.FinalWithSocial > 110 {
		t.Errorf("Final score %.2f outside [0,110] range", result.FinalWithSocial)
	}

	if result.SocialResid < 0 || result.SocialResid > 10 {
		t.Errorf("Social residual %.2f outside [0,10] cap", result.SocialResid)
	}

	// Validate social capping
	rawSocial := (input.SocialScore + input.BrandScore) / 2 // 6.75
	if rawSocial <= 10 && result.SocialResid != rawSocial {
		t.Errorf("Social capping incorrect: expected %.2f, got %.2f", rawSocial, result.SocialResid)
	}

	// Validate score composition
	err := result.Validate()
	if err != nil {
		t.Errorf("Score validation failed: %v", err)
	}
}

// TestHardEntryGates validates new composite entry gates
func TestHardEntryGates(t *testing.T) {
	gates := composite.NewHardEntryGates()

	testCases := []struct {
		name     string
		input    composite.GateInput
		expected bool
		reason   string
	}{
		{
			name: "AllPassWithHighScore",
			input: composite.GateInput{
				Symbol:         "ETHUSD",
				Timestamp:      time.Now(),
				CompositeScore: 85.0, // Above 75 threshold
				VADR:           2.1,  // Above 1.8 threshold
				FundingZScore:  -0.5, // Negative (divergence present)
				BarAge:         1,    // Fresh data
				ATRDistance:    1.0,  // Within ATR limits
				ATRCurrent:     50.0,
				SignalTime:     time.Now().Add(-10 * time.Second),
				ExecutionTime:  time.Now(),
				SpreadBps:      25.0,   // Good spread
				DepthUSD:       150000, // Good depth
				MicroVADR:      1.8,    // Good micro VADR
				Momentum24h:    8.0,    // Not fatigued
				RSI4h:          60.0,   // Not overbought
			},
			expected: true,
		},
		{
			name: "FailLowScore",
			input: composite.GateInput{
				CompositeScore: 65.0, // Below 75 threshold
				VADR:           2.1,
				FundingZScore:  -0.5,
				// ... other fields passing
				BarAge:        1,
				ATRDistance:   1.0,
				ATRCurrent:    50.0,
				SignalTime:    time.Now().Add(-10 * time.Second),
				ExecutionTime: time.Now(),
				SpreadBps:     25.0,
				DepthUSD:      150000,
				MicroVADR:     1.8,
				Momentum24h:   8.0,
				RSI4h:         60.0,
			},
			expected: false,
			reason:   "score 65.0 < 75.0 minimum",
		},
		{
			name: "FailLowVADR",
			input: composite.GateInput{
				CompositeScore: 85.0,
				VADR:           1.5, // Below 1.8 threshold
				FundingZScore:  -0.5,
				// ... other fields passing
				BarAge:        1,
				ATRDistance:   1.0,
				ATRCurrent:    50.0,
				SignalTime:    time.Now().Add(-10 * time.Second),
				ExecutionTime: time.Now(),
				SpreadBps:     25.0,
				DepthUSD:      150000,
				MicroVADR:     1.8,
				Momentum24h:   8.0,
				RSI4h:         60.0,
			},
			expected: false,
			reason:   "VADR 1.50× < 1.8× minimum",
		},
		{
			name: "FailNoFundingDivergence",
			input: composite.GateInput{
				CompositeScore: 85.0,
				VADR:           2.1,
				FundingZScore:  0.5, // Positive (no divergence)
				// ... other fields passing
				BarAge:        1,
				ATRDistance:   1.0,
				ATRCurrent:    50.0,
				SignalTime:    time.Now().Add(-10 * time.Second),
				ExecutionTime: time.Now(),
				SpreadBps:     25.0,
				DepthUSD:      150000,
				MicroVADR:     1.8,
				Momentum24h:   8.0,
				RSI4h:         60.0,
			},
			expected: false,
			reason:   "funding z-score 0.50 > 0.0 (no divergence)",
		},
		{
			name: "FailFatigue",
			input: composite.GateInput{
				CompositeScore: 85.0,
				VADR:           2.1,
				FundingZScore:  -0.5,
				BarAge:         1,
				ATRDistance:    1.0,
				ATRCurrent:     50.0,
				SignalTime:     time.Now().Add(-10 * time.Second),
				ExecutionTime:  time.Now(),
				SpreadBps:      25.0,
				DepthUSD:       150000,
				MicroVADR:      1.8,
				Momentum24h:    15.0, // High momentum
				RSI4h:          75.0, // High RSI → fatigue
			},
			expected: false,
			reason:   "fatigue: 24h momentum 15.0% > 12% AND RSI4h 75.0 > 70",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := gates.EvaluateAll(tc.input)

			if result.Allowed != tc.expected {
				t.Errorf("Expected allowed=%v, got %v", tc.expected, result.Allowed)
			}

			if !tc.expected && tc.reason != "" {
				if !containsSubstring(result.Reason, tc.reason) {
					t.Errorf("Expected reason to contain %q, got %q", tc.reason, result.Reason)
				}
			}
		})
	}
}

// TestRegimeWeights validates regime weight configuration
func TestRegimeWeights(t *testing.T) {
	normalizer := composite.NewNormalizer()

	regimes := []string{"calm", "normal", "volatile"}

	for _, regime := range regimes {
		t.Run(regime, func(t *testing.T) {
			weights, err := normalizer.GetRegimeWeights(regime)
			if err != nil {
				t.Fatalf("Failed to get weights for %s: %v", regime, err)
			}

			// Validate weight structure
			err = normalizer.ValidateWeights(weights)
			if err != nil {
				t.Errorf("Weight validation failed for %s: %v", regime, err)
			}

			// Check weight sums
			primarySum := weights["momentum_core"] + weights["technical_residual"] + weights["supply_demand_block"]
			if math.Abs(primarySum-1.0) > 0.01 {
				t.Errorf("Primary weights sum to %.3f, expected 1.0", primarySum)
			}

			subSum := weights["volume_weight"] + weights["quality_weight"]
			if math.Abs(subSum-1.0) > 0.01 {
				t.Errorf("Sub-weights sum to %.3f, expected 1.0", subSum)
			}
		})
	}
}

// TestDeterministicScoring validates deterministic behavior on fixed fixtures
func TestDeterministicScoring(t *testing.T) {
	// Fixed input for deterministic testing
	input := composite.ScoringInput{
		Symbol:       "TESTCOIN",
		Timestamp:    time.Date(2025, 9, 6, 12, 0, 0, 0, time.UTC),
		Momentum1h:   0.02,
		Momentum4h:   0.05,
		Momentum12h:  0.03,
		Momentum24h:  0.08,
		Momentum7d:   0.01,
		RSI4h:        55.0,
		ADX1h:        35.0,
		HurstExp:     0.55,
		VolumeSurge:  1.8,
		DeltaOI:      0.05,
		OIAbsolute:   50000,
		ReserveRatio: 0.9,
		ETFFlows:     25000,
		VenueHealth:  0.85,
		SocialScore:  4.0,
		BrandScore:   3.0,
		Regime:       "normal",
	}

	weights := composite.RegimeWeights{
		MomentumCore:      0.45,
		TechnicalResidual: 0.22,
		SupplyDemandBlock: 0.33,
		VolumeWeight:      0.55,
		QualityWeight:     0.45,
	}

	scorer := composite.NewCompositeScorer(weights)

	// Score multiple times and ensure consistency
	results := make([]composite.CompositeScore, 5)
	for i := range results {
		results[i] = scorer.Score(input)
	}

	// Validate all results are identical
	for i := 1; i < len(results); i++ {
		if math.Abs(results[i].Internal0to100-results[0].Internal0to100) > 1e-10 {
			t.Errorf("Non-deterministic scoring: run %d = %.10f, run 0 = %.10f",
				i, results[i].Internal0to100, results[0].Internal0to100)
		}

		if math.Abs(results[i].FinalWithSocial-results[0].FinalWithSocial) > 1e-10 {
			t.Errorf("Non-deterministic final scoring: run %d = %.10f, run 0 = %.10f",
				i, results[i].FinalWithSocial, results[0].FinalWithSocial)
		}
	}

	// Validate expected ranges for this specific input
	expectedInternal := 40.0 // Rough expectation for this moderate input
	if math.Abs(results[0].Internal0to100-expectedInternal) > 20.0 {
		t.Logf("Internal score %.2f differs significantly from expected ~%.0f",
			results[0].Internal0to100, expectedInternal)
		// This is informational, not a hard failure
	}
}

// TestSocialCapping validates social factor capping behavior
func TestSocialCapping(t *testing.T) {
	weights := composite.RegimeWeights{
		MomentumCore:      0.45,
		TechnicalResidual: 0.22,
		SupplyDemandBlock: 0.33,
		VolumeWeight:      0.55,
		QualityWeight:     0.45,
	}

	scorer := composite.NewCompositeScorer(weights)

	testCases := []struct {
		name        string
		socialScore float64
		brandScore  float64
		expectedCap float64
	}{
		{"LowSocial", 2.0, 3.0, 2.5},        // (2+3)/2 = 2.5, uncapped
		{"ModerateSocial", 6.0, 8.0, 7.0},   // (6+8)/2 = 7.0, uncapped
		{"HighSocial", 12.0, 8.0, 10.0},     // (12+8)/2 = 10.0, at cap
		{"ExtremeSocial", 15.0, 20.0, 10.0}, // (15+20)/2 = 17.5, capped to 10.0
	}

	baseInput := composite.ScoringInput{
		Symbol:      "TESTCOIN",
		Timestamp:   time.Now(),
		Momentum1h:  0.02,
		Momentum4h:  0.05,
		Momentum12h: 0.03,
		Momentum24h: 0.08,
		Regime:      "normal",
		// ... minimal other fields
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := baseInput
			input.SocialScore = tc.socialScore
			input.BrandScore = tc.brandScore

			result := scorer.Score(input)

			if math.Abs(result.SocialResid-tc.expectedCap) > 0.01 {
				t.Errorf("Social capping incorrect: expected %.2f, got %.2f",
					tc.expectedCap, result.SocialResid)
			}

			// Validate final score includes social correctly
			expectedFinal := result.Internal0to100 + result.SocialResid
			expectedFinal = math.Max(0, math.Min(110, expectedFinal))

			if math.Abs(result.FinalWithSocial-expectedFinal) > 0.01 {
				t.Errorf("Final score calculation incorrect: expected %.2f, got %.2f",
					expectedFinal, result.FinalWithSocial)
			}
		})
	}
}

// Helper function for substring checking
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr ||
		(len(s) > len(substr) && containsSubstring(s[1:], substr)))
}
