package unit

import (
	"math"
	"testing"

	"cryptorun/internal/application"
)

func TestCalculateADV(t *testing.T) {
	tests := []struct {
		name     string
		ticker   application.TickerData
		expected application.ADVResult
	}{
		{
			name: "Valid USD pair with quote volume",
			ticker: application.TickerData{
				Symbol:         "BTCUSD",
				Volume24hBase:  100.0,
				Volume24hQuote: 5000000.0,
				LastPrice:      50000.0,
				QuoteCurrency:  "USD",
			},
			expected: application.ADVResult{
				Symbol: "BTCUSD",
				ADVUSD: 5000000,
				Valid:  true,
			},
		},
		{
			name: "Valid USD pair with base volume and last price",
			ticker: application.TickerData{
				Symbol:         "ETHUSD",
				Volume24hBase:  1000.0,
				Volume24hQuote: 0.0,
				LastPrice:      3000.0,
				QuoteCurrency:  "USD",
			},
			expected: application.ADVResult{
				Symbol: "ETHUSD",
				ADVUSD: 3000000,
				Valid:  true,
			},
		},
		{
			name: "Rounding test - should round to nearest whole number",
			ticker: application.TickerData{
				Symbol:         "ADAUSD",
				Volume24hBase:  50000.0,
				Volume24hQuote: 0.0,
				LastPrice:      0.9999,
				QuoteCurrency:  "USD",
			},
			expected: application.ADVResult{
				Symbol: "ADAUSD",
				ADVUSD: 49995,
				Valid:  true,
			},
		},
		{
			name: "Zero volume should be invalid",
			ticker: application.TickerData{
				Symbol:         "ZEROUPD",
				Volume24hBase:  0.0,
				Volume24hQuote: 0.0,
				LastPrice:      100.0,
				QuoteCurrency:  "USD",
			},
			expected: application.ADVResult{
				Symbol:   "ZEROUPD",
				ADVUSD:   0,
				Valid:    false,
				ErrorMsg: "missing or invalid volume/price data",
			},
		},
		{
			name: "NaN volume should be invalid",
			ticker: application.TickerData{
				Symbol:         "NANUSD",
				Volume24hBase:  math.NaN(),
				Volume24hQuote: 0.0,
				LastPrice:      100.0,
				QuoteCurrency:  "USD",
			},
			expected: application.ADVResult{
				Symbol:   "NANUSD",
				ADVUSD:   0,
				Valid:    false,
				ErrorMsg: "missing or invalid volume/price data",
			},
		},
		{
			name: "Infinity volume should be invalid",
			ticker: application.TickerData{
				Symbol:         "INFUSD",
				Volume24hBase:  math.Inf(1),
				Volume24hQuote: 0.0,
				LastPrice:      100.0,
				QuoteCurrency:  "USD",
			},
			expected: application.ADVResult{
				Symbol:   "INFUSD",
				ADVUSD:   0,
				Valid:    false,
				ErrorMsg: "missing or invalid volume/price data",
			},
		},
		{
			name: "Negative volume should be invalid",
			ticker: application.TickerData{
				Symbol:         "NEGUSD",
				Volume24hBase:  -100.0,
				Volume24hQuote: 0.0,
				LastPrice:      100.0,
				QuoteCurrency:  "USD",
			},
			expected: application.ADVResult{
				Symbol:   "NEGUSD",
				ADVUSD:   0,
				Valid:    false,
				ErrorMsg: "missing or invalid volume/price data",
			},
		},
		{
			name: "Volume * Price resulting in infinity should be invalid",
			ticker: application.TickerData{
				Symbol:         "BIIGUSD",
				Volume24hBase:  1e200,
				Volume24hQuote: 0.0,
				LastPrice:      1e200,
				QuoteCurrency:  "USD",
			},
			expected: application.ADVResult{
				Symbol:   "BIIGUSD",
				ADVUSD:   0,
				Valid:    false,
				ErrorMsg: "volume*price calculation resulted in non-finite value",
			},
		},
		{
			name: "Non-USD quote currency should be invalid",
			ticker: application.TickerData{
				Symbol:         "BTCEUR",
				Volume24hBase:  100.0,
				Volume24hQuote: 0.0,
				LastPrice:      45000.0,
				QuoteCurrency:  "EUR",
			},
			expected: application.ADVResult{
				Symbol:   "BTCEUR",
				ADVUSD:   0,
				Valid:    false,
				ErrorMsg: "non-USD quote currency: EUR",
			},
		},
		{
			name: "Zero price should be invalid",
			ticker: application.TickerData{
				Symbol:         "ZEROPRICE",
				Volume24hBase:  1000.0,
				Volume24hQuote: 0.0,
				LastPrice:      0.0,
				QuoteCurrency:  "USD",
			},
			expected: application.ADVResult{
				Symbol:   "ZEROPRICE",
				ADVUSD:   0,
				Valid:    false,
				ErrorMsg: "missing or invalid volume/price data",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := application.CalculateADV(tt.ticker)

			if result.Symbol != tt.expected.Symbol {
				t.Errorf("Symbol mismatch: got %s, want %s", result.Symbol, tt.expected.Symbol)
			}

			if result.ADVUSD != tt.expected.ADVUSD {
				t.Errorf("ADVUSD mismatch: got %d, want %d", result.ADVUSD, tt.expected.ADVUSD)
			}

			if result.Valid != tt.expected.Valid {
				t.Errorf("Valid mismatch: got %v, want %v", result.Valid, tt.expected.Valid)
			}

			if !result.Valid && result.ErrorMsg != tt.expected.ErrorMsg {
				t.Errorf("ErrorMsg mismatch: got %s, want %s", result.ErrorMsg, tt.expected.ErrorMsg)
			}
		})
	}
}

func TestBatchCalculateADV(t *testing.T) {
	tickers := []application.TickerData{
		{
			Symbol:         "BTCUSD",
			Volume24hQuote: 10000000.0,
			LastPrice:      50000.0,
			QuoteCurrency:  "USD",
		},
		{
			Symbol:        "ETHUSD",
			Volume24hBase: 1000.0,
			LastPrice:     3000.0,
			QuoteCurrency: "USD",
		},
		{
			Symbol:        "SMALLUSD",
			Volume24hBase: 10.0,
			LastPrice:     1.0,
			QuoteCurrency: "USD",
		},
		{
			Symbol:        "INVALIDUSD",
			Volume24hBase: 0.0,
			LastPrice:     100.0,
			QuoteCurrency: "USD",
		},
	}

	results := application.BatchCalculateADV(tickers, 50000)

	if len(results) != 2 {
		t.Errorf("Expected 2 results above threshold, got %d", len(results))
	}

	expectedSymbols := []string{"BTCUSD", "ETHUSD"}
	for i, result := range results {
		if result.Symbol != expectedSymbols[i] {
			t.Errorf("Result %d: expected symbol %s, got %s", i, expectedSymbols[i], result.Symbol)
		}
		if !result.Valid {
			t.Errorf("Result %d: expected valid result for %s", i, result.Symbol)
		}
		if result.ADVUSD < 50000 {
			t.Errorf("Result %d: ADV %d below threshold 50000", i, result.ADVUSD)
		}
	}
}

func TestRoundingEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		volume   float64
		price    float64
		expected int64
	}{
		{"Round up at 0.5", 100.0, 10.005, 1001},
		{"Round down below 0.5", 100.0, 10.004, 1000},
		{"Large numbers", 1000000.0, 99.99, 99990000},
		{"Small fractional", 0.001, 1000000.0, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker := application.TickerData{
				Symbol:        "TEST",
				Volume24hBase: tt.volume,
				LastPrice:     tt.price,
				QuoteCurrency: "USD",
			}

			result := application.CalculateADV(ticker)

			if !result.Valid {
				t.Errorf("Expected valid result, got invalid with error: %s", result.ErrorMsg)
			}

			if result.ADVUSD != tt.expected {
				t.Errorf("Rounding error: got %d, want %d", result.ADVUSD, tt.expected)
			}
		})
	}
}
