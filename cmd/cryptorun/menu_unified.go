package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"cryptorun/internal/application/bench"
	"cryptorun/internal/application/metrics"
	"cryptorun/internal/application/pipeline"
	"cryptorun/internal/ui/menu"
	"cryptorun/src/application/premove"
)

// MenuUnifiedHandlers contains handlers that call the same pipeline functions as CLI commands
type MenuUnifiedHandlers struct {
	// Shared state if needed
}

// NewMenuUnifiedHandlers creates handlers that route to unified pipelines
func NewMenuUnifiedHandlers() *MenuUnifiedHandlers {
	return &MenuUnifiedHandlers{}
}

// handleScanMomentum executes momentum scan via the same pipeline as CLI
func (h *MenuUnifiedHandlers) handleScanMomentum(ctx context.Context) error {
	fmt.Println("🚀 Executing momentum scan via unified pipeline...")

	// Use the SAME pipeline function as cmd_scan.go
	opts := pipeline.ScanOptions{
		Exchange:    "kraken",
		Pairs:       "USD-only",
		DryRun:      false,
		OutputDir:   "out/scanner",
		SnapshotDir: "out/scanner/snapshots",
		MaxSymbols:  50,
		MinScore:    2.0,
		Regime:      "trending",
		ConfigFile:  "",
	}

	// SINGLE PIPELINE CALL - exactly the same as CLI
	result, artifacts, err := pipeline.Run(ctx, opts)
	if err != nil {
		return fmt.Errorf("momentum scan failed: %w", err)
	}

	// Display results (reuse from CLI)
	displayScanResults(result, artifacts)
	return nil
}

// handleScanDip executes dip scan via the same pipeline as CLI
func (h *MenuUnifiedHandlers) handleScanDip(ctx context.Context) error {
	fmt.Println("📉 Executing dip scan via unified pipeline...")

	// Use the SAME pipeline function with dip-specific configuration
	opts := pipeline.ScanOptions{
		Exchange:    "kraken",
		Pairs:       "USD-only",
		DryRun:      false,
		OutputDir:   "out/scanner",
		SnapshotDir: "out/scanner/snapshots",
		MaxSymbols:  50,
		MinScore:    1.5,               // Lower threshold for dips
		Regime:      "choppy",          // Dip-appropriate regime
		ConfigFile:  "config/dip.yaml", // Dip-specific config
	}

	// SINGLE PIPELINE CALL - same function, different config
	result, artifacts, err := pipeline.Run(ctx, opts)
	if err != nil {
		return fmt.Errorf("dip scan failed: %w", err)
	}

	// Display results (reuse from CLI)
	displayScanResults(result, artifacts)
	return nil
}

// handleBenchTopGainers executes top gainers benchmark via same pipeline as CLI
func (h *MenuUnifiedHandlers) handleBenchTopGainers(ctx context.Context) error {
	fmt.Println("📈 Executing top gainers benchmark via unified pipeline...")

	// Use the SAME pipeline function as cmd_bench.go
	opts := bench.TopGainersOptions{
		TTL:        15 * time.Minute,
		Limit:      20,
		Windows:    []string{"1h", "24h"},
		OutputDir:  "out/bench",
		DryRun:     false,
		APIBaseURL: "",
		ConfigFile: "",
	}

	// SINGLE PIPELINE CALL - exactly the same as CLI
	result, artifacts, err := bench.Run(ctx, opts)
	if err != nil {
		return fmt.Errorf("top gainers benchmark failed: %w", err)
	}

	// Display results (reuse from CLI)
	displayTopGainersResults(result, artifacts)
	return nil
}

// handleBenchDiagnostics executes diagnostics via same pipeline as CLI
func (h *MenuUnifiedHandlers) handleBenchDiagnostics(ctx context.Context) error {
	fmt.Println("🔍 Executing diagnostics via unified pipeline...")

	// Use the SAME pipeline function as cmd_bench.go
	opts := bench.DiagnosticsOptions{
		OutputDir:         "out/bench/diagnostics",
		AlignmentScore:    0.60, // Default based on recent benchmark
		BenchmarkWindow:   "1h",
		DetailLevel:       "high",
		ConfigFile:        "",
		IncludeSparklines: true,
	}

	// SINGLE PIPELINE CALL - exactly the same as CLI
	result, artifacts, err := bench.RunDiagnostics(ctx, opts)
	if err != nil {
		return fmt.Errorf("diagnostics failed: %w", err)
	}

	// Display results (reuse from CLI)
	displayDiagnosticsResults(result, artifacts)
	return nil
}

// handleHealthCheck executes health check via same pipeline as CLI
func (h *MenuUnifiedHandlers) handleHealthCheck(ctx context.Context) error {
	fmt.Println("🏥 Executing health check via unified pipeline...")

	// Use the SAME pipeline function as cmd_health.go
	opts := metrics.HealthOptions{
		IncludeMetrics:  true,
		IncludeCounters: true,
		Format:          "table",
		OutputFile:      "",
	}

	// SINGLE PIPELINE CALL - exactly the same as CLI
	snapshot, err := metrics.Snapshot(ctx, opts)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Display results (reuse from CLI)
	return outputHealthTable(snapshot, "")
}

// handlePreMoveBoard launches the real-time premove detection board
func (h *MenuUnifiedHandlers) handlePreMoveBoard(ctx context.Context) error {
	fmt.Println("🎯 Launching PreMove Detection Board...")

	// Initialize premove components (using mock runner for now)
	var runner *premove.Runner // Would be initialized with actual dependencies

	// Create the SSE-throttled board UI
	boardUI := menu.NewPreMoveBoardUI(runner)
	defer boardUI.Shutdown()

	// Start the interactive console display
	fmt.Println("📊 PreMove Board initialized with ≤1 Hz SSE throttling")
	fmt.Println("💡 Web dashboard available at: /premove/board")
	fmt.Println("🔧 Commands: 'r' = refresh, 'q' = quit")
	fmt.Println()

	// Display initial board state
	boardUI.DisplayConsoleBoard()

	// Interactive loop
	for {
		fmt.Print("Command (r/q): ")
		var input string
		fmt.Scanln(&input)

		switch input {
		case "r", "refresh":
			boardUI.ForceRefresh()
			boardUI.DisplayConsoleBoard()
		case "q", "quit", "exit":
			fmt.Println("🏁 Shutting down PreMove Board...")
			return nil
		default:
			fmt.Printf("❌ Unknown command: %s (use 'r' or 'q')\n", input)
		}
	}
}

// Updated menu handlers that use the unified functions

func (ui *MenuUI) handleScanUnified() error {
	fmt.Printf(`
╔══════════════ SCAN MENU ══════════════╗

 1. 🚀 Momentum Scan (Multi-timeframe)
 2. 📉 Dip Scan (Quality pullbacks)  
 3. ⚙️  Configure Regime (bull/choppy/high_vol)
 0. ← Back to Main Menu

╚═══════════════════════════════════════╝

Enter choice: `)

	var choice string
	fmt.Scanln(&choice)

	ctx := context.Background()
	handlers := NewMenuUnifiedHandlers()

	switch choice {
	case "1":
		if err := handlers.handleScanMomentum(ctx); err != nil {
			log.Error().Err(err).Msg("Momentum scan failed")
			fmt.Printf("❌ Momentum scan failed: %v\n", err)
		}
	case "2":
		if err := handlers.handleScanDip(ctx); err != nil {
			log.Error().Err(err).Msg("Dip scan failed")
			fmt.Printf("❌ Dip scan failed: %v\n", err)
		}
	case "3":
		return ui.configureRegime()
	case "0":
		return nil
	default:
		fmt.Printf("❌ Invalid choice: %s\n", choice)
	}

	ui.waitForEnter()
	return nil
}

func (ui *MenuUI) handleBenchUnified() error {
	fmt.Printf(`
╔════════════ BENCHMARK MENU ════════════╗

 1. 📈 Top Gainers Alignment (CoinGecko)
 2. 🔍 Diagnostics Analysis
 3. 📋 View Last Benchmark Report
 0. ← Back to Main Menu

╚════════════════════════════════════════╝

Enter choice: `)

	var choice string
	fmt.Scanln(&choice)

	ctx := context.Background()
	handlers := NewMenuUnifiedHandlers()

	switch choice {
	case "1":
		if err := handlers.handleBenchTopGainers(ctx); err != nil {
			log.Error().Err(err).Msg("Top gainers benchmark failed")
			fmt.Printf("❌ Benchmark failed: %v\n", err)
		}
	case "2":
		if err := handlers.handleBenchDiagnostics(ctx); err != nil {
			log.Error().Err(err).Msg("Diagnostics failed")
			fmt.Printf("❌ Diagnostics failed: %v\n", err)
		}
	case "3":
		fmt.Println("📋 Opening last benchmark report...")
		fmt.Println("See: out/bench/topgainers_alignment.md")
	case "0":
		return nil
	default:
		fmt.Printf("❌ Invalid choice: %s\n", choice)
	}

	ui.waitForEnter()
	return nil
}

func (ui *MenuUI) handleMonitorUnified() error {
	fmt.Printf(`
╔═══════════ MONITOR MENU ═══════════╗

 1. 🏥 System Health Check
 2. 📊 Performance Metrics
 3. 🔄 API Status Check
 4. 🎯 PreMove Detection Board (Live)
 0. ← Back to Main Menu

╚═══════════════════════════════════════╝

Enter choice: `)

	var choice string
	fmt.Scanln(&choice)

	ctx := context.Background()
	handlers := NewMenuUnifiedHandlers()

	switch choice {
	case "1", "2": // Both route to comprehensive health check
		if err := handlers.handleHealthCheck(ctx); err != nil {
			log.Error().Err(err).Msg("Health check failed")
			fmt.Printf("❌ Health check failed: %v\n", err)
		}
	case "3":
		fmt.Println("🔄 API status integrated into health check above")
	case "4":
		if err := handlers.handlePreMoveBoard(ctx); err != nil {
			log.Error().Err(err).Msg("PreMove board failed")
			fmt.Printf("❌ PreMove board failed: %v\n", err)
		}
	case "0":
		return nil
	default:
		fmt.Printf("❌ Invalid choice: %s\n", choice)
	}

	ui.waitForEnter()
	return nil
}
