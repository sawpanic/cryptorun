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

// TestPreMovementV33_FullSystemIntegration tests the complete Pre-Movement v3.3 pipeline
func TestPreMovementV33_FullSystemIntegration(t *testing.T) {
	// Create mock microstructure evaluator
	mockMicro := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			Symbol:    "BTC-USD",
			SpreadBps: 22.5,
			DepthUSD:  320000.0,
			VADR:      2.3,
		},
	}

	// Initialize engines with default configs
	scoreEngine := premove.NewScoreEngine(nil)
	gateEvaluator := premove.NewGateEvaluator(mockMicro, nil)

	// Create comprehensive test scenario: Bitcoin supply shock + institutional accumulation
	scoreData := &premove.PreMovementData{
		Symbol:    "BTC-USD",
		Timestamp: time.Now(),

		// Strong structural signals (36/40 points expected)
		FundingZScore:   3.8,   // Strong cross-venue funding divergence → 5.7 pts
		OIResidual:      1.8e6, // $1.8M OI residual → 4.0 pts
		ETFFlowTint:     0.95,  // 95% bullish ETF flows → 3.8 pts
		ReserveChange7d: -18.0, // -18% exchange reserves → 7.2 pts
		WhaleComposite:  0.92,  // 92% whale composite → 6.4 pts
		MicroDynamics:   0.85,  // 85% L1/L2 stress → 8.5 pts

		// Strong behavioral signals (31/35 points expected)
		SmartMoneyFlow: 0.88, // 88% institutional flows → 17.6 pts
		CVDResidual:    0.72, // 72% CVD residual → 10.8 pts

		// Moderate catalyst & compression (18/25 points expected)
		CatalystHeat:       0.7, // 70% catalyst heat → 10.5 pts
		VolCompressionRank: 0.8, // 80th percentile compression → 8.0 pts

		// Fresh data
		OldestFeedHours: 0.8, // 48 minutes old
	}

	gateData := &premove.ConfirmationData{
		Symbol:    "BTC-USD",
		Timestamp: time.Now(),

		// Core 2-of-3 confirmations (all should pass)
		FundingZScore:  3.8,  // Strong funding divergence ✅
		WhaleComposite: 0.92, // Strong whale activity ✅

		// Supply squeeze proxy (should pass with 3-of-4 components)
		ReserveChange7d:     -18.0, // Strong depletion ✅
		LargeWithdrawals24h: 120e6, // $120M withdrawals ✅
		StakingInflow24h:    18e6,  // $18M staking ✅
		DerivativesOIChange: 12.0,  // Below 15% threshold ❌
		SupplyProxyScore:    0.0,   // Will be calculated

		// Volume confirmation in normal regime (should not trigger)
		VolumeRatio24h: 1.8, // Below 2.5× threshold
		CurrentRegime:  "normal",

		// Microstructure context
		SpreadBps: 22.5,
		DepthUSD:  320000.0,
		VADR:      2.3,
	}

	// Execute full Pre-Movement evaluation
	scoreResult, err := scoreEngine.CalculateScore(context.Background(), scoreData)
	require.NoError(t, err, "Score calculation should succeed")

	gateResult, err := gateEvaluator.EvaluateConfirmation(context.Background(), gateData)
	require.NoError(t, err, "Gate evaluation should succeed")

	// Validate scoring results
	t.Run("ScoreValidation", func(t *testing.T) {
		assert.True(t, scoreResult.IsValid, "Score should be valid")
		assert.Greater(t, scoreResult.TotalScore, 80.0, "Strong signals should yield >80 score")
		assert.LessOrEqual(t, scoreResult.TotalScore, 100.0, "Score should be capped at 100")

		// Check component score ranges
		assert.InDelta(t, 13.5, scoreResult.ComponentScores["derivatives"], 2.0, "Derivatives score ~13.5")
		assert.InDelta(t, 13.6, scoreResult.ComponentScores["supply_demand"], 2.0, "Supply/demand score ~13.6")
		assert.InDelta(t, 8.5, scoreResult.ComponentScores["microstructure"], 1.0, "Microstructure score ~8.5")
		assert.InDelta(t, 17.6, scoreResult.ComponentScores["smart_money"], 2.0, "Smart money score ~17.6")
		assert.InDelta(t, 10.8, scoreResult.ComponentScores["cvd_residual"], 2.0, "CVD residual score ~10.8")

		// Verify attribution data
		assert.Contains(t, scoreResult.Attribution, "derivatives")
		assert.Contains(t, scoreResult.Attribution, "supply_demand")
	})

	// Validate gate confirmation results
	t.Run("GateValidation", func(t *testing.T) {
		assert.True(t, gateResult.Passed, "Should pass with 3-of-3 confirmations")
		assert.Equal(t, 3, gateResult.ConfirmationCount, "Should have all 3 confirmations")
		assert.Equal(t, 2, gateResult.RequiredCount, "Should require 2-of-3")

		// Check individual confirmations
		assert.Contains(t, gateResult.PassedGates, "funding_divergence")
		assert.Contains(t, gateResult.PassedGates, "whale_composite")
		assert.Contains(t, gateResult.PassedGates, "supply_squeeze")

		// Verify supply squeeze breakdown
		assert.NotNil(t, gateResult.SupplyBreakdown)
		assert.Equal(t, 3, gateResult.SupplyBreakdown.ComponentCount, "Should have 3-of-4 supply components")
		assert.GreaterOrEqual(t, gateResult.SupplyBreakdown.ProxyScore, 0.6, "Strong components should meet threshold")

		// Check precedence score (3.0 + 2.0 + 1.0 = 6.0 max)
		assert.Equal(t, 6.0, gateResult.PrecedenceScore, "Should have maximum precedence")

		// Volume confirmation should not be active in normal regime
		assert.False(t, gateResult.VolumeBoost, "No volume boost in normal regime")
	})

	// Test integrated decision making
	t.Run("IntegratedDecision", func(t *testing.T) {
		// Both score and gates pass → Strong Pre-Movement signal
		strongSignal := scoreResult.TotalScore > 75.0 && gateResult.Passed
		assert.True(t, strongSignal, "Should generate strong Pre-Movement signal")

		// Calculate combined confidence
		scoreConfidence := scoreResult.TotalScore / 100.0
		gateConfidence := float64(gateResult.ConfirmationCount) / float64(gateResult.RequiredCount)
		combinedConfidence := (scoreConfidence + gateConfidence) / 2.0

		assert.Greater(t, combinedConfidence, 0.8, "Combined confidence should be >80%")

		// Performance requirements
		assert.Less(t, scoreResult.EvaluationTimeMs, int64(100), "Score evaluation should be <100ms")
		assert.Less(t, gateResult.EvaluationTimeMs, int64(200), "Gate evaluation should be <200ms")
	})
}

// TestPreMovementV33_RegimeAdaptation tests behavior across different market regimes
func TestPreMovementV33_RegimeAdaptation(t *testing.T) {
	mockMicro := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			Symbol:    "ETH-USD",
			SpreadBps: 35.0,
			DepthUSD:  150000.0,
			VADR:      1.9,
		},
	}

	gateEvaluator := premove.NewGateEvaluator(mockMicro, nil)

	// Base data with marginal confirmations
	baseData := &premove.ConfirmationData{
		Symbol:    "ETH-USD",
		Timestamp: time.Now(),

		// Only 1 strong core confirmation
		FundingZScore:  2.2, // Passes ✅
		WhaleComposite: 0.6, // Fails (below 0.7)

		// Weak supply squeeze
		ReserveChange7d:     -3.0, // Fails
		LargeWithdrawals24h: 30e6, // Fails
		StakingInflow24h:    5e6,  // Fails
		DerivativesOIChange: 8.0,  // Fails
		SupplyProxyScore:    0.0,

		// Strong volume
		VolumeRatio24h: 3.5,

		SpreadBps: 35.0,
		DepthUSD:  150000.0,
		VADR:      1.9,
	}

	// Test normal regime (no volume boost)
	t.Run("NormalRegime", func(t *testing.T) {
		data := *baseData
		data.CurrentRegime = "normal"

		result, err := gateEvaluator.EvaluateConfirmation(context.Background(), &data)
		require.NoError(t, err)

		assert.False(t, result.Passed, "Should fail in normal regime (1-of-3, no volume boost)")
		assert.Equal(t, 1, result.ConfirmationCount)
		assert.Equal(t, 2, result.RequiredCount)
		assert.False(t, result.VolumeBoost)
	})

	// Test risk_off regime (volume boost enabled)
	t.Run("RiskOffRegime", func(t *testing.T) {
		data := *baseData
		data.CurrentRegime = "risk_off"

		result, err := gateEvaluator.EvaluateConfirmation(context.Background(), &data)
		require.NoError(t, err)

		assert.True(t, result.Passed, "Should pass in risk_off regime (1-of-3 + volume)")
		assert.Equal(t, 1, result.ConfirmationCount)
		assert.Equal(t, 1, result.RequiredCount, "Volume boost should reduce requirement")
		assert.True(t, result.VolumeBoost)
	})

	// Test btc_driven regime (volume boost enabled)
	t.Run("BTCDrivenRegime", func(t *testing.T) {
		data := *baseData
		data.CurrentRegime = "btc_driven"

		result, err := gateEvaluator.EvaluateConfirmation(context.Background(), &data)
		require.NoError(t, err)

		assert.True(t, result.Passed, "Should pass in btc_driven regime (1-of-3 + volume)")
		assert.True(t, result.VolumeBoost)
	})
}

// TestPreMovementV33_PrecedenceRules tests gate precedence and ranking
func TestPreMovementV33_PrecedenceRules(t *testing.T) {
	mockMicro := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			Symbol:    "SOL-USD",
			SpreadBps: 40.0,
			DepthUSD:  100000.0,
			VADR:      1.8,
		},
	}

	gateEvaluator := premove.NewGateEvaluator(mockMicro, nil)

	// Create multiple candidates with different precedence profiles
	candidates := []*premove.ConfirmationData{
		{
			Symbol:           "FUNDING-WHALE", // Highest precedence combination (3.0 + 2.0 = 5.0)
			FundingZScore:    3.0,
			WhaleComposite:   0.8,
			SupplyProxyScore: 0.4, // Fails
			VolumeRatio24h:   1.0,
			CurrentRegime:    "normal",
		},
		{
			Symbol:              "SUPPLY-WHALE", // Medium precedence (1.0 + 2.0 = 3.0)
			FundingZScore:       1.5,            // Fails
			WhaleComposite:      0.8,
			ReserveChange7d:     -10.0,
			LargeWithdrawals24h: 60e6,
			StakingInflow24h:    12e6,
			DerivativesOIChange: 20.0,
			SupplyProxyScore:    0.8,
			VolumeRatio24h:      1.0,
			CurrentRegime:       "normal",
		},
		{
			Symbol:              "FUNDING-SUPPLY", // Medium precedence (3.0 + 1.0 = 4.0)
			FundingZScore:       2.8,
			WhaleComposite:      0.5, // Fails
			ReserveChange7d:     -8.0,
			LargeWithdrawals24h: 55e6,
			StakingInflow24h:    11e6,
			DerivativesOIChange: 16.0,
			SupplyProxyScore:    0.7,
			VolumeRatio24h:      1.0,
			CurrentRegime:       "normal",
		},
	}

	// Evaluate all candidates
	var results []*premove.ConfirmationResult
	for _, candidate := range candidates {
		result, err := gateEvaluator.EvaluateConfirmation(context.Background(), candidate)
		require.NoError(t, err)
		results = append(results, result)
	}

	// All should pass (2-of-3 requirement)
	for _, result := range results {
		assert.True(t, result.Passed, "All candidates should pass 2-of-3")
	}

	// Rank by precedence
	ranked := premove.RankCandidates(results)

	// Check precedence order
	assert.Equal(t, "FUNDING-WHALE", ranked[0].Symbol, "Funding+Whale should rank first (5.0)")
	assert.Equal(t, "FUNDING-SUPPLY", ranked[1].Symbol, "Funding+Supply should rank second (4.0)")
	assert.Equal(t, "SUPPLY-WHALE", ranked[2].Symbol, "Supply+Whale should rank third (3.0)")

	// Verify precedence scores
	assert.Equal(t, 5.0, ranked[0].PrecedenceScore)
	assert.Equal(t, 4.0, ranked[1].PrecedenceScore)
	assert.Equal(t, 3.0, ranked[2].PrecedenceScore)
}

// TestPreMovementV33_DataQualityDegradation tests handling of degraded data quality
func TestPreMovementV33_DataQualityDegradation(t *testing.T) {
	scoreEngine := premove.NewScoreEngine(nil)

	// Test various data quality scenarios
	scenarios := []struct {
		name             string
		oldestFeedHours  float64
		expectedPenalty  float64
		expectedValid    bool
		expectedScoreCap float64
	}{
		{
			name:             "Fresh data",
			oldestFeedHours:  0.5, // 30 minutes
			expectedPenalty:  0.0, // No penalty
			expectedValid:    true,
			expectedScoreCap: 100.0,
		},
		{
			name:             "Acceptable staleness",
			oldestFeedHours:  2.0, // Exactly at threshold
			expectedPenalty:  0.0, // No penalty at threshold
			expectedValid:    true,
			expectedScoreCap: 100.0,
		},
		{
			name:             "Moderate staleness",
			oldestFeedHours:  3.0, // 1 hour past threshold
			expectedPenalty:  0.1, // 10% penalty
			expectedValid:    true,
			expectedScoreCap: 90.0,
		},
		{
			name:             "Significant staleness",
			oldestFeedHours:  4.0, // 2 hours past threshold (max penalty)
			expectedPenalty:  0.2, // 20% penalty
			expectedValid:    true,
			expectedScoreCap: 80.0,
		},
		{
			name:             "Extreme staleness",
			oldestFeedHours:  6.0, // 4 hours past threshold
			expectedPenalty:  0.2, // Capped at 20%
			expectedValid:    true,
			expectedScoreCap: 80.0,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			data := &premove.PreMovementData{
				Symbol:    "STALE-TEST",
				Timestamp: time.Now(),

				// Strong base signals (would score ~90 without penalty)
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

				OldestFeedHours: scenario.oldestFeedHours,
			}

			result, err := scoreEngine.CalculateScore(context.Background(), data)
			require.NoError(t, err)

			// Check freshness penalty
			assert.InDelta(t, scenario.expectedPenalty, result.DataFreshness.FreshnessPenalty, 0.01,
				"Freshness penalty should match expected")

			// Check score impact
			assert.LessOrEqual(t, result.TotalScore, scenario.expectedScoreCap,
				"Score should be capped by freshness penalty")

			// Check validity
			assert.Equal(t, scenario.expectedValid, result.IsValid,
				"Validity should match expected")
		})
	}
}

// Mock microstructure evaluator for integration tests
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
