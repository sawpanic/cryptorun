package bench

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ScoreWeights defines weights for different aspects of alignment scoring
type ScoreWeights struct {
	SymbolOverlap   float64 `yaml:"symbol_overlap"`   // Weight for symbol overlap (0.6)
	RankCorrelation float64 `yaml:"rank_correlation"` // Weight for rank correlation (0.3)
	PercentageAlign float64 `yaml:"percentage_align"` // Weight for percentage alignment (0.1)
}

// DefaultScoreWeights returns default scoring weights
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		SymbolOverlap:   0.6,
		RankCorrelation: 0.3,
		PercentageAlign: 0.1,
	}
}

// CompositeScore calculates a comprehensive alignment score
type CompositeScore struct {
	OverallScore    float64            `json:"overall_score"`
	SymbolOverlap   float64            `json:"symbol_overlap"`
	RankCorrelation float64            `json:"rank_correlation"`
	PercentageAlign float64            `json:"percentage_align"`
	Weights         ScoreWeights       `json:"weights"`
	ComponentScores map[string]float64 `json:"component_scores"`
	Details         ScoreDetails       `json:"details"`
}

// ScoreDetails provides detailed breakdown of scoring
type ScoreDetails struct {
	CommonSymbols    []string            `json:"common_symbols"`
	OnlyInGainers    []string            `json:"only_in_gainers"`
	OnlyInScan       []string            `json:"only_in_scan"`
	RankDifferences  map[string]RankDiff `json:"rank_differences"`
	TopGainersCount  int                 `json:"top_gainers_count"`
	ScanResultsCount int                 `json:"scan_results_count"`
}

// RankDiff represents ranking difference for a symbol
type RankDiff struct {
	GainerRank int `json:"gainer_rank"`
	ScanRank   int `json:"scan_rank"`
	Difference int `json:"difference"`
}

// CalculateCompositeScore computes comprehensive alignment between gainers and scan results
func CalculateCompositeScore(window string, gainers []TopGainerResult, scanResults []string, weights ScoreWeights) CompositeScore {
	score := CompositeScore{
		Weights:         weights,
		ComponentScores: make(map[string]float64),
		Details: ScoreDetails{
			RankDifferences: make(map[string]RankDiff),
		},
	}

	// Normalize inputs
	gainerSymbols := make([]string, len(gainers))
	for i, g := range gainers {
		gainerSymbols[i] = strings.ToUpper(g.Symbol)
	}

	normalizedScan := make([]string, len(scanResults))
	for i, s := range scanResults {
		normalizedScan[i] = strings.ToUpper(s)
	}

	score.Details.TopGainersCount = len(gainerSymbols)
	score.Details.ScanResultsCount = len(normalizedScan)

	// 1. Calculate Symbol Overlap Score
	score.SymbolOverlap = calculateSymbolOverlap(gainerSymbols, normalizedScan, &score.Details)
	score.ComponentScores["symbol_overlap"] = score.SymbolOverlap

	// 2. Calculate Rank Correlation Score
	score.RankCorrelation = calculateRankCorrelation(gainerSymbols, normalizedScan, &score.Details)
	score.ComponentScores["rank_correlation"] = score.RankCorrelation

	// 3. Calculate Percentage Alignment Score (future enhancement)
	score.PercentageAlign = calculatePercentageAlignment(gainers, normalizedScan)
	score.ComponentScores["percentage_alignment"] = score.PercentageAlign

	// Calculate weighted overall score
	score.OverallScore = (score.SymbolOverlap * weights.SymbolOverlap) +
		(score.RankCorrelation * weights.RankCorrelation) +
		(score.PercentageAlign * weights.PercentageAlign)

	return score
}

// calculateSymbolOverlap computes the overlap between two symbol sets
func calculateSymbolOverlap(gainers, scan []string, details *ScoreDetails) float64 {
	if len(gainers) == 0 {
		return 0.0
	}

	gainerSet := make(map[string]bool)
	for _, symbol := range gainers {
		gainerSet[symbol] = true
	}

	scanSet := make(map[string]bool)
	for _, symbol := range scan {
		scanSet[symbol] = true
	}

	// Find common symbols
	common := []string{}
	for symbol := range gainerSet {
		if scanSet[symbol] {
			common = append(common, symbol)
		}
	}

	// Find symbols only in gainers
	onlyGainers := []string{}
	for symbol := range gainerSet {
		if !scanSet[symbol] {
			onlyGainers = append(onlyGainers, symbol)
		}
	}

	// Find symbols only in scan
	onlyScan := []string{}
	for symbol := range scanSet {
		if !gainerSet[symbol] {
			onlyScan = append(onlyScan, symbol)
		}
	}

	// Sort for consistent output
	sort.Strings(common)
	sort.Strings(onlyGainers)
	sort.Strings(onlyScan)

	details.CommonSymbols = common
	details.OnlyInGainers = onlyGainers
	details.OnlyInScan = onlyScan

	// Calculate Jaccard similarity coefficient
	intersection := len(common)
	union := len(gainerSet) + len(scanSet) - intersection

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// calculateRankCorrelation computes rank correlation for common symbols
func calculateRankCorrelation(gainers, scan []string, details *ScoreDetails) float64 {
	if len(gainers) == 0 || len(scan) == 0 {
		return 0.0
	}

	// Create rank mappings
	gainerRanks := make(map[string]int)
	for i, symbol := range gainers {
		gainerRanks[symbol] = i + 1 // 1-based ranking
	}

	scanRanks := make(map[string]int)
	for i, symbol := range scan {
		scanRanks[symbol] = i + 1
	}

	// Find common symbols and calculate rank differences
	commonSymbols := []string{}
	rankDifferences := []float64{}

	for _, symbol := range gainers {
		if scanRank, exists := scanRanks[symbol]; exists {
			commonSymbols = append(commonSymbols, symbol)

			gainerRank := gainerRanks[symbol]
			diff := gainerRank - scanRank
			rankDifferences = append(rankDifferences, float64(diff))

			details.RankDifferences[symbol] = RankDiff{
				GainerRank: gainerRank,
				ScanRank:   scanRank,
				Difference: abs(diff),
			}
		}
	}

	if len(commonSymbols) == 0 {
		return 0.0
	}

	// Calculate Spearman's rank correlation approximation
	// For simplicity, use normalized rank difference approach
	totalDifference := 0.0
	maxPossibleDiff := 0.0

	for i, diff := range rankDifferences {
		absDiff := math.Abs(diff)
		totalDifference += absDiff

		// Maximum possible difference for this position
		maxDiff := float64(max(len(gainers)-i-1, len(scan)-i-1))
		maxPossibleDiff += maxDiff
	}

	if maxPossibleDiff == 0 {
		return 1.0 // Perfect correlation if no variation possible
	}

	// Normalize to 0-1 scale (1 = perfect correlation, 0 = no correlation)
	correlation := 1.0 - (totalDifference / maxPossibleDiff)
	return math.Max(0.0, correlation)
}

// calculatePercentageAlignment computes alignment based on price change percentages
func calculatePercentageAlignment(gainers []TopGainerResult, scan []string) float64 {
	// This is a placeholder for future enhancement
	// Would compare the price change percentages of common symbols
	// with the momentum scores from our scanner

	if len(gainers) == 0 || len(scan) == 0 {
		return 0.0
	}

	// For now, return a neutral score
	// Future implementation would:
	// 1. Parse percentage changes from gainers
	// 2. Get momentum scores for common symbols
	// 3. Calculate correlation between percentage and momentum

	return 0.5 // Neutral placeholder score
}

// WindowComparison compares alignment across multiple time windows
type WindowComparison struct {
	Windows         []string                  `json:"windows"`
	CompositeScores map[string]CompositeScore `json:"composite_scores"`
	BestWindow      string                    `json:"best_window"`
	WorstWindow     string                    `json:"worst_window"`
	AverageScore    float64                   `json:"average_score"`
	ScoreVariance   float64                   `json:"score_variance"`
	Insights        []string                  `json:"insights"`
}

// CompareWindows performs cross-window analysis
func CompareWindows(windowResults map[string]CompositeScore) WindowComparison {
	comparison := WindowComparison{
		Windows:         make([]string, 0, len(windowResults)),
		CompositeScores: windowResults,
		Insights:        []string{},
	}

	if len(windowResults) == 0 {
		return comparison
	}

	scores := make([]float64, 0, len(windowResults))
	bestScore := -1.0
	worstScore := 2.0

	for window, result := range windowResults {
		comparison.Windows = append(comparison.Windows, window)
		score := result.OverallScore
		scores = append(scores, score)

		if score > bestScore {
			bestScore = score
			comparison.BestWindow = window
		}

		if score < worstScore {
			worstScore = score
			comparison.WorstWindow = window
		}
	}

	// Calculate average
	sum := 0.0
	for _, score := range scores {
		sum += score
	}
	comparison.AverageScore = sum / float64(len(scores))

	// Calculate variance
	variance := 0.0
	for _, score := range scores {
		diff := score - comparison.AverageScore
		variance += diff * diff
	}
	comparison.ScoreVariance = variance / float64(len(scores))

	// Generate insights
	comparison.Insights = generateInsights(comparison)

	sort.Strings(comparison.Windows) // Sort for consistent output

	return comparison
}

// generateInsights creates interpretive insights from the analysis
func generateInsights(comparison WindowComparison) []string {
	insights := []string{}

	// Average score insights
	if comparison.AverageScore > 0.7 {
		insights = append(insights, "High overall alignment: Scanner is well-aligned with market gainers")
	} else if comparison.AverageScore > 0.4 {
		insights = append(insights, "Moderate alignment: Scanner captures some but not all market movements")
	} else {
		insights = append(insights, "Low alignment: Scanner focuses on different opportunities than pure gainers")
	}

	// Variance insights
	if comparison.ScoreVariance < 0.01 {
		insights = append(insights, "Consistent performance across time windows")
	} else if comparison.ScoreVariance > 0.05 {
		insights = append(insights, "High variance across windows suggests time-horizon specific behavior")
	}

	// Best/worst window insights
	if comparison.BestWindow != "" && comparison.WorstWindow != "" {
		insights = append(insights, fmt.Sprintf("Best alignment in %s window, weakest in %s window",
			comparison.BestWindow, comparison.WorstWindow))
	}

	return insights
}

// Utility functions
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
