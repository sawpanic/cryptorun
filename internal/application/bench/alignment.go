package bench

import (
	"math"
	"strings"

	"github.com/sawpanic/cryptorun/internal/application/pipeline"
)

// calculateAlignment computes alignment metrics between top gainers and scanner results
func (tgb *TopGainersBenchmark) calculateAlignment(topGainers []TopGainerEntry, scanResults []pipeline.CompositeScore, window string) WindowAlignment {
	// Convert to comparable formats
	topGainerSymbols := make(map[string]int)
	for i, entry := range topGainers {
		topGainerSymbols[entry.Symbol] = i + 1
	}

	scannerSymbols := make(map[string]int)
	for i, result := range scanResults {
		scannerSymbols[result.Symbol] = i + 1
	}

	// Calculate symbol overlap (Jaccard similarity)
	overlap := tgb.calculateSymbolOverlap(topGainerSymbols, scannerSymbols)

	// Calculate rank correlations
	kendallTau := tgb.calculateKendallTau(topGainerSymbols, scannerSymbols)
	pearson := tgb.calculatePearsonCorrelation(topGainers, scanResults)

	// Calculate Mean Absolute Error of ranks
	mae := tgb.calculateMAE(topGainerSymbols, scannerSymbols)

	// Generate sparkline for this window's top gainers
	sparkline := tgb.generateSparkline(topGainers)

	// Calculate composite alignment score
	score := (overlap * 0.6) + (math.Max(0, kendallTau) * 0.3) + (math.Max(0, pearson) * 0.1)

	matches := len(tgb.getCommonSymbols(topGainerSymbols, scannerSymbols))
	total := len(topGainers)

	return WindowAlignment{
		Window:     window,
		Score:      score,
		Matches:    matches,
		Total:      total,
		KendallTau: kendallTau,
		Pearson:    pearson,
		MAE:        mae,
		Sparkline:  sparkline,
	}
}

// calculateSymbolOverlap computes Jaccard similarity between two symbol sets
func (tgb *TopGainersBenchmark) calculateSymbolOverlap(set1, set2 map[string]int) float64 {
	intersection := 0
	union := make(map[string]bool)

	// Add all symbols to union
	for symbol := range set1 {
		union[symbol] = true
	}
	for symbol := range set2 {
		union[symbol] = true
	}

	// Count intersection
	for symbol := range set1 {
		if _, exists := set2[symbol]; exists {
			intersection++
		}
	}

	if len(union) == 0 {
		return 0.0
	}

	return float64(intersection) / float64(len(union))
}

// calculateKendallTau computes Kendall's τ rank correlation
func (tgb *TopGainersBenchmark) calculateKendallTau(ranks1, ranks2 map[string]int) float64 {
	common := tgb.getCommonSymbols(ranks1, ranks2)
	if len(common) < 2 {
		return 0.0
	}

	concordant := 0
	discordant := 0

	for i := 0; i < len(common); i++ {
		for j := i + 1; j < len(common); j++ {
			symbol1, symbol2 := common[i], common[j]

			rank1_diff := ranks1[symbol1] - ranks1[symbol2]
			rank2_diff := ranks2[symbol1] - ranks2[symbol2]

			if (rank1_diff > 0 && rank2_diff > 0) || (rank1_diff < 0 && rank2_diff < 0) {
				concordant++
			} else if (rank1_diff > 0 && rank2_diff < 0) || (rank1_diff < 0 && rank2_diff > 0) {
				discordant++
			}
		}
	}

	total_pairs := concordant + discordant
	if total_pairs == 0 {
		return 0.0
	}

	return float64(concordant-discordant) / float64(total_pairs)
}

// calculatePearsonCorrelation computes Pearson correlation between percentage gains and scanner scores
func (tgb *TopGainersBenchmark) calculatePearsonCorrelation(topGainers []TopGainerEntry, scanResults []pipeline.CompositeScore) float64 {
	// Create lookup maps
	gainMap := make(map[string]float64)
	for _, entry := range topGainers {
		gainMap[entry.Symbol] = entry.PriceChangePercentage
	}

	scoreMap := make(map[string]float64)
	for _, result := range scanResults {
		scoreMap[result.Symbol] = result.Score
	}

	// Get common symbols and their values
	var gains, scores []float64
	for symbol, gain := range gainMap {
		if score, exists := scoreMap[symbol]; exists {
			gains = append(gains, gain)
			scores = append(scores, score)
		}
	}

	if len(gains) < 2 {
		return 0.0
	}

	// Calculate Pearson correlation
	return tgb.pearsonCorrelation(gains, scores)
}

// pearsonCorrelation computes Pearson correlation coefficient
func (tgb *TopGainersBenchmark) pearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0.0
	}

	n := float64(len(x))

	// Calculate means
	meanX, meanY := 0.0, 0.0
	for i := 0; i < len(x); i++ {
		meanX += x[i]
		meanY += y[i]
	}
	meanX /= n
	meanY /= n

	// Calculate correlation
	numerator := 0.0
	sumXX := 0.0
	sumYY := 0.0

	for i := 0; i < len(x); i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		numerator += dx * dy
		sumXX += dx * dx
		sumYY += dy * dy
	}

	denominator := math.Sqrt(sumXX * sumYY)
	if denominator == 0 {
		return 0.0
	}

	return numerator / denominator
}

// calculateMAE computes Mean Absolute Error between ranks
func (tgb *TopGainersBenchmark) calculateMAE(ranks1, ranks2 map[string]int) float64 {
	common := tgb.getCommonSymbols(ranks1, ranks2)
	if len(common) == 0 {
		return math.Inf(1)
	}

	totalError := 0.0
	for _, symbol := range common {
		rank1 := ranks1[symbol]
		rank2 := ranks2[symbol]
		totalError += math.Abs(float64(rank1 - rank2))
	}

	return totalError / float64(len(common))
}

// getCommonSymbols returns symbols present in both maps
func (tgb *TopGainersBenchmark) getCommonSymbols(map1, map2 map[string]int) []string {
	var common []string
	for symbol := range map1 {
		if _, exists := map2[symbol]; exists {
			common = append(common, symbol)
		}
	}
	return common
}

// generateSparkline creates a Unicode sparkline from top gainers percentages
func (tgb *TopGainersBenchmark) generateSparkline(topGainers []TopGainerEntry) string {
	if len(topGainers) == 0 {
		return ""
	}

	// Extract percentages
	values := make([]float64, len(topGainers))
	for i, entry := range topGainers {
		values[i] = entry.PriceChangePercentage
	}

	// Find min/max for scaling
	minVal, maxVal := values[0], values[0]
	for _, val := range values {
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}

	// Sparkline characters (8 levels)
	chars := "▁▂▃▄▅▆▇█"

	if maxVal == minVal {
		return strings.Repeat("▄", len(values)) // Flat line
	}

	sparkline := ""
	for _, val := range values {
		// Scale to 0-7 range
		normalized := (val - minVal) / (maxVal - minVal)
		index := int(normalized * 7)
		if index > 7 {
			index = 7
		}
		sparkline += string(chars[index])
	}

	return sparkline
}

// enrichWithSparklines adds price trend sparklines to top gainers
func (tgb *TopGainersBenchmark) enrichWithSparklines(topGainers []TopGainerEntry, window string) {
	// For now, generate mock sparklines based on percentage
	// In production, this would fetch actual price bars from exchange-native APIs

	for i := range topGainers {
		// Mock sparkline based on gain percentage
		gain := topGainers[i].PriceChangePercentage
		if gain > 10 {
			topGainers[i].Name += " ▁▂▄▆█" // Strong uptrend
		} else if gain > 5 {
			topGainers[i].Name += " ▂▃▅▆▇" // Moderate uptrend
		} else if gain > 0 {
			topGainers[i].Name += " ▃▄▄▅▆" // Mild uptrend
		} else {
			topGainers[i].Name += " ▅▄▃▂▁" // Downtrend
		}
	}
}

// calculateOverallAlignment computes weighted average alignment across windows
func (tgb *TopGainersBenchmark) calculateOverallAlignment(alignments map[string]WindowAlignment) float64 {
	if len(alignments) == 0 {
		return 0.0
	}

	// Weight windows: 1h=0.3, 24h=0.7
	weights := map[string]float64{
		"1h":  0.3,
		"24h": 0.7,
		"7d":  0.2,
	}

	totalScore := 0.0
	totalWeight := 0.0

	for window, alignment := range alignments {
		weight := weights[window]
		if weight == 0 {
			weight = 1.0 / float64(len(alignments)) // Equal weight if not specified
		}

		totalScore += alignment.Score * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0.0
	}

	return totalScore / totalWeight
}

// generateMetricBreakdown creates detailed metric analysis
func (tgb *TopGainersBenchmark) generateMetricBreakdown(alignments map[string]WindowAlignment, apiCalls int) MetricBreakdown {
	// Calculate averages across windows
	avgOverlap := 0.0
	avgCorrelation := 0.0
	avgPercentAlign := 0.0
	totalSamples := 0

	for _, alignment := range alignments {
		avgOverlap += alignment.Score * 0.6 // Extract overlap component
		avgCorrelation += alignment.KendallTau
		avgPercentAlign += float64(alignment.Matches) / float64(alignment.Total)
		totalSamples += alignment.Total
	}

	count := float64(len(alignments))
	if count > 0 {
		avgOverlap /= count
		avgCorrelation /= count
		avgPercentAlign /= count
	}

	return MetricBreakdown{
		SymbolOverlap:   avgOverlap,
		RankCorrelation: avgCorrelation,
		PercentageAlign: avgPercentAlign,
		SampleSize:      totalSamples / len(alignments),
		DataSource:      "CoinGecko Free API",
	}
}

// generateCandidateAnalysis creates per-symbol rationale
func (tgb *TopGainersBenchmark) generateCandidateAnalysis(alignments map[string]WindowAlignment) []CandidateAnalysis {
	// This would be populated with actual candidate data in production
	// For now, return a placeholder
	return []CandidateAnalysis{
		{
			Symbol:            "BTCUSD",
			ScannerRank:       1,
			TopGainersRank:    2,
			ScannerScore:      90.5,
			TopGainersPercent: 8.3,
			InBothLists:       true,
			RankDifference:    1,
			Rationale:         "High momentum score aligns with top gainer performance",
			Sparkline:         "▁▂▄▆█",
		},
	}
}

// generateRecommendation provides actionable recommendation based on alignment
func (tgb *TopGainersBenchmark) generateRecommendation(overallAlignment float64) string {
	if overallAlignment >= 0.8 {
		return "Excellent alignment - scanner effectively identifies market leaders"
	} else if overallAlignment >= 0.6 {
		return "Good alignment - consider fine-tuning social/volume weights"
	} else if overallAlignment >= 0.4 {
		return "Fair alignment - review momentum timeframe weights and regime detection"
	} else {
		return "Poor alignment - significant recalibration needed for current market regime"
	}
}

// getAlignmentGrade converts numerical alignment to letter grade
func (tgb *TopGainersBenchmark) getAlignmentGrade(alignment float64) string {
	if alignment >= 0.9 {
		return "A+"
	} else if alignment >= 0.8 {
		return "A"
	} else if alignment >= 0.7 {
		return "B+"
	} else if alignment >= 0.6 {
		return "B"
	} else if alignment >= 0.5 {
		return "C+"
	} else if alignment >= 0.4 {
		return "C"
	} else if alignment >= 0.3 {
		return "D"
	} else {
		return "F"
	}
}
