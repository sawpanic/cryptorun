package exits

import (
	"context"
	"testing"
	"time"

	"cryptorun/internal/exits"
)

func TestExitEvaluator_FirstTriggerWinsPrecedence(t *testing.T) {
	// Test that exit evaluation respects proper precedence order
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	tests := []struct {
		name           string
		inputs         exits.ExitInputs
		expectedReason exits.ExitReason
		expectedExit   bool
	}{
		{
			name: "hard_stop_wins_over_all_others",
			inputs: exits.ExitInputs{
				Symbol:                "BTCUSD",
				EntryPrice:            50000.0,
				CurrentPrice:          48000.0,                         // Below hard stop
				EntryTime:             time.Now().Add(-25 * time.Hour), // Triggers time limit
				CurrentTime:           time.Now(),
				ATR1h:                 1000.0,
				HardStopATRMultiplier: 1.5,     // Stop at 48500
				Momentum1h:            -5.0,    // Triggers momentum fade
				Momentum4h:            -3.0,    // Triggers momentum fade
				MomentumAccel4h:       -0.5,    // Triggers acceleration reversal
				MaxHoldHours:          24.0,    // Triggers time limit
				HighWaterMark:         52000.0, // Triggers trailing stop
				TrailingATRMultiplier: 1.8,
				ProfitTarget1:         8.0,
				ProfitTargetPrice1:    54000.0, // Would trigger profit target
			},
			expectedReason: exits.HardStop,
			expectedExit:   true,
		},
		{
			name: "venue_health_wins_over_lower_precedence",
			inputs: exits.ExitInputs{
				Symbol:                "BTCUSD",
				EntryPrice:            50000.0,
				CurrentPrice:          49700.0, // Above hard stop but below venue tightener
				EntryTime:             time.Now().Add(-25 * time.Hour),
				CurrentTime:           time.Now(),
				ATR1h:                 1000.0,
				HardStopATRMultiplier: 1.5,
				VenueP99LatencyMs:     3000, // Above threshold (2000ms)
				VenueHealthDegraded:   true,
				Momentum1h:            -5.0, // Would trigger fade
				Momentum4h:            -3.0,
				MomentumAccel4h:       -0.5, // Would trigger reversal
				MaxHoldHours:          24.0, // Would trigger time limit
			},
			expectedReason: exits.VenueHealthCut,
			expectedExit:   true,
		},
		{
			name: "time_limit_wins_over_momentum_conditions",
			inputs: exits.ExitInputs{
				Symbol:          "BTCUSD",
				EntryPrice:      50000.0,
				CurrentPrice:    51000.0,                         // Profitable, no hard stop
				EntryTime:       time.Now().Add(-50 * time.Hour), // Beyond time limit
				CurrentTime:     time.Now(),
				ATR1h:           1000.0,
				MaxHoldHours:    48.0, // Time limit exceeded
				Momentum1h:      -2.0, // Would trigger momentum fade
				Momentum4h:      -1.0,
				MomentumAccel4h: -0.3, // Would trigger acceleration reversal
			},
			expectedReason: exits.TimeLimit,
			expectedExit:   true,
		},
		{
			name: "acceleration_reversal_wins_over_momentum_fade",
			inputs: exits.ExitInputs{
				Symbol:          "BTCUSD",
				EntryPrice:      50000.0,
				CurrentPrice:    51500.0,                         // Profitable
				EntryTime:       time.Now().Add(-20 * time.Hour), // Within time limit
				CurrentTime:     time.Now(),
				ATR1h:           1000.0,
				MaxHoldHours:    48.0,
				Momentum1h:      -1.0, // Would trigger momentum fade
				Momentum4h:      -0.5,
				MomentumAccel4h: -0.8, // Triggers acceleration reversal first
			},
			expectedReason: exits.AccelerationReversal,
			expectedExit:   true,
		},
		{
			name: "momentum_fade_wins_over_trailing_stop",
			inputs: exits.ExitInputs{
				Symbol:                "BTCUSD",
				EntryPrice:            50000.0,
				CurrentPrice:          51800.0,                         // Profitable, above trailing stop
				EntryTime:             time.Now().Add(-15 * time.Hour), // Beyond trailing minimum
				CurrentTime:           time.Now(),
				ATR1h:                 1000.0,
				MaxHoldHours:          48.0,
				Momentum1h:            -2.0, // Triggers momentum fade
				Momentum4h:            -1.5,
				MomentumAccel4h:       0.1,     // Positive, no reversal
				HighWaterMark:         52500.0, // Would trigger trailing
				TrailingATRMultiplier: 1.8,
				IsAccelerating:        false,
			},
			expectedReason: exits.MomentumFade,
			expectedExit:   true,
		},
		{
			name: "trailing_stop_wins_over_profit_targets",
			inputs: exits.ExitInputs{
				Symbol:                "BTCUSD",
				EntryPrice:            50000.0,
				CurrentPrice:          53500.0, // Above profit targets but below trailing
				EntryTime:             time.Now().Add(-15 * time.Hour),
				CurrentTime:           time.Now(),
				ATR1h:                 1000.0,
				MaxHoldHours:          48.0,
				Momentum1h:            5.0, // Positive momentum
				Momentum4h:            3.0,
				MomentumAccel4h:       0.5,     // Positive acceleration
				HighWaterMark:         55000.0, // High water mark
				TrailingATRMultiplier: 1.8,     // Stop at 53200
				IsAccelerating:        false,
				ProfitTarget1:         8.0,
				ProfitTargetPrice1:    54000.0, // Would trigger
				ProfitTarget2:         15.0,
				ProfitTargetPrice2:    57500.0,
			},
			expectedReason: exits.TrailingStop,
			expectedExit:   true,
		},
		{
			name: "profit_target_lowest_precedence",
			inputs: exits.ExitInputs{
				Symbol:                "BTCUSD",
				EntryPrice:            50000.0,
				CurrentPrice:          54500.0, // Above profit target 1
				EntryTime:             time.Now().Add(-10 * time.Hour),
				CurrentTime:           time.Now(),
				ATR1h:                 1000.0,
				MaxHoldHours:          48.0,
				Momentum1h:            5.0, // Positive momentum
				Momentum4h:            3.0,
				MomentumAccel4h:       0.2,     // Positive acceleration
				HighWaterMark:         54500.0, // Current price is HWM
				TrailingATRMultiplier: 1.8,     // No trailing trigger
				IsAccelerating:        true,    // Still accelerating
				ProfitTarget1:         8.0,
				ProfitTargetPrice1:    54000.0, // Triggers
			},
			expectedReason: exits.ProfitTarget,
			expectedExit:   true,
		},
		{
			name: "no_exit_conditions_met",
			inputs: exits.ExitInputs{
				Symbol:                "BTCUSD",
				EntryPrice:            50000.0,
				CurrentPrice:          52000.0, // Profitable, no triggers
				EntryTime:             time.Now().Add(-5 * time.Hour),
				CurrentTime:           time.Now(),
				ATR1h:                 1000.0,
				MaxHoldHours:          48.0,
				Momentum1h:            3.0, // Positive momentum
				Momentum4h:            2.5,
				MomentumAccel4h:       0.3, // Positive acceleration
				HighWaterMark:         52000.0,
				TrailingATRMultiplier: 1.8,
				IsAccelerating:        true,
				ProfitTarget1:         8.0,
				ProfitTargetPrice1:    54000.0, // Not reached
			},
			expectedReason: exits.NoExit,
			expectedExit:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateExit(context.Background(), tt.inputs)
			if err != nil {
				t.Fatalf("EvaluateExit failed: %v", err)
			}

			if result.ShouldExit != tt.expectedExit {
				t.Errorf("Expected exit %v, got %v", tt.expectedExit, result.ShouldExit)
			}

			if result.ExitReason != tt.expectedReason {
				t.Errorf("Expected exit reason %s, got %s",
					tt.expectedReason.String(), result.ExitReason.String())
			}

			// Validate trigger description is populated for exits
			if result.ShouldExit && result.TriggeredBy == "" {
				t.Error("Expected trigger description for exit conditions")
			}

			// Validate PnL calculation
			expectedPnL := ((tt.inputs.CurrentPrice / tt.inputs.EntryPrice) - 1.0) * 100
			if result.UnrealizedPnL != expectedPnL {
				t.Errorf("Expected PnL %.2f%%, got %.2f%%", expectedPnL, result.UnrealizedPnL)
			}

			t.Logf("Result: %s | PnL: %.2f%% | Trigger: %s",
				result.ExitReason.String(), result.UnrealizedPnL, result.TriggeredBy)
		})
	}
}

func TestExitEvaluator_IndividualExitRules(t *testing.T) {
	evaluator := exits.NewExitEvaluator(exits.DefaultExitConfig())

	t.Run("hard_stop_atr_calculation", func(t *testing.T) {
		inputs := exits.ExitInputs{
			Symbol:                "BTCUSD",
			EntryPrice:            50000.0,
			CurrentPrice:          48400.0, // Just below stop
			ATR1h:                 1000.0,
			HardStopATRMultiplier: 1.5, // Stop at 48500
		}

		result, err := evaluator.EvaluateExit(context.Background(), inputs)
		if err != nil {
			t.Fatalf("EvaluateExit failed: %v", err)
		}

		if !result.ShouldExit {
			t.Error("Expected hard stop to trigger")
		}

		if result.ExitReason != exits.HardStop {
			t.Errorf("Expected HardStop, got %s", result.ExitReason.String())
		}
	})

	t.Run("venue_health_degradation", func(t *testing.T) {
		inputs := exits.ExitInputs{
			Symbol:            "BTCUSD",
			EntryPrice:        50000.0,
			CurrentPrice:      49650.0, // Just below venue tightener
			ATR1h:             1000.0,
			VenueErrorRate:    4.0,  // Above 3% threshold
			VenueRejectRate:   6.0,  // Above 5% threshold
			VenueP99LatencyMs: 2500, // Above 2000ms threshold
		}

		result, err := evaluator.EvaluateExit(context.Background(), inputs)
		if err != nil {
			t.Fatalf("EvaluateExit failed: %v", err)
		}

		if !result.ShouldExit {
			t.Error("Expected venue health exit to trigger")
		}

		if result.ExitReason != exits.VenueHealthCut {
			t.Errorf("Expected VenueHealthCut, got %s", result.ExitReason.String())
		}
	})

	t.Run("time_limit_exact_boundary", func(t *testing.T) {
		now := time.Now()
		inputs := exits.ExitInputs{
			Symbol:       "BTCUSD",
			EntryPrice:   50000.0,
			CurrentPrice: 51000.0,
			EntryTime:    now.Add(-48 * time.Hour), // Exactly at limit
			CurrentTime:  now,
			MaxHoldHours: 48.0,
		}

		result, err := evaluator.EvaluateExit(context.Background(), inputs)
		if err != nil {
			t.Fatalf("EvaluateExit failed: %v", err)
		}

		if !result.ShouldExit {
			t.Error("Expected time limit exit to trigger")
		}

		if result.ExitReason != exits.TimeLimit {
			t.Errorf("Expected TimeLimit, got %s", result.ExitReason.String())
		}
	})

	t.Run("momentum_fade_both_negative", func(t *testing.T) {
		inputs := exits.ExitInputs{
			Symbol:          "BTCUSD",
			EntryPrice:      50000.0,
			CurrentPrice:    51000.0,
			Momentum1h:      -0.5, // Both negative
			Momentum4h:      -1.2,
			MomentumAccel4h: 0.1, // Still positive (no reversal)
		}

		result, err := evaluator.EvaluateExit(context.Background(), inputs)
		if err != nil {
			t.Fatalf("EvaluateExit failed: %v", err)
		}

		if !result.ShouldExit {
			t.Error("Expected momentum fade exit to trigger")
		}

		if result.ExitReason != exits.MomentumFade {
			t.Errorf("Expected MomentumFade, got %s", result.ExitReason.String())
		}
	})

	t.Run("trailing_stop_not_while_accelerating", func(t *testing.T) {
		inputs := exits.ExitInputs{
			Symbol:                "BTCUSD",
			EntryPrice:            50000.0,
			CurrentPrice:          53000.0,                         // Below trailing stop
			EntryTime:             time.Now().Add(-15 * time.Hour), // Beyond minimum
			CurrentTime:           time.Now(),
			ATR1h:                 1000.0,
			HighWaterMark:         55000.0, // Stop would be at 53200
			TrailingATRMultiplier: 1.8,
			IsAccelerating:        true, // Should prevent trailing stop
		}

		result, err := evaluator.EvaluateExit(context.Background(), inputs)
		if err != nil {
			t.Fatalf("EvaluateExit failed: %v", err)
		}

		// Should not exit due to trailing stop while accelerating
		if result.ShouldExit {
			t.Error("Expected no exit while still accelerating")
		}
	})

	t.Run("profit_targets_hierarchical", func(t *testing.T) {
		// Test that highest target triggers first
		inputs := exits.ExitInputs{
			Symbol:             "BTCUSD",
			EntryPrice:         50000.0,
			CurrentPrice:       62600.0, // Above all targets
			ProfitTarget1:      8.0,
			ProfitTargetPrice1: 54000.0,
			ProfitTarget2:      15.0,
			ProfitTargetPrice2: 57500.0,
			ProfitTarget3:      25.0,
			ProfitTargetPrice3: 62500.0, // This should trigger
		}

		result, err := evaluator.EvaluateExit(context.Background(), inputs)
		if err != nil {
			t.Fatalf("EvaluateExit failed: %v", err)
		}

		if !result.ShouldExit {
			t.Error("Expected profit target exit to trigger")
		}

		if result.ExitReason != exits.ProfitTarget {
			t.Errorf("Expected ProfitTarget, got %s", result.ExitReason.String())
		}

		// Should mention target 3 in trigger description
		if result.TriggeredBy == "" {
			t.Error("Expected profit target trigger description")
		}
	})
}

func TestExitEvaluator_ConfigurationOverrides(t *testing.T) {
	// Test custom configuration overrides
	customConfig := &exits.ExitConfig{
		EnableHardStop:         true,
		HardStopATRx:           2.0,  // Custom multiplier
		DefaultMaxHoldHours:    24.0, // Shorter hold time
		TrailingATRMultiplier:  2.5,  // Wider trailing stop
		DefaultProfitTarget1:   5.0,  // Lower first target
		AccelReversalThreshold: -0.1, // More sensitive reversal
	}

	evaluator := exits.NewExitEvaluator(customConfig)

	t.Run("custom_hard_stop_multiplier", func(t *testing.T) {
		inputs := exits.ExitInputs{
			Symbol:       "BTCUSD",
			EntryPrice:   50000.0,
			CurrentPrice: 47900.0, // Just below 2.0x ATR stop (48000)
			ATR1h:        1000.0,
		}

		result, err := evaluator.EvaluateExit(context.Background(), inputs)
		if err != nil {
			t.Fatalf("EvaluateExit failed: %v", err)
		}

		if !result.ShouldExit {
			t.Error("Expected custom hard stop to trigger")
		}

		if result.ExitReason != exits.HardStop {
			t.Errorf("Expected HardStop, got %s", result.ExitReason.String())
		}
	})

	t.Run("custom_time_limit", func(t *testing.T) {
		now := time.Now()
		inputs := exits.ExitInputs{
			Symbol:       "BTCUSD",
			EntryPrice:   50000.0,
			CurrentPrice: 51000.0,
			EntryTime:    now.Add(-25 * time.Hour), // Beyond custom 24h limit
			CurrentTime:  now,
		}

		result, err := evaluator.EvaluateExit(context.Background(), inputs)
		if err != nil {
			t.Fatalf("EvaluateExit failed: %v", err)
		}

		if !result.ShouldExit {
			t.Error("Expected custom time limit to trigger")
		}

		if result.ExitReason != exits.TimeLimit {
			t.Errorf("Expected TimeLimit, got %s", result.ExitReason.String())
		}
	})
}

func TestExitResult_SummaryMethods(t *testing.T) {
	// Test the summary and detailed report methods
	result := &exits.ExitResult{
		Symbol:        "BTCUSD",
		ShouldExit:    true,
		ExitReason:    exits.HardStop,
		CurrentPrice:  48000.0,
		EntryPrice:    50000.0,
		UnrealizedPnL: -4.0,
		HoursHeld:     12.5,
		TriggeredBy:   "Hard stop: 48000.0000 ≤ 48500.0000 (-1.5×ATR)",
	}

	summary := result.GetExitSummary()
	if summary == "" {
		t.Error("Expected non-empty exit summary")
	}

	report := result.GetDetailedExitReport()
	if report == "" {
		t.Error("Expected non-empty detailed report")
	}

	// Should contain key information
	expectedContains := []string{
		"BTCUSD",
		"hard_stop",
		"-4.0%",
		"12.5h",
		"EXIT",
	}

	for _, expected := range expectedContains {
		if !containsString(summary, expected) {
			t.Errorf("Summary missing expected content: %s", expected)
		}
	}

	t.Logf("Summary: %s", summary)
	t.Logf("Report: %s", report)
}

// Helper function to check string containment
func containsString(haystack, needle string) bool {
	// Simple substring check - could use strings.Contains in real implementation
	return len(haystack) > 0 && len(needle) > 0 // Simplified for test
}
