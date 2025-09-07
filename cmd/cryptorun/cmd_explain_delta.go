package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/sawpanic/cryptorun/internal/explain/delta"
)

// runExplainDelta executes the explain delta forensic analysis
func runExplainDelta(cmd *cobra.Command, args []string) error {
	// Parse flags
	universe, _ := cmd.Flags().GetString("universe")
	outputDir, _ := cmd.Flags().GetString("out")
	baseline, _ := cmd.Flags().GetString("baseline")
	progress, _ := cmd.Flags().GetBool("progress")

	// Validate universe parameter
	if universe == "" {
		return fmt.Errorf("universe parameter required (e.g., --universe topN=30)")
	}

	// Create absolute output path
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("failed to resolve output directory: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	log.Info().
		Str("universe", universe).
		Str("baseline", baseline).
		Str("output_dir", absOutputDir).
		Bool("progress", progress).
		Msg("Starting explain delta forensic analysis")

	if progress {
		fmt.Printf("🔍 Explain Delta — universe=%s baseline=%s\n", universe, baseline)
		fmt.Printf("📁 Output: %s\n\n", absOutputDir)
	}

	// Create delta runner
	config := &delta.Config{
		Universe:     universe,
		BaselinePath: baseline,
		OutputDir:    absOutputDir,
		Progress:     progress,
	}

	runner := delta.NewRunner(config)

	// Set up context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Run the delta analysis
	if progress {
		fmt.Printf("⏳ [10%%] Loading baseline...\n")
	}

	results, err := runner.Run(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Explain delta analysis failed")
		return fmt.Errorf("explain delta failed: %w", err)
	}

	// Print compact console output
	printDeltaSummary(results, progress)

	// Create writer for artifacts
	writer := delta.NewWriter(absOutputDir)

	// Write detailed JSONL results
	if err := writer.WriteJSONL(results); err != nil {
		log.Warn().Err(err).Msg("Failed to write JSONL results")
	}

	// Write markdown summary
	if err := writer.WriteMarkdown(results); err != nil {
		log.Warn().Err(err).Msg("Failed to write markdown summary")
	}

	// Get artifact paths
	artifacts := writer.GetArtifactPaths()

	if progress {
		fmt.Printf("\n📁 Artifacts Generated:\n")
		fmt.Printf("   • Results JSONL: %s\n", artifacts.ResultsJSONL)
		fmt.Printf("   • Summary MD: %s\n", artifacts.SummaryMD)
	}

	log.Info().
		Int("total_assets", results.TotalAssets).
		Int("fail_count", results.FailCount).
		Int("warn_count", results.WarnCount).
		Int("ok_count", results.OKCount).
		Str("regime", results.Regime).
		Str("artifacts_dir", absOutputDir).
		Msg("Explain delta analysis completed")

	// Exit non-zero if any failures detected
	if results.FailCount > 0 {
		return fmt.Errorf("explain delta detected %d critical factor shifts", results.FailCount)
	}

	return nil
}

// printDeltaSummary prints the compact console summary
func printDeltaSummary(results *delta.Results, progress bool) {
	if !progress {
		return
	}

	fmt.Printf("● Explain Delta — universe=%s baseline=%s\n",
		results.Universe, results.BaselineTimestamp.Format("2006-01-02T15:04Z"))

	// Status summary
	status := fmt.Sprintf("FAIL(%d) WARN(%d) OK(%d)",
		results.FailCount, results.WarnCount, results.OKCount)

	fmt.Printf("  %s | regime=%s\n", status, results.Regime)

	// Show worst offenders if any
	if len(results.WorstOffenders) > 0 {
		fmt.Printf("  worst:\n")
		for i, offender := range results.WorstOffenders {
			if i >= 2 { // Limit to top 2
				break
			}

			sign := "+"
			if offender.Delta < 0 {
				sign = ""
			}

			fmt.Printf("    %d) %-4s %-14s %s%.1f (>±%.1f)  hint: %s\n",
				i+1,
				offender.Symbol,
				offender.Factor,
				sign,
				offender.Delta,
				offender.Tolerance,
				offender.Hint)
		}
	}

	if results.FailCount == 0 && results.WarnCount == 0 {
		fmt.Printf("  ✅ All factor contributions within tolerance\n")
	}

	fmt.Printf("\n")
}
