package premove

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScoreEngine_CalculateScore_AllComponents(t *testing.T) {
	engine := NewScoreEngine(nil) // Use default config

	data := &PreMovementData{
		Symbol:    "BTC-USD",
		Timestamp: time.Now(),

		// Strong structural signals
		FundingZScore:   3.5,   // Strong funding divergence
		OIResidual:      1.2e6, // $1.2M OI residual
		ETFFlowTint:     0.8,   // 80% bullish flows
		ReserveChange7d: -15.0, // -15% exchange reserves
		WhaleComposite:  0.9,   // 90% whale activity
		MicroDynamics:   0.7,   // 70% L1/L2 stress

		// Strong behavioral signals
		SmartMoneyFlow: 0.85, // 85% institutional flow
		CVDResidual:    0.6,  // 60% CVD residual

		// Strong catalyst & compression
		CatalystHeat:       0.9,  // 90% catalyst significance
		VolCompressionRank: 0.95, // 95th percentile compression

		// Fresh data
		OldestFeedHours: 0.5, // 30 minutes
	}

	result, err := engine.CalculateScore(context.Background(), data)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Should achieve very high score with all strong signals
	assert.Greater(t, result.TotalScore, 90.0, "Strong signals should yield >90 score")
	assert.LessOrEqual(t, result.TotalScore, 100.0, "Score should not exceed 100")
	assert.True(t, result.IsValid, "Score should be valid")

	// Verify component scores are present
	assert.Contains(t, result.ComponentScores, "derivatives")
	assert.Contains(t, result.ComponentScores, "supply_demand")
	assert.Contains(t, result.ComponentScores, "microstructure")
	assert.Contains(t, result.ComponentScores, "smart_money")
	assert.Contains(t, result.ComponentScores, "cvd_residual")
	assert.Contains(t, result.ComponentScores, "catalyst")
	assert.Contains(t, result.ComponentScores, "compression")

	// Check individual component contributions
	derivScore := result.ComponentScores["derivatives"]
	assert.Greater(t, derivScore, 10.0, "Strong derivatives should contribute >10 points")
	assert.LessOrEqual(t, derivScore, 15.0, "Derivatives capped at 15 points")

	// Verify attribution data is present
	assert.Contains(t, result.Attribution, "derivatives")
	assert.Contains(t, result.Attribution, "smart_money")

	// Check freshness penalty (should be minimal)
	assert.NotNil(t, result.DataFreshness)
	assert.LessOrEqual(t, result.DataFreshness.FreshnessPenalty, 0.1, "Fresh data should have minimal penalty")
}

func TestScoreEngine_CalculateScore_WeakSignals(t *testing.T) {
	engine := NewScoreEngine(nil)

	data := &PreMovementData{
		Symbol:    "ETH-USD",
		Timestamp: time.Now(),

		// Weak structural signals
		FundingZScore:   0.5,   // Below 2.0 threshold
		OIResidual:      50000, // $50k OI residual
		ETFFlowTint:     0.1,   // 10% flows
		ReserveChange7d: -1.0,  // Minor reserve change
		WhaleComposite:  0.2,   // 20% whale activity
		MicroDynamics:   0.1,   // Low L1/L2 stress

		// Weak behavioral signals
		SmartMoneyFlow: 0.15, // 15% institutional flow
		CVDResidual:    0.05, // 5% CVD residual

		// Weak catalyst & compression
		CatalystHeat:       0.1, // 10% catalyst heat
		VolCompressionRank: 0.2, // 20th percentile

		// Fresh data
		OldestFeedHours: 1.0,
	}

	result, err := engine.CalculateScore(context.Background(), data)
	require.NoError(t, err)

	// Should yield low score with weak signals
	assert.Less(t, result.TotalScore, 30.0, "Weak signals should yield <30 score")
	assert.GreaterOrEqual(t, result.TotalScore, 0.0, "Score should not be negative")
}

func TestScoreEngine_FreshnessPenalty_StaleData(t *testing.T) {
	engine := NewScoreEngine(nil)

	data := &PreMovementData{
		Symbol:    "SOL-USD",
		Timestamp: time.Now(),

		// Moderate signals that would normally score ~60 points
		FundingZScore:      2.5,
		OIResidual:         800000,
		ETFFlowTint:        0.6,
		ReserveChange7d:    -8.0,
		WhaleComposite:     0.6,
		MicroDynamics:      0.5,
		SmartMoneyFlow:     0.5,
		CVDResidual:        0.4,
		CatalystHeat:       0.5,
		VolCompressionRank: 0.6,

		// Very stale data (4 hours old)
		OldestFeedHours: 4.0,
	}

	result, err := engine.CalculateScore(context.Background(), data)
	require.NoError(t, err)

	// Should apply significant freshness penalty
	assert.NotNil(t, result.DataFreshness)
	assert.Greater(t, result.DataFreshness.FreshnessPenalty, 0.15, "Stale data should have >15% penalty")
	assert.Equal(t, 4.0, result.DataFreshness.OldestFeedHours)

	// Final score should be meaningfully reduced
	// With 4hr old data (2x max freshness), expect ~20% penalty
	assert.Less(t, result.TotalScore, 50.0, "Freshness penalty should reduce score significantly")
}

func TestScoreEngine_ComponentBounds(t *testing.T) {
	engine := NewScoreEngine(nil)

	// Test extreme values to verify component bounds
	data := &PreMovementData{
		Symbol:    "EXTREME-TEST",
		Timestamp: time.Now(),

		// Extreme positive values
		FundingZScore:      100.0,  // Should cap at ~7 points
		OIResidual:         1e9,    // $1B OI - should cap at 4 points
		ETFFlowTint:        5.0,    // 500% - should cap at 4 points
		ReserveChange7d:    -100.0, // -100% reserves - should cap at 8 points
		WhaleComposite:     10.0,   // 1000% - should cap at 7 points
		MicroDynamics:      5.0,    // 500% - should cap at 10 points
		SmartMoneyFlow:     5.0,    // 500% - should cap at 20 points
		CVDResidual:        10.0,   // 1000% - should cap at 15 points
		CatalystHeat:       5.0,    // 500% - should cap at 15 points
		VolCompressionRank: 5.0,    // 500% - should cap at 10 points

		OldestFeedHours: 0.1, // Fresh
	}

	result, err := engine.CalculateScore(context.Background(), data)
	require.NoError(t, err)

	// Total should be capped at 100
	assert.LessOrEqual(t, result.TotalScore, 100.0, "Score should be capped at 100")

	// Individual components should respect their bounds
	assert.LessOrEqual(t, result.ComponentScores["derivatives"], 15.0)
	assert.LessOrEqual(t, result.ComponentScores["supply_demand"], 15.0)
	assert.LessOrEqual(t, result.ComponentScores["microstructure"], 10.0)
	assert.LessOrEqual(t, result.ComponentScores["smart_money"], 20.0)
	assert.LessOrEqual(t, result.ComponentScores["cvd_residual"], 15.0)
	assert.LessOrEqual(t, result.ComponentScores["catalyst"], 15.0)
	assert.LessOrEqual(t, result.ComponentScores["compression"], 10.0)
}

func TestScoreEngine_NegativeValues(t *testing.T) {
	engine := NewScoreEngine(nil)

	data := &PreMovementData{
		Symbol:    "NEGATIVE-TEST",
		Timestamp: time.Now(),

		// Negative values (should be handled gracefully)
		FundingZScore:      -2.0,   // Negative funding divergence
		OIResidual:         -50000, // Negative OI residual
		ETFFlowTint:        -0.5,   // -50% flows (bearish)
		ReserveChange7d:    10.0,   // Positive reserves (supply increase)
		WhaleComposite:     -0.3,   // Negative whale activity
		MicroDynamics:      -0.2,   // Negative dynamics
		SmartMoneyFlow:     -0.4,   // Outflows
		CVDResidual:        -0.8,   // Negative CVD residual
		CatalystHeat:       -0.2,   // Negative catalyst
		VolCompressionRank: -0.1,   // Negative compression

		OldestFeedHours: 1.0,
	}

	result, err := engine.CalculateScore(context.Background(), data)
	require.NoError(t, err)

	// Score should not be negative (components should floor at 0)
	assert.GreaterOrEqual(t, result.TotalScore, 0.0, "Score should not be negative")

	// Most component scores should be 0 or very low
	for component, score := range result.ComponentScores {
		assert.GreaterOrEqual(t, score, 0.0, "Component %s should not be negative", component)
	}
}

func TestScoreEngine_GetScoreSummary(t *testing.T) {
	result := &ScoreResult{
		Symbol:     "BTC-USD",
		TotalScore: 87.5,
		IsValid:    true,
		DataFreshness: &FreshnessInfo{
			FreshnessPenalty: 0.05, // 5% penalty
			OldestFeedHours:  2.5,
		},
		EvaluationTimeMs: 45,
	}

	summary := result.GetScoreSummary()
	assert.Contains(t, summary, "BTC-USD")
	assert.Contains(t, summary, "87.5")
	assert.Contains(t, summary, "✅ VALID")
	assert.Contains(t, summary, "45ms")
}

func TestScoreEngine_GetDetailedBreakdown(t *testing.T) {
	result := &ScoreResult{
		Symbol:     "ETH-USD",
		TotalScore: 72.3,
		IsValid:    true,
		ComponentScores: map[string]float64{
			"derivatives":    12.5,
			"supply_demand":  8.2,
			"microstructure": 6.1,
			"smart_money":    15.8,
			"cvd_residual":   11.3,
			"catalyst":       10.4,
			"compression":    8.0,
		},
		DataFreshness: &FreshnessInfo{
			FreshnessPenalty: 0.12,
			OldestFeedHours:  3.2,
		},
		EvaluationTimeMs: 67,
		Warnings:         []string{"CVD data quality degraded"},
	}

	breakdown := result.GetDetailedBreakdown()
	assert.Contains(t, breakdown, "ETH-USD")
	assert.Contains(t, breakdown, "72.3/100")
	assert.Contains(t, breakdown, "derivatives: 12.5 pts")
	assert.Contains(t, breakdown, "Freshness Penalty: -12.0%")
	assert.Contains(t, breakdown, "CVD data quality degraded")
}

func TestDefaultScoreConfig(t *testing.T) {
	config := DefaultScoreConfig()
	require.NotNil(t, config)

	// Check weights sum to 100 points
	totalWeight := config.DerivativesWeight + config.SupplyDemandWeight + config.MicrostructureWeight +
		config.SmartMoneyWeight + config.CVDResidualWeight + config.CatalystWeight + config.CompressionWeight

	assert.Equal(t, 100.0, totalWeight, "Component weights should sum to 100 points")

	// Check structural components total 40 points
	structuralTotal := config.DerivativesWeight + config.SupplyDemandWeight + config.MicrostructureWeight
	assert.Equal(t, 40.0, structuralTotal, "Structural components should total 40 points")

	// Check behavioral components total 35 points
	behavioralTotal := config.SmartMoneyWeight + config.CVDResidualWeight
	assert.Equal(t, 35.0, behavioralTotal, "Behavioral components should total 35 points")

	// Check catalyst & compression total 25 points
	catalystTotal := config.CatalystWeight + config.CompressionWeight
	assert.Equal(t, 25.0, catalystTotal, "Catalyst & compression should total 25 points")

	// Check freshness parameters
	assert.Equal(t, 2.0, config.MaxFreshnessHours, "Max freshness should be 2 hours")
	assert.Equal(t, 20.0, config.FreshnessPenaltyPct, "Max penalty should be 20%")
}

func TestScoreEngine_PerformanceRequirements(t *testing.T) {
	engine := NewScoreEngine(nil)

	data := &PreMovementData{
		Symbol:             "PERFORMANCE-TEST",
		Timestamp:          time.Now(),
		FundingZScore:      2.5,
		OIResidual:         500000,
		ETFFlowTint:        0.6,
		ReserveChange7d:    -7.0,
		WhaleComposite:     0.7,
		MicroDynamics:      0.5,
		SmartMoneyFlow:     0.6,
		CVDResidual:        0.4,
		CatalystHeat:       0.7,
		VolCompressionRank: 0.8,
		OldestFeedHours:    1.5,
	}

	// Test performance (should complete quickly)
	start := time.Now()
	result, err := engine.CalculateScore(context.Background(), data)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Less(t, duration.Milliseconds(), int64(50), "Score calculation should complete in <50ms")
	assert.Greater(t, result.EvaluationTimeMs, int64(0), "Should report evaluation time")
}
