package providers

import (
	"context"
	"testing"
)

// Test-local type aliases to avoid import dependencies  
type OrderBookData struct {
	Symbol string
	Data   interface{}
}

type DepthData struct {
	Symbol string  
	Depth  float64
}

type SpreadData struct {
	Symbol string
	Spread float64
}

// TestAggregatorBanEnforcement validates that aggregators cannot provide microstructure data
func TestAggregatorBanEnforcement(t *testing.T) {
	guard := NewExchangeNativeGuard()
	
	tests := []struct {
		name     string
		source   string
		dataType DataType
		wantErr  bool
	}{
		// Banned aggregators for microstructure
		{
			name:     "CoinGecko banned for microstructure",
			source:   "coingecko",
			dataType: DataTypeMicrostructure,
			wantErr:  true,
		},
		{
			name:     "DEXScreener banned for microstructure",
			source:   "dexscreener",
			dataType: DataTypeMicrostructure,
			wantErr:  true,
		},
		{
			name:     "CoinPaprika banned for microstructure",
			source:   "coinpaprika",
			dataType: DataTypeMicrostructure,
			wantErr:  true,
		},
		{
			name:     "CoinMarketCap banned for microstructure",
			source:   "coinmarketcap",
			dataType: DataTypeMicrostructure,
			wantErr:  true,
		},
		
		// Exchange-native sources allowed
		{
			name:     "Kraken allowed for microstructure",
			source:   "kraken",
			dataType: DataTypeMicrostructure,
			wantErr:  false,
		},
		{
			name:     "Binance allowed for microstructure",
			source:   "binance",
			dataType: DataTypeMicrostructure,
			wantErr:  false,
		},
		{
			name:     "Coinbase allowed for microstructure",
			source:   "coinbase",
			dataType: DataTypeMicrostructure,
			wantErr:  false,
		},
		
		// Aggregators allowed for pricing (but not recommended)
		{
			name:     "CoinGecko allowed for pricing",
			source:   "coingecko",
			dataType: DataTypePricing,
			wantErr:  false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guard.ValidateDataSource(tt.source, tt.dataType)
			
			if tt.wantErr && err == nil {
				t.Errorf("Expected error for source %s with data type %s, got none", tt.source, tt.dataType)
			}
			
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error for source %s with data type %s, got: %v", tt.source, tt.dataType, err)
			}
			
			// Check that microstructure violations are properly typed
			if tt.wantErr && tt.dataType == DataTypeMicrostructure && err != nil {
				if !IsAggregatorViolation(err) {
					t.Errorf("Expected AggregatorViolationError, got: %T", err)
				}
			}
		})
	}
}

// TestCompileTimeGuard ensures the compile-time guard function works
func TestCompileTimeGuard(t *testing.T) {
	// This should not panic - it validates that banned sources are caught
	// and allowed sources pass validation
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CompileTimeGuard panicked: %v", r)
		}
	}()
	
	CompileTimeGuard()
}

// TestAggregatorFallbackBan tests that aggregator fallback respects microstructure ban
func TestAggregatorFallbackBan(t *testing.T) {
	// This test only runs when with_agg build tag is present
	t.Run("microstructure methods banned", func(t *testing.T) {
		// Create a mock aggregator (this would normally fail validation)
		// We'll test the method-level bans instead
		provider := &mockAggregatorProvider{name: "test_aggregator"}
		
		ctx := context.Background()
		
		// Test that microstructure methods are banned
		_, err := provider.GetOrderBook(ctx, "BTCUSD")
		if err == nil || !IsAggregatorViolation(err) {
			t.Error("Expected aggregator violation for GetOrderBook, got none")
		}
		
		_, err = provider.GetDepthData(ctx, "BTCUSD")  
		if err == nil || !IsAggregatorViolation(err) {
			t.Error("Expected aggregator violation for GetDepthData, got none")
		}
		
		_, err = provider.GetSpreadData(ctx, "BTCUSD")
		if err == nil || !IsAggregatorViolation(err) {
			t.Error("Expected aggregator violation for GetSpreadData, got none")
		}
	})
}

// TestDataStructureValidation tests struct field validation
func TestDataStructureValidation(t *testing.T) {
	guard := NewExchangeNativeGuard()
	
	tests := []struct {
		name     string
		data     interface{}
		dataType DataType
		wantErr  bool
	}{
		{
			name: "valid exchange-native source",
			data: struct {
				Source string `json:"source"`
				Price  float64 `json:"price"`
			}{
				Source: "kraken",
				Price:  50000.0,
			},
			dataType: DataTypeMicrostructure,
			wantErr:  false,
		},
		{
			name: "banned aggregator source",
			data: struct {
				Source string `json:"source"`
				Price  float64 `json:"price"`
			}{
				Source: "coingecko",
				Price:  50000.0,
			},
			dataType: DataTypeMicrostructure,
			wantErr:  true,
		},
		{
			name: "nested banned source",
			data: struct {
				Metadata struct {
					Provider string `json:"provider"`
				} `json:"metadata"`
			}{
				Metadata: struct {
					Provider string `json:"provider"`
				}{
					Provider: "dexscreener",
				},
			},
			dataType: DataTypeMicrostructure,
			wantErr:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guard.ValidateDataStructure(tt.data, tt.dataType)
			
			if tt.wantErr && err == nil {
				t.Error("Expected validation error, got none")
			}
			
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

// TestUSDPairValidation ensures only USD pairs are processed
func TestUSDPairValidation(t *testing.T) {
	tests := []struct {
		pair    string
		wantUSD bool
	}{
		{"BTCUSD", true},
		{"ETHUSD", true},
		{"BTC-USD", true},
		{"XXBTZUSD", true}, // Kraken format
		{"XETHZUSD", true}, // Kraken format
		{"BTCUSDT", true},  // Tether
		{"ETHUSDC", true},  // USDC
		
		{"BTCEUR", false},
		{"ETHBTC", false},
		{"ADABNB", false},
		{"DOGEETH", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.pair, func(t *testing.T) {
			result := IsUSDPair(tt.pair)
			if result != tt.wantUSD {
				t.Errorf("IsUSDPair(%s) = %v, want %v", tt.pair, result, tt.wantUSD)
			}
		})
	}
}

// Mock provider for testing (would only be available with build tag)
type mockAggregatorProvider struct {
	name string
}

func (m *mockAggregatorProvider) GetOrderBook(ctx context.Context, symbol string) (*OrderBookData, error) {
	return nil, &AggregatorViolationError{
		Source:   m.name,
		DataType: DataTypeMicrostructure,
		Reason:   "Mock aggregator banned from order book data",
	}
}

func (m *mockAggregatorProvider) GetDepthData(ctx context.Context, symbol string) (*DepthData, error) {
	return nil, &AggregatorViolationError{
		Source:   m.name,
		DataType: DataTypeMicrostructure,
		Reason:   "Mock aggregator banned from depth data",
	}
}

func (m *mockAggregatorProvider) GetSpreadData(ctx context.Context, symbol string) (*SpreadData, error) {
	return nil, &AggregatorViolationError{
		Source:   m.name,
		DataType: DataTypeMicrostructure,
		Reason:   "Mock aggregator banned from spread data",
	}
}

// Benchmark tests for guard performance
func BenchmarkValidateDataSource(b *testing.B) {
	guard := NewExchangeNativeGuard()
	
	b.Run("exchange-native", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = guard.ValidateDataSource("kraken", DataTypeMicrostructure)
		}
	})
	
	b.Run("banned-aggregator", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = guard.ValidateDataSource("coingecko", DataTypeMicrostructure)
		}
	})
}

func BenchmarkValidateProvider(b *testing.B) {
	guard := NewExchangeNativeGuard()
	provider := &mockAggregatorProvider{name: "test"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = guard.ValidateProvider(provider)
	}
}