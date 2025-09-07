package gates

import (
	"context"
	"testing"
	"time"
)

func TestGuardMetrics_AllGuardsPass(t *testing.T) {
	gm := NewGuardMetrics(DefaultGuardConfig())

	inputs := GuardInputs{
		Symbol:              "BTCUSD",
		BarsSinceSignal:     1,     // Within 2 bar limit
		PriceChange24h:      8.0,   // Below fatigue threshold
		RSI4h:               60.0,  // Below overbought threshold
		DistanceFromTrigger: 0.001, // Very close to trigger
		ATR1h:               0.01,  // ATR for proximity calculation
		SecondsSinceTrigger: 15,    // Within 30s limit
		HasPullback:         false, // Not needed
		HasAcceleration:     false, // Not needed
		Timestamp:           time.Now(),
	}

	result, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.AllPassed {
		t.Errorf("Expected all guards to pass, got failures: %v", result.FailureReasons)
	}

	if len(result.PassedGuards) != 4 {
		t.Errorf("Expected 4 guards to pass, got %d", len(result.PassedGuards))
	}

	expectedGuards := []string{"freshness", "fatigue", "proximity", "late_fill"}
	for _, guardName := range expectedGuards {
		if guard, exists := result.GuardChecks[guardName]; !exists {
			t.Errorf("Expected guard %s to exist", guardName)
		} else if !guard.Passed {
			t.Errorf("Expected guard %s to pass, got: %s", guardName, guard.Description)
		}
	}
}

func TestGuardMetrics_FreshnessGuardFails(t *testing.T) {
	gm := NewGuardMetrics(DefaultGuardConfig())

	inputs := GuardInputs{
		Symbol:              "BTCUSD",
		BarsSinceSignal:     3, // Exceeds 2 bar limit
		PriceChange24h:      8.0,
		RSI4h:               60.0,
		DistanceFromTrigger: 0.001,
		ATR1h:               0.01,
		SecondsSinceTrigger: 15,
		Timestamp:           time.Now(),
	}

	result, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.AllPassed {
		t.Error("Expected guards to fail due to stale signal")
	}

	freshnessGuard := result.GuardChecks["freshness"]
	if freshnessGuard.Passed {
		t.Error("Expected freshness guard to fail")
	}

	if len(result.FailureReasons) == 0 {
		t.Error("Expected failure reasons to be present")
	}

	if !contains(result.FailureReasons[0], "Stale signal") {
		t.Errorf("Expected failure reason to mention stale signal, got: %s", result.FailureReasons[0])
	}
}

func TestGuardMetrics_FatigueGuardFails(t *testing.T) {
	gm := NewGuardMetrics(DefaultGuardConfig())

	inputs := GuardInputs{
		Symbol:              "BTCUSD",
		BarsSinceSignal:     1,
		PriceChange24h:      15.0, // Above 12% threshold
		RSI4h:               75.0, // Above 70 threshold
		DistanceFromTrigger: 0.001,
		ATR1h:               0.01,
		SecondsSinceTrigger: 15,
		HasPullback:         false, // No exception
		HasAcceleration:     false, // No exception
		Timestamp:           time.Now(),
	}

	result, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.AllPassed {
		t.Error("Expected guards to fail due to fatigue")
	}

	fatigueGuard := result.GuardChecks["fatigue"]
	if fatigueGuard.Passed {
		t.Error("Expected fatigue guard to fail")
	}

	if len(result.FailureReasons) == 0 {
		t.Error("Expected failure reasons to be present")
	}

	if !contains(result.FailureReasons[0], "Fatigue detected") {
		t.Errorf("Expected failure reason to mention fatigue, got: %s", result.FailureReasons[0])
	}
}

func TestGuardMetrics_FatigueGuardPassesWithPullback(t *testing.T) {
	gm := NewGuardMetrics(DefaultGuardConfig())

	inputs := GuardInputs{
		Symbol:              "BTCUSD",
		BarsSinceSignal:     1,
		PriceChange24h:      15.0, // Above 12% threshold
		RSI4h:               75.0, // Above 70 threshold
		DistanceFromTrigger: 0.001,
		ATR1h:               0.01,
		SecondsSinceTrigger: 15,
		HasPullback:         true, // Exception present
		HasAcceleration:     false,
		Timestamp:           time.Now(),
	}

	result, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.AllPassed {
		t.Errorf("Expected all guards to pass with pullback exception, got failures: %v", result.FailureReasons)
	}

	fatigueGuard := result.GuardChecks["fatigue"]
	if !fatigueGuard.Passed {
		t.Errorf("Expected fatigue guard to pass with pullback, got: %s", fatigueGuard.Description)
	}

	if !contains(fatigueGuard.Description, "EXCEPTION") {
		t.Errorf("Expected fatigue description to mention exception, got: %s", fatigueGuard.Description)
	}
}

func TestGuardMetrics_FatigueGuardPassesWithAcceleration(t *testing.T) {
	gm := NewGuardMetrics(DefaultGuardConfig())

	inputs := GuardInputs{
		Symbol:              "BTCUSD",
		BarsSinceSignal:     1,
		PriceChange24h:      15.0, // Above 12% threshold
		RSI4h:               75.0, // Above 70 threshold
		DistanceFromTrigger: 0.001,
		ATR1h:               0.01,
		SecondsSinceTrigger: 15,
		HasPullback:         false,
		HasAcceleration:     true, // Exception present
		Timestamp:           time.Now(),
	}

	result, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.AllPassed {
		t.Errorf("Expected all guards to pass with acceleration exception, got failures: %v", result.FailureReasons)
	}

	fatigueGuard := result.GuardChecks["fatigue"]
	if !fatigueGuard.Passed {
		t.Errorf("Expected fatigue guard to pass with acceleration, got: %s", fatigueGuard.Description)
	}
}

func TestGuardMetrics_ProximityGuardFails(t *testing.T) {
	gm := NewGuardMetrics(DefaultGuardConfig())

	inputs := GuardInputs{
		Symbol:              "BTCUSD",
		BarsSinceSignal:     1,
		PriceChange24h:      8.0,
		RSI4h:               60.0,
		DistanceFromTrigger: 0.02, // Too far from trigger
		ATR1h:               0.01, // 1.2x ATR = 0.012, so 0.02 > 0.012
		SecondsSinceTrigger: 15,
		Timestamp:           time.Now(),
	}

	result, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.AllPassed {
		t.Error("Expected guards to fail due to price distance")
	}

	proximityGuard := result.GuardChecks["proximity"]
	if proximityGuard.Passed {
		t.Error("Expected proximity guard to fail")
	}

	if len(result.FailureReasons) == 0 {
		t.Error("Expected failure reasons to be present")
	}

	if !contains(result.FailureReasons[0], "Price too far") {
		t.Errorf("Expected failure reason to mention price distance, got: %s", result.FailureReasons[0])
	}
}

func TestGuardMetrics_LateFillGuardFails(t *testing.T) {
	gm := NewGuardMetrics(DefaultGuardConfig())

	inputs := GuardInputs{
		Symbol:              "BTCUSD",
		BarsSinceSignal:     1,
		PriceChange24h:      8.0,
		RSI4h:               60.0,
		DistanceFromTrigger: 0.001,
		ATR1h:               0.01,
		SecondsSinceTrigger: 45, // Exceeds 30s limit
		Timestamp:           time.Now(),
	}

	result, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.AllPassed {
		t.Error("Expected guards to fail due to late fill")
	}

	lateFillGuard := result.GuardChecks["late_fill"]
	if lateFillGuard.Passed {
		t.Error("Expected late fill guard to fail")
	}

	if len(result.FailureReasons) == 0 {
		t.Error("Expected failure reasons to be present")
	}

	if !contains(result.FailureReasons[0], "Late fill") {
		t.Errorf("Expected failure reason to mention late fill, got: %s", result.FailureReasons[0])
	}
}

func TestGuardMetrics_MultipleGuardsFail(t *testing.T) {
	gm := NewGuardMetrics(DefaultGuardConfig())

	inputs := GuardInputs{
		Symbol:              "BTCUSD",
		BarsSinceSignal:     3,    // Fails freshness
		PriceChange24h:      15.0, // Fails fatigue
		RSI4h:               75.0, // Fails fatigue
		DistanceFromTrigger: 0.02, // Fails proximity
		ATR1h:               0.01,
		SecondsSinceTrigger: 45,    // Fails late fill
		HasPullback:         false, // No exceptions
		HasAcceleration:     false,
		Timestamp:           time.Now(),
	}

	result, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.AllPassed {
		t.Error("Expected guards to fail with multiple conditions failing")
	}

	// Should have 4 failure reasons
	expectedFailures := 4
	if len(result.FailureReasons) != expectedFailures {
		t.Errorf("Expected %d failure reasons, got %d: %v",
			expectedFailures, len(result.FailureReasons), result.FailureReasons)
	}

	// No guards should pass
	if len(result.PassedGuards) != 0 {
		t.Errorf("Expected no guards to pass, got %d: %v", len(result.PassedGuards), result.PassedGuards)
	}

	// All guard checks should fail
	expectedGuards := []string{"freshness", "fatigue", "proximity", "late_fill"}
	for _, guardName := range expectedGuards {
		if guard, exists := result.GuardChecks[guardName]; !exists {
			t.Errorf("Expected guard %s to exist", guardName)
		} else if guard.Passed {
			t.Errorf("Expected guard %s to fail, got: %s", guardName, guard.Description)
		}
	}
}

func TestGuardMetrics_CustomConfig(t *testing.T) {
	customConfig := &GuardConfig{
		MaxBarsAge:               1,    // More strict
		FatiguePrice24hThreshold: 10.0, // More strict
		FatigueRSI4hThreshold:    65.0, // More strict
		ProximityATRMultiple:     1.0,  // More strict
		MaxSecondsSinceTrigger:   20,   // More strict
	}

	gm := NewGuardMetrics(customConfig)

	inputs := GuardInputs{
		Symbol:              "BTCUSD",
		BarsSinceSignal:     2,     // Would pass default, fails custom
		PriceChange24h:      11.0,  // Would pass default, fails custom
		RSI4h:               68.0,  // Would pass default, fails custom
		DistanceFromTrigger: 0.011, // Would pass default, fails custom (1.0x ATR = 0.01)
		ATR1h:               0.01,
		SecondsSinceTrigger: 25, // Would pass default, fails custom
		HasPullback:         false,
		HasAcceleration:     false,
		Timestamp:           time.Now(),
	}

	result, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.AllPassed {
		t.Error("Expected guards to fail with stricter custom config")
	}

	// All guards should fail with stricter config
	expectedGuards := []string{"freshness", "fatigue", "proximity", "late_fill"}
	for _, guardName := range expectedGuards {
		if guard, exists := result.GuardChecks[guardName]; !exists {
			t.Errorf("Expected guard %s to exist", guardName)
		} else if guard.Passed {
			t.Errorf("Expected guard %s to fail with custom config, got: %s", guardName, guard.Description)
		}
	}
}

func TestGuardMetrics_Summary(t *testing.T) {
	gm := NewGuardMetrics(DefaultGuardConfig())

	inputs := GuardInputs{
		Symbol:              "BTCUSD",
		BarsSinceSignal:     1,
		PriceChange24h:      8.0,
		RSI4h:               60.0,
		DistanceFromTrigger: 0.001,
		ATR1h:               0.01,
		SecondsSinceTrigger: 15,
		Timestamp:           time.Now(),
	}

	result, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	summary := result.GetGuardSummary()
	if summary == "" {
		t.Error("Expected non-empty summary")
	}

	if result.AllPassed && !contains(summary, "✅") {
		t.Error("Expected success summary to contain ✅")
	}

	detailedReport := result.GetDetailedGuardReport()
	if detailedReport == "" {
		t.Error("Expected non-empty detailed report")
	}

	if !contains(detailedReport, result.Symbol) {
		t.Error("Expected detailed report to contain symbol")
	}

	// Test failure case
	inputs.BarsSinceSignal = 3 // Make freshness fail
	failResult, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	failSummary := failResult.GetGuardSummary()
	if !contains(failSummary, "❌") {
		t.Error("Expected failure summary to contain ❌")
	}
}

func TestGuardMetrics_DefaultConfig(t *testing.T) {
	config := DefaultGuardConfig()

	if config.MaxBarsAge != 2 {
		t.Errorf("Expected MaxBarsAge=2, got %d", config.MaxBarsAge)
	}

	if config.FatiguePrice24hThreshold != 12.0 {
		t.Errorf("Expected FatiguePrice24hThreshold=12.0, got %.1f", config.FatiguePrice24hThreshold)
	}

	if config.FatigueRSI4hThreshold != 70.0 {
		t.Errorf("Expected FatigueRSI4hThreshold=70.0, got %.1f", config.FatigueRSI4hThreshold)
	}

	if config.ProximityATRMultiple != 1.2 {
		t.Errorf("Expected ProximityATRMultiple=1.2, got %.1f", config.ProximityATRMultiple)
	}

	if config.MaxSecondsSinceTrigger != 30 {
		t.Errorf("Expected MaxSecondsSinceTrigger=30, got %d", config.MaxSecondsSinceTrigger)
	}
}

func TestGuardMetrics_NilConfig(t *testing.T) {
	gm := NewGuardMetrics(nil) // Should use default config

	inputs := GuardInputs{
		Symbol:              "BTCUSD",
		BarsSinceSignal:     1,
		PriceChange24h:      8.0,
		RSI4h:               60.0,
		DistanceFromTrigger: 0.001,
		ATR1h:               0.01,
		SecondsSinceTrigger: 15,
		Timestamp:           time.Now(),
	}

	result, err := gm.EvaluateGuards(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error with nil config, got %v", err)
	}

	if !result.AllPassed {
		t.Errorf("Expected all guards to pass with default config, got failures: %v", result.FailureReasons)
	}
}
