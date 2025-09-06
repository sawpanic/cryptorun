package conformance_test

import (
	"math"
	"testing"

	"gopkg.in/yaml.v3"
	"os"
)

// WeightConfig represents momentum weight configuration
type WeightConfig struct {
	Weights struct {
		TF1h  float64 `yaml:"tf_1h"`
		TF4h  float64 `yaml:"tf_4h"`
		TF12h float64 `yaml:"tf_12h"`
		TF24h float64 `yaml:"tf_24h"`
		TF7d  float64 `yaml:"tf_7d,omitempty"`
	} `yaml:"weights"`

	RegimeWeights struct {
		Trending struct {
			TF1hMultiplier  float64 `yaml:"tf_1h_multiplier"`
			TF4hMultiplier  float64 `yaml:"tf_4h_multiplier"`
			TF12hMultiplier float64 `yaml:"tf_12h_multiplier"`
			TF24hMultiplier float64 `yaml:"tf_24h_multiplier"`
			TF7dMultiplier  float64 `yaml:"tf_7d_multiplier,omitempty"`
		} `yaml:"trending"`

		Choppy struct {
			TF1hMultiplier  float64 `yaml:"tf_1h_multiplier"`
			TF4hMultiplier  float64 `yaml:"tf_4h_multiplier"`
			TF12hMultiplier float64 `yaml:"tf_12h_multiplier"`
			TF24hMultiplier float64 `yaml:"tf_24h_multiplier"`
			TF7dMultiplier  float64 `yaml:"tf_7d_multiplier,omitempty"`
		} `yaml:"choppy"`

		Volatile struct {
			TF1hMultiplier  float64 `yaml:"tf_1h_multiplier"`
			TF4hMultiplier  float64 `yaml:"tf_4h_multiplier"`
			TF12hMultiplier float64 `yaml:"tf_12h_multiplier"`
			TF24hMultiplier float64 `yaml:"tf_24h_multiplier"`
			TF7dMultiplier  float64 `yaml:"tf_7d_multiplier,omitempty"`
		} `yaml:"volatile"`
	} `yaml:"regime_weights"`
}

const tolerance = 1e-6

// TestMomentumWeightSumConformance verifies weight sum = 1.0 across all regimes
func TestMomentumWeightSumConformance(t *testing.T) {
	// Load momentum configuration
	configData, err := os.ReadFile("config/momentum.yaml")
	if err != nil {
		t.Fatalf("Failed to read momentum config: %v", err)
	}

	var config WeightConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse momentum config: %v", err)
	}

	// Test baseline weight sum
	t.Run("BaselineWeightSum", func(t *testing.T) {
		sum := config.Weights.TF1h + config.Weights.TF4h + config.Weights.TF12h + config.Weights.TF24h
		if config.Weights.TF7d > 0 {
			sum += config.Weights.TF7d
		}

		if math.Abs(sum-1.0) > tolerance {
			t.Errorf("CONFORMANCE VIOLATION: Baseline weight sum = %.6f, must equal 1.0", sum)
		}
	})

	// Test regime-adjusted weight sums
	regimes := []struct {
		name        string
		multipliers []float64
	}{
		{
			name: "Trending",
			multipliers: []float64{
				config.RegimeWeights.Trending.TF1hMultiplier,
				config.RegimeWeights.Trending.TF4hMultiplier,
				config.RegimeWeights.Trending.TF12hMultiplier,
				config.RegimeWeights.Trending.TF24hMultiplier,
				config.RegimeWeights.Trending.TF7dMultiplier,
			},
		},
		{
			name: "Choppy",
			multipliers: []float64{
				config.RegimeWeights.Choppy.TF1hMultiplier,
				config.RegimeWeights.Choppy.TF4hMultiplier,
				config.RegimeWeights.Choppy.TF12hMultiplier,
				config.RegimeWeights.Choppy.TF24hMultiplier,
				config.RegimeWeights.Choppy.TF7dMultiplier,
			},
		},
		{
			name: "Volatile",
			multipliers: []float64{
				config.RegimeWeights.Volatile.TF1hMultiplier,
				config.RegimeWeights.Volatile.TF4hMultiplier,
				config.RegimeWeights.Volatile.TF12hMultiplier,
				config.RegimeWeights.Volatile.TF24hMultiplier,
				config.RegimeWeights.Volatile.TF7dMultiplier,
			},
		},
	}

	baseWeights := []float64{
		config.Weights.TF1h,
		config.Weights.TF4h,
		config.Weights.TF12h,
		config.Weights.TF24h,
		config.Weights.TF7d,
	}

	for _, regime := range regimes {
		t.Run(regime.name+"RegimeWeightSum", func(t *testing.T) {
			var adjustedSum float64

			for i, baseWeight := range baseWeights {
				if baseWeight > 0 || regime.multipliers[i] > 0 {
					adjustedSum += baseWeight * regime.multipliers[i]
				}
			}

			if math.Abs(adjustedSum-1.0) > tolerance {
				t.Errorf("CONFORMANCE VIOLATION: %s regime weight sum = %.6f, must equal 1.0",
					regime.name, adjustedSum)
			}
		})
	}
}

// TestWeightBoundaryConformance verifies PRD weight boundaries
func TestWeightBoundaryConformance(t *testing.T) {
	// Load momentum configuration
	configData, err := os.ReadFile("config/momentum.yaml")
	if err != nil {
		t.Fatalf("Failed to read momentum config: %v", err)
	}

	var config WeightConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse momentum config: %v", err)
	}

	// Test 24h weight boundary [0.10, 0.15]
	t.Run("TF24hBoundary", func(t *testing.T) {
		w24h := config.Weights.TF24h
		if w24h < 0.10 || w24h > 0.15 {
			t.Errorf("CONFORMANCE VIOLATION: 24h weight = %.3f, must be in [0.10, 0.15]", w24h)
		}
	})

	// Test 7d weight boundary [0.05, 0.10] if present
	if config.Weights.TF7d > 0 {
		t.Run("TF7dBoundary", func(t *testing.T) {
			w7d := config.Weights.TF7d
			if w7d < 0.05 || w7d > 0.10 {
				t.Errorf("CONFORMANCE VIOLATION: 7d weight = %.3f, must be in [0.05, 0.10]", w7d)
			}
		})
	}
}
