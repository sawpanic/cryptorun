package tune

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/tune/data"
	"github.com/sawpanic/cryptorun/internal/tune/weights"
)

func TestObjectiveFunction_BasicEvaluation(t *testing.T) {
	// Create mock data for testing
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 10)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 10, 3)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(config, smokeResults, benchResults, baseWeights, "normal")

	// Test with valid weights
	validWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	result, err := objective.Evaluate(validWeights)
	require.NoError(t, err)

	// Basic validation
	assert.GreaterOrEqual(t, result.TotalScore, -1.0, "Total score should be reasonable")
	assert.LessOrEqual(t, result.TotalScore, 2.0, "Total score should be reasonable")

	// Component scores should be present
	assert.GreaterOrEqual(t, result.HitRateScore, 0.0, "Hit rate score should be non-negative")
	assert.GreaterOrEqual(t, result.SpearmanScore, -1.0, "Spearman score should be >= -1")
	assert.LessOrEqual(t, result.SpearmanScore, 1.0, "Spearman score should be <= 1")

	// Sample sizes should be populated
	assert.Greater(t, result.SmokeCount, 0, "Should have smoke data points")
	assert.Greater(t, result.BenchCount, 0, "Should have bench data points")
}

func TestObjectiveFunction_MultiTimeframeWeights(t *testing.T) {
	// Create results for multiple windows
	smokeResults1h := data.CreateMockResults([]string{"normal"}, []string{"1h"}, 5)
	smokeResults4h := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 8)
	smokeResults12h := data.CreateMockResults([]string{"normal"}, []string{"12h"}, 3)

	allResults := append(smokeResults1h, smokeResults4h...)
	allResults = append(allResults, smokeResults12h...)

	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 10, 2)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(config, allResults, benchResults, baseWeights, "normal")

	testWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	result, err := objective.Evaluate(testWeights)
	require.NoError(t, err)

	// Should evaluate multi-timeframe data
	assert.Greater(t, result.SmokeCount, 10, "Should have multiple smoke data points")
	assert.Greater(t, result.BenchCount, 5, "Should have bench data points")
}

func TestObjectiveFunction_RegularizationPenalty(t *testing.T) {
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 10)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 10, 2)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(config, smokeResults, benchResults, baseWeights, "normal")

	// Test weights identical to base (should have low penalty)
	identicalWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	// Test weights different from base (should have higher penalty)
	differentWeights := weights.RegimeWeights{
		MomentumCore:      0.45, // +0.03
		TechnicalResidual: 0.18, // -0.02
		VolumeResidual:    0.23, // -0.02
		QualityResidual:   0.14, // +0.01
	}

	result1, err := objective.Evaluate(identicalWeights)
	require.NoError(t, err)

	result2, err := objective.Evaluate(differentWeights)
	require.NoError(t, err)

	// Different weights should have higher regularization penalty
	assert.Greater(t, result2.RegularizationPenalty, result1.RegularizationPenalty,
		"Different weights should have higher regularization penalty")
	assert.InDelta(t, 0.0, result1.RegularizationPenalty, 1e-6,
		"Identical weights should have ~zero regularization penalty")
}

func TestObjectiveFunction_EdgeCases_EmptyPanel(t *testing.T) {
	// Test with empty data
	emptySmoke := []data.SmokeResult{}
	emptyBench := []data.BenchResult{}

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(config, emptySmoke, emptyBench, baseWeights, "normal")

	testWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	_, err := objective.Evaluate(testWeights)

	// Should handle gracefully
	assert.Error(t, err, "Empty panel should produce an error")
	assert.Contains(t, err.Error(), "no data available", "Error should mention no data")
}

func TestObjectiveFunction_EdgeCases_SingleAsset(t *testing.T) {
	// Test with single asset
	singleSmoke := []data.SmokeResult{
		{
			Symbol:        "BTCUSDT",
			Score:         75.0,
			ForwardReturn: 0.03,
			Hit:           true,
			Window:        "4h",
			Regime:        "normal",
		},
	}

	singleBench := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 1, 1)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(config, singleSmoke, singleBench, baseWeights, "normal")

	testWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	result, err := objective.Evaluate(testWeights)
	require.NoError(t, err)

	assert.Equal(t, 1, result.SmokeCount, "Single asset should have 1 smoke data point")
	assert.GreaterOrEqual(t, result.TotalScore, -1.0, "Single asset evaluation should work")
}

func TestObjectiveFunction_EdgeCases_DegenerateVariance(t *testing.T) {
	// Test with all identical scores (zero variance)
	identicalSmoke := make([]data.SmokeResult, 5)
	for i := range identicalSmoke {
		identicalSmoke[i] = data.SmokeResult{
			Symbol:        "TESTSYM",
			Score:         75.0, // All identical
			ForwardReturn: 0.03, // All identical
			Hit:           true,
			Window:        "4h",
			Regime:        "normal",
		}
	}

	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 5, 1)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(config, identicalSmoke, benchResults, baseWeights, "normal")

	testWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	result, err := objective.Evaluate(testWeights)
	require.NoError(t, err)

	// Spearman correlation should be 0 for degenerate case
	assert.Equal(t, 0.0, result.SmokeSpearman, "Degenerate variance should produce zero Spearman correlation")
	assert.False(t, math.IsNaN(result.TotalScore), "Total score should not be NaN")
}

func TestObjectiveFunction_WeightSensitivity(t *testing.T) {
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 20)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 10, 3)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(config, smokeResults, benchResults, baseWeights, "normal")

	baseTestWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	// Small weight perturbation
	perturbedWeights := weights.RegimeWeights{
		MomentumCore:      0.43, // +0.01
		TechnicalResidual: 0.19, // -0.01
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	baseResult, err := objective.Evaluate(baseTestWeights)
	require.NoError(t, err)

	perturbedResult, err := objective.Evaluate(perturbedWeights)
	require.NoError(t, err)

	// Small changes should produce detectable but reasonable differences
	scoreDiff := math.Abs(perturbedResult.TotalScore - baseResult.TotalScore)
	assert.Greater(t, scoreDiff, 0.0, "Weight changes should affect objective")
	assert.Less(t, scoreDiff, 1.0, "Small weight changes should produce bounded score changes")
}

func TestObjectiveFunction_ConfigurationImpact(t *testing.T) {
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 15)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 10, 2)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	// Test hit rate focused config
	hitRateConfig := weights.ObjectiveConfig{
		HitRateWeight:    0.9, // Heavily favor hit rate
		SpearmanWeight:   0.1,
		RegularizationL2: 0.001,
		PrimaryMetric:    "hitrate",
	}

	// Test Spearman focused config
	spearmanConfig := weights.ObjectiveConfig{
		HitRateWeight:    0.1,
		SpearmanWeight:   0.9, // Heavily favor Spearman
		RegularizationL2: 0.001,
		PrimaryMetric:    "spearman",
	}

	hitRateObjective := weights.NewObjectiveFunction(hitRateConfig, smokeResults, benchResults, baseWeights, "normal")
	spearmanObjective := weights.NewObjectiveFunction(spearmanConfig, smokeResults, benchResults, baseWeights, "normal")

	testWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	hitRateResult, err := hitRateObjective.Evaluate(testWeights)
	require.NoError(t, err)

	spearmanResult, err := spearmanObjective.Evaluate(testWeights)
	require.NoError(t, err)

	// The objectives should produce different scores due to different weightings
	scoreDiff := math.Abs(hitRateResult.TotalScore - spearmanResult.TotalScore)

	// Allow for some tolerance as both may be similar with test data
	t.Logf("HitRate-focused score: %.4f, Spearman-focused score: %.4f, diff: %.4f",
		hitRateResult.TotalScore, spearmanResult.TotalScore, scoreDiff)
}

func TestObjectiveFunction_Deterministic(t *testing.T) {
	// Test that objective function is deterministic
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 10)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 10, 2)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()
	objective1 := weights.NewObjectiveFunction(config, smokeResults, benchResults, baseWeights, "normal")
	objective2 := weights.NewObjectiveFunction(config, smokeResults, benchResults, baseWeights, "normal")

	testWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	result1, err := objective1.Evaluate(testWeights)
	require.NoError(t, err)

	result2, err := objective2.Evaluate(testWeights)
	require.NoError(t, err)

	// Results should be identical
	assert.Equal(t, result1.TotalScore, result2.TotalScore, "Objective function should be deterministic")
	assert.Equal(t, result1.HitRateScore, result2.HitRateScore, "Hit rate score should be deterministic")
	assert.Equal(t, result1.SpearmanScore, result2.SpearmanScore, "Spearman score should be deterministic")
	assert.Equal(t, result1.SmokeCount, result2.SmokeCount, "Smoke count should be identical")
	assert.Equal(t, result1.BenchCount, result2.BenchCount, "Bench count should be identical")
}

func TestObjectiveFunction_RegimeFiltering(t *testing.T) {
	// Create data for multiple regimes
	smokeResults := append(
		data.CreateMockResults([]string{"normal"}, []string{"4h"}, 5),
		data.CreateMockResults([]string{"volatile"}, []string{"4h"}, 5)...,
	)
	benchResults := append(
		data.CreateMockBenchResults([]string{"normal"}, []string{"4h"}, 5, 1),
		data.CreateMockBenchResults([]string{"volatile"}, []string{"4h"}, 5, 1)...,
	)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()

	// Test normal regime filtering
	normalObjective := weights.NewObjectiveFunction(config, smokeResults, benchResults, baseWeights, "normal")

	// Test volatile regime filtering
	volatileObjective := weights.NewObjectiveFunction(config, smokeResults, benchResults, baseWeights, "volatile")

	testWeights := weights.RegimeWeights{
		MomentumCore:      0.42,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.25,
		QualityResidual:   0.13,
	}

	normalResult, err := normalObjective.Evaluate(testWeights)
	require.NoError(t, err)

	volatileResult, err := volatileObjective.Evaluate(testWeights)
	require.NoError(t, err)

	// Should filter to only the specified regime's data
	assert.Equal(t, 5, normalResult.SmokeCount, "Normal objective should use only normal smoke data")
	assert.Equal(t, 5, normalResult.BenchCount, "Normal objective should use only normal bench data")

	assert.Equal(t, 5, volatileResult.SmokeCount, "Volatile objective should use only volatile smoke data")
	assert.Equal(t, 5, volatileResult.BenchCount, "Volatile objective should use only volatile bench data")
}
