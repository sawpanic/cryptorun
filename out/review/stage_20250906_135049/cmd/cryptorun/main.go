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

	"cryptorun/internal/application"
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
		Short: "Run scanning pipelines",
		Long:  "Run momentum or dip scanning with factor analysis and gate evaluation",
	}

	// Add momentum subcommand
	momentumCmd := &cobra.Command{
		Use:   "momentum",
		Short: "Run momentum scanning pipeline",
		Long:  "Multi-timeframe momentum scanning with Gram-Schmidt orthogonalization",
		RunE:  runScanMomentum,
	}

	// Add dip subcommand  
	dipCmd := &cobra.Command{
		Use:   "dip",
		Short: "Run quality-dip scanning pipeline",
		Long:  "Quality-dip scanner optimized for high-probability pullbacks within uptrends",
		RunE:  runScanDip,
	}

	// Add common flags to both subcommands
	for _, cmd := range []*cobra.Command{momentumCmd, dipCmd} {
		cmd.Flags().String("venues", "kraken,okx,coinbase", "Comma-separated venue list")
		cmd.Flags().Int("max-sample", 20, "Maximum sample size for scanning")
		cmd.Flags().Int("ttl", 300, "Cache TTL in seconds")
		cmd.Flags().String("progress", "auto", "Progress output mode (auto|plain|json)")
		cmd.Flags().String("regime", "bull", "Market regime (bull, choppy, high_vol)")
		cmd.Flags().Int("top-n", 20, "Number of top candidates to select")
	}

	scanCmd.AddCommand(momentumCmd)
	scanCmd.AddCommand(dipCmd)

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

	// Add ship command for release preparation
	shipCmd := &cobra.Command{
		Use:   "ship",
		Short: "Prepare release with results validation",
		Long:  "Validates results quality and prepares PR with performance metrics",
		RunE:  runShip,
	}
	
	shipCmd.Flags().String("title", "", "PR title (required)")
	shipCmd.Flags().String("description", "", "PR description")
	shipCmd.Flags().Bool("dry-run", false, "Generate PR body without opening PR")

	// Add monitor command for HTTP endpoints
	monitorCmd := &cobra.Command{
		Use:   "monitor",
		Short: "Start monitoring HTTP server",
		Long:  "Starts HTTP server with /health, /metrics, and /decile endpoints for system monitoring",
		RunE:  runMonitor,
	}
	
	monitorCmd.Flags().String("port", "8080", "HTTP server port")
	monitorCmd.Flags().String("host", "0.0.0.0", "HTTP server host")

	// Add selftest command for offline resilience testing
	selftestCmd := &cobra.Command{
		Use:   "selftest",
		Short: "Run offline resilience self-test",
		Long:  "Validates atomicity, gates, microstructure, universe hygiene, menu integrity (no network)",
		RunE:  runSelfTest,
	}

	// Add digest command for nightly results analysis
	digestCmd := &cobra.Command{
		Use:   "digest",
		Short: "Generate nightly results digest",
		Long:  "Creates comprehensive performance digest from ledger and daily summaries",
		RunE:  runDigest,
	}
	
	digestCmd.Flags().String("date", "", "Target date (YYYY-MM-DD), defaults to yesterday")

	// Add alerts command for Discord/Telegram notifications
	alertsCmd := &cobra.Command{
		Use:   "alerts",
		Short: "Send actionable alerts to Discord/Telegram",
		Long:  "Process scan candidates and send filtered alerts with deduplication and throttling",
		RunE:  runAlerts,
	}
	
	alertsCmd.Flags().Bool("dry-run", false, "Preview alerts without sending (default mode if neither flag specified)")
	alertsCmd.Flags().Bool("send", false, "Send alerts to configured destinations (requires alerts.enabled=true)")
	alertsCmd.Flags().Bool("test", false, "Test alert system configuration and connectivity")
	alertsCmd.Flags().String("symbol", "", "Filter alerts to specific symbol")

	// Add universe command for USD-only universe management
	universeCmd := &cobra.Command{
		Use:   "universe",
		Short: "Rebuild USD-only trading universe with ADV filtering",
		Long:  "Deterministic universe construction with hash integrity and daily snapshots",
		RunE:  runUniverseRebuild,
	}
	
	universeCmd.Flags().Bool("force", false, "Force rebuild even if hash unchanged")
	universeCmd.Flags().Bool("dry-run", false, "Preview rebuild without writing files")

	// Add spec command for resilience self-testing
	specCmd := &cobra.Command{
		Use:   "spec",
		Short: "Run specification compliance suite",
		Long:  "Self-auditing spec suite that fails on drift from product requirements",
		RunE:  runSpecSuite,
	}
	
	specCmd.Flags().Bool("compact", false, "Compact checklist output for menu integration")

	// Add bench command for benchmarking against external references
	benchCmd := &cobra.Command{
		Use:   "bench",
		Short: "Benchmark scanning results against external references",
		Long:  "Compare momentum/dip signals against market references like CoinGecko top gainers",
	}

	topGainersCmd := &cobra.Command{
		Use:   "topgainers",
		Short: "Benchmark against CoinGecko top gainers",
		Long:  "Compare scan results against CoinGecko top gainers at 1h, 24h, 7d timeframes",
		RunE:  runBenchTopGainers,
	}

	topGainersCmd.Flags().String("progress", "auto", "Progress output mode (auto|plain|json)")
	topGainersCmd.Flags().Int("ttl", 300, "Cache TTL in seconds (minimum 300)")
	topGainersCmd.Flags().Int("limit", 20, "Maximum number of top gainers to fetch per window")
	topGainersCmd.Flags().String("windows", "1h,24h,7d", "Comma-separated time windows to analyze")

	benchCmd.AddCommand(topGainersCmd)

	// Add qa command for first-class QA runner with provider guards
	qaCmd := &cobra.Command{
		Use:   "qa",
		Short: "Run first-class QA suite with provider guards",
		Long:  "Comprehensive QA runner with phases 0-6, provider health metrics, and hardened guards",
		RunE:  runQA,
	}
	
	qaCmd.Flags().String("progress", "auto", "Progress output mode (auto|plain|json)")
	qaCmd.Flags().Bool("resume", false, "Resume from last checkpoint")
	qaCmd.Flags().Int("ttl", 300, "Cache TTL in seconds")
	qaCmd.Flags().String("venues", "kraken,okx,coinbase", "Comma-separated venue list")
	qaCmd.Flags().Int("max-sample", 20, "Maximum sample size for testing")
	qaCmd.Flags().Bool("verify", true, "Run acceptance verification (Phase 7) after QA phases")
	qaCmd.Flags().Bool("fail-on-stubs", true, "Fail early if stubs/scaffolds found in non-test code")

	pairsCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(pairsCmd)
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(shipCmd)
	rootCmd.AddCommand(monitorCmd)
	rootCmd.AddCommand(selftestCmd)
	rootCmd.AddCommand(digestCmd)
	rootCmd.AddCommand(alertsCmd)
	rootCmd.AddCommand(universeCmd)
	rootCmd.AddCommand(specCmd)
	rootCmd.AddCommand(benchCmd)
	rootCmd.AddCommand(qaCmd)

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

// Handler functions are implemented in their respective *_main.go files

func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.0fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}