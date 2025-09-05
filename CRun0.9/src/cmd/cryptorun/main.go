package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"cryptorun/application"
)

const (
	appName = "CryptoRun"
	version = "v3.2.1"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.Kitchen})

	rootCmd := &cobra.Command{
		Use:     "cryptorun",
		Short:   "CryptoRun - Cryptocurrency momentum scanner",
		Version: version,
		Long:    "CryptoRun is a 6-48 hour cryptocurrency momentum scanner with advanced regime detection and microstructure analysis",
		Run:     runMenu, // Default to menu interface
	}
	
	// Add scan command for direct scanning
	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Run momentum scanning pipeline",
		Long:  "Run complete momentum scanning with factor analysis and gate evaluation",
		RunE:  runScan,
	}
	
	scanCmd.Flags().String("regime", "bull", "Market regime (bull, choppy, high_vol)")
	scanCmd.Flags().Int("top-n", 20, "Number of top candidates to select")

	pairsCmd := &cobra.Command{
		Use:   "pairs",
		Short: "Pair discovery and management commands",
		Long:  "Commands for discovering, syncing, and managing trading pairs",
	}

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync pairs from exchange with ADV filtering",
		Long:  "Discovers all USD spot pairs from the specified venue and filters by minimum average daily volume",
		RunE:  runPairsSync,
	}

	syncCmd.Flags().String("venue", "kraken", "Exchange venue (kraken)")
	syncCmd.Flags().String("quote", "USD", "Quote currency filter")
	syncCmd.Flags().Int64("min-adv", 100000, "Minimum average daily volume in USD")

	pairsCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(pairsCmd)
	rootCmd.AddCommand(scanCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Error().Err(err).Msg("command failed")
		os.Exit(1)
	}
}

// runMenu starts the interactive menu interface
func runMenu(cmd *cobra.Command, args []string) {
	menuUI := NewMenuUI()
	if err := menuUI.Run(); err != nil {
		log.Error().Err(err).Msg("menu interface failed")
		os.Exit(1)
	}
}

// runScan runs the scanning pipeline directly via CLI
func runScan(cmd *cobra.Command, args []string) error {
	regime, _ := cmd.Flags().GetString("regime")
	topN, _ := cmd.Flags().GetInt("top-n")
	
	log.Info().Str("regime", regime).Int("top_n", topN).Msg("Starting CLI scan")
	
	pipeline := application.NewScanPipeline("out/microstructure/snapshots")
	pipeline.SetRegime(regime)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	candidates, err := pipeline.ScanUniverse(ctx)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	
	// Limit to requested top-N
	if len(candidates) > topN {
		candidates = candidates[:topN]
	}
	
	if err := pipeline.WriteJSONL(candidates, "out/scanner"); err != nil {
		log.Warn().Err(err).Msg("Failed to write JSONL")
	}
	
	fmt.Printf("✅ Scan completed: %d candidates, saved to out/scanner/latest_candidates.jsonl\n", len(candidates))
	
	return nil
}

func runPairsSync(cmd *cobra.Command, args []string) error {
	venue, _ := cmd.Flags().GetString("venue")
	quote, _ := cmd.Flags().GetString("quote")
	minADV, _ := cmd.Flags().GetInt64("min-adv")

	if strings.ToLower(venue) != "kraken" {
		return fmt.Errorf("unsupported venue: %s (only 'kraken' supported)", venue)
	}

	if strings.ToUpper(quote) != "USD" {
		return fmt.Errorf("unsupported quote currency: %s (only 'USD' supported)", quote)
	}

	config := application.PairsSyncConfig{
		Venue:  strings.ToLower(venue),
		Quote:  strings.ToUpper(quote),
		MinADV: minADV,
	}

	sync := application.NewPairsSync(config)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	log.Info().Str("venue", venue).Str("quote", quote).Int64("min_adv", minADV).Msg("Starting pairs sync")

	report, err := sync.SyncPairs(ctx)
	if err != nil {
		return fmt.Errorf("pairs sync failed: %w", err)
	}

	fmt.Printf("Discovered %d %s pairs on %s\n", report.Found, quote, strings.Title(venue))
	fmt.Printf("Kept %d pairs with ADV≥$%s\n", report.Kept, formatNumber(minADV))
	fmt.Printf("Wrote config/universe.json (%d symbols)\n", report.Kept)

	if len(report.Sample) > 0 {
		fmt.Printf("Sample pairs: %s\n", strings.Join(report.Sample, ", "))
	}

	log.Info().Int("found", report.Found).Int("kept", report.Kept).Int("dropped", report.Dropped).Msg("Pairs sync completed")

	return nil
}

func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.0fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}