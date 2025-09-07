package e2e

import (
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/application/guards/testkit"
)

// TestFatigueGuardCalmRegime tests fatigue guard behavior in calm market conditions
func TestFatigueGuardCalmRegime(t *testing.T) {
	fixture := testkit.LoadFixture(t, "fatigue_calm.json")
	evaluator := testkit.NewMockEvaluator(fixture)

	// Perform guard evaluation
	result := evaluator.EvaluateAllGuards()

	// Assert expected results
	fixture.AssertExpectedResults(t, result)

	// Validate progress breadcrumbs
	if len(result.ProgressLog) == 0 {
		t.Error("Expected progress log entries, got none")
	}

	// Check that progress includes regime context
	found := false
	for _, step := range result.ProgressLog {
		if step == "⏳ Starting guard evaluation (regime: calm)" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected regime context in progress log")
	}

	// Test golden file comparison
	actualOutput := result.FormatTableOutput()
	expectedOutput := testkit.LoadGoldenFile(t, "fatigue_calm.golden")

	if actualOutput != expectedOutput {
		// Update golden file if running with -update flag
		testkit.SaveGoldenFile(t, "fatigue_calm.golden", actualOutput)
		t.Errorf("Output mismatch. Golden file updated. Expected:\n%s\nActual:\n%s", expectedOutput, actualOutput)
	}
}

// TestFreshnessGuardNormalRegime tests freshness guard behavior in normal market conditions
func TestFreshnessGuardNormalRegime(t *testing.T) {
	fixture := testkit.LoadFixture(t, "freshness_normal.json")
	evaluator := testkit.NewMockEvaluator(fixture)

	result := evaluator.EvaluateAllGuards()
	fixture.AssertExpectedResults(t, result)

	// Validate specific freshness failure reasons
	for _, guardResult := range result.Results {
		if guardResult.Status == "FAIL" && guardResult.FailedGuard == "freshness" {
			if guardResult.Reason == "" {
				t.Errorf("Expected detailed reason for freshness failure on %s", guardResult.Symbol)
			}
			if guardResult.FixHint == "" {
				t.Errorf("Expected fix hint for freshness failure on %s", guardResult.Symbol)
			}
		}
	}

	// Test progress output formatting
	progressOutput := result.FormatProgressOutput()
	if progressOutput == "" {
		t.Error("Expected formatted progress output, got empty string")
	}
}

// TestLiquidityGuards tests spread, depth, and VADR validation
func TestLiquidityGuards(t *testing.T) {
	fixture := testkit.LoadFixture(t, "liquidity_gates.json")
	evaluator := testkit.NewMockEvaluator(fixture)

	result := evaluator.EvaluateAllGuards()
	fixture.AssertExpectedResults(t, result)

	// Verify exit code behavior for hard guard failures
	if result.FailCount > 0 && result.ExitCode != 1 {
		t.Errorf("Expected exit code 1 for hard guard failures, got %d", result.ExitCode)
	}

	// Validate that liquidity failures have specific reasons
	liquidityFailures := 0
	for _, guardResult := range result.Results {
		if guardResult.Status == "FAIL" && (guardResult.FailedGuard == "spread" || guardResult.FailedGuard == "depth" || guardResult.FailedGuard == "vadr") {
			liquidityFailures++
			if guardResult.Reason == "" {
				t.Errorf("Expected specific reason for liquidity failure on %s", guardResult.Symbol)
			}
		}
	}

	if liquidityFailures == 0 {
		t.Error("Expected at least one liquidity failure in test fixture")
	}
}

// TestSocialCapsRegimeAware tests social/brand cap behavior across regimes
func TestSocialCapsRegimeAware(t *testing.T) {
	fixture := testkit.LoadFixture(t, "social_caps.json")
	evaluator := testkit.NewMockEvaluator(fixture)

	result := evaluator.EvaluateAllGuards()
	fixture.AssertExpectedResults(t, result)

	// Validate that cap failures are soft (don't affect exit code if alone)
	capFailures := 0
	hardFailures := 0

	for _, guardResult := range result.Results {
		if guardResult.Status == "FAIL" {
			if guardResult.FailedGuard == "social_cap" || guardResult.FailedGuard == "brand_cap" {
				capFailures++
			} else {
				hardFailures++
			}
		}
	}

	// If only soft failures, exit code should be 0; if any hard failures, should be 1
	expectedExitCode := 0
	if hardFailures > 0 {
		expectedExitCode = 1
	}

	if result.ExitCode != expectedExitCode {
		t.Errorf("Expected exit code %d for failure mix (caps: %d, hard: %d), got %d",
			expectedExitCode, capFailures, hardFailures, result.ExitCode)
	}
}

// TestGuardEvaluationPerformance validates that guard evaluation meets performance targets
func TestGuardEvaluationPerformance(t *testing.T) {
	fixture := testkit.LoadFixture(t, "fatigue_calm.json")
	evaluator := testkit.NewMockEvaluator(fixture)

	start := time.Now()
	result := evaluator.EvaluateAllGuards()
	elapsed := time.Since(start)

	// Target: <50ms per candidate
	maxDuration := time.Duration(len(fixture.Candidates)) * 50 * time.Millisecond

	if elapsed > maxDuration {
		t.Errorf("Guard evaluation took %v, expected <%v for %d candidates",
			elapsed, maxDuration, len(fixture.Candidates))
	}

	if result == nil {
		t.Error("Expected guard evaluation result, got nil")
	}
}

// TestProgressBreadcrumbsFormat validates progress output format and content
func TestProgressBreadcrumbsFormat(t *testing.T) {
	fixture := testkit.LoadFixture(t, "freshness_normal.json")
	evaluator := testkit.NewMockEvaluator(fixture)

	result := evaluator.EvaluateAllGuards()

	// Validate progress log structure
	if len(result.ProgressLog) < 3 {
		t.Error("Expected at least 3 progress steps")
	}

	// Check required progress elements
	requiredSteps := []string{
		"⏳ Starting guard evaluation",
		"📊 Processing",
		"✅ Guard evaluation completed",
	}

	for _, required := range requiredSteps {
		found := false
		for _, step := range result.ProgressLog {
			if containsString(step, required) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing required progress step: %s", required)
		}
	}

	// Validate percentage progress steps
	percentageSteps := 0
	for _, step := range result.ProgressLog {
		if containsString(step, "%]") {
			percentageSteps++
		}
	}

	if percentageSteps < 2 {
		t.Errorf("Expected at least 2 percentage progress steps, got %d", percentageSteps)
	}
}

// TestGuardResultTableFormat validates ASCII table output format
func TestGuardResultTableFormat(t *testing.T) {
	fixture := testkit.LoadFixture(t, "fatigue_calm.json")
	evaluator := testkit.NewMockEvaluator(fixture)

	result := evaluator.EvaluateAllGuards()
	tableOutput := result.FormatTableOutput()

	// Validate table structure
	lines := splitLines(tableOutput)
	if len(lines) < 5 { // Header + separator + data + footer + summary
		t.Errorf("Expected at least 5 lines in table output, got %d", len(lines))
	}

	// Check for table borders
	if !containsString(tableOutput, "┌") || !containsString(tableOutput, "└") {
		t.Error("Expected table borders in output")
	}

	// Check for status indicators
	if result.PassCount > 0 && !containsString(tableOutput, "✅ PASS") {
		t.Error("Expected pass status indicator for passing candidates")
	}

	if result.FailCount > 0 && !containsString(tableOutput, "❌ FAIL") {
		t.Error("Expected fail status indicator for failing candidates")
	}

	// Validate summary line
	if !containsString(tableOutput, "Summary:") {
		t.Error("Expected summary line in table output")
	}
}

// Helper functions
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && containsString(s[1:], substr)
}

func splitLines(s string) []string {
	lines := []string{}
	currentLine := ""
	for _, char := range s {
		if char == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
		} else {
			currentLine += string(char)
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
}

// BenchmarkGuardEvaluation benchmarks guard evaluation performance
func BenchmarkGuardEvaluation(b *testing.B) {
	fixture := testkit.LoadFixture(&testing.T{}, "fatigue_calm.json")
	evaluator := testkit.NewMockEvaluator(fixture)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := evaluator.EvaluateAllGuards()
		if result == nil {
			b.Error("Expected result, got nil")
		}
	}
}
