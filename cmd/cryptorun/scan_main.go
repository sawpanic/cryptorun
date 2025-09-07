package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	pipelineexec "github.com/sawpanic/cryptorun/internal/application/pipeline"
)

// runScanMomentum runs the momentum scanning pipeline via unified entry point
func runScanMomentum(cmd *cobra.Command, args []string) error {
	log.Info().Msg("Starting momentum scanning via UnifiedFactorEngine")

	// Get flags
	venues, _ := cmd.Flags().GetString("venues")
	maxSample, _ := cmd.Flags().GetInt("max-sample")
	ttl, _ := cmd.Flags().GetInt("ttl")
	progressMode, _ := cmd.Flags().GetString("progress")
	regime, _ := cmd.Flags().GetString("regime")
	showWeights, _ := cmd.Flags().GetBool("show-weights")
	explainRegime, _ := cmd.Flags().GetBool("explain-regime")
	topN, _ := cmd.Flags().GetInt("top-n")

	// Parse venues
	venueList := strings.Split(venues, ",")
	for i, venue := range venueList {
		venueList[i] = strings.TrimSpace(venue)
	}

	// Log regime override warning if manual regime specified
	if regime != "auto" {
		log.Warn().
			Str("regime", regime).
			Msg("Manual regime override - detector bypassed")
	}

	log.Info().
		Strs("venues", venueList).
		Int("max_sample", maxSample).
		Int("ttl", ttl).
		Str("progress", progressMode).
		Str("regime", regime).
		Bool("show_weights", showWeights).
		Bool("explain_regime", explainRegime).
		Int("top_n", topN).
		Msg("Momentum scan via unified pipeline")

	// Configure unified scan options
	opts := pipelineexec.ScanOptions{
		Exchange:    strings.Join(venueList, ","),
		Pairs:       "USD-only",
		DryRun:      false,
		OutputDir:   "out/scan",
		SnapshotDir: "out/microstructure/snapshots",
		MaxSymbols:  maxSample,
		MinScore:    2.0,
		Regime:      regime,
		ConfigFile:  "",
	}

	// Execute via SINGLE unified pipeline entry point
	ctx := context.Background()
	result, artifacts, err := pipelineexec.Run(ctx, opts)
	if err != nil {
		return fmt.Errorf("unified momentum pipeline failed: %w", err)
	}

	// Display results summary with regime information
	fmt.Printf("✅ Momentum scan completed via UnifiedFactorEngine\n")
	fmt.Printf("Processed: %d symbols\n", result.TotalSymbols)
	fmt.Printf("Candidates: %d\n", result.Candidates)
	fmt.Printf("Selected: %d\n", result.Selected)
	fmt.Printf("Duration: %s\n", result.ProcessingTime)

	// Display regime information with badge
	regimeBadge := getRegimeBadge(result.Regime)
	fmt.Printf("Regime: %s %s\n", regimeBadge, result.Regime)

	// Display weight map if requested
	if showWeights {
		displayWeightMap(result.Regime)
	}

	// Display regime explanation if requested
	if explainRegime {
		displayRegimeExplanation(result.Regime)
	}

	fmt.Printf("Results: %s\n", artifacts.CandidatesJSONL)

	return nil
}

// runScanDip runs the quality-dip scanning pipeline via unified entry point
func runScanDip(cmd *cobra.Command, args []string) error {
	log.Info().Msg("Starting dip scanning via UnifiedFactorEngine")

	// Get flags
	venues, _ := cmd.Flags().GetString("venues")
	maxSample, _ := cmd.Flags().GetInt("max-sample")
	ttl, _ := cmd.Flags().GetInt("ttl")
	progressMode, _ := cmd.Flags().GetString("progress")
	regime, _ := cmd.Flags().GetString("regime")
	showWeights, _ := cmd.Flags().GetBool("show-weights")
	explainRegime, _ := cmd.Flags().GetBool("explain-regime")
	topN, _ := cmd.Flags().GetInt("top-n")

	// Parse venues
	venueList := strings.Split(venues, ",")
	for i, venue := range venueList {
		venueList[i] = strings.TrimSpace(venue)
	}

	// Log regime override warning if manual regime specified
	if regime != "auto" {
		log.Warn().
			Str("regime", regime).
			Msg("Manual regime override - detector bypassed")
	}

	log.Info().
		Strs("venues", venueList).
		Int("max_sample", maxSample).
		Int("ttl", ttl).
		Str("progress", progressMode).
		Str("regime", regime).
		Bool("show_weights", showWeights).
		Bool("explain_regime", explainRegime).
		Int("top_n", topN).
		Msg("Dip scan via unified pipeline")

	// Configure unified scan options for dip mode
	opts := pipelineexec.ScanOptions{
		Exchange:    strings.Join(venueList, ","),
		Pairs:       "USD-only",
		DryRun:      false,
		OutputDir:   "out/scan",
		SnapshotDir: "out/microstructure/snapshots",
		MaxSymbols:  maxSample,
		MinScore:    1.5, // Lower threshold for dip candidates
		Regime:      regime,
		ConfigFile:  "",
	}

	// Execute via SAME unified pipeline entry point
	ctx := context.Background()
	result, artifacts, err := pipelineexec.Run(ctx, opts)
	if err != nil {
		return fmt.Errorf("unified dip pipeline failed: %w", err)
	}

	// Display results summary with regime information
	fmt.Printf("✅ Dip scan completed via UnifiedFactorEngine\n")
	fmt.Printf("Processed: %d symbols\n", result.TotalSymbols)
	fmt.Printf("Candidates: %d\n", result.Candidates)
	fmt.Printf("Selected: %d\n", result.Selected)
	fmt.Printf("Duration: %s\n", result.ProcessingTime)

	// Display regime information with badge
	regimeBadge := getRegimeBadge(result.Regime)
	fmt.Printf("Regime: %s %s\n", regimeBadge, result.Regime)

	// Display weight map if requested
	if showWeights {
		displayWeightMap(result.Regime)
	}

	// Display regime explanation if requested
	if explainRegime {
		displayRegimeExplanation(result.Regime)
	}

	fmt.Printf("Results: %s\n", artifacts.CandidatesJSONL)
	fmt.Printf("Note: Dip scoring uses same UnifiedFactorEngine with adjusted thresholds\n")

	return nil
}

// getRegimeBadge returns an emoji badge for the regime
func getRegimeBadge(regime string) string {
	switch strings.ToLower(regime) {
	case "trending_bull", "bull", "trending":
		return "📈"
	case "choppy", "chop", "ranging":
		return "↔️"
	case "high_vol", "volatile", "high_volatility":
		return "⚡"
	default:
		return "❓"
	}
}

// displayWeightMap shows the 5-way factor weight allocation for the current regime
func displayWeightMap(regime string) {
	fmt.Printf("\n🎯 Active Weight Map (%s regime):\n", regime)
	fmt.Printf("┌─────────────┬────────┬──────────────────────────────────┐\n")
	fmt.Printf("│ Factor      │ Weight │ Description                      │\n")
	fmt.Printf("├─────────────┼────────┼──────────────────────────────────┤\n")

	// Get regime-specific weights based on updated config
	weights := getRegimeWeights(regime)

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

// displayRegimeExplanation shows why the current regime was detected
func displayRegimeExplanation(regime string) {
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

	case "high_vol", "volatile", "high_volatility":
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

// getRegimeWeights returns weight allocation based on config/regimes.yaml
func getRegimeWeights(regime string) map[string]float64 {
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
	case "high_vol", "volatile", "high_volatility":
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
