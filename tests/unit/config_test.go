package unit

import (
	"path/filepath"
	"testing"
	"strings"

	"github.com/sawpanic/cryptorun/internal/application"
)

func TestWeightsConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *application.WeightsConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid weights configuration",
			config: &application.WeightsConfig{
				DefaultRegime: "bull",
				Validation: struct {
					WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
					MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
					MaxSocialWeight    float64 `yaml:"max_social_weight"`
					SocialHardCap      float64 `yaml:"social_hard_cap"`
				}{
					WeightSumTolerance: 0.001,
					MinMomentumWeight:  0.40,
					MaxSocialWeight:    0.15,
					SocialHardCap:      10.0,
				},
				Regimes: map[string]application.RegimeWeights{
					"bull": {
						Description:       "Bull market",
						MomentumCore:      0.50,
						TechnicalResidual: 0.20,
						VolumeResidual:    0.20,
						QualityResidual:   0.05,
						SocialResidual:    0.05,
					},
				},
			},
			expectError: false,
		},
		{
			name: "weights don't sum to 1.0",
			config: &application.WeightsConfig{
				Validation: struct {
					WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
					MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
					MaxSocialWeight    float64 `yaml:"max_social_weight"`
					SocialHardCap      float64 `yaml:"social_hard_cap"`
				}{
					WeightSumTolerance: 0.001,
					MinMomentumWeight:  0.40,
					MaxSocialWeight:    0.15,
					SocialHardCap:      10.0,
				},
				Regimes: map[string]application.RegimeWeights{
					"bull": {
						MomentumCore:      0.60,
						TechnicalResidual: 0.30,
						VolumeResidual:    0.20, // Total = 1.10
						QualityResidual:   0.05,
						SocialResidual:    0.05,
					},
				},
			},
			expectError: true,
			errorMsg:    "weights sum to",
		},
		{
			name: "momentum weight below minimum",
			config: &application.WeightsConfig{
				Validation: struct {
					WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
					MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
					MaxSocialWeight    float64 `yaml:"max_social_weight"`
					SocialHardCap      float64 `yaml:"social_hard_cap"`
				}{
					WeightSumTolerance: 0.001,
					MinMomentumWeight:  0.40,
					MaxSocialWeight:    0.15,
					SocialHardCap:      10.0,
				},
				Regimes: map[string]application.RegimeWeights{
					"bull": {
						MomentumCore:      0.30, // Below 0.40 minimum
						TechnicalResidual: 0.30,
						VolumeResidual:    0.20,
						QualityResidual:   0.15,
						SocialResidual:    0.05,
					},
				},
			},
			expectError: true,
			errorMsg:    "momentum weight",
		},
		{
			name: "social weight above maximum",
			config: &application.WeightsConfig{
				Validation: struct {
					WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
					MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
					MaxSocialWeight    float64 `yaml:"max_social_weight"`
					SocialHardCap      float64 `yaml:"social_hard_cap"`
				}{
					WeightSumTolerance: 0.001,
					MinMomentumWeight:  0.40,
					MaxSocialWeight:    0.15,
					SocialHardCap:      10.0,
				},
				Regimes: map[string]application.RegimeWeights{
					"bull": {
						MomentumCore:      0.50,
						TechnicalResidual: 0.20,
						VolumeResidual:    0.05,
						QualityResidual:   0.05,
						SocialResidual:    0.20, // Above 0.15 maximum
					},
				},
			},
			expectError: true,
			errorMsg:    "social weight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !containsSubstring(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestLoadWeightsConfig(t *testing.T) {
	// Test loading the actual weights config file
	configPath := filepath.Join("..", "..", "config", "weights.yaml")
	config, err := application.LoadWeightsConfig(configPath)
	
	if err != nil {
		t.Fatalf("failed to load weights config: %v", err)
	}

	// Validate the loaded config
	if config.DefaultRegime == "" {
		t.Error("default regime should not be empty")
	}

	// Check that all regimes have valid weights
	for regimeName, regime := range config.Regimes {
		total := regime.MomentumCore + regime.TechnicalResidual + regime.VolumeResidual + regime.QualityResidual + regime.SocialResidual
		if abs(total-1.0) > config.Validation.WeightSumTolerance {
			t.Errorf("regime %s weights sum to %.6f, expected 1.0 ±%.3f", regimeName, total, config.Validation.WeightSumTolerance)
		}

		if regime.MomentumCore < config.Validation.MinMomentumWeight {
			t.Errorf("regime %s momentum weight %.3f below minimum %.3f", regimeName, regime.MomentumCore, config.Validation.MinMomentumWeight)
		}

		if regime.SocialResidual > config.Validation.MaxSocialWeight {
			t.Errorf("regime %s social weight %.3f above maximum %.3f", regimeName, regime.SocialResidual, config.Validation.MaxSocialWeight)
		}
	}
}

func TestGuardsConfigGetActiveSettings(t *testing.T) {
	config := &application.GuardsConfig{
		ActiveProfile: "conservative",
		Profiles: map[string]application.GuardProfile{
			"conservative": {
				Name: "Conservative",
				Regimes: map[string]application.RegimeGuardSettings{
					"trending": {
						Fatigue: application.FatigueGuardConfig{
							Threshold24h: 12.0,
							RSI4h:        70,
						},
						Freshness: application.FreshnessGuardConfig{
							MaxBarsAge: 2,
							ATRFactor:  1.2,
						},
						LateFill: application.LateFillGuardConfig{
							MaxDelaySeconds: 30,
							P99LatencyReq:   400,
							ATRProximity:    1.2,
						},
					},
				},
			},
		},
	}

	// Test getting valid settings
	settings, err := config.GetActiveGuardSettings("trending")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if settings.Fatigue.Threshold24h != 12.0 {
		t.Errorf("expected fatigue threshold 12.0, got %.1f", settings.Fatigue.Threshold24h)
	}

	// Test getting settings for non-existent regime
	_, err = config.GetActiveGuardSettings("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent regime")
	}

	// Test with non-existent profile
	config.ActiveProfile = "nonexistent"
	_, err = config.GetActiveGuardSettings("trending")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}
}

func TestLoadGuardsConfig(t *testing.T) {
	// Test loading the actual guards config file
	configPath := filepath.Join("..", "..", "config", "guards.yaml")
	config, err := application.LoadGuardsConfig(configPath)
	
	if err != nil {
		t.Fatalf("failed to load guards config: %v", err)
	}

	if config.ActiveProfile == "" {
		t.Error("active profile should not be empty")
	}

	if len(config.Profiles) == 0 {
		t.Error("should have at least one guard profile")
	}

	// Test getting settings from the loaded config
	for profileName, profile := range config.Profiles {
		for regimeName := range profile.Regimes {
			config.ActiveProfile = profileName
			settings, err := config.GetActiveGuardSettings(regimeName)
			if err != nil {
				t.Errorf("failed to get settings for profile %s, regime %s: %v", profileName, regimeName, err)
			}
			
			// Validate reasonable threshold values
			if settings.Fatigue.Threshold24h <= 0 || settings.Fatigue.Threshold24h > 100 {
				t.Errorf("invalid fatigue threshold for %s/%s: %.1f", profileName, regimeName, settings.Fatigue.Threshold24h)
			}
			
			if settings.Freshness.MaxBarsAge <= 0 || settings.Freshness.MaxBarsAge > 10 {
				t.Errorf("invalid max bars age for %s/%s: %d", profileName, regimeName, settings.Freshness.MaxBarsAge)
			}
		}
	}
}

func TestFeatureFlagsIsEnabled(t *testing.T) {
	config := &application.FeatureFlagsConfig{
		Core: application.CoreFeatures{
			EnableRegimeAwareWeights: true,
			EnableAdaptiveGuards:     false,
		},
		Experimental: application.ExperimentalFeatures{
			EnableQuantumFactors: true,
			EnableAIOrthogonalization: false,
		},
		Safety: application.SafetyFeatures{
			EnableDryRun: true,
		},
	}

	tests := []struct {
		featurePath string
		expected    bool
	}{
		{"core.regime_aware_weights", true},
		{"core.adaptive_guards", false},
		{"experimental.quantum_factors", true},
		{"experimental.ai_orthogonalization", false},
		{"safety.dry_run", true},
		{"nonexistent.feature", false},
	}

	for _, tt := range tests {
		t.Run(tt.featurePath, func(t *testing.T) {
			result := config.IsEnabled(tt.featurePath)
			if result != tt.expected {
				t.Errorf("IsEnabled(%s) = %v, expected %v", tt.featurePath, result, tt.expected)
			}
		})
	}
}

func TestLoadFeatureFlagsConfig(t *testing.T) {
	// Test loading the actual feature flags config file
	configPath := filepath.Join("..", "..", "config", "feature_flags.yaml")
	config, err := application.LoadFeatureFlagsConfig(configPath)
	
	if err != nil {
		t.Fatalf("failed to load feature flags config: %v", err)
	}

	// Test some expected default values
	if !config.Core.EnableRegimeAwareWeights {
		t.Error("expected regime aware weights to be enabled by default")
	}

	if config.Experimental.EnableQuantumFactors {
		t.Error("expected quantum factors to be disabled by default")
	}

	if !config.Safety.EnableDryRun {
		t.Error("expected dry run to be enabled by default")
	}

	// Test IsEnabled with loaded config
	if !config.IsEnabled("core.regime_aware_weights") {
		t.Error("expected regime aware weights to be enabled")
	}

	if config.IsEnabled("experimental.quantum_factors") {
		t.Error("expected quantum factors to be disabled")
	}
}

func TestLoadLimitsConfig(t *testing.T) {
	// Test loading the actual limits config file
	configPath := filepath.Join("..", "..", "config", "limits.yaml")
	config, err := application.LoadLimitsConfig(configPath)
	
	if err != nil {
		t.Fatalf("failed to load limits config: %v", err)
	}

	// Validate reasonable limit values
	if config.Scanning.MaxPairs <= 0 {
		t.Error("max pairs should be positive")
	}

	if config.Entry.MinScore < 0 || config.Entry.MinScore > 100 {
		t.Errorf("invalid min score: %.1f", config.Entry.MinScore)
	}

	if config.Exit.MaxPositionHours <= 0 {
		t.Error("max position hours should be positive")
	}

	if config.Risk.MaxPositionSizeUSD <= 0 {
		t.Error("max position size should be positive")
	}

	if config.System.P99LatencyMaxMs <= 0 {
		t.Error("p99 latency max should be positive")
	}
}

// Helper functions
func containsSubstring(str, substr string) bool {
	return strings.Contains(str, substr)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}