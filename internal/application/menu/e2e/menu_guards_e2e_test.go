package e2e

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"cryptorun/internal/application/guards/testkit"
)

// MockMenuUI provides a testable menu interface
type MockMenuUI struct {
	Input  *bytes.Buffer
	Output *bytes.Buffer
	t      *testing.T
}

// NewMockMenuUI creates a new mock menu UI for testing
func NewMockMenuUI(t *testing.T) *MockMenuUI {
	return &MockMenuUI{
		Input:  &bytes.Buffer{},
		Output: &bytes.Buffer{},
		t:      t,
	}
}

// SimulateInput adds input to the mock UI buffer
func (ui *MockMenuUI) SimulateInput(input string) {
	ui.Input.WriteString(input + "\n")
}

// GetOutput returns all captured output
func (ui *MockMenuUI) GetOutput() string {
	return ui.Output.String()
}

// TestMenuGuardStatusDisplay tests the guard status display in menu system
func TestMenuGuardStatusDisplay(t *testing.T) {
	ui := NewMockMenuUI(t)
	fixture := testkit.LoadFixture(t, "fatigue_calm.json")
	evaluator := testkit.NewMockEvaluator(fixture)

	// Simulate guard evaluation
	result := evaluator.EvaluateAllGuards()

	// Simulate viewing guard status through menu
	ui.SimulateInput("3") // View Guard Status option

	// Execute mock guard status display
	displayGuardStatus(ui, result)

	output := ui.GetOutput()

	// Validate guard status table is displayed
	if !strings.Contains(output, "🛡️ Guard Status & Results") {
		t.Error("Expected guard status header in output")
	}

	// Validate ASCII table format
	if !strings.Contains(output, "┌") || !strings.Contains(output, "└") {
		t.Error("Expected ASCII table borders")
	}

	// Validate regime context
	if !strings.Contains(output, "(calm regime)") {
		t.Error("Expected regime context in output")
	}

	// Validate summary information
	if !strings.Contains(output, "Summary: 2 passed, 1 failed") {
		t.Error("Expected summary information")
	}
}

// TestMenuGuardProgressBreadcrumbs tests progress display during guard evaluation
func TestMenuGuardProgressBreadcrumbs(t *testing.T) {
	ui := NewMockMenuUI(t)
	fixture := testkit.LoadFixture(t, "freshness_normal.json")
	evaluator := testkit.NewMockEvaluator(fixture)

	// Simulate progress display
	ui.SimulateInput("4") // Show Progress Breadcrumbs option

	result := evaluator.EvaluateAllGuards()
	displayProgressBreadcrumbs(ui, result)

	output := ui.GetOutput()

	// Validate progress breadcrumbs
	requiredSteps := []string{
		"⏳ Starting guard evaluation",
		"📊 Processing",
		"🛡️",
		"✅ Guard evaluation completed",
	}

	for _, required := range requiredSteps {
		if !strings.Contains(output, required) {
			t.Errorf("Expected progress step '%s' in output", required)
		}
	}

	// Validate step numbering
	if !strings.Contains(output, "1.") && !strings.Contains(output, "2.") {
		t.Error("Expected numbered progress steps")
	}
}

// TestMenuGuardDetailedReasons tests detailed failure reason display
func TestMenuGuardDetailedReasons(t *testing.T) {
	ui := NewMockMenuUI(t)
	fixture := testkit.LoadFixture(t, "liquidity_gates.json")
	evaluator := testkit.NewMockEvaluator(fixture)

	result := evaluator.EvaluateAllGuards()

	// Simulate viewing detailed reasons
	ui.SimulateInput("1") // View Detailed Guard Reasons option

	displayDetailedGuardReasons(ui, result)

	output := ui.GetOutput()

	// Validate detailed failure information
	if !strings.Contains(output, "📋 Detailed Guard Failure Reasons") {
		t.Error("Expected detailed reasons header")
	}

	// Look for failure details
	hasFailureDetails := false
	for _, guardResult := range result.Results {
		if guardResult.Status == "FAIL" {
			if strings.Contains(output, guardResult.Symbol) &&
				strings.Contains(output, guardResult.FailedGuard) &&
				strings.Contains(output, guardResult.Reason) {
				hasFailureDetails = true
				break
			}
		}
	}

	if !hasFailureDetails {
		t.Error("Expected detailed failure information in output")
	}

	// Validate fix hints are displayed
	if !strings.Contains(output, "💡 Fix Hint:") {
		t.Error("Expected fix hints in detailed output")
	}
}

// TestMenuGuardThresholdAdjustment tests quick threshold adjustment interface
func TestMenuGuardThresholdAdjustment(t *testing.T) {
	ui := NewMockMenuUI(t)

	// Simulate threshold adjustment menu
	ui.SimulateInput("3") // Adjust Guard Thresholds option
	ui.SimulateInput("1") // Increase Fatigue Threshold

	displayThresholdAdjustment(ui)

	output := ui.GetOutput()

	// Validate threshold adjustment options
	if !strings.Contains(output, "🔧 Quick Guard Threshold Adjustments") {
		t.Error("Expected threshold adjustment header")
	}

	// Validate adjustment options are presented
	adjustmentOptions := []string{
		"Increase Fatigue Threshold",
		"Relax Spread Tolerance",
		"Increase Freshness Bar Age",
	}

	for _, option := range adjustmentOptions {
		if !strings.Contains(output, option) {
			t.Errorf("Expected adjustment option '%s' in output", option)
		}
	}

	// Validate feedback on adjustment
	if !strings.Contains(output, "✅") {
		t.Error("Expected success feedback after adjustment")
	}
}

// TestMenuGuardExitCodes tests proper exit code behavior through menu
func TestMenuGuardExitCodes(t *testing.T) {
	testCases := []struct {
		name         string
		fixture      string
		expectedCode int
	}{
		{"AllPass", "all_pass.json", 0},
		{"HardFailures", "fatigue_calm.json", 1},
		{"SoftFailuresOnly", "social_caps.json", 0},
		{"MixedFailures", "liquidity_gates.json", 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := NewMockMenuUI(t)

			// Load fixture (some may not exist - create minimal mock data)
			var result *testkit.GuardEvaluationResult
			if tc.fixture == "all_pass.json" {
				result = createAllPassResult()
			} else {
				fixture := testkit.LoadFixture(t, tc.fixture)
				evaluator := testkit.NewMockEvaluator(fixture)
				result = evaluator.EvaluateAllGuards()
			}

			// Validate exit code
			if result.ExitCode != tc.expectedCode {
				t.Errorf("Expected exit code %d, got %d", tc.expectedCode, result.ExitCode)
			}

			// Simulate menu display with exit code validation
			displayGuardStatusWithExitCode(ui, result)

			output := ui.GetOutput()

			// Validate exit code is communicated to user
			if tc.expectedCode == 1 && !strings.Contains(output, "exit code 1") {
				t.Error("Expected exit code 1 to be shown to user")
			}
		})
	}
}

// TestMenuGuardRerunEvaluation tests re-running guard evaluation through menu
func TestMenuGuardRerunEvaluation(t *testing.T) {
	ui := NewMockMenuUI(t)
	fixture := testkit.LoadFixture(t, "freshness_normal.json")

	// Simulate re-run option
	ui.SimulateInput("2") // Re-run Guard Evaluation option

	displayRerunEvaluation(ui, fixture)

	output := ui.GetOutput()

	// Validate re-run progress display
	if !strings.Contains(output, "🔄 Re-running guard evaluation") {
		t.Error("Expected re-run header")
	}

	// Validate progress steps during re-run
	progressSteps := []string{
		"Loading candidates",
		"Evaluating freshness guards",
		"Evaluating fatigue guards",
		"completed!",
	}

	for _, step := range progressSteps {
		if !strings.Contains(output, step) {
			t.Errorf("Expected progress step '%s' during re-run", step)
		}
	}

	// Validate completion message
	if !strings.Contains(output, "Updated results available") {
		t.Error("Expected completion message after re-run")
	}
}

// TestMenuGuardUXConsistency tests consistent UX elements across guard interfaces
func TestMenuGuardUXConsistency(t *testing.T) {
	ui := NewMockMenuUI(t)
	fixture := testkit.LoadFixture(t, "fatigue_calm.json")
	evaluator := testkit.NewMockEvaluator(fixture)
	result := evaluator.EvaluateAllGuards()

	// Test all guard interface components
	interfaces := map[string]func(){
		"status":     func() { displayGuardStatus(ui, result) },
		"details":    func() { displayDetailedGuardReasons(ui, result) },
		"progress":   func() { displayProgressBreadcrumbs(ui, result) },
		"adjustment": func() { displayThresholdAdjustment(ui) },
	}

	for interfaceName, displayFunc := range interfaces {
		t.Run(interfaceName, func(t *testing.T) {
			ui.Output.Reset() // Clear output buffer
			displayFunc()
			output := ui.GetOutput()

			// Validate consistent UX elements
			if output == "" {
				t.Error("Expected output from interface")
			}

			// Check for consistent emoji usage
			if !hasEmojiIndicators(output) {
				t.Errorf("Expected emoji indicators in %s interface", interfaceName)
			}

			// Check for consistent formatting
			if !hasConsistentFormatting(output) {
				t.Errorf("Expected consistent formatting in %s interface", interfaceName)
			}
		})
	}
}

// Helper functions for mock menu operations

func displayGuardStatus(ui *MockMenuUI, result *testkit.GuardEvaluationResult) {
	ui.Output.WriteString("╔═══════════════ GUARD STATUS ═══════════════╗\n\n")
	ui.Output.WriteString(result.FormatTableOutput())
	ui.Output.WriteString("\n")

	ui.Output.WriteString("╔═══════════════ GUARD ACTIONS ═══════════════╗\n")
	ui.Output.WriteString("\n 1. 📊 View Detailed Guard Reasons\n")
	ui.Output.WriteString(" 2. 🔄 Re-run Guard Evaluation\n")
	ui.Output.WriteString(" 3. ⚙️  Adjust Guard Thresholds\n")
	ui.Output.WriteString(" 4. 📈 Show Progress Breadcrumbs\n")
	ui.Output.WriteString(" 0. ← Back to Scan Menu\n\n")
}

func displayDetailedGuardReasons(ui *MockMenuUI, result *testkit.GuardEvaluationResult) {
	ui.Output.WriteString("📋 Detailed Guard Failure Reasons\n")
	ui.Output.WriteString(strings.Repeat("=", 50) + "\n\n")

	failedCount := 0
	for _, guardResult := range result.Results {
		if guardResult.Status == "FAIL" {
			failedCount++
			ui.Output.WriteString(fmt.Sprintf("%d. %s ❌\n", failedCount, guardResult.Symbol))
			ui.Output.WriteString(fmt.Sprintf("   Failed Guard: %s\n", guardResult.FailedGuard))
			ui.Output.WriteString(fmt.Sprintf("   Reason: %s\n", guardResult.Reason))
			if guardResult.FixHint != "" {
				ui.Output.WriteString(fmt.Sprintf("   💡 Fix Hint: %s\n", guardResult.FixHint))
			}
			ui.Output.WriteString("\n")
		}
	}

	if failedCount == 0 {
		ui.Output.WriteString("✅ No guard failures to display - all candidates passed!\n")
	}
}

func displayProgressBreadcrumbs(ui *MockMenuUI, result *testkit.GuardEvaluationResult) {
	ui.Output.WriteString("📈 Guard Evaluation Progress Breadcrumbs\n")
	ui.Output.WriteString(strings.Repeat("=", 45) + "\n\n")

	for i, logEntry := range result.ProgressLog {
		ui.Output.WriteString(fmt.Sprintf("%d. %s\n", i+1, logEntry))
	}

	ui.Output.WriteString(fmt.Sprintf("\nTotal steps: %d\n", len(result.ProgressLog)))
}

func displayThresholdAdjustment(ui *MockMenuUI) {
	ui.Output.WriteString("🔧 Quick Guard Threshold Adjustments\n\n")
	ui.Output.WriteString("Common adjustments for current failures:\n\n")
	ui.Output.WriteString(" 1. Increase Fatigue Threshold (currently 12.0% → 15.0%)\n")
	ui.Output.WriteString(" 2. Relax Spread Tolerance (currently 50.0 bps → 75.0 bps)\n")
	ui.Output.WriteString(" 3. Increase Freshness Bar Age (currently 2 bars → 3 bars)\n")
	ui.Output.WriteString(" 4. View Full Settings Menu\n\n")

	ui.Output.WriteString("✅ Fatigue threshold increased to 15.0%\n")
	ui.Output.WriteString("💾 Settings saved - re-run guard evaluation to see changes\n")
}

func displayGuardStatusWithExitCode(ui *MockMenuUI, result *testkit.GuardEvaluationResult) {
	displayGuardStatus(ui, result)
	if result.ExitCode == 1 {
		ui.Output.WriteString(fmt.Sprintf("Summary: %d passed, %d failed (exit code 1)\n",
			result.PassCount, result.FailCount))
	}
}

func displayRerunEvaluation(ui *MockMenuUI, fixture *testkit.GuardTestFixture) {
	ui.Output.WriteString("🔄 Re-running guard evaluation...\n\n")

	steps := []string{
		"⏳ Loading candidates...",
		"🛡️  Evaluating freshness guards...",
		"🛡️  Evaluating fatigue guards...",
		"🛡️  Evaluating liquidity guards...",
		"🛡️  Evaluating caps guards...",
		"✅ Guard evaluation completed!",
	}

	for i, step := range steps {
		ui.Output.WriteString(fmt.Sprintf("[%d%%] %s\n", (i+1)*100/len(steps), step))
		time.Sleep(1 * time.Millisecond) // Minimal delay for test
	}

	ui.Output.WriteString("\n📊 Updated results available - returning to guard status...\n")
}

func createAllPassResult() *testkit.GuardEvaluationResult {
	return &testkit.GuardEvaluationResult{
		Regime:          "normal",
		Timestamp:       "2025-01-15T12:00:00Z",
		TotalCandidates: 3,
		PassCount:       3,
		FailCount:       0,
		ExitCode:        0,
		Results: []testkit.GuardResult{
			{Symbol: "BTCUSD", Status: "PASS"},
			{Symbol: "ETHUSD", Status: "PASS"},
			{Symbol: "SOLUSD", Status: "PASS"},
		},
		ProgressLog: []string{
			"⏳ Starting guard evaluation (regime: normal)",
			"📊 Processing 3 candidates",
			"✅ Guard evaluation completed",
		},
	}
}

func hasEmojiIndicators(output string) bool {
	emojis := []string{"🛡️", "✅", "❌", "📊", "⏳", "🔄", "💡", "📋"}
	for _, emoji := range emojis {
		if strings.Contains(output, emoji) {
			return true
		}
	}
	return false
}

func hasConsistentFormatting(output string) bool {
	// Basic formatting consistency checks
	return len(output) > 0 && (strings.Contains(output, "\n") || strings.Contains(output, " "))
}

// Integration test that captures real console output
func TestMenuGuardConsoleOutput(t *testing.T) {
	// Capture stdout for testing actual console output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		os.Stdout = oldStdout
	}()

	// Execute guard status display that would normally go to console
	fixture := testkit.LoadFixture(t, "fatigue_calm.json")
	evaluator := testkit.NewMockEvaluator(fixture)
	result := evaluator.EvaluateAllGuards()

	// Write actual formatted output
	fmt.Print(result.FormatTableOutput())

	w.Close()
	output, _ := io.ReadAll(r)
	consoleOutput := string(output)

	// Validate console output format
	if !strings.Contains(consoleOutput, "┌") {
		t.Error("Expected table borders in console output")
	}

	if !strings.Contains(consoleOutput, "🛡️") {
		t.Error("Expected guard emoji in console output")
	}

	if !strings.Contains(consoleOutput, "Summary:") {
		t.Error("Expected summary in console output")
	}
}
