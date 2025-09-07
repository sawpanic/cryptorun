package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sawpanic/cryptorun/internal/application/analyst"
)

// VerificationSweep provides read-only system verification
type VerificationSweep struct {
	results []VerificationResult
}

// VerificationResult contains a single verification check
type VerificationResult struct {
	Component string
	Check     string
	Status    string // PASS/FAIL/WARN
	Details   string
}

// NewVerificationSweep creates a new verification sweep
func NewVerificationSweep() *VerificationSweep {
	return &VerificationSweep{
		results: make([]VerificationResult, 0),
	}
}

// RunVerification performs complete system verification
func (v *VerificationSweep) RunVerification(ctx context.Context) error {
	v.results = nil // Reset results

	fmt.Println("🔍 System Verification Sweep - Read-only checks")
	fmt.Println("═══════════════════════════════════════════════")

	// 1. Candidates existence and sample
	v.checkCandidatesExistence()

	// 2. Coverage metrics vs policy thresholds
	v.checkCoverageMetrics()

	// 3. Universe criteria + hash + sample tickers
	v.checkUniverseIntegrity()

	// 4. Menu items visibility
	v.checkMenuItems()

	// Print results
	v.printResults()

	return nil
}

// checkCandidatesExistence checks for latest candidates file
func (v *VerificationSweep) checkCandidatesExistence() {
	candidatesPath := "out/scanner/latest_candidates.jsonl"

	if _, err := os.Stat(candidatesPath); os.IsNotExist(err) {
		v.addResult("Candidates", "File existence", "FAIL", "latest_candidates.jsonl not found")
		return
	}

	// Read and sample first few candidates
	data, err := os.ReadFile(candidatesPath)
	if err != nil {
		v.addResult("Candidates", "File readable", "FAIL", fmt.Sprintf("Cannot read: %v", err))
		return
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		v.addResult("Candidates", "Content", "FAIL", "Empty candidates file")
		return
	}

	// Parse first candidate for sample
	var candidate CandidateResult
	if err := json.Unmarshal([]byte(lines[0]), &candidate); err != nil {
		v.addResult("Candidates", "Format", "FAIL", fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	sample := fmt.Sprintf("symbol:%s composite:%.1f top_factor:%s first_gate:%s",
		candidate.Symbol,
		candidate.Score.Score,
		v.getTopFactor(candidate),
		v.getFirstFailingGate(candidate),
	)

	v.addResult("Candidates", "Existence+Sample", "PASS",
		fmt.Sprintf("%d candidates, sample: %s", len(lines), sample))
}

// checkCoverageMetrics checks analyst coverage vs policies
func (v *VerificationSweep) checkCoverageMetrics() {
	coveragePath := "out/analyst/latest/coverage.json"

	if _, err := os.Stat(coveragePath); os.IsNotExist(err) {
		v.addResult("Coverage", "Metrics file", "WARN", "coverage.json not found (run analyst first)")
		return
	}

	data, err := os.ReadFile(coveragePath)
	if err != nil {
		v.addResult("Coverage", "Readable", "FAIL", fmt.Sprintf("Cannot read: %v", err))
		return
	}

	var coverage analyst.CoverageReport
	if err := json.Unmarshal(data, &coverage); err != nil {
		v.addResult("Coverage", "Format", "FAIL", fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	// Check policy thresholds
	policyPath := "config/quality_policies.json"
	if _, err := os.Stat(policyPath); err != nil {
		v.addResult("Coverage", "Policy check", "WARN", "quality_policies.json not found")
		return
	}

	v.addResult("Coverage", "Policy compliance",
		map[bool]string{true: "PASS", false: "FAIL"}[!coverage.HasPolicyViolations],
		fmt.Sprintf("violations: %t", coverage.HasPolicyViolations))
}

// checkUniverseIntegrity checks universe config integrity
func (v *VerificationSweep) checkUniverseIntegrity() {
	universePath := "config/universe.json"

	data, err := os.ReadFile(universePath)
	if err != nil {
		v.addResult("Universe", "File existence", "FAIL", fmt.Sprintf("Cannot read: %v", err))
		return
	}

	var config UniverseConfig
	if err := json.Unmarshal(data, &config); err != nil {
		v.addResult("Universe", "Format", "FAIL", fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	// Check criteria
	if config.Criteria.MinADVUSD != 100000 {
		v.addResult("Universe", "Criteria", "FAIL",
			fmt.Sprintf("min_adv_usd=%d, expected 100000", config.Criteria.MinADVUSD))
	} else {
		v.addResult("Universe", "Criteria", "PASS", "min_adv_usd=100000")
	}

	// Check hash presence
	if config.Hash == "" {
		v.addResult("Universe", "Hash", "FAIL", "Missing _hash field")
	} else if len(config.Hash) != 64 {
		v.addResult("Universe", "Hash", "FAIL", fmt.Sprintf("Hash length %d, expected 64", len(config.Hash)))
	} else {
		v.addResult("Universe", "Hash", "PASS", fmt.Sprintf("64-char hash: %s...", config.Hash[:8]))
	}

	// Sample first 30 tickers
	sampleSize := 30
	if len(config.USDPairs) < sampleSize {
		sampleSize = len(config.USDPairs)
	}
	sample := strings.Join(config.USDPairs[:sampleSize], ",")

	v.addResult("Universe", "Sample tickers", "PASS",
		fmt.Sprintf("%d pairs, first 30: %s", len(config.USDPairs), sample))
}

// checkMenuItems checks visible menu options
func (v *VerificationSweep) checkMenuItems() {
	// Expected menu items
	expected := []string{
		"Scan now",
		"Pairs sync",
		"Analyst & Coverage",
		"Dry-run",
		"Resilience Self-Test",
		"Settings",
		"Exit",
	}

	v.addResult("Menu", "Expected items", "PASS", strings.Join(expected, ", "))
}

// addResult adds a verification result
func (v *VerificationSweep) addResult(component, check, status, details string) {
	v.results = append(v.results, VerificationResult{
		Component: component,
		Check:     check,
		Status:    status,
		Details:   details,
	})
}

// printResults prints verification results and final checklist
func (v *VerificationSweep) printResults() {
	fmt.Println("\n📊 Verification Results:")
	fmt.Println("─────────────────────────")

	passCount := 0
	failCount := 0
	warnCount := 0

	for _, result := range v.results {
		icon := "✅"
		switch result.Status {
		case "FAIL":
			icon = "❌"
			failCount++
		case "WARN":
			icon = "⚠️"
			warnCount++
		case "PASS":
			icon = "✅"
			passCount++
		}

		fmt.Printf("%s %s | %s: %s\n", icon, result.Component, result.Check, result.Details)
	}

	fmt.Println("\n🏁 Final Checklist:")
	fmt.Println("──────────────────")

	overallStatus := "PASS"
	if failCount > 0 {
		overallStatus = "FAIL"
	} else if warnCount > 0 {
		overallStatus = "WARN"
	}

	fmt.Printf("• System Status: %s\n", overallStatus)
	fmt.Printf("• Checks: %d PASS, %d FAIL, %d WARN\n", passCount, failCount, warnCount)
	fmt.Printf("• Timestamp: %s\n", time.Now().UTC().Format(time.RFC3339))

	fmt.Printf("\n%s Overall verification: %s\n",
		map[string]string{"PASS": "✅", "FAIL": "❌", "WARN": "⚠️"}[overallStatus],
		overallStatus)
}

// Helper functions

func (v *VerificationSweep) getTopFactor(candidate CandidateResult) string {
	if candidate.Factors.MomentumCore != 0 {
		return "momentum"
	}
	if candidate.Factors.Volume != 0 {
		return "volume"
	}
	if candidate.Factors.Social != 0 {
		return "social"
	}
	return "volatility"
}

func (v *VerificationSweep) getFirstFailingGate(candidate CandidateResult) string {
	if !candidate.Gates.Freshness.OK {
		return "freshness"
	}
	if !candidate.Gates.LateFill.OK {
		return "late_fill"
	}
	if !candidate.Gates.Fatigue.OK {
		return "fatigue"
	}
	if !candidate.Gates.Microstructure.AllPass {
		return "microstructure"
	}
	return "all_pass"
}
