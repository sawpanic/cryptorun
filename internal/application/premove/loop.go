package premove

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/premove"
	"github.com/sawpanic/cryptorun/internal/interfaces/alerts"
)

type PremoveLoop struct {
	detector *premove.Detector
	emitter  *alerts.Emitter
}

func NewPremoveLoop() *PremoveLoop {
	return &PremoveLoop{
		detector: premove.NewDetector(),
		emitter:  alerts.NewEmitter(),
	}
}

func (pl *PremoveLoop) RunOnce() error {
	timestamp := time.Now().Format("20060102_150405")
	artifactDir := filepath.Join("C:", "CryptoRun", "artifacts", "premove", timestamp)
	
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("failed to create premove artifact directory: %w", err)
	}
	
	fmt.Println("🎯 Running hourly pre-movement detection...")
	
	// Execute 2-of-3 gate detection
	results, err := pl.detector.DetectPreMovement()
	if err != nil {
		return fmt.Errorf("pre-movement detection failed: %w", err)
	}
	
	// Emit alerts JSON
	alertsPath := filepath.Join(artifactDir, "alerts.json")
	if err := pl.emitter.EmitAlertsJSON(alertsPath, results); err != nil {
		return fmt.Errorf("failed to emit alerts JSON: %w", err)
	}
	
	// Emit explain JSON  
	explainPath := filepath.Join(artifactDir, "explain.json")
	if err := pl.emitter.EmitExplainJSON(explainPath, results); err != nil {
		return fmt.Errorf("failed to emit explain JSON: %w", err)
	}
	
	// Print summary
	fmt.Printf("✅ Pre-movement detection complete: %d candidates\n", len(results.Candidates))
	fmt.Printf("📁 Artifacts: %s\n", artifactDir)
	
	// Show top 5 with gate flags
	pl.printTop5Summary(results.Candidates[:min(5, len(results.Candidates))])
	
	return nil
}

func (pl *PremoveLoop) printTop5Summary(candidates []premove.Candidate) {
	if len(candidates) == 0 {
		fmt.Println("No pre-movement candidates found")
		return
	}
	
	fmt.Println("\n🎯 Top 5 Pre-Movement Candidates:")
	fmt.Println("Symbol   | Score | Gate A | Gate B | Gate C | Regime  | VADR")
	fmt.Println("---------|-------|--------|--------|--------|---------|------")
	
	for _, candidate := range candidates {
		fmt.Printf("%-8s | %5.1f | %-6s | %-6s | %-6s | %-7s | %4.1f\n",
			candidate.Symbol,
			candidate.Score,
			formatGateStatus(candidate.Gates.FundingDivergence),
			formatGateStatus(candidate.Gates.SupplySqueeze),
			formatGateStatus(candidate.Gates.WhaleAccumulation),
			candidate.Regime,
			candidate.VADR,
		)
	}
	fmt.Println()
}

func formatGateStatus(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}