package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sawpanic/cryptorun/internal/application/analyst"
)

// DryrunResult contains the results of a dry run execution
type DryrunResult struct {
	Timestamp   time.Time          `json:"timestamp"`
	PairsCount  int                `json:"pairs_count"`
	Candidates  int                `json:"candidates"`
	Selected    int                `json:"selected"`
	Coverage    map[string]float64 `json:"coverage_20"` // recall@20 by timeframe
	Reasons     map[string]int     `json:"reasons"`     // reason code counts
	Status      string             `json:"status"`      // PASS/FAIL
	Duration    time.Duration      `json:"duration"`
	PolicyCheck bool               `json:"policy_check"`
}

// DryrunExecutor handles complete dry run workflow
type DryrunExecutor struct {
	scanPipeline *ScanPipeline
}

// NewDryrunExecutor creates a new dry run executor
func NewDryrunExecutor() *DryrunExecutor {
	scanPipeline := NewScanPipeline("out/microstructure/snapshots")

	return &DryrunExecutor{
		scanPipeline: scanPipeline,
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
	coverage, err := analyst.RunCoverageAnalysis(analyst.AnalystConfig{
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

		// Step 3.1: Write coverage to canonical analyst output path
		if err := d.writeCoverageFile(coverage, result.PairsCount); err != nil {
			return nil, fmt.Errorf("failed to write coverage file: %w", err)
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

	// Normalize symbols at ingest boundary to prevent duplication
	normalized := make([]string, 0, len(config.USDPairs))
	for _, symbol := range config.USDPairs {
		normalizedSymbol := d.normalizeSymbol(symbol)
		normalized = append(normalized, normalizedSymbol)
	}

	return normalized, nil
}

// normalizeSymbol ensures symbol has exactly one USD suffix
func (d *DryrunExecutor) normalizeSymbol(symbol string) string {
	// Remove any existing USD suffixes first
	cleaned := symbol
	for strings.HasSuffix(cleaned, "USD") {
		cleaned = strings.TrimSuffix(cleaned, "USD")
	}

	// Add exactly one USD suffix
	return cleaned + "USD"
}

// runScanPipeline executes the scan pipeline with mock data
func (d *DryrunExecutor) runScanPipeline(ctx context.Context, universe []string) ([]CandidateResult, error) {
	// Use a subset for dry run to keep it fast
	testUniverse := universe
	if len(universe) > 20 {
		testUniverse = universe[:20]
	}

	// Run scan on entire universe and filter for test symbols
	allCandidates, err := d.scanPipeline.ScanUniverse(ctx)
	if err != nil {
		return nil, err
	}

	// Filter candidates to test universe
	var candidates []CandidateResult
	testSymbolMap := make(map[string]bool)
	for _, symbol := range testUniverse {
		testSymbolMap[symbol] = true
	}

	for _, candidate := range allCandidates {
		if testSymbolMap[candidate.Symbol] {
			candidates = append(candidates, candidate)
		}
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

// PrintSummary prints the 4-line dry run summary as specified
func (d *DryrunExecutor) PrintSummary(result *DryrunResult) {
	fmt.Printf("Universe: %d USD pairs\n", result.PairsCount)
	fmt.Printf("Candidates: %d\n", result.Candidates)
	fmt.Printf("Coverage@20: %.0f%% (1h) %.0f%% (24h) %.0f%% (7d)\n",
		result.Coverage["1h"]*100, result.Coverage["24h"]*100, result.Coverage["7d"]*100)

	// Format top reasons nicely
	topReasons := d.formatTopReasons(result.Reasons, 3)
	if len(topReasons) == 0 {
		fmt.Printf("Top reasons: none\n")
	} else {
		fmt.Printf("Top reasons: %s\n", strings.Join(topReasons, ", "))
	}
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

// writeCoverageFile writes coverage analysis to the canonical analyst output path
func (d *DryrunExecutor) writeCoverageFile(coverage *analyst.CoverageReport, symbolCount int) error {
	outputDir := "out/analyst"
	outputPath := filepath.Join(outputDir, "coverage.json")

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Marshal coverage data
	data, err := json.MarshalIndent(coverage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal coverage: %w", err)
	}

	// Write atomically using temp file + rename
	tempPath := outputPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempPath, outputPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Log the write operation
	log.Info().
		Str("path", outputPath).
		Int("symbols", symbolCount).
		Strs("windows", []string{"1h", "24h", "7d"}).
		Msg("Dry-run: wrote coverage.json")

	return nil
}
