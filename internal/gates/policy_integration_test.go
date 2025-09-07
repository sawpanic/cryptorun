package gates

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/derivs"
	"github.com/sawpanic/cryptorun/internal/microstructure"
)

// Mock providers for policy integration testing
type mockPolicyFundingProvider struct{}

func (m *mockPolicyFundingProvider) GetFundingSnapshot(ctx context.Context, symbol string) (*derivs.FundingSnapshot, error) {
	return &derivs.FundingSnapshot{
		Symbol:                   symbol,
		MaxVenueDivergence:       2.5,
		FundingDivergencePresent: true,
	}, nil
}

type mockPolicyOIProvider struct{}

func (m *mockPolicyOIProvider) GetOpenInterestSnapshot(ctx context.Context, symbol string, priceChange float64) (*derivs.OpenInterestSnapshot, error) {
	return &derivs.OpenInterestSnapshot{
		Symbol:     symbol,
		OIResidual: 1500000.0,
	}, nil
}

type mockPolicyETFProvider struct{}

func (m *mockPolicyETFProvider) GetETFFlowSnapshot(ctx context.Context, symbol string) (*derivs.ETFFlowSnapshot, error) {
	return &derivs.ETFFlowSnapshot{
		Symbol:    symbol,
		FlowTint:  0.5,
		ETFList:   []string{"IBIT", "GBTC"},
	}, nil
}

type mockPolicyMicroEvaluator struct{}

func (m *mockPolicyMicroEvaluator) EvaluateSnapshot(symbol string) (microstructure.EvaluationResult, error) {
	return microstructure.EvaluationResult{
		VADR:            2.1,  // Passes most regime thresholds  
		SpreadBps:       35.0, // Good spread
		DepthUSD:        180000.0,
		DailyVolumeUSD:  750000.0,
		BarCount:        25,
		ADX:             28.0,
		Hurst:           0.58,
		BarsFromTrigger: 1,
		LateFillDelay:   10 * time.Second,
		Healthy:         true,
	}, nil
}

func TestEvaluateEntryWithPolicyMatrix_AllPass(t *testing.T) {
	evaluator := NewEntryGateEvaluator(
		&mockPolicyMicroEvaluator{},
		&mockPolicyFundingProvider{},
		&mockPolicyOIProvider{},
		&mockPolicyETFProvider{},
	)

	// Create test order book
	orderbook := &microstructure.OrderBookSnapshot{
		Symbol:    "BTCUSD",
		Venue:     "kraken",
		Timestamp: time.Now(),
		Bids: []microstructure.PriceLevel{
			{Price: 45000.0, Size: 2.5},
			{Price: 44950.0, Size: 1.8},
			{Price: 44900.0, Size: 3.2},
		},
		Asks: []microstructure.PriceLevel{
			{Price: 45100.0, Size: 2.2},
			{Price: 45150.0, Size: 1.6},
			{Price: 45200.0, Size: 2.8},
		},
		LastPrice: 45050.0,
	}

	ctx := context.Background()
	result, err := evaluator.EvaluateEntryWithPolicyMatrix(ctx, "BTCUSD", "kraken", 80.0, 5.0, "trending", 5000000.0, orderbook)
	if err != nil {
		t.Fatalf("Policy matrix evaluation failed: %v", err)
	}

	// Verify policy result was included
	if result.PolicyResult == nil {
		t.Fatal("Policy result should not be nil")
	}

	// Verify policy passed
	if !result.PolicyResult.PolicyPassed {
		t.Errorf("Policy should pass with good inputs, got violations: %v", result.PolicyResult.PolicyViolations)
	}

	// Verify recommended action
	if result.PolicyResult.RecommendedAction != "proceed" {
		t.Errorf("Expected 'proceed' action, got '%s'", result.PolicyResult.RecommendedAction)
	}

	// Verify confidence score
	if result.PolicyResult.ConfidenceScore < 0.8 {
		t.Errorf("Expected high confidence (>0.8), got %.3f", result.PolicyResult.ConfidenceScore)
	}

	// Verify policy matrix gate check
	policyGateCheck, exists := result.GateResults["policy_matrix"]
	if !exists {
		t.Fatal("Policy matrix gate check should exist")
	}

	if !policyGateCheck.Passed {
		t.Error("Policy matrix gate should pass")
	}

	// Verify overall result passed
	if !result.Passed {
		t.Errorf("Overall evaluation should pass, got failures: %v", result.FailureReasons)
	}
}

func TestEvaluateEntryWithPolicyMatrix_VenueFallback(t *testing.T) {
	evaluator := NewEntryGateEvaluator(
		&mockPolicyMicroEvaluator{},
		&mockPolicyFundingProvider{},
		&mockPolicyOIProvider{},
		&mockPolicyETFProvider{},
	)

	// Mark primary venue as unhealthy
	unhealthyStatus := &microstructure.VenueHealthStatus{
		Healthy:        false,
		RejectRate:     15.0,
		LatencyP99Ms:   3000,
		ErrorRate:      8.0,
		Recommendation: "avoid",
		UptimePercent:  85.0,
	}

	err := evaluator.policyMatrix.UpdateVenueHealth("kraken", unhealthyStatus)
	if err != nil {
		t.Fatalf("Failed to update venue health: %v", err)
	}

	// Create test order book
	orderbook := &microstructure.OrderBookSnapshot{
		Symbol:    "BTCUSD",
		Venue:     "kraken", // Will be changed to fallback venue
		Timestamp: time.Now(),
		Bids: []microstructure.PriceLevel{
			{Price: 45000.0, Size: 2.5},
		},
		Asks: []microstructure.PriceLevel{
			{Price: 45100.0, Size: 2.2},
		},
		LastPrice: 45050.0,
	}

	ctx := context.Background()
	result, err := evaluator.EvaluateEntryWithPolicyMatrix(ctx, "BTCUSD", "kraken", 80.0, 5.0, "trending", 5000000.0, orderbook)
	if err != nil {
		t.Fatalf("Policy matrix evaluation with fallback failed: %v", err)
	}

	// Verify policy result
	if result.PolicyResult == nil {
		t.Fatal("Policy result should not be nil")
	}

	// Verify fallback was attempted
	if result.PolicyResult.FallbacksAttempted == 0 {
		t.Error("Should have attempted venue fallback")
	}

	// Verify fallback venue was selected
	if result.PolicyResult.FallbackVenue == "" {
		t.Error("Should have selected a fallback venue")
	}

	if result.PolicyResult.FallbackVenue == "kraken" {
		t.Error("Fallback venue should not be the same as failed venue")
	}

	// Verify venue health check results
	if result.PolicyResult.VenueHealthCheck == nil {
		t.Fatal("Venue health check should not be nil")
	}

	if result.PolicyResult.VenueHealthCheck.VenueHealthy {
		t.Error("Primary venue should be unhealthy")
	}

	if !result.PolicyResult.VenueHealthCheck.FallbackRequired {
		t.Error("Fallback should be required")
	}
}

func TestEvaluateEntryWithPolicyMatrix_DepegGuard(t *testing.T) {
	evaluator := NewEntryGateEvaluator(
		&mockPolicyMicroEvaluator{},
		&mockPolicyFundingProvider{},
		&mockPolicyOIProvider{},
		&mockPolicyETFProvider{},
	)

	// Manually add depeg alert to policy matrix with higher threshold breach
	evaluator.policyMatrix.depegAlerts["USDT"] = DepegAlert{
		Stablecoin:        "USDT", 
		CurrentPrice:      0.985, // 1.5% depeg
		DepegBps:          150.0, // Above the 100 bps default threshold
		Timestamp:         time.Now(),
		AlertLevel:        "warning",
		RecommendedAction: "monitor",
	}

	// Create test order book for stablecoin pair
	orderbook := &microstructure.OrderBookSnapshot{
		Symbol:    "BTCUSDT", // Stablecoin pair
		Venue:     "kraken",
		Timestamp: time.Now(),
		Bids: []microstructure.PriceLevel{
			{Price: 45000.0, Size: 2.5},
		},
		Asks: []microstructure.PriceLevel{
			{Price: 45100.0, Size: 2.2},
		},
		LastPrice: 45050.0,
	}

	ctx := context.Background()
	result, err := evaluator.EvaluateEntryWithPolicyMatrix(ctx, "BTCUSDT", "kraken", 80.0, 5.0, "trending", 5000000.0, orderbook)
	if err != nil {
		t.Fatalf("Policy matrix evaluation with depeg failed: %v", err)
	}

	// Verify policy result includes depeg check
	if result.PolicyResult == nil || result.PolicyResult.DepegCheck == nil {
		t.Fatal("Depeg check should not be nil for stablecoin pair")
	}

	if !result.PolicyResult.DepegCheck.DepegDetected {
		t.Error("Depeg should be detected")
	}

	if len(result.PolicyResult.DepegCheck.AffectedStablecoins) == 0 {
		t.Error("Should list affected stablecoins")
	}

	// Verify depeg gate check was created
	depegGateCheck, exists := result.GateResults["depeg_guard"]
	if !exists {
		t.Fatal("Depeg guard gate check should exist")
	}

	if depegGateCheck.Passed {
		t.Error("Depeg guard should fail when depeg is detected")
	}
}

func TestPolicyMatrixIntegrationDiagnostics(t *testing.T) {
	evaluator := NewEntryGateEvaluator(
		&mockPolicyMicroEvaluator{},
		&mockPolicyFundingProvider{},
		&mockPolicyOIProvider{},
		&mockPolicyETFProvider{},
	)

	// Test policy matrix status
	status := evaluator.policyMatrix.GetPolicyStatus()
	if status == nil {
		t.Fatal("Policy status should not be nil")
	}

	// Verify expected fields
	expectedFields := []string{
		"venue_fallback_enabled",
		"depeg_guard_enabled", 
		"risk_off_toggles_enabled",
		"healthy_venues",
		"total_venues",
		"risk_off_mode",
	}

	for _, field := range expectedFields {
		if _, exists := status[field]; !exists {
			t.Errorf("Policy status missing field: %s", field)
		}
	}

	// Test unified evaluator diagnostics
	unifiedDiagnostics := evaluator.unifiedEvaluator.GetDiagnostics()
	if unifiedDiagnostics == nil {
		t.Fatal("Unified diagnostics should not be nil")
	}

	if len(unifiedDiagnostics) == 0 {
		t.Error("Unified diagnostics should contain information")
	}
}

func TestPolicyMatrixConfiguration(t *testing.T) {
	// Test default configuration
	config := DefaultPolicyMatrixConfig()
	if config == nil {
		t.Fatal("Default config should not be nil")
	}

	// Verify venue fallback is enabled
	if !config.VenueFallbackEnabled {
		t.Error("Venue fallback should be enabled by default")
	}

	// Verify depeg guard is enabled
	if !config.DepegGuardEnabled {
		t.Error("Depeg guard should be enabled by default")
	}

	// Verify risk-off toggles are enabled
	if !config.RiskOffTogglesEnabled {
		t.Error("Risk-off toggles should be enabled by default")
	}

	// Verify primary venues are set
	if len(config.PrimaryVenues) == 0 {
		t.Error("Should have primary venues configured")
	}

	// Verify Kraken is preferred (first in list)
	if config.PrimaryVenues[0] != "kraken" {
		t.Error("Kraken should be the first preferred venue")
	}

	// Verify reasonable thresholds
	if config.DepegThresholdBps <= 0 || config.DepegThresholdBps > 1000 {
		t.Errorf("Depeg threshold should be reasonable, got %.1f bps", config.DepegThresholdBps)
	}

	if config.MinHealthyVenues <= 0 {
		t.Errorf("Should require at least 1 healthy venue, got %d", config.MinHealthyVenues)
	}
}