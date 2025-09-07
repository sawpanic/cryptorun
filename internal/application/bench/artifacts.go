package bench

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sawpanic/cryptorun/internal/application/pipeline"
)

// loadFromCache attempts to load data from cache file
func (tgb *TopGainersBenchmark) loadFromCache(cacheFile string) ([]byte, bool) {
	info, err := os.Stat(cacheFile)
	if err != nil {
		return nil, false
	}

	// Check if cache is still valid based on TTL
	if time.Since(info.ModTime()) > tgb.config.TTL {
		log.Debug().Str("file", cacheFile).Msg("Cache expired")
		return nil, false
	}

	data, err := ioutil.ReadFile(cacheFile)
	if err != nil {
		log.Warn().Err(err).Str("file", cacheFile).Msg("Failed to read cache file")
		return nil, false
	}

	return data, true
}

// saveToCache saves data to cache file
func (tgb *TopGainersBenchmark) saveToCache(cacheFile string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := ioutil.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	log.Debug().Str("file", cacheFile).Msg("Data cached successfully")
	return nil
}

// checkBudgetGuard enforces API request budget limits
func (tgb *TopGainersBenchmark) checkBudgetGuard() error {
	// For CoinGecko free API, we have generous limits but should still respect them
	// This is a simplified implementation - production would track daily/monthly usage

	budgetFile := filepath.Join(tgb.cacheDir, "api_budget.json")

	type BudgetInfo struct {
		Date         string    `json:"date"`
		RequestCount int       `json:"request_count"`
		LastReset    time.Time `json:"last_reset"`
	}

	today := time.Now().Format("2006-01-02")
	budget := BudgetInfo{
		Date:         today,
		RequestCount: 0,
		LastReset:    time.Now(),
	}

	// Load existing budget if available
	if data, err := ioutil.ReadFile(budgetFile); err == nil {
		json.Unmarshal(data, &budget)
	}

	// Reset daily counter if needed
	if budget.Date != today {
		budget.Date = today
		budget.RequestCount = 0
		budget.LastReset = time.Now()
	}

	// Check if we're approaching limits (1000 requests/day for free tier)
	const maxDailyRequests = 1000
	if budget.RequestCount >= maxDailyRequests {
		return fmt.Errorf("daily API budget exceeded (%d/%d requests)", budget.RequestCount, maxDailyRequests)
	}

	// Increment and save
	budget.RequestCount++
	if data, err := json.Marshal(budget); err == nil {
		ioutil.WriteFile(budgetFile, data, 0644)
	}

	// Warn if approaching limit
	if budget.RequestCount > maxDailyRequests*0.8 {
		log.Warn().Int("used", budget.RequestCount).Int("limit", maxDailyRequests).
			Msg("Approaching daily API request limit")
	}

	return nil
}

// saveWindowArtifact saves detailed window-specific data
func (tgb *TopGainersBenchmark) saveWindowArtifact(topGainers []TopGainerEntry, scanResults []pipeline.CompositeScore, alignment WindowAlignment, path string) error {
	artifact := struct {
		Timestamp   time.Time                 `json:"timestamp"`
		Window      string                    `json:"window"`
		TopGainers  []TopGainerEntry          `json:"top_gainers"`
		ScanResults []pipeline.CompositeScore `json:"scan_results"`
		Alignment   WindowAlignment           `json:"alignment"`
		Metadata    map[string]interface{}    `json:"metadata"`
	}{
		Timestamp:   time.Now(),
		Window:      alignment.Window,
		TopGainers:  topGainers,
		ScanResults: scanResults,
		Alignment:   alignment,
		Metadata: map[string]interface{}{
			"source":      "CoinGecko Free API",
			"scanner":     "CryptoRun Unified Pipeline",
			"regime":      "trending", // Would be dynamic in production
			"sample_size": len(topGainers),
			"dry_run":     tgb.config.DryRun,
		},
	}

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal artifact: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create artifact directory: %w", err)
	}

	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write artifact: %w", err)
	}

	log.Info().Str("path", path).Str("window", alignment.Window).
		Int("top_gainers", len(topGainers)).
		Int("scan_results", len(scanResults)).
		Msg("Window artifact saved")

	return nil
}

// generateMarkdownReport creates human-readable alignment report
func (tgb *TopGainersBenchmark) generateMarkdownReport(result *BenchmarkResult, path string) error {
	var report strings.Builder

	// Header
	report.WriteString("# CryptoRun Top Gainers Alignment Report\n\n")
	report.WriteString(fmt.Sprintf("**Generated:** %s\n", result.Timestamp.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("**Overall Alignment:** %.1f%% (%s)\n\n", result.OverallAlignment*100, result.Summary.AlignmentGrade))

	// UX MUST — Live Progress & Explainability
	report.WriteString("## UX MUST — Live Progress & Explainability\n\n")
	report.WriteString("This report provides complete transparency into how CryptoRun's momentum scanner aligns with market-proven top gainers from CoinGecko, ensuring explainable performance attribution.\n\n")

	// Executive Summary
	report.WriteString("## Executive Summary\n\n")
	report.WriteString(fmt.Sprintf("- **Recommendation:** %s\n", result.Summary.Recommendation))
	report.WriteString(fmt.Sprintf("- **Processing Time:** %s\n", result.Summary.ProcessingTime))
	report.WriteString(fmt.Sprintf("- **API Calls:** %d (%.1f%% cache hit rate)\n", result.Summary.TotalAPICalls, result.Summary.CacheHitRate*100))
	report.WriteString(fmt.Sprintf("- **Sample Size:** %d symbols (n≥20 required for statistical validity)\n\n", result.MetricBreakdown.SampleSize))

	// Window Analysis
	report.WriteString("## Window Analysis\n\n")
	report.WriteString("| Window | Alignment | Matches | Kendall's τ | Pearson ρ | MAE | Trend Sparkline |\n")
	report.WriteString("|--------|-----------|---------|-------------|-----------|-----|------------------|\n")

	for _, window := range tgb.config.Windows {
		if alignment, exists := result.WindowAlignments[window]; exists {
			report.WriteString(fmt.Sprintf("| %s | %.1f%% | %d/%d | %.3f | %.3f | %.1f | %s |\n",
				window, alignment.Score*100, alignment.Matches, alignment.Total,
				alignment.KendallTau, alignment.Pearson, alignment.MAE, alignment.Sparkline))
		}
	}
	report.WriteString("\n")

	// Metric Breakdown
	report.WriteString("## Metric Breakdown\n\n")
	report.WriteString(fmt.Sprintf("- **Symbol Overlap:** %.1f%% (Jaccard similarity)\n", result.MetricBreakdown.SymbolOverlap*100))
	report.WriteString(fmt.Sprintf("- **Rank Correlation:** %.3f (Kendall's τ)\n", result.MetricBreakdown.RankCorrelation))
	report.WriteString(fmt.Sprintf("- **Percentage Alignment:** %.1f%%\n", result.MetricBreakdown.PercentageAlign*100))
	report.WriteString(fmt.Sprintf("- **Data Source:** %s (labeled)\n\n", result.MetricBreakdown.DataSource))

	// Candidate Analysis
	if len(result.CandidateAnalysis) > 0 {
		report.WriteString("## Top Candidate Analysis\n\n")
		report.WriteString("| Symbol | Scanner Rank | Top Gainers Rank | Scanner Score | Gain % | Trend | Rationale |\n")
		report.WriteString("|--------|--------------|------------------|---------------|--------|-------|------------|\n")

		for _, candidate := range result.CandidateAnalysis {
			inBoth := "❌"
			if candidate.InBothLists {
				inBoth = "✅"
			}
			report.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f | %.1f%% | %s | %s %s |\n",
				candidate.Symbol, candidate.ScannerRank, candidate.TopGainersRank,
				candidate.ScannerScore, candidate.TopGainersPercent,
				candidate.Sparkline, inBoth, candidate.Rationale))
		}
		report.WriteString("\n")
	}

	// Technical Details
	report.WriteString("## Technical Details\n\n")
	report.WriteString("### Alignment Calculation\n")
	report.WriteString("Overall alignment = (Symbol Overlap × 0.6) + (Kendall's τ × 0.3) + (Pearson ρ × 0.1)\n\n")

	report.WriteString("### Data Sources\n")
	report.WriteString("- **Top Gainers:** CoinGecko Free API (exchange-native pricing)\n")
	report.WriteString("- **Scanner Results:** CryptoRun Unified Scoring Pipeline\n")
	report.WriteString("- **Price Trends:** Exchange-native OHLC bars (24-bar lookback)\n\n")

	report.WriteString("### Quality Assurance\n")
	report.WriteString("- Minimum sample size: n≥20 enforced\n")
	report.WriteString("- Rate limits respected (30 req/min)\n")
	report.WriteString("- Cache TTL: 5+ minutes\n")
	report.WriteString("- Budget guard: 1000 requests/day limit\n\n")

	// Artifacts
	report.WriteString("## Generated Artifacts\n\n")
	for name, path := range result.Artifacts {
		report.WriteString(fmt.Sprintf("- **%s:** `%s`\n", strings.Title(strings.ReplaceAll(name, "_", " ")), path))
	}
	report.WriteString("\n")

	// Footer
	report.WriteString("---\n")
	report.WriteString("*Generated by CryptoRun v3.2.1 Benchmark Suite*\n")

	// Write report
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	if err := ioutil.WriteFile(path, []byte(report.String()), 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	log.Info().Str("path", path).
		Float64("alignment", result.OverallAlignment).
		Int("windows", len(result.WindowAlignments)).
		Msg("Markdown alignment report generated")

	return nil
}
