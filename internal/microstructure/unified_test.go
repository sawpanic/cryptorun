package microstructure

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/microstructure/adapters"
)

func TestUnifiedMicrostructureEvaluator(t *testing.T) {
	config := DefaultConfig()
	evaluator := NewUnifiedMicrostructureEvaluator(config)

	if evaluator == nil {
		t.Fatal("Failed to create unified evaluator")
	}

	// Test supported venues
	venues := evaluator.GetSupportedVenues()
	if len(venues) == 0 {
		t.Error("No supported venues configured")
	}

	// Test venue support check
	if !evaluator.IsVenueSupported("binance") {
		t.Error("Binance should be supported")
	}

	if evaluator.IsVenueSupported("dexscreener") {
		t.Error("Aggregators should not be supported")
	}
}

func TestUnifiedEvaluation(t *testing.T) {
	config := DefaultConfig()
	evaluator := NewUnifiedMicrostructureEvaluator(config)

	// Create test data
	orderbook := &OrderBookSnapshot{
		Symbol:    "BTCUSD",
		Venue:     "binance",
		Timestamp: time.Now(),
		Bids: []PriceLevel{
			{Price: 45000.0, Size: 2.5},
			{Price: 44950.0, Size: 1.8},
			{Price: 44900.0, Size: 3.2},
		},
		Asks: []PriceLevel{
			{Price: 45100.0, Size: 2.2},
			{Price: 45150.0, Size: 1.6},
			{Price: 45200.0, Size: 2.8},
		},
		LastPrice: 45050.0,
	}

	vadrInput := &VADRInput{
		High:         45200.0,
		Low:          44800.0,
		Volume:       150.0,
		ADV:          5000000.0,
		CurrentPrice: 45050.0,
	}

	tier := &LiquidityTier{
		Name:         "tier1",
		ADVMin:       5000000,
		ADVMax:       1e12,
		DepthMinUSD:  150000,
		SpreadCapBps: 25,
		VADRMinimum:  1.85,
	}

	// Test unified evaluation
	ctx := context.Background()
	result, err := evaluator.EvaluateUnified(ctx, "BTCUSD", "binance", orderbook, vadrInput, tier)
	if err != nil {
		t.Fatalf("Unified evaluation failed: %v", err)
	}

	// Verify result structure
	if result == nil {
		t.Fatal("Result is nil")
	}

	if result.Venue != "binance" {
		t.Errorf("Expected venue 'binance', got '%s'", result.Venue)
	}

	if result.Attribution == nil {
		t.Error("Attribution data is missing")
	}

	if result.Attribution.ProcessingPath != "unified_evaluator" {
		t.Errorf("Expected processing path 'unified_evaluator', got '%s'", result.Attribution.ProcessingPath)
	}

	// Verify gate results
	expectedGates := []string{"spread", "depth", "vadr"}
	for _, gateName := range expectedGates {
		gateResult, exists := result.GateResults[gateName]
		if !exists {
			t.Errorf("Gate result missing for: %s", gateName)
			continue
		}

		if gateResult.Name != gateName {
			t.Errorf("Gate name mismatch: expected %s, got %s", gateName, gateResult.Name)
		}

		if gateResult.Description == "" {
			t.Errorf("Gate description is empty for: %s", gateName)
		}
	}

	// Verify venue policy was evaluated
	if result.VenuePolicy == nil {
		t.Error("Venue policy result is missing")
	} else {
		if !result.VenuePolicy.Approved {
			t.Error("Venue policy should approve binance for BTCUSD")
		}
	}
}

func TestSpreadCalculation(t *testing.T) {
	config := DefaultConfig()
	evaluator := NewUnifiedMicrostructureEvaluator(config)

	orderbook := &OrderBookSnapshot{
		Symbol:    "BTCUSD",
		Venue:     "binance",
		Timestamp: time.Now(),
		Bids: []PriceLevel{
			{Price: 45000.0, Size: 2.5},
		},
		Asks: []PriceLevel{
			{Price: 45100.0, Size: 2.2},
		},
		LastPrice: 45050.0,
	}

	spreadResult, err := evaluator.Spread(orderbook)
	if err != nil {
		t.Fatalf("Spread calculation failed: %v", err)
	}

	expectedSpreadBps := (45100.0 - 45000.0) / ((45100.0 + 45000.0) / 2.0) * 10000.0
	if abs(spreadResult.Current.SpreadBps-expectedSpreadBps) > 0.1 {
		t.Errorf("Spread calculation incorrect: expected %.2f bps, got %.2f bps",
			expectedSpreadBps, spreadResult.Current.SpreadBps)
	}
}

func TestDepthCalculation(t *testing.T) {
	config := DefaultConfig()
	evaluator := NewUnifiedMicrostructureEvaluator(config)

	orderbook := &OrderBookSnapshot{
		Symbol:    "BTCUSD",
		Venue:     "binance",
		Timestamp: time.Now(),
		Bids: []PriceLevel{
			{Price: 45000.0, Size: 2.0},  // Within 2% bound (44149)
			{Price: 44900.0, Size: 1.0},  // Within bound
			{Price: 44000.0, Size: 5.0},  // Outside bound (should be excluded)
		},
		Asks: []PriceLevel{
			{Price: 45100.0, Size: 1.5},  // Within 2% bound (45951)
			{Price: 45200.0, Size: 1.0},  // Within bound
			{Price: 46500.0, Size: 3.0},  // Outside bound (should be excluded)
		},
		LastPrice: 45050.0,
	}

	depthResult, err := evaluator.Depth(orderbook)
	if err != nil {
		t.Fatalf("Depth calculation failed: %v", err)
	}

	// Check that only in-bounds orders are counted
	expectedBidDepth := 45000.0*2.0 + 44900.0*1.0 // = 90000 + 44900 = 134900
	expectedAskDepth := 45100.0*1.5 + 45200.0*1.0 // = 67650 + 45200 = 112850

	if abs(depthResult.BidDepthUSD-expectedBidDepth) > 1.0 {
		t.Errorf("Bid depth incorrect: expected %.0f, got %.0f",
			expectedBidDepth, depthResult.BidDepthUSD)
	}

	if abs(depthResult.AskDepthUSD-expectedAskDepth) > 1.0 {
		t.Errorf("Ask depth incorrect: expected %.0f, got %.0f",
			expectedAskDepth, depthResult.AskDepthUSD)
	}
}

func TestVADRCalculation(t *testing.T) {
	config := DefaultConfig()
	evaluator := NewUnifiedMicrostructureEvaluator(config)

	vadrInput := &VADRInput{
		High:         45500.0,
		Low:          44500.0,
		Volume:       100.0,
		ADV:          5000000.0,
		CurrentPrice: 45000.0,
	}

	tier := &LiquidityTier{
		VADRMinimum: 1.85,
	}

	vadrResult, err := evaluator.VADR(vadrInput, tier)
	if err != nil {
		t.Fatalf("VADR calculation failed: %v", err)
	}

	if vadrResult.Current <= 0 {
		t.Error("VADR should be positive")
	}

	if vadrResult.TierMinimum != tier.VADRMinimum {
		t.Errorf("Tier minimum mismatch: expected %.2f, got %.2f",
			tier.VADRMinimum, vadrResult.TierMinimum)
	}
}

func TestAggregatorBanEnforcement(t *testing.T) {
	config := DefaultConfig()
	evaluator := NewUnifiedMicrostructureEvaluator(config)
	
	// Set to non-strict mode for testing
	evaluator.AggregateGuard = adapters.NewRuntimeAggregatorGuard(false)

	// Test with banned aggregator
	orderbook := &OrderBookSnapshot{
		Symbol:    "BTCUSD",
		Venue:     "dexscreener", // This should be banned
		Timestamp: time.Now(),
		Bids:      []PriceLevel{{Price: 45000.0, Size: 1.0}},
		Asks:      []PriceLevel{{Price: 45100.0, Size: 1.0}},
		LastPrice: 45050.0,
	}

	_, err := evaluator.Spread(orderbook)
	if err == nil {
		t.Error("Expected error for banned aggregator, but got none")
	}

	// Test with allowed exchange
	orderbook.Venue = "binance"
	_, err = evaluator.Spread(orderbook)
	if err != nil {
		t.Errorf("Allowed exchange should not error: %v", err)
	}
}

func TestLiquidityTierSelection(t *testing.T) {
	config := DefaultConfig()
	evaluator := NewUnifiedMicrostructureEvaluator(config)

	testCases := []struct {
		adv          float64
		expectedTier string
	}{
		{6000000.0, "tier1"},  // High ADV
		{2000000.0, "tier2"},  // Medium ADV
		{500000.0, "tier3"},   // Low ADV
		{50000.0, "tier3"},    // Very low ADV (should get lowest tier)
	}

	for _, tc := range testCases {
		tier := evaluator.GetLiquidityTier(tc.adv)
		if tier == nil {
			t.Errorf("No tier found for ADV %.0f", tc.adv)
			continue
		}

		if tier.Name != tc.expectedTier {
			t.Errorf("ADV %.0f: expected tier %s, got %s",
				tc.adv, tc.expectedTier, tier.Name)
		}
	}
}

func TestQualityScoreCalculation(t *testing.T) {
	config := DefaultConfig()
	evaluator := NewUnifiedMicrostructureEvaluator(config)

	// Create a result with known quality characteristics
	result := &UnifiedResult{
		Spread: &SpreadResult{
			DataQuality: "excellent",
		},
		VADR: &VADRResult{
			HistoryCount: 250, // Excellent history
		},
		Attribution: &AttributionData{
			DataAge: 2 * time.Second, // Fresh data
		},
	}

	qualityScore := evaluator.calculateQualityScore(result)

	// Should be high quality
	if qualityScore < 0.8 {
		t.Errorf("Expected high quality score (>0.8), got %.3f", qualityScore)
	}

	// Test with poor quality data
	result.Spread.DataQuality = "sparse"
	result.VADR.HistoryCount = 10 // Poor history
	result.Attribution.DataAge = 30 * time.Second // Stale data

	qualityScore = evaluator.calculateQualityScore(result)

	// Should be low quality
	if qualityScore > 0.6 {
		t.Errorf("Expected low quality score (<0.6), got %.3f", qualityScore)
	}
}

func TestDiagnostics(t *testing.T) {
	config := DefaultConfig()
	evaluator := NewUnifiedMicrostructureEvaluator(config)

	diagnostics := evaluator.GetDiagnostics()
	if diagnostics == nil {
		t.Fatal("Diagnostics should not be nil")
	}

	expectedFields := []string{
		"supported_venues",
		"aggregator_guard",
		"vadr_history_stats",
		"spread_window_sec",
		"depth_window_sec",
		"max_data_age_sec",
		"liquidity_tiers",
	}

	for _, field := range expectedFields {
		if _, exists := diagnostics[field]; !exists {
			t.Errorf("Diagnostics missing field: %s", field)
		}
	}
}

// Note: abs helper function is defined in tiered_gates_test.go