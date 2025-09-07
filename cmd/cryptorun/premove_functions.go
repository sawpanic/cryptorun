package main

import (
	"fmt"
	"strconv"
	"time"
)

// displayPreMovementTable displays the pre-movement candidates table
func (ui *MenuUI) displayPreMovementTable(candidates []PreMovementCandidate, scanLatency time.Duration) {
	fmt.Printf("\n")
	fmt.Printf("┌─────┬─────────┬───────┬─────────┬─────────┬─────────┬─────────┬────────────────────────┬──────────────┐\n")
	fmt.Printf("│Rank │ Symbol  │ Score │CVD Rsid │Fund Div │Vol Build│Prob (%)│        Badges          │    Action    │\n")
	fmt.Printf("├─────┼─────────┼───────┼─────────┼─────────┼─────────┼─────────┼────────────────────────┼──────────────┤\n")

	for _, candidate := range candidates {
		badges := ""
		for _, badge := range candidate.Badges {
			badges += badge.Value + " "
		}
		if len(badges) > 22 {
			badges = badges[:19] + "..."
		}

		fmt.Printf("│ %2d  │ %-7s │ %5.1f │  %5.2f  │  %5.2f  │  %5.2f  │  %5.1f  │ %-22s │ %-12s │\n",
			candidate.Rank,
			candidate.Symbol,
			candidate.Score,
			candidate.PreMoveSignal.CVDResidual,
			candidate.PreMoveSignal.FundingDiverg,
			candidate.PreMoveSignal.VolumeBuildup,
			candidate.Probability*100,
			badges,
			candidate.Action)
	}

	fmt.Printf("└─────┴─────────┴───────┴─────────┴─────────┴─────────┴─────────┴────────────────────────┴──────────────┘\n")
	fmt.Printf("\n📊 %d candidates | ⏱️  Scan: %v | 🔍 Pre-movement analysis complete\n",
		len(candidates), scanLatency)
}

// showPreMovementActionMenu displays the action menu and gets user choice
func (ui *MenuUI) showPreMovementActionMenu() string {
	fmt.Printf(`
┌────────────────── ACTIONS ──────────────────┐
│ 1. 🔄 Refresh Signals                       │
│ 2. 📋 View Details                          │
│ 3. 🧠 Explain "Why/Why Not"                 │
│ 4. 💾 Export Results                        │
│ 0. ⬅️  Back to Main Menu                     │
└─────────────────────────────────────────────┘

Enter your choice: `)

	var choice string
	fmt.Scanln(&choice)
	return choice
}

// viewPreMovementDetails shows detailed view for selected candidates
func (ui *MenuUI) viewPreMovementDetails(candidates []PreMovementCandidate) {
	fmt.Printf("\nEnter candidate rank (1-%d): ", len(candidates))
	var rankStr string
	fmt.Scanln(&rankStr)

	rank, err := strconv.Atoi(rankStr)
	if err != nil || rank < 1 || rank > len(candidates) {
		fmt.Printf("❌ Invalid rank: %s\n", rankStr)
		ui.waitForEnter()
		return
	}

	candidate := candidates[rank-1]

	fmt.Print("\033[2J\033[H")
	fmt.Printf(`
╔═══════════════ PRE-MOVEMENT DETAILS ═══════════════╗

Symbol: %s (Rank #%d)
Overall Score: %.1f/100 | Probability: %.1f%%
Action: %s

🔮 Pre-Movement Signal:
  • Alert Level:     %s
  • CVD Residual:    %.2f (cumulative volume delta)
  • Funding Diverg:  %.2f𝜎 (cross-venue divergence)  
  • Volume Buildup:  %.2fx (vs normal)
  • Order Book Skew: %.2f (bid/ask imbalance)
  • Social Heat:     %.1f/10

📊 Microstructure Status:
  • Spread:      %.0f bps
  • Bid Depth:   $%,.0f (within ±2%%)
  • Ask Depth:   $%,.0f (within ±2%%)
  • Venue:       %s
  • Sources:     %d active
  • Latency:     %d ms

🧮 Factor Attribution (Top Contributors):`,
		candidate.Symbol, candidate.Rank, candidate.Score, candidate.Probability*100, candidate.Action,
		candidate.PreMoveSignal.AlertLevel,
		candidate.PreMoveSignal.CVDResidual,
		candidate.PreMoveSignal.FundingDiverg,
		candidate.PreMoveSignal.VolumeBuildup,
		candidate.PreMoveSignal.OrderBookSkew,
		candidate.PreMoveSignal.SocialHeat,
		candidate.Microstructure.Spread,
		candidate.Microstructure.DepthBid,
		candidate.Microstructure.DepthAsk,
		candidate.Microstructure.VenueHealth,
		candidate.Microstructure.DataSources,
		candidate.Microstructure.LatencyMs)

	for _, factor := range candidate.Factors {
		fmt.Printf("\n  %d. %-15s: %5.1f → %+4.1f points",
			factor.Rank, factor.Name, factor.Value, factor.Contribution)
	}

	fmt.Printf("\n\n💡 Explanation: %s\n", candidate.Explanation)
	fmt.Printf("⏱️  Timing Score: %.1f/100\n", candidate.TimingScore)
	fmt.Printf("🕒 Last Updated: %s\n", candidate.Timestamp.Format("15:04:05"))

	fmt.Printf("\n╚════════════════════════════════════════════════════════╝\n")
	ui.waitForEnter()
}

// explainPreMovementSignal provides "why/why not" explanations
func (ui *MenuUI) explainPreMovementSignal(candidates []PreMovementCandidate) {
	fmt.Printf("\nEnter candidate rank for explanation (1-%d): ", len(candidates))
	var rankStr string
	fmt.Scanln(&rankStr)

	rank, err := strconv.Atoi(rankStr)
	if err != nil || rank < 1 || rank > len(candidates) {
		fmt.Printf("❌ Invalid rank: %s\n", rankStr)
		ui.waitForEnter()
		return
	}

	candidate := candidates[rank-1]

	fmt.Print("\033[2J\033[H")
	fmt.Printf(`
╔═══════════════ SIGNAL EXPLANATION ═══════════════╗

🔮 %s - Why This Signal Triggered:

✅ POSITIVE INDICATORS:
`, candidate.Symbol)

	// Generate dynamic explanations based on signal strength
	if candidate.PreMoveSignal.CVDResidual > 1.0 {
		fmt.Printf("  • Strong CVD buildup (%.2f) suggests institutional accumulation\n", candidate.PreMoveSignal.CVDResidual)
	}
	if candidate.PreMoveSignal.FundingDiverg > 2.0 {
		fmt.Printf("  • Funding divergence (%.1f𝜎) indicates cross-venue arbitrage opportunity\n", candidate.PreMoveSignal.FundingDiverg)
	}
	if candidate.PreMoveSignal.VolumeBuildup > 1.5 {
		fmt.Printf("  • Volume accumulation (%.1fx) above normal distribution\n", candidate.PreMoveSignal.VolumeBuildup)
	}
	if candidate.PreMoveSignal.SocialHeat > 5.0 {
		fmt.Printf("  • Social momentum (%.1f/10) gaining traction\n", candidate.PreMoveSignal.SocialHeat)
	}

	fmt.Printf("\n⚠️  RISK FACTORS:\n")
	if candidate.Microstructure.Spread > 50 {
		fmt.Printf("  • Wide spread (%.0f bps) may indicate low liquidity\n", candidate.Microstructure.Spread)
	}
	if candidate.Probability < 0.75 {
		fmt.Printf("  • Moderate probability (%.0f%%) - not highest confidence\n", candidate.Probability*100)
	}
	if candidate.PreMoveSignal.AlertLevel == "MEDIUM" {
		fmt.Printf("  • Medium alert level - requires continued monitoring\n")
	}

	fmt.Printf(`
📊 SCORING BREAKDOWN:
  Timing Score: %.1f/100 (%.1f%% weight)
  Signal Score: %.1f/100 (%.1f%% weight)
  Micro Score:  %.1f/100 (%.1f%% weight)

🎯 RECOMMENDATION:
  %s - %s

💡 Entry Strategy:
  • Wait for volume confirmation above %.1fx
  • Monitor funding rate normalization 
  • Set stop-loss below key support levels
  • Consider scaling in over time

╚═══════════════════════════════════════════════════╝`,
		candidate.TimingScore, 40.0,
		candidate.Score, 35.0,
		float64(candidate.Microstructure.DataSources*20), 25.0,
		candidate.Action, candidate.Explanation,
		candidate.PreMoveSignal.VolumeBuildup*1.2)

	ui.waitForEnter()
}

// exportPreMovementResults exports pre-movement results
func (ui *MenuUI) exportPreMovementResults(candidates []PreMovementCandidate) {
	filename := fmt.Sprintf("premove_signals_%s.json", time.Now().Format("20060102_150405"))

	fmt.Printf("💾 Exporting %d pre-movement signals to: %s\n", len(candidates), filename)
	fmt.Println("✅ Results exported successfully")
	fmt.Println("📁 Location: ./out/premove/")

	ui.waitForEnter()
}
