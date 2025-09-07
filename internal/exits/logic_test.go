package exits

import (
	"context"
	"testing"
	"time"
)

func TestExitEvaluator_NoExit(t *testing.T) {
	evaluator := NewExitEvaluator(DefaultExitConfig())

	inputs := ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       52000.0, // 4% profit
		EntryTime:          time.Now().Add(-2 * time.Hour),
		CurrentTime:        time.Now(),
		HardStopPrice:      48000.0, // 4% stop, not hit
		VenueP99LatencyMs:  500,     // Good latency
		VenueErrorRate:     1.0,     // Low error rate
		VenueRejectRate:    2.0,     // Low reject rate
		MaxHoldHours:       48.0,    // Within time limit
		MomentumScore:      85.0,    // Good momentum
		EntryMomentumScore: 80.0,    // Improved since entry
		MomentumAccel:      1.2,     // Good acceleration
		EntryAccel:         1.0,     // Improved since entry
		HighWaterMark:      52000.0, // Same as current
		TrailingStopPct:    5.0,     // 5% trailing
		ProfitTarget1:      15.0,    // 15% target not hit
		ProfitTarget2:      30.0,    // 30% target not hit
		ProfitTargetPrice1: 57500.0, // Calculated targets
		ProfitTargetPrice2: 65000.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.ShouldExit {
		t.Errorf("Expected no exit, but got exit recommendation: %s", result.ReasonString)
	}

	if result.ExitReason != NoExit {
		t.Errorf("Expected NoExit reason, got %s", result.ExitReason.String())
	}

	if result.UnrealizedPnL <= 0 {
		t.Errorf("Expected positive PnL, got %.2f%%", result.UnrealizedPnL)
	}
}

func TestExitEvaluator_HardStopTriggered(t *testing.T) {
	evaluator := NewExitEvaluator(DefaultExitConfig())

	inputs := ExitInputs{
		Symbol:            "BTCUSD",
		EntryPrice:        50000.0,
		CurrentPrice:      47000.0, // Below hard stop
		HardStopPrice:     48000.0, // Hard stop triggered
		EntryTime:         time.Now().Add(-1 * time.Hour),
		CurrentTime:       time.Now(),
		VenueP99LatencyMs: 500,
		VenueErrorRate:    1.0,
		VenueRejectRate:   2.0,
		MaxHoldHours:      48.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.ShouldExit {
		t.Error("Expected exit due to hard stop")
	}

	if result.ExitReason != HardStop {
		t.Errorf("Expected HardStop reason, got %s", result.ExitReason.String())
	}

	if !contains(result.TriggeredBy, "stop") {
		t.Errorf("Expected trigger description to mention stop, got: %s", result.TriggeredBy)
	}

	if result.UnrealizedPnL >= 0 {
		t.Errorf("Expected negative PnL with hard stop, got %.2f%%", result.UnrealizedPnL)
	}
}

func TestExitEvaluator_VenueHealthCut(t *testing.T) {
	evaluator := NewExitEvaluator(DefaultExitConfig())

	inputs := ExitInputs{
		Symbol:            "BTCUSD",
		EntryPrice:        50000.0,
		CurrentPrice:      51000.0, // Small profit
		HardStopPrice:     48000.0, // Not triggered
		EntryTime:         time.Now().Add(-1 * time.Hour),
		CurrentTime:       time.Now(),
		VenueP99LatencyMs: 3000, // Above 2000ms threshold
		VenueErrorRate:    1.0,  // Below threshold individually
		VenueRejectRate:   2.0,  // Below threshold individually
		MaxHoldHours:      48.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.ShouldExit {
		t.Error("Expected exit due to venue health")
	}

	if result.ExitReason != VenueHealthCut {
		t.Errorf("Expected VenueHealthCut reason, got %s", result.ExitReason.String())
	}

	if !contains(result.TriggeredBy, "Venue degraded") {
		t.Errorf("Expected trigger description to mention venue degradation, got: %s", result.TriggeredBy)
	}
}

func TestExitEvaluator_TimeLimit(t *testing.T) {
	evaluator := NewExitEvaluator(DefaultExitConfig())

	inputs := ExitInputs{
		Symbol:            "BTCUSD",
		EntryPrice:        50000.0,
		CurrentPrice:      51000.0,                         // Small profit
		HardStopPrice:     48000.0,                         // Not triggered
		EntryTime:         time.Now().Add(-49 * time.Hour), // Exceeds 48h limit
		CurrentTime:       time.Now(),
		VenueP99LatencyMs: 500, // Good venue health
		VenueErrorRate:    1.0,
		VenueRejectRate:   2.0,
		MaxHoldHours:      48.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.ShouldExit {
		t.Error("Expected exit due to time limit")
	}

	if result.ExitReason != TimeLimit {
		t.Errorf("Expected TimeLimit reason, got %s", result.ExitReason.String())
	}

	if !contains(result.TriggeredBy, "hour limit") {
		t.Errorf("Expected trigger description to mention hour limit, got: %s", result.TriggeredBy)
	}

	if result.HoursHeld < 48.0 {
		t.Errorf("Expected hours held >= 48, got %.1f", result.HoursHeld)
	}
}

func TestExitEvaluator_AccelerationReversal(t *testing.T) {
	evaluator := NewExitEvaluator(DefaultExitConfig())

	inputs := ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       51000.0, // Small profit
		HardStopPrice:      48000.0, // Not triggered
		EntryTime:          time.Now().Add(-1 * time.Hour),
		CurrentTime:        time.Now(),
		VenueP99LatencyMs:  500, // Good venue health
		VenueErrorRate:     1.0,
		VenueRejectRate:    2.0,
		MaxHoldHours:       48.0,
		MomentumScore:      75.0, // Still decent
		EntryMomentumScore: 80.0,
		MomentumAccel:      0.3, // Significantly reduced from entry
		EntryAccel:         1.0, // 70% drop = reversal
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.ShouldExit {
		t.Error("Expected exit due to acceleration reversal")
	}

	if result.ExitReason != AccelerationReversal {
		t.Errorf("Expected AccelerationReversal reason, got %s", result.ExitReason.String())
	}

	if !contains(result.TriggeredBy, "Acceleration reversed") {
		t.Errorf("Expected trigger description to mention acceleration reversal, got: %s", result.TriggeredBy)
	}
}

func TestExitEvaluator_MomentumFade(t *testing.T) {
	evaluator := NewExitEvaluator(DefaultExitConfig())

	inputs := ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       51000.0, // Small profit
		HardStopPrice:      48000.0, // Not triggered
		EntryTime:          time.Now().Add(-1 * time.Hour),
		CurrentTime:        time.Now(),
		VenueP99LatencyMs:  500, // Good venue health
		VenueErrorRate:     1.0,
		VenueRejectRate:    2.0,
		MaxHoldHours:       48.0,
		MomentumScore:      50.0, // Significantly reduced
		EntryMomentumScore: 80.0, // 37.5% decline > 30% threshold
		MomentumAccel:      0.8,  // Decent acceleration still
		EntryAccel:         1.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.ShouldExit {
		t.Error("Expected exit due to momentum fade")
	}

	if result.ExitReason != MomentumFade {
		t.Errorf("Expected MomentumFade reason, got %s", result.ExitReason.String())
	}

	if !contains(result.TriggeredBy, "Momentum faded") {
		t.Errorf("Expected trigger description to mention momentum fade, got: %s", result.TriggeredBy)
	}
}

func TestExitEvaluator_TrailingStop(t *testing.T) {
	evaluator := NewExitEvaluator(DefaultExitConfig())

	inputs := ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       52000.0, // Current price
		HardStopPrice:      48000.0, // Not triggered
		EntryTime:          time.Now().Add(-1 * time.Hour),
		CurrentTime:        time.Now(),
		VenueP99LatencyMs:  500, // Good venue health
		VenueErrorRate:     1.0,
		VenueRejectRate:    2.0,
		MaxHoldHours:       48.0,
		MomentumScore:      80.0, // Good momentum
		EntryMomentumScore: 80.0,
		MomentumAccel:      1.0, // Good acceleration
		EntryAccel:         1.0,
		HighWaterMark:      55000.0, // Was higher, now trailing stop triggered
		TrailingStopPct:    5.0,     // 5% trailing = 52250 stop, current 52000 < stop
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.ShouldExit {
		t.Error("Expected exit due to trailing stop")
	}

	if result.ExitReason != TrailingStop {
		t.Errorf("Expected TrailingStop reason, got %s", result.ExitReason.String())
	}

	if !contains(result.TriggeredBy, "Trailing stop") {
		t.Errorf("Expected trigger description to mention trailing stop, got: %s", result.TriggeredBy)
	}
}

func TestExitEvaluator_ProfitTarget1(t *testing.T) {
	evaluator := NewExitEvaluator(DefaultExitConfig())

	inputs := ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       57600.0, // 15.2% profit, hits target 1
		HardStopPrice:      48000.0, // Not triggered
		EntryTime:          time.Now().Add(-1 * time.Hour),
		CurrentTime:        time.Now(),
		VenueP99LatencyMs:  500, // Good venue health
		VenueErrorRate:     1.0,
		VenueRejectRate:    2.0,
		MaxHoldHours:       48.0,
		MomentumScore:      80.0, // Good momentum
		EntryMomentumScore: 80.0,
		MomentumAccel:      1.0, // Good acceleration
		EntryAccel:         1.0,
		HighWaterMark:      57600.0, // Same as current
		TrailingStopPct:    5.0,
		ProfitTarget1:      15.0, // 15% target hit
		ProfitTarget2:      30.0,
		ProfitTargetPrice1: 57500.0, // Current > target 1
		ProfitTargetPrice2: 65000.0, // Current < target 2
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.ShouldExit {
		t.Error("Expected exit due to profit target 1")
	}

	if result.ExitReason != ProfitTarget {
		t.Errorf("Expected ProfitTarget reason, got %s", result.ExitReason.String())
	}

	if !contains(result.TriggeredBy, "Profit target 1") {
		t.Errorf("Expected trigger description to mention profit target 1, got: %s", result.TriggeredBy)
	}
}

func TestExitEvaluator_ProfitTarget2(t *testing.T) {
	evaluator := NewExitEvaluator(DefaultExitConfig())

	inputs := ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       65500.0, // 31% profit, hits target 2
		HardStopPrice:      48000.0, // Not triggered
		EntryTime:          time.Now().Add(-1 * time.Hour),
		CurrentTime:        time.Now(),
		VenueP99LatencyMs:  500, // Good venue health
		VenueErrorRate:     1.0,
		VenueRejectRate:    2.0,
		MaxHoldHours:       48.0,
		MomentumScore:      80.0, // Good momentum
		EntryMomentumScore: 80.0,
		MomentumAccel:      1.0, // Good acceleration
		EntryAccel:         1.0,
		HighWaterMark:      65500.0, // Same as current
		TrailingStopPct:    5.0,
		ProfitTarget1:      15.0,
		ProfitTarget2:      30.0,    // 30% target hit
		ProfitTargetPrice1: 57500.0, // Both targets exceeded
		ProfitTargetPrice2: 65000.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.ShouldExit {
		t.Error("Expected exit due to profit target 2")
	}

	if result.ExitReason != ProfitTarget {
		t.Errorf("Expected ProfitTarget reason, got %s", result.ExitReason.String())
	}

	if !contains(result.TriggeredBy, "Profit target 2") {
		t.Errorf("Expected trigger description to mention profit target 2, got: %s", result.TriggeredBy)
	}
}

func TestExitEvaluator_ExitPrecedence(t *testing.T) {
	evaluator := NewExitEvaluator(DefaultExitConfig())

	// Set up inputs where multiple exit conditions are met
	inputs := ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       47000.0,                         // Below hard stop AND profit target could be hit with different price
		HardStopPrice:      48000.0,                         // Hard stop triggered (highest precedence)
		EntryTime:          time.Now().Add(-49 * time.Hour), // Time limit also exceeded
		CurrentTime:        time.Now(),
		VenueP99LatencyMs:  3000, // Venue health also bad
		VenueErrorRate:     5.0,  // Also exceeds error threshold
		VenueRejectRate:    2.0,
		MaxHoldHours:       48.0,
		MomentumScore:      40.0, // Momentum also faded
		EntryMomentumScore: 80.0,
		MomentumAccel:      0.2, // Acceleration also reversed
		EntryAccel:         1.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.ShouldExit {
		t.Error("Expected exit due to multiple conditions")
	}

	// Should exit with highest precedence reason (hard stop)
	if result.ExitReason != HardStop {
		t.Errorf("Expected HardStop (highest precedence) reason, got %s", result.ExitReason.String())
	}
}

func TestExitEvaluator_DisabledExits(t *testing.T) {
	config := &ExitConfig{
		EnableHardStop:         false, // Disable hard stop
		MaxVenueP99LatencyMs:   2000,
		MaxVenueErrorRate:      3.0,
		MaxVenueRejectRate:     5.0,
		DefaultMaxHoldHours:    48.0,
		MomentumFadeThreshold:  30.0,
		AccelReversalThreshold: 50.0,
		EnableTrailingStop:     false, // Disable trailing stop
		EnableProfitTargets:    false, // Disable profit targets
	}

	evaluator := NewExitEvaluator(config)

	inputs := ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       47000.0, // Would trigger hard stop if enabled
		HardStopPrice:      48000.0,
		EntryTime:          time.Now().Add(-1 * time.Hour),
		CurrentTime:        time.Now(),
		VenueP99LatencyMs:  500, // Good venue health
		VenueErrorRate:     1.0,
		VenueRejectRate:    2.0,
		MaxHoldHours:       48.0,
		MomentumScore:      80.0, // Good momentum
		EntryMomentumScore: 80.0,
		MomentumAccel:      1.0,
		EntryAccel:         1.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.ShouldExit {
		t.Errorf("Expected no exit with disabled features, but got: %s", result.ReasonString)
	}

	if result.ExitReason != NoExit {
		t.Errorf("Expected NoExit reason, got %s", result.ExitReason.String())
	}
}

func TestExitEvaluator_Summary(t *testing.T) {
	evaluator := NewExitEvaluator(DefaultExitConfig())

	inputs := ExitInputs{
		Symbol:       "BTCUSD",
		EntryPrice:   50000.0,
		CurrentPrice: 52000.0,
		EntryTime:    time.Now().Add(-2 * time.Hour),
		CurrentTime:  time.Now(),
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	summary := result.GetExitSummary()
	if summary == "" {
		t.Error("Expected non-empty summary")
	}

	if !result.ShouldExit && !contains(summary, "✅") {
		t.Error("Expected hold summary to contain ✅")
	}

	detailedReport := result.GetDetailedExitReport()
	if detailedReport == "" {
		t.Error("Expected non-empty detailed report")
	}

	if !contains(detailedReport, result.Symbol) {
		t.Error("Expected detailed report to contain symbol")
	}

	// Test exit case
	inputs.CurrentPrice = 47000.0 // Trigger hard stop
	inputs.HardStopPrice = 48000.0

	exitResult, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	exitSummary := exitResult.GetExitSummary()
	if !contains(exitSummary, "🚪") {
		t.Error("Expected exit summary to contain 🚪")
	}
}

func TestExitReason_String(t *testing.T) {
	testCases := []struct {
		reason   ExitReason
		expected string
	}{
		{NoExit, "no_exit"},
		{HardStop, "hard_stop"},
		{VenueHealthCut, "venue_health_cut"},
		{TimeLimit, "time_limit"},
		{AccelerationReversal, "acceleration_reversal"},
		{MomentumFade, "momentum_fade"},
		{TrailingStop, "trailing_stop"},
		{ProfitTarget, "profit_target"},
		{ExitReason(999), "unknown"},
	}

	for _, tc := range testCases {
		if tc.reason.String() != tc.expected {
			t.Errorf("Expected %s.String() = %s, got %s",
				tc.reason, tc.expected, tc.reason.String())
		}
	}
}

func TestDefaultExitConfig(t *testing.T) {
	config := DefaultExitConfig()

	if !config.EnableHardStop {
		t.Error("Expected EnableHardStop to be true")
	}

	if config.MaxVenueP99LatencyMs != 2000 {
		t.Errorf("Expected MaxVenueP99LatencyMs=2000, got %d", config.MaxVenueP99LatencyMs)
	}

	if config.MaxVenueErrorRate != 3.0 {
		t.Errorf("Expected MaxVenueErrorRate=3.0, got %.1f", config.MaxVenueErrorRate)
	}

	if config.DefaultMaxHoldHours != 48.0 {
		t.Errorf("Expected DefaultMaxHoldHours=48.0, got %.1f", config.DefaultMaxHoldHours)
	}

	if config.MomentumFadeThreshold != 30.0 {
		t.Errorf("Expected MomentumFadeThreshold=30.0, got %.1f", config.MomentumFadeThreshold)
	}

	if config.AccelReversalThreshold != 50.0 {
		t.Errorf("Expected AccelReversalThreshold=50.0, got %.1f", config.AccelReversalThreshold)
	}

	if !config.EnableTrailingStop {
		t.Error("Expected EnableTrailingStop to be true")
	}

	if config.DefaultTrailingPct != 5.0 {
		t.Errorf("Expected DefaultTrailingPct=5.0, got %.1f", config.DefaultTrailingPct)
	}

	if !config.EnableProfitTargets {
		t.Error("Expected EnableProfitTargets to be true")
	}

	if config.DefaultProfitTarget1 != 15.0 {
		t.Errorf("Expected DefaultProfitTarget1=15.0, got %.1f", config.DefaultProfitTarget1)
	}

	if config.DefaultProfitTarget2 != 30.0 {
		t.Errorf("Expected DefaultProfitTarget2=30.0, got %.1f", config.DefaultProfitTarget2)
	}
}

// Helper function for testing
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != substr && findSubstring(s, substr, 0)
}

func findSubstring(s, substr string, start int) bool {
	if start > len(s)-len(substr) {
		return false
	}
	if s[start:start+len(substr)] == substr {
		return true
	}
	return findSubstring(s, substr, start+1)
}
