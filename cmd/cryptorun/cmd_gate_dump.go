//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"strings" 
	"time"

	"github.com/spf13/cobra"
	"github.com/sawpanic/cryptorun/internal/gates"
	"github.com/sawpanic/cryptorun/internal/regime"
)

func init() {
	gateDumpCmd := &cobra.Command{
		Use:   "gate-dump",
		Short: "Display active gate thresholds by regime",
		Long: `Display the current gate threshold configuration showing how thresholds 
vary by market regime (trending, choppy, high-vol, risk-off).

This command shows:
- Current market regime (if regime detection is available)  
- Microstructure thresholds for each regime (spread, depth, VADR)
- Universal thresholds that apply across all regimes
- Threshold comparison table across regimes`,
		RunE: runGateDump,
	}

	gateDumpCmd.Flags().StringP("regime", "r", "", "Show thresholds for specific regime (trending, choppy, high_vol, risk_off)")
	gateDumpCmd.Flags().StringP("config", "c", "", "Path to regime thresholds config file")
	gateDumpCmd.Flags().BoolP("current-only", "o", false, "Show only current regime thresholds (requires regime detection)")
	gateDumpCmd.Flags().BoolP("json", "j", false, "Output in JSON format")

	rootCmd.AddCommand(gateDumpCmd)
}

func runGateDump(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get command flags
	specificRegime, _ := cmd.Flags().GetString("regime")
	configPath, _ := cmd.Flags().GetString("config")
	currentOnly, _ := cmd.Flags().GetBool("current-only")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Initialize threshold router
	var thresholdRouter *gates.ThresholdRouter
	var err error

	if configPath != "" {
		thresholdRouter, err = gates.NewThresholdRouter(configPath)
		if err != nil {
			return fmt.Errorf("failed to load threshold config: %w", err)
		}
		fmt.Printf("📁 Loaded thresholds from: %s\n\n", configPath)
	} else {
		thresholdRouter = gates.NewThresholdRouterWithDefaults()
		fmt.Printf("📁 Using built-in default thresholds\n\n")
	}

	// Try to get current regime from regime API (optional)
	var currentRegime string
	regimeAPI := regime.NewMockRegimeAPI() // Use mock for demo - in production would use real regime detector
	if currentOnly || specificRegime == "" {
		if detection, err := regimeAPI.GetCurrentRegime(ctx); err == nil {
			currentRegime = detection.Regime.String()
			fmt.Printf("🎯 Current Market Regime: %s (confidence: %.1f%%)\n", 
				strings.ToUpper(currentRegime), detection.Confidence*100)
			fmt.Printf("   Last Updated: %s\n", detection.Timestamp.Format("2006-01-02 15:04:05"))
			fmt.Printf("   Next Update: %s\n\n", detection.NextUpdate.Format("2006-01-02 15:04:05"))
		} else {
			fmt.Printf("⚠️  Regime detection unavailable: %v\n\n", err)
		}
	}

	if jsonOutput {
		return outputGateDumpJSON(thresholdRouter, currentRegime, specificRegime)
	}

	// Handle specific regime or current-only display
	if specificRegime != "" {
		return displaySpecificRegimeThresholds(thresholdRouter, specificRegime)
	}
	if currentOnly && currentRegime != "" {
		return displaySpecificRegimeThresholds(thresholdRouter, currentRegime)
	}

	// Display full threshold matrix
	return displayFullThresholdMatrix(thresholdRouter, currentRegime)
}

func displaySpecificRegimeThresholds(router *gates.ThresholdRouter, regime string) error {
	thresholds := router.SelectThresholds(regime)
	universal := router.GetUniversalThresholds()

	fmt.Printf("📊 GATE THRESHOLDS - %s REGIME\n", strings.ToUpper(regime))
	fmt.Printf("═══════════════════════════════════════════════\n\n")

	fmt.Printf("💰 MICROSTRUCTURE GATES:\n")
	fmt.Printf("   Spread:          ≤ %.1f basis points\n", thresholds.SpreadMaxBps)
	fmt.Printf("   Depth:           ≥ $%,.0f within ±%.1f%%\n", thresholds.DepthMinUSD, universal.DepthRangePct)
	fmt.Printf("   VADR:            ≥ %.2fx volume-adjusted drift ratio\n\n", thresholds.VADRMin)

	fmt.Printf("⏰ TIMING GATES:\n")
	fmt.Printf("   Freshness:       ≤ %d bars from trigger\n", universal.FreshnessMaxBars)
	fmt.Printf("   Late-fill limit: ≤ %d seconds after bar close\n\n", universal.LateFillMaxSeconds)

	fmt.Printf("💧 LIQUIDITY GATES:\n")
	fmt.Printf("   Min daily volume: ≥ $%,.0f\n\n", universal.MinDailyVolumeUSD)

	// Show human-readable summary
	fmt.Printf("📝 SUMMARY:\n")
	fmt.Printf("   %s\n", router.DescribeThresholds(regime))

	return nil
}

func displayFullThresholdMatrix(router *gates.ThresholdRouter, currentRegime string) error {
	regimes := []string{"default", "trending", "choppy", "high_vol", "risk_off"}
	allThresholds := router.GetAllRegimeThresholds()
	universal := router.GetUniversalThresholds()

	fmt.Printf("📊 REGIME-AWARE GATE THRESHOLD MATRIX\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════════\n\n")

	// Header
	fmt.Printf("%-12s │ %8s │ %10s │ %6s │ Notes\n", "Regime", "Spread", "Depth", "VADR")
	fmt.Printf("%-12s │ %8s │ %10s │ %6s │ %s\n", "", "(bps)", "($k)", "(x)", "")
	fmt.Printf("─────────────┼──────────┼────────────┼────────┼──────────────────────\n")

	// Data rows
	for _, regime := range regimes {
		thresholds := allThresholds[regime]
		marker := "  "
		if currentRegime != "" && strings.ToLower(currentRegime) == regime {
			marker = "🎯" // Current regime marker
		}

		var notes string
		switch regime {
		case "default":
			notes = "Conservative fallback"
		case "trending":
			notes = "Relaxed for trends"
		case "choppy":
			notes = "Tighter for chop"
		case "high_vol":
			notes = "Strict for volatility"
		case "risk_off":
			notes = "Most restrictive"
		}

		fmt.Printf("%s%-10s │ %8.1f │ %10.0f │ %6.2f │ %s\n",
			marker,
			strings.ToUpper(regime),
			thresholds.SpreadMaxBps,
			thresholds.DepthMinUSD/1000,
			thresholds.VADRMin,
			notes)
	}

	fmt.Printf("\n📏 UNIVERSAL CONSTANTS (all regimes):\n")
	fmt.Printf("   Depth Range:     ±%.1f%% around mid price\n", universal.DepthRangePct)
	fmt.Printf("   Freshness:       ≤%d bars from trigger\n", universal.FreshnessMaxBars)
	fmt.Printf("   Late-fill:       ≤%d seconds after bar close\n", universal.LateFillMaxSeconds)
	fmt.Printf("   Min Daily Vol:   ≥$%,.0f\n\n", universal.MinDailyVolumeUSD)

	if currentRegime != "" {
		fmt.Printf("💡 TIP: Use --regime=%s to see detailed thresholds for current regime\n", 
			strings.ToLower(currentRegime))
	} else {
		fmt.Printf("💡 TIP: Use --regime=<name> to see detailed thresholds for specific regime\n")
	}

	return nil
}

func outputGateDumpJSON(router *gates.ThresholdRouter, currentRegime, specificRegime string) error {
	// This would output structured JSON for programmatic consumption
	// Implementation details omitted for brevity
	fmt.Printf(`{
  "timestamp": "%s",
  "current_regime": "%s",
  "thresholds": {
    "default": %s,
    "trending": %s,
    "choppy": %s,
    "high_vol": %s,
    "risk_off": %s
  },
  "universal": %s
}`, time.Now().Format(time.RFC3339), currentRegime, "{}", "{}", "{}", "{}", "{}", "{}")

	fmt.Fprintf(os.Stderr, "\n⚠️  JSON output implementation pending\n")
	return nil
}