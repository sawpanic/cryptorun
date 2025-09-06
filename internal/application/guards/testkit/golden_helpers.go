package testkit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GoldenHelper manages golden file generation and comparison
type GoldenHelper struct {
	basePath string
}

// NewGoldenHelper creates a new golden file helper
func NewGoldenHelper(basePath string) *GoldenHelper {
	return &GoldenHelper{basePath: basePath}
}

// GuardResult represents the outcome of a single guard check
type GuardResult struct {
	Symbol      string `json:"symbol"`
	Status      string `json:"status"`
	FailedGuard string `json:"failed_guard,omitempty"`
	Reason      string `json:"reason,omitempty"`
	FixHint     string `json:"fix_hint,omitempty"`
}

// GuardSummary represents the overall guard execution summary
type GuardSummary struct {
	TestName    string        `json:"test_name"`
	Regime      string        `json:"regime"`
	Timestamp   string        `json:"timestamp"`
	PassCount   int           `json:"pass_count"`
	FailCount   int           `json:"fail_count"`
	ExitCode    int           `json:"exit_code"`
	Results     []GuardResult `json:"results"`
	ProgressLog []string      `json:"progress_log"`
}

// SaveGolden saves the guard summary as a golden file
func (gh *GoldenHelper) SaveGolden(testName string, summary *GuardSummary) error {
	// Ensure results are sorted by symbol for stable output
	sort.Slice(summary.Results, func(i, j int) bool {
		return summary.Results[i].Symbol < summary.Results[j].Symbol
	})

	// Generate compact table format
	compactTable := gh.generateCompactTable(summary)

	// Create golden content with both table and JSON
	goldenContent := GoldenContent{
		Summary:      summary,
		CompactTable: compactTable,
	}

	// Save to golden file
	goldenPath := filepath.Join(gh.basePath, testName+".golden")
	if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(goldenContent, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(goldenPath, data, 0644)
}

// LoadGolden loads a golden file for comparison
func (gh *GoldenHelper) LoadGolden(testName string) (*GoldenContent, error) {
	goldenPath := filepath.Join(gh.basePath, testName+".golden")

	data, err := os.ReadFile(goldenPath)
	if err != nil {
		return nil, err
	}

	var content GoldenContent
	if err := json.Unmarshal(data, &content); err != nil {
		return nil, err
	}

	return &content, nil
}

// CompareWithGolden compares current results with golden file
func (gh *GoldenHelper) CompareWithGolden(testName string, summary *GuardSummary) error {
	golden, err := gh.LoadGolden(testName)
	if err != nil {
		// If golden doesn't exist, save current as golden
		return gh.SaveGolden(testName, summary)
	}

	// Compare key fields
	if summary.PassCount != golden.Summary.PassCount {
		return fmt.Errorf("pass count mismatch: got %d, golden %d",
			summary.PassCount, golden.Summary.PassCount)
	}

	if summary.FailCount != golden.Summary.FailCount {
		return fmt.Errorf("fail count mismatch: got %d, golden %d",
			summary.FailCount, golden.Summary.FailCount)
	}

	if summary.ExitCode != golden.Summary.ExitCode {
		return fmt.Errorf("exit code mismatch: got %d, golden %d",
			summary.ExitCode, golden.Summary.ExitCode)
	}

	// Compare results order and content
	if len(summary.Results) != len(golden.Summary.Results) {
		return fmt.Errorf("result count mismatch: got %d, golden %d",
			len(summary.Results), len(golden.Summary.Results))
	}

	// Sort both for comparison
	sort.Slice(summary.Results, func(i, j int) bool {
		return summary.Results[i].Symbol < summary.Results[j].Symbol
	})
	sort.Slice(golden.Summary.Results, func(i, j int) bool {
		return golden.Summary.Results[i].Symbol < golden.Summary.Results[j].Symbol
	})

	for i, result := range summary.Results {
		goldenResult := golden.Summary.Results[i]

		if result.Symbol != goldenResult.Symbol {
			return fmt.Errorf("result[%d] symbol mismatch: got %s, golden %s",
				i, result.Symbol, goldenResult.Symbol)
		}

		if result.Status != goldenResult.Status {
			return fmt.Errorf("result[%d] status mismatch for %s: got %s, golden %s",
				i, result.Symbol, result.Status, goldenResult.Status)
		}

		if result.FailedGuard != goldenResult.FailedGuard {
			return fmt.Errorf("result[%d] failed guard mismatch for %s: got %s, golden %s",
				i, result.Symbol, result.FailedGuard, goldenResult.FailedGuard)
		}

		if result.Reason != goldenResult.Reason {
			return fmt.Errorf("result[%d] reason mismatch for %s: got %s, golden %s",
				i, result.Symbol, result.Reason, goldenResult.Reason)
		}
	}

	return nil
}

// GoldenContent represents the complete golden file content
type GoldenContent struct {
	Summary      *GuardSummary `json:"summary"`
	CompactTable string        `json:"compact_table"`
}

// generateCompactTable creates a compact ASCII table representation
func (gh *GoldenHelper) generateCompactTable(summary *GuardSummary) string {
	var lines []string

	// Header
	lines = append(lines, fmt.Sprintf("🛡️  Guard Results (%s regime)", summary.Regime))
	lines = append(lines, "┌──────────┬────────┬─────────────┬──────────────────────────────────────┐")
	lines = append(lines, "│ Symbol   │ Status │ Failed Guard│ Reason                               │")
	lines = append(lines, "├──────────┼────────┼─────────────┼──────────────────────────────────────┤")

	// Results rows
	for _, result := range summary.Results {
		status := result.Status
		if status == "PASS" {
			status = "✅ PASS"
		} else {
			status = "❌ FAIL"
		}

		failedGuard := result.FailedGuard
		if failedGuard == "" {
			failedGuard = "-"
		}

		reason := result.Reason
		if len(reason) > 36 {
			reason = reason[:33] + "..."
		}
		if reason == "" {
			reason = "-"
		}

		lines = append(lines, fmt.Sprintf("│ %-8s │ %-6s │ %-11s │ %-36s │",
			result.Symbol, status, failedGuard, reason))
	}

	// Footer with summary
	lines = append(lines, "└──────────┴────────┴─────────────┴──────────────────────────────────────┘")
	lines = append(lines, fmt.Sprintf("Summary: %d passed, %d failed (exit code %d)",
		summary.PassCount, summary.FailCount, summary.ExitCode))

	// Progress log if available
	if len(summary.ProgressLog) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Progress Log:")
		for _, log := range summary.ProgressLog {
			lines = append(lines, "  "+log)
		}
	}

	return strings.Join(lines, "\n")
}

// GenerateProgressLog creates deterministic progress log entries
func GenerateProgressLog(regime string, candidates []CandidateFixture) []string {
	var log []string

	log = append(log, fmt.Sprintf("⏳ Starting guard evaluation (regime: %s)", regime))
	log = append(log, fmt.Sprintf("📊 Processing %d candidates", len(candidates)))

	guardSteps := []string{
		"freshness", "fatigue", "liquidity", "caps", "final",
	}

	for i, step := range guardSteps {
		progress := int(float64(i+1) / float64(len(guardSteps)) * 100)
		log = append(log, fmt.Sprintf("🛡️  [%d%%] Evaluating %s guards...", progress, step))
	}

	log = append(log, "✅ Guard evaluation completed")

	return log
}

// CreateTestDataFixture creates a complete test data file
func CreateTestDataFixture(testCase *GuardTestCase, filename string) error {
	data, err := json.MarshalIndent(testCase, "", "  ")
	if err != nil {
		return err
	}

	testDataPath := filepath.Join("testdata", "guards", filename)
	if err := os.MkdirAll(filepath.Dir(testDataPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(testDataPath, data, 0644)
}
