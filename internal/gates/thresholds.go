package gates

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RegimeThresholds holds microstructure thresholds for a specific regime
type RegimeThresholds struct {
	SpreadMaxBps  float64 `yaml:"spread_max_bps"`  // Maximum spread in basis points
	DepthMinUSD   float64 `yaml:"depth_min_usd"`   // Minimum depth in USD within ±2%
	VADRMin       float64 `yaml:"vadr_min"`        // Minimum VADR multiplier
}

// UniversalThresholds holds constants that apply across all regimes
type UniversalThresholds struct {
	DepthRangePct      float64 `yaml:"depth_range_pct"`       // ±% range for depth calculation (2.0)
	FreshnessMaxBars   int     `yaml:"freshness_max_bars"`    // Maximum bars from trigger (2)
	LateFillMaxSeconds int     `yaml:"late_fill_max_seconds"` // Maximum seconds after bar close (30)
	MinDailyVolumeUSD  float64 `yaml:"min_daily_volume_usd"`  // Minimum daily volume in USD (500k)
}

// RegimeThresholdConfig contains all regime-specific thresholds
type RegimeThresholdConfig struct {
	Default   RegimeThresholds    `yaml:"default"`
	Trending  RegimeThresholds    `yaml:"trending"`
	Choppy    RegimeThresholds    `yaml:"choppy"`
	HighVol   RegimeThresholds    `yaml:"high_vol"`
	RiskOff   RegimeThresholds    `yaml:"risk_off"`
	Universal UniversalThresholds `yaml:"universal"`
}

// ThresholdRouter selects appropriate thresholds based on regime
type ThresholdRouter struct {
	config *RegimeThresholdConfig
}

// NewThresholdRouter creates a router with loaded configuration
func NewThresholdRouter(configPath string) (*ThresholdRouter, error) {
	if configPath == "" {
		configPath = "config/gates/regime_thresholds.yaml"
	}

	config, err := LoadRegimeThresholds(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load regime thresholds: %w", err)
	}

	return &ThresholdRouter{
		config: config,
	}, nil
}

// NewThresholdRouterWithDefaults creates a router with built-in defaults (testing/fallback)
func NewThresholdRouterWithDefaults() *ThresholdRouter {
	config := &RegimeThresholdConfig{
		Default: RegimeThresholds{
			SpreadMaxBps: 50.0,
			DepthMinUSD:  100000.0,
			VADRMin:      1.75,
		},
		Trending: RegimeThresholds{
			SpreadMaxBps: 55.0,
			DepthMinUSD:  100000.0,
			VADRMin:      1.6,
		},
		Choppy: RegimeThresholds{
			SpreadMaxBps: 40.0,
			DepthMinUSD:  150000.0,
			VADRMin:      2.0,
		},
		HighVol: RegimeThresholds{
			SpreadMaxBps: 35.0,
			DepthMinUSD:  175000.0,
			VADRMin:      2.1,
		},
		RiskOff: RegimeThresholds{
			SpreadMaxBps: 30.0,
			DepthMinUSD:  200000.0,
			VADRMin:      2.2,
		},
		Universal: UniversalThresholds{
			DepthRangePct:      2.0,
			FreshnessMaxBars:   2,
			LateFillMaxSeconds: 30,
			MinDailyVolumeUSD:  500000.0,
		},
	}

	return &ThresholdRouter{
		config: config,
	}
}

// SelectThresholds returns the appropriate thresholds for the given regime
func (tr *ThresholdRouter) SelectThresholds(regime string) RegimeThresholds {
	switch regime {
	case "trending", "trending_bull", "bull":
		return tr.config.Trending
	case "choppy", "chop", "sideways":
		return tr.config.Choppy
	case "high_vol", "volatile", "high_volatility":
		return tr.config.HighVol
	case "risk_off", "bear", "crisis":
		return tr.config.RiskOff
	default:
		// Unknown regime - use conservative defaults
		return tr.config.Default
	}
}

// GetUniversalThresholds returns thresholds that apply to all regimes
func (tr *ThresholdRouter) GetUniversalThresholds() UniversalThresholds {
	return tr.config.Universal
}

// GetAllRegimeThresholds returns a map of all regime thresholds for inspection
func (tr *ThresholdRouter) GetAllRegimeThresholds() map[string]RegimeThresholds {
	return map[string]RegimeThresholds{
		"default":   tr.config.Default,
		"trending":  tr.config.Trending,
		"choppy":    tr.config.Choppy,
		"high_vol":  tr.config.HighVol,
		"risk_off":  tr.config.RiskOff,
	}
}

// LoadRegimeThresholds loads thresholds from YAML file
func LoadRegimeThresholds(configPath string) (*RegimeThresholdConfig, error) {
	// Try absolute path first, then relative to working directory
	var data []byte
	var err error
	
	if filepath.IsAbs(configPath) {
		data, err = os.ReadFile(configPath)
	} else {
		// Try current directory first
		data, err = os.ReadFile(configPath)
		if err != nil {
			// Try from project root
			rootPath := filepath.Join("../../..", configPath)
			data, err = os.ReadFile(rootPath)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config RegimeThresholdConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Validate configuration
	if err := validateThresholdConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid threshold configuration: %w", err)
	}

	return &config, nil
}

// validateThresholdConfig ensures all thresholds are reasonable
func validateThresholdConfig(config *RegimeThresholdConfig) error {
	regimes := map[string]RegimeThresholds{
		"default":  config.Default,
		"trending": config.Trending,
		"choppy":   config.Choppy,
		"high_vol": config.HighVol,
		"risk_off": config.RiskOff,
	}

	for name, thresholds := range regimes {
		if thresholds.SpreadMaxBps <= 0 || thresholds.SpreadMaxBps > 500 {
			return fmt.Errorf("invalid spread threshold for %s: %.2f bps (must be 0-500)", name, thresholds.SpreadMaxBps)
		}
		if thresholds.DepthMinUSD <= 0 || thresholds.DepthMinUSD > 10_000_000 {
			return fmt.Errorf("invalid depth threshold for %s: $%.0f (must be $0-$10M)", name, thresholds.DepthMinUSD)
		}
		if thresholds.VADRMin <= 0 || thresholds.VADRMin > 10 {
			return fmt.Errorf("invalid VADR threshold for %s: %.2f (must be 0-10)", name, thresholds.VADRMin)
		}
	}

	// Validate universal thresholds
	if config.Universal.DepthRangePct <= 0 || config.Universal.DepthRangePct > 10 {
		return fmt.Errorf("invalid depth range: %.2f%% (must be 0-10%%)", config.Universal.DepthRangePct)
	}
	if config.Universal.FreshnessMaxBars < 1 || config.Universal.FreshnessMaxBars > 10 {
		return fmt.Errorf("invalid freshness bars: %d (must be 1-10)", config.Universal.FreshnessMaxBars)
	}

	return nil
}

// DescribeThresholds returns a human-readable description of thresholds for a regime
func (tr *ThresholdRouter) DescribeThresholds(regime string) string {
	thresholds := tr.SelectThresholds(regime)
	universal := tr.GetUniversalThresholds()

	return fmt.Sprintf("Regime: %s | Spread: ≤%.1f bps | Depth: ≥$%.0fk (±%.1f%%) | VADR: ≥%.2fx | Freshness: ≤%d bars | Late-fill: ≤%ds",
		regime,
		thresholds.SpreadMaxBps,
		thresholds.DepthMinUSD/1000,
		universal.DepthRangePct,
		thresholds.VADRMin,
		universal.FreshnessMaxBars,
		universal.LateFillMaxSeconds)
}