package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"cryptorun/internal/backtest/smoke90"
)

// runBacktestSmoke90 executes the 90-day smoke backtest
func runBacktestSmoke90(cmd *cobra.Command, args []string) error {
	// Parse flags
	topN, _ := cmd.Flags().GetInt("top-n")
	stride, _ := cmd.Flags().GetDuration("stride")
	hold, _ := cmd.Flags().GetDuration("hold")
	outputDir, _ := cmd.Flags().GetString("output")
	useCache, _ := cmd.Flags().GetBool("use-cache")
	progress, _ := cmd.Flags().GetString("progress")

	// Validate parameters
	if topN <= 0 {
		return fmt.Errorf("top-n must be positive, got: %d", topN)
	}
	if stride <= 0 {
		return fmt.Errorf("stride must be positive, got: %v", stride)
	}
	if hold <= 0 {
		return fmt.Errorf("hold must be positive, got: %v", hold)
	}

	// Create absolute output path
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("failed to resolve output directory: %w", err)
	}

	log.Info().
		Int("top_n", topN).
		Dur("stride", stride).
		Dur("hold", hold).
		Str("output_dir", absOutputDir).
		Bool("use_cache", useCache).
		Str("progress", progress).
		Msg("Starting smoke90 backtest")

	// Create configuration
	config := &smoke90.Config{
		TopN:     topN,
		Stride:   stride,
		Hold:     hold,
		UseCache: useCache,
	}

	// Create runner
	runner := smoke90.NewRunner(config, absOutputDir)

	// Set up context with timeout (30 minutes should be enough for cached data)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Run the backtest
	fmt.Printf("🔍 Running smoke90 backtest (90-day cache-only validation)...\n")
	fmt.Printf("   Configuration: TopN=%d, Stride=%v, Hold=%v\n", topN, stride, hold)
	fmt.Printf("   Output: %s\n", absOutputDir)
	fmt.Printf("   Use Cache Only: %t\n\n", useCache)

	results, err := runner.Run(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Smoke90 backtest failed")
		return fmt.Errorf("smoke90 backtest failed: %w", err)
	}

	// Create writer for artifacts
	writer := smoke90.NewWriter(absOutputDir)

	// Write results to JSONL
	if err := writer.WriteResults(results); err != nil {
		log.Warn().Err(err).Msg("Failed to write JSONL results")
	}

	// Write comprehensive report
	if err := writer.WriteReport(results); err != nil {
		log.Warn().Err(err).Msg("Failed to write markdown report")
	}

	// Write compact summary
	if err := writer.WriteSummaryJSON(results); err != nil {
		log.Warn().Err(err).Msg("Failed to write summary JSON")
	}

	// Get artifact paths
	artifacts := writer.GetArtifactPaths()

	// Print summary
	fmt.Printf("✅ Smoke90 backtest completed successfully!\n\n")
	fmt.Printf("📊 Summary:\n")
	fmt.Printf("   • Coverage: %d/%d windows (%.1f%%)\n",
		results.ProcessedWindows, results.TotalWindows,
		float64(results.ProcessedWindows)/float64(results.TotalWindows)*100)
	fmt.Printf("   • Candidates: %d total\n", results.Metrics.TotalCandidates)
	fmt.Printf("   • Pass Rate: %.1f%% (%d passed, %d failed)\n",
		results.Metrics.OverallPassRate,
		results.Metrics.PassedCandidates,
		results.Metrics.FailedCandidates)

	if results.Metrics.ErrorCount > 0 {
		fmt.Printf("   • Errors: %d\n", results.Metrics.ErrorCount)
	}

	// Print TopGainers alignment if available
	if results.Metrics.TopGainersHitRate != nil {
		hitRate := results.Metrics.TopGainersHitRate
		fmt.Printf("\n📈 TopGainers Alignment:\n")
		fmt.Printf("   • 1h Hit Rate: %.1f%% (%d/%d)\n",
			hitRate.OneHour.HitRate, hitRate.OneHour.Hits, hitRate.OneHour.Total)
		fmt.Printf("   • 24h Hit Rate: %.1f%% (%d/%d)\n",
			hitRate.TwentyFourHour.HitRate, hitRate.TwentyFourHour.Hits, hitRate.TwentyFourHour.Total)
		fmt.Printf("   • 7d Hit Rate: %.1f%% (%d/%d)\n",
			hitRate.SevenDay.HitRate, hitRate.SevenDay.Hits, hitRate.SevenDay.Total)
	}

	// Print P99 relaxation stats if available
	if results.Metrics.RelaxStats != nil && results.Metrics.RelaxStats.TotalEvents > 0 {
		relax := results.Metrics.RelaxStats
		fmt.Printf("\n⚡ P99 Relaxation Events:\n")
		fmt.Printf("   • Total: %d (%.2f per 100 signals)\n",
			relax.TotalEvents, relax.EventsPer100)
		fmt.Printf("   • Average P99: %.1f ms, Grace: %.1f ms\n",
			relax.AvgP99Ms, relax.AvgGraceMs)
	}

	// Print throttling stats if available
	if results.Metrics.ThrottleStats != nil && results.Metrics.ThrottleStats.TotalEvents > 0 {
		throttle := results.Metrics.ThrottleStats
		fmt.Printf("\n🚦 Provider Throttling:\n")
		fmt.Printf("   • Total: %d (%.2f per 100 signals)\n",
			throttle.TotalEvents, throttle.EventsPer100)
		fmt.Printf("   • Most Throttled: %s\n", throttle.MostThrottled)
	}

	fmt.Printf("\n📁 Artifacts Generated:\n")
	fmt.Printf("   • Results JSONL: %s\n", artifacts.ResultsJSONL)
	fmt.Printf("   • Report MD: %s\n", artifacts.ReportMD)
	fmt.Printf("   • Output Directory: %s\n", artifacts.OutputDir)

	// Check if artifacts exist and print file sizes
	if info, err := os.Stat(artifacts.ResultsJSONL); err == nil {
		fmt.Printf("     └─ JSONL size: %s\n", formatFileSize(info.Size()))
	}
	if info, err := os.Stat(artifacts.ReportMD); err == nil {
		fmt.Printf("     └─ Report size: %s\n", formatFileSize(info.Size()))
	}

	log.Info().
		Int("processed_windows", results.ProcessedWindows).
		Int("total_windows", results.TotalWindows).
		Int("total_candidates", results.Metrics.TotalCandidates).
		Float64("pass_rate", results.Metrics.OverallPassRate).
		Int("errors", results.Metrics.ErrorCount).
		Str("artifacts_dir", artifacts.OutputDir).
		Msg("Smoke90 backtest completed")

	return nil
}

// formatFileSize formats file size in human-readable format
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
