package unit

import (
	"encoding/json"
	"fmt"
	"testing"

	"cryptorun/internal/regime"
)

func TestRegimeWeightsSnapshot(t *testing.T) {
	// This test verifies the exact weight values for each regime
	// and serves as documentation of the current weight configuration

	weightManager := regime.NewWeightManager()

	// Test each regime's weight configuration
	regimes := []regime.Regime{
		regime.TrendingBull,
		regime.Choppy,
		regime.HighVol,
	}

	expectedWeights := map[regime.Regime]map[string]float64{
		regime.TrendingBull: {
			"momentum_1h":      0.25,
			"momentum_4h":      0.20,
			"momentum_12h":     0.15,
			"momentum_24h":     0.10,
			"weekly_7d_carry":  0.10, // Trending-only factor
			"volume_surge":     0.08,
			"volatility_score": 0.05, // Reduced in trending
			"quality_score":    0.04,
			"social_sentiment": 0.03,
		},
		regime.Choppy: {
			"momentum_1h":      0.20, // Reduced short-term
			"momentum_4h":      0.18,
			"momentum_12h":     0.15,
			"momentum_24h":     0.12,
			"weekly_7d_carry":  0.00, // No weekly carry in chop
			"volume_surge":     0.12, // Higher volume emphasis
			"volatility_score": 0.10, // Volatility important
			"quality_score":    0.08, // Quality matters more
			"social_sentiment": 0.05,
		},
		regime.HighVol: {
			"momentum_1h":      0.15, // Reduced short-term (noisy)
			"momentum_4h":      0.15,
			"momentum_12h":     0.18, // Favor longer timeframes
			"momentum_24h":     0.15,
			"weekly_7d_carry":  0.00, // No weekly carry in volatility
			"volume_surge":     0.08, // Lower volume weight
			"volatility_score": 0.15, // High volatility awareness
			"quality_score":    0.12, // Quality crucial
			"social_sentiment": 0.02, // Minimal social (noise)
		},
	}

	expectedMovementGates := map[regime.Regime]regime.MovementGateConfig{
		regime.TrendingBull: {
			MinMovementPercent:  3.5, // Lower threshold in trends
			TimeWindowHours:     48,
			VolumeSurgeRequired: false, // Not required in strong trends
			TightenedThresholds: false,
		},
		regime.Choppy: {
			MinMovementPercent:  5.0, // Standard threshold
			TimeWindowHours:     48,
			VolumeSurgeRequired: true, // Require volume in chop
			TightenedThresholds: false,
		},
		regime.HighVol: {
			MinMovementPercent:  7.0,  // Tightened threshold
			TimeWindowHours:     36,   // Shorter window
			VolumeSurgeRequired: true, // Volume required
			TightenedThresholds: true, // Higher bars for entry
		},
	}

	for _, regimeType := range regimes {
		t.Run(regimeType.String(), func(t *testing.T) {
			preset, err := weightManager.GetWeightsForRegime(regimeType)
			if err != nil {
				t.Fatalf("Failed to get weights for regime %s: %v", regimeType.String(), err)
			}

			// Verify exact weights match expected values
			expectedWeightMap := expectedWeights[regimeType]
			if len(preset.Weights) != len(expectedWeightMap) {
				t.Errorf("Weight count mismatch for %s: expected %d, got %d",
					regimeType.String(), len(expectedWeightMap), len(preset.Weights))
			}

			for factor, expectedWeight := range expectedWeightMap {
				actualWeight, exists := preset.Weights[factor]
				if !exists {
					t.Errorf("Missing factor %s in %s regime", factor, regimeType.String())
					continue
				}
				if actualWeight != expectedWeight {
					t.Errorf("Weight mismatch for %s.%s: expected %.3f, got %.3f",
						regimeType.String(), factor, expectedWeight, actualWeight)
				}
			}

			// Verify movement gate configuration
			expectedGate := expectedMovementGates[regimeType]
			actualGate := preset.MovementGate

			if actualGate.MinMovementPercent != expectedGate.MinMovementPercent {
				t.Errorf("Movement threshold mismatch for %s: expected %.1f%%, got %.1f%%",
					regimeType.String(), expectedGate.MinMovementPercent, actualGate.MinMovementPercent)
			}

			if actualGate.TimeWindowHours != expectedGate.TimeWindowHours {
				t.Errorf("Time window mismatch for %s: expected %d hours, got %d hours",
					regimeType.String(), expectedGate.TimeWindowHours, actualGate.TimeWindowHours)
			}

			if actualGate.VolumeSurgeRequired != expectedGate.VolumeSurgeRequired {
				t.Errorf("Volume surge requirement mismatch for %s: expected %v, got %v",
					regimeType.String(), expectedGate.VolumeSurgeRequired, actualGate.VolumeSurgeRequired)
			}

			if actualGate.TightenedThresholds != expectedGate.TightenedThresholds {
				t.Errorf("Tightened thresholds mismatch for %s: expected %v, got %v",
					regimeType.String(), expectedGate.TightenedThresholds, actualGate.TightenedThresholds)
			}

			// Validate weight sum
			err = weightManager.ValidateWeights(regimeType)
			if err != nil {
				t.Errorf("Weight validation failed for %s: %v", regimeType.String(), err)
			}

			// Log full configuration for documentation
			configJSON, _ := json.MarshalIndent(map[string]interface{}{
				"regime":        preset.Name,
				"description":   preset.Description,
				"weights":       preset.Weights,
				"movement_gate": preset.MovementGate,
				"metadata":      preset.Metadata,
			}, "", "  ")

			t.Logf("Full configuration for %s:\n%s", regimeType.String(), string(configJSON))
		})
	}
}

func TestRegimeWeightDifferences(t *testing.T) {
	// Test weight transitions between regimes
	weightManager := regime.NewWeightManager()

	transitions := []struct {
		from, to    regime.Regime
		description string
	}{
		{
			from:        regime.Choppy,
			to:          regime.TrendingBull,
			description: "choppy to trending should gain weekly carry and momentum emphasis",
		},
		{
			from:        regime.Choppy,
			to:          regime.HighVol,
			description: "choppy to high vol should emphasize quality and longer timeframes",
		},
		{
			from:        regime.TrendingBull,
			to:          regime.HighVol,
			description: "trending to high vol should lose carry and emphasize quality",
		},
	}

	for _, transition := range transitions {
		t.Run(fmt.Sprintf("%s_to_%s", transition.from.String(), transition.to.String()), func(t *testing.T) {
			differences, err := weightManager.GetWeightDifferences(transition.from, transition.to)
			if err != nil {
				t.Fatalf("Failed to get weight differences: %v", err)
			}

			// Verify we get some differences
			hasChanges := false
			for _, diff := range differences {
				if diff != 0 {
					hasChanges = true
					break
				}
			}

			if !hasChanges {
				t.Error("Expected some weight differences between regimes")
			}

			// Log all differences for documentation
			t.Logf("Weight differences %s → %s: %s", transition.from.String(), transition.to.String(), transition.description)
			for factor, diff := range differences {
				if diff != 0 {
					sign := "+"
					if diff < 0 {
						sign = ""
					}
					t.Logf("  %s: %s%.3f", factor, sign, diff)
				}
			}
		})
	}
}

func TestRegimeSpecialFactors(t *testing.T) {
	// Test regime-specific factor behaviors
	weightManager := regime.NewWeightManager()

	// Weekly carry should only appear in trending regimes
	trendingPreset, _ := weightManager.GetWeightsForRegime(regime.TrendingBull)
	choppyPreset, _ := weightManager.GetWeightsForRegime(regime.Choppy)
	highVolPreset, _ := weightManager.GetWeightsForRegime(regime.HighVol)

	// Weekly carry checks
	if trendingPreset.Weights["weekly_7d_carry"] <= 0 {
		t.Error("Trending bull should have positive weekly carry weight")
	}
	if choppyPreset.Weights["weekly_7d_carry"] != 0 {
		t.Error("Choppy regime should have zero weekly carry weight")
	}
	if highVolPreset.Weights["weekly_7d_carry"] != 0 {
		t.Error("High vol regime should have zero weekly carry weight")
	}

	// Quality emphasis in high volatility
	if highVolPreset.Weights["quality_score"] <= choppyPreset.Weights["quality_score"] {
		t.Error("High vol should emphasize quality more than choppy")
	}
	if highVolPreset.Weights["quality_score"] <= trendingPreset.Weights["quality_score"] {
		t.Error("High vol should emphasize quality more than trending")
	}

	// Social factor reduction in high vol
	if highVolPreset.Weights["social_sentiment"] >= choppyPreset.Weights["social_sentiment"] {
		t.Error("High vol should reduce social sentiment vs choppy")
	}
	if highVolPreset.Weights["social_sentiment"] >= trendingPreset.Weights["social_sentiment"] {
		t.Error("High vol should reduce social sentiment vs trending")
	}

	// Volume emphasis in choppy markets
	if choppyPreset.Weights["volume_surge"] <= trendingPreset.Weights["volume_surge"] {
		t.Error("Choppy should emphasize volume more than trending")
	}
	if choppyPreset.Weights["volume_surge"] <= highVolPreset.Weights["volume_surge"] {
		t.Error("Choppy should emphasize volume more than high vol")
	}
}
