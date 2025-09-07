package unit

import (
	"math"
	"testing"

	"cryptorun/internal/domain/regime"
)

func TestFactorWeights_Validation(t *testing.T) {
	validCases := []struct {
		name    string
		weights regime.FactorWeights
	}{
		{
			name: "default trending bull weights",
			weights: regime.FactorWeights{
				Momentum:  50.0,
				Technical: 20.0,
				Volume:    15.0,
				Quality:   10.0,
				Catalyst:  5.0,
			},
		},
		{
			name: "minimum momentum allocation",
			weights: regime.FactorWeights{
				Momentum:  25.0, // Minimum for MomentumCore protection
				Technical: 30.0,
				Volume:    25.0,
				Quality:   15.0,
				Catalyst:  5.0,
			},
		},
	}

	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			err := regime.ValidateFactorWeights(tc.weights)
			if err != nil {
				t.Errorf("valid weights should not produce error: %v", err)
			}
		})
	}

	invalidCases := []struct {
		name    string
		weights regime.FactorWeights
		errMsg  string
	}{
		{
			name: "negative weight",
			weights: regime.FactorWeights{
				Momentum:  -5.0, // Negative weight
				Technical: 35.0,
				Volume:    35.0,
				Quality:   25.0,
				Catalyst:  10.0,
			},
			errMsg: "out of bounds",
		},
		{
			name: "weights don't sum to 100",
			weights: regime.FactorWeights{
				Momentum:  40.0,
				Technical: 20.0,
				Volume:    15.0,
				Quality:   10.0,
				Catalyst:  5.0, // Sum = 90, not 100
			},
			errMsg: "do not sum to 100",
		},
		{
			name: "momentum below minimum",
			weights: regime.FactorWeights{
				Momentum:  20.0, // Below 25% minimum
				Technical: 30.0,
				Volume:    25.0,
				Quality:   20.0,
				Catalyst:  5.0,
			},
			errMsg: "too low",
		},
		{
			name: "excessive single weight",
			weights: regime.FactorWeights{
				Momentum:  110.0, // Over 100%
				Technical: 0.0,
				Volume:    0.0,
				Quality:   0.0,
				Catalyst:  -10.0, // Negative to make sum work
			},
			errMsg: "out of bounds",
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := regime.ValidateFactorWeights(tc.weights)
			if err == nil {
				t.Errorf("invalid weights should produce error")
			}
			if err != nil && tc.errMsg != "" {
				if len(err.Error()) == 0 || err.Error()[:min(len(err.Error()), len(tc.errMsg))] != tc.errMsg[:min(len(err.Error()), len(tc.errMsg))] {
					t.Errorf("expected error containing '%s', got '%s'", tc.errMsg, err.Error())
				}
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestRegimeWeightMap_DefaultValues(t *testing.T) {
	weightMap := regime.GetDefaultWeightMap()

	// Test all regimes are present and valid
	regimes := []struct {
		name    string
		weights regime.FactorWeights
	}{
		{"trending_bull", weightMap.TrendingBull},
		{"choppy", weightMap.Choppy},
		{"high_vol", weightMap.HighVol},
	}

	for _, regime := range regimes {
		t.Run(regime.name, func(t *testing.T) {
			// Validate weights
			err := regime.ValidateFactorWeights(regime.weights)
			if err != nil {
				t.Errorf("default %s weights invalid: %v", regime.name, err)
			}

			// Check sum equals 100
			sum := regime.weights.Momentum + regime.weights.Technical +
				regime.weights.Volume + regime.weights.Quality + regime.weights.Catalyst
			if math.Abs(sum-100.0) > 0.1 {
				t.Errorf("%s weights sum to %f, expected 100", regime.name, sum)
			}

			// Check momentum protection
			if regime.weights.Momentum < 25.0 {
				t.Errorf("%s momentum weight %f below minimum 25%%", regime.name, regime.weights.Momentum)
			}
		})
	}

	// Verify weight differences between regimes make sense
	if weightMap.TrendingBull.Momentum <= weightMap.Choppy.Momentum {
		t.Errorf("trending bull should have higher momentum weight than choppy")
	}

	if weightMap.HighVol.Quality <= weightMap.Choppy.Quality {
		t.Errorf("high vol regime should emphasize quality more than choppy")
	}
}

func TestWeightResolver_RegimeWeights(t *testing.T) {
	// Create mock detector with known regime
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	weightMap := regime.GetDefaultWeightMap()
	resolver := regime.NewWeightResolver(weightMap, detector)

	// Test getting weights for each regime
	regimeTypes := []regime.RegimeType{
		regime.TrendingBull,
		regime.Choppy,
		regime.HighVol,
	}

	for _, regimeType := range regimeTypes {
		weights := resolver.GetWeightsForRegime(regimeType)

		// Validate the weights
		err := regime.ValidateFactorWeights(weights)
		if err != nil {
			t.Errorf("weights for regime %s are invalid: %v", regimeType, err)
		}

		// Verify weights match expected regime characteristics
		switch regimeType {
		case regime.TrendingBull:
			if weights.Momentum < 45.0 {
				t.Errorf("trending bull should have high momentum weight, got %f", weights.Momentum)
			}
		case regime.Choppy:
			if weights.Technical < weights.Momentum {
				t.Errorf("choppy regime should have relatively high technical weight")
			}
		case regime.HighVol:
			if weights.Quality < 15.0 {
				t.Errorf("high vol regime should emphasize quality, got %f", weights.Quality)
			}
		}
	}
}

func TestNormalizeWeights_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		input    regime.FactorWeights
		expected float64 // Expected sum after normalization
	}{
		{
			name: "already normalized",
			input: regime.FactorWeights{
				Momentum:  50.0,
				Technical: 20.0,
				Volume:    15.0,
				Quality:   10.0,
				Catalyst:  5.0,
			},
			expected: 100.0,
		},
		{
			name: "needs scaling up",
			input: regime.FactorWeights{
				Momentum:  25.0,
				Technical: 10.0,
				Volume:    7.5,
				Quality:   5.0,
				Catalyst:  2.5, // Sum = 50, should be scaled to 100
			},
			expected: 100.0,
		},
		{
			name: "needs scaling down",
			input: regime.FactorWeights{
				Momentum:  100.0,
				Technical: 40.0,
				Volume:    30.0,
				Quality:   20.0,
				Catalyst:  10.0, // Sum = 200, should be scaled to 100
			},
			expected: 100.0,
		},
		{
			name: "zero weights (fallback to default)",
			input: regime.FactorWeights{
				Momentum:  0.0,
				Technical: 0.0,
				Volume:    0.0,
				Quality:   0.0,
				Catalyst:  0.0,
			},
			expected: 100.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			normalized := regime.NormalizeWeights(tc.input)

			sum := normalized.Momentum + normalized.Technical +
				normalized.Volume + normalized.Quality + normalized.Catalyst

			if math.Abs(sum-tc.expected) > 0.001 {
				t.Errorf("normalized sum %f, expected %f", sum, tc.expected)
			}

			// Verify all weights are non-negative
			if normalized.Momentum < 0 || normalized.Technical < 0 ||
				normalized.Volume < 0 || normalized.Quality < 0 || normalized.Catalyst < 0 {
				t.Errorf("normalized weights should be non-negative: %+v", normalized)
			}
		})
	}
}

func TestApplySocialCap(t *testing.T) {
	testCases := []struct {
		name         string
		baseScore    float64
		socialSignal float64
		expected     float64
	}{
		{
			name:         "positive social within cap",
			baseScore:    75.0,
			socialSignal: 5.0,
			expected:     80.0,
		},
		{
			name:         "positive social exceeds cap",
			baseScore:    80.0,
			socialSignal: 15.0, // Should be capped at +10
			expected:     90.0,
		},
		{
			name:         "negative social within cap",
			baseScore:    85.0,
			socialSignal: -7.0,
			expected:     78.0,
		},
		{
			name:         "negative social exceeds cap",
			baseScore:    90.0,
			socialSignal: -15.0, // Should be capped at -10
			expected:     80.0,
		},
		{
			name:         "zero social signal",
			baseScore:    70.0,
			socialSignal: 0.0,
			expected:     70.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := regime.ApplySocialCap(tc.baseScore, tc.socialSignal)
			if math.Abs(result-tc.expected) > 0.001 {
				t.Errorf("expected %f, got %f", tc.expected, result)
			}
		})
	}
}

func TestMomentumProtectionStatus(t *testing.T) {
	testCases := []struct {
		name            string
		weights         regime.FactorWeights
		expectProtected bool
	}{
		{
			name: "well protected momentum",
			weights: regime.FactorWeights{
				Momentum:  40.0,
				Technical: 25.0,
				Volume:    20.0,
				Quality:   10.0,
				Catalyst:  5.0,
			},
			expectProtected: true,
		},
		{
			name: "minimum momentum protection",
			weights: regime.FactorWeights{
				Momentum:  25.0, // Exactly at minimum
				Technical: 30.0,
				Volume:    25.0,
				Quality:   15.0,
				Catalyst:  5.0,
			},
			expectProtected: true,
		},
		{
			name: "insufficient momentum protection",
			weights: regime.FactorWeights{
				Momentum:  20.0, // Below minimum
				Technical: 35.0,
				Volume:    25.0,
				Quality:   15.0,
				Catalyst:  5.0,
			},
			expectProtected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status := regime.GetMomentumProtectionStatus(tc.weights)

			isProtected, ok := status["is_protected"].(bool)
			if !ok {
				t.Errorf("protection status should include is_protected boolean")
				return
			}

			if isProtected != tc.expectProtected {
				t.Errorf("expected protection status %v, got %v", tc.expectProtected, isProtected)
			}

			// Verify other status fields are present
			expectedFields := []string{"momentum_weight", "min_required", "protection_margin", "total_allocation", "social_handled_separately"}
			for _, field := range expectedFields {
				if _, exists := status[field]; !exists {
					t.Errorf("protection status missing field: %s", field)
				}
			}
		})
	}
}

func TestWeightAllocationSummary(t *testing.T) {
	weights := regime.FactorWeights{
		Momentum:  45.0,
		Technical: 25.0,
		Volume:    15.0,
		Quality:   10.0,
		Catalyst:  5.0,
	}

	summary := regime.GetWeightAllocationSummary(regime.TrendingBull, weights)

	// Verify summary structure
	regime, ok := summary["regime"].(string)
	if !ok || regime != "TRENDING_BULL" {
		t.Errorf("expected regime TRENDING_BULL, got %v", summary["regime"])
	}

	allocations, ok := summary["allocations"].(map[string]float64)
	if !ok {
		t.Errorf("allocations should be map[string]float64")
		return
	}

	// Verify all allocations are present
	expectedAllocations := []string{"momentum", "technical", "volume", "quality", "catalyst"}
	for _, alloc := range expectedAllocations {
		if _, exists := allocations[alloc]; !exists {
			t.Errorf("allocation summary missing: %s", alloc)
		}
	}

	// Verify total calculation
	total, ok := summary["total"].(float64)
	if !ok {
		t.Errorf("total should be float64")
		return
	}

	expectedTotal := weights.Momentum + weights.Technical + weights.Volume + weights.Quality + weights.Catalyst
	if math.Abs(total-expectedTotal) > 0.001 {
		t.Errorf("expected total %f, got %f", expectedTotal, total)
	}

	// Verify validation flags
	isValid, ok := summary["is_valid"].(bool)
	if !ok || !isValid {
		t.Errorf("summary should indicate valid weights")
	}

	momentumProtected, ok := summary["momentum_protected"].(bool)
	if !ok || !momentumProtected {
		t.Errorf("summary should indicate momentum is protected")
	}
}
