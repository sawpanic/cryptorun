package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// DryrunResult contains the results of a dry run execution
type DryrunResult struct {
	Timestamp    time.Time                   `json:"timestamp"`
	PairsCount   int                         `json:"pairs_count"`
	Candidates   int                         `json:"candidates"`
	Selected     int                         `json:"selected"`
	Coverage     map[string]float64          `json:"coverage_20"` // recall@20 by timeframe
	Reasons      map[string]int              `json:"reasons"`      // reason code counts
	Status       string                      `json:"status"`       // PASS/FAIL
	Duration     time.Duration               `json:"duration"`
	PolicyCheck  bool                        `json:"policy_check"`
}

// DryrunExecutor handles complete dry run workflow
type DryrunExecutor struct {
	scanPipeline   *ScanPipeline
}

// NewDryrunExecutor creates a new dry run executor
func NewDryrunExecutor() *DryrunExecutor {
	scanPipeline := NewScanPipeline("out/microstructure/snapshots")
	
	return &DryrunExecutor{
		scanPipeline:  scanPipeline,
	}
}

// ExecuteDryrun runs the complete dry run workflow
func (d *DryrunExecutor) ExecuteDryrun(ctx context.Context) (*DryrunResult, error) {
	startTime := time.Now()
	
	result := &DryrunResult{
		Timestamp: startTime,
		Coverage:  make(map[string]float64),
		Reasons:   make(map[string]int),
		Status:    "PASS",
	}
	
	// Step 1: Load universe
	universe, err := d.loadUniverse()
	if err != nil {
		return nil, fmt.Errorf("failed to load universe: %w", err)
	}
	result.PairsCount = len(universe)
	
	// Step 2: Run scan pipeline
	candidates, err := d.runScanPipeline(ctx, universe)
	if err != nil {
		return nil, fmt.Errorf("failed to run scan: %w", err)
	}
	result.Candidates = len(candidates)
	
	// Count selected candidates
	selectedCount := 0
	for _, candidate := range candidates {
		if candidate.Selected {
			selectedCount++
		}
	}
	result.Selected = selectedCount
	
	// Step 3: Run analyst coverage (using fixture data)
	coverage, err := RunCoverageAnalysis(ctx, AnalystConfig{
		UseFixtures: true,
		Timeframes:  []string{"1h", "24h", "7d"},
		OutputDir:   "out/analyst",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run analyst: %w", err)
	}
	
	// Extract coverage metrics and reason codes
	if coverage != nil {
		for timeframe, metrics := range coverage.Metrics {
			if metrics != nil {
				result.Coverage[timeframe] = metrics.RecallAt20
			}
		}
		
		// Count reason codes from misses
		for _, miss := range coverage.Misses {
			result.Reasons[miss.ReasonCode]++
		}
		
		// Check policy violations
		if coverage.HasPolicyViolations {
			result.Status = "FAIL"
			result.PolicyCheck = false
		} else {
			result.PolicyCheck = true
		}
	}
	
	result.Duration = time.Since(startTime)
	
	// Step 4: Append to CHANGELOG
	err = d.appendDryrunToChangelog(result)
	if err != nil {
		return nil, fmt.Errorf("failed to update changelog: %w", err)
	}
	
	return result, nil
}

// loadUniverse loads the trading universe from config
func (d *DryrunExecutor) loadUniverse() ([]string, error) {
	data, err := os.ReadFile("config/universe.json")
	if err != nil {
		return nil, err
	}
	
	var config UniverseConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return config.USDPairs, nil
}

// runScanPipeline executes the scan pipeline with mock data
func (d *DryrunExecutor) runScanPipeline(ctx context.Context, universe []string) ([]CandidateResult, error) {
	// Use a subset for dry run to keep it fast
	testUniverse := universe
	if len(universe) > 20 {
		testUniverse = universe[:20]
	}
	
	var candidates []CandidateResult
	for _, symbol := range testUniverse {
		candidate, err := d.scanPipeline.ScanSymbol(ctx, symbol)
		if err != nil {
			continue // Skip errors in dry run
		}
		candidates = append(candidates, *candidate)
	}
	
	return candidates, nil
}

// runAnalystCoverage runs analyst coverage analysis (removed - using direct function)

// appendDryrunToChangelog appends the DRYRUN line to CHANGELOG.md
func (d *DryrunExecutor) appendDryrunToChangelog(result *DryrunResult) error {
	// Format coverage as 1h:X%,24h:Y%,7d:Z%
	covStr := ""
	for _, tf := range []string{"1h", "24h", "7d"} {
		if cov, exists := result.Coverage[tf]; exists {
			if covStr != "" {
				covStr += ","
			}
			covStr += fmt.Sprintf("%s:%.0f%%", tf, cov*100)
		}
	}
	
	// Format reasons as [REASON:count,...]
	reasonStr := "["
	first := true
	for reason, count := range result.Reasons {
		if !first {
			reasonStr += ","
		}
		reasonStr += fmt.Sprintf("%s:%d", reason, count)
		first = false
	}
	reasonStr += "]"
	
	// Create DRYRUN line
	dryrunLine := fmt.Sprintf("DRYRUN: ts=%s pairs=%d candidates=%d cov20={%s} reasons=%s status=%s",
		result.Timestamp.Format(time.RFC3339),
		result.PairsCount,
		result.Candidates,
		covStr,
		reasonStr,
		result.Status,
	)
	
	// Append to CHANGELOG.md
	file, err := os.OpenFile("CHANGELOG.md", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	
	_, err = file.WriteString("\n" + dryrunLine + "\n")
	return err
}

// PrintSummary prints the 4-line dry run summary
func (d *DryrunExecutor) PrintSummary(result *DryrunResult) {
	fmt.Printf("=== Dry-run Summary ===\n")
	fmt.Printf("Scanned %d pairs → %d candidates (%d selected) in %.2fs\n", 
		result.PairsCount, result.Candidates, result.Selected, result.Duration.Seconds())
	fmt.Printf("Coverage: 1h:%.0f%% 24h:%.0f%% 7d:%.0f%%\n",
		result.Coverage["1h"]*100, result.Coverage["24h"]*100, result.Coverage["7d"]*100)
	fmt.Printf("Top miss reasons: %v\n", d.formatTopReasons(result.Reasons, 3))
	fmt.Printf("Policy check: %s (Status: %s)\n", 
		map[bool]string{true: "PASS", false: "FAIL"}[result.PolicyCheck], result.Status)
}

// formatTopReasons formats top N reason codes by count
func (d *DryrunExecutor) formatTopReasons(reasons map[string]int, topN int) []string {
	type reasonCount struct {
		reason string
		count  int
	}
	
	var sorted []reasonCount
	for reason, count := range reasons {
		sorted = append(sorted, reasonCount{reason, count})
	}
	
	// Sort by count descending
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	var result []string
	for i := 0; i < topN && i < len(sorted); i++ {
		result = append(result, fmt.Sprintf("%s:%d", sorted[i].reason, sorted[i].count))
	}
	
	return result
}