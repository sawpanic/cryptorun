package unit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/microstructure"
)

func TestMicrostructureValidation(t *testing.T) {
	validator := microstructure.NewMicrostructureValidator(microstructure.DefaultRequirementThresholds())

	// Test data that should pass all validations
	validData := createValidMicrostructureData()
	result := validator.ValidateMicrostructure(validData)

	if !result.Passed {
		t.Errorf("Valid data should pass validation. Failures: %v", result.FailureReasons)
	}

	if result.ConfidenceScore < 95 {
		t.Errorf("Expected high confidence for valid data, got %.1f", result.ConfidenceScore)
	}

	// Check metrics calculation
	if result.Metrics.SpreadBps <= 0 {
		t.Error("Spread should be calculated and positive")
	}

	if result.Metrics.TotalDepth <= 0 {
		t.Error("Total depth should be calculated and positive")
	}
}

func TestMicrostructureValidationFailures(t *testing.T) {
	validator := microstructure.NewMicrostructureValidator(microstructure.DefaultRequirementThresholds())

	tests := []struct {
		name           string
		modifyData     func(*microstructure.MicrostructureData)
		expectFailure  bool
		expectedReason string
	}{
		{
			name: "wide spread",
			modifyData: func(data *microstructure.MicrostructureData) {
				data.BestAsk = 50300.0 // Wide spread > 50 bps limit
			},
			expectFailure:  true,
			expectedReason: "Spread too wide",
		},
		{
			name: "insufficient depth",
			modifyData: func(data *microstructure.MicrostructureData) {
				// Clear order book to have no depth
				data.OrderBook.Bids = []microstructure.OrderLevel{}
				data.OrderBook.Asks = []microstructure.OrderLevel{}
			},
			expectFailure:  true,
			expectedReason: "Insufficient depth",
		},
		{
			name: "stale data",
			modifyData: func(data *microstructure.MicrostructureData) {
				data.Metadata.Staleness = 120.0 // 2 minutes > 60s limit
			},
			expectFailure:  true,
			expectedReason: "Data too stale",
		},
		{
			name: "non-exchange-native",
			modifyData: func(data *microstructure.MicrostructureData) {
				data.Metadata.IsExchangeNative = false
			},
			expectFailure:  true,
			expectedReason: "not exchange-native",
		},
		{
			name: "banned aggregator",
			modifyData: func(data *microstructure.MicrostructureData) {
				data.Metadata.APIEndpoint = "https://api.dexscreener.com/data"
			},
			expectFailure:  true,
			expectedReason: "Banned aggregator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := createValidMicrostructureData()
			tt.modifyData(&data)
			
			result := validator.ValidateMicrostructure(data)
			
			if tt.expectFailure {
				if result.Passed {
					t.Error("Expected validation to fail but it passed")
				}
				
				found := false
				for _, reason := range result.FailureReasons {
					if strings.Contains(reason, tt.expectedReason) {
						found = true
						break
					}
				}
				
				if !found {
					t.Errorf("Expected failure reason containing '%s', got: %v", 
						tt.expectedReason, result.FailureReasons)
				}
			}
		})
	}
}

func TestKrakenAdapter(t *testing.T) {
	adapter := microstructure.NewKrakenAdapter()
	ctx := context.Background()

	// Test basic properties
	if adapter.GetName() != "kraken" {
		t.Errorf("Expected name 'kraken', got '%s'", adapter.GetName())
	}

	// Test supported symbols
	if !adapter.IsSupported("BTC-USD") {
		t.Error("BTC-USD should be supported by Kraken adapter")
	}

	if adapter.IsSupported("INVALID-PAIR") {
		t.Error("Invalid pair should not be supported")
	}

	// Test microstructure data retrieval
	data, err := adapter.GetMicrostructureData(ctx, "BTC-USD")
	if err != nil {
		t.Fatalf("GetMicrostructureData failed: %v", err)
	}

	if data.Symbol != "BTC-USD" {
		t.Errorf("Expected symbol 'BTC-USD', got '%s'", data.Symbol)
	}

	if data.Exchange != "kraken" {
		t.Errorf("Expected exchange 'kraken', got '%s'", data.Exchange)
	}

	if !data.Metadata.IsExchangeNative {
		t.Error("Kraken data should be marked as exchange-native")
	}
}

func TestBinanceAdapter(t *testing.T) {
	adapter := microstructure.NewBinanceAdapter()
	ctx := context.Background()

	if adapter.GetName() != "binance" {
		t.Errorf("Expected name 'binance', got '%s'", adapter.GetName())
	}

	data, err := adapter.GetMicrostructureData(ctx, "BTC-USD")
	if err != nil {
		t.Fatalf("GetMicrostructureData failed: %v", err)
	}

	if data.Exchange != "binance" {
		t.Errorf("Expected exchange 'binance', got '%s'", data.Exchange)
	}

	if !data.Metadata.IsExchangeNative {
		t.Error("Binance data should be marked as exchange-native")
	}
}

func TestExchangeManager(t *testing.T) {
	manager := microstructure.NewExchangeManager()
	ctx := context.Background()

	// Test getting data for supported symbol
	data, err := manager.GetMicrostructureData(ctx, "BTC-USD")
	if err != nil {
		t.Fatalf("GetMicrostructureData failed: %v", err)
	}

	if data.Symbol != "BTC-USD" {
		t.Errorf("Expected symbol 'BTC-USD', got '%s'", data.Symbol)
	}

	// Should prefer Kraken (primary exchange)
	if data.Exchange != "kraken" {
		t.Errorf("Expected primary exchange 'kraken', got '%s'", data.Exchange)
	}

	// Test best exchange selection
	bestExchange := manager.GetBestExchangeForSymbol("BTC-USD")
	if bestExchange != "kraken" {
		t.Errorf("Expected best exchange 'kraken', got '%s'", bestExchange)
	}
}

// Helper functions for tests
func createValidMicrostructureData() microstructure.MicrostructureData {
	return microstructure.MicrostructureData{
		Symbol:    "BTC-USD",
		Exchange:  "kraken",
		Timestamp: time.Now(),
		BestBid:   50000.0,
		BestAsk:   50020.0,
		BidSize:   2.0,
		AskSize:   1.5,
		OrderBook: microstructure.OrderBook{
			Bids: []microstructure.OrderLevel{
				{Price: 50000.0, Size: 2.0, SizeUSD: 100000.0, OrderCount: 3},
				{Price: 49990.0, Size: 3.0, SizeUSD: 149970.0, OrderCount: 5},
			},
			Asks: []microstructure.OrderLevel{
				{Price: 50020.0, Size: 1.5, SizeUSD: 75030.0, OrderCount: 2},
				{Price: 50030.0, Size: 2.5, SizeUSD: 125075.0, OrderCount: 4},
			},
			Timestamp: time.Now(),
			Sequence:  12345,
		},
		RecentTrades: []microstructure.Trade{
			{Price: 50010.0, Size: 0.5, Side: "buy", Timestamp: time.Now().Add(-30 * time.Second), TradeID: "t1"},
		},
		Volume24h:         50000000.0,
		MarketCap:         1000000000000.0,
		CirculatingSupply: 19500000.0,
		Metadata: microstructure.MicrostructureMetadata{
			DataSource:       "kraken_native_api",
			LastUpdate:       time.Now(),
			Staleness:        5.0,
			IsExchangeNative: true,
			APIEndpoint:      "https://api.kraken.com/0/public/Depth",
			RateLimit: microstructure.RateLimit{
				RequestsUsed:      10,
				RequestsRemaining: 170,
				ResetTimestamp:    time.Now().Add(time.Minute).Unix(),
			},
		},
	}
}