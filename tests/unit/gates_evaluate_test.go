package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sawpanic/cryptorun/internal/domain/gates"
)

func TestEvaluateAllGates_AllPassing(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	
	inputs := gates.EvaluateAllGatesInputs{
		Symbol:        "BTCUSD",
		Timestamp:     now,
		BarsAge:       1,           // ≤ 2 (pass)
		PriceChange:   100.0,       // Price change
		ATR1h:         120.0,       // ATR > price change / 1.2 (pass)
		Momentum24h:   8.0,         // < 12% (pass)
		RSI4h:         65.0,        // < 70 (pass)
		Acceleration:  1.0,         // Not needed when not fatigued
		SignalTime:    now.Add(-20 * time.Second), // 20s ago
		ExecutionTime: now,         // Now (20s delay < 30s, pass)
	}
	
	result, err := gates.EvaluateAllGates(ctx, inputs)
	require.NoError(t, err)
	
	assert.True(t, result.Passed, "All gates should pass")
	assert.Equal(t, "all_gates_passed", result.OverallReason)
	assert.Equal(t, "BTCUSD", result.Symbol)
	assert.Len(t, result.Reasons, 3) // freshness, fatigue, late_fill
	
	// Verify each gate passed
	for _, reason := range result.Reasons {
		assert.True(t, reason.Passed, "Gate %s should pass", reason.Name)
	}
}

func TestEvaluateAllGates_BoundaryConditions_Freshness(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	
	testCases := []struct {
		name        string
		barsAge     int
		priceChange float64
		atr1h       float64
		shouldPass  bool
		expectedMsg string
	}{
		{
			name:        "exactly_2_bars_pass",
			barsAge:     2,
			priceChange: 100.0,
			atr1h:       120.0, // 100/120 = 0.83 < 1.2
			shouldPass:  true,
			expectedMsg: "fresh",
		},
		{
			name:        "3_bars_fail",
			barsAge:     3,
			priceChange: 100.0,
			atr1h:       120.0,
			shouldPass:  false,
			expectedMsg: "stale_bars",
		},
		{
			name:        "exactly_1.2_atr_ratio_pass", 
			barsAge:     1,
			priceChange: 120.0,
			atr1h:       100.0, // 120/100 = 1.2 exactly
			shouldPass:  true,
			expectedMsg: "fresh",
		},
		{
			name:        "1.21_atr_ratio_fail",
			barsAge:     1,
			priceChange: 121.0,
			atr1h:       100.0, // 121/100 = 1.21 > 1.2
			shouldPass:  false,
			expectedMsg: "excessive_move",
		},
		{
			name:        "1.19_atr_ratio_pass",
			barsAge:     1,
			priceChange: 119.0,
			atr1h:       100.0, // 119/100 = 1.19 < 1.2
			shouldPass:  true,
			expectedMsg: "fresh",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := gates.EvaluateAllGatesInputs{
				Symbol:        "BTCUSD",
				Timestamp:     now,
				BarsAge:       tc.barsAge,
				PriceChange:   tc.priceChange,
				ATR1h:         tc.atr1h,
				Momentum24h:   5.0,  // Not fatigued
				RSI4h:         50.0, // Not fatigued
				Acceleration:  0.0,  // Not needed
				SignalTime:    now.Add(-10 * time.Second),
				ExecutionTime: now,
			}
			
			result, err := gates.EvaluateAllGates(ctx, inputs)
			require.NoError(t, err)
			
			// Find freshness gate result
			var freshnessReason *gates.GateReason
			for _, reason := range result.Reasons {
				if reason.Name == "freshness" {
					freshnessReason = &reason
					break
				}
			}
			
			require.NotNil(t, freshnessReason, "Freshness gate should be evaluated")
			assert.Equal(t, tc.shouldPass, freshnessReason.Passed, 
				"Freshness gate pass/fail for %s", tc.name)
			assert.Equal(t, tc.expectedMsg, freshnessReason.Message,
				"Freshness gate message for %s", tc.name)
			
			// Check ATR ratio in metrics
			atrRatio := freshnessReason.Metrics["atr_ratio"]
			expectedRatio := tc.priceChange / tc.atr1h
			assert.InDelta(t, expectedRatio, atrRatio, 0.001, 
				"ATR ratio should be calculated correctly")
		})
	}
}

func TestEvaluateAllGates_BoundaryConditions_Fatigue(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	
	testCases := []struct {
		name         string
		momentum24h  float64
		rsi4h        float64
		acceleration float64
		shouldPass   bool
		expectedMsg  string
	}{
		{
			name:         "11.9_momentum_pass",
			momentum24h:  11.9,
			rsi4h:        75.0,
			acceleration: 0.0,
			shouldPass:   true,
			expectedMsg:  "fatigue_pass",
		},
		{
			name:         "12.1_momentum_rsi_69_pass",
			momentum24h:  12.1,
			rsi4h:        69.0,
			acceleration: 0.0,
			shouldPass:   true,
			expectedMsg:  "fatigue_pass",
		},
		{
			name:         "12.1_momentum_rsi_71_accel_1.9_fail",
			momentum24h:  12.1,
			rsi4h:        71.0,
			acceleration: 1.9,
			shouldPass:   false,
			expectedMsg:  "fatigue_block",
		},
		{
			name:         "12.1_momentum_rsi_71_accel_2.0_pass",
			momentum24h:  12.1,
			rsi4h:        71.0,
			acceleration: 2.0,
			shouldPass:   true,
			expectedMsg:  "fatigue_pass",
		},
		{
			name:         "exactly_12_momentum_rsi_70_fail",
			momentum24h:  12.0,
			rsi4h:        70.0,
			acceleration: 0.0,
			shouldPass:   false,
			expectedMsg:  "fatigue_block",
		},
		{
			name:         "exactly_12_momentum_rsi_70_accel_2.5_pass",
			momentum24h:  12.0,
			rsi4h:        70.0,
			acceleration: 2.5,
			shouldPass:   true,
			expectedMsg:  "fatigue_pass",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := gates.EvaluateAllGatesInputs{
				Symbol:        "ETHUSD",
				Timestamp:     now,
				BarsAge:       1,
				PriceChange:   50.0,
				ATR1h:         100.0,
				Momentum24h:   tc.momentum24h,
				RSI4h:         tc.rsi4h,
				Acceleration:  tc.acceleration,
				SignalTime:    now.Add(-10 * time.Second),
				ExecutionTime: now,
			}
			
			result, err := gates.EvaluateAllGates(ctx, inputs)
			require.NoError(t, err)
			
			// Find fatigue gate result
			var fatigueReason *gates.GateReason
			for _, reason := range result.Reasons {
				if reason.Name == "fatigue" {
					fatigueReason = &reason
					break
				}
			}
			
			require.NotNil(t, fatigueReason, "Fatigue gate should be evaluated")
			assert.Equal(t, tc.shouldPass, fatigueReason.Passed, 
				"Fatigue gate pass/fail for %s", tc.name)
			assert.Equal(t, tc.expectedMsg, fatigueReason.Message,
				"Fatigue gate message for %s", tc.name)
			
			// Verify metrics
			assert.Equal(t, tc.momentum24h, fatigueReason.Metrics["momentum_24h"])
			assert.Equal(t, tc.rsi4h, fatigueReason.Metrics["rsi_4h"])
			assert.Equal(t, tc.acceleration, fatigueReason.Metrics["acceleration"])
		})
	}
}

func TestEvaluateAllGates_BoundaryConditions_LateFill(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	
	testCases := []struct {
		name            string
		delaySeconds    int
		shouldPass      bool
		expectedMsg     string
	}{
		{
			name:         "29_seconds_pass",
			delaySeconds: 29,
			shouldPass:   true,
			expectedMsg:  "timely_fill",
		},
		{
			name:         "30_seconds_pass",
			delaySeconds: 30,
			shouldPass:   true,
			expectedMsg:  "timely_fill",
		},
		{
			name:         "31_seconds_fail",
			delaySeconds: 31,
			shouldPass:   false,
			expectedMsg:  "late_fill",
		},
		{
			name:         "0_seconds_pass",
			delaySeconds: 0,
			shouldPass:   true,
			expectedMsg:  "timely_fill",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			signalTime := now.Add(-time.Duration(tc.delaySeconds) * time.Second)
			
			inputs := gates.EvaluateAllGatesInputs{
				Symbol:        "ADAUSD",
				Timestamp:     now,
				BarsAge:       1,
				PriceChange:   30.0,
				ATR1h:         100.0,
				Momentum24h:   5.0,
				RSI4h:         50.0,
				Acceleration:  0.0,
				SignalTime:    signalTime,
				ExecutionTime: now,
			}
			
			result, err := gates.EvaluateAllGates(ctx, inputs)
			require.NoError(t, err)
			
			// Find late fill gate result
			var lateFillReason *gates.GateReason
			for _, reason := range result.Reasons {
				if reason.Name == "late_fill" {
					lateFillReason = &reason
					break
				}
			}
			
			require.NotNil(t, lateFillReason, "Late fill gate should be evaluated")
			assert.Equal(t, tc.shouldPass, lateFillReason.Passed, 
				"Late fill gate pass/fail for %s", tc.name)
			assert.Equal(t, tc.expectedMsg, lateFillReason.Message,
				"Late fill gate message for %s", tc.name)
			
			// Verify delay metric
			expectedDelay := float64(tc.delaySeconds)
			actualDelay := lateFillReason.Metrics["execution_delay_seconds"]
			assert.InDelta(t, expectedDelay, actualDelay, 0.1,
				"Execution delay should be calculated correctly")
		})
	}
}

func TestEvaluateAllGates_BoundaryConditions_Microstructure(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	
	testCases := []struct {
		name        string
		spreadBps   *float64
		depthUSD    *float64
		vadr        *float64
		shouldPass  bool
		description string
	}{
		{
			name:        "all_microstructure_pass",
			spreadBps:   floatPtr(45.0),
			depthUSD:    floatPtr(150000.0),
			vadr:        floatPtr(2.0),
			shouldPass:  true,
			description: "All microstructure metrics within limits",
		},
		{
			name:        "spread_exactly_50_pass",
			spreadBps:   floatPtr(50.0),
			depthUSD:    floatPtr(100000.0),
			vadr:        floatPtr(1.75),
			shouldPass:  true,
			description: "Spread exactly at limit",
		},
		{
			name:        "spread_50.1_fail",
			spreadBps:   floatPtr(50.1),
			depthUSD:    floatPtr(100000.0),
			vadr:        floatPtr(1.75),
			shouldPass:  false,
			description: "Spread just over limit",
		},
		{
			name:        "depth_exactly_100k_pass",
			spreadBps:   floatPtr(40.0),
			depthUSD:    floatPtr(100000.0),
			vadr:        floatPtr(1.75),
			shouldPass:  true,
			description: "Depth exactly at limit",
		},
		{
			name:        "depth_99999_fail",
			spreadBps:   floatPtr(40.0),
			depthUSD:    floatPtr(99999.0),
			vadr:        floatPtr(1.75),
			shouldPass:  false,
			description: "Depth just under limit",
		},
		{
			name:        "vadr_exactly_1.75_pass",
			spreadBps:   floatPtr(40.0),
			depthUSD:    floatPtr(120000.0),
			vadr:        floatPtr(1.75),
			shouldPass:  true,
			description: "VADR exactly at limit",
		},
		{
			name:        "vadr_1.74_fail",
			spreadBps:   floatPtr(40.0),
			depthUSD:    floatPtr(120000.0),
			vadr:        floatPtr(1.74),
			shouldPass:  false,
			description: "VADR just under limit",
		},
		{
			name:        "no_microstructure_data",
			spreadBps:   nil,
			depthUSD:    nil,
			vadr:        nil,
			shouldPass:  true,
			description: "No microstructure gate when data unavailable",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := gates.EvaluateAllGatesInputs{
				Symbol:        "SOLUSD",
				Timestamp:     now,
				BarsAge:       1,
				PriceChange:   50.0,
				ATR1h:         100.0,
				Momentum24h:   8.0,
				RSI4h:         60.0,
				Acceleration:  0.0,
				SignalTime:    now.Add(-15 * time.Second),
				ExecutionTime: now,
				Spread:        tc.spreadBps,
				Depth:         tc.depthUSD,
				VADR:          tc.vadr,
			}
			
			result, err := gates.EvaluateAllGates(ctx, inputs)
			require.NoError(t, err)
			
			// Find microstructure gate result (if it should exist)
			var microReason *gates.GateReason
			for _, reason := range result.Reasons {
				if reason.Name == "microstructure" {
					microReason = &reason
					break
				}
			}
			
			if tc.spreadBps == nil && tc.depthUSD == nil && tc.vadr == nil {
				// No microstructure data - gate should not exist
				assert.Nil(t, microReason, "Microstructure gate should not exist when data unavailable")
				assert.True(t, result.Passed, "Should pass when no microstructure requirements")
			} else {
				// Microstructure data provided - gate should exist
				require.NotNil(t, microReason, "Microstructure gate should be evaluated")
				assert.Equal(t, tc.shouldPass, microReason.Passed, 
					"Microstructure gate pass/fail for %s", tc.name)
				
				// Verify metrics are populated
				if tc.spreadBps != nil {
					assert.Equal(t, *tc.spreadBps, microReason.Metrics["spread_bps"])
				}
				if tc.depthUSD != nil {
					assert.Equal(t, *tc.depthUSD, microReason.Metrics["depth_usd"])
				}
				if tc.vadr != nil {
					assert.Equal(t, *tc.vadr, microReason.Metrics["vadr"])
				}
			}
		})
	}
}

func TestEvaluateAllGates_MultipleFailures_ShortCircuit(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	
	// Create inputs that fail multiple gates
	inputs := gates.EvaluateAllGatesInputs{
		Symbol:        "DOGEUSD",
		Timestamp:     now,
		BarsAge:       5,           // Freshness fail: > 2 bars
		PriceChange:   150.0,       // Freshness fail: > 1.2 * ATR
		ATR1h:         100.0,       
		Momentum24h:   15.0,        // Fatigue fail: > 12%
		RSI4h:         75.0,        // Fatigue fail: > 70
		Acceleration:  1.0,         // Fatigue fail: accel < 2% when fatigued
		SignalTime:    now.Add(-45 * time.Second), // Late fill fail: > 30s
		ExecutionTime: now,
		Spread:        floatPtr(60.0), // Microstructure fail: > 50 bps
		Depth:         floatPtr(50000.0), // Microstructure fail: < 100k
		VADR:          floatPtr(1.5),  // Microstructure fail: < 1.75
	}
	
	result, err := gates.EvaluateAllGates(ctx, inputs)
	require.NoError(t, err)
	
	// Should fail overall
	assert.False(t, result.Passed, "Should fail with multiple gate failures")
	
	// Should still evaluate all gates (no short-circuit for transparency)
	assert.Len(t, result.Reasons, 4, "All gates should be evaluated: freshness, fatigue, late_fill, microstructure")
	
	// All gates should fail
	for _, reason := range result.Reasons {
		assert.False(t, reason.Passed, "Gate %s should fail", reason.Name)
	}
	
	// Overall reason should be the first failure (freshness)
	assert.Contains(t, result.OverallReason, "blocked_by_freshness", 
		"Overall reason should indicate first failing gate")
}

func TestFormatGateExplanation(t *testing.T) {
	now := time.Now()
	
	// Create a result with mixed pass/fail
	result := &gates.EvaluateAllGatesResult{
		Passed:        false,
		OverallReason: "blocked_by_fatigue: fatigue_block",
		Symbol:        "BTCUSD",
		Timestamp:     now,
		Reasons: []gates.GateReason{
			{
				Name:    "freshness",
				Passed:  true,
				Message: "fresh",
				Metrics: map[string]float64{
					"bars_age":     1.0,
					"atr_ratio":    0.8,
					"price_change": 80.0,
					"atr_1h":       100.0,
				},
			},
			{
				Name:    "fatigue",
				Passed:  false,
				Message: "fatigue_block",
				Metrics: map[string]float64{
					"momentum_24h": 15.0,
					"rsi_4h":       75.0,
					"acceleration": 1.5,
				},
			},
		},
	}
	
	explanation := gates.FormatGateExplanation(result)
	
	// Should include symbol and overall status
	assert.Contains(t, explanation, "BTCUSD")
	assert.Contains(t, explanation, "ENTRY BLOCKED")
	assert.Contains(t, explanation, "blocked_by_fatigue")
	
	// Should show gate details
	assert.Contains(t, explanation, "✅ freshness: fresh")
	assert.Contains(t, explanation, "❌ fatigue: fatigue_block")
	
	// Should include key metrics
	assert.Contains(t, explanation, "0.80x ATR")
	assert.Contains(t, explanation, "15.0%")
	assert.Contains(t, explanation, "75.0")
	assert.Contains(t, explanation, "1.5%")
	
	// Should include timestamp
	assert.Contains(t, explanation, now.Format("2006-01-02 15:04:05"))
}

// Helper function to create float64 pointers
func floatPtr(f float64) *float64 {
	return &f
}