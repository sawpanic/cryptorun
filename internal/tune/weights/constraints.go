package weights

import (
	"fmt"
	"math"
)

// RegimeWeights represents the weight allocation for a specific regime
type RegimeWeights struct {
	MomentumCore      float64 `yaml:"momentum_core"`      // Protected factor (40-50%)
	TechnicalResidual float64 `yaml:"technical_residual"` // Technical indicators (18-25%)
	VolumeResidual    float64 `yaml:"volume_residual"`    // Volume component of S/D block
	QualityResidual   float64 `yaml:"quality_residual"`   // Quality component of S/D block
	// Social is handled separately with hard +10 cap, not part of regime weights
}

// RegimeConstraints defines the valid bounds for each regime's weight allocation
type RegimeConstraints struct {
	Regime             string     `yaml:"regime"`
	MomentumBounds     [2]float64 `yaml:"momentum_bounds"`      // [min, max] for momentum_core
	TechnicalBounds    [2]float64 `yaml:"technical_bounds"`     // [min, max] for technical_residual
	SupplyDemandBounds [2]float64 `yaml:"supply_demand_bounds"` // [min, max] for combined volume+quality
	CatalystBounds     [2]float64 `yaml:"catalyst_bounds"`      // [min, max] for catalyst factors
	QualityMinimum     float64    `yaml:"quality_minimum"`      // Minimum quality allocation
}

// ConstraintSystem manages regime weight constraints and validation
type ConstraintSystem struct {
	constraints map[string]RegimeConstraints
}

// NewConstraintSystem creates a new constraint system with default bounds
func NewConstraintSystem() *ConstraintSystem {
	return &ConstraintSystem{
		constraints: getDefaultConstraints(),
	}
}

// getDefaultConstraints returns the default constraint bounds per regime
func getDefaultConstraints() map[string]RegimeConstraints {
	return map[string]RegimeConstraints{
		"calm": {
			Regime:             "calm",
			MomentumBounds:     [2]float64{0.40, 0.50}, // 40-50%
			TechnicalBounds:    [2]float64{0.18, 0.25}, // 18-25%
			SupplyDemandBounds: [2]float64{0.25, 0.35}, // 25-35% combined
			CatalystBounds:     [2]float64{0.05, 0.12}, // 5-12% (if implemented)
			QualityMinimum:     0.08,                   // Minimum 8% for quality
		},
		"normal": {
			Regime:             "normal",
			MomentumBounds:     [2]float64{0.40, 0.45}, // 40-45%
			TechnicalBounds:    [2]float64{0.18, 0.22}, // 18-22%
			SupplyDemandBounds: [2]float64{0.20, 0.22}, // 20-22% combined
			CatalystBounds:     [2]float64{0.08, 0.12}, // 8-12%
			QualityMinimum:     0.08,                   // Minimum 8% for quality
		},
		"volatile": {
			Regime:             "volatile",
			MomentumBounds:     [2]float64{0.42, 0.48}, // 42-48%
			TechnicalBounds:    [2]float64{0.15, 0.22}, // 15-22% (reduced for noise)
			SupplyDemandBounds: [2]float64{0.22, 0.28}, // 22-28% (higher volume importance)
			CatalystBounds:     [2]float64{0.08, 0.15}, // 8-15%
			QualityMinimum:     0.06,                   // Minimum 6% for quality
		},
	}
}

// ValidateWeights checks if weights satisfy all constraints for a regime
func (cs *ConstraintSystem) ValidateWeights(regime string, weights RegimeWeights) error {
	constraint, exists := cs.constraints[regime]
	if !exists {
		return fmt.Errorf("no constraints defined for regime: %s", regime)
	}

	// Check momentum bounds
	if weights.MomentumCore < constraint.MomentumBounds[0] || weights.MomentumCore > constraint.MomentumBounds[1] {
		return fmt.Errorf("momentum_core %.3f outside bounds [%.3f, %.3f] for regime %s",
			weights.MomentumCore, constraint.MomentumBounds[0], constraint.MomentumBounds[1], regime)
	}

	// Check technical bounds
	if weights.TechnicalResidual < constraint.TechnicalBounds[0] || weights.TechnicalResidual > constraint.TechnicalBounds[1] {
		return fmt.Errorf("technical_residual %.3f outside bounds [%.3f, %.3f] for regime %s",
			weights.TechnicalResidual, constraint.TechnicalBounds[0], constraint.TechnicalBounds[1], regime)
	}

	// Check supply/demand block bounds (volume + quality combined)
	supplyDemandTotal := weights.VolumeResidual + weights.QualityResidual
	if supplyDemandTotal < constraint.SupplyDemandBounds[0] || supplyDemandTotal > constraint.SupplyDemandBounds[1] {
		return fmt.Errorf("supply_demand total %.3f outside bounds [%.3f, %.3f] for regime %s",
			supplyDemandTotal, constraint.SupplyDemandBounds[0], constraint.SupplyDemandBounds[1], regime)
	}

	// Check quality minimum
	if weights.QualityResidual < constraint.QualityMinimum {
		return fmt.Errorf("quality_residual %.3f below minimum %.3f for regime %s",
			weights.QualityResidual, constraint.QualityMinimum, regime)
	}

	// Check sum-to-1 constraint (excluding social)
	total := weights.MomentumCore + weights.TechnicalResidual + weights.VolumeResidual + weights.QualityResidual
	if math.Abs(total-1.0) > 0.001 {
		return fmt.Errorf("weights sum to %.6f, must equal 1.000 (±0.001) for regime %s", total, regime)
	}

	return nil
}

// ClampWeights enforces constraints by clamping weights to valid bounds
func (cs *ConstraintSystem) ClampWeights(regime string, weights RegimeWeights) (RegimeWeights, error) {
	constraint, exists := cs.constraints[regime]
	if !exists {
		return weights, fmt.Errorf("no constraints defined for regime: %s", regime)
	}

	clamped := weights

	// Clamp momentum to bounds
	clamped.MomentumCore = clamp(weights.MomentumCore, constraint.MomentumBounds[0], constraint.MomentumBounds[1])

	// Clamp technical to bounds
	clamped.TechnicalResidual = clamp(weights.TechnicalResidual, constraint.TechnicalBounds[0], constraint.TechnicalBounds[1])

	// Handle supply/demand block constraint
	supplyDemandTotal := weights.VolumeResidual + weights.QualityResidual
	supplyDemandClamped := clamp(supplyDemandTotal, constraint.SupplyDemandBounds[0], constraint.SupplyDemandBounds[1])

	// Redistribute within supply/demand block while respecting quality minimum
	if supplyDemandTotal > 0 {
		qualityRatio := weights.QualityResidual / supplyDemandTotal
		volumeRatio := weights.VolumeResidual / supplyDemandTotal

		// Ensure quality meets minimum
		clamped.QualityResidual = math.Max(constraint.QualityMinimum, supplyDemandClamped*qualityRatio)
		clamped.VolumeResidual = supplyDemandClamped - clamped.QualityResidual
		_ = volumeRatio // Mark as used to avoid compiler error

		// If volume becomes negative, adjust
		if clamped.VolumeResidual < 0 {
			clamped.QualityResidual = supplyDemandClamped
			clamped.VolumeResidual = 0
		}
	} else {
		// Handle edge case where both are zero
		clamped.QualityResidual = constraint.QualityMinimum
		clamped.VolumeResidual = math.Max(0, supplyDemandClamped-constraint.QualityMinimum)
	}

	// Renormalize to sum to 1.0 while respecting bounds
	total := clamped.MomentumCore + clamped.TechnicalResidual + clamped.VolumeResidual + clamped.QualityResidual
	if total > 0 && math.Abs(total-1.0) > 0.001 {
		// Proportionally adjust all weights to sum to 1.0
		factor := 1.0 / total
		clamped.MomentumCore *= factor
		clamped.TechnicalResidual *= factor
		clamped.VolumeResidual *= factor
		clamped.QualityResidual *= factor

		// Re-clamp after normalization if needed
		if clamped.MomentumCore > constraint.MomentumBounds[1] || clamped.MomentumCore < constraint.MomentumBounds[0] {
			clamped.MomentumCore = clamp(clamped.MomentumCore, constraint.MomentumBounds[0], constraint.MomentumBounds[1])
		}
		if clamped.TechnicalResidual > constraint.TechnicalBounds[1] || clamped.TechnicalResidual < constraint.TechnicalBounds[0] {
			clamped.TechnicalResidual = clamp(clamped.TechnicalResidual, constraint.TechnicalBounds[0], constraint.TechnicalBounds[1])
		}

		// Ensure final sum is approximately 1.0
		finalTotal := clamped.MomentumCore + clamped.TechnicalResidual + clamped.VolumeResidual + clamped.QualityResidual
		if math.Abs(finalTotal-1.0) > 0.001 {
			// Adjust quality as the most flexible component
			diff := 1.0 - finalTotal
			clamped.QualityResidual += diff
			if clamped.QualityResidual < constraint.QualityMinimum {
				clamped.QualityResidual = constraint.QualityMinimum
				clamped.VolumeResidual = 1.0 - clamped.MomentumCore - clamped.TechnicalResidual - clamped.QualityResidual
			}
		}
	}

	// Final validation after clamping
	if err := cs.ValidateWeights(regime, clamped); err != nil {
		return clamped, fmt.Errorf("weights still invalid after clamping: %w", err)
	}

	return clamped, nil
}

// GetConstraints returns the constraints for a specific regime
func (cs *ConstraintSystem) GetConstraints(regime string) (RegimeConstraints, error) {
	constraint, exists := cs.constraints[regime]
	if !exists {
		return RegimeConstraints{}, fmt.Errorf("no constraints defined for regime: %s", regime)
	}
	return constraint, nil
}

// GetAllRegimes returns all available regime names
func (cs *ConstraintSystem) GetAllRegimes() []string {
	regimes := make([]string, 0, len(cs.constraints))
	for regime := range cs.constraints {
		regimes = append(regimes, regime)
	}
	return regimes
}

// AddConstraints adds or updates constraints for a regime
func (cs *ConstraintSystem) AddConstraints(regime string, constraints RegimeConstraints) {
	cs.constraints[regime] = constraints
}

// CalculateSlack returns how much "slack" each weight has within its bounds
func (cs *ConstraintSystem) CalculateSlack(regime string, weights RegimeWeights) (map[string]float64, error) {
	constraint, exists := cs.constraints[regime]
	if !exists {
		return nil, fmt.Errorf("no constraints defined for regime: %s", regime)
	}

	slack := make(map[string]float64)

	// Momentum slack (minimum distance to bounds)
	slack["momentum_core"] = math.Min(
		weights.MomentumCore-constraint.MomentumBounds[0],
		constraint.MomentumBounds[1]-weights.MomentumCore,
	)

	// Technical slack
	slack["technical_residual"] = math.Min(
		weights.TechnicalResidual-constraint.TechnicalBounds[0],
		constraint.TechnicalBounds[1]-weights.TechnicalResidual,
	)

	// Supply/demand slack
	supplyDemandTotal := weights.VolumeResidual + weights.QualityResidual
	slack["supply_demand_total"] = math.Min(
		supplyDemandTotal-constraint.SupplyDemandBounds[0],
		constraint.SupplyDemandBounds[1]-supplyDemandTotal,
	)

	// Quality slack (distance from minimum)
	slack["quality_minimum"] = weights.QualityResidual - constraint.QualityMinimum

	return slack, nil
}

// GenerateRandomValidWeights creates random valid weights within constraints
func (cs *ConstraintSystem) GenerateRandomValidWeights(regime string, rng *RandGen) (RegimeWeights, error) {
	constraint, exists := cs.constraints[regime]
	if !exists {
		return RegimeWeights{}, fmt.Errorf("no constraints defined for regime: %s", regime)
	}

	maxAttempts := 100
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Generate random weights within individual bounds
		momentum := constraint.MomentumBounds[0] + rng.Float64()*(constraint.MomentumBounds[1]-constraint.MomentumBounds[0])
		technical := constraint.TechnicalBounds[0] + rng.Float64()*(constraint.TechnicalBounds[1]-constraint.TechnicalBounds[0])

		// Remaining weight for supply/demand block
		remaining := 1.0 - momentum - technical
		if remaining < constraint.SupplyDemandBounds[0] || remaining > constraint.SupplyDemandBounds[1] {
			continue
		}

		// Split supply/demand between volume and quality, respecting quality minimum
		qualityWeight := constraint.QualityMinimum + rng.Float64()*(remaining-constraint.QualityMinimum)
		volumeWeight := remaining - qualityWeight

		if volumeWeight < 0 {
			continue
		}

		weights := RegimeWeights{
			MomentumCore:      momentum,
			TechnicalResidual: technical,
			VolumeResidual:    volumeWeight,
			QualityResidual:   qualityWeight,
		}

		// Validate the generated weights
		if err := cs.ValidateWeights(regime, weights); err == nil {
			return weights, nil
		}
	}

	return RegimeWeights{}, fmt.Errorf("failed to generate valid weights after %d attempts", maxAttempts)
}

// clamp restricts a value to be within [min, max]
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// RandGen is a simple deterministic random number generator for testing
type RandGen struct {
	state uint64
}

// NewRandGen creates a new random generator with a seed
func NewRandGen(seed uint64) *RandGen {
	return &RandGen{state: seed}
}

// Float64 returns a pseudo-random float64 in [0.0, 1.0)
func (r *RandGen) Float64() float64 {
	// Simple linear congruential generator
	r.state = r.state*1103515245 + 12345
	return float64(r.state&0x7FFFFFFF) / float64(0x80000000)
}
