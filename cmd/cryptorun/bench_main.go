package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/sawpanic/cryptorun/internal/application/bench"
)

// runBenchTopGainers runs the top gainers benchmark via unified entry point
func runBenchTopGainers(cmd *cobra.Command, args []string) error {
	log.Info().Msg("Starting top gainers benchmark via unified pipeline")

	// Get flags
	progressMode, _ := cmd.Flags().GetString("progress")
	ttl, _ := cmd.Flags().GetInt("ttl")
	limit, _ := cmd.Flags().GetInt("n")
	windows, _ := cmd.Flags().GetString("windows")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Parse time windows
	windowList := strings.Split(windows, ",")
	for i, window := range windowList {
		windowList[i] = strings.TrimSpace(window)
	}

	// Validate windows
	validWindows := map[string]bool{"1h": true, "24h": true, "7d": true}
	for _, window := range windowList {
		if !validWindows[window] {
			return fmt.Errorf("invalid time window: %s (valid: 1h, 24h, 7d)", window)
		}
	}

	if dryRun {
		log.Info().Msg("Dry-run mode: will preview benchmark without API calls")
	}

	log.Info().
		Str("progress", progressMode).
		Int("ttl", ttl).
		Int("limit", limit).
		Strs("windows", windowList).
		Msg("Top gainers benchmark via unified pipeline")

	// Configure unified benchmark options
	opts := bench.TopGainersOptions{
		TTL:        time.Duration(ttl) * time.Second,
		Limit:      limit,
		Windows:    windowList,
		OutputDir:  "out/bench",
		DryRun:     dryRun,
		APIBaseURL: "https://api.coingecko.com/api/v3",
		ConfigFile: "",
	}

	// Execute via SINGLE unified benchmark entry point
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, artifacts, err := bench.Run(ctx, opts)
	if err != nil {
		return fmt.Errorf("unified benchmark pipeline failed: %w", err)
	}

	// Display results summary
	fmt.Printf("✅ Top gainers benchmark completed via UnifiedFactorEngine\n")
	fmt.Printf("Windows analyzed: %s\n", strings.Join(windowList, ", "))
	fmt.Printf("Alignment score: %.2f%%\n", result.OverallAlignment*100)
	fmt.Printf("Grade: %s\n", result.Grade)
	fmt.Printf("Duration: %s\n", result.ProcessingTime)
	fmt.Printf("Artifacts: %s\n", artifacts.AlignmentReport)

	// Log detailed results
	for window, alignment := range result.WindowResults {
		fmt.Printf("  %s alignment: %.2f%% (%d matches / %d total)\n",
			window, alignment.Score*100, alignment.Matches, alignment.Total)
	}

	return nil
}
