package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cryptorun/internal/exits"
)

// GateOrchestrator provides unified entry/exit gate evaluation
type GateOrchestrator struct {
	entryEvaluator *EntryGateEvaluator
	guardMetrics   *GuardMetrics
	exitEvaluator  *exits.ExitEvaluator
}

// NewGateOrchestrator creates a comprehensive gate evaluation system
func NewGateOrchestrator(
	entryEvaluator *EntryGateEvaluator,
	guardMetrics *GuardMetrics,
	exitEvaluator *exits.ExitEvaluator,
) *GateOrchestrator {
	return &GateOrchestrator{
		entryEvaluator: entryEvaluator,
		guardMetrics:   guardMetrics,
		exitEvaluator:  exitEvaluator,
	}
}

// EntryEvaluationInputs contains all data needed for complete entry evaluation
type EntryEvaluationInputs struct {
	// Basic position data
	Symbol         string    `json:"symbol"`
	CompositeScore float64   `json:"composite_score"`
	PriceChange24h float64   `json:"price_change_24h"`
	Timestamp      time.Time `json:"timestamp"`
	
	// Guard metrics
	BarsSinceSignal     int     `json:"bars_since_signal"`
	RSI4h               float64 `json:"rsi_4h"`
	DistanceFromTrigger float64 `json:"distance_from_trigger"`
	ATR1h               float64 `json:"atr_1h"`
	SecondsSinceTrigger int64   `json:"seconds_since_trigger"`
	HasPullback         bool    `json:"has_pullback"`
	HasAcceleration     bool    `json:"has_acceleration"`
}

// EntryEvaluationResult combines hard gates and guard evaluation
type EntryEvaluationResult struct {
	Symbol           string           `json:"symbol"`
	Timestamp        time.Time        `json:"timestamp"`
	EntryAllowed     bool             `json:"entry_allowed"`
	CompositeScore   float64          `json:"composite_score"`
	
	// Gate results
	HardGateResult   *EntryGateResult `json:"hard_gate_result"`   // Score/VADR/Funding gates
	GuardResult      *GuardResult     `json:"guard_result"`       // Freshness/Fatigue/Proximity/LateFill
	
	// Summary
	OverallReason    string           `json:"overall_reason"`     // Why entry was allowed/blocked
	FailureReasons   []string         `json:"failure_reasons"`    // All failure reasons
	TotalGatesPassed int              `json:"total_gates_passed"` // Count of passed gates
	TotalGatesCount  int              `json:"total_gates_count"`  // Total gates evaluated
	EvaluationTimeMs int64            `json:"evaluation_time_ms"`
}

// GateReport provides a stable JSON snapshot for deterministic testing
type GateReport struct {
	Symbol           string                 `json:"symbol"`
	Timestamp        string                 `json:"timestamp"` // ISO format for stability
	EntryAllowed     bool                   `json:"entry_allowed"`
	CompositeScore   float64                `json:"composite_score"`
	HardGates        map[string]interface{} `json:"hard_gates"`     // Gate name -> result
	Guards           map[string]interface{} `json:"guards"`         // Guard name -> result
	ExitEvaluation   map[string]interface{} `json:"exit_evaluation,omitempty"` // If position exists
	Summary          string                 `json:"summary"`
	Version          string                 `json:"version"` // For schema versioning
}

// EvaluateEntry performs comprehensive entry evaluation (hard gates + guards)
func (go *GateOrchestrator) EvaluateEntry(ctx context.Context, inputs EntryEvaluationInputs) (*EntryEvaluationResult, error) {
	startTime := time.Now()
	
	// 1. Evaluate hard gates (Score, VADR, Funding divergence)
	hardGateResult, err := go.entryEvaluator.EvaluateEntry(ctx, inputs.Symbol, inputs.CompositeScore, inputs.PriceChange24h)
	if err != nil {
		return nil, fmt.Errorf("hard gate evaluation failed: %w", err)
	}
	
	// 2. Evaluate guards (Freshness, Fatigue, Proximity, Late-fill)
	guardInputs := GuardInputs{
		Symbol:              inputs.Symbol,
		BarsSinceSignal:     inputs.BarsSinceSignal,
		PriceChange24h:      inputs.PriceChange24h,
		RSI4h:               inputs.RSI4h,
		DistanceFromTrigger: inputs.DistanceFromTrigger,
		ATR1h:               inputs.ATR1h,
		SecondsSinceTrigger: inputs.SecondsSinceTrigger,
		HasPullback:         inputs.HasPullback,
		HasAcceleration:     inputs.HasAcceleration,
		Timestamp:           inputs.Timestamp,
	}
	
	guardResult, err := go.guardMetrics.EvaluateGuards(ctx, guardInputs)
	if err != nil {
		return nil, fmt.Errorf("guard evaluation failed: %w", err)
	}
	
	// 3. Combine results
	result := &EntryEvaluationResult{
		Symbol:         inputs.Symbol,
		Timestamp:      inputs.Timestamp,
		CompositeScore: inputs.CompositeScore,
		HardGateResult: hardGateResult,
		GuardResult:    guardResult,
	}
	
	// Entry is allowed only if ALL gates and guards pass
	result.EntryAllowed = hardGateResult.Passed && guardResult.AllPassed
	
	// Collect all failure reasons
	result.FailureReasons = append(result.FailureReasons, hardGateResult.FailureReasons...)
	result.FailureReasons = append(result.FailureReasons, guardResult.FailureReasons...)
	
	// Count gates
	result.TotalGatesCount = len(hardGateResult.GateResults) + len(guardResult.GuardChecks)
	result.TotalGatesPassed = len(hardGateResult.PassedGates) + len(guardResult.PassedGuards)
	
	// Generate overall reason
	if result.EntryAllowed {
		result.OverallReason = fmt.Sprintf("ENTRY CLEARED: All %d gates passed (score %.1f)", 
			result.TotalGatesCount, inputs.CompositeScore)
	} else {
		result.OverallReason = fmt.Sprintf("ENTRY BLOCKED: %d failures", len(result.FailureReasons))
		if len(result.FailureReasons) > 0 {
			result.OverallReason += fmt.Sprintf(" (primary: %s)", result.FailureReasons[0])
		}
	}
	
	result.EvaluationTimeMs = time.Since(startTime).Milliseconds()
	return result, nil
}

// EvaluateExit performs exit evaluation for existing positions
func (go *GateOrchestrator) EvaluateExit(ctx context.Context, inputs exits.ExitInputs) (*exits.ExitResult, error) {
	return go.exitEvaluator.EvaluateExit(ctx, inputs)
}

// GenerateGateReport creates a stable JSON report for deterministic testing
func (go *GateOrchestrator) GenerateGateReport(ctx context.Context, entryInputs EntryEvaluationInputs, exitInputs *exits.ExitInputs) (*GateReport, error) {
	// Evaluate entry
	entryResult, err := go.EvaluateEntry(ctx, entryInputs)
	if err != nil {
		return nil, fmt.Errorf("entry evaluation failed: %w", err)
	}
	
	report := &GateReport{
		Symbol:         entryResult.Symbol,
		Timestamp:      entryResult.Timestamp.Format(time.RFC3339),
		EntryAllowed:   entryResult.EntryAllowed,
		CompositeScore: entryResult.CompositeScore,
		HardGates:      make(map[string]interface{}),
		Guards:         make(map[string]interface{}),
		Summary:        entryResult.OverallReason,
		Version:        "v1.0",
	}
	
	// Serialize hard gate results
	for gateName, gateCheck := range entryResult.HardGateResult.GateResults {
		report.HardGates[gateName] = map[string]interface{}{
			"passed":    gateCheck.Passed,
			"value":     gateCheck.Value,
			"threshold": gateCheck.Threshold,
			"description": gateCheck.Description,
		}
	}
	
	// Serialize guard results  
	for guardName, guardCheck := range entryResult.GuardResult.GuardChecks {
		report.Guards[guardName] = map[string]interface{}{
			"passed":    guardCheck.Passed,
			"value":     guardCheck.Value,
			"threshold": guardCheck.Threshold,
			"description": guardCheck.Description,
		}
	}
	
	// Add exit evaluation if position data provided
	if exitInputs != nil {
		exitResult, err := go.EvaluateExit(ctx, *exitInputs)
		if err == nil {
			report.ExitEvaluation = map[string]interface{}{
				"should_exit":    exitResult.ShouldExit,
				"exit_reason":    exitResult.ExitReason.String(),
				"triggered_by":   exitResult.TriggeredBy,
				"unrealized_pnl": exitResult.UnrealizedPnL,
				"hours_held":     exitResult.HoursHeld,
			}
		}
	}
	
	return report, nil
}

// GetGateReportJSON returns the gate report as stable JSON for testing
func (gr *GateReport) GetGateReportJSON() (string, error) {
	jsonBytes, err := json.MarshalIndent(gr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal gate report: %w", err)
	}
	return string(jsonBytes), nil
}

// GetEntrySummary returns a concise entry evaluation summary
func (eer *EntryEvaluationResult) GetEntrySummary() string {
	if eer.EntryAllowed {
		return fmt.Sprintf("✅ ENTRY CLEARED — %s (score %.1f, %d/%d gates passed)", 
			eer.Symbol, eer.CompositeScore, eer.TotalGatesPassed, eer.TotalGatesCount)
	} else {
		return fmt.Sprintf("❌ ENTRY BLOCKED — %s (%d failures)", 
			eer.Symbol, len(eer.FailureReasons))
	}
}

// GetDetailedEntryReport returns comprehensive entry analysis
func (eer *EntryEvaluationResult) GetDetailedEntryReport() string {
	report := fmt.Sprintf("Entry Gate Evaluation: %s (%.1f score)\n", eer.Symbol, eer.CompositeScore)
	report += fmt.Sprintf("Decision: %s\n", 
		map[bool]string{true: "ENTRY ALLOWED ✅", false: "ENTRY BLOCKED ❌"}[eer.EntryAllowed])
	report += fmt.Sprintf("Gates: %d/%d passed | Evaluation: %dms\n\n", 
		eer.TotalGatesPassed, eer.TotalGatesCount, eer.EvaluationTimeMs)

	// Hard gates section
	report += "=== HARD GATES ===\n"
	gateOrder := []string{"composite_score", "vadr", "spread", "depth", "funding_divergence", "oi_residual", "etf_flows"}
	for _, gateName := range gateOrder {
		if gateCheck, exists := eer.HardGateResult.GateResults[gateName]; exists {
			status := map[bool]string{true: "✅", false: "❌"}[gateCheck.Passed]
			report += fmt.Sprintf("%s %s: %s\n", status, gateName, gateCheck.Description)
		}
	}
	
	// Guards section
	report += "\n=== GUARDS ===\n"
	guardOrder := []string{"freshness", "fatigue", "proximity", "late_fill"}
	for _, guardName := range guardOrder {
		if guardCheck, exists := eer.GuardResult.GuardChecks[guardName]; exists {
			status := map[bool]string{true: "✅", false: "❌"}[guardCheck.Passed]
			report += fmt.Sprintf("%s %s: %s\n", status, guardName, guardCheck.Description)
		}
	}

	// Failure summary
	if len(eer.FailureReasons) > 0 {
		report += fmt.Sprintf("\n=== FAILURES (%d) ===\n", len(eer.FailureReasons))
		for i, reason := range eer.FailureReasons {
			report += fmt.Sprintf("  %d. %s\n", i+1, reason)
		}
	}

	return report
}