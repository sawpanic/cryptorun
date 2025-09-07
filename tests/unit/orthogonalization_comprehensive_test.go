package unit

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/score/composite"
)

// TestOrthogonalizationResidualizationSanity tests the core requirement:
// RESIDUALIZATION SANITY - orthogonality checks for Gram-Schmidt
func TestOrthogonalizationResidualizationSanity(t *testing.T) {
	orthogonalizer := composite.NewOrthogonalizer()

	t.Run("MomentumCore_Protection", func(t *testing.T) {
		// MomentumCore must remain unchanged (protected)
		originalMomentum := []float64{0.8, 0.6, 0.9, 0.7, 0.5}
		factors := []composite.Factor{
			{Name: "momentum_core", Values: originalMomentum, Protected: true},
			{Name: "technical", Values: []float64{0.7, 0.5, 0.8, 0.6, 0.4}},
			{Name: "volume", Values: []float64{0.6, 0.8, 0.7, 0.5, 0.9}},
			{Name: "quality", Values: []float64{0.5, 0.7, 0.6, 0.8, 0.3}},
		}

		result, err := orthogonalizer.Orthogonalize(factors)
		require.NoError(t, err)

		// MomentumCore should be identical to input
		assert.Equal(t, originalMomentum, result.MomentumCore.Values, "MomentumCore must remain unchanged")
		assert.True(t, result.MomentumCore.Protected, "MomentumCore must remain protected")
	})

	t.Run("Orthogonality_Verification", func(t *testing.T) {
		// Create factors with known correlation
		factors := []composite.Factor{
			{Name: "momentum_core", Values: []float64{1.0, 2.0, 3.0, 4.0, 5.0}, Protected: true},
			{Name: "technical", Values: []float64{1.1, 2.1, 3.1, 4.1, 5.1}}, // Highly correlated with momentum
			{Name: "volume", Values: []float64{2.0, 4.0, 6.0, 8.0, 10.0}},   // 2x momentum
			{Name: "quality", Values: []float64{0.5, 1.0, 1.5, 2.0, 2.5}},   // 0.5x momentum
		}

		result, err := orthogonalizer.Orthogonalize(factors)
		require.NoError(t, err)

		// Verify orthogonality: dot product between any two residualized factors should be ~0
		tolerance := 1e-10

		// MomentumCore vs TechnicalResid
		dotProduct1 := dotProduct(result.MomentumCore.Values, result.TechnicalResid.Values)
		assert.InDelta(t, 0.0, dotProduct1, tolerance, "MomentumCore and TechnicalResid should be orthogonal")

		// MomentumCore vs VolumeResid
		dotProduct2 := dotProduct(result.MomentumCore.Values, result.VolumeResid.Values)
		assert.InDelta(t, 0.0, dotProduct2, tolerance, "MomentumCore and VolumeResid should be orthogonal")

		// MomentumCore vs QualityResid
		dotProduct3 := dotProduct(result.MomentumCore.Values, result.QualityResid.Values)
		assert.InDelta(t, 0.0, dotProduct3, tolerance, "MomentumCore and QualityResid should be orthogonal")

		// TechnicalResid vs VolumeResid
		dotProduct4 := dotProduct(result.TechnicalResid.Values, result.VolumeResid.Values)
		assert.InDelta(t, 0.0, dotProduct4, tolerance, "TechnicalResid and VolumeResid should be orthogonal")

		// TechnicalResid vs QualityResid
		dotProduct5 := dotProduct(result.TechnicalResid.Values, result.QualityResid.Values)
		assert.InDelta(t, 0.0, dotProduct5, tolerance, "TechnicalResid and QualityResid should be orthogonal")

		// VolumeResid vs QualityResid
		dotProduct6 := dotProduct(result.VolumeResid.Values, result.QualityResid.Values)
		assert.InDelta(t, 0.0, dotProduct6, tolerance, "VolumeResid and QualityResid should be orthogonal")
	})

	t.Run("Sequential_Order_Enforcement", func(t *testing.T) {
		// Test that order matters: MomentumCore → Technical → Volume → Quality
		factors := []composite.Factor{
			{Name: "momentum_core", Values: []float64{1.0, 0.8, 0.9, 0.7, 0.6}, Protected: true},
			{Name: "technical", Values: []float64{0.9, 0.7, 0.8, 0.6, 0.5}},
			{Name: "volume", Values: []float64{0.8, 0.6, 0.7, 0.5, 0.4}},
			{Name: "quality", Values: []float64{0.7, 0.5, 0.6, 0.4, 0.3}},
		}

		result, err := orthogonalizer.Orthogonalize(factors)
		require.NoError(t, err)

		// Technical should be orthogonalized against MomentumCore only
		assert.NotEqual(t, factors[1].Values, result.TechnicalResid.Values, "Technical should be modified")

		// Volume should be orthogonalized against MomentumCore AND TechnicalResid
		assert.NotEqual(t, factors[2].Values, result.VolumeResid.Values, "Volume should be modified")

		// Quality should be orthogonalized against all previous factors
		assert.NotEqual(t, factors[3].Values, result.QualityResid.Values, "Quality should be modified")
	})

	t.Run("Error_Conditions", func(t *testing.T) {
		// Test invalid number of factors
		wrongFactors := []composite.Factor{
			{Name: "momentum_core", Values: []float64{1.0}, Protected: true},
			{Name: "technical", Values: []float64{1.0}},
		}
		_, err := orthogonalizer.Orthogonalize(wrongFactors)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected 4 factors")

		// Test non-protected first factor
		unprotectedFactors := []composite.Factor{
			{Name: "momentum_core", Values: []float64{1.0}, Protected: false},
			{Name: "technical", Values: []float64{1.0}},
			{Name: "volume", Values: []float64{1.0}},
			{Name: "quality", Values: []float64{1.0}},
		}
		_, err = orthogonalizer.Orthogonalize(unprotectedFactors)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "first factor must be protected momentum_core")

		// Test wrong first factor name
		wrongNameFactors := []composite.Factor{
			{Name: "technical", Values: []float64{1.0}, Protected: true},
			{Name: "momentum_core", Values: []float64{1.0}},
			{Name: "volume", Values: []float64{1.0}},
			{Name: "quality", Values: []float64{1.0}},
		}
		_, err = orthogonalizer.Orthogonalize(wrongNameFactors)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "first factor must be protected momentum_core")
	})

	t.Run("Zero_Vector_Handling", func(t *testing.T) {
		// Test handling of zero vectors
		factors := []composite.Factor{
			{Name: "momentum_core", Values: []float64{0.0, 0.0, 0.0, 0.0, 0.0}, Protected: true},
			{Name: "technical", Values: []float64{1.0, 2.0, 3.0, 4.0, 5.0}},
			{Name: "volume", Values: []float64{0.5, 1.0, 1.5, 2.0, 2.5}},
			{Name: "quality", Values: []float64{0.2, 0.4, 0.6, 0.8, 1.0}},
		}

		result, err := orthogonalizer.Orthogonalize(factors)
		require.NoError(t, err)

		// When momentum is zero, other factors should remain unchanged
		assert.Equal(t, factors[1].Values, result.TechnicalResid.Values, "Technical should be unchanged when momentum is zero")
	})

	t.Run("Perfect_Correlation_Elimination", func(t *testing.T) {
		// Test case where a factor is perfectly correlated with a previous one
		factors := []composite.Factor{
			{Name: "momentum_core", Values: []float64{1.0, 2.0, 3.0, 4.0, 5.0}, Protected: true},
			{Name: "technical", Values: []float64{2.0, 4.0, 6.0, 8.0, 10.0}}, // Exactly 2x momentum
			{Name: "volume", Values: []float64{0.8, 0.6, 0.4, 0.2, 0.0}},     // Independent
			{Name: "quality", Values: []float64{0.1, 0.2, 0.3, 0.4, 0.5}},    // Independent
		}

		result, err := orthogonalizer.Orthogonalize(factors)
		require.NoError(t, err)

		// Technical should become ~zero vector after orthogonalization (perfect correlation removed)
		for _, val := range result.TechnicalResid.Values {
			assert.InDelta(t, 0.0, val, 1e-10, "Perfectly correlated technical factor should become zero")
		}
	})
}

// TestOrthogonalizationNumericalStability tests numerical stability of orthogonalization
func TestOrthogonalizationNumericalStability(t *testing.T) {
	orthogonalizer := composite.NewOrthogonalizer()

	t.Run("Large_Values", func(t *testing.T) {
		// Test with large values
		factors := []composite.Factor{
			{Name: "momentum_core", Values: []float64{1e6, 2e6, 3e6, 4e6, 5e6}, Protected: true},
			{Name: "technical", Values: []float64{1e6, 2e6, 3e6, 4e6, 5e6}},
			{Name: "volume", Values: []float64{2e6, 4e6, 6e6, 8e6, 10e6}},
			{Name: "quality", Values: []float64{0.5e6, 1e6, 1.5e6, 2e6, 2.5e6}},
		}

		result, err := orthogonalizer.Orthogonalize(factors)
		require.NoError(t, err)

		// Verify orthogonality is maintained even with large values
		dotProduct := dotProduct(result.MomentumCore.Values, result.TechnicalResid.Values)
		assert.InDelta(t, 0.0, dotProduct, 1e-3, "Orthogonality should be maintained with large values")
	})

	t.Run("Small_Values", func(t *testing.T) {
		// Test with very small values
		factors := []composite.Factor{
			{Name: "momentum_core", Values: []float64{1e-6, 2e-6, 3e-6, 4e-6, 5e-6}, Protected: true},
			{Name: "technical", Values: []float64{1e-6, 2e-6, 3e-6, 4e-6, 5e-6}},
			{Name: "volume", Values: []float64{2e-6, 4e-6, 6e-6, 8e-6, 10e-6}},
			{Name: "quality", Values: []float64{0.5e-6, 1e-6, 1.5e-6, 2e-6, 2.5e-6}},
		}

		result, err := orthogonalizer.Orthogonalize(factors)
		require.NoError(t, err)

		// Should handle small values without numerical issues
		assert.NotNil(t, result)
		for _, val := range result.TechnicalResid.Values {
			assert.False(t, math.IsNaN(val), "Should not produce NaN values")
			assert.False(t, math.IsInf(val, 0), "Should not produce infinite values")
		}
	})
}

// Helper function to calculate dot product
func dotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	sum := 0.0
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// TestOrthogonalizationRegressionPrevention prevents regression in orthogonalization behavior
func TestOrthogonalizationRegressionPrevention(t *testing.T) {
	orthogonalizer := composite.NewOrthogonalizer()

	t.Run("Known_Good_Case", func(t *testing.T) {
		// A specific test case that should always work the same way
		factors := []composite.Factor{
			{Name: "momentum_core", Values: []float64{0.85, 0.70, 0.92, 0.65, 0.78}, Protected: true},
			{Name: "technical", Values: []float64{0.75, 0.60, 0.80, 0.55, 0.68}},
			{Name: "volume", Values: []float64{0.65, 0.80, 0.70, 0.75, 0.60}},
			{Name: "quality", Values: []float64{0.55, 0.70, 0.60, 0.65, 0.50}},
		}

		result, err := orthogonalizer.Orthogonalize(factors)
		require.NoError(t, err)

		// MomentumCore should be exactly the same
		assert.Equal(t, factors[0].Values, result.MomentumCore.Values)

		// Technical should be different (orthogonalized)
		assert.NotEqual(t, factors[1].Values, result.TechnicalResid.Values)

		// All residuals should have reasonable magnitudes (not zero, not extreme)
		for _, val := range result.TechnicalResid.Values {
			assert.True(t, math.Abs(val) < 10.0, "Technical residual should have reasonable magnitude")
		}
		for _, val := range result.VolumeResid.Values {
			assert.True(t, math.Abs(val) < 10.0, "Volume residual should have reasonable magnitude")
		}
		for _, val := range result.QualityResid.Values {
			assert.True(t, math.Abs(val) < 10.0, "Quality residual should have reasonable magnitude")
		}

		// Check that orthogonalization preserves some signal (not all zeros)
		technicalNorm := vectorNorm(result.TechnicalResid.Values)
		volumeNorm := vectorNorm(result.VolumeResid.Values)
		qualityNorm := vectorNorm(result.QualityResid.Values)

		assert.True(t, technicalNorm > 0.01, "Technical residual should preserve some signal")
		assert.True(t, volumeNorm > 0.01, "Volume residual should preserve some signal")
		assert.True(t, qualityNorm > 0.01, "Quality residual should preserve some signal")
	})
}

// Helper function to calculate vector norm
func vectorNorm(v []float64) float64 {
	sum := 0.0
	for _, val := range v {
		sum += val * val
	}
	return math.Sqrt(sum)
}
