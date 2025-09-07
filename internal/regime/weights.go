package regime

import (
	"fmt"
)

// WeightPreset defines factor weights for a specific regime
type WeightPreset struct {
	Regime       Regime                 `json:"regime"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Weights      map[string]float64     `json:"weights"` // Factor -> Weight (0.0-1.0)
	MovementGate MovementGateConfig     `json:"movement_gate"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// MovementGateConfig defines movement detection thresholds per regime
type MovementGateConfig struct {
	MinMovementPercent  float64 `json:"min_movement_percent"`  // Minimum % move to trigger
	TimeWindowHours     int     `json:"time_window_hours"`     // Detection window
	VolumeSurgeRequired bool    `json:"volume_surge_required"` // Require volume confirmation
	TightenedThresholds bool    `json:"tightened_thresholds"`  // Higher bars for entry
}

// WeightManager manages regime-based factor weights
type WeightManager struct {
	presets       map[Regime]*WeightPreset
	currentRegime Regime
	lastUpdate    *DetectionResult
}

// NewWeightManager creates a weight manager with default presets
func NewWeightManager() *WeightManager {
	wm := &WeightManager{
		presets:       make(map[Regime]*WeightPreset),
		currentRegime: Choppy, // Safe default
	}

	wm.initializeDefaultPresets()
	return wm
}

// initializeDefaultPresets sets up the regime-specific weight tables
func (wm *WeightManager) initializeDefaultPresets() {
	// Trending Bull: Higher momentum, weekly carry, reduced volatility emphasis
	wm.presets[TrendingBull] = &WeightPreset{
		Regime:      TrendingBull,
		Name:        "Trending Bull",
		Description: "Bull trend with sustained momentum and 7d carry",
		Weights: map[string]float64{
			"momentum_1h":      0.25, // Core momentum
			"momentum_4h":      0.20, // Medium-term
			"momentum_12h":     0.15, // Longer-term
			"momentum_24h":     0.10, // Extended
			"weekly_7d_carry":  0.10, // Trending-only factor
			"volume_surge":     0.08, // Volume confirmation
			"volatility_score": 0.05, // Reduced in trending
			"quality_score":    0.04, // Quality overlay
			"social_sentiment": 0.03, // Social factor (capped)
		},
		MovementGate: MovementGateConfig{
			MinMovementPercent:  3.5, // Lower threshold in trends
			TimeWindowHours:     48,
			VolumeSurgeRequired: false, // Not required in strong trends
			TightenedThresholds: false,
		},
		Metadata: map[string]interface{}{
			"regime_characteristics": []string{"sustained_momentum", "lower_volatility", "directional_bias"},
			"special_factors":        []string{"weekly_7d_carry"},
		},
	}

	// Choppy: Balanced weights, no weekly carry, standard gates
	wm.presets[Choppy] = &WeightPreset{
		Regime:      Choppy,
		Name:        "Choppy",
		Description: "Mixed signals with balanced factor allocation",
		Weights: map[string]float64{
			"momentum_1h":      0.20, // Reduced short-term
			"momentum_4h":      0.18, // Core timeframe
			"momentum_12h":     0.15, // Medium-term
			"momentum_24h":     0.12, // Extended
			"weekly_7d_carry":  0.00, // No weekly carry in chop
			"volume_surge":     0.12, // Higher volume emphasis
			"volatility_score": 0.10, // Volatility important
			"quality_score":    0.08, // Quality matters more
			"social_sentiment": 0.05, // Social factor (capped)
		},
		MovementGate: MovementGateConfig{
			MinMovementPercent:  5.0, // Standard threshold
			TimeWindowHours:     48,
			VolumeSurgeRequired: true, // Require volume in chop
			TightenedThresholds: false,
		},
		Metadata: map[string]interface{}{
			"regime_characteristics": []string{"mixed_signals", "range_bound", "mean_reverting"},
			"disabled_factors":       []string{"weekly_7d_carry"},
		},
	}

	// High Vol: Defensive positioning, tightened gates, quality emphasis
	wm.presets[HighVol] = &WeightPreset{
		Regime:      HighVol,
		Name:        "High Volatility",
		Description: "High volatility regime with tightened movement gates",
		Weights: map[string]float64{
			"momentum_1h":      0.15, // Reduced short-term (noisy)
			"momentum_4h":      0.15, // Reduced medium-term
			"momentum_12h":     0.18, // Favor longer timeframes
			"momentum_24h":     0.15, // Extended view
			"weekly_7d_carry":  0.00, // No weekly carry in volatility
			"volume_surge":     0.08, // Lower volume weight (can be misleading)
			"volatility_score": 0.15, // High volatility awareness
			"quality_score":    0.12, // Quality crucial in volatility
			"social_sentiment": 0.02, // Minimal social (noise)
		},
		MovementGate: MovementGateConfig{
			MinMovementPercent:  7.0,  // Tightened threshold
			TimeWindowHours:     36,   // Shorter window (faster decay)
			VolumeSurgeRequired: true, // Volume required
			TightenedThresholds: true, // Higher bars for entry
		},
		Metadata: map[string]interface{}{
			"regime_characteristics": []string{"high_volatility", "defensive_positioning", "quality_focus"},
			"tightened_gates":        true,
			"risk_adjustments":       []string{"reduced_short_term", "quality_emphasis", "minimal_social"},
		},
	}
}

// GetActiveWeights returns the current regime's factor weights
func (wm *WeightManager) GetActiveWeights() *WeightPreset {
	if preset, exists := wm.presets[wm.currentRegime]; exists {
		return preset
	}
	// Fallback to choppy if current regime not found
	return wm.presets[Choppy]
}

// GetWeightsForRegime returns weights for a specific regime
func (wm *WeightManager) GetWeightsForRegime(regime Regime) (*WeightPreset, error) {
	if preset, exists := wm.presets[regime]; exists {
		return preset, nil
	}
	return nil, fmt.Errorf("no weight preset found for regime: %s", regime.String())
}

// UpdateCurrentRegime updates the active regime based on detection result
func (wm *WeightManager) UpdateCurrentRegime(result *DetectionResult) {
	wm.currentRegime = result.Regime
	wm.lastUpdate = result
}

// GetAllPresets returns all available weight presets
func (wm *WeightManager) GetAllPresets() map[Regime]*WeightPreset {
	return wm.presets
}

// ValidateWeights checks that weights sum to approximately 1.0
func (wm *WeightManager) ValidateWeights(regime Regime) error {
	preset, exists := wm.presets[regime]
	if !exists {
		return fmt.Errorf("regime %s not found", regime.String())
	}

	total := 0.0
	for _, weight := range preset.Weights {
		total += weight
	}

	// Allow for small floating point errors
	if total < 0.95 || total > 1.05 {
		return fmt.Errorf("weights for regime %s sum to %.3f, expected ~1.0", regime.String(), total)
	}

	return nil
}

// GetCurrentRegime returns the currently active regime
func (wm *WeightManager) GetCurrentRegime() Regime {
	return wm.currentRegime
}

// GetLastUpdate returns the most recent detection result
func (wm *WeightManager) GetLastUpdate() *DetectionResult {
	return wm.lastUpdate
}

// GetRegimeTransitionMatrix returns transition statistics
func (wm *WeightManager) GetRegimeTransitionMatrix() map[string]interface{} {
	// This would be populated with historical transition data
	// For now, return structure for future implementation
	return map[string]interface{}{
		"transitions": map[string]map[string]int{
			"trending_bull_to_choppy":   {"count": 0, "avg_duration_hours": 0},
			"trending_bull_to_high_vol": {"count": 0, "avg_duration_hours": 0},
			"choppy_to_trending_bull":   {"count": 0, "avg_duration_hours": 0},
			"choppy_to_high_vol":        {"count": 0, "avg_duration_hours": 0},
			"high_vol_to_trending_bull": {"count": 0, "avg_duration_hours": 0},
			"high_vol_to_choppy":        {"count": 0, "avg_duration_hours": 0},
		},
		"stability_metrics": map[string]interface{}{
			"avg_regime_duration_hours": 0,
			"most_stable_regime":        "choppy",
			"transition_frequency":      0.0,
		},
	}
}

// GetWeightDifferences compares weights between two regimes
func (wm *WeightManager) GetWeightDifferences(from, to Regime) (map[string]float64, error) {
	fromPreset, err := wm.GetWeightsForRegime(from)
	if err != nil {
		return nil, fmt.Errorf("invalid 'from' regime: %w", err)
	}

	toPreset, err := wm.GetWeightsForRegime(to)
	if err != nil {
		return nil, fmt.Errorf("invalid 'to' regime: %w", err)
	}

	differences := make(map[string]float64)

	// Calculate differences for all factors
	allFactors := make(map[string]bool)
	for factor := range fromPreset.Weights {
		allFactors[factor] = true
	}
	for factor := range toPreset.Weights {
		allFactors[factor] = true
	}

	for factor := range allFactors {
		fromWeight := fromPreset.Weights[factor] // 0.0 if not present
		toWeight := toPreset.Weights[factor]     // 0.0 if not present
		differences[factor] = toWeight - fromWeight
	}

	return differences, nil
}
