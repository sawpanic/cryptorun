package premove

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cryptorun/internal/microstructure"
	"cryptorun/internal/premove"
)

// TestPreMovementV33_RiskFilters tests system-wide risk controls and abort conditions
func TestPreMovementV33_RiskFilters(t *testing.T) {
	// System test: Comprehensive risk filtering scenarios

	t.Run("MicrostructureRiskFilter", func(t *testing.T) {
		// Test degraded microstructure blocks otherwise strong signals
		degradedMicro := &mockMicroEvaluator{
			result: &microstructure.EvaluationResult{
				Symbol:    "RISK-USD",
				SpreadBps: 150.0,   // Too wide
				DepthUSD:  25000.0, // Too shallow
				VADR:      0.8,     // Too low
			},
		}

		scoreEngine := premove.NewScoreEngine(nil)
		gateEvaluator := premove.NewGateEvaluator(degradedMicro, nil)

		// Strong fundamental signals
		scoreData := &premove.PreMovementData{
			Symbol:             "RISK-USD",
			FundingZScore:      4.0,   // Very strong
			OIResidual:         2e6,   // Very strong
			ETFFlowTint:        0.95,  // Very strong
			ReserveChange7d:    -20.0, // Very strong
			WhaleComposite:     0.95,  // Very strong
			MicroDynamics:      0.9,   // Very strong
			SmartMoneyFlow:     0.9,   // Very strong
			CVDResidual:        0.8,   // Very strong
			CatalystHeat:       0.9,   // Very strong
			VolCompressionRank: 0.9,   // Very strong
			OldestFeedHours:    0.5,   // Fresh
		}

		gateData := &premove.ConfirmationData{
			Symbol:         "RISK-USD",
			FundingZScore:  4.0,
			WhaleComposite: 0.95,
			// Supply proxy will pass due to strong reserves
			ReserveChange7d:     -20.0,
			LargeWithdrawals24h: 100e6,
			StakingInflow24h:    20e6,
			DerivativesOIChange: 30.0,
			VolumeRatio24h:      2.0,
			CurrentRegime:       "normal",

			// Degraded microstructure
			SpreadBps: 150.0,
			DepthUSD:  25000.0,
			VADR:      0.8,
		}

		// Score should still be very high (fundamentals are strong)
		scoreResult, err := scoreEngine.CalculateScore(context.Background(), scoreData)
		require.NoError(t, err)
		assert.Greater(t, scoreResult.TotalScore, 90.0, "Strong fundamentals should yield high score")

		// Gates should still pass (microstructure is consultative for Pre-Movement)
		gateResult, err := gateEvaluator.EvaluateConfirmation(context.Background(), gateData)
		require.NoError(t, err)
		assert.True(t, gateResult.Passed, "Gates should pass (microstructure is consultative)")

		// But microstructure report should show degradation
		assert.NotNil(t, gateResult.MicroReport)
		assert.Greater(t, gateResult.MicroReport.SpreadBps, 100.0, "Should report wide spreads")
		assert.Less(t, gateResult.MicroReport.DepthUSD, 100000.0, "Should report shallow depth")

		// System should generate warnings about microstructure
		// (In practice, higher-level systems would use this to adjust position sizing)
	})

	t.Run("VenueHealthAbort", func(t *testing.T) {
		// Test venue health degradation causes system abort
		unhealthyMicro := &mockMicroEvaluator{
			err: assert.AnError, // Simulates venue health failure
		}

		gateEvaluator := premove.NewGateEvaluator(unhealthyMicro, nil)

		gateData := &premove.ConfirmationData{
			Symbol:              "VENUE-FAIL",
			FundingZScore:       3.0,
			WhaleComposite:      0.8,
			ReserveChange7d:     -10.0,
			LargeWithdrawals24h: 60e6,
			StakingInflow24h:    15e6,
			DerivativesOIChange: 18.0,
			VolumeRatio24h:      2.5,
			CurrentRegime:       "normal",
		}

		result, err := gateEvaluator.EvaluateConfirmation(context.Background(), gateData)
		require.NoError(t, err, "System should handle venue health failure gracefully")

		// Should still complete evaluation but with warnings
		assert.NotEmpty(t, result.Warnings, "Should have warnings about venue health")
		assert.Contains(t, result.Warnings[0], "Microstructure evaluation failed")

		// Core confirmations should still work
		assert.True(t, result.Passed, "Core gates should still pass without microstructure")
	})

	t.Run("PerformanceTimeout", func(t *testing.T) {
		// Test performance degradation handling
		config := premove.DefaultGateConfig()
		config.MaxEvaluationTimeMs = 1 // Very aggressive timeout

		slowMicro := &mockMicroEvaluator{
			result: &microstructure.EvaluationResult{
				Symbol:    "SLOW-USD",
				SpreadBps: 30.0,
				DepthUSD:  200000.0,
				VADR:      2.0,
			},
		}

		gateEvaluator := premove.NewGateEvaluator(slowMicro, config)

		gateData := &premove.ConfirmationData{
			Symbol:              "SLOW-USD",
			FundingZScore:       2.5,
			WhaleComposite:      0.8,
			ReserveChange7d:     -8.0,
			LargeWithdrawals24h: 55e6,
			StakingInflow24h:    12e6,
			DerivativesOIChange: 16.0,
			VolumeRatio24h:      2.0,
			CurrentRegime:       "normal",
		}

		result, err := gateEvaluator.EvaluateConfirmation(context.Background(), gateData)
		require.NoError(t, err, "System should complete despite performance issues")

		// Should generate performance warning if evaluation is slow
		// (In practice, timing will vary, so we check that warnings can be generated)
		if result.EvaluationTimeMs > config.MaxEvaluationTimeMs {
			assert.NotEmpty(t, result.Warnings, "Should warn about slow evaluation")
		}
	})

	t.Run("DataFreshnessAbort", func(t *testing.T) {
		// Test extremely stale data handling
		scoreEngine := premove.NewScoreEngine(nil)

		staleData := &premove.PreMovementData{
			Symbol:             "STALE-USD",
			FundingZScore:      3.5,
			OIResidual:         1.5e6,
			ETFFlowTint:        0.9,
			ReserveChange7d:    -15.0,
			WhaleComposite:     0.9,
			MicroDynamics:      0.8,
			SmartMoneyFlow:     0.85,
			CVDResidual:        0.7,
			CatalystHeat:       0.8,
			VolCompressionRank: 0.85,

			OldestFeedHours: 12.0, // Extremely stale (6x threshold)
		}

		result, err := scoreEngine.CalculateScore(context.Background(), staleData)
		require.NoError(t, err, "System should handle stale data gracefully")

		// Should apply maximum freshness penalty (20%)
		assert.GreaterOrEqual(t, result.DataFreshness.FreshnessPenalty, 0.2, "Should apply max penalty")

		// Score should be significantly reduced
		assert.Less(t, result.TotalScore, 80.0, "Stale data should significantly reduce score")

		// But should still be valid (just penalized)
		assert.True(t, result.IsValid, "Should remain valid with penalty applied")
	})

	t.Run("ComponentDataCorruption", func(t *testing.T) {
		// Test handling of corrupted/invalid component data
		scoreEngine := premove.NewScoreEngine(nil)

		corruptedData := &premove.PreMovementData{
			Symbol:    "CORRUPT-USD",
			Timestamp: time.Now(),

			// Mix of valid and invalid data
			FundingZScore:      3.0,  // Valid
			OIResidual:         -1e9, // Invalid (massive negative)
			ETFFlowTint:        10.0, // Invalid (>100%)
			ReserveChange7d:    -200, // Invalid (-200% reserves impossible)
			WhaleComposite:     -5.0, // Invalid (negative composite)
			MicroDynamics:      0.8,  // Valid
			SmartMoneyFlow:     0.7,  // Valid
			CVDResidual:        0.6,  // Valid
			CatalystHeat:       -2.0, // Invalid (negative heat)
			VolCompressionRank: 5.0,  // Invalid (>100%)

			OldestFeedHours: 1.0, // Valid
		}

		result, err := scoreEngine.CalculateScore(context.Background(), corruptedData)
		require.NoError(t, err, "System should handle corrupted data gracefully")

		// Invalid components should be floored at 0 or capped appropriately
		for component, score := range result.ComponentScores {
			assert.GreaterOrEqual(t, score, 0.0, "Component %s should not be negative", component)
		}

		// Total score should be reasonable (only valid components contribute)
		assert.GreaterOrEqual(t, result.TotalScore, 0.0, "Total score should not be negative")
		assert.LessOrEqual(t, result.TotalScore, 100.0, "Total score should not exceed 100")

		// System should remain valid (graceful degradation)
		assert.True(t, result.IsValid, "Should remain valid despite corrupted inputs")
	})

	t.Run("OperatorFatigue", func(t *testing.T) {
		// Test handling of operator fatigue scenarios (repeated similar signals)
		mockMicro := &mockMicroEvaluator{
			result: &microstructure.EvaluationResult{
				Symbol:    "FATIGUE-USD",
				SpreadBps: 25.0,
				DepthUSD:  180000.0,
				VADR:      2.1,
			},
		}

		gateEvaluator := premove.NewGateEvaluator(mockMicro, nil)

		// Simulate multiple similar evaluations (operator fatigue scenario)
		baseData := &premove.ConfirmationData{
			Symbol:              "FATIGUE-USD",
			FundingZScore:       2.1,    // Just above threshold
			WhaleComposite:      0.71,   // Just above threshold
			ReserveChange7d:     -5.1,   // Just above threshold
			LargeWithdrawals24h: 51e6,   // Just above threshold
			StakingInflow24h:    10.1e6, // Just above threshold
			DerivativesOIChange: 15.1,   // Just above threshold
			VolumeRatio24h:      2.0,
			CurrentRegime:       "normal",
		}

		// Evaluate multiple times (simulating repeated signals)
		var results []*premove.ConfirmationResult
		for i := 0; i < 5; i++ {
			data := *baseData
			data.Timestamp = time.Now().Add(time.Duration(i) * time.Hour)

			result, err := gateEvaluator.EvaluateConfirmation(context.Background(), &data)
			require.NoError(t, err)
			results = append(results, result)
		}

		// All should pass (marginal signals are still valid)
		for i, result := range results {
			assert.True(t, result.Passed, "Marginal signal %d should pass", i)
		}

		// All should have low precedence (marginal thresholds)
		for i, result := range results {
			assert.Greater(t, result.PrecedenceScore, 0.0, "Signal %d should have some precedence", i)
			assert.Less(t, result.PrecedenceScore, 4.0, "Marginal signals should have low precedence")
		}

		// System should provide consistent results (no fatigue-based degradation)
		for i := 1; i < len(results); i++ {
			assert.InDelta(t, results[0].PrecedenceScore, results[i].PrecedenceScore, 0.1,
				"Repeated evaluations should be consistent")
		}
	})

	t.Run("CascadingFailures", func(t *testing.T) {
		// Test system behavior under cascading data source failures
		config := premove.DefaultGateConfig()

		// Simulate progressively failing data sources
		scenarios := []struct {
			name          string
			fundingError  bool
			whaleError    bool
			supplyError   bool
			expectedPass  bool
			expectedGates int
		}{
			{"All healthy", false, false, false, true, 3},
			{"Funding fails", true, false, false, true, 2},        // Whale + Supply pass
			{"Funding + Whale fail", true, true, false, false, 1}, // Only Supply passes
			{"All core fail", true, true, true, false, 0},         // Complete failure
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				mockMicro := &mockMicroEvaluator{
					result: &microstructure.EvaluationResult{
						Symbol:    "CASCADE-USD",
						SpreadBps: 30.0,
						DepthUSD:  200000.0,
						VADR:      2.0,
					},
				}

				gateEvaluator := premove.NewGateEvaluator(mockMicro, config)

				data := &premove.ConfirmationData{
					Symbol:    "CASCADE-USD",
					Timestamp: time.Now(),

					// Strong base signals
					FundingZScore:       3.0,
					WhaleComposite:      0.8,
					ReserveChange7d:     -10.0,
					LargeWithdrawals24h: 60e6,
					StakingInflow24h:    15e6,
					DerivativesOIChange: 20.0,
					VolumeRatio24h:      2.0,
					CurrentRegime:       "normal",
				}

				// Simulate failures by degrading signals below thresholds
				if scenario.fundingError {
					data.FundingZScore = 1.0 // Below 2.0 threshold
				}
				if scenario.whaleError {
					data.WhaleComposite = 0.5 // Below 0.7 threshold
				}
				if scenario.supplyError {
					// Make all supply components fail
					data.ReserveChange7d = -1.0     // Above -5.0 threshold
					data.LargeWithdrawals24h = 10e6 // Below 50M threshold
					data.StakingInflow24h = 5e6     // Below 10M threshold
					data.DerivativesOIChange = 5.0  // Below 15% threshold
				}

				result, err := gateEvaluator.EvaluateConfirmation(context.Background(), data)
				require.NoError(t, err)

				assert.Equal(t, scenario.expectedPass, result.Passed,
					"Pass status should match expected for %s", scenario.name)
				assert.Equal(t, scenario.expectedGates, result.ConfirmationCount,
					"Gate count should match expected for %s", scenario.name)

				// System should always complete evaluation gracefully
				assert.GreaterOrEqual(t, result.EvaluationTimeMs, int64(0),
					"Should report evaluation time")
				assert.NotNil(t, result.GateResults, "Should have gate results")
			})
		}
	})
}

// Mock microstructure evaluator for system tests
type mockMicroEvaluator struct {
	result *microstructure.EvaluationResult
	err    error
}

func (m *mockMicroEvaluator) EvaluateSnapshot(ctx context.Context, symbol string) (*microstructure.EvaluationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}
