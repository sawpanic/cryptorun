package empirical

import (
	"encoding/json"
	"math"
	"os"
	"sort"
	"testing"
)

// SyntheticPanelEntry represents a single asset in the synthetic test panel
type SyntheticPanelEntry struct {
	Symbol                  string  `json:"symbol"`
	Timestamp               string  `json:"timestamp"`
	CompositeScore          float64 `json:"composite_score"`
	Regime                  string  `json:"regime"`
	ForwardReturn4h         float64 `json:"forward_return_4h"`
	ForwardReturn24h        float64 `json:"forward_return_24h"`
	Decile                  int     `json:"decile"`
	EntryPrice              float64 `json:"entry_price"`
	ExitPrice4h             float64 `json:"exit_price_4h"`
	ExitPrice24h            float64 `json:"exit_price_24h"`
	VADR                    float64 `json:"vadr"`
	FundingDivergenceZScore float64 `json:"funding_divergence_zscore"`
	MeetsGates              bool    `json:"meets_gates"`
}

// DecileAnalysis holds the results of decile lift analysis
type DecileAnalysis struct {
	Decile              int
	Count               int
	AvgCompositeScore   float64
	AvgForwardReturn4h  float64
	AvgForwardReturn24h float64
	MinScore            float64
	MaxScore            float64
}

func TestDecileLift_MonotonicityCheck(t *testing.T) {
	// Load synthetic panel data
	panel := loadSyntheticPanel(t)

	// Group by decile and calculate statistics
	decileStats := calculateDecileStatistics(panel)

	// Sort by decile to ensure proper order
	sort.Slice(decileStats, func(i, j int) bool {
		return decileStats[i].Decile < decileStats[j].Decile
	})

	// Check monotonicity: higher deciles should have higher average returns
	violationsCount4h := 0
	violationsCount24h := 0

	for i := 1; i < len(decileStats); i++ {
		prevDecile := decileStats[i-1]
		currDecile := decileStats[i]

		// 4h return monotonicity check
		if currDecile.AvgForwardReturn4h <= prevDecile.AvgForwardReturn4h {
			violationsCount4h++
			t.Logf("4h monotonicity violation: decile %d (%.4f) <= decile %d (%.4f)",
				currDecile.Decile, currDecile.AvgForwardReturn4h,
				prevDecile.Decile, prevDecile.AvgForwardReturn4h)
		}

		// 24h return monotonicity check
		if currDecile.AvgForwardReturn24h <= prevDecile.AvgForwardReturn24h {
			violationsCount24h++
			t.Logf("24h monotonicity violation: decile %d (%.4f) <= decile %d (%.4f)",
				currDecile.Decile, currDecile.AvgForwardReturn24h,
				prevDecile.Decile, prevDecile.AvgForwardReturn24h)
		}
	}

	// Acceptance criteria: ≥8/10 deciles should maintain monotonicity (≤2 violations)
	maxAllowedViolations := 2

	if violationsCount4h > maxAllowedViolations {
		t.Errorf("4h return monotonicity failed: %d violations > %d allowed", violationsCount4h, maxAllowedViolations)
	}

	if violationsCount24h > maxAllowedViolations {
		t.Errorf("24h return monotonicity failed: %d violations > %d allowed", violationsCount24h, maxAllowedViolations)
	}

	t.Logf("Monotonicity check passed: 4h violations=%d, 24h violations=%d (max allowed=%d)",
		violationsCount4h, violationsCount24h, maxAllowedViolations)
}

func TestDecileLift_TopBottomSpread(t *testing.T) {
	panel := loadSyntheticPanel(t)
	decileStats := calculateDecileStatistics(panel)

	// Sort by decile
	sort.Slice(decileStats, func(i, j int) bool {
		return decileStats[i].Decile < decileStats[j].Decile
	})

	if len(decileStats) < 2 {
		t.Fatal("insufficient deciles for spread analysis")
	}

	bottomDecile := decileStats[0]               // Decile 1
	topDecile := decileStats[len(decileStats)-1] // Decile 10

	// Calculate spreads
	spread4h := topDecile.AvgForwardReturn4h - bottomDecile.AvgForwardReturn4h
	spread24h := topDecile.AvgForwardReturn24h - bottomDecile.AvgForwardReturn24h

	// Minimum expected spread thresholds (indicative of signal quality)
	minSpread4h := 0.020  // 2.0% minimum spread
	minSpread24h := 0.035 // 3.5% minimum spread

	if spread4h < minSpread4h {
		t.Errorf("4h top-bottom spread %.3f%% below minimum %.3f%%", spread4h*100, minSpread4h*100)
	}

	if spread24h < minSpread24h {
		t.Errorf("24h top-bottom spread %.3f%% below minimum %.3f%%", spread24h*100, minSpread24h*100)
	}

	t.Logf("Decile spreads: 4h=%.3f%%, 24h=%.3f%% (top decile %d vs bottom decile %d)",
		spread4h*100, spread24h*100, topDecile.Decile, bottomDecile.Decile)
}

func TestDecileLift_ScoreCompositeAlignment(t *testing.T) {
	panel := loadSyntheticPanel(t)
	decileStats := calculateDecileStatistics(panel)

	// Check that average composite scores increase with decile
	sort.Slice(decileStats, func(i, j int) bool {
		return decileStats[i].Decile < decileStats[j].Decile
	})

	scoreViolations := 0
	for i := 1; i < len(decileStats); i++ {
		prevDecile := decileStats[i-1]
		currDecile := decileStats[i]

		if currDecile.AvgCompositeScore <= prevDecile.AvgCompositeScore {
			scoreViolations++
			t.Logf("Score monotonicity violation: decile %d (%.2f) <= decile %d (%.2f)",
				currDecile.Decile, currDecile.AvgCompositeScore,
				prevDecile.Decile, prevDecile.AvgCompositeScore)
		}
	}

	// Composite scores should be perfectly monotonic (0 violations allowed)
	if scoreViolations > 0 {
		t.Errorf("composite score monotonicity failed: %d violations", scoreViolations)
	}
}

func TestDecileLift_RegimeSpecificAnalysis(t *testing.T) {
	panel := loadSyntheticPanel(t)

	// Group by regime
	regimeGroups := make(map[string][]SyntheticPanelEntry)
	for _, entry := range panel {
		regimeGroups[entry.Regime] = append(regimeGroups[entry.Regime], entry)
	}

	for regime, entries := range regimeGroups {
		if len(entries) < 3 {
			t.Logf("Skipping regime %s (insufficient entries: %d)", regime, len(entries))
			continue
		}

		t.Run(regime+"_regime", func(t *testing.T) {
			// Sort by composite score within regime
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].CompositeScore < entries[j].CompositeScore
			})

			// Check that higher scores correlate with higher returns within regime
			topThird := entries[len(entries)*2/3:]
			bottomThird := entries[:len(entries)/3]

			if len(topThird) == 0 || len(bottomThird) == 0 {
				t.Skip("insufficient entries for top/bottom third analysis")
			}

			topAvgReturn := calculateAvgReturn(topThird, "4h")
			bottomAvgReturn := calculateAvgReturn(bottomThird, "4h")

			if topAvgReturn <= bottomAvgReturn {
				t.Errorf("regime %s: top third avg return (%.3f) <= bottom third avg return (%.3f)",
					regime, topAvgReturn, bottomAvgReturn)
			}

			t.Logf("Regime %s: top third %.3f%% vs bottom third %.3f%% (spread: %.3f%%)",
				regime, topAvgReturn*100, bottomAvgReturn*100, (topAvgReturn-bottomAvgReturn)*100)
		})
	}
}

func TestDecileLift_StatisticalSignificance(t *testing.T) {
	panel := loadSyntheticPanel(t)

	// Simple statistical significance test using t-test approximation
	// Compare top decile vs bottom decile returns

	topDecileEntries := filterByDecile(panel, 10)
	bottomDecileEntries := filterByDecile(panel, 1)

	if len(topDecileEntries) == 0 || len(bottomDecileEntries) == 0 {
		t.Skip("insufficient entries for statistical test")
	}

	topReturns := extractReturns(topDecileEntries, "4h")
	bottomReturns := extractReturns(bottomDecileEntries, "4h")

	topMean := calculateMean(topReturns)
	bottomMean := calculateMean(bottomReturns)
	topStd := calculateStdDev(topReturns, topMean)
	bottomStd := calculateStdDev(bottomReturns, bottomMean)

	// Simple two-sample t-test statistic
	pooledStd := math.Sqrt((topStd*topStd + bottomStd*bottomStd) / 2)
	if pooledStd == 0 {
		t.Skip("zero pooled standard deviation, cannot perform t-test")
	}

	tStat := (topMean - bottomMean) / (pooledStd * math.Sqrt(2.0/float64(len(topReturns))))

	// For a crude significance test, |t| > 2 suggests p < 0.05
	minTStat := 2.0

	if math.Abs(tStat) < minTStat {
		t.Logf("Warning: t-statistic %.3f < %.1f, difference may not be statistically significant",
			math.Abs(tStat), minTStat)
	} else {
		t.Logf("Statistical significance: t-statistic=%.3f (>%.1f), difference likely significant",
			math.Abs(tStat), minTStat)
	}
}

// Helper functions

func loadSyntheticPanel(t *testing.T) []SyntheticPanelEntry {
	data, err := os.ReadFile("../../../testdata/tuner/synthetic_panel.json")
	if err != nil {
		t.Fatalf("failed to read synthetic panel: %v", err)
	}

	var panel []SyntheticPanelEntry
	if err := json.Unmarshal(data, &panel); err != nil {
		t.Fatalf("failed to unmarshal synthetic panel: %v", err)
	}

	return panel
}

func calculateDecileStatistics(panel []SyntheticPanelEntry) []DecileAnalysis {
	decileGroups := make(map[int][]SyntheticPanelEntry)

	for _, entry := range panel {
		decileGroups[entry.Decile] = append(decileGroups[entry.Decile], entry)
	}

	var stats []DecileAnalysis
	for decile, entries := range decileGroups {
		if len(entries) == 0 {
			continue
		}

		analysis := DecileAnalysis{
			Decile: decile,
			Count:  len(entries),
		}

		// Calculate averages
		scoreSum, return4hSum, return24hSum := 0.0, 0.0, 0.0
		minScore, maxScore := entries[0].CompositeScore, entries[0].CompositeScore

		for _, entry := range entries {
			scoreSum += entry.CompositeScore
			return4hSum += entry.ForwardReturn4h
			return24hSum += entry.ForwardReturn24h

			if entry.CompositeScore < minScore {
				minScore = entry.CompositeScore
			}
			if entry.CompositeScore > maxScore {
				maxScore = entry.CompositeScore
			}
		}

		count := float64(len(entries))
		analysis.AvgCompositeScore = scoreSum / count
		analysis.AvgForwardReturn4h = return4hSum / count
		analysis.AvgForwardReturn24h = return24hSum / count
		analysis.MinScore = minScore
		analysis.MaxScore = maxScore

		stats = append(stats, analysis)
	}

	return stats
}

func calculateAvgReturn(entries []SyntheticPanelEntry, timeframe string) float64 {
	if len(entries) == 0 {
		return 0
	}

	sum := 0.0
	for _, entry := range entries {
		if timeframe == "4h" {
			sum += entry.ForwardReturn4h
		} else {
			sum += entry.ForwardReturn24h
		}
	}

	return sum / float64(len(entries))
}

func filterByDecile(panel []SyntheticPanelEntry, decile int) []SyntheticPanelEntry {
	var filtered []SyntheticPanelEntry
	for _, entry := range panel {
		if entry.Decile == decile {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func extractReturns(entries []SyntheticPanelEntry, timeframe string) []float64 {
	var returns []float64
	for _, entry := range entries {
		if timeframe == "4h" {
			returns = append(returns, entry.ForwardReturn4h)
		} else {
			returns = append(returns, entry.ForwardReturn24h)
		}
	}
	return returns
}

func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateStdDev(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}

	sumSquaredDiffs := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquaredDiffs += diff * diff
	}

	return math.Sqrt(sumSquaredDiffs / float64(len(values)-1))
}
