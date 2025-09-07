package gates

import (
	"os"
	"path/filepath"
	"testing"
	"gopkg.in/yaml.v3"
)

func TestNewThresholdRouter(t *testing.T) {
	// Create a temporary config file for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_thresholds.yaml")

	// Write test config
	testConfig := RegimeThresholdConfig{
		Default: RegimeThresholds{SpreadMaxBps: 45.0, DepthMinUSD: 90000.0, VADRMin: 1.7},
		Trending: RegimeThresholds{SpreadMaxBps: 50.0, DepthMinUSD: 95000.0, VADRMin: 1.5},
		Choppy: RegimeThresholds{SpreadMaxBps: 35.0, DepthMinUSD: 140000.0, VADRMin: 1.9},
		HighVol: RegimeThresholds{SpreadMaxBps: 32.0, DepthMinUSD: 165000.0, VADRMin: 2.05},
		RiskOff: RegimeThresholds{SpreadMaxBps: 28.0, DepthMinUSD: 195000.0, VADRMin: 2.15},
		Universal: UniversalThresholds{DepthRangePct: 2.5, FreshnessMaxBars: 3, LateFillMaxSeconds: 25, MinDailyVolumeUSD: 400000.0},
	}

	yamlData, err := yaml.Marshal(&testConfig)
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}

	if err := os.WriteFile(configPath, yamlData, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test loading from file
	router, err := NewThresholdRouter(configPath)
	if err != nil {
		t.Fatalf("Failed to create threshold router: %v", err)
	}

	// Verify loaded values
	trending := router.SelectThresholds("trending")
	if trending.SpreadMaxBps != 50.0 {
		t.Errorf("Expected trending spread 50.0, got %.1f", trending.SpreadMaxBps)
	}
	if trending.DepthMinUSD != 95000.0 {
		t.Errorf("Expected trending depth 95000.0, got %.1f", trending.DepthMinUSD)
	}
	if trending.VADRMin != 1.5 {
		t.Errorf("Expected trending VADR 1.5, got %.2f", trending.VADRMin)
	}
}

func TestThresholdRouterWithDefaults(t *testing.T) {
	router := NewThresholdRouterWithDefaults()

	// Test default regime selection
	defaultThresholds := router.SelectThresholds("unknown_regime")
	if defaultThresholds.SpreadMaxBps != 50.0 {
		t.Errorf("Expected default spread 50.0, got %.1f", defaultThresholds.SpreadMaxBps)
	}
}

func TestSelectThresholds(t *testing.T) {
	router := NewThresholdRouterWithDefaults()

	testCases := []struct {
		regime          string
		expectedSpread  float64
		expectedDepth   float64
		expectedVADR    float64
		description     string
	}{
		{"trending", 55.0, 100000.0, 1.6, "trending regime should have relaxed thresholds"},
		{"trending_bull", 55.0, 100000.0, 1.6, "trending_bull should map to trending"},
		{"bull", 55.0, 100000.0, 1.6, "bull should map to trending"},
		{"choppy", 40.0, 150000.0, 2.0, "choppy regime should have tighter thresholds"},
		{"chop", 40.0, 150000.0, 2.0, "chop should map to choppy"},
		{"sideways", 40.0, 150000.0, 2.0, "sideways should map to choppy"},
		{"high_vol", 35.0, 175000.0, 2.1, "high_vol regime should have strict thresholds"},
		{"volatile", 35.0, 175000.0, 2.1, "volatile should map to high_vol"},
		{"high_volatility", 35.0, 175000.0, 2.1, "high_volatility should map to high_vol"},
		{"risk_off", 30.0, 200000.0, 2.2, "risk_off regime should have strictest thresholds"},
		{"bear", 30.0, 200000.0, 2.2, "bear should map to risk_off"},
		{"crisis", 30.0, 200000.0, 2.2, "crisis should map to risk_off"},
		{"unknown", 50.0, 100000.0, 1.75, "unknown regime should use default"},
	}

	for _, tc := range testCases {
		t.Run(tc.regime, func(t *testing.T) {
			thresholds := router.SelectThresholds(tc.regime)

			if thresholds.SpreadMaxBps != tc.expectedSpread {
				t.Errorf("%s: expected spread %.1f, got %.1f", tc.description, tc.expectedSpread, thresholds.SpreadMaxBps)
			}
			if thresholds.DepthMinUSD != tc.expectedDepth {
				t.Errorf("%s: expected depth %.1f, got %.1f", tc.description, tc.expectedDepth, thresholds.DepthMinUSD)
			}
			if thresholds.VADRMin != tc.expectedVADR {
				t.Errorf("%s: expected VADR %.2f, got %.2f", tc.description, tc.expectedVADR, thresholds.VADRMin)
			}
		})
	}
}

func TestGetUniversalThresholds(t *testing.T) {
	router := NewThresholdRouterWithDefaults()
	universal := router.GetUniversalThresholds()

	if universal.DepthRangePct != 2.0 {
		t.Errorf("Expected depth range 2.0%%, got %.1f%%", universal.DepthRangePct)
	}
	if universal.FreshnessMaxBars != 2 {
		t.Errorf("Expected freshness 2 bars, got %d", universal.FreshnessMaxBars)
	}
	if universal.LateFillMaxSeconds != 30 {
		t.Errorf("Expected late-fill 30 seconds, got %d", universal.LateFillMaxSeconds)
	}
	if universal.MinDailyVolumeUSD != 500000.0 {
		t.Errorf("Expected min daily volume 500000.0, got %.1f", universal.MinDailyVolumeUSD)
	}
}

func TestGetAllRegimeThresholds(t *testing.T) {
	router := NewThresholdRouterWithDefaults()
	allThresholds := router.GetAllRegimeThresholds()

	expectedRegimes := []string{"default", "trending", "choppy", "high_vol", "risk_off"}
	for _, regime := range expectedRegimes {
		if _, exists := allThresholds[regime]; !exists {
			t.Errorf("Expected regime %s to exist in all thresholds map", regime)
		}
	}

	// Verify thresholds are ordered correctly (trending most relaxed, risk_off strictest)
	if allThresholds["trending"].SpreadMaxBps <= allThresholds["risk_off"].SpreadMaxBps {
		t.Error("Trending should have more relaxed (higher) spread threshold than risk_off")
	}
	if allThresholds["trending"].DepthMinUSD >= allThresholds["risk_off"].DepthMinUSD {
		t.Error("Trending should have lower depth requirement than risk_off")
	}
	if allThresholds["trending"].VADRMin >= allThresholds["risk_off"].VADRMin {
		t.Error("Trending should have lower VADR requirement than risk_off")
	}
}

func TestDescribeThresholds(t *testing.T) {
	router := NewThresholdRouterWithDefaults()
	
	description := router.DescribeThresholds("choppy")
	expectedParts := []string{
		"choppy", "40.0 bps", "$150k", "±2.0%", "2.00x", "≤2 bars", "≤30s",
	}
	
	for _, part := range expectedParts {
		if !contains(description, part) {
			t.Errorf("Description '%s' should contain '%s'", description, part)
		}
	}
}

func TestValidateThresholdConfig(t *testing.T) {
	// Test valid config
	validConfig := &RegimeThresholdConfig{
		Default:  RegimeThresholds{SpreadMaxBps: 50.0, DepthMinUSD: 100000.0, VADRMin: 1.75},
		Trending: RegimeThresholds{SpreadMaxBps: 55.0, DepthMinUSD: 100000.0, VADRMin: 1.6},
		Choppy:   RegimeThresholds{SpreadMaxBps: 40.0, DepthMinUSD: 150000.0, VADRMin: 2.0},
		HighVol:  RegimeThresholds{SpreadMaxBps: 35.0, DepthMinUSD: 175000.0, VADRMin: 2.1},
		RiskOff:  RegimeThresholds{SpreadMaxBps: 30.0, DepthMinUSD: 200000.0, VADRMin: 2.2},
		Universal: UniversalThresholds{DepthRangePct: 2.0, FreshnessMaxBars: 2, LateFillMaxSeconds: 30, MinDailyVolumeUSD: 500000.0},
	}
	
	if err := validateThresholdConfig(validConfig); err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}

	// Test invalid spread (too high)
	invalidSpread := *validConfig
	invalidSpread.Default.SpreadMaxBps = 600.0
	if err := validateThresholdConfig(&invalidSpread); err == nil {
		t.Error("Config with invalid spread should fail validation")
	}

	// Test invalid depth (negative)
	invalidDepth := *validConfig
	invalidDepth.Choppy.DepthMinUSD = -1000.0
	if err := validateThresholdConfig(&invalidDepth); err == nil {
		t.Error("Config with invalid depth should fail validation")
	}

	// Test invalid VADR (too high)
	invalidVADR := *validConfig
	invalidVADR.HighVol.VADRMin = 15.0
	if err := validateThresholdConfig(&invalidVADR); err == nil {
		t.Error("Config with invalid VADR should fail validation")
	}

	// Test invalid universal depth range
	invalidRange := *validConfig
	invalidRange.Universal.DepthRangePct = 20.0
	if err := validateThresholdConfig(&invalidRange); err == nil {
		t.Error("Config with invalid depth range should fail validation")
	}
}

func TestThresholdOrdering(t *testing.T) {
	// Verify that threshold strictness follows expected order:
	// trending (most relaxed) > choppy > high_vol > risk_off (strictest)
	router := NewThresholdRouterWithDefaults()
	
	trending := router.SelectThresholds("trending")
	choppy := router.SelectThresholds("choppy")
	highVol := router.SelectThresholds("high_vol")
	riskOff := router.SelectThresholds("risk_off")

	// Spread thresholds (higher = more relaxed)
	if trending.SpreadMaxBps <= choppy.SpreadMaxBps {
		t.Error("Trending spread should be more relaxed than choppy")
	}
	if choppy.SpreadMaxBps <= highVol.SpreadMaxBps {
		t.Error("Choppy spread should be more relaxed than high_vol")
	}
	if highVol.SpreadMaxBps <= riskOff.SpreadMaxBps {
		t.Error("High_vol spread should be more relaxed than risk_off")
	}

	// Depth thresholds (lower = more relaxed)
	if trending.DepthMinUSD >= choppy.DepthMinUSD {
		t.Error("Trending depth should be more relaxed than choppy")
	}
	if choppy.DepthMinUSD >= highVol.DepthMinUSD {
		t.Error("Choppy depth should be more relaxed than high_vol")
	}
	if highVol.DepthMinUSD >= riskOff.DepthMinUSD {
		t.Error("High_vol depth should be more relaxed than risk_off")
	}

	// VADR thresholds (lower = more relaxed)
	if trending.VADRMin >= choppy.VADRMin {
		t.Error("Trending VADR should be more relaxed than choppy")
	}
	if choppy.VADRMin >= highVol.VADRMin {
		t.Error("Choppy VADR should be more relaxed than high_vol")
	}
	if highVol.VADRMin >= riskOff.VADRMin {
		t.Error("High_vol VADR should be more relaxed than risk_off")
	}
}

// Note: contains helper function is defined in entry_test.go