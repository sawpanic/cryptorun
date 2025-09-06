package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"cryptorun/internal/bench"
	"cryptorun/internal/scan/progress"
)

// runBenchTopGainers runs the top gainers benchmark against CoinGecko
func runBenchTopGainers(cmd *cobra.Command, args []string) error {
	log.Info().Msg("Starting top gainers benchmark")

	// Get flags
	progressMode, _ := cmd.Flags().GetString("progress")
	ttl, _ := cmd.Flags().GetInt("ttl")
	limit, _ := cmd.Flags().GetInt("limit")
	windows, _ := cmd.Flags().GetString("windows")

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

	// Create progress bus for streaming
	progressBus := progress.NewScanProgressBus(progressMode, "out/audit")

	// Create benchmark config
	config := bench.TopGainersConfig{
		TTL:       time.Duration(ttl) * time.Second,
		Limit:     limit,
		Windows:   windowList,
		OutputDir: "out/bench",
		AuditDir:  "out/audit",
	}

	log.Info().
		Str("progress", progressMode).
		Int("ttl", ttl).
		Int("limit", limit).
		Strs("windows", windowList).
		Msg("Top gainers benchmark configuration")

	// Create benchmark runner
	runner := bench.NewTopGainersBenchmark(config)
	runner.SetProgressBus(progressBus)

	// Run benchmark
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := runner.RunBenchmark(ctx)
	if err != nil {
		return fmt.Errorf("benchmark failed: %w", err)
	}

	// Output results
	fmt.Printf("✅ Top gainers benchmark completed\n")
	fmt.Printf("Windows analyzed: %s\n", strings.Join(windowList, ", "))
	fmt.Printf("Alignment score: %.2f%%\n", result.OverallAlignment*100)
	fmt.Printf("Artifacts written to: out/bench/\n")

	// Log detailed results
	for _, window := range windowList {
		if alignment, exists := result.WindowAlignments[window]; exists {
			fmt.Printf("  %s alignment: %.2f%% (%d matches / %d total)\n",
				window, alignment.Score*100, alignment.Matches, alignment.Total)
		}
	}

	return nil
}
