package microstructure

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ExchangeAdapter defines the interface for exchange-native microstructure data
type ExchangeAdapter interface {
	GetName() string
	IsSupported(symbol string) bool
	GetMicrostructureData(ctx context.Context, symbol string) (MicrostructureData, error)
	GetBatchMicrostructureData(ctx context.Context, symbols []string) ([]MicrostructureData, []error)
	GetSupportedSymbols() []string
	GetRateLimit() RateLimit
	ValidateConnection() error
}

// BaseExchangeAdapter provides common functionality for all exchanges
type BaseExchangeAdapter struct {
	Name               string
	BaseURL            string
	WSBaseURL          string
	SupportedSymbols   []string
	RateLimitRequests  int
	RateLimitWindow    time.Duration
	APIKey             string // Optional for public endpoints
	RequestsUsed       int
	LastResetTime      time.Time
}

// KrakenAdapter implements ExchangeAdapter for Kraken
type KrakenAdapter struct {
	BaseExchangeAdapter
}

// NewKrakenAdapter creates a new Kraken exchange adapter
func NewKrakenAdapter() *KrakenAdapter {
	return &KrakenAdapter{
		BaseExchangeAdapter: BaseExchangeAdapter{
			Name:              "kraken",
			BaseURL:           "https://api.kraken.com",
			WSBaseURL:         "wss://ws.kraken.com",
			RateLimitRequests: 180,                   // 180 requests per minute for public endpoints
			RateLimitWindow:   time.Minute,
			SupportedSymbols:  getKrakenUSDPairs(),
			LastResetTime:     time.Now(),
		},
	}
}

func (k *KrakenAdapter) GetName() string {
	return k.Name
}

func (k *KrakenAdapter) IsSupported(symbol string) bool {
	krakenSymbol := convertToKrakenSymbol(symbol)
	for _, supported := range k.SupportedSymbols {
		if supported == krakenSymbol {
			return true
		}
	}
	return false
}

func (k *KrakenAdapter) GetMicrostructureData(ctx context.Context, symbol string) (MicrostructureData, error) {
	if !k.IsSupported(symbol) {
		return MicrostructureData{}, fmt.Errorf("symbol %s not supported by Kraken", symbol)
	}

	krakenSymbol := convertToKrakenSymbol(symbol)
	
	// In a real implementation, this would make HTTP requests to Kraken's API
	// For now, we'll return mock data that demonstrates the structure
	data := MicrostructureData{
		Symbol:    symbol,
		Exchange:  k.Name,
		Timestamp: time.Now(),
		BestBid:   50000.0,
		BestAsk:   50025.0,
		BidSize:   1.5,
		AskSize:   2.0,
		OrderBook: OrderBook{
			Bids: []OrderLevel{
				{Price: 50000.0, Size: 1.5, SizeUSD: 75000.0, OrderCount: 3},
				{Price: 49995.0, Size: 2.0, SizeUSD: 99990.0, OrderCount: 2},
				{Price: 49990.0, Size: 3.0, SizeUSD: 149970.0, OrderCount: 5},
				{Price: 49985.0, Size: 1.0, SizeUSD: 49985.0, OrderCount: 1},
				{Price: 49980.0, Size: 2.5, SizeUSD: 124950.0, OrderCount: 4},
			},
			Asks: []OrderLevel{
				{Price: 50025.0, Size: 2.0, SizeUSD: 100050.0, OrderCount: 2},
				{Price: 50030.0, Size: 1.5, SizeUSD: 75045.0, OrderCount: 1},
				{Price: 50035.0, Size: 3.0, SizeUSD: 150105.0, OrderCount: 3},
				{Price: 50040.0, Size: 1.0, SizeUSD: 50040.0, OrderCount: 1},
				{Price: 50045.0, Size: 2.0, SizeUSD: 100090.0, OrderCount: 2},
			},
			Timestamp: time.Now(),
			Sequence:  12345678,
		},
		RecentTrades: []Trade{
			{Price: 50010.0, Size: 0.5, Side: "buy", Timestamp: time.Now().Add(-30 * time.Second), TradeID: "t1"},
			{Price: 50005.0, Size: 0.8, Side: "sell", Timestamp: time.Now().Add(-60 * time.Second), TradeID: "t2"},
			{Price: 50015.0, Size: 0.3, Side: "buy", Timestamp: time.Now().Add(-90 * time.Second), TradeID: "t3"},
		},
		Volume24h:         50000000.0,  // $50M
		MarketCap:         1000000000000.0, // $1T
		CirculatingSupply: 19500000.0,  // ~19.5M BTC
		Metadata: MicrostructureMetadata{
			DataSource:       "kraken_rest_api",
			LastUpdate:       time.Now(),
			Staleness:        5.0, // 5 seconds old
			IsExchangeNative: true,
			APIEndpoint:      fmt.Sprintf("%s/0/public/Depth?pair=%s", k.BaseURL, krakenSymbol),
			RateLimit:        k.GetRateLimit(),
		},
	}

	// Update rate limit tracking
	k.RequestsUsed++
	
	return data, nil
}

func (k *KrakenAdapter) GetBatchMicrostructureData(ctx context.Context, symbols []string) ([]MicrostructureData, []error) {
	results := make([]MicrostructureData, len(symbols))
	errors := make([]error, len(symbols))
	
	// For efficiency, Kraken supports batch requests, but we'll simulate individual requests
	for i, symbol := range symbols {
		data, err := k.GetMicrostructureData(ctx, symbol)
		results[i] = data
		errors[i] = err
	}
	
	return results, errors
}

func (k *KrakenAdapter) GetSupportedSymbols() []string {
	return k.SupportedSymbols
}

func (k *KrakenAdapter) GetRateLimit() RateLimit {
	now := time.Now()
	
	// Reset rate limit if window has passed
	if now.Sub(k.LastResetTime) > k.RateLimitWindow {
		k.RequestsUsed = 0
		k.LastResetTime = now
	}
	
	return RateLimit{
		RequestsUsed:      k.RequestsUsed,
		RequestsRemaining: k.RateLimitRequests - k.RequestsUsed,
		ResetTimestamp:    k.LastResetTime.Add(k.RateLimitWindow).Unix(),
	}
}

func (k *KrakenAdapter) ValidateConnection() error {
	// In a real implementation, this would make a test API call
	if k.RateLimitRequests <= 0 {
		return fmt.Errorf("invalid rate limit configuration")
	}
	return nil
}

// BinanceAdapter implements ExchangeAdapter for Binance
type BinanceAdapter struct {
	BaseExchangeAdapter
}

func NewBinanceAdapter() *BinanceAdapter {
	return &BinanceAdapter{
		BaseExchangeAdapter: BaseExchangeAdapter{
			Name:              "binance",
			BaseURL:           "https://api.binance.com",
			WSBaseURL:         "wss://stream.binance.com:9443",
			RateLimitRequests: 1200, // 1200 requests per minute
			RateLimitWindow:   time.Minute,
			SupportedSymbols:  getBinanceUSDTPairs(),
			LastResetTime:     time.Now(),
		},
	}
}

func (b *BinanceAdapter) GetName() string {
	return b.Name
}

func (b *BinanceAdapter) IsSupported(symbol string) bool {
	binanceSymbol := convertToBinanceSymbol(symbol)
	for _, supported := range b.SupportedSymbols {
		if supported == binanceSymbol {
			return true
		}
	}
	return false
}

func (b *BinanceAdapter) GetMicrostructureData(ctx context.Context, symbol string) (MicrostructureData, error) {
	if !b.IsSupported(symbol) {
		return MicrostructureData{}, fmt.Errorf("symbol %s not supported by Binance", symbol)
	}

	binanceSymbol := convertToBinanceSymbol(symbol)
	
	// Mock data for Binance (similar structure to Kraken but with Binance-specific values)
	data := MicrostructureData{
		Symbol:    symbol,
		Exchange:  b.Name,
		Timestamp: time.Now(),
		BestBid:   50001.0,
		BestAsk:   50026.0,
		BidSize:   2.1,
		AskSize:   1.8,
		OrderBook: OrderBook{
			Bids: []OrderLevel{
				{Price: 50001.0, Size: 2.1, SizeUSD: 105002.1, OrderCount: 4},
				{Price: 49996.0, Size: 1.8, SizeUSD: 89992.8, OrderCount: 3},
				{Price: 49991.0, Size: 2.5, SizeUSD: 124977.5, OrderCount: 6},
			},
			Asks: []OrderLevel{
				{Price: 50026.0, Size: 1.8, SizeUSD: 90046.8, OrderCount: 3},
				{Price: 50031.0, Size: 2.2, SizeUSD: 110068.2, OrderCount: 4},
				{Price: 50036.0, Size: 1.5, SizeUSD: 75054.0, OrderCount: 2},
			},
			Timestamp: time.Now(),
			Sequence:  87654321,
		},
		RecentTrades: []Trade{
			{Price: 50012.0, Size: 0.3, Side: "buy", Timestamp: time.Now().Add(-15 * time.Second), TradeID: "b1"},
			{Price: 50008.0, Size: 0.6, Side: "sell", Timestamp: time.Now().Add(-45 * time.Second), TradeID: "b2"},
		},
		Volume24h:         45000000.0,
		MarketCap:         1000000000000.0,
		CirculatingSupply: 19500000.0,
		Metadata: MicrostructureMetadata{
			DataSource:       "binance_rest_api",
			LastUpdate:       time.Now(),
			Staleness:        3.0,
			IsExchangeNative: true,
			APIEndpoint:      fmt.Sprintf("%s/api/v3/depth?symbol=%s", b.BaseURL, binanceSymbol),
			RateLimit:        b.GetRateLimit(),
		},
	}

	b.RequestsUsed++
	return data, nil
}

func (b *BinanceAdapter) GetBatchMicrostructureData(ctx context.Context, symbols []string) ([]MicrostructureData, []error) {
	results := make([]MicrostructureData, len(symbols))
	errors := make([]error, len(symbols))
	
	for i, symbol := range symbols {
		data, err := b.GetMicrostructureData(ctx, symbol)
		results[i] = data
		errors[i] = err
	}
	
	return results, errors
}

func (b *BinanceAdapter) GetSupportedSymbols() []string {
	return b.SupportedSymbols
}

func (b *BinanceAdapter) GetRateLimit() RateLimit {
	now := time.Now()
	
	if now.Sub(b.LastResetTime) > b.RateLimitWindow {
		b.RequestsUsed = 0
		b.LastResetTime = now
	}
	
	return RateLimit{
		RequestsUsed:      b.RequestsUsed,
		RequestsRemaining: b.RateLimitRequests - b.RequestsUsed,
		ResetTimestamp:    b.LastResetTime.Add(b.RateLimitWindow).Unix(),
	}
}

func (b *BinanceAdapter) ValidateConnection() error {
	if b.RateLimitRequests <= 0 {
		return fmt.Errorf("invalid rate limit configuration")
	}
	return nil
}

// ExchangeManager manages multiple exchange adapters
type ExchangeManager struct {
	adapters       map[string]ExchangeAdapter
	primaryExchange string
	fallbackOrder   []string
}

// NewExchangeManager creates a new exchange manager
func NewExchangeManager() *ExchangeManager {
	manager := &ExchangeManager{
		adapters:      make(map[string]ExchangeAdapter),
		primaryExchange: "kraken", // CryptoRun prefers Kraken
		fallbackOrder: []string{"kraken", "binance", "okx", "coinbase"},
	}
	
	// Register exchange adapters
	manager.RegisterAdapter(NewKrakenAdapter())
	manager.RegisterAdapter(NewBinanceAdapter())
	// Additional adapters would be registered here
	
	return manager
}

func (em *ExchangeManager) RegisterAdapter(adapter ExchangeAdapter) {
	em.adapters[adapter.GetName()] = adapter
}

func (em *ExchangeManager) GetMicrostructureData(ctx context.Context, symbol string) (MicrostructureData, error) {
	// Try primary exchange first
	if adapter, exists := em.adapters[em.primaryExchange]; exists {
		if adapter.IsSupported(symbol) {
			return adapter.GetMicrostructureData(ctx, symbol)
		}
	}
	
	// Try fallback exchanges in order
	for _, exchangeName := range em.fallbackOrder {
		if adapter, exists := em.adapters[exchangeName]; exists {
			if adapter.IsSupported(symbol) {
				return adapter.GetMicrostructureData(ctx, symbol)
			}
		}
	}
	
	return MicrostructureData{}, fmt.Errorf("symbol %s not supported by any exchange", symbol)
}

func (em *ExchangeManager) GetBestExchangeForSymbol(symbol string) string {
	// Try primary first
	if adapter, exists := em.adapters[em.primaryExchange]; exists {
		if adapter.IsSupported(symbol) {
			return em.primaryExchange
		}
	}
	
	// Try fallbacks
	for _, exchangeName := range em.fallbackOrder {
		if adapter, exists := em.adapters[exchangeName]; exists {
			if adapter.IsSupported(symbol) {
				return exchangeName
			}
		}
	}
	
	return ""
}

// Helper functions for symbol conversion

func convertToKrakenSymbol(symbol string) string {
	// Convert standard format (BTC-USD) to Kraken format (XBTUSD)
	parts := strings.Split(symbol, "-")
	if len(parts) != 2 {
		return symbol
	}
	
	base := parts[0]
	quote := parts[1]
	
	// Kraken symbol mappings
	if base == "BTC" {
		base = "XBT"
	}
	
	return base + quote
}

func convertToBinanceSymbol(symbol string) string {
	// Convert standard format (BTC-USD) to Binance format (BTCUSDT)
	parts := strings.Split(symbol, "-")
	if len(parts) != 2 {
		return symbol
	}
	
	base := parts[0]
	quote := parts[1]
	
	// Binance uses USDT instead of USD for most pairs
	if quote == "USD" {
		quote = "USDT"
	}
	
	return base + quote
}

func getKrakenUSDPairs() []string {
	return []string{
		"XBTUSD", "ETHUSD", "ADAUSD", "DOTUSD", "LINKUSD",
		"LTCUSD", "XLMUSD", "XRPUSD", "SOLUSD", "MATICUSD",
		"AVAXUSD", "ATOMUSD", "ALGOUSD", "FILUSD", "UNIUSD",
		"AAVEUSD", "COMPUSD", "YFIUSD", "SNXUSD", "MKRUSD",
	}
}

func getBinanceUSDTPairs() []string {
	return []string{
		"BTCUSDT", "ETHUSDT", "ADAUSDT", "DOTUSDT", "LINKUSDT",
		"LTCUSDT", "XLMUSDT", "XRPUSDT", "SOLUSDT", "MATICUSDT",
		"AVAXUSDT", "ATOMUSDT", "ALGOUSDT", "FILUSDT", "UNIUSDT",
		"AAVEUSDT", "COMPUSDT", "YFIUSDT", "SNXUSDT", "MKRUSDT",
		"BNBUSDT", "FTMUSDT", "HBARUSDT", "ICPUSDT", "VETUSDT",
	}
}

// Mock implementations for OKX and Coinbase would follow similar patterns
// For brevity, focusing on Kraken and Binance as the primary examples