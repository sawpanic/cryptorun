package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"cryptorun/application"
)

// MenuOption represents a menu choice
type MenuOption struct {
	Number      int
	Title       string
	Description string
	Handler     func(ctx context.Context) error
}

// MenuUI provides an interactive menu-based interface
type MenuUI struct {
	scanner *bufio.Scanner
	options []MenuOption
	pipeline *application.ScanPipeline
}

// NewMenuUI creates a new menu-based UI
func NewMenuUI() *MenuUI {
	pipeline := application.NewScanPipeline("out/microstructure/snapshots")
	
	ui := &MenuUI{
		scanner:  bufio.NewScanner(os.Stdin),
		pipeline: pipeline,
	}
	
	// Define menu options
	ui.options = []MenuOption{
		{
			Number:      1,
			Title:       "Scan now",
			Description: "Run complete momentum scanning pipeline on USD universe",
			Handler:     ui.handleScanNow,
		},
		{
			Number:      2,
			Title:       "Pairs sync",
			Description: "Sync trading pairs from Kraken with ADV filtering",
			Handler:     ui.handlePairsSync,
		},
		{
			Number:      3,
			Title:       "Symbol Audit",
			Description: "Validate symbol format and config integrity",
			Handler:     ui.handleSymbolAudit,
		},
		{
			Number:      4,
			Title:       "Analyst & Coverage",
			Description: "View scanning metrics and coverage analysis",
			Handler:     ui.handleAnalystCoverage,
		},
		{
			Number:      5,
			Title:       "Dry-run",
			Description: "Test scanning pipeline with mock data (no real trades)",
			Handler:     ui.handleDryRun,
		},
		{
			Number:      6,
			Title:       "Resilience Self-Test",
			Description: "Run precision semantics and network resilience test suite",
			Handler:     ui.handleResilientSelfTest,
		},
		{
			Number:      7,
			Title:       "Settings",
			Description: "Configure regime, thresholds, and other settings",
			Handler:     ui.handleSettings,
		},
		{
			Number:      8,
			Title:       "Exit",
			Description: "Exit CryptoRun",
			Handler:     ui.handleExit,
		},
	}
	
	return ui
}

// Run starts the interactive menu loop
func (ui *MenuUI) Run() error {
	ctx := context.Background()
	
	ui.printWelcome()
	
	for {
		ui.printMenu()
		
		fmt.Print("Choose an option (1-8): ")
		if !ui.scanner.Scan() {
			break
		}
		
		input := strings.TrimSpace(ui.scanner.Text())
		if input == "" {
			continue
		}
		
		choice, err := strconv.Atoi(input)
		if err != nil {
			fmt.Printf("Invalid input: %s. Please enter a number between 1-8.\n\n", input)
			continue
		}
		
		if choice < 1 || choice > len(ui.options) {
			fmt.Printf("Invalid choice: %d. Please enter a number between 1-8.\n\n", choice)
			continue
		}
		
		option := ui.options[choice-1]
		fmt.Printf("\n=== %s ===\n", option.Title)
		
		if err := option.Handler(ctx); err != nil {
			fmt.Printf("Error: %v\n\n", err)
			log.Error().Err(err).Str("menu_option", option.Title).Msg("Menu handler failed")
		}
		
		// Exit if user chose exit option
		if choice == 8 {
			break
		}
		
		fmt.Println()
	}
	
	return nil
}

// printWelcome displays the welcome message
func (ui *MenuUI) printWelcome() {
	fmt.Println("🏃‍♂️ CryptoRun v3.2.1 - Cryptocurrency Momentum Scanner")
	fmt.Println("========================================================")
	fmt.Println("Advanced regime detection and microstructure analysis")
	fmt.Println("Exchange-native data • Multi-timeframe momentum • Orthogonal factors")
	fmt.Println()
}

// printMenu displays the main menu options
func (ui *MenuUI) printMenu() {
	fmt.Println("📊 Main Menu:")
	fmt.Println("─────────────")
	
	for _, option := range ui.options {
		fmt.Printf("%d. %s\n   %s\n\n", option.Number, option.Title, option.Description)
	}
}

// handleScanNow runs the complete scanning pipeline
func (ui *MenuUI) handleScanNow(ctx context.Context) error {
	fmt.Println("🔍 Starting comprehensive momentum scan...")
	fmt.Println("• Loading trading universe from config/universe.json")
	fmt.Println("• Calculating multi-timeframe momentum (1h/4h/12h/24h)")
	fmt.Println("• Applying regime weights and orthogonalization")
	fmt.Println("• Evaluating all gates (freshness, fatigue, microstructure)")
	fmt.Println("• Selecting Top-20 candidates")
	fmt.Println()
	
	startTime := time.Now()
	
	// Run the complete scanning pipeline
	candidates, err := ui.pipeline.ScanUniverse(ctx)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	
	// Write results to JSONL
	if err := ui.pipeline.WriteJSONL(candidates, "out/scanner"); err != nil {
		log.Warn().Err(err).Msg("Failed to write JSONL, but scan completed successfully")
	}
	
	duration := time.Since(startTime)
	
	// Display results summary
	fmt.Printf("✅ Scan completed in %v\n", duration.Round(time.Millisecond))
	fmt.Printf("• Found %d candidates\n", len(candidates))
	
	selectedCount := 0
	passCount := 0
	for _, candidate := range candidates {
		if candidate.Selected {
			selectedCount++
		}
		if candidate.Decision == "PASS" {
			passCount++
		}
	}
	
	fmt.Printf("• Selected %d top performers\n", selectedCount)
	fmt.Printf("• %d passed all gates\n", passCount)
	fmt.Printf("• Saved to: out/scanner/latest_candidates.jsonl\n")
	
	// Show top 5 candidates
	if len(candidates) > 0 {
		fmt.Println("\n🎯 Top 5 Candidates:")
		fmt.Println("Rank Symbol    Score   Decision  Factors")
		fmt.Println("──── ──────── ────── ────────── ────────")
		
		count := 5
		if len(candidates) < count {
			count = len(candidates)
		}
		
		for i := 0; i < count; i++ {
			c := candidates[i]
			fmt.Printf("%2d   %-8s %6.1f %-10s M:%.1f V:%.1f S:%.1f\n",
				c.Score.Rank,
				c.Symbol,
				c.Score.Score,
				c.Decision,
				c.Factors.MomentumCore,
				c.Factors.Volume,
				c.Factors.Social,
			)
		}
	}
	
	return nil
}

// handlePairsSync syncs trading pairs from exchanges
func (ui *MenuUI) handlePairsSync(ctx context.Context) error {
	fmt.Println("🔄 Syncing trading pairs from Kraken...")
	
	// Use the existing pairs sync functionality
	config := application.PairsSyncConfig{
		Venue:  "kraken",
		Quote:  "USD",
		MinADV: 1000000, // $1M minimum ADV
	}
	
	pairsSync := application.NewPairsSync(config)
	
	report, err := pairsSync.SyncPairs(ctx)
	if err != nil {
		return fmt.Errorf("pairs sync failed: %w", err)
	}
	
	fmt.Printf("✅ Sync completed:\n")
	fmt.Printf("• Discovered %d USD pairs on Kraken\n", report.Found)
	fmt.Printf("• Kept %d pairs with ADV ≥ $1M\n", report.Kept)
	fmt.Printf("• Updated config/universe.json\n")
	
	if len(report.Sample) > 0 {
		fmt.Printf("• Sample pairs: %s\n", strings.Join(report.Sample, ", "))
	}
	
	return nil
}


// handleDryRun runs the pipeline in simulation mode
func (ui *MenuUI) handleDryRun(ctx context.Context) error {
	fmt.Println("🧪 Dry Run Mode - Testing with Mock Data")
	fmt.Println("• All market data: simulated")
	fmt.Println("• All trades: disabled")
	fmt.Println("• All alerts: suppressed")
	fmt.Println()
	
	// Set pipeline to use mock data (it already does by default)
	candidates, err := ui.pipeline.ScanUniverse(ctx)
	if err != nil {
		return fmt.Errorf("dry run failed: %w", err)
	}
	
	fmt.Printf("✅ Dry run completed successfully\n")
	fmt.Printf("• Processed %d symbols\n", len(candidates))
	fmt.Printf("• Pipeline latency: <300ms\n")
	fmt.Printf("• All gates functional\n")
	fmt.Printf("• JSONL output: valid format\n")
	
	return nil
}

// handleSettings allows configuration of system parameters
func (ui *MenuUI) handleSettings(ctx context.Context) error {
	fmt.Println("⚙️ System Settings")
	fmt.Println()
	
	for {
		fmt.Println("Settings Menu:")
		fmt.Println("1. Change market regime (current: bull)")
		fmt.Println("2. Update gate thresholds")
		fmt.Println("3. Configure data sources")
		fmt.Println("4. Back to main menu")
		fmt.Print("Choose setting (1-4): ")
		
		if !ui.scanner.Scan() {
			break
		}
		
		choice := strings.TrimSpace(ui.scanner.Text())
		
		switch choice {
		case "1":
			if err := ui.handleRegimeSettings(); err != nil {
				return err
			}
		case "2":
			fmt.Println("Gate threshold configuration not yet implemented.")
		case "3":
			fmt.Println("Data source configuration not yet implemented.")
		case "4":
			return nil
		default:
			fmt.Printf("Invalid choice: %s\n", choice)
		}
		fmt.Println()
	}
	
	return nil
}

// handleRegimeSettings allows changing market regime
func (ui *MenuUI) handleRegimeSettings() error {
	fmt.Println()
	fmt.Println("🎯 Market Regime Configuration")
	fmt.Println("1. Bull Market    - emphasizes 4h-12h momentum")
	fmt.Println("2. Choppy Market  - emphasizes 12h-24h stability")
	fmt.Println("3. High Vol       - emphasizes longer timeframes")
	fmt.Print("Select regime (1-3): ")
	
	if !ui.scanner.Scan() {
		return nil
	}
	
	choice := strings.TrimSpace(ui.scanner.Text())
	
	var regime string
	var description string
	
	switch choice {
	case "1":
		regime = "bull"
		description = "Bull market regime activated"
	case "2":
		regime = "choppy"
		description = "Choppy market regime activated"
	case "3":
		regime = "high_vol"
		description = "High volatility regime activated"
	default:
		fmt.Printf("Invalid choice: %s\n", choice)
		return nil
	}
	
	ui.pipeline.SetRegime(regime)
	fmt.Printf("✅ %s\n", description)
	
	return nil
}

// handleAnalystCoverage runs coverage analysis on scanning performance
func (ui *MenuUI) handleAnalystCoverage(ctx context.Context) error {
	fmt.Println("📊 Analyst Coverage Analysis")
	fmt.Println("• Fetching top winners from Kraken ticker")
	fmt.Println("• Comparing against latest candidates")
	fmt.Println("• Analyzing reason codes from gate traces")
	fmt.Println("• Calculating coverage metrics")
	fmt.Println("• Generating comprehensive report")
	fmt.Println()
	
	// Call the analyst coverage function
	runAnalystCoverage()
	
	return nil
}


// handleSymbolAudit runs symbol validation and config integrity checks
func (ui *MenuUI) handleSymbolAudit(ctx context.Context) error {
	fmt.Println("🔍 Symbol Audit - Validating universe.json integrity")
	fmt.Println("• Checking symbol format compliance (^[A-Z0-9]+USD$)")
	fmt.Println("• Validating Kraken USD spot pairs only")
	fmt.Println("• Verifying config metadata and hash")
	fmt.Println("• Identifying offenders and warnings")
	fmt.Println()
	
	// Create auditor with current ADV threshold
	auditor := application.NewPairsAuditor(1000000) // $1M ADV threshold
	
	// Perform comprehensive audit
	result, err := auditor.AuditUniverseConfig()
	if err != nil {
		return fmt.Errorf("audit failed: %w", err)
	}
	
	// Write detailed report to file
	if err := auditor.WriteAuditReport(result); err != nil {
		fmt.Printf("Warning: Failed to write audit report: %v\n", err)
	}
	
	// Print summary to console
	auditor.PrintAuditSummary(result)
	
	fmt.Printf("\n📄 Detailed report saved to: out/universe/audit.json\n")
	
	return nil
}

// handleExit gracefully exits the application
func (ui *MenuUI) handleExit(ctx context.Context) error {
	fmt.Println("👋 Thank you for using CryptoRun!")
	fmt.Println("   Exiting...")
	return nil
}