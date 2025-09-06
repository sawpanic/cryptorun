package testkit

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"
)

// GuardTestFixture represents a complete guard test scenario with expected outcomes
type GuardTestFixture struct {
	Name       string          `json:"name"`
	Regime     string          `json:"regime"`
	Timestamp  string          `json:"timestamp"`
	Candidates []TestCandidate `json:"candidates"`
	Expected   ExpectedResults `json:"expected"`
}

// TestCandidate represents a candidate for guard evaluation testing
type TestCandidate struct {
	Symbol         string  `json:"symbol"`
	CompositeScore float64 `json:"composite_score"`
	MomentumCore   float64 `json:"momentum_core"`
	RSI4h          float64 `json:"rsi_4h"`
	Volume24h      float64 `json:"volume_24h"`
	VolumeAvg      float64 `json:"volume_avg"`
	SpreadBps      float64 `json:"spread_bps"`
	DepthUSD       float64 `json:"depth_usd"`
	VADR           float64 `json:"vadr"`
	CatalystHeat   float64 `json:"catalyst_heat"`
	SocialScore    float64 `json:"social_score"`
	BrandScore     float64 `json:"brand_score"`
	LastUpdate     string  `json:"last_update"`
	BarAge         int     `json:"bar_age"`
	ATRCurrent     float64 `json:"atr_current"`
	PriceMoveATR   float64 `json:"price_move_atr"`
}

// ExpectedResults defines the expected outcomes for a guard test scenario
type ExpectedResults struct {
	PassCount    int                   `json:"pass_count"`
	FailCount    int                   `json:"fail_count"`
	ExitCode     int                   `json:"exit_code"`
	GuardResults []ExpectedGuardResult `json:"guard_results"`
}

// ExpectedGuardResult defines expected outcome for a single candidate
type ExpectedGuardResult struct {
	Symbol      string `json:"symbol"`
	Status      string `json:"status"`
	FailedGuard string `json:"failed_guard,omitempty"`
	Reason      string `json:"reason,omitempty"`
	FixHint     string `json:"fix_hint,omitempty"`
}

// GuardProgressLog represents expected progress breadcrumbs
type GuardProgressLog struct {
	Steps         []string `json:"steps"`
	EstimatedTime string   `json:"estimated_time"`
	Regime        string   `json:"regime"`
}

// LoadFixture loads a guard test fixture from testdata
func LoadFixture(t *testing.T, filename string) *GuardTestFixture {
	fixturePath := filepath.Join("../../../testdata/guards", filename)

	data, err := ioutil.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to load fixture %s: %v", filename, err)
	}

	var fixture GuardTestFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("Failed to parse fixture %s: %v", filename, err)
	}

	return &fixture
}

// LoadGoldenFile loads expected golden output for comparison
func LoadGoldenFile(t *testing.T, filename string) string {
	goldenPath := filepath.Join("../../../testdata/guards/golden", filename)

	data, err := ioutil.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to load golden file %s: %v", filename, err)
	}

	return string(data)
}

// SaveGoldenFile saves actual output as golden file for future comparison
func SaveGoldenFile(t *testing.T, filename, content string) {
	goldenPath := filepath.Join("../../../testdata/guards/golden", filename)

	if err := ioutil.WriteFile(goldenPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to save golden file %s: %v", filename, err)
	}
}

// MockGuardEvaluator provides deterministic guard evaluation for testing
type MockGuardEvaluator struct {
	Regime    string
	Fixture   *GuardTestFixture
	StartTime time.Time
}

// NewMockEvaluator creates a new mock evaluator with fixture data
func NewMockEvaluator(fixture *GuardTestFixture) *MockGuardEvaluator {
	return &MockGuardEvaluator{
		Regime:    fixture.Regime,
		Fixture:   fixture,
		StartTime: time.Now(),
	}
}

// EvaluateAllGuards performs mock guard evaluation using fixture expectations
func (m *MockGuardEvaluator) EvaluateAllGuards() *GuardEvaluationResult {
	result := &GuardEvaluationResult{
		Regime:          m.Regime,
		Timestamp:       m.Fixture.Timestamp,
		TotalCandidates: len(m.Fixture.Candidates),
		PassCount:       m.Fixture.Expected.PassCount,
		FailCount:       m.Fixture.Expected.FailCount,
		ExitCode:        m.Fixture.Expected.ExitCode,
		Results:         make([]GuardResult, 0, len(m.Fixture.Expected.GuardResults)),
		ProgressLog:     m.generateProgressLog(),
	}

	// Convert expected results to actual result format
	for _, expected := range m.Fixture.Expected.GuardResults {
		guardResult := GuardResult{
			Symbol:      expected.Symbol,
			Status:      expected.Status,
			FailedGuard: expected.FailedGuard,
			Reason:      expected.Reason,
			FixHint:     expected.FixHint,
		}
		result.Results = append(result.Results, guardResult)
	}

	return result
}

// GuardEvaluationResult represents the complete result of guard evaluation
type GuardEvaluationResult struct {
	Regime          string        `json:"regime"`
	Timestamp       string        `json:"timestamp"`
	TotalCandidates int           `json:"total_candidates"`
	PassCount       int           `json:"pass_count"`
	FailCount       int           `json:"fail_count"`
	ExitCode        int           `json:"exit_code"`
	Results         []GuardResult `json:"results"`
	ProgressLog     []string      `json:"progress_log"`
}

// GuardResult represents the outcome for a single candidate
type GuardResult struct {
	Symbol      string `json:"symbol"`
	Status      string `json:"status"`
	FailedGuard string `json:"failed_guard,omitempty"`
	Reason      string `json:"reason,omitempty"`
	FixHint     string `json:"fix_hint,omitempty"`
}

// generateProgressLog creates realistic progress breadcrumbs for the test scenario
func (m *MockGuardEvaluator) generateProgressLog() []string {
	candidateCount := len(m.Fixture.Candidates)

	steps := []string{
		fmt.Sprintf("⏳ Starting guard evaluation (regime: %s)", m.Regime),
		fmt.Sprintf("📊 Processing %d candidates", candidateCount),
	}

	// Add per-guard-type progress steps
	guardTypes := []string{"freshness", "fatigue", "liquidity", "caps", "policy"}
	for i, guardType := range guardTypes {
		progress := (i + 1) * 100 / len(guardTypes)
		steps = append(steps, fmt.Sprintf("🛡️ [%d%%] Evaluating %s guards...", progress, guardType))
	}

	steps = append(steps, "✅ Guard evaluation completed")

	return steps
}

// FormatProgressOutput formats progress log for display testing
func (r *GuardEvaluationResult) FormatProgressOutput() string {
	output := ""
	for _, step := range r.ProgressLog {
		output += step + "\n"
	}
	return output
}

// FormatTableOutput formats results as ASCII table for UX testing
func (r *GuardEvaluationResult) FormatTableOutput() string {
	output := fmt.Sprintf("🛡️ Guard Results (%s regime) - %s\n", r.Regime, r.Timestamp[:19])
	output += "┌──────────┬────────┬─────────────┬──────────────────────────────────────┐\n"
	output += "│ Symbol   │ Status │ Failed Guard│ Reason                               │\n"
	output += "├──────────┼────────┼─────────────┼──────────────────────────────────────┤\n"

	for _, result := range r.Results {
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

		output += fmt.Sprintf("│ %-8s │ %-6s │ %-11s │ %-36s │\n",
			result.Symbol, status, failedGuard, reason)
	}

	output += "└──────────┴────────┴─────────────┴──────────────────────────────────────┘\n"
	output += fmt.Sprintf("Summary: %d passed, %d failed\n", r.PassCount, r.FailCount)

	return output
}

// AssertExpectedResults validates that actual results match fixture expectations
func (fixture *GuardTestFixture) AssertExpectedResults(t *testing.T, actual *GuardEvaluationResult) {
	// Check summary counts
	if actual.PassCount != fixture.Expected.PassCount {
		t.Errorf("Expected pass count %d, got %d", fixture.Expected.PassCount, actual.PassCount)
	}

	if actual.FailCount != fixture.Expected.FailCount {
		t.Errorf("Expected fail count %d, got %d", fixture.Expected.FailCount, actual.FailCount)
	}

	if actual.ExitCode != fixture.Expected.ExitCode {
		t.Errorf("Expected exit code %d, got %d", fixture.Expected.ExitCode, actual.ExitCode)
	}

	// Check individual results
	if len(actual.Results) != len(fixture.Expected.GuardResults) {
		t.Errorf("Expected %d results, got %d", len(fixture.Expected.GuardResults), len(actual.Results))
		return
	}

	for i, expected := range fixture.Expected.GuardResults {
		actual := actual.Results[i]

		if actual.Symbol != expected.Symbol {
			t.Errorf("Result %d: expected symbol %s, got %s", i, expected.Symbol, actual.Symbol)
		}

		if actual.Status != expected.Status {
			t.Errorf("Result %d (%s): expected status %s, got %s", i, expected.Symbol, expected.Status, actual.Status)
		}

		if actual.FailedGuard != expected.FailedGuard {
			t.Errorf("Result %d (%s): expected failed guard %s, got %s", i, expected.Symbol, expected.FailedGuard, actual.FailedGuard)
		}

		if actual.Reason != expected.Reason {
			t.Errorf("Result %d (%s): expected reason %s, got %s", i, expected.Symbol, expected.Reason, actual.Reason)
		}
	}
}
