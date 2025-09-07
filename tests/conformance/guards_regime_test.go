package conformance

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

// RegimeConfig mirrors expected regime configuration structure
type RegimeConfig struct {
	Regimes map[string]RegimeSettings `yaml:"regimes"`
	Guards  GuardsConfig              `yaml:"guards"`
}

type RegimeSettings struct {
	Name        string             `yaml:"name"`
	Enabled     bool               `yaml:"enabled"`
	Multipliers map[string]float64 `yaml:"multipliers,omitempty"`
}

type GuardsConfig struct {
	RegimeAware bool                      `yaml:"regime_aware"`
	Fatigue     map[string]FatigueGuard   `yaml:"fatigue"`
	Freshness   map[string]FreshnessGuard `yaml:"freshness"`
	LateFill    map[string]LateFillGuard  `yaml:"late_fill"`
}

type FatigueGuard struct {
	Threshold float64 `yaml:"threshold"`
	RSILimit  float64 `yaml:"rsi_limit"`
}

type FreshnessGuard struct {
	MaxBarsAge int     `yaml:"max_bars_age"`
	ATRLimit   float64 `yaml:"atr_limit"`
}

type LateFillGuard struct {
	MaxDelaySeconds int `yaml:"max_delay_seconds"`
}

// TestGuardsRegimeEnforcement verifies guards respect regime settings when enabled
func TestGuardsRegimeEnforcement(t *testing.T) {
	// Check if regime-aware config exists
	regimePath := filepath.Join("..", "..", "config", "regimes.yaml")
	if !fileExists(regimePath) {
		t.Skip("CONFORMANCE SKIP: regimes.yaml not found - regime awareness not implemented")
		return
	}

	data, err := ioutil.ReadFile(regimePath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read regimes.yaml: %v", err)
	}

	var config RegimeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot parse regimes.yaml: %v", err)
	}

	// If regime_aware is true, must have regime-specific guard settings
	if config.Guards.RegimeAware {
		t.Log("CONFORMANCE CHECK: Regime-aware guards enabled - validating regime-specific settings")

		requiredRegimes := []string{"trending", "choppy", "high_vol"}
		for _, regimeName := range requiredRegimes {
			// Check fatigue guard has regime-specific settings
			if _, exists := config.Guards.Fatigue[regimeName]; !exists {
				t.Errorf("CONFORMANCE VIOLATION: Missing fatigue guard settings for regime '%s'", regimeName)
			}

			// Check freshness guard has regime-specific settings
			if _, exists := config.Guards.Freshness[regimeName]; !exists {
				t.Errorf("CONFORMANCE VIOLATION: Missing freshness guard settings for regime '%s'", regimeName)
			}

			// Check late-fill guard has regime-specific settings
			if _, exists := config.Guards.LateFill[regimeName]; !exists {
				t.Errorf("CONFORMANCE VIOLATION: Missing late-fill guard settings for regime '%s'", regimeName)
			}
		}

		// Verify regime-specific threshold variations make sense
		if len(config.Guards.Fatigue) >= 2 {
			validateFatigueRegimeDifferences(t, config.Guards.Fatigue)
		}
	} else {
		t.Log("CONFORMANCE CHECK: Regime-aware guards disabled - validating legacy behavior")

		// When regime_aware is false, should have default/baseline settings only
		if len(config.Guards.Fatigue) > 1 {
			t.Error("CONFORMANCE VIOLATION: Multiple fatigue settings found but regime_aware=false")
		}
	}
}

// TestLegacyGuardsBehavior ensures guards work without regime awareness
func TestLegacyGuardsBehavior(t *testing.T) {
	// Check guards implementation can handle non-regime-aware mode
	guardsPath := filepath.Join("..", "..", "internal", "domain", "gates.go")

	if !fileExists(guardsPath) {
		t.Skip("CONFORMANCE SKIP: gates.go not found - guards not implemented")
		return
	}

	content, err := readFileContent(guardsPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read gates.go: %v", err)
	}

	// Look for fallback/default behavior when regime is not provided
	requiredPatterns := []string{
		"default", "Default", // Must have default values
		"baseline", "Baseline", // Must have baseline thresholds
	}

	for _, pattern := range requiredPatterns {
		if !strings.Contains(content, pattern) {
			t.Errorf("CONFORMANCE VIOLATION: gates.go missing '%s' pattern for legacy compatibility", pattern)
		}
	}

	// Must not require regime parameter in all guard functions
	forbiddenPatterns := []string{
		"func.*Guard.*regime.*string", // Guard functions must not require regime
	}

	for _, pattern := range forbiddenPatterns {
		if containsPattern(content, pattern) {
			t.Errorf("CONFORMANCE VIOLATION: gates.go contains mandatory regime parameter pattern '%s'", pattern)
		}
	}
}

// TestRegimeToggleConsistency verifies regime flag controls behavior consistently
func TestRegimeToggleConsistency(t *testing.T) {
	configPath := filepath.Join("..", "..", "config", "regimes.yaml")
	if !fileExists(configPath) {
		t.Skip("CONFORMANCE SKIP: regimes.yaml not found")
		return
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read regimes.yaml: %v", err)
	}

	var config RegimeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot parse regimes.yaml: %v", err)
	}

	// When regime_aware=true, all guards must have regime-specific configs
	if config.Guards.RegimeAware {
		guardTypes := []string{"fatigue", "freshness", "late_fill"}
		for _, guardType := range guardTypes {
			var guardCount int
			switch guardType {
			case "fatigue":
				guardCount = len(config.Guards.Fatigue)
			case "freshness":
				guardCount = len(config.Guards.Freshness)
			case "late_fill":
				guardCount = len(config.Guards.LateFill)
			}

			if guardCount < 2 { // Should have at least 2 regimes configured
				t.Errorf("CONFORMANCE VIOLATION: regime_aware=true but %s guard has only %d configuration(s)",
					guardType, guardCount)
			}
		}
	}
}

// TestGuardThresholdRanges ensures guard thresholds are within reasonable bounds
func TestGuardThresholdRanges(t *testing.T) {
	configPath := filepath.Join("..", "..", "config", "regimes.yaml")
	if !fileExists(configPath) {
		t.Skip("CONFORMANCE SKIP: regimes.yaml not found")
		return
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read regimes.yaml: %v", err)
	}

	var config RegimeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot parse regimes.yaml: %v", err)
	}

	// Validate fatigue thresholds
	for regimeName, fatigue := range config.Guards.Fatigue {
		if fatigue.Threshold < 5.0 || fatigue.Threshold > 25.0 {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' fatigue threshold %.1f%% outside reasonable range [5%%,25%%]",
				regimeName, fatigue.Threshold)
		}
		if fatigue.RSILimit < 60 || fatigue.RSILimit > 80 {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' RSI limit %.1f outside reasonable range [60,80]",
				regimeName, fatigue.RSILimit)
		}
	}

	// Validate freshness settings
	for regimeName, freshness := range config.Guards.Freshness {
		if freshness.MaxBarsAge < 1 || freshness.MaxBarsAge > 5 {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' max bars age %d outside reasonable range [1,5]",
				regimeName, freshness.MaxBarsAge)
		}
		if freshness.ATRLimit < 0.5 || freshness.ATRLimit > 2.0 {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' ATR limit %.1f outside reasonable range [0.5,2.0]",
				regimeName, freshness.ATRLimit)
		}
	}

	// Validate late-fill settings
	for regimeName, lateFill := range config.Guards.LateFill {
		if lateFill.MaxDelaySeconds < 15 || lateFill.MaxDelaySeconds > 60 {
			t.Errorf("CONFORMANCE VIOLATION: Regime '%s' max delay %ds outside reasonable range [15s,60s]",
				regimeName, lateFill.MaxDelaySeconds)
		}
	}
}

// Helper functions
func validateFatigueRegimeDifferences(t *testing.T, fatigueGuards map[string]FatigueGuard) {
	// Trending regime should have higher thresholds than choppy
	trending, hasTrending := fatigueGuards["trending"]
	choppy, hasChoppy := fatigueGuards["choppy"]

	if hasTrending && hasChoppy {
		if trending.Threshold <= choppy.Threshold {
			t.Errorf("CONFORMANCE VIOLATION: Trending fatigue threshold %.1f should be higher than choppy %.1f",
				trending.Threshold, choppy.Threshold)
		}
	}
}

func fileExists(path string) bool {
	_, err := ioutil.ReadFile(path)
	return err == nil
}
