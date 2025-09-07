package fake

import (
	"context"
	"crypto/md5"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sawpanic/cryptorun/internal/data/interfaces"
)

// Adapter implements Exchange interface with deterministic fake data for testing
type Adapter struct {
	name   string
	seed   int64
	
	// Configuration
	priceBase   map[string]float64 // Base prices for symbols
	volatility  float64             // Daily volatility (0.02 = 2%)
	trendBias   float64             // Upward/downward bias (-0.5 to 0.5)
	
	// State tracking for deterministic behavior
	lastUpdate time.Time
	sequence    int64
	
	// Callbacks
	tradesCallbacks map[string]interfaces.TradesCallback
	bookCallbacks   map[string]interfaces.BookL2Callback
	klinesCallbacks map[string]interfaces.KlinesCallback
}

// NewAdapter creates a deterministic fake exchange adapter
func NewAdapter(name string, seed int64) *Adapter {
	return &Adapter{
		name:            name,
		seed:            seed,
		volatility:      0.02, // 2% daily volatility
		trendBias:       0.0,  // No trend bias by default
		priceBase:       getDefaultPrices(),
		lastUpdate:      time.Now(),
		tradesCallbacks: make(map[string]interfaces.TradesCallback),
		bookCallbacks:   make(map[string]interfaces.BookL2Callback),
		klinesCallbacks: make(map[string]interfaces.KlinesCallback),
	}
}

// NewDeterministicAdapter creates an adapter with deterministic seed based on name
func NewDeterministicAdapter(name string) *Adapter {
	// Generate deterministic seed from name
	hash := md5.Sum([]byte(name))
	seed := int64(hash[0])<<56 | int64(hash[1])<<48 | int64(hash[2])<<40 | int64(hash[3])<<32 |
		   int64(hash[4])<<24 | int64(hash[5])<<16 | int64(hash[6])<<8 | int64(hash[7])
	
	adapter := NewAdapter(name, seed)
	log.Info().Str("venue", name).Int64("seed", seed).Msg("Created deterministic fake adapter")
	return adapter
}

// SetVolatility configures the volatility for price generation
func (a *Adapter) SetVolatility(volatility float64) {
	a.volatility = volatility
}

// SetTrendBias configures directional bias for price movement
func (a *Adapter) SetTrendBias(bias float64) {
	a.trendBias = bias
}

// SetBasePrice sets the base price for a symbol
func (a *Adapter) SetBasePrice(symbol string, price float64) {
	a.priceBase[strings.ToUpper(symbol)] = price
}

// Name returns the exchange name
func (a *Adapter) Name() string {
	return a.name
}

// ConnectWS simulates WebSocket connection (always succeeds)
func (a *Adapter) ConnectWS(ctx context.Context) error {
	log.Info().Str("venue", a.name).Msg("Fake WebSocket connection established")
	return nil
}

// SubscribeTrades stores callback for trade data generation
func (a *Adapter) SubscribeTrades(symbol string, callback interfaces.TradesCallback) error {
	normalizedSymbol := a.NormalizeSymbol(symbol)
	a.tradesCallbacks[normalizedSymbol] = callback
	
	log.Info().Str("venue", a.name).Str("symbol", normalizedSymbol).
		Msg("Subscribed to fake trades")
	
	// Start generating fake trades
	go a.generateTrades(normalizedSymbol, callback)
	
	return nil
}

// SubscribeBookL2 stores callback for orderbook data generation
func (a *Adapter) SubscribeBookL2(symbol string, callback interfaces.BookL2Callback) error {
	normalizedSymbol := a.NormalizeSymbol(symbol)
	a.bookCallbacks[normalizedSymbol] = callback
	
	log.Info().Str("venue", a.name).Str("symbol", normalizedSymbol).
		Msg("Subscribed to fake orderbook")
		
	// Start generating fake orderbook updates
	go a.generateBookUpdates(normalizedSymbol, callback)
	
	return nil
}

// StreamKlines stores callback for kline data generation
func (a *Adapter) StreamKlines(symbol string, interval string, callback interfaces.KlinesCallback) error {
	normalizedSymbol := a.NormalizeSymbol(symbol)
	key := fmt.Sprintf("%s:%s", normalizedSymbol, interval)
	a.klinesCallbacks[key] = callback
	
	log.Info().Str("venue", a.name).Str("symbol", normalizedSymbol).
		Str("interval", interval).Msg("Subscribed to fake klines")
		
	// Start generating fake kline updates
	go a.generateKlines(normalizedSymbol, interval, callback)
	
	return nil
}

// GetKlines generates deterministic historical klines
func (a *Adapter) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]interfaces.Kline, error) {
	normalizedSymbol := a.NormalizeSymbol(symbol)
	
	// Generate deterministic price movements
	klines := a.generateHistoricalKlines(normalizedSymbol, interval, limit)
	
	log.Debug().Str("venue", a.name).Str("symbol", symbol).
		Int("count", len(klines)).Msg("Generated fake klines")
	
	return klines, nil
}

// GetTrades generates deterministic recent trades
func (a *Adapter) GetTrades(ctx context.Context, symbol string, limit int) ([]interfaces.Trade, error) {
	normalizedSymbol := a.NormalizeSymbol(symbol)
	
	trades := a.generateHistoricalTrades(normalizedSymbol, limit)
	
	log.Debug().Str("venue", a.name).Str("symbol", symbol).
		Int("count", len(trades)).Msg("Generated fake trades")
	
	return trades, nil
}

// GetBookL2 generates a deterministic orderbook snapshot
func (a *Adapter) GetBookL2(ctx context.Context, symbol string) (*interfaces.BookL2, error) {
	normalizedSymbol := a.NormalizeSymbol(symbol)
	
	book := a.generateOrderBook(normalizedSymbol)
	
	log.Debug().Str("venue", a.name).Str("symbol", symbol).
		Int("bids", len(book.Bids)).Int("asks", len(book.Asks)).
		Msg("Generated fake orderbook")
	
	return book, nil
}

// NormalizeSymbol converts symbol to exchange format
func (a *Adapter) NormalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.ReplaceAll(symbol, "/", ""))
}

// NormalizeInterval converts interval to exchange format
func (a *Adapter) NormalizeInterval(interval string) string {
	return strings.ToLower(interval)
}

// Health returns simulated health status
func (a *Adapter) Health() interfaces.HealthStatus {
	return interfaces.HealthStatus{
		Venue:        a.name,
		Status:       "healthy",
		LastSeen:     time.Now(),
		ErrorRate:    0.001, // Very low error rate for fake data
		P99Latency:   50 * time.Millisecond,
		WSConnected:  true,
		RESTHealthy:  true,
		Recommendation: "",
	}
}

// Helper methods for data generation

func (a *Adapter) getPrice(symbol string, timestamp time.Time) float64 {
	basePrice, exists := a.priceBase[symbol]
	if !exists {
		basePrice = 50000.0 // Default BTC-like price
	}
	
	// Create deterministic but realistic price movements
	rng := rand.New(rand.NewSource(a.seed + timestamp.Unix()))
	
	// Calculate time-based drift (trend)
	hours := float64(timestamp.Sub(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)).Hours())
	trendComponent := a.trendBias * hours * 0.001 // Small trend over time
	
	// Add random walk component
	randomWalk := rng.NormFloat64() * a.volatility * basePrice * 0.1
	
	// Add some realistic volatility clustering
	volatilityCluster := math.Sin(hours*0.1) * a.volatility * basePrice * 0.05
	
	return basePrice * (1 + trendComponent) + randomWalk + volatilityCluster
}

func (a *Adapter) generateHistoricalKlines(symbol string, interval string, limit int) []interfaces.Kline {
	var klines []interfaces.Kline
	
	// Parse interval to duration
	intervalDuration := parseInterval(interval)
	
	// Generate historical data starting from limit intervals ago
	startTime := time.Now().Truncate(intervalDuration).Add(-time.Duration(limit) * intervalDuration)
	
	for i := 0; i < limit; i++ {
		timestamp := startTime.Add(time.Duration(i) * intervalDuration)
		
		// Generate OHLC data with some correlation
		open := a.getPrice(symbol, timestamp)
		close := a.getPrice(symbol, timestamp.Add(intervalDuration))
		
		// High and low based on open/close with some randomness
		rng := rand.New(rand.NewSource(a.seed + timestamp.Unix() + int64(i)))
		
		rangePct := 0.02 * rng.Float64() // 0-2% range
		high := math.Max(open, close) * (1 + rangePct)
		low := math.Min(open, close) * (1 - rangePct)
		
		// Volume correlated with price movement and volatility
		priceMove := math.Abs(close-open) / open
		volume := 100 + priceMove*1000 + rng.Float64()*200
		
		kline := interfaces.Kline{
			Symbol:    symbol,
			Venue:     a.name,
			Timestamp: timestamp,
			Interval:  interval,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			QuoteVol:  volume * (open+high+low+close)/4, // Approximate
		}
		
		klines = append(klines, kline)
	}
	
	return klines
}

func (a *Adapter) generateHistoricalTrades(symbol string, limit int) []interfaces.Trade {
	var trades []interfaces.Trade
	
	baseTime := time.Now().Add(-time.Hour) // Start 1 hour ago
	currentPrice := a.getPrice(symbol, baseTime)
	
	for i := 0; i < limit; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Second * 10) // Every 10 seconds
		
		// Small random price movements
		rng := rand.New(rand.NewSource(a.seed + timestamp.Unix() + int64(i)))
		priceMove := (rng.Float64() - 0.5) * 0.001 * currentPrice // ±0.1%
		currentPrice += priceMove
		
		// Random trade size
		size := 0.1 + rng.Float64()*2.0 // 0.1 to 2.1
		
		// Random side with slight buy bias
		side := "buy"
		if rng.Float64() < 0.48 {
			side = "sell"
		}
		
		trade := interfaces.Trade{
			Symbol:    symbol,
			Venue:     a.name,
			Timestamp: timestamp,
			Price:     currentPrice,
			Size:      size,
			Side:      side,
			TradeID:   fmt.Sprintf("fake_%d_%d", timestamp.Unix(), i),
		}
		
		trades = append(trades, trade)
	}
	
	return trades
}

func (a *Adapter) generateOrderBook(symbol string) *interfaces.BookL2 {
	currentPrice := a.getPrice(symbol, time.Now())
	
	// Generate spread (0.01% to 0.05%)
	rng := rand.New(rand.NewSource(a.seed + time.Now().Unix()))
	spread := currentPrice * (0.0001 + rng.Float64()*0.0004)
	
	bidPrice := currentPrice - spread/2
	askPrice := currentPrice + spread/2
	
	// Generate order book levels
	var bids, asks []interfaces.BookLevel
	
	// 10 levels on each side
	for i := 0; i < 10; i++ {
		bidLevelPrice := bidPrice - float64(i)*spread*0.2
		askLevelPrice := askPrice + float64(i)*spread*0.2
		
		// Size decreases with distance from mid
		sizeFactor := 1.0 / (1.0 + float64(i)*0.3)
		bidSize := (2.0 + rng.Float64()*3.0) * sizeFactor
		askSize := (2.0 + rng.Float64()*3.0) * sizeFactor
		
		bids = append(bids, interfaces.BookLevel{
			Price: bidLevelPrice,
			Size:  bidSize,
		})
		
		asks = append(asks, interfaces.BookLevel{
			Price: askLevelPrice,
			Size:  askSize,
		})
	}
	
	return &interfaces.BookL2{
		Symbol:    symbol,
		Venue:     a.name,
		Timestamp: time.Now(),
		Bids:      bids,
		Asks:      asks,
		Sequence:  a.sequence + 1,
	}
}

// Streaming data generation goroutines

func (a *Adapter) generateTrades(symbol string, callback interfaces.TradesCallback) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		trades := a.generateHistoricalTrades(symbol, 3) // 3 new trades every 5s
		if err := callback(trades); err != nil {
			log.Warn().Str("venue", a.name).Err(err).Msg("Trade callback error")
		}
	}
}

func (a *Adapter) generateBookUpdates(symbol string, callback interfaces.BookL2Callback) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		book := a.generateOrderBook(symbol)
		if err := callback(book); err != nil {
			log.Warn().Str("venue", a.name).Err(err).Msg("Book callback error")
		}
	}
}

func (a *Adapter) generateKlines(symbol string, interval string, callback interfaces.KlinesCallback) {
	intervalDuration := parseInterval(interval)
	ticker := time.NewTicker(intervalDuration)
	defer ticker.Stop()
	
	for range ticker.C {
		// Generate one new completed kline
		klines := a.generateHistoricalKlines(symbol, interval, 1)
		if err := callback(klines); err != nil {
			log.Warn().Str("venue", a.name).Err(err).Msg("Klines callback error")
		}
	}
}

// Helper functions

func getDefaultPrices() map[string]float64 {
	return map[string]float64{
		"BTCUSD":  67500.0,
		"ETHUSD":  3200.0,
		"SOLUSD":  150.0,
		"ADAUSD":  0.45,
		"LINKUSD": 14.0,
		"DOTUSD":  6.8,
		"MATICUSD": 0.85,
		"AVAXUSD":  35.0,
		"UNIUSD":   8.5,
		"LTCUSD":   82.0,
	}
}

func parseInterval(interval string) time.Duration {
	switch strings.ToLower(interval) {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return time.Hour
	}
}