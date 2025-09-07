package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/sawpanic/cryptorun/internal/application/bench"
)

// benchCmd represents the benchmark command
var benchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Execute benchmark pipelines",
	Long: `Execute benchmark pipelines for Top Gainers alignment and diagnostics.

Available subcommands:
  topgainers   - Compare scanner output with CoinGecko top gainers
  diagnostics  - Analyze hit/miss attribution and generate insights`,
}

// topgainersCmd represents the topgainers benchmark subcommand
var topgainersCmd = &cobra.Command{
	Use:   "topgainers",
	Short: "Compare scanner with CoinGecko top gainers",
	Long: `Execute Top Gainers benchmark to measure scanner alignment with 
CoinGecko top gainers across multiple time windows.

Examples:
  cryptorun bench topgainers --windows 1h,24h --limit 20
  cryptorun bench topgainers --dry-run --output ./bench_results
  cryptorun bench topgainers --windows 1h --limit 50 --ttl 15m`,
	RunE: runTopGainers,
}

// diagnosticsCmd represents the diagnostics subcommand
var diagnosticsCmd = &cobra.Command{
	Use:   "diagnostics",
	Short: "Analyze hit/miss attribution and generate insights",
	Long: `Execute diagnostics pipeline to analyze why top gainers were missed
and generate actionable insights for configuration improvements.

Examples:
  cryptorun bench diagnostics --alignment-score 0.60
  cryptorun bench diagnostics --window 1h --detail-level high
  cryptorun bench diagnostics --output ./diagnostics --sparklines`,
	RunE: runDiagnostics,
}

// Top Gainers flags
var (
	tgWindows    string
	tgLimit      int
	tgTTL        time.Duration
	tgOutputDir  string
	tgDryRun     bool
	tgAPIURL     string
	tgConfigFile string
)

// Diagnostics flags
var (
	diagOutputDir      string
	diagAlignmentScore float64
	diagWindow         string
	diagDetailLevel    string
	diagConfigFile     string
	diagSparklines     bool
)

func init() {
	rootCmd.AddCommand(benchCmd)
	benchCmd.AddCommand(topgainersCmd)
	benchCmd.AddCommand(diagnosticsCmd)

	// Top Gainers flags
	topgainersCmd.Flags().StringVar(&tgWindows, "windows", "1h,24h", "Time windows (comma-separated)")
	topgainersCmd.Flags().IntVar(&tgLimit, "limit", 20, "Number of top gainers to compare")
	topgainersCmd.Flags().DurationVar(&tgTTL, "ttl", 15*time.Minute, "Cache TTL for API results")
	topgainersCmd.Flags().StringVar(&tgOutputDir, "output", "out/bench", "Output directory")
	topgainersCmd.Flags().BoolVar(&tgDryRun, "dry-run", false, "Use mock data instead of API calls")
	topgainersCmd.Flags().StringVar(&tgAPIURL, "api-url", "", "Custom API base URL")
	topgainersCmd.Flags().StringVar(&tgConfigFile, "config", "", "Custom configuration file")

	// Diagnostics flags
	diagnosticsCmd.Flags().StringVar(&diagOutputDir, "output", "out/bench", "Output directory")
	diagnosticsCmd.Flags().Float64Var(&diagAlignmentScore, "alignment-score", 0.60, "Current alignment score to analyze")
	diagnosticsCmd.Flags().StringVar(&diagWindow, "window", "1h", "Time window to focus analysis")
	diagnosticsCmd.Flags().StringVar(&diagDetailLevel, "detail-level", "high", "Analysis detail level (low|medium|high)")
	diagnosticsCmd.Flags().StringVar(&diagConfigFile, "config", "", "Custom configuration file")
	diagnosticsCmd.Flags().BoolVar(&diagSparklines, "sparklines", false, "Include sparkline trend indicators")

	// Mark required flags
	diagnosticsCmd.MarkFlagRequired("alignment-score")
}

// runTopGainers executes the top gainers benchmark via unified pipeline
func runTopGainers(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate and parse inputs
	windows := parseWindows(tgWindows)
	if len(windows) == 0 {
		return fmt.Errorf("invalid windows specification: %s", tgWindows)
	}

	if err := validateTopGainersInputs(); err != nil {
		return fmt.Errorf("invalid benchmark parameters: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(tgOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Configure pipeline options
	opts := bench.TopGainersOptions{
		TTL:        tgTTL,
		Limit:      tgLimit,
		Windows:    windows,
		OutputDir:  tgOutputDir,
		DryRun:     tgDryRun,
		APIBaseURL: tgAPIURL,
		ConfigFile: tgConfigFile,
	}

	log.Info().
		Str("command", "bench topgainers").
		Int("limit", opts.Limit).
		Strs("windows", opts.Windows).
		Bool("dry_run", opts.DryRun).
		Str("ttl", opts.TTL.String()).
		Msg("Executing top gainers benchmark via unified pipeline")

	// SINGLE PIPELINE CALL - unified benchmark execution
	result, artifacts, err := bench.Run(ctx, opts)
	if err != nil {
		return fmt.Errorf("top gainers benchmark failed: %w", err)
	}

	// Display results
	displayTopGainersResults(result, artifacts)

	log.Info().
		Float64("overall_alignment", result.OverallAlignment).
		Str("grade", result.Grade).
		Str("duration", result.ProcessingTime).
		Int("artifacts", len(result.Artifacts)).
		Msg("Top gainers benchmark completed successfully")

	return nil
}

// runDiagnostics executes the diagnostics pipeline
func runDiagnostics(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate inputs
	if err := validateDiagnosticsInputs(); err != nil {
		return fmt.Errorf("invalid diagnostics parameters: %w", err)
	}

	// Create output directory
	diagnosticsDir := fmt.Sprintf("%s/diagnostics", diagOutputDir)
	if err := os.MkdirAll(diagnosticsDir, 0755); err != nil {
		return fmt.Errorf("failed to create diagnostics directory: %w", err)
	}

	// Configure pipeline options
	opts := bench.DiagnosticsOptions{
		OutputDir:         diagnosticsDir,
		AlignmentScore:    diagAlignmentScore,
		BenchmarkWindow:   diagWindow,
		DetailLevel:       diagDetailLevel,
		ConfigFile:        diagConfigFile,
		IncludeSparklines: diagSparklines,
	}

	log.Info().
		Str("command", "bench diagnostics").
		Float64("alignment_score", opts.AlignmentScore).
		Str("window", opts.BenchmarkWindow).
		Str("detail_level", opts.DetailLevel).
		Bool("sparklines", opts.IncludeSparklines).
		Msg("Executing diagnostics pipeline")

	// SINGLE PIPELINE CALL - unified diagnostics execution
	result, artifacts, err := bench.RunDiagnostics(ctx, opts)
	if err != nil {
		return fmt.Errorf("diagnostics pipeline failed: %w", err)
	}

	// Display results
	displayDiagnosticsResults(result, artifacts)

	log.Info().
		Int("insights", len(result.ActionableInsights)).
		Int("missed_symbols", result.MissAttribution.TotalMisses).
		Float64("kendall_tau", result.CorrelationStats.KendallTau).
		Str("duration", result.ProcessingTime).
		Msg("Diagnostics pipeline completed successfully")

	return nil
}

// parseWindows converts comma-separated windows string to slice
func parseWindows(windowsStr string) []string {
	windows := strings.Split(windowsStr, ",")
	var validWindows []string

	for _, window := range windows {
		trimmed := strings.TrimSpace(window)
		if trimmed != "" {
			validWindows = append(validWindows, trimmed)
		}
	}

	return validWindows
}

// validateTopGainersInputs ensures benchmark parameters are valid
func validateTopGainersInputs() error {
	if tgLimit <= 0 || tgLimit > 100 {
		return fmt.Errorf("limit must be between 1 and 100")
	}

	if tgTTL < time.Minute || tgTTL > time.Hour {
		return fmt.Errorf("TTL must be between 1 minute and 1 hour")
	}

	return nil
}

// validateDiagnosticsInputs ensures diagnostics parameters are valid
func validateDiagnosticsInputs() error {
	if diagAlignmentScore < 0 || diagAlignmentScore > 1 {
		return fmt.Errorf("alignment-score must be between 0.0 and 1.0")
	}

	validWindows := []string{"1h", "4h", "12h", "24h", "7d"}
	if !contains(validWindows, diagWindow) {
		return fmt.Errorf("invalid window '%s', must be one of: %v", diagWindow, validWindows)
	}

	validLevels := []string{"low", "medium", "high"}
	if !contains(validLevels, diagDetailLevel) {
		return fmt.Errorf("invalid detail-level '%s', must be one of: %v", diagDetailLevel, validLevels)
	}

	return nil
}

// displayTopGainersResults outputs benchmark results to console
func displayTopGainersResults(result *bench.TopGainersResult, artifacts *bench.TopGainersArtifacts) {
	fmt.Printf("\n🏆 Top Gainers Benchmark Results\n")
	fmt.Printf("═══════════════════════════════════\n")
	fmt.Printf("Timestamp:         %s\n", result.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("Overall Alignment: %.1f%%\n", result.OverallAlignment*100)
	fmt.Printf("Grade:             %s\n", result.Grade)
	fmt.Printf("Processing Time:   %s\n", result.ProcessingTime)
	fmt.Printf("Recommendation:    %s\n", result.Recommendation)

	fmt.Printf("\n📊 Window Alignments:\n")
	for window, alignment := range result.WindowResults {
		fmt.Printf("  %s: %.1f%% (%d matches)\n", window, alignment.Score*100, alignment.Matches)
	}

	fmt.Printf("\n📁 Generated Artifacts:\n")
	if artifacts.AlignmentReport != "" {
		fmt.Printf("  Report:     %s\n", artifacts.AlignmentReport)
	}
	if artifacts.BenchmarkResult != "" {
		fmt.Printf("  Raw Data:   %s\n", artifacts.BenchmarkResult)
	}
	for window, path := range artifacts.WindowJSONs {
		fmt.Printf("  %s JSON:   %s\n", window, path)
	}

	fmt.Printf("\n✅ Benchmark completed successfully\n\n")
}

// displayDiagnosticsResults outputs diagnostics results to console
func displayDiagnosticsResults(result *bench.DiagnosticsResult, artifacts *bench.DiagnosticsArtifacts) {
	fmt.Printf("\n🔍 Diagnostics Analysis Results\n")
	fmt.Printf("══════════════════════════════════\n")
	fmt.Printf("Timestamp:       %s\n", result.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("Alignment Score: %.1f%%\n", result.AlignmentScore*100)
	fmt.Printf("Processing Time: %s\n", result.ProcessingTime)

	fmt.Printf("\n📈 Miss Attribution:\n")
	fmt.Printf("  Total Hits:   %d\n", result.MissAttribution.TotalHits)
	fmt.Printf("  Total Misses: %d\n", result.MissAttribution.TotalMisses)

	fmt.Printf("\n🚪 Gate Failures:\n")
	for gate, count := range result.MissAttribution.GateFailures {
		fmt.Printf("  %s: %d\n", gate, count)
	}

	fmt.Printf("\n📊 Correlation Statistics:\n")
	fmt.Printf("  Kendall Tau:    %.3f\n", result.CorrelationStats.KendallTau)
	fmt.Printf("  Spearman Rho:   %.3f\n", result.CorrelationStats.SpearmanRho)
	fmt.Printf("  Symbol Overlap: %.1f%%\n", result.CorrelationStats.SymbolOverlap*100)

	fmt.Printf("\n💡 Actionable Insights (%d):\n", len(result.ActionableInsights))
	for i, insight := range result.ActionableInsights[:min(3, len(result.ActionableInsights))] {
		fmt.Printf("  %d. [%s] %s\n", i+1, insight.Priority, insight.Change)
		fmt.Printf("     Impact: %s\n", insight.Impact)
		fmt.Printf("     Risk: %s\n", insight.Risk)
	}

	fmt.Printf("\n📁 Generated Artifacts:\n")
	fmt.Printf("  Diagnostic Report: %s\n", artifacts.DiagnosticReport)
	fmt.Printf("  Hit/Miss Analysis: %s\n", artifacts.HitMissBreakdown)
	fmt.Printf("  Gate Analysis:     %s\n", artifacts.GateAnalysis)
	fmt.Printf("  Correlation Data:  %s\n", artifacts.CorrelationData)

	fmt.Printf("\n✅ Diagnostics completed successfully\n\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
