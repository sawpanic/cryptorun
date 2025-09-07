package smoke90

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Writer handles writing backtest artifacts to disk
type Writer struct {
	outputDir string
	dateDir   string
}

// NewWriter creates a new artifact writer
func NewWriter(outputDir string) *Writer {
	dateDir := time.Now().Format("2006-01-02")
	fullOutputDir := filepath.Join(outputDir, dateDir)

	return &Writer{
		outputDir: fullOutputDir,
		dateDir:   dateDir,
	}
}

// GetOutputDir returns the full output directory path
func (w *Writer) GetOutputDir() string {
	return w.outputDir
}

// WriteResults writes the results to JSONL format
func (w *Writer) WriteResults(results *BacktestResults) error {
	// Ensure output directory exists
	if err := os.MkdirAll(w.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	resultsFile := filepath.Join(w.outputDir, "results.jsonl")
	file, err := os.Create(resultsFile)
	if err != nil {
		return fmt.Errorf("failed to create results file: %w", err)
	}
	defer file.Close()

	// Write each window result as a separate JSON line
	for _, window := range results.Windows {
		jsonData, err := json.Marshal(window)
		if err != nil {
			return fmt.Errorf("failed to marshal window result: %w", err)
		}

		if _, err := file.Write(jsonData); err != nil {
			return fmt.Errorf("failed to write window result: %w", err)
		}

		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}

	// Write summary as final line
	summaryData, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	if _, err := file.Write(summaryData); err != nil {
		return fmt.Errorf("failed to write summary: %w", err)
	}

	if _, err := file.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write final newline: %w", err)
	}

	return nil
}

// WriteReport writes a comprehensive markdown report
func (w *Writer) WriteReport(results *BacktestResults) error {
	reportFile := filepath.Join(w.outputDir, "report.md")
	file, err := os.Create(reportFile)
	if err != nil {
		return fmt.Errorf("failed to create report file: %w", err)
	}
	defer file.Close()

	report := w.generateMarkdownReport(results)

	if _, err := file.WriteString(report); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

// generateMarkdownReport generates the complete markdown report
func (w *Writer) generateMarkdownReport(results *BacktestResults) string {
	var report strings.Builder

	// Header
	report.WriteString(fmt.Sprintf("# Smoke90 Backtest Report\n\n"))
	report.WriteString(fmt.Sprintf("**Generated**: %s\n", time.Now().Format("2006-01-02 15:04:05 UTC")))
	report.WriteString(fmt.Sprintf("**Period**: %s to %s (90 days)\n",
		results.EndTime.Format("2006-01-02"), results.StartTime.Format("2006-01-02")))
	report.WriteString(fmt.Sprintf("**Configuration**: TopN=%d, Stride=%v, Hold=%v\n\n",
		results.Config.TopN, results.Config.Stride, results.Config.Hold))

	// Executive Summary
	report.WriteString("## Executive Summary\n\n")
	coverage := float64(results.ProcessedWindows) / float64(results.TotalWindows) * 100
	report.WriteString(fmt.Sprintf("- **Coverage**: %d/%d windows processed (%.1f%%)\n",
		results.ProcessedWindows, results.TotalWindows, coverage))
	report.WriteString(fmt.Sprintf("- **Total Candidates**: %d\n", results.Metrics.TotalCandidates))
	report.WriteString(fmt.Sprintf("- **Pass Rate**: %.1f%% (%d passed, %d failed)\n",
		results.Metrics.OverallPassRate, results.Metrics.PassedCandidates, results.Metrics.FailedCandidates))
	report.WriteString(fmt.Sprintf("- **Errors**: %d\n\n", results.Metrics.ErrorCount))

	// TopGainers Alignment
	if results.Metrics.TopGainersHitRate != nil {
		report.WriteString("## TopGainers Alignment\n\n")
		report.WriteString("| Timeframe | Hit Rate | Hits | Misses | Total |\n")
		report.WriteString("|-----------|----------|------|--------:|-------:|\n")

		hitRate := results.Metrics.TopGainersHitRate
		report.WriteString(fmt.Sprintf("| 1h | %.1f%% | %d | %d | %d |\n",
			hitRate.OneHour.HitRate, hitRate.OneHour.Hits, hitRate.OneHour.Misses, hitRate.OneHour.Total))
		report.WriteString(fmt.Sprintf("| 24h | %.1f%% | %d | %d | %d |\n",
			hitRate.TwentyFourHour.HitRate, hitRate.TwentyFourHour.Hits, hitRate.TwentyFourHour.Misses, hitRate.TwentyFourHour.Total))
		report.WriteString(fmt.Sprintf("| 7d | %.1f%% | %d | %d | %d |\n\n",
			hitRate.SevenDay.HitRate, hitRate.SevenDay.Hits, hitRate.SevenDay.Misses, hitRate.SevenDay.Total))
	}

	// Guard Attribution
	report.WriteString("## Guard Attribution\n\n")
	if len(results.Metrics.GuardStats) > 0 {
		report.WriteString("| Guard | Type | Pass Rate | Passed | Failed | Total |\n")
		report.WriteString("|-------|------|-----------|--------|--------|-------:|\n")

		for guardName, stat := range results.Metrics.GuardStats {
			guardType := stat.Type
			if guardType == "hard" {
				guardType = "🔴 Hard"
			} else {
				guardType = "🟡 Soft"
			}

			report.WriteString(fmt.Sprintf("| %s | %s | %.1f%% | %d | %d | %d |\n",
				guardName, guardType, stat.PassRate, stat.Passed, stat.Failed, stat.Total))
		}
		report.WriteString("\n")
	} else {
		report.WriteString("No guard statistics available.\n\n")
	}

	// Relaxation Events
	if results.Metrics.RelaxStats != nil && results.Metrics.RelaxStats.TotalEvents > 0 {
		report.WriteString("## P99 Relaxation Events\n\n")
		relax := results.Metrics.RelaxStats
		report.WriteString(fmt.Sprintf("- **Total Events**: %d\n", relax.TotalEvents))
		report.WriteString(fmt.Sprintf("- **Rate**: %.2f events per 100 signals\n", relax.EventsPer100))
		report.WriteString(fmt.Sprintf("- **Average P99 Latency**: %.1f ms\n", relax.AvgP99Ms))
		report.WriteString(fmt.Sprintf("- **Average Grace Period**: %.1f ms\n\n", relax.AvgGraceMs))
	}

	// Throttling Events
	if results.Metrics.ThrottleStats != nil && results.Metrics.ThrottleStats.TotalEvents > 0 {
		report.WriteString("## Provider Throttling\n\n")
		throttle := results.Metrics.ThrottleStats
		report.WriteString(fmt.Sprintf("- **Total Events**: %d\n", throttle.TotalEvents))
		report.WriteString(fmt.Sprintf("- **Rate**: %.2f events per 100 signals\n", throttle.EventsPer100))
		report.WriteString(fmt.Sprintf("- **Most Throttled**: %s\n\n", throttle.MostThrottled))

		if len(throttle.ProviderCounts) > 0 {
			report.WriteString("### Provider Breakdown\n\n")
			report.WriteString("| Provider | Events | Rate |\n")
			report.WriteString("|----------|--------:|------:|\n")

			for provider, count := range throttle.ProviderCounts {
				rate := float64(count) / float64(results.Metrics.TotalCandidates) * 100
				report.WriteString(fmt.Sprintf("| %s | %d | %.2f%% |\n", provider, count, rate))
			}
			report.WriteString("\n")
		}
	}

	// Skip Analysis
	if results.Metrics.SkipStats != nil && results.Metrics.SkipStats.TotalSkips > 0 {
		report.WriteString("## Skip Analysis\n\n")
		skip := results.Metrics.SkipStats
		report.WriteString(fmt.Sprintf("- **Total Skips**: %d windows\n", skip.TotalSkips))
		report.WriteString(fmt.Sprintf("- **Most Common**: %s\n\n", skip.MostCommon))

		if len(skip.SkipReasons) > 0 {
			report.WriteString("### Skip Reasons\n\n")
			report.WriteString("| Reason | Count | Rate |\n")
			report.WriteString("|--------|-------:|------:|\n")

			for reason, count := range skip.SkipReasons {
				rate := float64(count) / float64(results.TotalWindows) * 100
				report.WriteString(fmt.Sprintf("| %s | %d | %.1f%% |\n", reason, count, rate))
			}
			report.WriteString("\n")
		}
	}

	// Performance Analysis
	report.WriteString("## Performance Analysis\n\n")
	if len(results.Windows) > 0 {
		avgCandidatesPerWindow := float64(results.Metrics.TotalCandidates) / float64(len(results.Windows))
		report.WriteString(fmt.Sprintf("- **Average Candidates per Window**: %.1f\n", avgCandidatesPerWindow))
		report.WriteString(fmt.Sprintf("- **Windows with Results**: %d\n", len(results.Windows)))
		report.WriteString(fmt.Sprintf("- **Cache-Only Mode**: %t\n\n", results.Config.UseCache))
	}

	// Errors (if any)
	if results.Metrics.ErrorCount > 0 {
		report.WriteString("## Errors\n\n")
		report.WriteString(fmt.Sprintf("Total errors encountered: %d\n\n", results.Metrics.ErrorCount))

		if len(results.Metrics.Errors) > 0 {
			report.WriteString("### Most Frequent Errors\n\n")
			for i, err := range results.Metrics.Errors {
				if i >= 10 { // Limit to top 10
					break
				}
				report.WriteString(fmt.Sprintf("%d. %s\n", i+1, err))
			}
			report.WriteString("\n")
		}
	}

	// Methodology
	report.WriteString("## Methodology\n\n")
	report.WriteString("This smoke backtest validates the unified scanner end-to-end using only cached/cold data:\n\n")
	report.WriteString("1. **Unified Scoring**: Candidates must achieve Score ≥ 75\n")
	report.WriteString("2. **Hard Gates**: VADR ≥ 1.8× and funding divergence present\n")
	report.WriteString("3. **Guards Pipeline**: Freshness, fatigue, and late-fill guards with P99 relaxation\n")
	report.WriteString("4. **Microstructure Validation**: Spread/depth/VADR proofs across venues\n")
	report.WriteString("5. **Provider Operations**: Rate limiting and circuit breaker simulation\n")
	report.WriteString("6. **Cache-Only**: Zero live fetches, explicit SKIP reasons for gaps\n\n")

	// Limitations
	report.WriteString("## Limitations\n\n")
	report.WriteString("- Cache-only data may not reflect real-time conditions\n")
	report.WriteString("- Simulated PnL calculation based on cached price movements\n")
	report.WriteString("- Provider throttling and P99 latency events are simulated\n")
	report.WriteString("- Missing data windows are skipped rather than interpolated\n\n")

	// Artifacts
	report.WriteString("## Artifact Paths\n\n")
	report.WriteString(fmt.Sprintf("- **Results JSONL**: `%s`\n", filepath.Join(w.outputDir, "results.jsonl")))
	report.WriteString(fmt.Sprintf("- **Report Markdown**: `%s`\n", filepath.Join(w.outputDir, "report.md")))
	report.WriteString(fmt.Sprintf("- **Output Directory**: `%s`\n", w.outputDir))

	return report.String()
}

// WriteSummaryJSON writes a compact summary JSON file
func (w *Writer) WriteSummaryJSON(results *BacktestResults) error {
	summaryFile := filepath.Join(w.outputDir, "summary.json")
	file, err := os.Create(summaryFile)
	if err != nil {
		return fmt.Errorf("failed to create summary file: %w", err)
	}
	defer file.Close()

	// Create a compact summary
	summary := map[string]interface{}{
		"timestamp":        time.Now().Format(time.RFC3339),
		"period":           fmt.Sprintf("%s to %s", results.EndTime.Format("2006-01-02"), results.StartTime.Format("2006-01-02")),
		"coverage":         float64(results.ProcessedWindows) / float64(results.TotalWindows) * 100,
		"total_candidates": results.Metrics.TotalCandidates,
		"pass_rate":        results.Metrics.OverallPassRate,
		"error_count":      results.Metrics.ErrorCount,
		"artifacts": map[string]string{
			"results": filepath.Join(w.outputDir, "results.jsonl"),
			"report":  filepath.Join(w.outputDir, "report.md"),
			"summary": filepath.Join(w.outputDir, "summary.json"),
		},
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(summary); err != nil {
		return fmt.Errorf("failed to encode summary: %w", err)
	}

	return nil
}

// GetArtifactPaths returns the paths of all generated artifacts
func (w *Writer) GetArtifactPaths() *ArtifactPaths {
	return &ArtifactPaths{
		ResultsJSONL: filepath.Join(w.outputDir, "results.jsonl"),
		ReportMD:     filepath.Join(w.outputDir, "report.md"),
		OutputDir:    w.outputDir,
	}
}
