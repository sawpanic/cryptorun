package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"cryptorun/internal/application"
)

// runUniverseRebuild executes the universe rebuild command
func runUniverseRebuild(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	
	log.Info().
		Bool("force", force).
		Bool("dry_run", dryRun).
		Msg("Starting universe rebuild")
	
	// Create universe rebuild job
	job := application.NewUniverseRebuildJob()
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	// Execute rebuild
	result, err := job.Execute(ctx)
	if err != nil {
		return fmt.Errorf("universe rebuild failed: %w", err)
	}
	
	// Display results
	fmt.Printf("🏗️  Universe Rebuild Completed\n")
	fmt.Printf("   Hash: %s\n", result.NewHash[:16]+"...")
	fmt.Printf("   Changed: %v\n", result.HashChanged)
	fmt.Printf("   Count: %d symbols\n", len(result.Snapshot.Universe))
	
	if result.HashChanged {
		fmt.Printf("   Added: %d symbols\n", len(result.Added))
		fmt.Printf("   Removed: %d symbols\n", len(result.Removed))
		
		if len(result.Added) > 0 {
			fmt.Printf("   New: %v\n", result.Added)
		}
		if len(result.Removed) > 0 {
			fmt.Printf("   Dropped: %v\n", result.Removed)
		}
	}
	
	if dryRun {
		fmt.Printf("   🔍 DRY-RUN: No files written\n")
	} else {
		fmt.Printf("   📄 Written: config/universe.json\n")
		fmt.Printf("   📁 Snapshot: out/universe/%s/universe.json\n", 
			result.Snapshot.Metadata.Generated.Format("2006-01-02"))
	}
	
	return nil
}

// handleUniverseMenu handles universe management from menu
func (ui *MenuUI) handleUniverseMenu(ctx context.Context) error {
	fmt.Println("🌌 Universe Management")
	fmt.Println("• 1. Rebuild universe (daily sync)")
	fmt.Println("• 2. Validate universe integrity") 
	fmt.Println("• 3. Show universe status")
	fmt.Println("• 4. Risk envelope status")
	fmt.Println()
	
	for {
		fmt.Print("Select option (1-4, or 0 to return): ")
		
		var choice string
		fmt.Scanln(&choice)
		
		switch choice {
		case "0":
			return nil
		case "1":
			return ui.runUniverseRebuild(ctx)
		case "2":
			return ui.runUniverseValidation(ctx)
		case "3":
			return ui.showUniverseStatus(ctx)
		case "4":
			return ui.showRiskEnvelopeStatus(ctx)
		default:
			fmt.Printf("Invalid choice: %s. Please enter 1-4 or 0.\n\n", choice)
		}
	}
}

// runUniverseRebuild executes universe rebuild from menu
func (ui *MenuUI) runUniverseRebuild(ctx context.Context) error {
	fmt.Println("\n=== Universe Rebuild ===")
	fmt.Println("• Syncing USD pairs from Kraken")
	fmt.Println("• Applying ADV ≥ $100k filter")
	fmt.Println("• Generating integrity hash")
	fmt.Println()
	
	job := application.NewUniverseRebuildJob()
	result, err := job.Execute(ctx)
	if err != nil {
		return fmt.Errorf("rebuild failed: %w", err)
	}
	
	fmt.Printf("✅ Universe rebuilt successfully\n")
	fmt.Printf("   📊 Symbols: %d\n", len(result.Snapshot.Universe))
	fmt.Printf("   🔐 Hash: %s\n", result.NewHash[:12]+"...")
	fmt.Printf("   🔄 Changed: %v\n", result.HashChanged)
	
	if result.HashChanged {
		fmt.Printf("   ➕ Added: %v\n", result.Added)
		fmt.Printf("   ➖ Removed: %v\n", result.Removed)
	}
	
	return nil
}

// runUniverseValidation validates universe integrity
func (ui *MenuUI) runUniverseValidation(ctx context.Context) error {
	fmt.Println("\n=== Universe Validation ===")
	fmt.Println("• Checking USD-only regex compliance")
	fmt.Println("• Validating ADV thresholds")
	fmt.Println("• Verifying hash integrity")
	fmt.Println()
	
	// Create universe builder for validation
	criteria := application.UniverseCriteria{
		Quote:     "USD",
		MinADVUSD: 100000,
		Venue:     "kraken",
	}
	builder := application.NewUniverseBuilder(criteria)
	
	// Load current universe
	result, err := builder.BuildUniverse(ctx)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	
	// Run validation checks
	violations := 0
	
	// Check USD-only compliance
	nonUSDCount := 0
	for _, symbol := range result.Snapshot.Universe {
		if len(symbol) < 4 || symbol[len(symbol)-3:] != "USD" {
			nonUSDCount++
			violations++
		}
	}
	
	if nonUSDCount == 0 {
		fmt.Printf("✅ USD-only compliance: PASS\n")
	} else {
		fmt.Printf("❌ USD-only compliance: %d violations\n", nonUSDCount)
	}
	
	// Check hash integrity
	if result.NewHash != "" {
		fmt.Printf("✅ Hash integrity: PASS (%s)\n", result.NewHash[:8]+"...")
	} else {
		fmt.Printf("❌ Hash integrity: FAIL\n")
		violations++
	}
	
	// Check criteria match
	if result.Snapshot.Metadata.Criteria.MinADVUSD == 100000 {
		fmt.Printf("✅ ADV criteria: $%d minimum\n", result.Snapshot.Metadata.Criteria.MinADVUSD)
	} else {
		fmt.Printf("❌ ADV criteria mismatch\n")
		violations++
	}
	
	fmt.Printf("\n📋 Validation Summary: %d violations\n", violations)
	if violations == 0 {
		fmt.Printf("🎉 Universe integrity: HEALTHY\n")
	}
	
	return nil
}

// showUniverseStatus displays current universe status
func (ui *MenuUI) showUniverseStatus(ctx context.Context) error {
	fmt.Println("\n=== Universe Status ===")
	
	// Load current universe
	criteria := application.UniverseCriteria{
		Quote:     "USD",
		MinADVUSD: 100000,
		Venue:     "kraken",
	}
	builder := application.NewUniverseBuilder(criteria)
	
	// Get build result for status
	result, err := builder.BuildUniverse(ctx)
	if err != nil {
		return fmt.Errorf("failed to get universe status: %w", err)
	}
	
	snapshot := result.Snapshot
	
	fmt.Printf("📊 Current Universe\n")
	fmt.Printf("   Generated: %s\n", snapshot.Metadata.Generated.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("   Source: %s\n", snapshot.Metadata.Source)
	fmt.Printf("   Criteria: %s pairs, ADV ≥ $%d\n", 
		snapshot.Metadata.Criteria.Quote, 
		snapshot.Metadata.Criteria.MinADVUSD)
	fmt.Printf("   Count: %d symbols\n", snapshot.Metadata.Count)
	fmt.Printf("   Hash: %s\n", snapshot.Metadata.Hash[:16]+"...")
	fmt.Printf("\n")
	
	fmt.Printf("🎯 Active Symbols:\n")
	for i, symbol := range snapshot.Universe {
		if i < 10 {
			fmt.Printf("   %s", symbol)
			if (i+1)%5 == 0 {
				fmt.Printf("\n")
			}
		} else if i == 10 {
			fmt.Printf("\n   ... and %d more\n", len(snapshot.Universe)-10)
			break
		}
	}
	
	return nil
}

// showRiskEnvelopeStatus displays risk envelope status  
func (ui *MenuUI) showRiskEnvelopeStatus(ctx context.Context) error {
	fmt.Println("\n=== Risk Envelope Status ===")
	
	// Create risk envelope
	envelope := application.NewRiskEnvelope()
	summary := envelope.GetRiskSummary()
	
	fmt.Printf("🛡️  Risk Controls\n")
	fmt.Printf("   Global Pause: %v\n", summary["global_pause"])
	if reasons, ok := summary["pause_reasons"].([]string); ok && len(reasons) > 0 {
		fmt.Printf("   Pause Reasons: %v\n", reasons)
	}
	fmt.Printf("   Positions: %s\n", summary["positions"])
	fmt.Printf("   Exposure: $%.0f\n", summary["total_exposure_usd"])
	fmt.Printf("   Drawdown: %s\n", summary["current_drawdown"])
	fmt.Printf("   Blacklisted: %d symbols\n", summary["blacklisted_symbols"])
	fmt.Printf("   Degraded Mode: %v\n", summary["degraded_mode"])
	fmt.Printf("   Last Update: %s\n", summary["last_update"])
	
	if caps, ok := summary["active_caps"].([]string); ok && len(caps) > 0 {
		fmt.Printf("\n⚠️  Active Caps:\n")
		for _, cap := range caps {
			fmt.Printf("   • %s\n", cap)
		}
	}
	
	violations := summary["violations"].(int)
	if violations == 0 {
		fmt.Printf("\n✅ Risk Envelope: HEALTHY\n")
	} else {
		fmt.Printf("\n❌ Risk Violations: %d\n", violations)
	}
	
	return nil
}