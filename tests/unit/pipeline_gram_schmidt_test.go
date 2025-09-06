package unit

import (
	"math"
	"testing"

	"cryptorun/internal/application/pipeline"
)

func TestOrthogonalizer_OrthogonalizeFactors(t *testing.T) {
	orthogonalizer := pipeline.NewOrthogonalizer()

	// Create test factor sets with known correlation
	factorSets := []pipeline.FactorSet{
		{
			Symbol:       "TEST1",
			MomentumCore: 10.0,
			Volume:       2.0,
			Social:       5.0,
			Volatility:   15.0,
			Raw:          make(map[string]float64),
		},
		{
			Symbol:       "TEST2",
			MomentumCore: 15.0,
			Volume:       3.0,
			Social:       -2.0,
			Volatility:   20.0,
			Raw:          make(map[string]float64),
		},
		{
			Symbol:       "TEST3",
			MomentumCore: -5.0,
			Volume:       1.0,
			Social:       0.0,
			Volatility:   10.0,
			Raw:          make(map[string]float64),
		},
	}

	result, err := orthogonalizer.OrthogonalizeFactors(factorSets)

	if err != nil {
		t.Fatalf("OrthogonalizeFactors failed: %v", err)
	}

	if len(result) != len(factorSets) {
		t.Errorf("Expected %d factor sets, got %d", len(factorSets), len(result))
	}

	for i, fs := range result {
		// Verify MomentumCore is protected (unchanged)
		originalMomentum := factorSets[i].MomentumCore
		if math.Abs(fs.MomentumCore-originalMomentum) > 0.001 {
			t.Errorf("MomentumCore not protected for %s: original=%.3f, result=%.3f",
				fs.Symbol, originalMomentum, fs.MomentumCore)
		}

		// Verify orthogonal factors are populated
		if len(fs.Orthogonal) != 4 {
			t.Errorf("Expected 4 orthogonal factors for %s, got %d", fs.Symbol, len(fs.Orthogonal))
		}

		requiredFactors := []string{"momentum_core", "volume", "social", "volatility"}
		for _, factor := range requiredFactors {
			if _, exists := fs.Orthogonal[factor]; !exists {
				t.Errorf("Missing orthogonal factor %s for %s", factor, fs.Symbol)
			}
		}

		// Verify meta information
		if len(fs.Meta.ProtectedFactors) == 0 {
			t.Errorf("Protected factors not recorded for %s", fs.Symbol)
		}

		if fs.Meta.OrthogonalityCheck <= 0 {
			t.Errorf("Orthogonality check should be positive for %s, got %.3f",
				fs.Symbol, fs.Meta.OrthogonalityCheck)
		}

		// Verify no NaN values introduced
		factors := []float64{fs.MomentumCore, fs.Volume, fs.Social, fs.Volatility}
		for j, factor := range factors {
			if math.IsNaN(factor) {
				t.Errorf("Factor %d is NaN for %s after orthogonalization", j, fs.Symbol)
			}
		}
	}
}

func TestOrthogonalizer_EmptyInput(t *testing.T) {
	orthogonalizer := pipeline.NewOrthogonalizer()

	result, err := orthogonalizer.OrthogonalizeFactors([]pipeline.FactorSet{})

	if err != nil {
		t.Errorf("OrthogonalizeFactors should handle empty input gracefully: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result for empty input, got %d factors", len(result))
	}
}

func TestOrthogonalizer_SingleFactorSet(t *testing.T) {
	orthogonalizer := pipeline.NewOrthogonalizer()

	factorSet := pipeline.FactorSet{
		Symbol:       "SINGLE",
		MomentumCore: 12.5,
		Volume:       1.8,
		Social:       3.2,
		Volatility:   18.0,
		Raw:          make(map[string]float64),
	}

	result, err := orthogonalizer.OrthogonalizeFactors([]pipeline.FactorSet{factorSet})

	if err != nil {
		t.Fatalf("OrthogonalizeFactors failed for single input: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 factor set, got %d", len(result))
		return
	}

	fs := result[0]

	// MomentumCore should be protected
	if math.Abs(fs.MomentumCore-factorSet.MomentumCore) > 0.001 {
		t.Errorf("MomentumCore not protected: original=%.3f, result=%.3f",
			factorSet.MomentumCore, fs.MomentumCore)
	}

	// Other factors may change but should not be NaN
	if math.IsNaN(fs.Volume) || math.IsNaN(fs.Social) || math.IsNaN(fs.Volatility) {
		t.Errorf("Orthogonalized factors contain NaN values")
	}
}

func TestApplySocialCap(t *testing.T) {
	orthogonalizer := pipeline.NewOrthogonalizer()

	// Create factor sets with various social values
	factorSets := []pipeline.FactorSet{
		{
			Symbol: "NORMAL",
			Social: 5.0, // Within cap
		},
		{
			Symbol: "OVER_CAP",
			Social: 15.0, // Over +10 cap
		},
		{
			Symbol: "UNDER_CAP",
			Social: -5.0, // Within cap
		},
		{
			Symbol: "WAY_OVER",
			Social: 25.0, // Way over cap
		},
	}

	// Initialize orthogonal maps
	for i := range factorSets {
		factorSets[i].Orthogonal = make(map[string]float64)
		factorSets[i].Raw = make(map[string]float64)
		factorSets[i].Orthogonal["social"] = factorSets[i].Social
	}

	result := orthogonalizer.ApplySocialCap(factorSets)

	// Check capping behavior
	testCases := []struct {
		symbol   string
		original float64
		expected float64
	}{
		{"NORMAL", 5.0, 5.0},      // Should remain unchanged
		{"OVER_CAP", 15.0, 10.0},  // Should be capped at 10
		{"UNDER_CAP", -5.0, -5.0}, // Negative values not capped upward
		{"WAY_OVER", 25.0, 10.0},  // Should be capped at 10
	}

	for _, tc := range testCases {
		var found *pipeline.FactorSet
		for i := range result {
			if result[i].Symbol == tc.symbol {
				found = &result[i]
				break
			}
		}

		if found == nil {
			t.Errorf("Symbol %s not found in result", tc.symbol)
			continue
		}

		if math.Abs(found.Social-tc.expected) > 0.001 {
			t.Errorf("Social cap for %s: expected %.1f, got %.1f",
				tc.symbol, tc.expected, found.Social)
		}

		// Verify orthogonal map is also updated
		if orthSocial, exists := found.Orthogonal["social"]; exists {
			if math.Abs(orthSocial-tc.expected) > 0.001 {
				t.Errorf("Orthogonal social for %s: expected %.1f, got %.1f",
					tc.symbol, tc.expected, orthSocial)
			}
		}

		// Verify original value is preserved in Raw
		if originalSocial, exists := found.Raw["social_before_cap"]; exists {
			if math.Abs(originalSocial-tc.original) > 0.001 {
				t.Errorf("Original social not preserved for %s: expected %.1f, got %.1f",
					tc.symbol, tc.original, originalSocial)
			}
		}
	}
}

func TestValidateFactorSet(t *testing.T) {
	testCases := []struct {
		name     string
		fs       pipeline.FactorSet
		expected bool
	}{
		{
			name: "Valid factor set",
			fs: pipeline.FactorSet{
				MomentumCore: 10.0,
				Volume:       1.5,
				Social:       3.0,
				Volatility:   15.0,
			},
			expected: true,
		},
		{
			name: "NaN momentum core",
			fs: pipeline.FactorSet{
				MomentumCore: math.NaN(),
				Volume:       1.5,
				Social:       3.0,
				Volatility:   15.0,
			},
			expected: false, // MomentumCore is required
		},
		{
			name: "Some NaN factors but momentum valid",
			fs: pipeline.FactorSet{
				MomentumCore: 10.0,
				Volume:       math.NaN(),
				Social:       3.0,
				Volatility:   15.0,
			},
			expected: true, // Need at least 2 valid including momentum
		},
		{
			name: "Only momentum valid",
			fs: pipeline.FactorSet{
				MomentumCore: 10.0,
				Volume:       math.NaN(),
				Social:       math.NaN(),
				Volatility:   math.NaN(),
			},
			expected: false, // Need at least momentum + 1 other
		},
		{
			name: "Inf values",
			fs: pipeline.FactorSet{
				MomentumCore: 10.0,
				Volume:       math.Inf(1),
				Social:       3.0,
				Volatility:   15.0,
			},
			expected: false, // Inf values not allowed
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := pipeline.ValidateFactorSet(tc.fs)
			if result != tc.expected {
				t.Errorf("ValidateFactorSet: expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestBuildFactorSet(t *testing.T) {
	// Create mock momentum factors
	momentum := &pipeline.MomentumFactors{
		Symbol:      "BUILDTEST",
		Momentum1h:  5.0,
		Momentum4h:  10.0,
		Momentum12h: 15.0,
		Momentum24h: 8.0,
		Momentum7d:  3.0,
		Volume1h:    100000,
		Volume4h:    200000,
		Volume24h:   500000,
		RSI4h:       65.0,
		ATR1h:       2.5,
		Raw:         make(map[pipeline.TimeFrame]float64),
	}

	// Set some raw values for regime weighting
	momentum.Raw[pipeline.TF1h] = momentum.Momentum1h
	momentum.Raw[pipeline.TF4h] = momentum.Momentum4h
	momentum.Raw[pipeline.TF12h] = momentum.Momentum12h
	momentum.Raw[pipeline.TF24h] = momentum.Momentum24h
	momentum.Raw[pipeline.TF7d] = momentum.Momentum7d

	volumeFactor := 1.8
	socialFactor := 4.2
	volatilityFactor := 18.5

	factorSet := pipeline.BuildFactorSet("BUILDTEST", momentum, volumeFactor, socialFactor, volatilityFactor)

	// Verify symbol
	if factorSet.Symbol != "BUILDTEST" {
		t.Errorf("Symbol mismatch: expected BUILDTEST, got %s", factorSet.Symbol)
	}

	// Verify factors are set
	if factorSet.Volume != volumeFactor {
		t.Errorf("Volume factor mismatch: expected %.1f, got %.1f", volumeFactor, factorSet.Volume)
	}

	if factorSet.Social != socialFactor {
		t.Errorf("Social factor mismatch: expected %.1f, got %.1f", socialFactor, factorSet.Social)
	}

	if factorSet.Volatility != volatilityFactor {
		t.Errorf("Volatility factor mismatch: expected %.1f, got %.1f", volatilityFactor, factorSet.Volatility)
	}

	// Verify MomentumCore is calculated (should not be NaN)
	if math.IsNaN(factorSet.MomentumCore) {
		t.Errorf("MomentumCore should be calculated, got NaN")
	}

	// Verify Raw map contains expected values
	expectedRawKeys := []string{
		"momentum_core", "volume", "social", "volatility",
		"momentum_1h", "momentum_4h", "momentum_12h", "momentum_24h", "momentum_7d",
		"volume_1h", "volume_4h", "volume_24h", "rsi_4h", "atr_1h",
	}

	for _, key := range expectedRawKeys {
		if _, exists := factorSet.Raw[key]; !exists {
			t.Errorf("Missing raw factor: %s", key)
		}
	}

	// Verify meta information
	if len(factorSet.Meta.ProtectedFactors) == 0 {
		t.Errorf("Protected factors should be set")
	}

	if factorSet.Meta.ProtectedFactors[0] != "momentum_core" {
		t.Errorf("Expected momentum_core to be protected factor")
	}
}
