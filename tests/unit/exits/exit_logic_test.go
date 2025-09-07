package exits

import (
	"context"
	"testing"
	"time"

	"cryptorun/internal/exits"
)

func TestExitEvaluator_NoExit(t *testing.T) {
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	inputs := exits.ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       52000.0,
		EntryTime:          time.Now().Add(-2 * time.Hour),
		CurrentTime:        time.Now(),
		HardStopPrice:      47500.0, // 5% stop, not triggered
		VenueP99LatencyMs:  1500,    // Under 2000ms threshold
		VenueErrorRate:     2.0,     // Under 3% threshold
		VenueRejectRate:    3.0,     // Under 5% threshold
		MaxHoldHours:       48.0,    // 2 hours < 48 hour limit
		MomentumScore:      85.0,    // Strong momentum
		EntryMomentumScore: 80.0,    // Slightly improved
		MomentumAccel:      0.5,     // Positive acceleration
		EntryAccel:         0.4,     // Improved from entry
		HighWaterMark:      52500.0, // HWM above current price
		TrailingStopPct:    5.0,     // 5% trailing
		ProfitTarget1:      15.0,    // 15% target
		ProfitTarget2:      30.0,    // 30% target
		ProfitTargetPrice1: 57500.0, // Not reached
		ProfitTargetPrice2: 65000.0, // Not reached
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("EvaluateExit failed: %v", err)
	}

	if result.ShouldExit {
		t.Errorf("Expected no exit, got exit reason: %s", result.ExitReason.String())
	}

	if result.ExitReason != exits.NoExit {
		t.Errorf("Expected NoExit, got %s", result.ExitReason.String())
	}

	expectedPnL := 4.0 // (52000/50000 - 1) * 100 = 4%
	if result.UnrealizedPnL < expectedPnL-0.1 || result.UnrealizedPnL > expectedPnL+0.1 {
		t.Errorf("Expected PnL around %.1f%%, got %.1f%%", expectedPnL, result.UnrealizedPnL)
	}
}

func TestExitEvaluator_HardStop(t *testing.T) {
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	inputs := exits.ExitInputs{
		Symbol:        "BTCUSD",
		EntryPrice:    50000.0,
		CurrentPrice:  47000.0, // Below hard stop
		HardStopPrice: 47500.0, // 5% stop loss
		CurrentTime:   time.Now(),
		EntryTime:     time.Now().Add(-1 * time.Hour),
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("EvaluateExit failed: %v", err)
	}

	if !result.ShouldExit {
		t.Errorf("Expected exit due to hard stop")
	}

	if result.ExitReason != exits.HardStop {
		t.Errorf("Expected HardStop, got %s", result.ExitReason.String())
	}

	if result.UnrealizedPnL > -5.0 {
		t.Errorf("Expected negative PnL due to stop loss, got %.1f%%", result.UnrealizedPnL)
	}
}

func TestExitEvaluator_VenueHealthCut(t *testing.T) {
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	testCases := []struct {
		name            string
		p99LatencyMs    int64
		errorRate       float64
		rejectRate      float64
		expectedExit    bool
		expectedTrigger string
	}{
		{
			name:         "high_latency",
			p99LatencyMs: 2500, // Above 2000ms threshold
			errorRate:    1.0,
			rejectRate:   2.0,
			expectedExit: true,
		},
		{
			name:         "high_error_rate",
			p99LatencyMs: 1500,
			errorRate:    4.0, // Above 3% threshold
			rejectRate:   2.0,
			expectedExit: true,
		},
		{
			name:         "high_reject_rate",
			p99LatencyMs: 1500,
			errorRate:    2.0,
			rejectRate:   6.0, // Above 5% threshold
			expectedExit: true,
		},
		{
			name:         "all_healthy",
			p99LatencyMs: 1500,
			errorRate:    2.0,
			rejectRate:   3.0,
			expectedExit: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := exits.ExitInputs{
				Symbol:            "BTCUSD",
				EntryPrice:        50000.0,
				CurrentPrice:      52000.0,
				VenueP99LatencyMs: tc.p99LatencyMs,
				VenueErrorRate:    tc.errorRate,
				VenueRejectRate:   tc.rejectRate,
				CurrentTime:       time.Now(),
				EntryTime:         time.Now().Add(-1 * time.Hour),
			}

			result, err := evaluator.EvaluateExit(context.Background(), inputs)
			if err != nil {
				t.Fatalf("EvaluateExit failed: %v", err)
			}

			if result.ShouldExit != tc.expectedExit {
				t.Errorf("Expected exit=%v, got %v", tc.expectedExit, result.ShouldExit)
			}

			if tc.expectedExit && result.ExitReason != exits.VenueHealthCut {
				t.Errorf("Expected VenueHealthCut, got %s", result.ExitReason.String())
			}
		})
	}
}

func TestExitEvaluator_TimeLimit(t *testing.T) {
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	inputs := exits.ExitInputs{
		Symbol:       "BTCUSD",
		EntryPrice:   50000.0,
		CurrentPrice: 52000.0,
		EntryTime:    time.Now().Add(-50 * time.Hour), // Over 48 hour limit
		CurrentTime:  time.Now(),
		MaxHoldHours: 48.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("EvaluateExit failed: %v", err)
	}

	if !result.ShouldExit {
		t.Errorf("Expected exit due to time limit")
	}

	if result.ExitReason != exits.TimeLimit {
		t.Errorf("Expected TimeLimit, got %s", result.ExitReason.String())
	}

	if result.HoursHeld < 49.0 {
		t.Errorf("Expected hours held > 49, got %.1f", result.HoursHeld)
	}
}

func TestExitEvaluator_AccelerationReversal(t *testing.T) {
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	inputs := exits.ExitInputs{
		Symbol:            "BTCUSD",
		EntryPrice:        50000.0,
		CurrentPrice:      52000.0,
		EntryTime:         time.Now().Add(-2 * time.Hour),
		CurrentTime:       time.Now(),
		MomentumAccel:     0.1,  // Current acceleration
		EntryAccel:        0.4,  // Entry acceleration - 75% decline
		MaxHoldHours:      48.0, // Within time limit
		VenueP99LatencyMs: 1500, // Venue healthy
		VenueErrorRate:    2.0,
		VenueRejectRate:   3.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("EvaluateExit failed: %v", err)
	}

	if !result.ShouldExit {
		t.Errorf("Expected exit due to acceleration reversal")
	}

	if result.ExitReason != exits.AccelerationReversal {
		t.Errorf("Expected AccelerationReversal, got %s", result.ExitReason.String())
	}
}

func TestExitEvaluator_MomentumFade(t *testing.T) {
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	inputs := exits.ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       52000.0,
		EntryTime:          time.Now().Add(-2 * time.Hour),
		CurrentTime:        time.Now(),
		MomentumScore:      56.0, // Current momentum
		EntryMomentumScore: 80.0, // Entry momentum - 30% decline
		MomentumAccel:      0.3,  // Acceleration still decent
		EntryAccel:         0.4,  // Not significant acceleration decline
		MaxHoldHours:       48.0,
		VenueP99LatencyMs:  1500,
		VenueErrorRate:     2.0,
		VenueRejectRate:    3.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("EvaluateExit failed: %v", err)
	}

	if !result.ShouldExit {
		t.Errorf("Expected exit due to momentum fade")
	}

	if result.ExitReason != exits.MomentumFade {
		t.Errorf("Expected MomentumFade, got %s", result.ExitReason.String())
	}
}

func TestExitEvaluator_TrailingStop(t *testing.T) {
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	inputs := exits.ExitInputs{
		Symbol:          "BTCUSD",
		EntryPrice:      50000.0,
		CurrentPrice:    52000.0, // Current price
		EntryTime:       time.Now().Add(-2 * time.Hour),
		CurrentTime:     time.Now(),
		HighWaterMark:   55000.0, // HWM at 55k
		TrailingStopPct: 5.0,     // 5% trailing stop
		// Stop price = 55000 * 0.95 = 52250
		// Current price 52000 < 52250, so should trigger
		MomentumScore:      75.0, // Good momentum (no fade)
		EntryMomentumScore: 80.0,
		MomentumAccel:      0.3,
		EntryAccel:         0.4,
		MaxHoldHours:       48.0,
		VenueP99LatencyMs:  1500,
		VenueErrorRate:     2.0,
		VenueRejectRate:    3.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("EvaluateExit failed: %v", err)
	}

	if !result.ShouldExit {
		t.Errorf("Expected exit due to trailing stop")
	}

	if result.ExitReason != exits.TrailingStop {
		t.Errorf("Expected TrailingStop, got %s", result.ExitReason.String())
	}
}

func TestExitEvaluator_ProfitTarget(t *testing.T) {
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	// Test profit target 1 hit
	inputs := exits.ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       58000.0, // +16% profit
		EntryTime:          time.Now().Add(-2 * time.Hour),
		CurrentTime:        time.Now(),
		ProfitTarget1:      15.0,    // 15% target
		ProfitTargetPrice1: 57500.0, // 15% target price
		ProfitTarget2:      30.0,    // 30% target
		ProfitTargetPrice2: 65000.0, // 30% target price
		HighWaterMark:      58000.0, // No trailing stop trigger
		TrailingStopPct:    5.0,
		MomentumScore:      75.0, // No momentum fade
		EntryMomentumScore: 80.0,
		MomentumAccel:      0.3,
		EntryAccel:         0.4,
		MaxHoldHours:       48.0,
		VenueP99LatencyMs:  1500,
		VenueErrorRate:     2.0,
		VenueRejectRate:    3.0,
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("EvaluateExit failed: %v", err)
	}

	if !result.ShouldExit {
		t.Errorf("Expected exit due to profit target")
	}

	if result.ExitReason != exits.ProfitTarget {
		t.Errorf("Expected ProfitTarget, got %s", result.ExitReason.String())
	}

	if result.UnrealizedPnL < 15.0 {
		t.Errorf("Expected PnL > 15%%, got %.1f%%", result.UnrealizedPnL)
	}
}

func TestExitEvaluator_ExitPrecedence(t *testing.T) {
	// Test that exit reasons follow proper precedence
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	// Set up inputs that would trigger multiple exit conditions
	inputs := exits.ExitInputs{
		Symbol:             "BTCUSD",
		EntryPrice:         50000.0,
		CurrentPrice:       47000.0,                         // Hard stop + other conditions
		HardStopPrice:      47500.0,                         // Hard stop triggered
		EntryTime:          time.Now().Add(-50 * time.Hour), // Time limit exceeded
		CurrentTime:        time.Now(),
		MaxHoldHours:       48.0,
		VenueP99LatencyMs:  3000, // Venue unhealthy
		VenueErrorRate:     5.0,  // High error rate
		MomentumScore:      40.0, // Momentum faded
		EntryMomentumScore: 80.0,
		ProfitTargetPrice1: 45000.0, // Profit target "hit" (irrelevant when losing)
	}

	result, err := evaluator.EvaluateExit(context.Background(), inputs)
	if err != nil {
		t.Fatalf("EvaluateExit failed: %v", err)
	}

	if !result.ShouldExit {
		t.Errorf("Expected exit due to multiple conditions")
	}

	// Hard stop should take precedence over all other conditions
	if result.ExitReason != exits.HardStop {
		t.Errorf("Expected HardStop (highest precedence), got %s", result.ExitReason.String())
	}
}

func TestExitEvaluator_BoundaryConditions(t *testing.T) {
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	testCases := []struct {
		name           string
		inputs         exits.ExitInputs
		shouldExit     bool
		expectedReason exits.ExitReason
	}{
		{
			name: "exactly_at_stop",
			inputs: exits.ExitInputs{
				Symbol:        "BTCUSD",
				EntryPrice:    50000.0,
				CurrentPrice:  47500.0, // Exactly at stop price
				HardStopPrice: 47500.0,
				CurrentTime:   time.Now(),
				EntryTime:     time.Now().Add(-1 * time.Hour),
			},
			shouldExit:     true,
			expectedReason: exits.HardStop,
		},
		{
			name: "exactly_at_time_limit",
			inputs: exits.ExitInputs{
				Symbol:       "BTCUSD",
				EntryPrice:   50000.0,
				CurrentPrice: 52000.0,
				EntryTime:    time.Now().Add(-48 * time.Hour),
				CurrentTime:  time.Now(),
				MaxHoldHours: 48.0,
			},
			shouldExit:     true,
			expectedReason: exits.TimeLimit,
		},
		{
			name: "momentum_fade_threshold",
			inputs: exits.ExitInputs{
				Symbol:             "BTCUSD",
				EntryPrice:         50000.0,
				CurrentPrice:       52000.0,
				EntryTime:          time.Now().Add(-2 * time.Hour),
				CurrentTime:        time.Now(),
				MomentumScore:      56.0, // Exactly 30% decline
				EntryMomentumScore: 80.0,
				MaxHoldHours:       48.0,
				VenueP99LatencyMs:  1500,
				VenueErrorRate:     2.0,
				VenueRejectRate:    3.0,
			},
			shouldExit:     true,
			expectedReason: exits.MomentumFade,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.EvaluateExit(context.Background(), tc.inputs)
			if err != nil {
				t.Fatalf("EvaluateExit failed: %v", err)
			}

			if result.ShouldExit != tc.shouldExit {
				t.Errorf("Expected shouldExit=%v, got %v", tc.shouldExit, result.ShouldExit)
			}

			if tc.shouldExit && result.ExitReason != tc.expectedReason {
				t.Errorf("Expected exit reason %s, got %s", tc.expectedReason.String(), result.ExitReason.String())
			}
		})
	}
}
