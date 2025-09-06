package conformance

import (
	"testing"

	"gopkg.in/yaml.v2"
	"io/ioutil"
	"math"
	"path/filepath"
)

// WeightsConfig mirrors the config/weights.yaml structure
type WeightsConfig struct {
	Regimes map[string]RegimeWeights `yaml:"regimes"`
}

type RegimeWeights struct {
	Momentum   float64 `yaml:"momentum"`
	Volume     float64 `yaml:"volume"`
	Social     float64 `yaml:"social"`
	Volatility float64 `yaml:"volatility"`
}

// TestWeightsSumToOne enforces that all regime weights sum to exactly 1.0
func TestWeightsSumToOne(t *testing.T) {
	weightsPath := filepath.Join("..", "..", "config", "weights.yaml")
	data, err := ioutil.ReadFile(weightsPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read weights config: %v", err)
	}

	var config WeightsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot parse weights YAML: %v", err)
	}

	const tolerance = 0.001
	requiredRegimes := []string{"trending", "choppy", "high_vol"}

	for _, regimeName := range requiredRegimes {
		regime, exists := config.Regimes[regimeName]
		if !exists {
			t.Errorf("CONFORMANCE VIOLATION: Missing required regime '%s'", regimeName)
			continue
		}

		sum := regime.Momentum + regime.Volume + regime.Social + regime.Volatility
		if math.Abs(sum-1.0) > tolerance {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' weights sum to %.6f, must equal 1.0 (±%.3f)",
				regimeName, sum, tolerance)
		}

		// Individual weight bounds
		if regime.Momentum <= 0 || regime.Momentum > 1.0 {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' momentum weight %.3f out of bounds (0,1]",
				regimeName, regime.Momentum)
		}

		if regime.Volume < 0 || regime.Volume > 1.0 {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' volume weight %.3f out of bounds [0,1]",
				regimeName, regime.Volume)
		}

		if regime.Social < 0 || regime.Social > 0.15 { // Social cap at 15% max
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' social weight %.3f exceeds 15%% cap",
				regimeName, regime.Social)
		}

		if regime.Volatility < 0 || regime.Volatility > 1.0 {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' volatility weight %.3f out of bounds [0,1]",
				regimeName, regime.Volatility)
		}
	}
}

// TestMomentumDominance ensures momentum weight is always highest in all regimes
func TestMomentumDominance(t *testing.T) {
	weightsPath := filepath.Join("..", "..", "config", "weights.yaml")
	data, err := ioutil.ReadFile(weightsPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read weights config: %v", err)
	}

	var config WeightsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot parse weights YAML: %v", err)
	}

	for regimeName, regime := range config.Regimes {
		if regime.Momentum <= regime.Volume {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' momentum weight %.3f not dominant over volume %.3f",
				regimeName, regime.Momentum, regime.Volume)
		}

		if regime.Momentum <= regime.Social {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' momentum weight %.3f not dominant over social %.3f",
				regimeName, regime.Momentum, regime.Social)
		}

		if regime.Momentum <= regime.Volatility {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' momentum weight %.3f not dominant over volatility %.3f",
				regimeName, regime.Momentum, regime.Volatility)
		}

		// Minimum momentum threshold (must be at least 40%)
		const minMomentum = 0.40
		if regime.Momentum < minMomentum {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' momentum weight %.3f below minimum %.3f",
				regimeName, regime.Momentum, minMomentum)
		}
	}
}

// TestSocialCapEnforcement verifies social weight never exceeds 10%
func TestSocialCapEnforcement(t *testing.T) {
	weightsPath := filepath.Join("..", "..", "config", "weights.yaml")
	data, err := ioutil.ReadFile(weightsPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read weights config: %v", err)
	}

	var config WeightsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot parse weights YAML: %v", err)
	}

	const maxSocial = 0.10 // 10% cap
	for regimeName, regime := range config.Regimes {
		if regime.Social > maxSocial {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' social weight %.3f exceeds 10%% cap",
				regimeName, regime.Social)
		}
	}
}

// TestWeightsPrecision ensures weights use reasonable precision (3 decimal places)
func TestWeightsPrecision(t *testing.T) {
	weightsPath := filepath.Join("..", "..", "config", "weights.yaml")
	data, err := ioutil.ReadFile(weightsPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read weights config: %v", err)
	}

	var config WeightsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot parse weights YAML: %v", err)
	}

	for regimeName, regime := range config.Regimes {
		weights := map[string]float64{
			"momentum":   regime.Momentum,
			"volume":     regime.Volume,
			"social":     regime.Social,
			"volatility": regime.Volatility,
		}

		for factorName, weight := range weights {
			// Check for excessive precision (more than 3 decimal places)
			rounded := math.Round(weight*1000) / 1000
			if math.Abs(weight-rounded) > 1e-6 {
				t.Errorf("CONFORMANCE VIOLATION: Regime '%s' %s weight %.6f has excessive precision, use 3 decimals max",
					regimeName, factorName, weight)
			}
		}
	}
}
