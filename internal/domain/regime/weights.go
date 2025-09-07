package regime

import (
	"fmt"
	"math"
	"time"
)

// FactorWeights represents the weight allocation for a specific regime
// All weights sum to 100 (excluding Social which is capped separately)
type FactorWeights struct {
	Momentum  float64 `yaml:"momentum" json:"momentum"`   // MomentumCore (protected)
	Technical float64 `yaml:"technical" json:"technical"` // Technical indicators
	Volume    float64 `yaml:"volume" json:"volume"`       // Volume analysis
	Quality   float64 `yaml:"quality" json:"quality"`     // Quality metrics
	Catalyst  float64 `yaml:"catalyst" json:"catalyst"`   // Catalyst factors

	// Social is handled separately - always capped at +10 and applied OUTSIDE the 100% allocation
}

// RegimeWeightMap defines weight allocations for each market regime
type RegimeWeightMap struct {
	TrendingBull FactorWeights `yaml:"trending_bull" json:"trending_bull"`
	Choppy       FactorWeights `yaml:"choppy" json:"choppy"`
	HighVol      FactorWeights `yaml:"high_vol" json:"high_vol"`
}

// GetDefaultWeightMap returns the default regime weight configuration
func GetDefaultWeightMap() RegimeWeightMap {
	return RegimeWeightMap{
		// TRENDING_BULL: High momentum allocation during trending markets
		TrendingBull: FactorWeights{
			Momentum:  50.0, // High momentum weight for trend following
			Technical: 20.0, // Moderate technical indicators
			Volume:    15.0, // Volume confirmation
			Quality:   10.0, // Quality screens
			Catalyst:  5.0,  // Event catalysts
		},

		// CHOPPY: Balanced allocation with focus on technical and quality
		Choppy: FactorWeights{
			Momentum:  35.0, // Reduced momentum in sideways markets
			Technical: 30.0, // Higher technical weight for range trading
			Volume:    15.0, // Volume analysis for breakouts
			Quality:   15.0, // Quality becomes more important
			Catalyst:  5.0,  // Catalyst factors
		},

		// HIGH_VOL: Emphasis on quality and risk management
		HighVol: FactorWeights{
			Momentum:  30.0, // Lower momentum due to noise
			Technical: 25.0, // Technical analysis for volatility
			Volume:    20.0, // Volume spikes matter more
			Quality:   20.0, // Quality screens critical
			Catalyst:  5.0,  // Catalyst factors minimal
		},
	}
}

// WeightResolver resolves factor weights based on current market regime
type WeightResolver struct {
	weightMap RegimeWeightMap
	detector  *RegimeDetector
}

// NewWeightResolver creates a new weight resolver with detector integration
func NewWeightResolver(weightMap RegimeWeightMap, detector *RegimeDetector) *WeightResolver {
	return &WeightResolver{
		weightMap: weightMap,
		detector:  detector,
	}
}

// GetWeights returns the current factor weights based on detected regime
func (wr *WeightResolver) GetWeights() FactorWeights {
	// Use default market data for current regime
	defaultData := MarketData{
		Timestamp:    time.Now(),
		CurrentPrice: 1.0,
		MA20:        1.0,
		RealizedVol7d: 0.2,
		Prices:      []float64{1.0},
	}
	currentRegime, _ := wr.detector.GetCurrentRegime(defaultData)
	return wr.GetWeightsForRegime(currentRegime)
}

// GetWeightsForRegime returns weights for a specific regime
func (wr *WeightResolver) GetWeightsForRegime(regime RegimeType) FactorWeights {
	switch regime {
	case RegimeCalm:
		return wr.weightMap.TrendingBull // Calm markets favor trending approach
	case RegimeNormal:
		return wr.weightMap.Choppy      // Normal markets are mixed
	case RegimeVolatile:
		return wr.weightMap.HighVol     // Volatile markets need special handling
	default:
		// Fallback to choppy as safe default
		return wr.weightMap.Choppy
	}
}

// GetAllWeights returns the complete weight map
func (wr *WeightResolver) GetAllWeights() RegimeWeightMap {
	return wr.weightMap
}

// UpdateWeightMap updates the weight configuration
func (wr *WeightResolver) UpdateWeightMap(newWeightMap RegimeWeightMap) error {
	// Validate all weight allocations
	if err := ValidateWeightMap(newWeightMap); err != nil {
		return fmt.Errorf("invalid weight map: %w", err)
	}

	wr.weightMap = newWeightMap
	return nil
}

// ValidateWeightMap ensures all weight allocations are valid
func ValidateWeightMap(weightMap RegimeWeightMap) error {
	regimes := map[string]FactorWeights{
		"trending_bull": weightMap.TrendingBull,
		"choppy":        weightMap.Choppy,
		"high_vol":      weightMap.HighVol,
	}

	for regime, weights := range regimes {
		if err := ValidateFactorWeights(weights); err != nil {
			return fmt.Errorf("regime %s: %w", regime, err)
		}
	}

	return nil
}

// ValidateFactorWeights ensures factor weights are valid and sum to 100
func ValidateFactorWeights(weights FactorWeights) error {
	// Check individual weight bounds
	if weights.Momentum < 0 || weights.Momentum > 100 {
		return fmt.Errorf("momentum weight out of bounds: %f (expected 0-100)", weights.Momentum)
	}
	if weights.Technical < 0 || weights.Technical > 100 {
		return fmt.Errorf("technical weight out of bounds: %f (expected 0-100)", weights.Technical)
	}
	if weights.Volume < 0 || weights.Volume > 100 {
		return fmt.Errorf("volume weight out of bounds: %f (expected 0-100)", weights.Volume)
	}
	if weights.Quality < 0 || weights.Quality > 100 {
		return fmt.Errorf("quality weight out of bounds: %f (expected 0-100)", weights.Quality)
	}
	if weights.Catalyst < 0 || weights.Catalyst > 100 {
		return fmt.Errorf("catalyst weight out of bounds: %f (expected 0-100)", weights.Catalyst)
	}

	// Check sum equals 100 (within tolerance)
	sum := weights.Momentum + weights.Technical + weights.Volume + weights.Quality + weights.Catalyst
	if math.Abs(sum-100.0) > 0.1 {
		return fmt.Errorf("factor weights do not sum to 100: got %f", sum)
	}

	// Ensure minimum momentum allocation (MomentumCore protection)
	if weights.Momentum < 25.0 {
		return fmt.Errorf("momentum weight too low: %f (minimum 25%% for MomentumCore protection)", weights.Momentum)
	}

	return nil
}

// NormalizeWeights adjusts weights to sum exactly to 100 while preserving ratios
func NormalizeWeights(weights FactorWeights) FactorWeights {
	sum := weights.Momentum + weights.Technical + weights.Volume + weights.Quality + weights.Catalyst

	if sum == 0 {
		// Return default safe weights
		return FactorWeights{
			Momentum:  40.0,
			Technical: 25.0,
			Volume:    15.0,
			Quality:   15.0,
			Catalyst:  5.0,
		}
	}

	// Scale to sum to 100
	factor := 100.0 / sum
	return FactorWeights{
		Momentum:  weights.Momentum * factor,
		Technical: weights.Technical * factor,
		Volume:    weights.Volume * factor,
		Quality:   weights.Quality * factor,
		Catalyst:  weights.Catalyst * factor,
	}
}

// RegimeWeights defines factor weights for a specific market regime (used in composite scoring)
type RegimeWeights struct {
	Description    string  `yaml:"description"`
	MomentumCore   float64 `yaml:"momentum_core"`
	Technical      float64 `yaml:"technical"`
	Volume         float64 `yaml:"volume"`
	Quality        float64 `yaml:"quality"`
	Social         float64 `yaml:"social"`
}

// WeightsConfig defines the unified factor weights configuration
type WeightsConfig struct {
	DefaultRegime string `yaml:"default_regime"`
	Validation    struct {
		WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
		MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
		MaxSocialWeight    float64 `yaml:"max_social_weight"`
		SocialHardCap      float64 `yaml:"social_hard_cap"`
	} `yaml:"validation"`
	Regimes         map[string]RegimeWeights `yaml:"regimes"`
	QARequirements  QARequirements           `yaml:"qa_requirements"`
}

// QARequirements defines quality assurance thresholds
type QARequirements struct {
	CorrelationThreshold float64 `yaml:"correlation_threshold"`
	WeightSumExact      float64 `yaml:"weight_sum_exact"`
	MomentumMinimum     float64 `yaml:"momentum_minimum"`
	SocialMaximum       float64 `yaml:"social_maximum"`
}

// ValidateRegimeWeights checks if the weights configuration is valid
func ValidateRegimeWeights(weights RegimeWeights, config WeightsConfig) error {
	total := weights.MomentumCore + weights.Technical + weights.Volume + weights.Quality + weights.Social
	if math.Abs(total-1.0) > config.Validation.WeightSumTolerance {
		return fmt.Errorf("weights sum to %.6f, expected 1.0 ±%.3f", total, config.Validation.WeightSumTolerance)
	}
	
	if weights.MomentumCore < config.Validation.MinMomentumWeight {
		return fmt.Errorf("momentum weight %.3f below minimum %.3f", weights.MomentumCore, config.Validation.MinMomentumWeight)
	}
	
	if weights.Social > config.Validation.MaxSocialWeight {
		return fmt.Errorf("social weight %.3f above maximum %.3f", weights.Social, config.Validation.MaxSocialWeight)
	}
	
	return nil
}

// ApplySocialCap applies the social media factor cap (+10 max) OUTSIDE the base scoring
// This ensures social factors don't interfere with the core 100-point allocation
func ApplySocialCap(baseScore float64, socialSignal float64) float64 {
	// Social cap is strictly limited to +10 points maximum
	socialBoost := math.Max(-10.0, math.Min(10.0, socialSignal))

	// Apply social boost outside the base 100-point system
	return baseScore + socialBoost
}

// GetMomentumProtectionStatus returns information about MomentumCore protection
func GetMomentumProtectionStatus(weights FactorWeights) map[string]interface{} {
	return map[string]interface{}{
		"momentum_weight":           weights.Momentum,
		"is_protected":              weights.Momentum >= 25.0,
		"min_required":              25.0,
		"protection_margin":         weights.Momentum - 25.0,
		"total_allocation":          weights.Momentum + weights.Technical + weights.Volume + weights.Quality + weights.Catalyst,
		"social_handled_separately": true,
	}
}

// GetWeightAllocationSummary returns a summary of weight allocation for analysis
func GetWeightAllocationSummary(regime RegimeType, weights FactorWeights) map[string]interface{} {
	total := weights.Momentum + weights.Technical + weights.Volume + weights.Quality + weights.Catalyst

	return map[string]interface{}{
		"regime": regime.String(),
		"allocations": map[string]float64{
			"momentum":  weights.Momentum,
			"technical": weights.Technical,
			"volume":    weights.Volume,
			"quality":   weights.Quality,
			"catalyst":  weights.Catalyst,
		},
		"total":              total,
		"is_valid":           math.Abs(total-100.0) <= 0.1,
		"momentum_protected": weights.Momentum >= 25.0,
		"social_cap_note":    "Social factors capped at +10, applied outside base scoring",
	}
}
