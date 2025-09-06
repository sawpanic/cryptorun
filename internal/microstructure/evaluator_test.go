package microstructure

import (
	"context"
	"testing"
	"time"
)

// TestMicrostructureEvaluator tests the main evaluator with synthetic order books
func TestMicrostructureEvaluator(t *testing.T) {
	evaluator := NewMicrostructureEvaluator(DefaultConfig())
	ctx := context.Background()

	tests := []struct {
		name        string
		symbol      string
		venue       string
		orderbook   *OrderBookSnapshot
		adv         float64
		expectPass  bool
		expectTier  string
		description string
	}{
		{
			name:   "tier1_btc_passes_all_gates",
			symbol: "BTC-USD",
			venue:  "binance",
			orderbook: createTier1OrderBook("BTC-USD", 50000.0),
			adv:    10000000, // $10M ADV
			expectPass: true,
			expectTier: "tier1",
			description: "BTC with excellent liquidity should pass all Tier 1 gates",
		},
		{
			name:   "tier2_eth_passes_gates",
			symbol: "ETH-USD", 
			venue:  "coinbase",
			orderbook: createTier2OrderBook("ETH-USD", 3000.0),
			adv:    3000000, // $3M ADV
			expectPass: true,
			expectTier: "tier2",
			description: "ETH with good liquidity should pass Tier 2 gates",
		},
		{
			name:   "tier3_smallcap_marginal",
			symbol: "SMALL-USD",
			venue:  "okx",
			orderbook: createTier3OrderBook("SMALL-USD", 5.0),
			adv:    500000, // $500k ADV
			expectPass: true,
			expectTier: "tier3",
			description: "Small cap with minimal liquidity passes Tier 3 gates",
		},
		{
			name:   "wide_spread_fails",
			symbol: "WIDE-USD",
			venue:  "binance",
			orderbook: createWideSpreadOrderBook("WIDE-USD", 100.0),
			adv:    2000000, // $2M ADV
			expectPass: false,
			expectTier: "tier2",
			description: "Asset with excessive spread should fail gates",
		},
		{
			name:   "thin_depth_fails",
			symbol: "THIN-USD",
			venue:  "coinbase",
			orderbook: createThinDepthOrderBook("THIN-USD", 10.0),
			adv:    1500000, // $1.5M ADV
			expectPass: false,
			expectTier: "tier2",
			description: "Asset with insufficient depth should fail gates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := evaluator.EvaluateGates(ctx, tt.symbol, tt.venue, tt.orderbook, tt.adv)
			if err != nil {
				t.Fatalf("EvaluateGates failed: %v", err)
			}

			// Check tier assignment
			if report.Details.LiquidityTier != tt.expectTier {
				t.Errorf("Expected tier %s, got %s", tt.expectTier, report.Details.LiquidityTier)
			}

			// Check overall result
			if report.ExecutionFeasible != tt.expectPass {
				t.Errorf("Expected feasible=%v, got %v. Failures: %v", 
					tt.expectPass, report.ExecutionFeasible, report.FailureReasons)
			}

			// Verify individual gates align with overall result
			allGatesPass := report.DepthOK && report.SpreadOK && report.VadrOK
			if tt.expectPass && !allGatesPass {
				t.Errorf("Expected pass but gates failed: depth=%v, spread=%v, vadr=%v",
					report.DepthOK, report.SpreadOK, report.VadrOK)
			}

			t.Logf("%s: %s (tier=%s, depth=$%.0f, spread=%.1fbps, vadr=%.3f)",
				tt.description, 
				map[bool]string{true: "PASS", false: "FAIL"}[report.ExecutionFeasible],
				report.Details.LiquidityTier,
				report.Details.TotalDepthUSD,
				report.Details.SpreadBps,
				report.Details.VADRCurrent)
		})
	}
}

// TestVenueHealthIntegration tests venue health impact on gate reports
func TestVenueHealthIntegration(t *testing.T) {
	evaluator := NewMicrostructureEvaluator(DefaultConfig())
	ctx := context.Background()

	// Simulate unhealthy venue
	evaluator.healthMonitor.RecordRequest("binance", "orderbook", 3000, false, 500, "timeout")
	evaluator.healthMonitor.RecordRequest("binance", "orderbook", 2500, false, 503, "unavailable") 
	evaluator.healthMonitor.RecordRequest("binance", "orderbook", 4000, false, 429, "rate_limit")

	orderbook := createTier1OrderBook("BTC-USD", 50000.0)
	report, err := evaluator.EvaluateGates(ctx, "BTC-USD", "binance", orderbook, 10000000)
	
	if err != nil {
		t.Fatalf("EvaluateGates failed: %v", err)
	}

	// Even if microstructure gates pass, unhealthy venue should recommend size reduction
	if report.RecommendedAction != "halve_size" && report.RecommendedAction != "avoid" {
		t.Errorf("Expected venue health to recommend size reduction, got: %s", report.RecommendedAction)
	}

	venueHealth := report.Details.VenueHealth
	if venueHealth.Healthy {
		t.Errorf("Expected venue to be unhealthy after error simulation")
	}

	t.Logf("Venue health: %v, recommendation: %s (reject_rate=%.1f%%, latency=%dms, error_rate=%.1f%%)",
		venueHealth.Healthy, venueHealth.Recommendation, 
		venueHealth.RejectRate, venueHealth.LatencyP99Ms, venueHealth.ErrorRate)
}

// TestLiquidityTiers tests tier assignment logic
func TestLiquidityTiers(t *testing.T) {
	tierManager := NewLiquidityTierManager()

	testCases := []struct {
		adv        float64
		expectTier string
	}{
		{10000000, "tier1"}, // $10M
		{3000000,  "tier2"}, // $3M  
		{500000,   "tier3"}, // $500k
		{50000,    "tier3"}, // $50k (below minimum, gets tier3)
	}

	for _, tc := range testCases {
		tier, _ := tierManager.GetTierByADV(tc.adv)
		if tier.Name != tc.expectTier {
			t.Errorf("ADV $%.0f: expected tier %s, got %s", tc.adv, tc.expectTier, tier.Name)
		}
	}
}

// Synthetic order book generators

func createTier1OrderBook(symbol string, midPrice float64) *OrderBookSnapshot {
	return &OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     "binance",
		Timestamp: time.Now(),
		LastPrice: midPrice,
		// Tight spread (20 bps) with deep liquidity
		Bids: []PriceLevel{
			{Price: midPrice * 0.9990, Size: 100.0}, // $5M at best bid
			{Price: midPrice * 0.9985, Size: 80.0},
			{Price: midPrice * 0.9980, Size: 120.0}, // Deep within 2%
			{Price: midPrice * 0.9975, Size: 150.0},
			{Price: midPrice * 0.9900, Size: 200.0},
		},
		Asks: []PriceLevel{
			{Price: midPrice * 1.0010, Size: 100.0}, // $5M at best ask  
			{Price: midPrice * 1.0015, Size: 85.0},
			{Price: midPrice * 1.0020, Size: 125.0}, // Deep within 2%
			{Price: midPrice * 1.0025, Size: 160.0},
			{Price: midPrice * 1.0100, Size: 180.0},
		},
		Metadata: SnapshotMetadata{
			Source:      "binance",
			IsStale:     false,
			BookQuality: "full",
		},
	}
}

func createTier2OrderBook(symbol string, midPrice float64) *OrderBookSnapshot {
	return &OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     "coinbase",
		Timestamp: time.Now(),
		LastPrice: midPrice,
		// Medium spread (45 bps) with adequate liquidity
		Bids: []PriceLevel{
			{Price: midPrice * 0.9977, Size: 30.0}, // ~$90k at best
			{Price: midPrice * 0.9970, Size: 25.0}, 
			{Price: midPrice * 0.9960, Size: 35.0},
			{Price: midPrice * 0.9950, Size: 40.0},
		},
		Asks: []PriceLevel{
			{Price: midPrice * 1.0023, Size: 28.0}, // ~$84k at best
			{Price: midPrice * 1.0030, Size: 22.0},
			{Price: midPrice * 1.0040, Size: 32.0}, 
			{Price: midPrice * 1.0050, Size: 38.0},
		},
		Metadata: SnapshotMetadata{
			Source:      "coinbase",
			IsStale:     false,
			BookQuality: "full",
		},
	}
}

func createTier3OrderBook(symbol string, midPrice float64) *OrderBookSnapshot {
	return &OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     "okx",
		Timestamp: time.Now(),
		LastPrice: midPrice,
		// Wide spread (75 bps) with minimal liquidity
		Bids: []PriceLevel{
			{Price: midPrice * 0.9962, Size: 3000.0}, // ~$15k at best
			{Price: midPrice * 0.9955, Size: 2800.0}, 
			{Price: midPrice * 0.9945, Size: 3200.0},
		},
		Asks: []PriceLevel{
			{Price: midPrice * 1.0038, Size: 2900.0}, // ~$14.5k at best
			{Price: midPrice * 1.0045, Size: 2600.0},
			{Price: midPrice * 1.0055, Size: 3100.0},
		},
		Metadata: SnapshotMetadata{
			Source:      "okx",
			IsStale:     false,
			BookQuality: "partial",
		},
	}
}

func createWideSpreadOrderBook(symbol string, midPrice float64) *OrderBookSnapshot {
	return &OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     "binance",
		Timestamp: time.Now(),
		LastPrice: midPrice,
		// Excessive spread (120 bps) - should fail spread gate
		Bids: []PriceLevel{
			{Price: midPrice * 0.9940, Size: 500.0}, // Wide spread
		},
		Asks: []PriceLevel{
			{Price: midPrice * 1.0060, Size: 500.0}, 
		},
		Metadata: SnapshotMetadata{
			Source:      "binance",
			IsStale:     false,
			BookQuality: "degraded",
		},
	}
}

func createThinDepthOrderBook(symbol string, midPrice float64) *OrderBookSnapshot {
	return &OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     "coinbase", 
		Timestamp: time.Now(),
		LastPrice: midPrice,
		// Tight spread but very thin depth - should fail depth gate
		Bids: []PriceLevel{
			{Price: midPrice * 0.9995, Size: 50.0}, // Only $500 depth
		},
		Asks: []PriceLevel{
			{Price: midPrice * 1.0005, Size: 50.0}, // Only $500 depth
		},
		Metadata: SnapshotMetadata{
			Source:      "coinbase",
			IsStale:     false,
			BookQuality: "partial",
		},
	}
}