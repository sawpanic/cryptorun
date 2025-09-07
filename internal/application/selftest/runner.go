package selftest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TestResult represents the result of a single test
type TestResult struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"` // PASS, FAIL, SKIP
	Duration  time.Duration `json:"duration"`
	Message   string        `json:"message,omitempty"`
	Details   []string      `json:"details,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// TestResults holds all test results
type TestResults struct {
	OverallStatus string        `json:"overall_status"` // PASS, FAIL
	TotalCount    int           `json:"total_count"`
	PassedCount   int           `json:"passed_count"`
	FailedCount   int           `json:"failed_count"`
	SkippedCount  int           `json:"skipped_count"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	Duration      time.Duration `json:"duration"`
	Tests         []TestResult  `json:"tests"`
}

// Runner executes self-tests
type Runner struct {
	validators []Validator
}

// Validator interface for test components
type Validator interface {
	Name() string
	Validate() TestResult
}

// NewRunner creates a new self-test runner
func NewRunner() *Runner {
	return &Runner{
		validators: []Validator{
			NewAtomicityValidator(),
			NewUniverseHygieneValidator(),
			NewGateValidator(),
			NewMicrostructureValidator(),
			NewMenuIntegrityValidator(),
		},
	}
}

// RunAllTests executes all configured tests
func (r *Runner) RunAllTests() (*TestResults, error) {
	results := &TestResults{
		StartTime: time.Now(),
		Tests:     make([]TestResult, 0, len(r.validators)),
	}

	for _, validator := range r.validators {
		result := validator.Validate()
		results.Tests = append(results.Tests, result)

		switch result.Status {
		case "PASS":
			results.PassedCount++
		case "FAIL":
			results.FailedCount++
		case "SKIP":
			results.SkippedCount++
		}
	}

	results.EndTime = time.Now()
	results.Duration = results.EndTime.Sub(results.StartTime)
	results.TotalCount = len(results.Tests)

	if results.FailedCount == 0 {
		results.OverallStatus = "PASS"
	} else {
		results.OverallStatus = "FAIL"
	}

	return results, nil
}

// GenerateReport creates a markdown report
func (r *Runner) GenerateReport(results *TestResults, outputPath string) error {
	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	var sb strings.Builder

	// Header
	sb.WriteString("# CryptoRun Self-Test Report\n\n")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n", results.EndTime.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("**Duration:** %s\n", results.Duration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("**Overall Status:** %s\n\n", results.OverallStatus))

	// Summary
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Tests:** %d\n", results.TotalCount))
	sb.WriteString(fmt.Sprintf("- **Passed:** %d\n", results.PassedCount))
	sb.WriteString(fmt.Sprintf("- **Failed:** %d\n", results.FailedCount))
	sb.WriteString(fmt.Sprintf("- **Skipped:** %d\n\n", results.SkippedCount))

	// Test Results
	sb.WriteString("## Test Results\n\n")

	for _, test := range results.Tests {
		statusIcon := "✅"
		if test.Status == "FAIL" {
			statusIcon = "❌"
		} else if test.Status == "SKIP" {
			statusIcon = "⚠️"
		}

		sb.WriteString(fmt.Sprintf("### %s %s\n\n", statusIcon, test.Name))
		sb.WriteString(fmt.Sprintf("- **Status:** %s\n", test.Status))
		sb.WriteString(fmt.Sprintf("- **Duration:** %s\n", test.Duration.Round(time.Millisecond)))

		if test.Message != "" {
			sb.WriteString(fmt.Sprintf("- **Message:** %s\n", test.Message))
		}

		if len(test.Details) > 0 {
			sb.WriteString("- **Details:**\n")
			for _, detail := range test.Details {
				sb.WriteString(fmt.Sprintf("  - %s\n", detail))
			}
		}

		sb.WriteString("\n")
	}

	// Write report
	if err := os.WriteFile(outputPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}
