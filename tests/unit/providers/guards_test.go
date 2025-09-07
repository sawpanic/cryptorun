package providers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sawpanic/cryptorun/internal/providers"
)

func TestExchangeNativeGuard_ValidateDataSource_Microstructure(t *testing.T) {
	guard := providers.NewExchangeNativeGuard()

	tests := []struct {
		name      string
		source    string
		dataType  providers.DataType
		shouldErr bool
		errMsg    string
	}{
		// Exchange-native sources (should pass for microstructure)
		{
			name:      "Kraken allowed",
			source:    "kraken",
			dataType:  providers.DataTypeMicrostructure,
			shouldErr: false,
		},
		{
			name:      "Binance allowed",
			source:    "binance",
			dataType:  providers.DataTypeMicrostructure,
			shouldErr: false,
		},
		{
			name:      "OKX allowed",
			source:    "okx",
			dataType:  providers.DataTypeMicrostructure,
			shouldErr: false,
		},
		{
			name:      "Coinbase allowed",
			source:    "coinbase",
			dataType:  providers.DataTypeMicrostructure,
			shouldErr: false,
		},
		// Aggregator sources (should fail for microstructure)
		{
			name:      "CoinGecko banned for microstructure",
			source:    "coingecko",
			dataType:  providers.DataTypeMicrostructure,
			shouldErr: true,
			errMsg:    "Aggregator 'coingecko' banned for microstructure data",
		},
		{
			name:      "CoinPaprika banned for microstructure",
			source:    "coinpaprika",
			dataType:  providers.DataTypeMicrostructure,
			shouldErr: true,
			errMsg:    "Aggregator 'coinpaprika' banned for microstructure data",
		},
		{
			name:      "DEXScreener banned for microstructure",
			source:    "dexscreener",
			dataType:  providers.DataTypeMicrostructure,
			shouldErr: true,
			errMsg:    "Aggregator 'dexscreener' banned for microstructure data",
		},
		{
			name:      "Generic aggregated source banned",
			source:    "aggregated",
			dataType:  providers.DataTypeMicrostructure,
			shouldErr: true,
			errMsg:    "Aggregator 'aggregated' banned for microstructure data",
		},
		// Case sensitivity tests
		{
			name:      "Kraken uppercase allowed",
			source:    "KRAKEN",
			dataType:  providers.DataTypeMicrostructure,
			shouldErr: false,
		},
		{
			name:      "CoinGecko uppercase banned",
			source:    "COINGECKO",
			dataType:  providers.DataTypeMicrostructure,
			shouldErr: true,
			errMsg:    "Aggregator 'COINGECKO' banned for microstructure data",
		},
		// Non-microstructure data (aggregators allowed for pricing)
		{
			name:      "CoinGecko allowed for pricing",
			source:    "coingecko",
			dataType:  providers.DataTypePricing,
			shouldErr: false,
		},
		// Unknown exchange
		{
			name:      "Unknown exchange banned",
			source:    "unknown_exchange",
			dataType:  providers.DataTypeMicrostructure,
			shouldErr: true,
			errMsg:    "Source 'unknown_exchange' not in allowed exchange list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guard.ValidateDataSource(tt.source, tt.dataType)
			
			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error for source %s, got nil", tt.source)
					return
				}
				
				if !providers.IsAggregatorViolation(err) {
					t.Errorf("Expected AggregatorViolationError, got %T", err)
				}
				
				if tt.errMsg != "" && !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for source %s, got: %v", tt.source, err)
				}
			}
		})
	}
}

func TestExchangeNativeGuard_ValidateDataStructure(t *testing.T) {
	guard := providers.NewExchangeNativeGuard()

	t.Run("Valid exchange-native structure", func(t *testing.T) {
		validData := struct {
			Source string `json:"source"`
			Venue  string `json:"venue"`
			Data   map[string]interface{} `json:"data"`
		}{
			Source: "kraken",
			Venue:  "binance",
			Data: map[string]interface{}{
				"provider": "okx",
				"exchange": "coinbase",
			},
		}

		err := guard.ValidateDataStructure(validData, providers.DataTypeMicrostructure)
		if err != nil {
			t.Errorf("Expected no error for valid structure, got: %v", err)
		}
	})

	t.Run("Invalid aggregator in structure", func(t *testing.T) {
		invalidData := struct {
			Source   string `json:"source"`
			Provider string `json:"provider"`
		}{
			Source:   "kraken",
			Provider: "coingecko", // Banned aggregator
		}

		err := guard.ValidateDataStructure(invalidData, providers.DataTypeMicrostructure)
		if err == nil {
			t.Error("Expected error for structure with banned aggregator, got nil")
		}

		if !providers.IsAggregatorViolation(err) {
			t.Errorf("Expected AggregatorViolationError, got %T", err)
		}
	})

	t.Run("Nested structure validation", func(t *testing.T) {
		nestedData := struct {
			Exchanges []struct {
				Name   string `json:"name"`
				Source string `json:"source"`
			} `json:"exchanges"`
		}{
			Exchanges: []struct {
				Name   string `json:"name"`
				Source string `json:"source"`
			}{
				{Name: "Kraken", Source: "kraken"},
				{Name: "Aggregated", Source: "dexscreener"}, // This should fail
			},
		}

		err := guard.ValidateDataStructure(nestedData, providers.DataTypeMicrostructure)
		if err == nil {
			t.Error("Expected error for nested structure with banned source, got nil")
		}

		if !containsSubstring(err.Error(), "dexscreener") {
			t.Errorf("Expected error to mention banned source 'dexscreener', got: %v", err.Error())
		}
	})

	t.Run("Map key validation", func(t *testing.T) {
		mapData := map[string]interface{}{
			"kraken":    "valid_data",
			"coingecko": "invalid_aggregator", // Key should be validated
		}

		err := guard.ValidateDataStructure(mapData, providers.DataTypeMicrostructure)
		if err == nil {
			t.Error("Expected error for map with banned key, got nil")
		}

		if !containsSubstring(err.Error(), "coingecko") {
			t.Errorf("Expected error to mention banned key 'coingecko', got: %v", err.Error())
		}
	})
}

func TestExchangeNativeGuard_GetAllowedExchanges(t *testing.T) {
	guard := providers.NewExchangeNativeGuard()
	
	allowed := guard.GetAllowedExchanges()
	
	expectedExchanges := []string{"binance", "okx", "coinbase", "kraken"}
	
	if len(allowed) != len(expectedExchanges) {
		t.Errorf("Expected %d allowed exchanges, got %d", len(expectedExchanges), len(allowed))
	}

	// Check that all expected exchanges are present
	allowedMap := make(map[string]bool)
	for _, exchange := range allowed {
		allowedMap[exchange] = true
	}

	for _, expected := range expectedExchanges {
		if !allowedMap[expected] {
			t.Errorf("Expected exchange '%s' not found in allowed list", expected)
		}
	}
}

func TestExchangeNativeGuard_GetBannedSources(t *testing.T) {
	guard := providers.NewExchangeNativeGuard()
	
	banned := guard.GetBannedSources()
	
	// Check for key banned sources
	expectedBanned := []string{"coingecko", "coinpaprika", "dexscreener", "aggregated"}
	
	bannedMap := make(map[string]bool)
	for _, source := range banned {
		bannedMap[source] = true
	}

	for _, expected := range expectedBanned {
		if !bannedMap[expected] {
			t.Errorf("Expected banned source '%s' not found in banned list", expected)
		}
	}
}

func TestExchangeNativeGuard_IsExchangeNative(t *testing.T) {
	guard := providers.NewExchangeNativeGuard()

	tests := []struct {
		source   string
		expected bool
	}{
		{"kraken", true},
		{"binance", true},
		{"okx", true},
		{"coinbase", true},
		{"coingecko", false},
		{"dexscreener", false},
		{"aggregated", false},
		{"unknown", false},
		{"KRAKEN", true}, // Case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			result := guard.IsExchangeNative(tt.source)
			if result != tt.expected {
				t.Errorf("IsExchangeNative(%s) = %v, expected %v", tt.source, result, tt.expected)
			}
		})
	}
}

func TestDataType_String(t *testing.T) {
	tests := []struct {
		dataType providers.DataType
		expected string
	}{
		{providers.DataTypeMicrostructure, "microstructure"},
		{providers.DataTypePricing, "pricing"},
		{providers.DataTypeFunding, "funding"},
		{providers.DataTypeSocial, "social"},
		{providers.DataTypeUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.dataType.String()
			if result != tt.expected {
				t.Errorf("DataType.String() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestAggregatorViolationError(t *testing.T) {
	err := &providers.AggregatorViolationError{
		Source:   "coingecko",
		DataType: providers.DataTypeMicrostructure,
		Reason:   "Aggregator banned for microstructure data",
	}

	expectedMsg := "aggregator violation: Aggregator banned for microstructure data (source: coingecko, type: microstructure)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	if !providers.IsAggregatorViolation(err) {
		t.Error("IsAggregatorViolation should return true for AggregatorViolationError")
	}

	// Test with non-aggregator error
	otherErr := fmt.Errorf("some other error")
	if providers.IsAggregatorViolation(otherErr) {
		t.Error("IsAggregatorViolation should return false for non-aggregator error")
	}
}

func TestCompileTimeGuard(t *testing.T) {
	// This test ensures the compile-time guard doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("CompileTimeGuard panicked: %v", r)
		}
	}()

	providers.CompileTimeGuard()
}

// Benchmark tests
func BenchmarkExchangeNativeGuard_ValidateDataSource(b *testing.B) {
	guard := providers.NewExchangeNativeGuard()
	
	b.Run("AllowedSource", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			guard.ValidateDataSource("kraken", providers.DataTypeMicrostructure)
		}
	})
	
	b.Run("BannedSource", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			guard.ValidateDataSource("coingecko", providers.DataTypeMicrostructure)
		}
	})
}

func BenchmarkExchangeNativeGuard_ValidateDataStructure(b *testing.B) {
	guard := providers.NewExchangeNativeGuard()
	
	testData := struct {
		Source   string                 `json:"source"`
		Provider string                 `json:"provider"`
		Data     map[string]interface{} `json:"data"`
	}{
		Source:   "kraken",
		Provider: "binance",
		Data: map[string]interface{}{
			"venue":    "okx",
			"exchange": "coinbase",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		guard.ValidateDataStructure(testData, providers.DataTypeMicrostructure)
	}
}

// Helper function
func containsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}

