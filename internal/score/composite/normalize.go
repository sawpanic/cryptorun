package composite

import (
	"fmt"
	"math"
)

// Normalizer applies regime weights and normalizes to 100 (ex-social)
type Normalizer struct {
	regimeWeights map[string]map[string]float64
}

// NewNormalizer creates a new normalizer with default weights
func NewNormalizer() *Normalizer {
	return &Normalizer{
		regimeWeights: getDefaultRegimeWeights(),
	}
}

// NormalizedFactors represents factors after weight application and normalization
type NormalizedFactors struct {
	MomentumCore   float64 `json:"momentum_core"`
	TechnicalResid float64 `json:"technical_resid"`
	VolumeResid    float64 `json:"volume_resid"`
	QualityResid   float64 `json:"quality_resid"`
}

// Apply applies regime weights and normalizes to 100 (ex-social)
func (n *Normalizer) Apply(orthogonalized *OrthogonalizedFactors, weights map[string]float64) (*NormalizedFactors, error) {
	// Validate weights sum to approximately 1.0
	if err := n.validateWeights(weights); err != nil {
		return nil, fmt.Errorf("invalid weights: %w", err)
	}

	// Extract base weights (before supply/demand split)
	momentumWeight := weights["momentum_core"]
	technicalWeight := weights["technical_resid"]
	supplyDemandWeight := weights["supply_demand_block"]

	// Split supply_demand_block between VolumeResid and QualityResid
	volumeWeight := 0.55 * supplyDemandWeight  // 55% of supply/demand to volume
	qualityWeight := 0.45 * supplyDemandWeight // 45% of supply/demand to quality

	// Apply weights with bounds checking
	weighted := &NormalizedFactors{
		MomentumCore:   n.applyWeight(n.extractScalar(orthogonalized.MomentumCore), momentumWeight),
		TechnicalResid: n.applyWeight(n.extractScalar(orthogonalized.TechnicalResid), technicalWeight),
		VolumeResid:    n.applyWeight(n.extractScalar(orthogonalized.VolumeResid), volumeWeight),
		QualityResid:   n.applyWeight(n.extractScalar(orthogonalized.QualityResid), qualityWeight),
	}

	// Validate final scores are reasonable
	if err := n.validateNormalized(weighted); err != nil {
		return nil, fmt.Errorf("normalization validation failed: %w", err)
	}

	return weighted, nil
}

// applyWeight applies a weight to a factor score with bounds checking
func (n *Normalizer) applyWeight(factorScore, weight float64) float64 {
	weighted := factorScore * weight

	// Apply reasonable bounds (individual factors shouldn't exceed 60 points)
	return math.Max(0, math.Min(60, weighted))
}

// extractScalar extracts a single scalar from a factor
func (n *Normalizer) extractScalar(factor Factor) float64 {
	if len(factor.Values) == 0 {
		return 0.0
	}

	if len(factor.Values) == 1 {
		return factor.Values[0]
	}

	// Average multiple values
	sum := 0.0
	for _, v := range factor.Values {
		sum += v
	}
	return sum / float64(len(factor.Values))
}

// GetRegimeWeights returns the weight configuration for a regime
func (n *Normalizer) GetRegimeWeights(regime string) (map[string]float64, error) {
	weights, exists := n.regimeWeights[regime]
	if !exists {
		return nil, fmt.Errorf("unknown regime: %s", regime)
	}

	// Return a copy to prevent external modification
	result := make(map[string]float64)
	for k, v := range weights {
		result[k] = v
	}

	return result, nil
}

// LoadRegimeWeights loads weights from configuration
func (n *Normalizer) LoadRegimeWeights(configWeights map[string]map[string]float64) error {
	// Validate all regime configurations
	for regime, weights := range configWeights {
		if err := n.validateWeights(weights); err != nil {
			return fmt.Errorf("invalid weights for regime %s: %w", regime, err)
		}
	}

	n.regimeWeights = configWeights
	return nil
}

// validateWeights ensures weights are valid and sum to approximately 1.0
func (n *Normalizer) validateWeights(weights map[string]float64) error {
	requiredKeys := []string{"momentum_core", "technical_resid", "supply_demand_block", "catalyst_block"}

	// Check required keys exist
	for _, key := range requiredKeys {
		if _, exists := weights[key]; !exists {
			return fmt.Errorf("missing required weight: %s", key)
		}

		if weights[key] < 0 {
			return fmt.Errorf("negative weight for %s: %f", key, weights[key])
		}
	}

	// Check sum is approximately 1.0 (allowing for floating point precision)
	sum := weights["momentum_core"] + weights["technical_resid"] + weights["supply_demand_block"] + weights["catalyst_block"]

	if math.Abs(sum-1.0) > 0.01 {
		return fmt.Errorf("weights sum to %f, expected ~1.0", sum)
	}

	return nil
}

// validateNormalized ensures normalized factors are reasonable
func (n *Normalizer) validateNormalized(factors *NormalizedFactors) error {
	values := []struct {
		name  string
		value float64
	}{
		{"momentum_core", factors.MomentumCore},
		{"technical_resid", factors.TechnicalResid},
		{"volume_resid", factors.VolumeResid},
		{"quality_resid", factors.QualityResid},
	}

	total := 0.0
	for _, v := range values {
		if math.IsNaN(v.value) || math.IsInf(v.value, 0) {
			return fmt.Errorf("invalid value for %s: %f", v.name, v.value)
		}

		if v.value < -10 || v.value > 70 {
			return fmt.Errorf("extreme value for %s: %f", v.name, v.value)
		}

		total += v.value
	}

	// Total should be roughly 0-120 range after weighting
	if total < -20 || total > 120 {
		return fmt.Errorf("total normalized score %f outside reasonable bounds", total)
	}

	return nil
}

// getDefaultRegimeWeights provides default weight configurations
// Updated per PROMPT_ID=SCORING+REGIME+GATES+MENU.V1 requirements
func getDefaultRegimeWeights() map[string]map[string]float64 {
	return map[string]map[string]float64{
		// Trending_Bull: Momentum 40-45%, Technical 18-22%, Supply/Demand 25-30%
		"trending_bull": {
			"momentum_core":       0.42,
			"technical_resid":     0.20,
			"supply_demand_block": 0.28, // Split: 55% volume, 45% quality
			"catalyst_block":      0.10, // Catalyst 12-15%
		},
		// Choppy: Momentum 25-30%, Technical 22-28%, Supply/Demand 30-35%
		"choppy": {
			"momentum_core":       0.27,
			"technical_resid":     0.25,
			"supply_demand_block": 0.33,
			"catalyst_block":      0.15, // Catalyst 18-22%
		},
		// High_Vol: Momentum 28-35%, Technical 20-25%, Supply/Demand 30-40%
		"high_vol": {
			"momentum_core":       0.32,
			"technical_resid":     0.22,
			"supply_demand_block": 0.35, // Quality gets 30-35% focus in high vol
			"catalyst_block":      0.11,
		},
		// Legacy regimes for backward compatibility
		"calm": {
			"momentum_core":       0.27, // Map to choppy
			"technical_resid":     0.25,
			"supply_demand_block": 0.33,
			"catalyst_block":      0.15,
		},
		"normal": {
			"momentum_core":       0.42, // Map to trending_bull
			"technical_resid":     0.20,
			"supply_demand_block": 0.28,
			"catalyst_block":      0.10,
		},
		"volatile": {
			"momentum_core":       0.32, // Map to high_vol
			"technical_resid":     0.22,
			"supply_demand_block": 0.35,
			"catalyst_block":      0.11,
		},
	}
}

// GetWeightSummary returns a human-readable summary of current weights
func (n *Normalizer) GetWeightSummary(regime string) (string, error) {
	weights, err := n.GetRegimeWeights(regime)
	if err != nil {
		return "", err
	}

	// Calculate derived weights
	supplyDemand := weights["supply_demand_block"]
	volumeWeight := 0.55 * supplyDemand
	qualityWeight := 0.45 * supplyDemand

	summary := fmt.Sprintf("Regime: %s\n", regime)
	summary += fmt.Sprintf("  MomentumCore: %.1f%%\n", weights["momentum_core"]*100)
	summary += fmt.Sprintf("  TechnicalResid: %.1f%%\n", weights["technical_resid"]*100)
	summary += fmt.Sprintf("  VolumeResid: %.1f%% (%.1f%% × 55%%)\n", volumeWeight*100, supplyDemand*100)
	summary += fmt.Sprintf("  QualityResid: %.1f%% (%.1f%% × 45%%)\n", qualityWeight*100, supplyDemand*100)
	summary += fmt.Sprintf("  CatalystResid: %.1f%%\n", weights["catalyst_block"]*100)
	summary += fmt.Sprintf("  SocialResid: +10 max (outside 100%%)\n")

	return summary, nil
}
