package weights

import (
	"fmt"
	"math"

	"cryptorun/internal/tune/data"
)

// ObjectiveConfig defines the configuration for the objective function
type ObjectiveConfig struct {
	HitRateWeight    float64 `json:"hit_rate_weight"`   // w1: Weight for hit rate (default: 0.7)
	SpearmanWeight   float64 `json:"spearman_weight"`   // w2: Weight for Spearman correlation (default: 0.3)
	RegularizationL2 float64 `json:"regularization_l2"` // λ: L2 regularization strength (default: 0.005)
	PrimaryMetric    string  `json:"primary_metric"`    // "hitrate" or "spearman"
}

// DefaultObjectiveConfig returns the default objective configuration
func DefaultObjectiveConfig() ObjectiveConfig {
	return ObjectiveConfig{
		HitRateWeight:    0.7,
		SpearmanWeight:   0.3,
		RegularizationL2: 0.005,
		PrimaryMetric:    "hitrate",
	}
}

// ObjectiveFunction evaluates weight configurations for optimization
type ObjectiveFunction struct {
	config      ObjectiveConfig
	smokeData   []data.SmokeResult
	benchData   []data.BenchResult
	baseWeights map[string]RegimeWeights // Original weights for regularization
	regime      string
}

// NewObjectiveFunction creates a new objective function
func NewObjectiveFunction(config ObjectiveConfig, smokeData []data.SmokeResult, benchData []data.BenchResult, baseWeights map[string]RegimeWeights, regime string) *ObjectiveFunction {
	return &ObjectiveFunction{
		config:      config,
		smokeData:   smokeData,
		benchData:   benchData,
		baseWeights: baseWeights,
		regime:      regime,
	}
}

// ObjectiveResult holds the evaluation result for a weight configuration
type ObjectiveResult struct {
	TotalScore            float64 `json:"total_score"`
	HitRateScore          float64 `json:"hit_rate_score"`
	SpearmanScore         float64 `json:"spearman_score"`
	RegularizationPenalty float64 `json:"regularization_penalty"`

	// Detailed metrics
	SmokeHitRate  float64 `json:"smoke_hit_rate"`
	SmokeSpearman float64 `json:"smoke_spearman"`
	BenchHitRate  float64 `json:"bench_hit_rate"`
	BenchSpearman float64 `json:"bench_spearman"`

	// Sample sizes
	SmokeCount int `json:"smoke_count"`
	BenchCount int `json:"bench_count"`
}

// Evaluate computes the objective function value for given weights
func (of *ObjectiveFunction) Evaluate(weights RegimeWeights) (ObjectiveResult, error) {
	result := ObjectiveResult{}

	// Calculate smoke test metrics
	smokeRegimeData := of.filterSmokeByRegime(of.smokeData, of.regime)
	if len(smokeRegimeData) > 0 {
		smokeMetrics := of.calculateSmokeMetrics(smokeRegimeData, weights)
		result.SmokeHitRate = smokeMetrics.HitRate
		result.SmokeSpearman = smokeMetrics.SpearmanCorr
		result.SmokeCount = len(smokeRegimeData)
	}

	// Calculate bench test metrics
	benchRegimeData := of.filterBenchByRegime(of.benchData, of.regime)
	if len(benchRegimeData) > 0 {
		benchMetrics := of.calculateBenchMetrics(benchRegimeData, weights)
		result.BenchHitRate = benchMetrics.HitRate
		result.BenchSpearman = benchMetrics.RankCorr
		result.BenchCount = len(benchRegimeData)
	}

	// Combine metrics (smoke and bench equally weighted if both available)
	var combinedHitRate, combinedSpearman float64
	if result.SmokeCount > 0 && result.BenchCount > 0 {
		combinedHitRate = 0.6*result.SmokeHitRate + 0.4*result.BenchHitRate // Favor smoke tests
		combinedSpearman = 0.6*result.SmokeSpearman + 0.4*result.BenchSpearman
	} else if result.SmokeCount > 0 {
		combinedHitRate = result.SmokeHitRate
		combinedSpearman = result.SmokeSpearman
	} else if result.BenchCount > 0 {
		combinedHitRate = result.BenchHitRate
		combinedSpearman = result.BenchSpearman
	} else {
		return result, fmt.Errorf("no data available for regime %s", of.regime)
	}

	// Calculate component scores
	result.HitRateScore = combinedHitRate
	result.SpearmanScore = combinedSpearman

	// Calculate L2 regularization penalty
	baseWeights, exists := of.baseWeights[of.regime]
	if exists {
		result.RegularizationPenalty = of.calculateL2Penalty(weights, baseWeights)
	}

	// Calculate total objective score
	result.TotalScore = of.config.HitRateWeight*result.HitRateScore +
		of.config.SpearmanWeight*result.SpearmanScore -
		of.config.RegularizationL2*result.RegularizationPenalty

	// Adjust based on primary metric preference
	if of.config.PrimaryMetric == "spearman" {
		result.TotalScore = of.config.SpearmanWeight*result.SpearmanScore +
			of.config.HitRateWeight*result.HitRateScore -
			of.config.RegularizationL2*result.RegularizationPenalty
	}

	return result, nil
}

// filterSmokeByRegime filters smoke results to only include specified regime
func (of *ObjectiveFunction) filterSmokeByRegime(results []data.SmokeResult, regime string) []data.SmokeResult {
	var filtered []data.SmokeResult
	for _, result := range results {
		if result.Regime == regime {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

// filterBenchByRegime filters bench results to only include specified regime
func (of *ObjectiveFunction) filterBenchByRegime(results []data.BenchResult, regime string) []data.BenchResult {
	var filtered []data.BenchResult
	for _, result := range results {
		if result.Regime == regime {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

// calculateSmokeMetrics calculates performance metrics for smoke test data
func (of *ObjectiveFunction) calculateSmokeMetrics(results []data.SmokeResult, weights RegimeWeights) data.RegimeMetrics {
	// For this simulation, we assume re-scoring with new weights affects the results
	// In practice, this would involve re-running the composite scoring with new weights

	// Simulate the effect of weight changes on hit rate and correlation
	hitCount := 0
	scores := make([]float64, len(results))
	returns := make([]float64, len(results))

	for i, result := range results {
		// Simulate adjusted score based on weight perturbation
		adjustedScore := of.simulateAdjustedScore(result.Score, weights)
		scores[i] = adjustedScore
		returns[i] = result.ForwardReturn

		// Recalculate hit based on adjusted score threshold
		if result.ForwardReturn >= 0.025 && adjustedScore >= 75.0 {
			hitCount++
		}
	}

	hitRate := float64(hitCount) / float64(len(results))
	spearmanCorr := calculateSpearmanCorrelation(scores, returns)

	return data.RegimeMetrics{
		Regime:       of.regime,
		TotalSignals: len(results),
		Hits:         hitCount,
		HitRate:      hitRate,
		SpearmanCorr: spearmanCorr,
	}
}

// calculateBenchMetrics calculates performance metrics for benchmark data
func (of *ObjectiveFunction) calculateBenchMetrics(results []data.BenchResult, weights RegimeWeights) data.BenchMetrics {
	hitCount := 0
	ranks := make([]float64, len(results))
	gains := make([]float64, len(results))

	for i, result := range results {
		// Simulate adjusted score affecting ranking
		adjustedScore := of.simulateAdjustedScore(result.Score, weights)

		// Use negative adjusted score for rank correlation (higher score = better rank = lower number)
		ranks[i] = -adjustedScore
		gains[i] = result.ActualGain

		// Count benchmark hits (simplified logic)
		if result.BenchmarkHit && adjustedScore >= 75.0 {
			hitCount++
		}
	}

	hitRate := float64(hitCount) / float64(len(results))
	rankCorr := calculateSpearmanCorrelation(ranks, gains)

	return data.BenchMetrics{
		Regime:       of.regime,
		TotalSymbols: len(results),
		HitRate:      hitRate,
		RankCorr:     rankCorr,
	}
}

// simulateAdjustedScore simulates how weight changes might affect composite scores
func (of *ObjectiveFunction) simulateAdjustedScore(originalScore float64, newWeights RegimeWeights) float64 {
	// Get baseline weights for comparison
	baseWeights, exists := of.baseWeights[of.regime]
	if !exists {
		return originalScore
	}

	// Calculate relative weight changes
	momentumDelta := newWeights.MomentumCore - baseWeights.MomentumCore
	technicalDelta := newWeights.TechnicalResidual - baseWeights.TechnicalResidual
	volumeDelta := newWeights.VolumeResidual - baseWeights.VolumeResidual
	qualityDelta := newWeights.QualityResidual - baseWeights.QualityResidual

	// Simulate score adjustment based on weight perturbations
	// This is a simplified model - in reality, weight changes affect orthogonalization
	adjustment := momentumDelta*0.5 + technicalDelta*0.3 + volumeDelta*0.15 + qualityDelta*0.05
	adjustedScore := originalScore + adjustment*20.0 // Scale factor for realistic score changes

	// Clamp to valid score range
	return math.Max(0, math.Min(100, adjustedScore))
}

// calculateL2Penalty computes the L2 regularization penalty
func (of *ObjectiveFunction) calculateL2Penalty(newWeights, baseWeights RegimeWeights) float64 {
	momentumDiff := newWeights.MomentumCore - baseWeights.MomentumCore
	technicalDiff := newWeights.TechnicalResidual - baseWeights.TechnicalResidual
	volumeDiff := newWeights.VolumeResidual - baseWeights.VolumeResidual
	qualityDiff := newWeights.QualityResidual - baseWeights.QualityResidual

	return momentumDiff*momentumDiff + technicalDiff*technicalDiff + volumeDiff*volumeDiff + qualityDiff*qualityDiff
}

// CalculateConfidenceInterval calculates confidence intervals for metrics
func (of *ObjectiveFunction) CalculateConfidenceInterval(values []float64, confidence float64) (float64, float64, error) {
	if len(values) < 2 {
		return 0, 0, fmt.Errorf("insufficient data for confidence interval")
	}

	// Calculate mean and standard error
	var sum, sumSq float64
	n := float64(len(values))

	for _, v := range values {
		sum += v
		sumSq += v * v
	}

	mean := sum / n
	variance := (sumSq - sum*sum/n) / (n - 1)
	stdErr := math.Sqrt(variance / n)

	// Use t-distribution approximation for 95% CI
	tValue := 1.96 // Approximation for large samples
	if n < 30 {
		// Rough t-values for small samples
		tValues := map[int]float64{
			5: 2.78, 10: 2.23, 15: 2.14, 20: 2.09, 25: 2.06,
		}
		for size, t := range tValues {
			if int(n) <= size {
				tValue = t
				break
			}
		}
	}

	margin := tValue * stdErr
	return mean - margin, mean + margin, nil
}

// calculateSpearmanCorrelation computes Spearman rank correlation (reused from data package)
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

// getRanks converts values to ranks (1-based)
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
