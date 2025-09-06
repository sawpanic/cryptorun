package common

import (
	"fmt"
	"math"
	"sort"
)

// BenchmarkMetrics contains comparative metrics between two scoring systems
type BenchmarkMetrics struct {
	SpearmanCorrelations map[string]float64 `json:"spearman_correlations"` // window -> correlation
	HitRates             HitRateMetrics     `json:"hit_rates"`
	DisagreementRate     float64            `json:"disagreement_rate"`
	AverageScoreDelta    float64            `json:"average_score_delta"`
	GatePassThrough      float64            `json:"gate_pass_through"`
	UnprovenCount        int                `json:"unproven_count"`
	SampleSize           map[string]int     `json:"sample_size"` // window -> count
}

// HitRateMetrics contains hit rates for both scoring systems
type HitRateMetrics struct {
	Unified map[string]float64 `json:"unified"` // window -> hit_rate
	Legacy  map[string]float64 `json:"legacy"`  // window -> hit_rate
}

// MetricsCalculator computes benchmark comparison metrics
type MetricsCalculator struct{}

// NewMetricsCalculator creates a metrics calculator
func NewMetricsCalculator() *MetricsCalculator {
	return &MetricsCalculator{}
}

// ScorePair represents unified and legacy scores for the same asset/window
type ScorePair struct {
	Symbol        string  `json:"symbol"`
	Window        string  `json:"window"`
	UnifiedScore  float64 `json:"unified_score"`
	LegacyScore   float64 `json:"legacy_score"`
	ForwardReturn float64 `json:"forward_return"`
	Hit           bool    `json:"hit"`
	UnifiedHit    bool    `json:"unified_hit"`
	LegacyHit     bool    `json:"legacy_hit"`
}

// CalculateMetrics computes all comparison metrics
func (mc *MetricsCalculator) CalculateMetrics(
	scorePairs []ScorePair,
	gatePassCount, totalCandidates, unprovenCount int,
) BenchmarkMetrics {

	// Group by window for analysis
	windowGroups := mc.groupByWindow(scorePairs)

	metrics := BenchmarkMetrics{
		SpearmanCorrelations: make(map[string]float64),
		HitRates: HitRateMetrics{
			Unified: make(map[string]float64),
			Legacy:  make(map[string]float64),
		},
		SampleSize:      make(map[string]int),
		UnprovenCount:   unprovenCount,
		GatePassThrough: float64(gatePassCount) / float64(totalCandidates),
	}

	// Calculate per-window metrics
	allScorePairs := []ScorePair{}
	for window, pairs := range windowGroups {
		metrics.SampleSize[window] = len(pairs)

		// Spearman correlation
		metrics.SpearmanCorrelations[window] = mc.calculateSpearmanCorrelation(pairs)

		// Hit rates
		metrics.HitRates.Unified[window] = mc.calculateHitRate(pairs, true)
		metrics.HitRates.Legacy[window] = mc.calculateHitRate(pairs, false)

		allScorePairs = append(allScorePairs, pairs...)
	}

	// Overall metrics
	metrics.DisagreementRate = mc.calculateDisagreementRate(allScorePairs)
	metrics.AverageScoreDelta = mc.calculateAverageScoreDelta(allScorePairs)

	return metrics
}

// groupByWindow groups score pairs by window for analysis
func (mc *MetricsCalculator) groupByWindow(scorePairs []ScorePair) map[string][]ScorePair {
	groups := make(map[string][]ScorePair)
	for _, pair := range scorePairs {
		groups[pair.Window] = append(groups[pair.Window], pair)
	}
	return groups
}

// calculateSpearmanCorrelation computes rank correlation between unified and legacy scores
func (mc *MetricsCalculator) calculateSpearmanCorrelation(pairs []ScorePair) float64 {
	if len(pairs) < 2 {
		return 0.0
	}

	// Create rank arrays
	unifiedRanks := mc.calculateRanks(pairs, func(p ScorePair) float64 { return p.UnifiedScore })
	legacyRanks := mc.calculateRanks(pairs, func(p ScorePair) float64 { return p.LegacyScore })

	// Calculate Spearman correlation
	return mc.correlation(unifiedRanks, legacyRanks)
}

// calculateRanks assigns ranks to scores (highest = rank 1)
func (mc *MetricsCalculator) calculateRanks(pairs []ScorePair, scoreFunc func(ScorePair) float64) []float64 {
	// Create indexed scores for sorting
	type indexedScore struct {
		index int
		score float64
	}

	indexedScores := make([]indexedScore, len(pairs))
	for i, pair := range pairs {
		indexedScores[i] = indexedScore{index: i, score: scoreFunc(pair)}
	}

	// Sort by score descending
	sort.Slice(indexedScores, func(i, j int) bool {
		return indexedScores[i].score > indexedScores[j].score
	})

	// Assign ranks
	ranks := make([]float64, len(pairs))
	for rank, item := range indexedScores {
		ranks[item.index] = float64(rank + 1)
	}

	return ranks
}

// correlation calculates Pearson correlation coefficient
func (mc *MetricsCalculator) correlation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0.0
	}

	n := float64(len(x))

	// Calculate means
	sumX, sumY := 0.0, 0.0
	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
	}
	meanX, meanY := sumX/n, sumY/n

	// Calculate correlation components
	numerator := 0.0
	sumSqX, sumSqY := 0.0, 0.0

	for i := 0; i < len(x); i++ {
		dx, dy := x[i]-meanX, y[i]-meanY
		numerator += dx * dy
		sumSqX += dx * dx
		sumSqY += dy * dy
	}

	denominator := math.Sqrt(sumSqX * sumSqY)
	if denominator == 0 {
		return 0.0
	}

	return numerator / denominator
}

// calculateHitRate computes hit rate for unified or legacy scores
func (mc *MetricsCalculator) calculateHitRate(pairs []ScorePair, unified bool) float64 {
	if len(pairs) == 0 {
		return 0.0
	}

	hits := 0
	for _, pair := range pairs {
		if unified && pair.UnifiedHit {
			hits++
		} else if !unified && pair.LegacyHit {
			hits++
		}
	}

	return float64(hits) / float64(len(pairs))
}

// calculateDisagreementRate computes the rate where systems disagree on >= 75 threshold
func (mc *MetricsCalculator) calculateDisagreementRate(pairs []ScorePair) float64 {
	if len(pairs) == 0 {
		return 0.0
	}

	disagreements := 0
	threshold := 75.0

	for _, pair := range pairs {
		unifiedAbove := pair.UnifiedScore >= threshold
		legacyAbove := pair.LegacyScore >= threshold

		// XOR - disagreement if one is above threshold and other is not
		if unifiedAbove != legacyAbove {
			disagreements++
		}
	}

	return float64(disagreements) / float64(len(pairs))
}

// calculateAverageScoreDelta computes mean absolute difference between scores
func (mc *MetricsCalculator) calculateAverageScoreDelta(pairs []ScorePair) float64 {
	if len(pairs) == 0 {
		return 0.0
	}

	totalDelta := 0.0
	for _, pair := range pairs {
		totalDelta += math.Abs(pair.UnifiedScore - pair.LegacyScore)
	}

	return totalDelta / float64(len(pairs))
}

// FormatMetricsSummary creates a console-friendly metrics summary
func FormatMetricsSummary(metrics BenchmarkMetrics, universeSize int) string {
	summary := "● BENCH — Legacy FactorWeights vs Unified"

	// Sample info
	if len(metrics.SampleSize) > 0 {
		var totalSamples int
		for _, count := range metrics.SampleSize {
			totalSamples += count
		}
		summary += fmt.Sprintf(" (n=%d windows; universe=%d; guards applied)\n", totalSamples, universeSize)
	}

	// Spearman correlations
	summary += "  Spearman ρ (scores):"
	for window, corr := range metrics.SpearmanCorrelations {
		summary += fmt.Sprintf(" %.2f (%s)", corr, window)
	}
	summary += "\n"

	// Hit rates
	summary += "  Hit-rate (Unified):"
	for window, rate := range metrics.HitRates.Unified {
		summary += fmt.Sprintf(" %d%% (%s)", int(rate*100), window)
	}
	summary += "\n"

	summary += "  Hit-rate (Legacy): "
	for window, rate := range metrics.HitRates.Legacy {
		summary += fmt.Sprintf(" %d%% (%s)", int(rate*100), window)
	}
	summary += "\n"

	// Summary stats
	summary += fmt.Sprintf("  Disagreement rate (≥75 threshold): %d%%\n", int(metrics.DisagreementRate*100))
	summary += fmt.Sprintf("  Avg |delta|: %.1f pts | Gate pass-through: %d%% | UNPROVEN micro: %d assets (excluded)",
		metrics.AverageScoreDelta, int(metrics.GatePassThrough*100), metrics.UnprovenCount)

	return summary
}
