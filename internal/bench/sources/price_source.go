package sources

import (
	"context"
	"fmt"
	"time"
)

// PriceSource handles exchange-native price data retrieval with fallback labeling
type PriceSource struct {
	sourceStrategy     string // exchange_native_first, aggregator_fallback
	exchangeNativeOnly bool   // Enforce exchange-native data only
	binanceClient      *BinanceClient
	krakenClient       *KrakenClient
	coinbaseClient     *CoinbaseClient
}

// PriceBar represents a single OHLCV bar
type PriceBar struct {
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	Source    string    `json:"source"` // exchange_native|aggregator_fallback
}

// PriceSeriesResult contains price data with source attribution
type PriceSeriesResult struct {
	Symbol     string     `json:"symbol"`
	Window     string     `json:"window"`
	Bars       []PriceBar `json:"bars"`
	DataSource string     `json:"data_source"`   // binance|kraken|coinbase
	SourceType string     `json:"source_type"`   // exchange_native|aggregator_fallback
	Fallback   bool       `json:"fallback_used"` // True if had to use aggregator
}

// NewPriceSource creates a new price data source manager
func NewPriceSource(sourceStrategy string, exchangeNativeOnly bool) *PriceSource {
	return &PriceSource{
		sourceStrategy:     sourceStrategy,
		exchangeNativeOnly: exchangeNativeOnly,
		binanceClient:      NewBinanceClient(),
		krakenClient:       NewKrakenClient(),
		coinbaseClient:     NewCoinbaseClient(),
	}
}

// GetPriceSeries retrieves OHLCV data with exchange-native priority
func (ps *PriceSource) GetPriceSeries(ctx context.Context, symbol, window string) ([]PriceBar, error) {
	result, err := ps.GetPriceSeriesWithAttribution(ctx, symbol, window)
	if err != nil {
		return nil, err
	}
	return result.Bars, nil
}

// GetPriceSeriesWithAttribution retrieves price data with full source attribution
func (ps *PriceSource) GetPriceSeriesWithAttribution(ctx context.Context, symbol, window string) (*PriceSeriesResult, error) {
	// Try exchange-native sources in priority order
	exchanges := []string{"binance", "kraken", "coinbase"}

	for _, exchange := range exchanges {
		bars, err := ps.tryExchangeNative(ctx, exchange, symbol, window)
		if err == nil && len(bars) > 0 {
			return &PriceSeriesResult{
				Symbol:     symbol,
				Window:     window,
				Bars:       bars,
				DataSource: exchange,
				SourceType: "exchange_native",
				Fallback:   false,
			}, nil
		}
	}

	// If exchange-native only mode, fail here
	if ps.exchangeNativeOnly {
		return nil, fmt.Errorf("no exchange-native data available for %s and exchange-native only mode enabled", symbol)
	}

	// Fall back to aggregators with labeling
	bars, err := ps.tryAggregatorFallback(ctx, symbol, window)
	if err != nil {
		return nil, fmt.Errorf("failed to get price data from any source: %w", err)
	}

	return &PriceSeriesResult{
		Symbol:     symbol,
		Window:     window,
		Bars:       bars,
		DataSource: "coingecko_aggregator",
		SourceType: "aggregator_fallback",
		Fallback:   true,
	}, nil
}

// tryExchangeNative attempts to get data from a specific exchange
func (ps *PriceSource) tryExchangeNative(ctx context.Context, exchange, symbol, window string) ([]PriceBar, error) {
	switch exchange {
	case "binance":
		return ps.binanceClient.GetKlines(ctx, symbol, window)
	case "kraken":
		return ps.krakenClient.GetOHLCV(ctx, symbol, window)
	case "coinbase":
		return ps.coinbaseClient.GetCandles(ctx, symbol, window)
	default:
		return nil, fmt.Errorf("unsupported exchange: %s", exchange)
	}
}

// tryAggregatorFallback uses aggregator data as labeled fallback
func (ps *PriceSource) tryAggregatorFallback(ctx context.Context, symbol, window string) ([]PriceBar, error) {
	// Mock aggregator fallback - in production, use CoinGecko or similar
	// This should be clearly labeled as non-exchange-native

	now := time.Now()
	bars := []PriceBar{}

	// Generate 48 hours of mock data
	for i := 48; i >= 0; i-- {
		timestamp := now.Add(time.Duration(-i) * time.Hour)
		price := 100.0 + float64(i)*0.1 // Mock price movement

		bar := PriceBar{
			Timestamp: timestamp,
			Open:      price * 0.999,
			High:      price * 1.002,
			Low:       price * 0.998,
			Close:     price,
			Volume:    1000.0 + float64(i)*10.0,
			Source:    "aggregator_fallback",
		}
		bars = append(bars, bar)
	}

	return bars, nil
}

// Exchange client stubs - in production these would be full implementations
type BinanceClient struct{}
type KrakenClient struct{}
type CoinbaseClient struct{}

func NewBinanceClient() *BinanceClient {
	return &BinanceClient{}
}

func NewKrakenClient() *KrakenClient {
	return &KrakenClient{}
}

func NewCoinbaseClient() *CoinbaseClient {
	return &CoinbaseClient{}
}

// Mock implementations - in production these would make actual API calls
func (bc *BinanceClient) GetKlines(ctx context.Context, symbol, window string) ([]PriceBar, error) {
	// Mock Binance kline data
	return generateMockExchangeData(symbol, window, "binance"), nil
}

func (kc *KrakenClient) GetOHLCV(ctx context.Context, symbol, window string) ([]PriceBar, error) {
	// Mock Kraken OHLCV data
	return generateMockExchangeData(symbol, window, "kraken"), nil
}

func (cc *CoinbaseClient) GetCandles(ctx context.Context, symbol, window string) ([]PriceBar, error) {
	// Mock Coinbase candle data
	return generateMockExchangeData(symbol, window, "coinbase"), nil
}

// generateMockExchangeData creates realistic exchange-native mock data
func generateMockExchangeData(symbol, window, exchange string) []PriceBar {
	now := time.Now()
	bars := []PriceBar{}

	// Generate appropriate number of bars based on window
	numBars := 48 // Default for hourly data
	interval := time.Hour

	switch window {
	case "1h":
		numBars = 48
		interval = time.Hour
	case "24h":
		numBars = 30
		interval = 24 * time.Hour
	case "7d":
		numBars = 52
		interval = 7 * 24 * time.Hour
	}

	for i := numBars; i >= 0; i-- {
		timestamp := now.Add(time.Duration(-i) * interval)
		basePrice := 100.0

		// Add some exchange-specific price variation
		switch exchange {
		case "binance":
			basePrice = 100.0 + float64(i)*0.05
		case "kraken":
			basePrice = 100.1 + float64(i)*0.06 // Slightly different prices
		case "coinbase":
			basePrice = 99.9 + float64(i)*0.04
		}

		bar := PriceBar{
			Timestamp: timestamp,
			Open:      basePrice * 0.999,
			High:      basePrice * 1.003,
			Low:       basePrice * 0.997,
			Close:     basePrice,
			Volume:    1500.0 + float64(i)*25.0,
			Source:    fmt.Sprintf("exchange_native_%s", exchange),
		}
		bars = append(bars, bar)
	}

	return bars
}
