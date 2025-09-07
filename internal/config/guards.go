package config

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// GuardsConfig represents the guards configuration structure
type GuardsConfig struct {
	RegimeAware bool                    `yaml:"regime_aware"`
	Profiles    map[string]GuardProfile `yaml:"profiles"`
	Active      string                  `yaml:"active_profile"`
}

// GuardProfile represents a set of guard thresholds for a regime
type GuardProfile struct {
	Name        string                  `yaml:"name"`
	Description string                  `yaml:"description"`
	Regimes     map[string]RegimeGuards `yaml:"regimes"`
}

// RegimeGuards represents guard thresholds for a specific regime
type RegimeGuards struct {
	Fatigue   FatigueGuardConfig   `yaml:"fatigue"`
	Freshness FreshnessGuardConfig `yaml:"freshness"`
	LateFill  LateFillGuardConfig  `yaml:"late_fill"`
}

// FatigueGuardConfig represents fatigue guard thresholds
type FatigueGuardConfig struct {
	Threshold24h float64 `yaml:"threshold_24h"` // 24h momentum threshold %
	RSI4h        float64 `yaml:"rsi_4h"`        // RSI 4h threshold
}

// FreshnessGuardConfig represents freshness guard thresholds
type FreshnessGuardConfig struct {
	MaxBarsAge int     `yaml:"max_bars_age"` // Maximum bar age
	ATRFactor  float64 `yaml:"atr_factor"`   // ATR proximity factor
}

// LateFillGuardConfig represents late-fill guard thresholds
type LateFillGuardConfig struct {
	MaxDelaySeconds int     `yaml:"max_delay_seconds"` // Maximum execution delay
	P99LatencyReq   float64 `yaml:"p99_latency_req"`   // P99 latency requirement for trending
	ATRProximity    float64 `yaml:"atr_proximity"`     // ATR proximity requirement
}

// LoadGuardsConfig loads guards configuration from file
func LoadGuardsConfig(configPath string) (*GuardsConfig, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read guards config: %w", err)
	}

	var config GuardsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse guards YAML: %w", err)
	}

	return &config, nil
}

// SaveGuardsConfig saves guards configuration to file
func SaveGuardsConfig(config *GuardsConfig, configPath string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal guards config: %w", err)
	}

	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write guards config: %w", err)
	}

	return nil
}

// GetActiveProfile returns the currently active guard profile
func (gc *GuardsConfig) GetActiveProfile() (*GuardProfile, error) {
	if gc.Active == "" {
		return nil, fmt.Errorf("no active profile set")
	}

	profile, exists := gc.Profiles[gc.Active]
	if !exists {
		return nil, fmt.Errorf("active profile '%s' not found", gc.Active)
	}

	return &profile, nil
}

// GetRegimeThresholds returns guard thresholds for a specific regime
func (gp *GuardProfile) GetRegimeThresholds(regime string) (*RegimeGuards, error) {
	guards, exists := gp.Regimes[regime]
	if !exists {
		return nil, fmt.Errorf("regime '%s' not found in profile '%s'", regime, gp.Name)
	}

	return &guards, nil
}

// ValidateProfile validates a guard profile for safety and consistency
func (gp *GuardProfile) ValidateProfile() []string {
	var errors []string

	requiredRegimes := []string{"trending", "choppy", "high_vol"}
	for _, regime := range requiredRegimes {
		guards, exists := gp.Regimes[regime]
		if !exists {
			errors = append(errors, fmt.Sprintf("Missing regime configuration: %s", regime))
			continue
		}

		// Validate fatigue thresholds
		if guards.Fatigue.Threshold24h < 5.0 || guards.Fatigue.Threshold24h > 25.0 {
			errors = append(errors, fmt.Sprintf("Regime %s: fatigue threshold %.1f%% outside [5%%, 25%%] range", regime, guards.Fatigue.Threshold24h))
		}

		if guards.Fatigue.RSI4h < 60 || guards.Fatigue.RSI4h > 80 {
			errors = append(errors, fmt.Sprintf("Regime %s: RSI threshold %.0f outside [60, 80] range", regime, guards.Fatigue.RSI4h))
		}

		// Validate freshness thresholds
		if guards.Freshness.MaxBarsAge < 1 || guards.Freshness.MaxBarsAge > 5 {
			errors = append(errors, fmt.Sprintf("Regime %s: max bars age %d outside [1, 5] range", regime, guards.Freshness.MaxBarsAge))
		}

		if guards.Freshness.ATRFactor < 0.8 || guards.Freshness.ATRFactor > 2.0 {
			errors = append(errors, fmt.Sprintf("Regime %s: ATR factor %.1f outside [0.8, 2.0] range", regime, guards.Freshness.ATRFactor))
		}

		// Validate late-fill thresholds
		if guards.LateFill.MaxDelaySeconds < 15 || guards.LateFill.MaxDelaySeconds > 60 {
			errors = append(errors, fmt.Sprintf("Regime %s: max delay %ds outside [15s, 60s] range", regime, guards.LateFill.MaxDelaySeconds))
		}

		// Trending regime specific validations
		if regime == "trending" {
			if guards.LateFill.P99LatencyReq == 0 {
				guards.LateFill.P99LatencyReq = 400.0 // Default requirement
			}
			if guards.LateFill.P99LatencyReq > 500 {
				errors = append(errors, fmt.Sprintf("Trending regime: P99 latency requirement %.0fms exceeds 500ms safety limit", guards.LateFill.P99LatencyReq))
			}

			if guards.LateFill.ATRProximity == 0 {
				guards.LateFill.ATRProximity = 1.2 // Default proximity
			}
			if guards.LateFill.ATRProximity > 2.0 {
				errors = append(errors, fmt.Sprintf("Trending regime: ATR proximity %.1f× exceeds 2.0× safety limit", guards.LateFill.ATRProximity))
			}
		}
	}

	return errors
}

// GetDefaultGuardsConfig returns a safe default guards configuration
func GetDefaultGuardsConfig() *GuardsConfig {
	return &GuardsConfig{
		RegimeAware: false, // Start with legacy behavior
		Active:      "conservative",
		Profiles: map[string]GuardProfile{
			"conservative": {
				Name:        "Conservative",
				Description: "Safe baseline thresholds for all regimes",
				Regimes: map[string]RegimeGuards{
					"trending": {
						Fatigue:   FatigueGuardConfig{Threshold24h: 12.0, RSI4h: 70},
						Freshness: FreshnessGuardConfig{MaxBarsAge: 2, ATRFactor: 1.2},
						LateFill:  LateFillGuardConfig{MaxDelaySeconds: 30, P99LatencyReq: 400, ATRProximity: 1.2},
					},
					"choppy": {
						Fatigue:   FatigueGuardConfig{Threshold24h: 12.0, RSI4h: 70},
						Freshness: FreshnessGuardConfig{MaxBarsAge: 2, ATRFactor: 1.2},
						LateFill:  LateFillGuardConfig{MaxDelaySeconds: 30},
					},
					"high_vol": {
						Fatigue:   FatigueGuardConfig{Threshold24h: 12.0, RSI4h: 65},
						Freshness: FreshnessGuardConfig{MaxBarsAge: 2, ATRFactor: 1.0},
						LateFill:  LateFillGuardConfig{MaxDelaySeconds: 30},
					},
				},
			},
			"trending_risk_on": {
				Name:        "Trending Risk-On",
				Description: "Relaxed thresholds for trending markets with safety conditions",
				Regimes: map[string]RegimeGuards{
					"trending": {
						Fatigue:   FatigueGuardConfig{Threshold24h: 18.0, RSI4h: 70},
						Freshness: FreshnessGuardConfig{MaxBarsAge: 3, ATRFactor: 1.2},
						LateFill:  LateFillGuardConfig{MaxDelaySeconds: 45, P99LatencyReq: 400, ATRProximity: 1.2},
					},
					"choppy": {
						Fatigue:   FatigueGuardConfig{Threshold24h: 12.0, RSI4h: 70},
						Freshness: FreshnessGuardConfig{MaxBarsAge: 2, ATRFactor: 1.2},
						LateFill:  LateFillGuardConfig{MaxDelaySeconds: 30},
					},
					"high_vol": {
						Fatigue:   FatigueGuardConfig{Threshold24h: 10.0, RSI4h: 65},
						Freshness: FreshnessGuardConfig{MaxBarsAge: 2, ATRFactor: 1.0},
						LateFill:  LateFillGuardConfig{MaxDelaySeconds: 30},
					},
				},
			},
		},
	}
}

// GetConfigPath returns the default path for guards configuration
func GetGuardsConfigPath() string {
	return filepath.Join("config", "guards.yaml")
}
