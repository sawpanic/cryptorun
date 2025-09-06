package premove

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cryptorun/internal/microstructure"
)

// Mock microstructure evaluator for testing
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

func TestGateEvaluator_EvaluateConfirmation_2of3Pass(t *testing.T) {
	mockMicro := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			Symbol:    "BTC-USD",
			SpreadBps: 25.0,
			DepthUSD:  250000.0,
			VADR:      2.1,
		},
	}

	evaluator := NewGateEvaluator(mockMicro, nil)

	data := &ConfirmationData{
		Symbol:    "BTC-USD",
		Timestamp: time.Now(),

		// Strong funding divergence (PASS)
		FundingZScore: 3.2,

		// Strong whale activity (PASS)
		WhaleComposite: 0.8,

		// Weak supply squeeze proxy (FAIL)
		SupplyProxyScore:     0.4, // Below 0.6 threshold
		ReserveChange7d:      -2.0, // Not sufficient for depletion
		LargeWithdrawals24h:  20e6, // Below $50M threshold
		StakingInflow24h:     5e6,  // Below $10M threshold
		DerivativesOIChange:  8.0,  // Below 15% threshold

		// Volume confirmation
		VolumeRatio24h: 3.0,
		CurrentRegime:  "risk_off", // Enables volume confirmation

		// Microstructure context
		SpreadBps: 25.0,
		DepthUSD:  250000.0,
		VADR:      2.1,
	}

	result, err := evaluator.EvaluateConfirmation(context.Background(), data)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Should pass with 2-of-3 + volume boost
	assert.True(t, result.Passed, "Should pass with 2 strong confirmations")
	assert.Equal(t, 2, result.ConfirmationCount, "Should have 2 confirmations")
	assert.True(t, result.VolumeBoost, "Should have volume boost in risk_off regime")
	
	// Check individual gate results
	assert.Contains(t, result.PassedGates, "funding_divergence")
	assert.Contains(t, result.PassedGates, "whale_composite")
	assert.Contains(t, result.PassedGates, "volume_confirmation")
	assert.Contains(t, result.FailedGates, "supply_squeeze")

	// Should have precedence score
	assert.Greater(t, result.PrecedenceScore, 0.0, "Should calculate precedence score")

	// Should have supply breakdown
	assert.NotNil(t, result.SupplyBreakdown)
	assert.Less(t, result.SupplyBreakdown.ComponentCount, 2, "Should have <2 supply components")
}

func TestGateEvaluator_EvaluateConfirmation_SupplySqueezeProxy(t *testing.T) {
	mockMicro := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			Symbol:    "ETH-USD",
			SpreadBps: 30.0,
			DepthUSD:  180000.0,
			VADR:      1.9,
		},
	}

	evaluator := NewGateEvaluator(mockMicro, nil)

	data := &ConfirmationData{
		Symbol:    "ETH-USD",
		Timestamp: time.Now(),

		// Weak funding and whale
		FundingZScore:  1.5, // Below 2.0 threshold
		WhaleComposite: 0.5, // Below 0.7 threshold

		// Strong supply squeeze (4-of-4 components passing)
		ReserveChange7d:     -12.0, // Strong depletion ✅
		LargeWithdrawals24h: 80e6,  // Large withdrawals ✅
		StakingInflow24h:    15e6,  // Strong staking ✅
		DerivativesOIChange: 25.0,  // Strong derivatives ✅

		// Should generate high proxy score
		SupplyProxyScore: 0.0, // Will be calculated

		VolumeRatio24h: 1.5,
		CurrentRegime:  "normal",
	}

	result, err := evaluator.EvaluateConfirmation(context.Background(), data)
	require.NoError(t, err)

	// Should fail overall (only 1-of-3 core confirmations)
	assert.False(t, result.Passed, "Should fail with only supply squeeze passing")
	assert.Equal(t, 1, result.ConfirmationCount, "Should have 1 confirmation")

	// But supply squeeze should pass with strong proxy score
	assert.Contains(t, result.PassedGates, "supply_squeeze")
	assert.NotNil(t, result.SupplyBreakdown)
	assert.Equal(t, 4, result.SupplyBreakdown.ComponentCount, "Should have all 4 supply components")
	assert.GreaterOrEqual(t, result.SupplyBreakdown.ProxyScore, 0.6, "Strong components should yield high proxy score")

	// Check individual supply components
	supplyBreakdown := result.SupplyBreakdown
	assert.True(t, supplyBreakdown.ComponentResults["reserve_depletion"].Passed)
	assert.True(t, supplyBreakdown.ComponentResults["large_withdrawals"].Passed)
	assert.True(t, supplyBreakdown.ComponentResults["staking_inflow"].Passed)
	assert.True(t, supplyBreakdown.ComponentResults["derivatives_oi"].Passed)
}

func TestGateEvaluator_EvaluateConfirmation_VolumeBoostRegimes(t *testing.T) {
	mockMicro := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			Symbol:    "SOL-USD", 
			SpreadBps: 35.0,
			DepthUSD:  120000.0,
			VADR:      1.8,
		},
	}

	evaluator := NewGateEvaluator(mockMicro, nil)

	// Test volume boost in btc_driven regime
	data := &ConfirmationData{
		Symbol:         "SOL-USD",
		Timestamp:      time.Now(),
		FundingZScore:  2.5,  // Only 1 strong confirmation
		WhaleComposite: 0.5,  // Weak
		SupplyProxyScore: 0.4, // Weak

		// Strong volume confirmation
		VolumeRatio24h: 4.0,
		CurrentRegime:  "btc_driven", // Should enable volume boost
	}

	result, err := evaluator.EvaluateConfirmation(context.Background(), data)
	require.NoError(t, err)

	// Should pass with 1-of-3 + volume boost in btc_driven regime
	assert.True(t, result.Passed, "Should pass with volume boost in btc_driven regime")
	assert.Equal(t, 1, result.ConfirmationCount, "Should have 1 core confirmation")
	assert.Equal(t, 1, result.RequiredCount, "Volume boost should reduce requirement to 1")
	assert.True(t, result.VolumeBoost, "Should have volume boost")

	// Test no volume boost in normal regime
	data.CurrentRegime = "normal"
	result, err = evaluator.EvaluateConfirmation(context.Background(), data)
	require.NoError(t, err)

	assert.False(t, result.Passed, "Should not pass without volume boost in normal regime")
	assert.Equal(t, 2, result.RequiredCount, "Should require 2-of-3 in normal regime")
	assert.False(t, result.VolumeBoost, "Should not have volume boost in normal regime")
}

func TestGateEvaluator_EvaluateConfirmation_PrecedenceRanking(t *testing.T) {
	mockMicro := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			Symbol:    "ADA-USD",
			SpreadBps: 40.0,
			DepthUSD:  90000.0,
			VADR:      1.7,
		},
	}

	evaluator := NewGateEvaluator(mockMicro, nil)

	// Test all 3 confirmations passing (should get max precedence)
	data := &ConfirmationData{
		Symbol:    "ADA-USD",
		Timestamp: time.Now(),

		// All strong confirmations
		FundingZScore:        3.0, // Funding (precedence 3.0)
		WhaleComposite:       0.8, // Whale (precedence 2.0)  
		SupplyProxyScore:     0.7, // Supply (precedence 1.0)
		ReserveChange7d:      -10.0,
		LargeWithdrawals24h:  60e6,
		StakingInflow24h:     12e6,
		DerivativesOIChange:  18.0,

		VolumeRatio24h: 2.0,
		CurrentRegime:  "normal",
	}

	result, err := evaluator.EvaluateConfirmation(context.Background(), data)
	require.NoError(t, err)

	assert.True(t, result.Passed, "Should pass with all confirmations")
	assert.Equal(t, 3, result.ConfirmationCount, "Should have all 3 confirmations")
	
	// Should have maximum precedence (3.0 + 2.0 + 1.0 = 6.0)
	assert.Equal(t, 6.0, result.PrecedenceScore, "Should have max precedence with all gates")
}

func TestGateEvaluator_RankCandidates(t *testing.T) {
	// Create mock results with different strengths
	results := []*ConfirmationResult{
		{
			Symbol:            "WEAK",
			Passed:            false,
			ConfirmationCount: 0,
			PrecedenceScore:   0.0,
		},
		{
			Symbol:            "STRONG",
			Passed:            true,
			ConfirmationCount: 3,
			PrecedenceScore:   6.0, // All gates
		},
		{
			Symbol:            "MODERATE",
			Passed:            true,
			ConfirmationCount: 2,
			PrecedenceScore:   5.0, // Funding + whale
		},
		{
			Symbol:            "BLOCKED",
			Passed:            false,
			ConfirmationCount: 1,
			PrecedenceScore:   3.0,
		},
	}

	ranked := RankCandidates(results)

	// Should sort passed first, then by precedence
	assert.Equal(t, "STRONG", ranked[0].Symbol, "Strongest should rank first")
	assert.Equal(t, "MODERATE", ranked[1].Symbol, "Moderate should rank second")
	assert.True(t, ranked[0].Passed && ranked[1].Passed, "Passed results should rank first")
	assert.False(t, ranked[2].Passed || ranked[3].Passed, "Failed results should rank last")
}

func TestGateEvaluator_PerformanceTimeout(t *testing.T) {
	// Mock slow microstructure evaluator
	slowMicro := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			Symbol:    "SLOW-USD",
			SpreadBps: 50.0,
			DepthUSD:  75000.0,
			VADR:      1.5,
		},
	}

	config := DefaultGateConfig()
	config.MaxEvaluationTimeMs = 10 // Very low timeout for testing

	evaluator := NewGateEvaluator(slowMicro, config)

	data := &ConfirmationData{
		Symbol:         "SLOW-USD",
		Timestamp:      time.Now(),
		FundingZScore:  2.5,
		WhaleComposite: 0.8,
		SupplyProxyScore: 0.7,
		VolumeRatio24h: 2.0,
		CurrentRegime:  "normal",
	}

	result, err := evaluator.EvaluateConfirmation(context.Background(), data)
	require.NoError(t, err)

	// Should complete but may have performance warning
	// (In practice, we'd add artificial delay to test timeout warning)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.EvaluationTimeMs, int64(0))
}

func TestGateEvaluator_GetConfirmationSummary(t *testing.T) {
	result := &ConfirmationResult{
		Symbol:            "BTC-USD",
		Passed:            true,
		ConfirmationCount: 2,
		RequiredCount:     2,
		VolumeBoost:       true,
		PrecedenceScore:   5.0,
		EvaluationTimeMs:  123,
	}

	summary := result.GetConfirmationSummary()
	assert.Contains(t, summary, "✅ CONFIRMED")
	assert.Contains(t, summary, "BTC-USD")
	assert.Contains(t, summary, "2/2 gates")
	assert.Contains(t, summary, "+VOL")
	assert.Contains(t, summary, "5.0 precedence")
	assert.Contains(t, summary, "123ms")
}

func TestGateEvaluator_GetDetailedReport(t *testing.T) {
	result := &ConfirmationResult{
		Symbol:            "ETH-USD",
		Passed:            true,
		ConfirmationCount: 2,
		RequiredCount:     2,
		PrecedenceScore:   5.0,
		EvaluationTimeMs:  89,
		GateResults: map[string]*GateCheck{
			"funding_divergence": {
				Name:        "funding_divergence",
				Passed:      true,
				Description: "Funding z-score 2.50 ≥ 2.00",
			},
			"whale_composite": {
				Name:        "whale_composite", 
				Passed:      true,
				Description: "Whale composite 0.75 ≥ 0.70",
			},
			"supply_squeeze": {
				Name:        "supply_squeeze",
				Passed:      false,
				Description: "Supply proxy 0.45 ≥ 0.60 (1/4 components)",
			},
		},
		SupplyBreakdown: &SupplySqueezeBreakdown{
			ComponentCount: 1,
			ComponentResults: map[string]*GateCheck{
				"reserve_depletion": {
					Name:        "reserve_depletion",
					Passed:      true,
					Description: "Reserve change -8.0% ≤ -5.0%",
				},
			},
		},
		VolumeBoost: false,
		MicroReport: &microstructure.EvaluationResult{
			SpreadBps: 28.0,
			DepthUSD:  180000.0,
			VADR:      2.0,
		},
	}

	report := result.GetDetailedReport()
	assert.Contains(t, report, "ETH-USD")
	assert.Contains(t, report, "CONFIRMED ✅")
	assert.Contains(t, report, "2/2")
	assert.Contains(t, report, "5.0 precedence")
	assert.Contains(t, report, "✅ funding_divergence")
	assert.Contains(t, report, "✅ whale_composite") 
	assert.Contains(t, report, "❌ supply_squeeze")
	assert.Contains(t, report, "Supply Squeeze Components (1/4 passed")
	assert.Contains(t, report, "Spread: 28.0 bps")
}

func TestDefaultGateConfig(t *testing.T) {
	config := DefaultGateConfig()
	require.NotNil(t, config)

	// Check core thresholds
	assert.Equal(t, 2.0, config.FundingDivergenceThreshold)
	assert.Equal(t, 0.6, config.SupplySqueezeThreshold)
	assert.Equal(t, 0.7, config.WhaleCompositeThreshold)

	// Check supply squeeze component thresholds
	assert.Equal(t, -5.0, config.ReserveDepletionThreshold)
	assert.Equal(t, 50e6, config.LargeWithdrawalsThreshold)
	assert.Equal(t, 10e6, config.StakingInflowThreshold)
	assert.Equal(t, 15.0, config.DerivativesLeverageThreshold)

	// Check precedence weights
	assert.Equal(t, 3.0, config.FundingPrecedence, "Funding should have highest precedence")
	assert.Equal(t, 2.0, config.WhalePrecedence, "Whale should have medium precedence")
	assert.Equal(t, 1.0, config.SupplyPrecedence, "Supply should have lowest precedence")

	// Check volume confirmation
	assert.True(t, config.VolumeConfirmationEnabled)
	assert.Equal(t, 2.5, config.VolumeConfirmationThreshold)

	// Check performance limits
	assert.Equal(t, int64(500), config.MaxEvaluationTimeMs)
}