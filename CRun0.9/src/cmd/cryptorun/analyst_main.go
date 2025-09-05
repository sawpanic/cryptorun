package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	"cryptorun/application/analyst"
)

// handleAnalystCoverage performs comprehensive coverage analysis
func (ui *MenuUI) handleAnalystCoverage(ctx context.Context) error {
	fmt.Println("📈 Starting Analyst Coverage Analysis...")
	fmt.Println("• Fetching top performers from Kraken (with fixture fallback)")
	fmt.Println("• Loading latest scan candidates")
	fmt.Println("• Computing coverage metrics across 1h/24h/7d windows")
	fmt.Println("• Analyzing misses with reason codes")
	fmt.Println("• Enforcing quality policies")
	fmt.Println()

	startTime := time.Now()

	// Create analyst runner
	outputDir := "out/analyst"
	candidatesFile := "out/scanner/latest_candidates.jsonl"
	
	runner := analyst.NewAnalystRunner(outputDir, candidatesFile)

	// Run coverage analysis
	report, err := runner.RunCoverageAnalysis(ctx)
	if err != nil {
		return fmt.Errorf("coverage analysis failed: %w", err)
	}

	duration := time.Since(startTime)

	// Display results
	fmt.Printf("✅ Coverage analysis completed in %v\n", duration.Round(time.Millisecond))
	fmt.Println()
	
	// Summary metrics
	fmt.Println("📊 Coverage Summary:")
	fmt.Printf("• 1h window:  %d winners, %d hits, %.1f%% recall (%.1f%% miss rate)\n",
		report.Coverage1h.TotalWinners,
		report.Coverage1h.Hits,
		report.Coverage1h.RecallAt20*100,
		report.Coverage1h.BadMissRate*100)
	
	fmt.Printf("• 24h window: %d winners, %d hits, %.1f%% recall (%.1f%% miss rate)\n",
		report.Coverage24h.TotalWinners,
		report.Coverage24h.Hits,
		report.Coverage24h.RecallAt20*100,
		report.Coverage24h.BadMissRate*100)
	
	fmt.Printf("• 7d window:  %d winners, %d hits, %.1f%% recall (%.1f%% miss rate)\n",
		report.Coverage7d.TotalWinners,
		report.Coverage7d.Hits,
		report.Coverage7d.RecallAt20*100,
		report.Coverage7d.BadMissRate*100)
	
	fmt.Println()

	// Policy compliance
	policyIcon := "✅"
	policyStatus := "PASS"
	if !report.PolicyPass {
		policyIcon = "❌"
		policyStatus = "FAIL"
	}
	
	fmt.Printf("%s Quality Policy: %s\n", policyIcon, policyStatus)
	
	// Show threshold details
	if report.Coverage1h.ThresholdBreach {
		fmt.Printf("  ⚠️  1h miss rate %.1f%% exceeds threshold %.1f%%\n",
			report.Coverage1h.BadMissRate*100,
			report.Coverage1h.PolicyThreshold*100)
	}
	
	if report.Coverage24h.ThresholdBreach {
		fmt.Printf("  ⚠️  24h miss rate %.1f%% exceeds threshold %.1f%%\n",
			report.Coverage24h.BadMissRate*100,
			report.Coverage24h.PolicyThreshold*100)
	}
	
	if report.Coverage7d.ThresholdBreach {
		fmt.Printf("  ⚠️  7d miss rate %.1f%% exceeds threshold %.1f%%\n",
			report.Coverage7d.BadMissRate*100,
			report.Coverage7d.PolicyThreshold*100)
	}
	
	fmt.Println()

	// Top misses
	if len(report.TopMisses) > 0 {
		fmt.Println("🎯 Top Misses:")
		fmt.Println("Symbol    TF   Perf%   Reason")
		fmt.Println("──────── ──── ────── ──────────────")
		
		count := len(report.TopMisses)
		if count > 5 {
			count = 5
		}
		
		for i := 0; i < count; i++ {
			miss := report.TopMisses[i]
			fmt.Printf("%-8s %-4s %6.1f%% %s\n",
				miss.Symbol,
				miss.TimeFrame,
				miss.Performance,
				miss.ReasonCode)
		}
		
		if len(report.TopMisses) > 5 {
			fmt.Printf("... and %d more (see misses.jsonl)\n", len(report.TopMisses)-5)
		}
		fmt.Println()
	}

	// File outputs
	fmt.Println("📁 Files Generated:")
	fmt.Printf("• out/analyst/winners.json   - Top performers data\n")
	fmt.Printf("• out/analyst/misses.jsonl   - Detailed miss analysis\n")
	fmt.Printf("• out/analyst/coverage.json  - Machine-readable metrics\n")
	fmt.Printf("• out/analyst/report.md      - Human-readable report\n")
	fmt.Println()

	// Check if we should fail the run due to policy violations
	if !report.PolicyPass {
		fmt.Println("❌ Quality policy violations detected!")
		fmt.Println("   This would cause a non-zero exit in automated runs.")
		
		// In menu mode, we don't actually exit, but we log the failure
		log.Error().
			Float64("miss_rate_1h", report.Coverage1h.BadMissRate).
			Float64("miss_rate_24h", report.Coverage24h.BadMissRate).
			Float64("miss_rate_7d", report.Coverage7d.BadMissRate).
			Msg("Quality policy thresholds breached")
		
		// Note: In a standalone analyst command, we would do os.Exit(1) here
		// But in menu mode, we just warn and continue
	} else {
		fmt.Println("✅ All quality thresholds met")
	}

	return nil
}

// RunStandaloneAnalyst runs coverage analysis with policy enforcement (for CLI use)
func RunStandaloneAnalyst(outputDir, candidatesFile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	runner := analyst.NewAnalystRunner(outputDir, candidatesFile)
	report, err := runner.RunCoverageAnalysis(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Coverage analysis failed: %v\n", err)
		return err
	}

	// Print summary to stdout
	fmt.Println(report.Summary)

	// Enforce policy - exit with non-zero code if thresholds breached
	if !report.PolicyPass {
		fmt.Fprintf(os.Stderr, "Quality policy violations detected:\n")
		
		if report.Coverage1h.ThresholdBreach {
			fmt.Fprintf(os.Stderr, "  1h miss rate %.1f%% > %.1f%% threshold\n",
				report.Coverage1h.BadMissRate*100,
				report.Coverage1h.PolicyThreshold*100)
		}
		
		if report.Coverage24h.ThresholdBreach {
			fmt.Fprintf(os.Stderr, "  24h miss rate %.1f%% > %.1f%% threshold\n",
				report.Coverage24h.BadMissRate*100,
				report.Coverage24h.PolicyThreshold*100)
		}
		
		if report.Coverage7d.ThresholdBreach {
			fmt.Fprintf(os.Stderr, "  7d miss rate %.1f%% > %.1f%% threshold\n",
				report.Coverage7d.BadMissRate*100,
				report.Coverage7d.PolicyThreshold*100)
		}
		
		os.Exit(1) // Non-zero exit for policy violation
	}

	return nil
}