package tune

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/tune/data"
	"github.com/sawpanic/cryptorun/internal/tune/weights"
)

// TestObjectiveFunctionEvaluation tests basic objective function evaluation
func TestObjectiveFunctionEvaluation(t *testing.T) {
	// Create mock data
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 50)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"24h"}, 20, 3)

	// Create base weights
	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	// Create objective function
	config := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(config, smokeResults, benchResults, baseWeights, "normal")

	// Test evaluation
	testWeights := baseWeights["normal"]
	result, err := objective.Evaluate(testWeights)
	require.NoError(t, err)

	// Validate result structure
	assert.GreaterOrEqual(t, result.TotalScore, 0.0, "Total score should be non-negative")
	assert.GreaterOrEqual(t, result.HitRateScore, 0.0, "Hit rate score should be non-negative")
	assert.LessOrEqual(t, result.HitRateScore, 1.0, "Hit rate score should be ≤ 1.0")
	assert.GreaterOrEqual(t, result.SmokeCount, 0, "Smoke count should be non-negative")
	assert.GreaterOrEqual(t, result.BenchCount, 0, "Bench count should be non-negative")

	// Should have some data
	assert.Greater(t, result.SmokeCount+result.BenchCount, 0, "Should have some test data")
}

// TestObjectiveFunctionMonotonicity tests that objective function responds to improvements
func TestObjectiveFunctionMonotonicity(t *testing.T) {
	// Create synthetic data where higher scores correlate with higher returns
	smokeResults := []data.SmokeResult{
		{Symbol: "TEST1", Score: 80, ForwardReturn: 0.03, Regime: "normal", Window: "4h", Hit: true},
		{Symbol: "TEST2", Score: 70, ForwardReturn: 0.02, Regime: "normal", Window: "4h", Hit: false},
		{Symbol: "TEST3", Score: 90, ForwardReturn: 0.04, Regime: "normal", Window: "4h", Hit: true},
		{Symbol: "TEST4", Score: 60, ForwardReturn: 0.01, Regime: "normal", Window: "4h", Hit: false},
	}

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(config, smokeResults, []data.BenchResult{}, baseWeights, "normal")

	// Test baseline weights
	baselineResult, err := objective.Evaluate(baseWeights["normal"])
	require.NoError(t, err)

	// Test slightly adjusted weights (small improvement)
	improvedWeights := baseWeights["normal"]
	improvedWeights.MomentumCore += 0.01
	improvedWeights.TechnicalResidual -= 0.01 // Keep sum = 1

	improvedResult, err := objective.Evaluate(improvedWeights)
	require.NoError(t, err)

	// The objective function should be able to differentiate (though direction depends on data)
	assert.NotEqual(t, baselineResult.TotalScore, improvedResult.TotalScore,
		"Different weights should produce different objective scores")

	// Regularization penalty should be small for small changes
	assert.Less(t, improvedResult.RegularizationPenalty, 0.01,
		"Small weight changes should have small regularization penalty")
}

// TestObjectiveConfigurationVariation tests different objective configurations
func TestObjectiveConfigurationVariation(t *testing.T) {
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 30)
	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	testWeights := baseWeights["normal"]

	// Test hit rate focused configuration
	hitRateConfig := weights.ObjectiveConfig{
		HitRateWeight:    0.9,
		SpearmanWeight:   0.1,
		RegularizationL2: 0.005,
		PrimaryMetric:    "hitrate",
	}

	hitRateObjective := weights.NewObjectiveFunction(hitRateConfig, smokeResults, []data.BenchResult{}, baseWeights, "normal")
	hitRateResult, err := hitRateObjective.Evaluate(testWeights)
	require.NoError(t, err)

	// Test Spearman focused configuration
	spearmanConfig := weights.ObjectiveConfig{
		HitRateWeight:    0.1,
		SpearmanWeight:   0.9,
		RegularizationL2: 0.005,
		PrimaryMetric:    "spearman",
	}

	spearmanObjective := weights.NewObjectiveFunction(spearmanConfig, smokeResults, []data.BenchResult{}, baseWeights, "normal")
	spearmanResult, err := spearmanObjective.Evaluate(testWeights)
	require.NoError(t, err)

	// Different configurations should potentially produce different total scores
	// (depending on the relative values of hit rate vs Spearman correlation)
	assert.True(t, hitRateResult.TotalScore != spearmanResult.TotalScore ||
		hitRateResult.HitRateScore == hitRateResult.SpearmanScore,
		"Different objective configurations should produce different results (unless hit rate equals Spearman)")
}

// TestRegularizationPenalty tests L2 regularization calculation
func TestRegularizationPenalty(t *testing.T) {
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 10)

	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()
	objective := weights.NewObjectiveFunction(config, smokeResults, []data.BenchResult{}, baseWeights, "normal")

	// Test identical weights (no penalty)
	result, err := objective.Evaluate(baseWeights["normal"])
	require.NoError(t, err)
	assert.Equal(t, 0.0, result.RegularizationPenalty, "Identical weights should have zero regularization penalty")

	// Test modified weights (should have penalty)
	modifiedWeights := baseWeights["normal"]
	modifiedWeights.MomentumCore = 0.45      // +0.03 change
	modifiedWeights.TechnicalResidual = 0.17 // -0.03 change to maintain sum

	result, err = objective.Evaluate(modifiedWeights)
	require.NoError(t, err)

	// Penalty should be sum of squared differences
	expectedPenalty := (0.45-0.42)*(0.45-0.42) + (0.17-0.20)*(0.17-0.20) // 0.03² + (-0.03)² = 0.0018
	assert.InDelta(t, expectedPenalty, result.RegularizationPenalty, 0.0001,
		"Regularization penalty should match L2 norm of weight changes")
}

// TestObjectiveWithMixedData tests objective function with both smoke and bench data
func TestObjectiveWithMixedData(t *testing.T) {
	smokeResults := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 20)
	benchResults := data.CreateMockBenchResults([]string{"normal"}, []string{"24h"}, 15, 2)

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

	result, err := objective.Evaluate(baseWeights["normal"])
	require.NoError(t, err)

	// Should have data from both sources
	assert.Greater(t, result.SmokeCount, 0, "Should have smoke data")
	assert.Greater(t, result.BenchCount, 0, "Should have bench data")

	// Combined metrics should be reasonable
	assert.GreaterOrEqual(t, result.HitRateScore, 0.0, "Combined hit rate should be non-negative")
	assert.LessOrEqual(t, result.HitRateScore, 1.0, "Combined hit rate should be ≤ 1.0")
	assert.GreaterOrEqual(t, result.SpearmanScore, -1.0, "Combined Spearman should be ≥ -1.0")
	assert.LessOrEqual(t, result.SpearmanScore, 1.0, "Combined Spearman should be ≤ 1.0")
}

// TestObjectiveWithNoData tests objective function with insufficient data
func TestObjectiveWithNoData(t *testing.T) {
	baseWeights := map[string]weights.RegimeWeights{
		"normal": {
			MomentumCore:      0.42,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.25,
			QualityResidual:   0.13,
		},
	}

	config := weights.DefaultObjectiveConfig()

	// Test with no data for regime
	objective := weights.NewObjectiveFunction(config, []data.SmokeResult{}, []data.BenchResult{}, baseWeights, "normal")

	_, err := objective.Evaluate(baseWeights["normal"])
	assert.Error(t, err, "Should fail with no data available")
	assert.Contains(t, err.Error(), "no data available", "Error should mention no data")

	// Test with data for different regime
	smokeDifferentRegime := data.CreateMockResults([]string{"volatile"}, []string{"4h"}, 10) // Different regime
	objective = weights.NewObjectiveFunction(config, smokeDifferentRegime, []data.BenchResult{}, baseWeights, "normal")

	_, err = objective.Evaluate(baseWeights["normal"])
	assert.Error(t, err, "Should fail when no data matches target regime")
}

// TestSpearmanCorrelationCalculation tests the Spearman correlation calculation
func TestSpearmanCorrelationCalculation(t *testing.T) {
	// Test perfect positive correlation
	x1 := []float64{1, 2, 3, 4, 5}
	y1 := []float64{2, 4, 6, 8, 10}
	corr1 := calculateSpearmanCorrelation(x1, y1)
	assert.InDelta(t, 1.0, corr1, 0.01, "Perfect positive correlation should be ~1.0")

	// Test perfect negative correlation
	x2 := []float64{1, 2, 3, 4, 5}
	y2 := []float64{10, 8, 6, 4, 2}
	corr2 := calculateSpearmanCorrelation(x2, y2)
	assert.InDelta(t, -1.0, corr2, 0.01, "Perfect negative correlation should be ~-1.0")

	// Test no correlation (random order)
	x3 := []float64{1, 2, 3, 4, 5}
	y3 := []float64{3, 1, 5, 2, 4}
	corr3 := calculateSpearmanCorrelation(x3, y3)
	assert.GreaterOrEqual(t, corr3, -1.0, "Correlation should be ≥ -1.0")
	assert.LessOrEqual(t, corr3, 1.0, "Correlation should be ≤ 1.0")
	assert.NotEqual(t, 1.0, corr3, "Random order should not be perfect correlation")
	assert.NotEqual(t, -1.0, corr3, "Random order should not be perfect anti-correlation")

	// Test edge cases
	assert.Equal(t, 0.0, calculateSpearmanCorrelation([]float64{}, []float64{}), "Empty arrays should return 0")
	assert.Equal(t, 0.0, calculateSpearmanCorrelation([]float64{1}, []float64{2}), "Single elements should return 0")
	assert.Equal(t, 0.0, calculateSpearmanCorrelation([]float64{1, 2}, []float64{3}), "Mismatched lengths should return 0")
}

// TestConfidenceIntervalCalculation tests confidence interval calculation
func TestConfidenceIntervalCalculation(t *testing.T) {
	objective := &weights.ObjectiveFunction{} // Just need the method

	// Test with simple data
	values := []float64{0.8, 0.85, 0.9, 0.82, 0.88, 0.86, 0.87, 0.83, 0.89, 0.84}

	lower, upper, err := objective.CalculateConfidenceInterval(values, 0.95)
	require.NoError(t, err)

	// Calculate expected mean
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	assert.Less(t, lower, mean, "Lower bound should be below mean")
	assert.Greater(t, upper, mean, "Upper bound should be above mean")
	assert.Less(t, lower, upper, "Lower bound should be below upper bound")

	// Test with insufficient data
	_, _, err = objective.CalculateConfidenceInterval([]float64{0.5}, 0.95)
	assert.Error(t, err, "Should fail with insufficient data")
}

// TestDefaultObjectiveConfig tests the default configuration
func TestDefaultObjectiveConfig(t *testing.T) {
	config := weights.DefaultObjectiveConfig()

	assert.Equal(t, 0.7, config.HitRateWeight, "Default hit rate weight should be 0.7")
	assert.Equal(t, 0.3, config.SpearmanWeight, "Default Spearman weight should be 0.3")
	assert.Equal(t, 0.005, config.RegularizationL2, "Default L2 regularization should be 0.005")
	assert.Equal(t, "hitrate", config.PrimaryMetric, "Default primary metric should be hitrate")

	// Weights should sum to 1.0
	assert.InDelta(t, 1.0, config.HitRateWeight+config.SpearmanWeight, 0.001,
		"Hit rate and Spearman weights should sum to 1.0")
}

// Helper function - make sure it matches the implementation
func calculateSpearmanCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0.0
	}

	n := len(x)

	// Create rank arrays
	xRanks := getRanks(x)
	yRanks := getRanks(y)

	// Calculate differences and sum of squared differences
	var sumDiff2 float64
	for i := 0; i < n; i++ {
		diff := xRanks[i] - yRanks[i]
		sumDiff2 += diff * diff
	}

	// Spearman correlation formula
	spearman := 1.0 - (6.0*sumDiff2)/float64(n*(n*n-1))

	return spearman
}

// Helper function for ranking
func getRanks(values []float64) []float64 {
	n := len(values)

	// Create index-value pairs for sorting
	type IndexValue struct {
		Index int
		Value float64
	}

	pairs := make([]IndexValue, n)
	for i, v := range values {
		pairs[i] = IndexValue{Index: i, Value: v}
	}

	// Sort by value
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[i].Value > pairs[j].Value {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	// Assign ranks
	ranks := make([]float64, n)
	for rank, pair := range pairs {
		ranks[pair.Index] = float64(rank + 1) // 1-based ranks
	}

	return ranks
}
