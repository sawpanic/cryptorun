package regime

import (
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// WeightsConfig represents the regime weights configuration
type WeightsConfig struct {
	Regimes    map[string]RegimeWeights `yaml:"regimes"`
	Social     SocialConfig             `yaml:"social"`
	Validation ValidationConfig         `yaml:"validation"`
}

// RegimeWeights defines the weight allocation for a market regime
type RegimeWeights struct {
	MomentumCore      float64 `yaml:"momentum_core"`
	TechnicalResid    float64 `yaml:"technical_resid"`
	SupplyDemandBlock float64 `yaml:"supply_demand_block"`
	CatalystBlock     float64 `yaml:"catalyst_block"`
}

// SocialConfig defines social factor configuration
type SocialConfig struct {
	MaxContribution           float64 `yaml:"max_contribution"`
	AppliedAfterNormalization bool    `yaml:"applied_after_normalization"`
}

// ValidationConfig defines validation parameters
type ValidationConfig struct {
	WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
	MinWeight          float64 `yaml:"min_weight"`
	MaxWeight          float64 `yaml:"max_weight"`
}

// WeightsLoader handles loading and validation of regime weights
type WeightsLoader struct {
	config *WeightsConfig
}

// NewWeightsLoader creates a new weights loader
func NewWeightsLoader() *WeightsLoader {
	return &WeightsLoader{}
}

// LoadFromFile loads regime weights from a YAML configuration file
func (wl *WeightsLoader) LoadFromFile(configPath string) error {
	// Read the configuration file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Parse YAML
	var config WeightsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Validate configuration
	if err := wl.validateConfig(&config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	wl.config = &config
	return nil
}

// LoadDefault loads default weights configuration
func (wl *WeightsLoader) LoadDefault() error {
	config := &WeightsConfig{
		Regimes: map[string]RegimeWeights{
			"calm": {
				MomentumCore:      0.40,
				TechnicalResid:    0.20,
				SupplyDemandBlock: 0.30,
				CatalystBlock:     0.10,
			},
			"normal": {
				MomentumCore:      0.43,
				TechnicalResid:    0.20,
				SupplyDemandBlock: 0.27,
				CatalystBlock:     0.10,
			},
			"volatile": {
				MomentumCore:      0.45,
				TechnicalResid:    0.18,
				SupplyDemandBlock: 0.25,
				CatalystBlock:     0.12,
			},
		},
		Social: SocialConfig{
			MaxContribution:           10.0,
			AppliedAfterNormalization: true,
		},
		Validation: ValidationConfig{
			WeightSumTolerance: 0.01,
			MinWeight:          0.05,
			MaxWeight:          0.60,
		},
	}

	if err := wl.validateConfig(config); err != nil {
		return fmt.Errorf("default config validation failed: %w", err)
	}

	wl.config = config
	return nil
}

// GetWeights returns the weight map for a specific regime
func (wl *WeightsLoader) GetWeights(regime string) (map[string]float64, error) {
	if wl.config == nil {
		return nil, fmt.Errorf("weights not loaded - call LoadFromFile or LoadDefault first")
	}

	regimeWeights, exists := wl.config.Regimes[regime]
	if !exists {
		return nil, fmt.Errorf("unknown regime: %s", regime)
	}

	// Convert to standard map format expected by normalizer
	return map[string]float64{
		"momentum_core":       regimeWeights.MomentumCore,
		"technical_resid":     regimeWeights.TechnicalResid,
		"supply_demand_block": regimeWeights.SupplyDemandBlock,
		"catalyst_block":      regimeWeights.CatalystBlock,
	}, nil
}

// GetSocialConfig returns the social factor configuration
func (wl *WeightsLoader) GetSocialConfig() (*SocialConfig, error) {
	if wl.config == nil {
		return nil, fmt.Errorf("config not loaded")
	}

	return &wl.config.Social, nil
}

// GetAvailableRegimes returns the list of configured regimes
func (wl *WeightsLoader) GetAvailableRegimes() []string {
	if wl.config == nil {
		return nil
	}

	regimes := make([]string, 0, len(wl.config.Regimes))
	for regime := range wl.config.Regimes {
		regimes = append(regimes, regime)
	}

	return regimes
}

// GetWeightsSummary returns a formatted summary of all regime weights
func (wl *WeightsLoader) GetWeightsSummary() (string, error) {
	if wl.config == nil {
		return "", fmt.Errorf("config not loaded")
	}

	summary := "Regime Weight Configuration:\n\n"

	for regime, weights := range wl.config.Regimes {
		summary += fmt.Sprintf("%s regime:\n", regime)
		summary += fmt.Sprintf("  Momentum Core: %.1f%%\n", weights.MomentumCore*100)
		summary += fmt.Sprintf("  Technical Residual: %.1f%%\n", weights.TechnicalResid*100)

		// Show supply/demand split
		volumeWeight := 0.55 * weights.SupplyDemandBlock
		qualityWeight := 0.45 * weights.SupplyDemandBlock
		summary += fmt.Sprintf("  Volume Residual: %.1f%% (%.1f%% × 55%%)\n", volumeWeight*100, weights.SupplyDemandBlock*100)
		summary += fmt.Sprintf("  Quality Residual: %.1f%% (%.1f%% × 45%%)\n", qualityWeight*100, weights.SupplyDemandBlock*100)
		summary += fmt.Sprintf("  Catalyst Block: %.1f%% (not implemented)\n", weights.CatalystBlock*100)
		summary += "\n"
	}

	summary += fmt.Sprintf("Social Factor: +%.1f max (applied outside 100%%)\n", wl.config.Social.MaxContribution)

	return summary, nil
}

// validateConfig validates the entire weights configuration
func (wl *WeightsLoader) validateConfig(config *WeightsConfig) error {
	// Validate required regimes exist
	requiredRegimes := []string{"calm", "normal", "volatile"}
	for _, regime := range requiredRegimes {
		if _, exists := config.Regimes[regime]; !exists {
			return fmt.Errorf("missing required regime: %s", regime)
		}
	}

	// Validate each regime's weights
	for regime, weights := range config.Regimes {
		if err := wl.validateRegimeWeights(regime, &weights, &config.Validation); err != nil {
			return err
		}
	}

	// Validate social configuration
	if config.Social.MaxContribution < 0 || config.Social.MaxContribution > 50 {
		return fmt.Errorf("social max_contribution %.1f outside reasonable range [0, 50]",
			config.Social.MaxContribution)
	}

	return nil
}

// validateRegimeWeights validates weights for a single regime
func (wl *WeightsLoader) validateRegimeWeights(regime string, weights *RegimeWeights, validation *ValidationConfig) error {
	values := map[string]float64{
		"momentum_core":       weights.MomentumCore,
		"technical_resid":     weights.TechnicalResid,
		"supply_demand_block": weights.SupplyDemandBlock,
		"catalyst_block":      weights.CatalystBlock,
	}

	// Check individual weight bounds
	for name, value := range values {
		if value < 0 {
			return fmt.Errorf("regime %s has negative weight for %s: %.3f", regime, name, value)
		}

		if value < validation.MinWeight {
			return fmt.Errorf("regime %s weight for %s (%.3f) below minimum (%.3f)",
				regime, name, value, validation.MinWeight)
		}

		if value > validation.MaxWeight {
			return fmt.Errorf("regime %s weight for %s (%.3f) above maximum (%.3f)",
				regime, name, value, validation.MaxWeight)
		}
	}

	// Check sum equals 1.0 within tolerance
	sum := weights.MomentumCore + weights.TechnicalResid + weights.SupplyDemandBlock + weights.CatalystBlock

	if math.Abs(sum-1.0) > validation.WeightSumTolerance {
		return fmt.Errorf("regime %s weights sum to %.4f, expected 1.0 ± %.3f",
			regime, sum, validation.WeightSumTolerance)
	}

	return nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	return filepath.Join("config", "regime_weights.yaml")
}
