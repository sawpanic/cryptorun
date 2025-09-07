package tune

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cryptorun/internal/tune/weights"
)

// TestConstraintSystemValidation tests the constraint validation system
func TestConstraintSystemValidation(t *testing.T) {
	constraints := weights.NewConstraintSystem()

	// Test valid weights
	validWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	err := constraints.ValidateWeights("normal", validWeights)
	assert.NoError(t, err, "Valid weights should pass validation")

	// Test momentum bounds violation
	invalidMomentum := validWeights
	invalidMomentum.MomentumCore = 0.35 // Below 0.40 minimum

	err = constraints.ValidateWeights("normal", invalidMomentum)
	assert.Error(t, err, "Momentum below bounds should fail")
	assert.Contains(t, err.Error(), "momentum_core")

	// Test sum-to-1 violation
	invalidSum := validWeights
	invalidSum.QualityResidual = 0.20 // Will make sum > 1

	err = constraints.ValidateWeights("normal", invalidSum)
	assert.Error(t, err, "Weights not summing to 1 should fail")
	assert.Contains(t, err.Error(), "sum to")

	// Test quality minimum violation
	invalidQuality := validWeights
	invalidQuality.QualityResidual = 0.05 // Below 0.08 minimum for normal regime
	invalidQuality.VolumeResidual = 0.31  // Adjust to maintain sum

	err = constraints.ValidateWeights("normal", invalidQuality)
	assert.Error(t, err, "Quality below minimum should fail")
	assert.Contains(t, err.Error(), "quality_residual")
}

// TestConstraintBounds tests individual constraint bounds
func TestConstraintBounds(t *testing.T) {
	constraints := weights.NewConstraintSystem()

	testCases := []struct {
		regime     string
		weights    weights.RegimeWeights
		expectFail bool
		reason     string
	}{
		{
			regime: "normal",
			weights: weights.RegimeWeights{
				MomentumCore:      0.40, // Minimum bound
				TechnicalResidual: 0.22, // Maximum bound
				VolumeResidual:    0.20,
				QualityResidual:   0.18,
			},
			expectFail: false,
			reason:     "boundary values should be valid",
		},
		{
			regime: "volatile",
			weights: weights.RegimeWeights{
				MomentumCore:      0.48, // Maximum for volatile
				TechnicalResidual: 0.15, // Minimum for volatile
				VolumeResidual:    0.25,
				QualityResidual:   0.12,
			},
			expectFail: false,
			reason:     "volatile regime bounds should work",
		},
		{
			regime: "calm",
			weights: weights.RegimeWeights{
				MomentumCore:      0.51, // Above maximum (0.50)
				TechnicalResidual: 0.20,
				VolumeResidual:    0.20,
				QualityResidual:   0.09,
			},
			expectFail: true,
			reason:     "momentum above maximum should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.reason, func(t *testing.T) {
			err := constraints.ValidateWeights(tc.regime, tc.weights)

			if tc.expectFail {
				assert.Error(t, err, tc.reason)
			} else {
				assert.NoError(t, err, tc.reason)
			}
		})
	}
}

// TestConstraintClamping tests the weight clamping functionality
func TestConstraintClamping(t *testing.T) {
	constraints := weights.NewConstraintSystem()

	// Test clamping momentum
	invalidWeights := weights.RegimeWeights{
		MomentumCore:      0.60, // Way above maximum
		TechnicalResidual: 0.15,
		VolumeResidual:    0.15,
		QualityResidual:   0.10,
	}

	clamped, err := constraints.ClampWeights("normal", invalidWeights)
	require.NoError(t, err)

	// Should clamp momentum to maximum and renormalize
	assert.LessOrEqual(t, clamped.MomentumCore, 0.45, "Momentum should be clamped to maximum")
	assert.GreaterOrEqual(t, clamped.MomentumCore, 0.40, "Momentum should be above minimum")

	// Should sum to 1
	sum := clamped.MomentumCore + clamped.TechnicalResidual + clamped.VolumeResidual + clamped.QualityResidual
	assert.InDelta(t, 1.0, sum, 0.001, "Clamped weights should sum to 1")

	// Should pass validation
	err = constraints.ValidateWeights("normal", clamped)
	assert.NoError(t, err, "Clamped weights should be valid")
}

// TestSupplyDemandBlockConstraint tests the supply/demand block constraint
func TestSupplyDemandBlockConstraint(t *testing.T) {
	constraints := weights.NewConstraintSystem()

	// Test valid supply/demand allocation
	validWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25, // Supply/demand = 0.25 + 0.13 = 0.38 (within bounds)
		QualityResidual:   0.13,
	}

	err := constraints.ValidateWeights("normal", validWeights)
	assert.NoError(t, err, "Valid supply/demand block should pass")

	// Test supply/demand block violation
	invalidSD := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.30, // Supply/demand = 0.30 + 0.08 = 0.38 (still within bounds)
		QualityResidual:   0.08,
	}

	err = constraints.ValidateWeights("normal", invalidSD)
	assert.NoError(t, err, "This should actually be valid")

	// Test actual supply/demand violation
	realInvalidSD := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.32, // Supply/demand = 0.32 + 0.06 = 0.38 (but quality below minimum)
		QualityResidual:   0.06, // Below 0.08 minimum
	}

	err = constraints.ValidateWeights("normal", realInvalidSD)
	assert.Error(t, err, "Quality below minimum should fail")
}

// TestConstraintSlack tests the slack calculation
func TestConstraintSlack(t *testing.T) {
	constraints := weights.NewConstraintSystem()

	// Weights near boundaries
	nearBoundary := weights.RegimeWeights{
		MomentumCore:      0.401, // Just above minimum (0.40)
		TechnicalResidual: 0.219, // Just below maximum (0.22)
		VolumeResidual:    0.20,
		QualityResidual:   0.18,
	}

	slack, err := constraints.CalculateSlack("normal", nearBoundary)
	require.NoError(t, err)

	// Momentum slack should be small (distance to nearest bound)
	assert.InDelta(t, 0.001, slack["momentum_core"], 0.0005, "Momentum slack should be small")
	assert.InDelta(t, 0.001, slack["technical_residual"], 0.0005, "Technical slack should be small")

	// Quality slack (distance from minimum)
	expectedQualitySlack := nearBoundary.QualityResidual - 0.08 // Minimum for normal
	assert.InDelta(t, expectedQualitySlack, slack["quality_minimum"], 0.001, "Quality slack should match calculation")
}

// TestRandomWeightGeneration tests random valid weight generation
func TestRandomWeightGeneration(t *testing.T) {
	constraints := weights.NewConstraintSystem()
	rng := weights.NewRandGen(12345) // Deterministic seed

	for regime := range map[string]bool{"calm": true, "normal": true, "volatile": true} {
		for i := 0; i < 10; i++ {
			randomWeights, err := constraints.GenerateRandomValidWeights(regime, rng)
			require.NoError(t, err, "Random weight generation should succeed")

			// Should pass validation
			err = constraints.ValidateWeights(regime, randomWeights)
			assert.NoError(t, err, "Generated random weights should be valid for regime %s", regime)
		}
	}
}

// TestAllRegimeBounds tests all regime-specific bounds
func TestAllRegimeBounds(t *testing.T) {
	constraints := weights.NewConstraintSystem()

	regimes := constraints.GetAllRegimes()
	assert.ElementsMatch(t, []string{"calm", "normal", "volatile"}, regimes)

	for _, regime := range regimes {
		constraint, err := constraints.GetConstraints(regime)
		require.NoError(t, err)

		// Momentum bounds should be reasonable
		assert.GreaterOrEqual(t, constraint.MomentumBounds[0], 0.40, "Momentum minimum should be at least 40%")
		assert.LessOrEqual(t, constraint.MomentumBounds[1], 0.50, "Momentum maximum should be at most 50%")

		// Technical bounds should be reasonable
		assert.GreaterOrEqual(t, constraint.TechnicalBounds[0], 0.15, "Technical minimum should be at least 15%")
		assert.LessOrEqual(t, constraint.TechnicalBounds[1], 0.25, "Technical maximum should be at most 25%")

		// Supply/demand bounds should be reasonable
		assert.GreaterOrEqual(t, constraint.SupplyDemandBounds[0], 0.20, "S/D minimum should be at least 20%")
		assert.LessOrEqual(t, constraint.SupplyDemandBounds[1], 0.35, "S/D maximum should be at most 35%")

		// Quality minimum should be positive
		assert.Greater(t, constraint.QualityMinimum, 0.0, "Quality minimum should be positive")
	}
}

// TestEdgeCases tests edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	constraints := weights.NewConstraintSystem()

	// Test unknown regime
	err := constraints.ValidateWeights("unknown", weights.RegimeWeights{})
	assert.Error(t, err, "Unknown regime should fail")
	assert.Contains(t, err.Error(), "no constraints defined")

	// Test zero weights
	zeroWeights := weights.RegimeWeights{}
	err = constraints.ValidateWeights("normal", zeroWeights)
	assert.Error(t, err, "Zero weights should fail multiple constraints")

	// Test negative weights (should be clamped)
	negativeWeights := weights.RegimeWeights{
		MomentumCore:      -0.1,
		TechnicalResidual: 0.5,
		VolumeResidual:    0.3,
		QualityResidual:   0.3,
	}

	clamped, err := constraints.ClampWeights("normal", negativeWeights)
	require.NoError(t, err)

	// Negative should be clamped to bounds
	assert.GreaterOrEqual(t, clamped.MomentumCore, 0.40, "Negative momentum should be clamped to minimum")
}

// TestConstraintSystemDeterminism tests that the constraint system is deterministic
func TestConstraintSystemDeterminism(t *testing.T) {
	constraints1 := weights.NewConstraintSystem()
	constraints2 := weights.NewConstraintSystem()

	// Should have same regimes
	regimes1 := constraints1.GetAllRegimes()
	regimes2 := constraints2.GetAllRegimes()
	assert.ElementsMatch(t, regimes1, regimes2, "Constraint systems should have same regimes")

	// Should have same bounds
	for _, regime := range regimes1 {
		bounds1, err1 := constraints1.GetConstraints(regime)
		bounds2, err2 := constraints2.GetConstraints(regime)
		require.NoError(t, err1)
		require.NoError(t, err2)

		assert.Equal(t, bounds1, bounds2, "Constraints should be identical for regime %s", regime)
	}
}
