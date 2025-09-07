package microstructure

import (
	"fmt"
	"strings"
	"testing"

	"cryptorun/internal/microstructure/adapters"
)

func TestGuardAgainstAggregator(t *testing.T) {
	testCases := []struct {
		name          string
		source        string
		shouldFail    bool
		expectedError string
	}{
		{
			name:          "CoinGecko - banned aggregator",
			source:        "coingecko",
			shouldFail:    true,
			expectedError: "contains banned aggregator 'coingecko'",
		},
		{
			name:          "DEXScreener - banned aggregator",
			source:        "dexscreener",
			shouldFail:    true,
			expectedError: "contains banned aggregator 'dexscreener'",
		},
		{
			name:          "CoinMarketCap - banned aggregator",
			source:        "coinmarketcap",
			shouldFail:    true,
			expectedError: "contains banned aggregator 'coinmarketcap'",
		},
		{
			name:          "Mixed case CoinGecko - banned aggregator",
			source:        "CoinGecko",
			shouldFail:    true,
			expectedError: "contains banned aggregator 'coingecko'",
		},
		{
			name:          "Aggregated source - banned pattern",
			source:        "aggregated_data",
			shouldFail:    true,
			expectedError: "contains banned aggregator 'aggregated'",
		},
		{
			name:       "Binance - allowed exchange",
			source:     "binance",
			shouldFail: false,
		},
		{
			name:       "OKX - allowed exchange",
			source:     "okx",
			shouldFail: false,
		},
		{
			name:       "Coinbase - allowed exchange",
			source:     "coinbase",
			shouldFail: false,
		},
		{
			name:       "Kraken - allowed exchange",
			source:     "kraken",
			shouldFail: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := adapters.GuardAgainstAggregator(tc.source)

			if tc.shouldFail {
				if err == nil {
					t.Errorf("Expected error for source '%s', but got nil", tc.source)
					return
				}

				banErr, ok := err.(*adapters.AggregatorBanError)
				if !ok {
					t.Errorf("Expected AggregatorBanError, got %T", err)
					return
				}

				if !strings.Contains(banErr.Reason, tc.expectedError) {
					t.Errorf("Expected error reason to contain '%s', got '%s'", tc.expectedError, banErr.Reason)
				}

				if banErr.Source != tc.source {
					t.Errorf("Expected source '%s' in error, got '%s'", tc.source, banErr.Source)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for source '%s', but got: %v", tc.source, err)
				}
			}
		})
	}
}

func TestValidateExchangeNativeSource(t *testing.T) {
	testCases := []struct {
		name          string
		source        string
		shouldFail    bool
		expectedError string
	}{
		{
			name:       "Binance API - valid",
			source:     "binance_api",
			shouldFail: false,
		},
		{
			name:       "OKX WebSocket - valid",
			source:     "okx_websocket",
			shouldFail: false,
		},
		{
			name:       "Coinbase Pro - valid",
			source:     "coinbase_pro",
			shouldFail: false,
		},
		{
			name:          "CoinGecko API - banned aggregator",
			source:        "coingecko_api",
			shouldFail:    true,
			expectedError: "contains banned aggregator 'coingecko'",
		},
		{
			name:          "Unknown exchange - not allowed",
			source:        "ftx",
			shouldFail:    true,
			expectedError: "not from allowed exchanges",
		},
		{
			name:          "Generic API - not allowed",
			source:        "market_data_api",
			shouldFail:    true,
			expectedError: "not from allowed exchanges",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := adapters.ValidateExchangeNativeSource(tc.source)

			if tc.shouldFail {
				if err == nil {
					t.Errorf("Expected error for source '%s', but got nil", tc.source)
					return
				}

				if !strings.Contains(err.Error(), tc.expectedError) {
					t.Errorf("Expected error to contain '%s', got '%s'", tc.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for source '%s', but got: %v", tc.source, err)
				}
			}
		})
	}
}

func TestCheckMicrostructureDataSource(t *testing.T) {
	testCases := []struct {
		name          string
		source        string
		endpoint      string
		shouldFail    bool
		expectedError string
	}{
		{
			name:       "Valid Binance L1 endpoint",
			source:     "binance",
			endpoint:   "/api/v3/ticker/bookTicker",
			shouldFail: false,
		},
		{
			name:       "Valid OKX L2 endpoint",
			source:     "okx",
			endpoint:   "/api/v5/market/books",
			shouldFail: false,
		},
		{
			name:          "Binance with aggregated endpoint",
			source:        "binance",
			endpoint:      "/api/v3/aggregated/ticker",
			shouldFail:    true,
			expectedError: "suspicious aggregation pattern",
		},
		{
			name:          "Valid source with composite endpoint",
			source:        "coinbase",
			endpoint:      "/products/composite/index",
			shouldFail:    true,
			expectedError: "suspicious aggregation pattern",
		},
		{
			name:          "Valid source with weighted endpoint",
			source:        "okx",
			endpoint:      "/api/v5/market/weighted/price",
			shouldFail:    true,
			expectedError: "suspicious aggregation pattern",
		},
		{
			name:          "Invalid source with valid endpoint",
			source:        "dexscreener",
			endpoint:      "/api/v1/ticker",
			shouldFail:    true,
			expectedError: "contains banned aggregator 'dexscreener'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := adapters.CheckMicrostructureDataSource(tc.source, tc.endpoint, nil)

			if tc.shouldFail {
				if err == nil {
					t.Errorf("Expected error for source '%s' with endpoint '%s', but got nil", tc.source, tc.endpoint)
					return
				}

				if !strings.Contains(err.Error(), tc.expectedError) {
					t.Errorf("Expected error to contain '%s', got '%s'", tc.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for source '%s' with endpoint '%s', but got: %v", tc.source, tc.endpoint, err)
				}
			}
		})
	}
}

func TestRuntimeAggregatorGuard(t *testing.T) {
	t.Run("Strict mode panics on violation", func(t *testing.T) {
		guard := adapters.NewRuntimeAggregatorGuard(true) // Strict mode

		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic for banned source in strict mode")
			}
		}()

		guard.CheckSource("coingecko")
	})

	t.Run("Non-strict mode records violations", func(t *testing.T) {
		guard := adapters.NewRuntimeAggregatorGuard(false) // Non-strict mode

		err := guard.CheckSource("dexscreener")
		if err == nil {
			t.Errorf("Expected error for banned source")
		}

		violations := guard.GetViolations()
		if len(violations) != 1 {
			t.Errorf("Expected 1 violation, got %d", len(violations))
		}

		if violations[0].Source != "dexscreener" {
			t.Errorf("Expected violation source 'dexscreener', got '%s'", violations[0].Source)
		}

		// Clear violations
		guard.ClearViolations()
		violations = guard.GetViolations()
		if len(violations) != 0 {
			t.Errorf("Expected 0 violations after clearing, got %d", len(violations))
		}
	})

	t.Run("Disabled guard allows everything", func(t *testing.T) {
		guard := adapters.NewRuntimeAggregatorGuard(false)
		guard.Disable()

		err := guard.CheckSource("coingecko")
		if err != nil {
			t.Errorf("Expected no error when guard is disabled, got: %v", err)
		}

		violations := guard.GetViolations()
		if len(violations) != 0 {
			t.Errorf("Expected no violations when disabled, got %d", len(violations))
		}
	})
}

func TestMustBeExchangeNative(t *testing.T) {
	t.Run("Valid exchange does not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Unexpected panic for valid exchange: %v", r)
			}
		}()

		adapters.MustBeExchangeNative("binance")
	})

	t.Run("Invalid source panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic for banned aggregator")
			} else {
				panicMsg := fmt.Sprintf("%v", r)
				if !strings.Contains(panicMsg, "CRITICAL") {
					t.Errorf("Expected CRITICAL in panic message, got: %s", panicMsg)
				}
			}
		}()

		adapters.MustBeExchangeNative("coingecko")
	})
}

func TestConvenienceValidationFunctions(t *testing.T) {
	testCases := []struct {
		name     string
		function func(string) error
		funcName string
	}{
		{
			name:     "ValidateL1DataSource",
			function: adapters.ValidateL1DataSource,
			funcName: "L1",
		},
		{
			name:     "ValidateL2DataSource",
			function: adapters.ValidateL2DataSource,
			funcName: "L2",
		},
		{
			name:     "ValidateOrderBookSource",
			function: adapters.ValidateOrderBookSource,
			funcName: "orderbook",
		},
		{
			name:     "ValidateTickerSource",
			function: adapters.ValidateTickerSource,
			funcName: "ticker",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Valid exchange should pass
			err := tc.function("binance")
			if err != nil {
				t.Errorf("Expected no error for valid exchange, got: %v", err)
			}

			// Banned aggregator should fail
			err = tc.function("coingecko")
			if err == nil {
				t.Errorf("Expected error for banned aggregator")
			}
		})
	}
}

func TestIsAggregatorBanned(t *testing.T) {
	testCases := []struct {
		name           string
		source         string
		expectedBanned bool
	}{
		{
			name:           "CoinGecko is banned",
			source:         "coingecko",
			expectedBanned: true,
		},
		{
			name:           "DEXScreener is banned",
			source:         "dexscreener",
			expectedBanned: true,
		},
		{
			name:           "Binance is not banned",
			source:         "binance",
			expectedBanned: false,
		},
		{
			name:           "OKX is not banned",
			source:         "okx",
			expectedBanned: false,
		},
		{
			name:           "Unknown exchange is not banned (but may fail validation)",
			source:         "unknown_exchange",
			expectedBanned: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			banned := adapters.IsAggregatorBanned(tc.source)
			if banned != tc.expectedBanned {
				t.Errorf("Expected banned=%t for source '%s', got %t", tc.expectedBanned, tc.source, banned)
			}
		})
	}
}

func TestAggregatorBanError(t *testing.T) {
	err := &adapters.AggregatorBanError{
		Source:     "coingecko",
		Function:   "test_function",
		Reason:     "contains banned aggregator",
		StackTrace: "mock stack trace",
	}

	errorMsg := err.Error()
	expectedParts := []string{
		"AGGREGATOR BAN VIOLATION",
		"coingecko",
		"test_function",
		"contains banned aggregator",
	}

	for _, part := range expectedParts {
		if !strings.Contains(errorMsg, part) {
			t.Errorf("Expected error message to contain '%s', got: %s", part, errorMsg)
		}
	}
}

// Test that the ban lists are properly configured
func TestBanListConfiguration(t *testing.T) {
	// Verify banned aggregators list is not empty
	if len(adapters.BannedAggregators) == 0 {
		t.Error("BannedAggregators list should not be empty")
	}

	// Verify allowed exchanges list is not empty
	if len(adapters.AllowedExchanges) == 0 {
		t.Error("AllowedExchanges list should not be empty")
	}

	// Verify some expected entries
	expectedBanned := []string{"coingecko", "dexscreener", "coinmarketcap"}
	for _, banned := range expectedBanned {
		found := false
		for _, entry := range adapters.BannedAggregators {
			if entry == banned {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected '%s' to be in BannedAggregators list", banned)
		}
	}

	expectedAllowed := []string{"binance", "okx", "coinbase"}
	for _, allowed := range expectedAllowed {
		found := false
		for _, entry := range adapters.AllowedExchanges {
			if entry == allowed {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected '%s' to be in AllowedExchanges list", allowed)
		}
	}
}

// Benchmark aggregator checking performance
func BenchmarkGuardAgainstAggregator(b *testing.B) {
	sources := []string{"binance", "okx", "coinbase", "coingecko", "dexscreener"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source := sources[i%len(sources)]
		_ = adapters.GuardAgainstAggregator(source)
	}
}

func BenchmarkValidateExchangeNativeSource(b *testing.B) {
	sources := []string{"binance", "okx", "coinbase", "coingecko", "dexscreener"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source := sources[i%len(sources)]
		_ = adapters.ValidateExchangeNativeSource(source)
	}
}
