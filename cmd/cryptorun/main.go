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
	"golang.org/x/term"

	"github.com/sawpanic/cryptorun/internal/application"
	httpmetrics "github.com/sawpanic/cryptorun/internal/interfaces/http"
)

const (
	appName = "CryptoRun"
	version = "v3.2.1"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.Kitchen})

	// Initialize metrics system
	httpmetrics.InitializeMetrics()
	log.Info().Msg("CryptoRun metrics system initialized")

	rootCmd := &cobra.Command{
		Use:     "cryptorun",
		Short:   "MENU IS CANON — use `cryptorun` to open it.",
		Version: version,
		Long: `🎯 MENU IS CANON — use 'cryptorun' to open it.

CryptoRun is a 6-48 hour cryptocurrency momentum scanner with advanced regime detection and microstructure analysis.

THE INTERACTIVE MENU IS THE PRIMARY INTERFACE
   Run 'cryptorun' in a terminal for the full interactive experience.
   CLI flags and subcommands are automation shims for non-interactive use.`,
		Run: runDefaultEntry, // TTY detection and menu routing
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
		cmd.Flags().String("regime", "auto", "Market regime (auto|bull|chop|highvol)")
		cmd.Flags().Bool("show-weights", false, "Display 5-way factor weight allocation")
		cmd.Flags().Bool("explain-regime", false, "Show regime detection explanation")
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

	// Add schedule command for production scan loops
	scheduleCmd := &cobra.Command{
		Use:   "schedule",
		Short: "Production scan scheduler with hot/warm cycles",
		Long:  "Manage scheduled jobs for hot momentum scans (15m), warm scans (2h), and regime refresh (4h)",
	}

	// Add list subcommand
	scheduleListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured scheduled jobs",
		Long:  "Display all jobs with their schedules, status, and descriptions",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Schedule list functionality disabled in this build")
			return nil
		},
	}

	// Add start subcommand
	scheduleStartCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the scheduler daemon",
		Long:  "Start the background scheduler daemon to execute jobs on their configured schedules",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Schedule start functionality disabled in this build")
			return nil
		},
	}

	// Add status subcommand
	scheduleStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show scheduler status",
		Long:  "Display current scheduler status, uptime, and next run information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Schedule status functionality disabled in this build")
			return nil
		},
	}

	// Add run subcommand
	scheduleRunCmd := &cobra.Command{
		Use:   "run [job-name]",
		Short: "Execute a specific job immediately",
		Long:  "Run a scheduled job immediately for testing or manual execution",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Schedule run functionality disabled in this build")
			return nil
		},
		Args:  cobra.MinimumNArgs(1),
	}

	scheduleRunCmd.Flags().Bool("dry-run", false, "Preview job execution without creating artifacts")

	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleStartCmd)
	scheduleCmd.AddCommand(scheduleStatusCmd)
	scheduleCmd.AddCommand(scheduleRunCmd)

	// Add explain command for factor analysis
	explainCmd := &cobra.Command{
		Use:   "explain",
		Short: "Factor explanation and forensic analysis",
		Long:  "Analyze and explain factor contributions with delta detection",
	}

	// Add report command for automated reporting
	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Generate automated analysis reports",
		Long:  "Create comprehensive reports for regime analysis, performance tracking, and portfolio monitoring",
	}

	// Add delta subcommand for forensic factor comparison
	deltaCmd := &cobra.Command{
		Use:   "delta",
		Short: "Compare factor explanations against baseline",
		Long:  "Forensic lens to detect factor contribution shifts beyond tolerances",
		RunE:  runExplainDelta,
	}

	deltaCmd.Flags().String("universe", "", "Universe specification (e.g., topN=30)")
	deltaCmd.Flags().String("out", "artifacts/explain_delta", "Output directory for artifacts")
	deltaCmd.Flags().String("baseline", "latest", "Baseline to compare against (latest|date|path)")
	deltaCmd.Flags().Bool("progress", true, "Show progress indicators")

	explainCmd.AddCommand(deltaCmd)

	// Add regime subcommand for weekly regime analysis
	regimeReportCmd := &cobra.Command{
		Use:   "regime",
		Short: "Generate weekly regime analysis report",
		Long:  "Analyze regime flip history, exit distributions, and score→return lift by regime",
		RunE:  runReportRegime,
	}

	regimeReportCmd.Flags().String("since", "28d", "Analysis period (28d|4w|1m|90d)")
	regimeReportCmd.Flags().String("out", "./artifacts/reports", "Output directory for reports and CSV files")
	regimeReportCmd.Flags().Bool("charts", false, "Generate decile lift charts")
	regimeReportCmd.Flags().Bool("pit", true, "Use point-in-time data integrity")

	reportCmd.AddCommand(regimeReportCmd)

	// Add probe command for data facade testing
	probeCmd := &cobra.Command{
		Use:   "probe",
		Short: "Probe and test system components",
		Long:  "Test and monitor various CryptoRun components including data feeds, venues, and caches",
	}

	// Add data subcommand for data facade probing
	probeDataCmd := &cobra.Command{
		Use:   "data",
		Short: "Probe data facade and exchange connectivity",
		Long:  "Test data facade functionality, venue health, cache performance, and streaming capabilities",
		RunE:  runProbeData,
	}

	probeDataCmd.Flags().String("pair", "BTCUSD", "Trading pair to test")
	probeDataCmd.Flags().String("venue", "kraken", "Exchange venue to test")
	probeDataCmd.Flags().Int("mins", 5, "Duration in minutes for probe")
	probeDataCmd.Flags().Bool("stream", false, "Enable streaming mode")

	probeCmd.AddCommand(probeDataCmd)

	// Add spec command for resilience self-testing
	specCmd := &cobra.Command{
		Use:   "spec",
		Short: "Run specification compliance suite",
		Long:  "Self-auditing spec suite that fails on drift from product requirements",
		RunE:  runSpecSuite,
	}

	specCmd.Flags().Bool("compact", false, "Compact checklist output for menu integration")

	// Add backtest command for validation backtests
	backtestCmd := &cobra.Command{
		Use:   "backtest",
		Short: "Run validation backtests against cached data",
		Long:  "Execute backtests to validate scanner performance using cached/historical data",
	}

	// Add smoke90 subcommand for 90-day smoke backtest
	smoke90Cmd := &cobra.Command{
		Use:   "smoke90",
		Short: "Run 90-day cached smoke backtest",
		Long:  "Validates unified scanner end-to-end using cached data with 4h stride over 90 days",
		RunE:  runBacktestSmoke90,
	}

	smoke90Cmd.Flags().Int("top-n", 20, "Top N candidates per window")
	smoke90Cmd.Flags().Duration("stride", 4*time.Hour, "Time stride between windows")
	smoke90Cmd.Flags().Duration("hold", 24*time.Hour, "Hold period for P&L calculation")
	smoke90Cmd.Flags().String("output", "out/backtest", "Output directory for results")
	smoke90Cmd.Flags().Bool("use-cache", true, "Use cached data only (no live fetches)")
	smoke90Cmd.Flags().String("progress", "auto", "Progress output mode (auto|plain|json)")

	backtestCmd.AddCommand(smoke90Cmd)

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
	topGainersCmd.Flags().Int("n", 20, "Maximum number of top gainers to fetch per window")
	topGainersCmd.Flags().String("windows", "1h,24h", "Comma-separated time windows to analyze")
	topGainersCmd.Flags().Bool("dry-run", false, "Preview benchmark without making API calls")

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

	// Add explicit menu command (though it's also the default)
	menuCmd := &cobra.Command{
		Use:   "menu",
		Short: "Interactive menu interface (canonical UX)",
		Long:  "Start the interactive menu system for full CryptoRun functionality",
		Run:   runMenu,
	}

	pairsCmd.AddCommand(syncCmd)
	benchCmd.AddCommand(topGainersCmd)

	// Add commands in Menu-first order
	rootCmd.AddCommand(menuCmd)     // Menu first
	rootCmd.AddCommand(scanCmd)     // Primary functionality
	rootCmd.AddCommand(scheduleCmd) // Scheduled scanning
	rootCmd.AddCommand(backtestCmd) // Backtesting
	rootCmd.AddCommand(benchCmd)    // Benchmarking
	rootCmd.AddCommand(pairsCmd)    // Data management
	rootCmd.AddCommand(qaCmd)       // Quality assurance
	rootCmd.AddCommand(selftestCmd) // Testing
	rootCmd.AddCommand(specCmd)     // Compliance
	rootCmd.AddCommand(shipCmd)     // Release
	rootCmd.AddCommand(monitorCmd)  // Monitoring
	rootCmd.AddCommand(digestCmd)   // Analysis
	rootCmd.AddCommand(alertsCmd)   // Notifications
	rootCmd.AddCommand(universeCmd) // Universe management
	rootCmd.AddCommand(explainCmd)  // Factor analysis
	rootCmd.AddCommand(reportCmd)   // Automated reporting
	rootCmd.AddCommand(probeCmd)    // System probing

	if err := rootCmd.Execute(); err != nil {
		log.Error().Err(err).Msg("command failed")
		os.Exit(1)
	}
}

// runDefaultEntry implements TTY detection and routing to menu or help
func runDefaultEntry(cmd *cobra.Command, args []string) {
	// Check if we have a TTY (interactive terminal)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Non-interactive environment - show guidance
		fmt.Fprintf(os.Stderr, "❌ Interactive menu requires a TTY terminal.\n")
		fmt.Fprintf(os.Stderr, "   Use subcommands and flags for non-interactive automation:\n\n")
		fmt.Fprintf(os.Stderr, "   cryptorun scan momentum --venues kraken --top-n 20\n")
		fmt.Fprintf(os.Stderr, "   cryptorun bench topgainers --windows 1h,24h --dry-run\n")
		fmt.Fprintf(os.Stderr, "   cryptorun --help\n\n")
		fmt.Fprintf(os.Stderr, "   See docs/CLI.md for complete automation reference.\n")
		os.Exit(2)
	}

	// Interactive terminal - launch menu as canonical interface
	runMenu(cmd, args)
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
