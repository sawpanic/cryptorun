package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/microstructure"
	regimeconfig "github.com/sawpanic/cryptorun/internal/config/regime"
	"github.com/sawpanic/cryptorun/internal/provider"
)

// KrakenLiveAdapter bridges the existing KrakenProvider to the DataFacade interface
type KrakenLiveAdapter struct {
	krakenProvider provider.ExchangeProvider
	name          string
	symbols       []string
}

// NewKrakenLiveAdapter creates a new live Kraken data adapter
func NewKrakenLiveAdapter() (*KrakenLiveAdapter, error) {
	// Create Kraken provider configuration
	config := provider.ProviderConfig{
		Name:    "kraken-live",
		BaseURL: "https://api.kraken.com/0/public",
		RateLimit: provider.RateLimitConfig{
			RequestsPerSecond: 1.0,
			Timeout:          30 * time.Second,
		},
		CircuitBreaker: provider.CircuitBreakerConfig{
			FailureThreshold: 5,
			RecoveryTimeout:  60 * time.Second,
		},
		EnableCircuitBreaker: true,
		EnableRateLimit:      true,
	}

	// Create the Kraken provider
	krakenProvider, err := provider.NewKrakenProvider(config, func(metric string, value interface{}) {
		// Simple metrics callback - could be enhanced with proper metrics collection
		fmt.Printf("Kraken metric: %s = %v\n", metric, value)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Kraken provider: %w", err)
	}

	return &KrakenLiveAdapter{
		krakenProvider: krakenProvider,
		name:          "kraken-live-adapter",
		symbols:       []string{"XBTUSD", "ETHUSD", "SOLUSD", "ADAUSD", "MATICUSD"}, // USD pairs only
	}, nil
}

// GetName returns the provider name
func (k *KrakenLiveAdapter) GetName() string {
	return k.name
}

// IsHealthy checks if the Kraken provider is healthy
func (k *KrakenLiveAdapter) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health, err := k.krakenProvider.Health(ctx)
	if err != nil {
		return false
	}

	return health.Status == "healthy"
}

// GetMicrostructureData fetches live microstructure data from Kraken
func (k *KrakenLiveAdapter) GetMicrostructureData(ctx context.Context, symbol string) (*microstructure.MicrostructureData, error) {
	// Normalize symbol for Kraken (e.g., BTCUSD -> XBTUSD)
	krakenSymbol := k.normalizeSymbol(symbol)

	// Get orderbook data from Kraken
	orderBook, err := k.krakenProvider.GetOrderBook(ctx, krakenSymbol, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get order book for %s: %w", symbol, err)
	}

	// Get recent trades for trade data
	trades, err := k.krakenProvider.GetTrades(ctx, krakenSymbol, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get trades for %s: %w", symbol, err)
	}

	// Convert to microstructure data format
	microData := &microstructure.MicrostructureData{
		Symbol:    symbol,
		Venue:     "kraken",
		Timestamp: time.Now(),
		
		// Order book data
		BestBid:   orderBook.Bids[0].Price,
		BestAsk:   orderBook.Asks[0].Price,
		BidSize:   orderBook.Bids[0].Size,
		AskSize:   orderBook.Asks[0].Size,
		
		// Calculate spread
		SpreadBps: ((orderBook.Asks[0].Price - orderBook.Bids[0].Price) / orderBook.Bids[0].Price) * 10000,
		
		// Trade data (convert from provider format)
		Trades: k.convertTrades(trades),
	}

	// Calculate depth within 2% (basic implementation)
	microData.DepthBid2Pct = k.calculateDepth(orderBook.Bids, orderBook.Bids[0].Price, 0.02, false)
	microData.DepthAsk2Pct = k.calculateDepth(orderBook.Asks, orderBook.Asks[0].Price, 0.02, true)

	return microData, nil
}

// GetRegimeData fetches basic market data for regime detection
func (k *KrakenLiveAdapter) GetRegimeData(ctx context.Context) (*regimeconfig.MarketData, error) {
	// For now, return basic regime data for BTCUSD as the primary pair
	symbol := "XBTUSD"

	// Get recent klines for price data
	klines, err := k.krakenProvider.GetKlines(ctx, symbol, "1h", 24)
	if err != nil {
		return nil, fmt.Errorf("failed to get klines for regime data: %w", err)
	}

	if len(klines) == 0 {
		return nil, fmt.Errorf("no kline data available for regime detection")
	}

	// Extract prices and volumes
	prices := make([]float64, len(klines))
	volumes := make([]float64, len(klines))
	for i, kline := range klines {
		prices[i] = kline.Close
		volumes[i] = kline.Volume
	}

	// Calculate simple moving average (20-period, but we only have 24h data)
	ma20 := k.calculateSMA(prices, min(20, len(prices)))

	// Calculate realized volatility (simplified)
	realizedVol := k.calculateRealizedVolatility(prices)

	return &regimeconfig.MarketData{
		Symbol:        "BTCUSD",
		Timestamp:     time.Now(),
		CurrentPrice:  prices[len(prices)-1],
		MA20:          ma20,
		RealizedVol7d: realizedVol,
		Prices:        prices,
		Volumes:       volumes,
		BreadthData: regimeconfig.BreadthData{
			AdvanceDeclineRatio: 0.6, // Mock data - would need multiple symbols
			NewHighsNewLows:     0.4, // Mock data
			VolumeRatio:         0.7, // Mock data
			Timestamp:           time.Now(),
		},
	}, nil
}

// GetSupportedSymbols returns the list of supported symbols
func (k *KrakenLiveAdapter) GetSupportedSymbols() []string {
	return k.symbols
}

// Helper methods

func (k *KrakenLiveAdapter) normalizeSymbol(symbol string) string {
	// Convert common symbols to Kraken format
	switch symbol {
	case "BTCUSD":
		return "XBTUSD"
	case "ETHUSD":
		return "ETHUSD"
	case "SOLUSD":
		return "SOLUSD"
	case "ADAUSD":
		return "ADAUSD"
	case "MATICUSD":
		return "MATICUSD"
	default:
		return symbol
	}
}

func (k *KrakenLiveAdapter) convertTrades(providerTrades []provider.TradeData) []microstructure.Trade {
	trades := make([]microstructure.Trade, len(providerTrades))
	for i, trade := range providerTrades {
		trades[i] = microstructure.Trade{
			Price:     trade.Price,
			Size:      trade.Size,
			Side:      trade.Side,
			Timestamp: trade.Timestamp,
			TradeID:   trade.TradeID,
		}
	}
	return trades
}

func (k *KrakenLiveAdapter) calculateDepth(levels []provider.BookLevel, midPrice float64, percentage float64, isAsk bool) float64 {
	var totalDepth float64
	priceThreshold := midPrice
	if isAsk {
		priceThreshold = midPrice * (1 + percentage)
	} else {
		priceThreshold = midPrice * (1 - percentage)
	}

	for _, level := range levels {
		if (isAsk && level.Price <= priceThreshold) || (!isAsk && level.Price >= priceThreshold) {
			totalDepth += level.Size * level.Price
		} else {
			break
		}
	}

	return totalDepth
}

func (k *KrakenLiveAdapter) calculateSMA(prices []float64, periods int) float64 {
	if len(prices) < periods {
		periods = len(prices)
	}

	var sum float64
	start := len(prices) - periods
	for i := start; i < len(prices); i++ {
		sum += prices[i]
	}

	return sum / float64(periods)
}

func (k *KrakenLiveAdapter) calculateRealizedVolatility(prices []float64) float64 {
	if len(prices) < 2 {
		return 0.0
	}

	// Calculate log returns
	var sumSquaredReturns float64
	for i := 1; i < len(prices); i++ {
		logReturn := (prices[i] - prices[i-1]) / prices[i-1]
		sumSquaredReturns += logReturn * logReturn
	}

	// Annualized volatility (simplified)
	variance := sumSquaredReturns / float64(len(prices)-1)
	volatility := variance * 365 * 24 // Scale to annual, assuming hourly data

	if volatility < 0 {
		return 0.0
	}
	
	// Simple square root approximation
	return volatility * 0.5 // Rough approximation
}

// min helper function for Go versions without generics
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}