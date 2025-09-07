package unit

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/venue/types"
	"github.com/sawpanic/cryptorun/internal/domain/microstructure"
)

func TestCheckerValidateOrderBook(t *testing.T) {
	checker := microstructure.NewChecker(nil) // Use default config

	tests := []struct {
		name        string
		orderBook   *types.OrderBook
		vadr        float64
		adv         float64
		expectValid bool
		description string
	}{
		{
			name: "Valid orderbook - all requirements met",
			orderBook: &types.OrderBook{
				Symbol:                "BTCUSDT",
				Venue:                 "binance",
				TimestampMono:         time.Now(),
				SpreadBPS:             35.0,   // < 50 bps ✓
				DepthUSDPlusMinus2Pct: 150000, // >= $100k ✓
			},
			vadr:        2.1, // >= 1.75x ✓
			adv:         500000,
			expectValid: true,
			description: "Should pass all microstructure requirements",
		},
		{
			name: "Invalid spread - too wide",
			orderBook: &types.OrderBook{
				Symbol:                "ALTUSDT",
				Venue:                 "okx",
				TimestampMono:         time.Now(),
				SpreadBPS:             65.0,   // > 50 bps ❌
				DepthUSDPlusMinus2Pct: 150000, // >= $100k ✓
			},
			vadr:        2.1, // >= 1.75x ✓
			adv:         500000,
			expectValid: false,
			description: "Should fail due to excessive spread",
		},
		{
			name: "Invalid depth - insufficient liquidity",
			orderBook: &types.OrderBook{
				Symbol:                "LOWLIQ",
				Venue:                 "coinbase",
				TimestampMono:         time.Now(),
				SpreadBPS:             35.0,  // < 50 bps ✓
				DepthUSDPlusMinus2Pct: 75000, // < $100k ❌
			},
			vadr:        2.1, // >= 1.75x ✓
			adv:         500000,
			expectValid: false,
			description: "Should fail due to insufficient depth",
		},
		{
			name: "Invalid VADR - low volatility",
			orderBook: &types.OrderBook{
				Symbol:                "STABLE",
				Venue:                 "binance",
				TimestampMono:         time.Now(),
				SpreadBPS:             35.0,   // < 50 bps ✓
				DepthUSDPlusMinus2Pct: 150000, // >= $100k ✓
			},
			vadr:        1.5, // < 1.75x ❌
			adv:         500000,
			expectValid: false,
			description: "Should fail due to low VADR",
		},
		{
			name: "Multiple violations",
			orderBook: &types.OrderBook{
				Symbol:                "BADPAIR",
				Venue:                 "okx",
				TimestampMono:         time.Now(),
				SpreadBPS:             85.0,  // > 50 bps ❌
				DepthUSDPlusMinus2Pct: 50000, // < $100k ❌
			},
			vadr:        1.2, // < 1.75x ❌
			adv:         100000,
			expectValid: false,
			description: "Should fail with multiple violations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			metrics := checker.ValidateOrderBook(ctx, tt.orderBook, tt.vadr, tt.adv)

			if metrics.OverallValid != tt.expectValid {
				t.Errorf("Expected OverallValid=%v, got %v - %s",
					tt.expectValid, metrics.OverallValid, tt.description)
			}

			// Verify individual validations
			expectedSpreadValid := tt.orderBook.SpreadBPS < 50.0
			if metrics.SpreadValid != expectedSpreadValid {
				t.Errorf("Expected SpreadValid=%v, got %v (spread=%.1f bps)",
					expectedSpreadValid, metrics.SpreadValid, tt.orderBook.SpreadBPS)
			}

			expectedDepthValid := tt.orderBook.DepthUSDPlusMinus2Pct >= 100000
			if metrics.DepthValid != expectedDepthValid {
				t.Errorf("Expected DepthValid=%v, got %v (depth=$%.0f)",
					expectedDepthValid, metrics.DepthValid, tt.orderBook.DepthUSDPlusMinus2Pct)
			}

			expectedVADRValid := tt.vadr >= 1.75
			if metrics.VADRValid != expectedVADRValid {
				t.Errorf("Expected VADRValid=%v, got %v (VADR=%.2fx)",
					expectedVADRValid, metrics.VADRValid, tt.vadr)
			}

			// Verify metadata
			if metrics.Symbol != tt.orderBook.Symbol {
				t.Errorf("Expected Symbol=%s, got %s", tt.orderBook.Symbol, metrics.Symbol)
			}
			if metrics.Venue != tt.orderBook.Venue {
				t.Errorf("Expected Venue=%s, got %s", tt.orderBook.Venue, metrics.Venue)
			}
		})
	}
}

func TestCheckerGenerateProof(t *testing.T) {
	checker := microstructure.NewChecker(nil)

	orderBook := &types.OrderBook{
		Symbol:                "TESTCOIN",
		Venue:                 "binance",
		TimestampMono:         time.Now(),
		SpreadBPS:             45.0,
		DepthUSDPlusMinus2Pct: 120000,
	}

	ctx := context.Background()
	metrics := checker.ValidateOrderBook(ctx, orderBook, 2.0, 500000)
	proof := checker.GenerateProof(ctx, orderBook, metrics)

	// Verify proof structure
	if proof.AssetSymbol != orderBook.Symbol {
		t.Errorf("Expected AssetSymbol=%s, got %s", orderBook.Symbol, proof.AssetSymbol)
	}

	if proof.VenueUsed != orderBook.Venue {
		t.Errorf("Expected VenueUsed=%s, got %s", orderBook.Venue, proof.VenueUsed)
	}

	if proof.ProvenValid != metrics.OverallValid {
		t.Errorf("Expected ProvenValid=%v, got %v", metrics.OverallValid, proof.ProvenValid)
	}

	// Verify spread proof
	if proof.SpreadProof.Metric != "spread_bps" {
		t.Errorf("Expected SpreadProof.Metric='spread_bps', got '%s'", proof.SpreadProof.Metric)
	}
	if proof.SpreadProof.ActualValue != orderBook.SpreadBPS {
		t.Errorf("Expected SpreadProof.ActualValue=%.1f, got %.1f",
			orderBook.SpreadBPS, proof.SpreadProof.ActualValue)
	}
	if proof.SpreadProof.RequiredValue != 50.0 {
		t.Errorf("Expected SpreadProof.RequiredValue=50.0, got %.1f", proof.SpreadProof.RequiredValue)
	}
	if proof.SpreadProof.Operator != "<" {
		t.Errorf("Expected SpreadProof.Operator='<', got '%s'", proof.SpreadProof.Operator)
	}

	// Verify depth proof
	if proof.DepthProof.Metric != "depth_usd_plus_minus_2pct" {
		t.Errorf("Expected DepthProof.Metric='depth_usd_plus_minus_2pct', got '%s'", proof.DepthProof.Metric)
	}
	if proof.DepthProof.ActualValue != orderBook.DepthUSDPlusMinus2Pct {
		t.Errorf("Expected DepthProof.ActualValue=%.0f, got %.0f",
			orderBook.DepthUSDPlusMinus2Pct, proof.DepthProof.ActualValue)
	}
	if proof.DepthProof.RequiredValue != 100000 {
		t.Errorf("Expected DepthProof.RequiredValue=100000, got %.0f", proof.DepthProof.RequiredValue)
	}
	if proof.DepthProof.Operator != ">=" {
		t.Errorf("Expected DepthProof.Operator='>=', got '%s'", proof.DepthProof.Operator)
	}

	// Verify VADR proof
	if proof.VADRProof.Metric != "vadr" {
		t.Errorf("Expected VADRProof.Metric='vadr', got '%s'", proof.VADRProof.Metric)
	}
	if proof.VADRProof.ActualValue != 2.0 {
		t.Errorf("Expected VADRProof.ActualValue=2.0, got %.2f", proof.VADRProof.ActualValue)
	}
	if proof.VADRProof.RequiredValue != 1.75 {
		t.Errorf("Expected VADRProof.RequiredValue=1.75, got %.2f", proof.VADRProof.RequiredValue)
	}
}

func TestCheckerCustomConfig(t *testing.T) {
	// Test with custom configuration
	customConfig := &microstructure.Config{
		MaxSpreadBPS:     75.0,  // Relaxed spread
		MinDepthUSD:      50000, // Reduced depth requirement
		MinVADR:          1.5,   // Lower VADR requirement
		RequireAllVenues: true,  // Stricter venue requirement
	}

	checker := microstructure.NewChecker(customConfig)

	orderBook := &types.OrderBook{
		Symbol:                "TESTCOIN",
		Venue:                 "okx",
		TimestampMono:         time.Now(),
		SpreadBPS:             60.0,  // Would fail default (>50) but passes custom (<75)
		DepthUSDPlusMinus2Pct: 75000, // Would fail default (<100k) but passes custom (>50k)
	}

	ctx := context.Background()
	metrics := checker.ValidateOrderBook(ctx, orderBook, 1.6, 200000) // VADR would fail default (<1.75) but passes custom (>1.5)

	if !metrics.OverallValid {
		t.Errorf("Expected custom config to pass relaxed requirements, but validation failed")
	}
	if !metrics.SpreadValid {
		t.Errorf("Expected spread validation to pass with custom config (%.1f < %.1f)",
			orderBook.SpreadBPS, customConfig.MaxSpreadBPS)
	}
	if !metrics.DepthValid {
		t.Errorf("Expected depth validation to pass with custom config ($%.0f >= $%.0f)",
			orderBook.DepthUSDPlusMinus2Pct, customConfig.MinDepthUSD)
	}
	if !metrics.VADRValid {
		t.Errorf("Expected VADR validation to pass with custom config (%.2fx >= %.2fx)",
			1.6, customConfig.MinVADR)
	}
}

// MockVenueClient for testing CheckAssetEligibility
type MockVenueClient struct {
	orderBook *types.OrderBook
	err       error
}

func (m *MockVenueClient) FetchOrderBook(ctx context.Context, symbol string) (*types.OrderBook, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.orderBook, nil
}

func TestCheckerCheckAssetEligibility(t *testing.T) {
	checker := microstructure.NewChecker(nil)

	// Create mock venue clients
	goodOrderBook := &types.OrderBook{
		Symbol:                "BTCUSDT",
		Venue:                 "binance",
		TimestampMono:         time.Now(),
		SpreadBPS:             35.0,
		DepthUSDPlusMinus2Pct: 150000,
	}

	badOrderBook := &types.OrderBook{
		Symbol:                "BTCUSDT",
		Venue:                 "okx",
		TimestampMono:         time.Now(),
		SpreadBPS:             85.0,  // Too wide
		DepthUSDPlusMinus2Pct: 50000, // Too shallow
	}

	venueClients := map[string]microstructure.VenueClient{
		"binance":  &MockVenueClient{orderBook: goodOrderBook},
		"okx":      &MockVenueClient{orderBook: badOrderBook},
		"coinbase": &MockVenueClient{err: fmt.Errorf("connection timeout")},
	}

	ctx := context.Background()
	result, err := checker.CheckAssetEligibility(ctx, "BTCUSDT", venueClients)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be eligible because binance passes (any venue sufficient)
	if !result.OverallEligible {
		t.Errorf("Expected asset to be eligible (binance passes), but got ineligible")
	}

	if len(result.EligibleVenues) != 1 {
		t.Errorf("Expected 1 eligible venue, got %d", len(result.EligibleVenues))
	}

	if result.EligibleVenues[0] != "binance" {
		t.Errorf("Expected binance to be eligible venue, got %s", result.EligibleVenues[0])
	}

	// Should have venue error for coinbase
	if len(result.VenueErrors) != 1 {
		t.Errorf("Expected 1 venue error, got %d", len(result.VenueErrors))
	}
}

func TestCheckerRequireAllVenues(t *testing.T) {
	// Test config requiring all venues to pass
	config := &microstructure.Config{
		MaxSpreadBPS:     50.0,
		MinDepthUSD:      100000,
		MinVADR:          1.75,
		RequireAllVenues: true, // All must pass
	}

	checker := microstructure.NewChecker(config)

	goodOrderBook := &types.OrderBook{
		Symbol:                "BTCUSDT",
		Venue:                 "binance",
		TimestampMono:         time.Now(),
		SpreadBPS:             35.0,
		DepthUSDPlusMinus2Pct: 150000,
	}

	badOrderBook := &types.OrderBook{
		Symbol:                "BTCUSDT",
		Venue:                 "okx",
		TimestampMono:         time.Now(),
		SpreadBPS:             85.0, // Fails
		DepthUSDPlusMinus2Pct: 150000,
	}

	venueClients := map[string]microstructure.VenueClient{
		"binance": &MockVenueClient{orderBook: goodOrderBook},
		"okx":     &MockVenueClient{orderBook: badOrderBook},
	}

	ctx := context.Background()
	result, err := checker.CheckAssetEligibility(ctx, "BTCUSDT", venueClients)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should NOT be eligible because okx fails and all venues required
	if result.OverallEligible {
		t.Errorf("Expected asset to be ineligible (okx fails, all required), but got eligible")
	}
}
