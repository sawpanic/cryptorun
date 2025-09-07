package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/sawpanic/cryptorun/internal/application/pipeline"
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Execute momentum scanning pipeline",
	Long: `Execute the complete momentum scanning pipeline with microstructure gates,
guards, and factor orthogonalization.

Examples:
  cryptorun scan --exchange kraken --pairs USD-only
  cryptorun scan --exchange kraken --pairs USD-only --dry-run
  cryptorun scan --regime trending --output ./results`,
	RunE: runScan,
}

// Command-line flags for scan
var (
	scanExchange      string
	scanPairs         string
	scanDryRun        bool
	scanOutputDir     string
	scanMaxSymbols    int
	scanMinScore      float64
	scanRegime        string
	scanShowWeights   bool
	scanExplainRegime bool
	scanConfigFile    string
)

func init() {
	rootCmd.AddCommand(scanCmd)

	// Required flags
	scanCmd.Flags().StringVar(&scanExchange, "exchange", "kraken", "Target exchange (kraken)")
	scanCmd.Flags().StringVar(&scanPairs, "pairs", "USD-only", "Pair filter (USD-only)")

	// Optional flags
	scanCmd.Flags().BoolVar(&scanDryRun, "dry-run", false, "Execute without live API calls")
	scanCmd.Flags().StringVar(&scanOutputDir, "output", "out/scanner", "Output directory for results")
	scanCmd.Flags().IntVar(&scanMaxSymbols, "max-symbols", 50, "Maximum symbols to scan")
	scanCmd.Flags().Float64Var(&scanMinScore, "min-score", 2.0, "Minimum composite score threshold")
	scanCmd.Flags().StringVar(&scanRegime, "regime", "auto", "Market regime (auto|bull|chop|highvol)")
	scanCmd.Flags().BoolVar(&scanShowWeights, "show-weights", false, "Display 5-way factor weight allocation")
	scanCmd.Flags().BoolVar(&scanExplainRegime, "explain-regime", false, "Show regime detection explanation")
	scanCmd.Flags().StringVar(&scanConfigFile, "config", "", "Custom configuration file")

	// Mark required flags
	scanCmd.MarkFlagRequired("exchange")
	scanCmd.MarkFlagRequired("pairs")
}

// runScan executes the scan command by calling the unified pipeline
func runScan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate inputs
	if err := validateScanInputs(); err != nil {
		return fmt.Errorf("invalid scan parameters: %w", err)
	}

	// Create output directories
	if err := os.MkdirAll(scanOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	snapshotDir := filepath.Join(scanOutputDir, "snapshots")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Configure pipeline options
	opts := pipeline.ScanOptions{
		Exchange:    scanExchange,
		Pairs:       scanPairs,
		DryRun:      scanDryRun,
		OutputDir:   scanOutputDir,
		SnapshotDir: snapshotDir,
		MaxSymbols:  scanMaxSymbols,
		MinScore:    scanMinScore,
		Regime:      scanRegime,
		ConfigFile:  scanConfigFile,
	}

	log.Info().
		Str("command", "scan").
		Str("exchange", opts.Exchange).
		Str("pairs", opts.Pairs).
		Bool("dry_run", opts.DryRun).
		Str("regime", opts.Regime).
		Msg("Executing scan command via unified pipeline")

	// SINGLE PIPELINE CALL - this is the key architectural change
	result, artifacts, err := pipeline.Run(ctx, opts)
	if err != nil {
		return fmt.Errorf("scan pipeline failed: %w", err)
	}

	// Display results
	displayScanResults(result, artifacts)

	log.Info().
		Int("total_symbols", result.TotalSymbols).
		Int("candidates", result.Candidates).
		Int("selected", result.Selected).
		Str("duration", result.ProcessingTime).
		Msg("Scan command completed successfully")

	return nil
}

// validateScanInputs ensures scan parameters are valid
func validateScanInputs() error {
	// Validate exchange
	validExchanges := []string{"kraken", "binance", "coinbase"}
	if !contains(validExchanges, scanExchange) {
		return fmt.Errorf("invalid exchange '%s', must be one of: %v", scanExchange, validExchanges)
	}

	// Validate pairs
	validPairs := []string{"USD-only", "BTC-pairs", "ETH-pairs"}
	if !contains(validPairs, scanPairs) {
		return fmt.Errorf("invalid pairs '%s', must be one of: %v", scanPairs, validPairs)
	}

	// Validate regime
	validRegimes := []string{"auto", "trending", "bull", "choppy", "chop", "high-vol", "highvol", "volatile"}
	if !contains(validRegimes, scanRegime) {
		return fmt.Errorf("invalid regime '%s', must be one of: %v", scanRegime, validRegimes)
	}

	// Validate numerical parameters
	if scanMaxSymbols <= 0 || scanMaxSymbols > 500 {
		return fmt.Errorf("max-symbols must be between 1 and 500")
	}

	if scanMinScore < 0 || scanMinScore > 100 {
		return fmt.Errorf("min-score must be between 0 and 100")
	}

	return nil
}

// displayScanResults outputs the scan results to console
func displayScanResults(result *pipeline.ScanResult, artifacts *pipeline.ScanArtifacts) {
	fmt.Printf("\n🏃‍♂️ CryptoRun Scan Results\n")
	fmt.Printf("═══════════════════════════\n")
	fmt.Printf("Timestamp:       %s\n", result.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("Total Symbols:   %d\n", result.TotalSymbols)
	fmt.Printf("Candidates:      %d\n", result.Candidates)
	fmt.Printf("Selected:        %d\n", result.Selected)
	fmt.Printf("Processing Time: %s\n", result.ProcessingTime)

	// Display regime information with badge
	regimeBadge := getRegimeBadgeOlder(result.Regime)
	fmt.Printf("Regime:          %s %s\n", regimeBadge, result.Regime)

	// Display weight map if requested
	if scanShowWeights {
		displayWeightMapOlder(result.Regime)
	}

	// Display regime explanation if requested
	if scanExplainRegime {
		displayRegimeExplanationOlder(result.Regime)
	}

	fmt.Printf("\n📁 Output Artifacts:\n")
	fmt.Printf("  Candidates:    %s\n", artifacts.CandidatesJSONL)
	fmt.Printf("  Ledger:        %s\n", artifacts.Ledger)
	fmt.Printf("  Snapshots:     %d saved\n", artifacts.SnapshotCount)
	fmt.Printf("\n✅ Scan completed successfully\n\n")
}

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// getRegimeBadgeOlder returns an emoji badge for the regime (older scan implementation)
func getRegimeBadgeOlder(regime string) string {
	switch strings.ToLower(regime) {
	case "trending_bull", "bull", "trending":
		return "📈"
	case "choppy", "chop", "ranging":
		return "↔️"
	case "high_vol", "volatile", "high_volatility", "highvol":
		return "⚡"
	default:
		return "❓"
	}
}

// displayWeightMapOlder shows the 5-way factor weight allocation (older scan implementation)
func displayWeightMapOlder(regime string) {
	fmt.Printf("\n🎯 Active Weight Map (%s regime):\n", regime)
	fmt.Printf("┌─────────────┬────────┬──────────────────────────────────┐\n")
	fmt.Printf("│ Factor      │ Weight │ Description                      │\n")
	fmt.Printf("├─────────────┼────────┼──────────────────────────────────┤\n")

	// Get regime-specific weights based on updated config
	weights := getRegimeWeightsOlder(regime)

	fmt.Printf("│ Momentum    │ %5.1f%% │ Multi-timeframe momentum signals │\n", weights["momentum"])
	fmt.Printf("│ Technical   │ %5.1f%% │ Chart patterns, RSI, indicators  │\n", weights["technical"])
	fmt.Printf("│ Volume      │ %5.1f%% │ Volume surge, OI, liquidity      │\n", weights["volume"])
	fmt.Printf("│ Quality     │ %5.1f%% │ Venue health, reserves, ETF      │\n", weights["quality"])
	fmt.Printf("│ Catalyst    │ %5.1f%% │ News events, funding divergence  │\n", weights["catalyst"])
	fmt.Printf("└─────────────┴────────┴──────────────────────────────────┘\n")
	fmt.Printf("Social factor: +10 max (applied separately)\n")
	fmt.Printf("Total base allocation: %.1f%% (excluding Social)\n\n",
		weights["momentum"]+weights["technical"]+weights["volume"]+weights["quality"]+weights["catalyst"])
}

// displayRegimeExplanationOlder shows why the current regime was detected (older scan implementation)
func displayRegimeExplanationOlder(regime string) {
	fmt.Printf("\n💡 Regime Detection Explanation:\n")

	switch strings.ToLower(regime) {
	case "trending_bull", "bull", "trending":
		fmt.Printf("🔍 Detected: TRENDING BULL market\n")
		fmt.Printf("• 7d volatility: Low (≤30%%)\n")
		fmt.Printf("• Above 20MA: High (≥65%% of universe)\n")
		fmt.Printf("• Breadth thrust: Positive (≥15%%)\n")
		fmt.Printf("• Strategy: Emphasize momentum (50%%), relax guards\n")

	case "choppy", "chop", "ranging":
		fmt.Printf("🔍 Detected: CHOPPY/RANGING market\n")
		fmt.Printf("• 7d volatility: Moderate (30-60%%)\n")
		fmt.Printf("• Above 20MA: Mixed (35-65%% of universe)\n")
		fmt.Printf("• Breadth thrust: Weak (-15%% to +15%%)\n")
		fmt.Printf("• Strategy: Reduce momentum (35%%), tighten technical (30%%)\n")

	case "high_vol", "volatile", "high_volatility", "highvol":
		fmt.Printf("🔍 Detected: HIGH VOLATILITY market\n")
		fmt.Printf("• 7d volatility: High (≥60%%)\n")
		fmt.Printf("• Above 20MA: Any (volatility dominates)\n")
		fmt.Printf("• Breadth thrust: Any (unreliable in high vol)\n")
		fmt.Printf("• Strategy: Balanced momentum (30%%), emphasize quality (20%%)\n")

	default:
		fmt.Printf("🔍 Regime: %s (manual override or fallback)\n", regime)
		fmt.Printf("• Using configured weight profile\n")
		fmt.Printf("• Automatic detection bypassed\n")
	}
	fmt.Printf("\n")
}

// getRegimeWeightsOlder returns weight allocation based on config/regimes.yaml (older scan implementation)
func getRegimeWeightsOlder(regime string) map[string]float64 {
	// These match the weights from config/regimes.yaml
	switch strings.ToLower(regime) {
	case "trending_bull", "bull", "trending":
		return map[string]float64{
			"momentum":  50.0,
			"technical": 20.0,
			"volume":    15.0,
			"quality":   10.0,
			"catalyst":  5.0,
		}
	case "choppy", "chop", "ranging":
		return map[string]float64{
			"momentum":  35.0,
			"technical": 30.0,
			"volume":    15.0,
			"quality":   15.0,
			"catalyst":  5.0,
		}
	case "high_vol", "volatile", "high_volatility", "highvol":
		return map[string]float64{
			"momentum":  30.0,
			"technical": 25.0,
			"volume":    20.0,
			"quality":   20.0,
			"catalyst":  5.0,
		}
	default:
		// Default to choppy weights
		return map[string]float64{
			"momentum":  35.0,
			"technical": 30.0,
			"volume":    15.0,
			"quality":   15.0,
			"catalyst":  5.0,
		}
	}
}
