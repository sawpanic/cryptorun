package gates

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/derivs"
	"github.com/sawpanic/cryptorun/internal/microstructure"
)

// Mock implementations for testing

type mockMicroEvaluator struct {
	vadr      float64
	spreadBps float64
	depthUSD  float64
	err       error
}

func (m *mockMicroEvaluator) EvaluateSnapshot(symbol string) (microstructure.EvaluationResult, error) {
	if m.err != nil {
		return microstructure.EvaluationResult{}, m.err
	}
	return microstructure.EvaluationResult{
		VADR:      m.vadr,
		SpreadBps: m.spreadBps,
		DepthUSD:  m.depthUSD,
		Healthy:   m.vadr >= 1.75 && m.spreadBps < 50.0 && m.depthUSD > 100000.0,
	}, nil
}

// Implement other required methods for microstructure.Evaluator interface
func (m *mockMicroEvaluator) EvaluateGates(ctx context.Context, symbol, venue string, orderbook *microstructure.OrderBookSnapshot, adv float64) (*microstructure.GateReport, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

func (m *mockMicroEvaluator) GetLiquidityTier(adv float64) *microstructure.LiquidityTier {
	return nil
}

func (m *mockMicroEvaluator) UpdateVenueHealth(venue string, health microstructure.VenueHealthStatus) error {
	return fmt.Errorf("not implemented in mock")
}

func (m *mockMicroEvaluator) GetVenueHealth(venue string) (*microstructure.VenueHealthStatus, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

type mockFundingProvider struct {
	hasSignificant bool
	maxDivergence  float64
	venue          string
	zScore         float64
	err            error
}

func (m *mockFundingProvider) GetFundingSnapshot(ctx context.Context, symbol string) (*derivs.FundingSnapshot, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &derivs.FundingSnapshot{
		Symbol:                   symbol,
		Timestamp:                time.Now(),
		MaxVenueDivergence:       m.maxDivergence,
		FundingDivergencePresent: m.hasSignificant,
	}, nil
}

type mockOIProvider struct {
	oiResidual float64
	err        error
}

func (m *mockOIProvider) GetOpenInterestSnapshot(ctx context.Context, symbol string, priceChange float64) (*derivs.OpenInterestSnapshot, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &derivs.OpenInterestSnapshot{
		Symbol:     symbol,
		Timestamp:  time.Now(),
		OIResidual: m.oiResidual,
	}, nil
}

type mockETFProvider struct {
	flowTint float64
	etfList  []string
	err      error
}

func (m *mockETFProvider) GetETFFlowSnapshot(ctx context.Context, symbol string) (*derivs.ETFFlowSnapshot, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &derivs.ETFFlowSnapshot{
		Symbol:    symbol,
		Timestamp: time.Now(),
		FlowTint:  m.flowTint,
		ETFList:   m.etfList,
	}, nil
}

// Test helper function
func createTestEvaluator(vadr, spreadBps, depthUSD float64, hasSignificantFunding bool, fundingDiv float64) *EntryGateEvaluator {
	return &EntryGateEvaluator{
		microEvaluator: &mockMicroEvaluator{
			vadr:      vadr,
			spreadBps: spreadBps,
			depthUSD:  depthUSD,
		},
		fundingProvider: &mockFundingProvider{
			hasSignificant: hasSignificantFunding,
			maxDivergence:  fundingDiv,
			venue:          "binance",
			zScore:         fundingDiv,
		},
		oiProvider: &mockOIProvider{
			oiResidual: 1500000.0, // Above threshold
		},
		etfProvider: &mockETFProvider{
			flowTint: 0.5, // Above threshold
			etfList:  []string{"IBIT", "GBTC"},
		},
		config: DefaultEntryGateConfig(),
	}
}

func TestEntryGateEvaluator_AllGatesPass(t *testing.T) {
	evaluator := createTestEvaluator(2.0, 30.0, 150000.0, true, 2.5)

	result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.Passed {
		t.Errorf("Expected all gates to pass, got failures: %v", result.FailureReasons)
	}

	if len(result.PassedGates) != len(result.GateResults) {
		t.Errorf("Expected all gates to pass. Passed: %d, Total: %d", len(result.PassedGates), len(result.GateResults))
	}

	// Check specific gates
	expectedGates := []string{"composite_score", "vadr", "spread", "depth", "funding_divergence"}
	for _, gateName := range expectedGates {
		if gateResult, exists := result.GateResults[gateName]; !exists {
			t.Errorf("Expected gate %s to exist", gateName)
		} else if !gateResult.Passed {
			t.Errorf("Expected gate %s to pass, got: %s", gateName, gateResult.Description)
		}
	}
}

func TestEntryGateEvaluator_ScoreGateFails(t *testing.T) {
	evaluator := createTestEvaluator(2.0, 30.0, 150000.0, true, 2.5)

	result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 70.0, 8.0, "TRENDING") // Score below 75
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Passed {
		t.Error("Expected gates to fail due to low score")
	}

	scoreGate := result.GateResults["composite_score"]
	if scoreGate.Passed {
		t.Error("Expected score gate to fail")
	}

	if len(result.FailureReasons) == 0 {
		t.Error("Expected failure reasons to be present")
	}
}

func TestEntryGateEvaluator_VADRGateFails(t *testing.T) {
	evaluator := createTestEvaluator(1.5, 30.0, 150000.0, true, 2.5) // VADR below 1.8

	result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Passed {
		t.Error("Expected gates to fail due to low VADR")
	}

	vadrGate := result.GateResults["vadr"]
	if vadrGate.Passed {
		t.Errorf("Expected VADR gate to fail, got: %s", vadrGate.Description)
	}
}

func TestEntryGateEvaluator_SpreadGateFails(t *testing.T) {
	evaluator := createTestEvaluator(2.0, 60.0, 150000.0, true, 2.5) // Spread above 50bps

	result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Passed {
		t.Error("Expected gates to fail due to wide spread")
	}

	spreadGate := result.GateResults["spread"]
	if spreadGate.Passed {
		t.Errorf("Expected spread gate to fail, got: %s", spreadGate.Description)
	}
}

func TestEntryGateEvaluator_DepthGateFails(t *testing.T) {
	evaluator := createTestEvaluator(2.0, 30.0, 50000.0, true, 2.5) // Depth below $100k

	result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Passed {
		t.Error("Expected gates to fail due to insufficient depth")
	}

	depthGate := result.GateResults["depth"]
	if depthGate.Passed {
		t.Errorf("Expected depth gate to fail, got: %s", depthGate.Description)
	}
}

func TestEntryGateEvaluator_FundingGateFails(t *testing.T) {
	evaluator := createTestEvaluator(2.0, 30.0, 150000.0, false, 1.5) // No significant funding divergence

	result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Passed {
		t.Error("Expected gates to fail due to insufficient funding divergence")
	}

	fundingGate := result.GateResults["funding_divergence"]
	if fundingGate.Passed {
		t.Errorf("Expected funding gate to fail, got: %s", fundingGate.Description)
	}
}

func TestEntryGateEvaluator_OptionalGatesHandleMissingData(t *testing.T) {
	evaluator := &EntryGateEvaluator{
		microEvaluator: &mockMicroEvaluator{
			vadr:      2.0,
			spreadBps: 30.0,
			depthUSD:  150000.0,
		},
		fundingProvider: &mockFundingProvider{
			hasSignificant: true,
			maxDivergence:  2.5,
			venue:          "binance",
			zScore:         2.5,
		},
		oiProvider: &mockOIProvider{
			err: fmt.Errorf("OI data unavailable"),
		},
		etfProvider: &mockETFProvider{
			err: fmt.Errorf("ETF data unavailable"),
		},
		config: DefaultEntryGateConfig(),
	}

	result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should still pass because OI and ETF are optional
	if !result.Passed {
		t.Errorf("Expected gates to pass with missing optional data, got failures: %v", result.FailureReasons)
	}

	// Check that optional gates defaulted to pass
	if oiGate, exists := result.GateResults["oi_residual"]; exists {
		if !oiGate.Passed {
			t.Error("Expected OI gate to pass when data unavailable (optional)")
		}
	}

	if etfGate, exists := result.GateResults["etf_flows"]; exists {
		if !etfGate.Passed {
			t.Error("Expected ETF gate to pass when data unavailable (optional)")
		}
	}
}

func TestEntryGateEvaluator_FundingDataUnavailable(t *testing.T) {
	evaluator := &EntryGateEvaluator{
		microEvaluator: &mockMicroEvaluator{
			vadr:      2.0,
			spreadBps: 30.0,
			depthUSD:  150000.0,
		},
		fundingProvider: &mockFundingProvider{
			err: fmt.Errorf("funding data unavailable"),
		},
		oiProvider: &mockOIProvider{
			oiResidual: 1500000.0,
		},
		etfProvider: &mockETFProvider{
			flowTint: 0.5,
			etfList:  []string{"IBIT", "GBTC"},
		},
		config: DefaultEntryGateConfig(),
	}

	result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should fail because funding divergence is required
	if result.Passed {
		t.Error("Expected gates to fail when required funding data unavailable")
	}

	fundingGate := result.GateResults["funding_divergence"]
	if fundingGate.Passed {
		t.Error("Expected funding gate to fail when data unavailable")
	}

	if len(result.FailureReasons) == 0 {
		t.Error("Expected failure reason for missing funding data")
	}
}

func TestEntryGateEvaluator_Summary(t *testing.T) {
	evaluator := createTestEvaluator(2.0, 30.0, 150000.0, true, 2.5)

	result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	summary := result.GetGateSummary()
	if summary == "" {
		t.Error("Expected non-empty summary")
	}

	if !result.Passed && !strings.HasPrefix(summary, "❌") {
		t.Error("Expected failure summary to start with ❌")
	}

	if result.Passed && !strings.HasPrefix(summary, "✅") {
		t.Error("Expected success summary to start with ✅")
	}

	detailedReport := result.GetDetailedReport()
	if detailedReport == "" {
		t.Error("Expected non-empty detailed report")
	}

	if !contains(detailedReport, result.Symbol) {
		t.Error("Expected detailed report to contain symbol")
	}
}

func TestEntryGateEvaluator_MultipleFailures(t *testing.T) {
	// Create evaluator with multiple failing conditions
	evaluator := createTestEvaluator(1.0, 80.0, 30000.0, false, 1.0) // Multiple failures

	result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 60.0, 8.0, "TRENDING") // Score also fails
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Passed {
		t.Error("Expected gates to fail with multiple conditions failing")
	}

	// Should have multiple failure reasons
	expectedFailures := 4 // score, vadr, spread, depth, funding
	if len(result.FailureReasons) < expectedFailures {
		t.Errorf("Expected at least %d failure reasons, got %d: %v",
			expectedFailures, len(result.FailureReasons), result.FailureReasons)
	}

	// Check that no critical gates passed
	passedCount := len(result.PassedGates)
	expectedPassed := 7 // OI, ETF, and some new gates might pass with default good values
	if passedCount > expectedPassed {
		t.Errorf("Expected at most %d gates to pass, got %d: %v", expectedPassed, passedCount, result.PassedGates)
	}
}

// Test new gates - movement threshold by regime
func TestEntryGateEvaluator_MovementThresholdGates(t *testing.T) {
	evaluator := createTestEvaluator(2.0, 30.0, 150000.0, true, 2.5)

	tests := []struct {
		name         string
		regime       string
		priceChange  float64
		shouldPass   bool
		expectedGate string
	}{
		{"TRENDING - sufficient movement", "TRENDING", 3.0, true, "movement_threshold"},
		{"TRENDING - insufficient movement", "TRENDING", 2.0, false, "movement_threshold"},
		{"CHOP - sufficient movement", "CHOP", 3.5, true, "movement_threshold"},
		{"CHOP - insufficient movement", "CHOP", 2.5, false, "movement_threshold"},
		{"HIGH_VOL - sufficient movement", "HIGH_VOL", 4.5, true, "movement_threshold"},
		{"HIGH_VOL - insufficient movement", "HIGH_VOL", 3.5, false, "movement_threshold"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, tt.priceChange, tt.regime)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			movementGate := result.GateResults[tt.expectedGate]
			if movementGate == nil {
				t.Fatalf("Expected movement threshold gate to exist")
			}

			if movementGate.Passed != tt.shouldPass {
				t.Errorf("Expected movement gate pass=%v, got pass=%v for regime %s with change %.1f%%",
					tt.shouldPass, movementGate.Passed, tt.regime, tt.priceChange)
			}
		})
	}
}

// Test volume surge gate
func TestEntryGateEvaluator_VolumeSurgeGate(t *testing.T) {
	tests := []struct {
		name       string
		vadr       float64
		barCount   int
		shouldPass bool
	}{
		{"Sufficient VADR with enough bars", 2.0, 25, true},
		{"Insufficient VADR", 1.5, 25, false},
		{"Sufficient VADR but insufficient bars", 2.0, 15, false},
		{"Freeze case - insufficient bars", 1.8, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := createTestEvaluatorWithBarCount(tt.vadr, 30.0, 150000.0, true, 2.5, tt.barCount)

			result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			volumeGate := result.GateResults["volume_surge"]
			if volumeGate == nil {
				t.Fatalf("Expected volume surge gate to exist")
			}

			if volumeGate.Passed != tt.shouldPass {
				t.Errorf("Expected volume surge gate pass=%v, got pass=%v for VADR=%.1f, bars=%d",
					tt.shouldPass, volumeGate.Passed, tt.vadr, tt.barCount)
			}
		})
	}
}

// Test liquidity gate
func TestEntryGateEvaluator_LiquidityGate(t *testing.T) {
	tests := []struct {
		name           string
		dailyVolumeUSD float64
		shouldPass     bool
	}{
		{"Sufficient liquidity", 750000.0, true},
		{"Marginal liquidity", 500000.0, true},
		{"Insufficient liquidity", 300000.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := createTestEvaluatorWithVolume(2.0, 30.0, 150000.0, true, 2.5, tt.dailyVolumeUSD)

			result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			liquidityGate := result.GateResults["liquidity"]
			if liquidityGate == nil {
				t.Fatalf("Expected liquidity gate to exist")
			}

			if liquidityGate.Passed != tt.shouldPass {
				t.Errorf("Expected liquidity gate pass=%v, got pass=%v for daily volume $%.0f",
					tt.shouldPass, liquidityGate.Passed, tt.dailyVolumeUSD)
			}
		})
	}
}

// Test trend quality gate
func TestEntryGateEvaluator_TrendQualityGate(t *testing.T) {
	tests := []struct {
		name       string
		adx        float64
		hurst      float64
		shouldPass bool
	}{
		{"Strong ADX", 30.0, 0.45, true},
		{"Strong Hurst", 20.0, 0.60, true},
		{"Both strong", 28.0, 0.58, true},
		{"Both weak", 20.0, 0.45, false},
		{"ADX marginal", 25.0, 0.45, true},
		{"Hurst marginal", 20.0, 0.55, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := createTestEvaluatorWithTrendQuality(2.0, 30.0, 150000.0, true, 2.5, tt.adx, tt.hurst)

			result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			trendGate := result.GateResults["trend_quality"]
			if trendGate == nil {
				t.Fatalf("Expected trend quality gate to exist")
			}

			if trendGate.Passed != tt.shouldPass {
				t.Errorf("Expected trend quality gate pass=%v, got pass=%v for ADX=%.1f, Hurst=%.2f",
					tt.shouldPass, trendGate.Passed, tt.adx, tt.hurst)
			}
		})
	}
}

// Test freshness gate
func TestEntryGateEvaluator_FreshnessGate(t *testing.T) {
	tests := []struct {
		name               string
		barsFromTrigger    int
		lateFillSecondsAgo int
		shouldPass         bool
	}{
		{"Fresh data, quick fill", 1, 10, true},
		{"Fresh data, slow fill", 1, 35, false},
		{"Stale data, quick fill", 3, 10, false},
		{"Edge case - exactly 2 bars", 2, 25, true},
		{"Edge case - exactly 30 seconds", 1, 30, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := createTestEvaluatorWithFreshness(2.0, 30.0, 150000.0, true, 2.5,
				tt.barsFromTrigger, time.Duration(tt.lateFillSecondsAgo)*time.Second)

			result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 8.0, "TRENDING")
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			freshnessGate := result.GateResults["freshness"]
			if freshnessGate == nil {
				t.Fatalf("Expected freshness gate to exist")
			}

			if freshnessGate.Passed != tt.shouldPass {
				t.Errorf("Expected freshness gate pass=%v, got pass=%v for bars=%d, fill_delay=%ds",
					tt.shouldPass, freshnessGate.Passed, tt.barsFromTrigger, tt.lateFillSecondsAgo)
			}
		})
	}
}

// Test all new gates together
func TestEntryGateEvaluator_AllNewGatesPass(t *testing.T) {
	evaluator := createTestEvaluatorWithAllNewGates(2.0, 30.0, 150000.0, true, 2.5,
		25, 750000.0, 28.0, 0.58, 1, 10*time.Second)

	result, err := evaluator.EvaluateEntry(context.Background(), "BTCUSD", 80.0, 3.5, "TRENDING")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.Passed {
		t.Errorf("Expected all gates to pass, got failures: %v", result.FailureReasons)
	}

	// Check that all new gates exist and pass
	newGates := []string{"movement_threshold", "volume_surge", "liquidity", "trend_quality", "freshness"}
	for _, gateName := range newGates {
		if gateResult, exists := result.GateResults[gateName]; !exists {
			t.Errorf("Expected gate %s to exist", gateName)
		} else if !gateResult.Passed {
			t.Errorf("Expected gate %s to pass, got: %s", gateName, gateResult.Description)
		}
	}
}

// Test helper functions for the new tests
func createTestEvaluatorWithBarCount(vadr, spreadBps, depthUSD float64, hasSignificantFunding bool, fundingDiv float64, barCount int) *EntryGateEvaluator {
	return &EntryGateEvaluator{
		microEvaluator: &mockMicroEvaluatorExtended{
			vadr:      vadr,
			spreadBps: spreadBps,
			depthUSD:  depthUSD,
			barCount:  barCount,
		},
		fundingProvider: &mockFundingProvider{
			hasSignificant: hasSignificantFunding,
			maxDivergence:  fundingDiv,
			venue:          "binance",
			zScore:         fundingDiv,
		},
		oiProvider: &mockOIProvider{
			oiResidual: 1500000.0,
		},
		etfProvider: &mockETFProvider{
			flowTint: 0.5,
			etfList:  []string{"IBIT", "GBTC"},
		},
		config: DefaultEntryGateConfig(),
	}
}

func createTestEvaluatorWithVolume(vadr, spreadBps, depthUSD float64, hasSignificantFunding bool, fundingDiv, dailyVolumeUSD float64) *EntryGateEvaluator {
	return &EntryGateEvaluator{
		microEvaluator: &mockMicroEvaluatorExtended{
			vadr:           vadr,
			spreadBps:      spreadBps,
			depthUSD:       depthUSD,
			dailyVolumeUSD: dailyVolumeUSD,
			barCount:       25,
		},
		fundingProvider: &mockFundingProvider{
			hasSignificant: hasSignificantFunding,
			maxDivergence:  fundingDiv,
			venue:          "binance",
			zScore:         fundingDiv,
		},
		oiProvider: &mockOIProvider{
			oiResidual: 1500000.0,
		},
		etfProvider: &mockETFProvider{
			flowTint: 0.5,
			etfList:  []string{"IBIT", "GBTC"},
		},
		config: DefaultEntryGateConfig(),
	}
}

func createTestEvaluatorWithTrendQuality(vadr, spreadBps, depthUSD float64, hasSignificantFunding bool, fundingDiv, adx, hurst float64) *EntryGateEvaluator {
	return &EntryGateEvaluator{
		microEvaluator: &mockMicroEvaluatorExtended{
			vadr:           vadr,
			spreadBps:      spreadBps,
			depthUSD:       depthUSD,
			dailyVolumeUSD: 750000.0,
			barCount:       25,
			adx:            adx,
			hurst:          hurst,
		},
		fundingProvider: &mockFundingProvider{
			hasSignificant: hasSignificantFunding,
			maxDivergence:  fundingDiv,
			venue:          "binance",
			zScore:         fundingDiv,
		},
		oiProvider: &mockOIProvider{
			oiResidual: 1500000.0,
		},
		etfProvider: &mockETFProvider{
			flowTint: 0.5,
			etfList:  []string{"IBIT", "GBTC"},
		},
		config: DefaultEntryGateConfig(),
	}
}

func createTestEvaluatorWithFreshness(vadr, spreadBps, depthUSD float64, hasSignificantFunding bool, fundingDiv float64,
	barsFromTrigger int, lateFillDelay time.Duration) *EntryGateEvaluator {
	return &EntryGateEvaluator{
		microEvaluator: &mockMicroEvaluatorExtended{
			vadr:            vadr,
			spreadBps:       spreadBps,
			depthUSD:        depthUSD,
			dailyVolumeUSD:  750000.0,
			barCount:        25,
			adx:             28.0,
			hurst:           0.58,
			barsFromTrigger: barsFromTrigger,
			lateFillDelay:   lateFillDelay,
		},
		fundingProvider: &mockFundingProvider{
			hasSignificant: hasSignificantFunding,
			maxDivergence:  fundingDiv,
			venue:          "binance",
			zScore:         fundingDiv,
		},
		oiProvider: &mockOIProvider{
			oiResidual: 1500000.0,
		},
		etfProvider: &mockETFProvider{
			flowTint: 0.5,
			etfList:  []string{"IBIT", "GBTC"},
		},
		config: DefaultEntryGateConfig(),
	}
}

func createTestEvaluatorWithAllNewGates(vadr, spreadBps, depthUSD float64, hasSignificantFunding bool, fundingDiv float64,
	barCount int, dailyVolumeUSD, adx, hurst float64, barsFromTrigger int, lateFillDelay time.Duration) *EntryGateEvaluator {
	return &EntryGateEvaluator{
		microEvaluator: &mockMicroEvaluatorExtended{
			vadr:            vadr,
			spreadBps:       spreadBps,
			depthUSD:        depthUSD,
			dailyVolumeUSD:  dailyVolumeUSD,
			barCount:        barCount,
			adx:             adx,
			hurst:           hurst,
			barsFromTrigger: barsFromTrigger,
			lateFillDelay:   lateFillDelay,
		},
		fundingProvider: &mockFundingProvider{
			hasSignificant: hasSignificantFunding,
			maxDivergence:  fundingDiv,
			venue:          "binance",
			zScore:         fundingDiv,
		},
		oiProvider: &mockOIProvider{
			oiResidual: 1500000.0,
		},
		etfProvider: &mockETFProvider{
			flowTint: 0.5,
			etfList:  []string{"IBIT", "GBTC"},
		},
		config: DefaultEntryGateConfig(),
	}
}

// Extended mock microstructure evaluator with new fields
type mockMicroEvaluatorExtended struct {
	vadr            float64
	spreadBps       float64
	depthUSD        float64
	dailyVolumeUSD  float64
	barCount        int
	adx             float64
	hurst           float64
	barsFromTrigger int
	lateFillDelay   time.Duration
	err             error
}

func (m *mockMicroEvaluatorExtended) EvaluateSnapshot(symbol string) (microstructure.EvaluationResult, error) {
	if m.err != nil {
		return microstructure.EvaluationResult{}, m.err
	}
	return microstructure.EvaluationResult{
		VADR:      m.vadr,
		SpreadBps: m.spreadBps,
		DepthUSD:  m.depthUSD,
		Healthy:   m.vadr >= 1.75 && m.spreadBps < 50.0 && m.depthUSD > 100000.0,
	}, nil
}

// Implement other required methods for microstructure.Evaluator interface
func (m *mockMicroEvaluatorExtended) EvaluateGates(ctx context.Context, symbol, venue string, orderbook *microstructure.OrderBookSnapshot, adv float64) (*microstructure.GateReport, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

func (m *mockMicroEvaluatorExtended) GetLiquidityTier(adv float64) *microstructure.LiquidityTier {
	return nil
}

func (m *mockMicroEvaluatorExtended) UpdateVenueHealth(venue string, health microstructure.VenueHealthStatus) error {
	return fmt.Errorf("not implemented in mock")
}

func (m *mockMicroEvaluatorExtended) GetVenueHealth(venue string) (*microstructure.VenueHealthStatus, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != substr &&
		(len(s) >= len(substr)) && s[0:len(substr)] == substr ||
		(len(s) > len(substr) && containsAt(s, substr, 1))
}

func containsAt(s, substr string, start int) bool {
	if start >= len(s) {
		return false
	}
	if start+len(substr) > len(s) {
		return containsAt(s, substr, start+1)
	}
	if s[start:start+len(substr)] == substr {
		return true
	}
	return containsAt(s, substr, start+1)
}
