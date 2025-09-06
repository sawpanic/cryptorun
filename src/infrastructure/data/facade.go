// Package data provides hot WebSocket streams and warm REST caching with point-in-time integrity
package data

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Bar represents a OHLCV candlestick bar
type Bar struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	Source    string    `json:"source"`
}

// BookSnapshot represents L2 order book snapshot
type BookSnapshot struct {
	Symbol    string      `json:"symbol"`
	Timestamp time.Time   `json:"timestamp"`
	Bids      []BookLevel `json:"bids"`
	Asks      []BookLevel `json:"asks"`
	Source    string      `json:"source"` // Exchange-native only
}

// BookLevel represents a single order book level
type BookLevel struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
}

// Trade represents a single trade execution
type Trade struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
	Size      float64   `json:"size"`
	Side      string    `json:"side"` // "buy" or "sell"
	Source    string    `json:"source"`
}

// Stream represents a hot data stream
type Stream interface {
	Subscribe(symbols []string, dataTypes []string) error
	Unsubscribe(symbols []string) error
	Trades() <-chan Trade
	Books() <-chan BookSnapshot
	Bars() <-chan Bar
	Close() error
	Health() StreamHealth
}

// StreamHealth represents connection health status
type StreamHealth struct {
	Connected     bool      `json:"connected"`
	LastMessage   time.Time `json:"last_message"`
	MessageCount  int64     `json:"message_count"`
	Reconnects    int       `json:"reconnects"`
	Exchange      string    `json:"exchange"`
	LatencyMs     float64   `json:"latency_ms"`
	ErrorCount    int       `json:"error_count"`
	LastError     string    `json:"last_error,omitempty"`
}

// KlineReq represents a request for historical kline data
type KlineReq struct {
	Symbol string    `json:"symbol"`
	TF     string    `json:"timeframe"` // "1m", "5m", "1h", "1d"
	Since  time.Time `json:"since"`
	Until  time.Time `json:"until"`
	Limit  int       `json:"limit,omitempty"`
}

// KlineResp represents kline response with PIT integrity
type KlineResp struct {
	Bars      []Bar     `json:"bars"`
	PIT       bool      `json:"point_in_time"` // True if from PIT snapshot
	Source    string    `json:"source"`
	CachedAt  time.Time `json:"cached_at,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// DataFacade provides unified access to hot and warm data
type DataFacade interface {
	// Hot data - WebSocket streams for real-time data
	HotSubscribe(symbols []string) (Stream, error)
	
	// Warm data - REST with TTL caching and PIT snapshots
	WarmKlines(req KlineReq) (KlineResp, error)
	
	// Exchange-native order books (no aggregators)
	L2Book(symbol string) (BookSnapshot, error)
	
	// Health and metrics
	Health() FacadeHealth
	Close() error
}

// FacadeHealth represents overall facade health
type FacadeHealth struct {
	HotStreams    map[string]StreamHealth `json:"hot_streams"`
	CacheHitRate  float64                 `json:"cache_hit_rate"`
	WarmSources   map[string]bool         `json:"warm_sources_healthy"`
	LastReconcile time.Time               `json:"last_reconcile"`
	ErrorCount    int                     `json:"error_count"`
	LastError     string                  `json:"last_error,omitempty"`
}

// DataFacadeImpl implements the DataFacade interface
type DataFacadeImpl struct {
	mu sync.RWMutex
	
	// Hot streams per exchange
	hotStreams map[string]Stream
	hotSet     map[string]bool // symbols in hot set
	
	// Cache and warm sources
	cache       CacheManager
	reconciler  Reconciler
	
	// Configuration
	config *Config
	
	// Health tracking
	health FacadeHealth
	ctx    context.Context
	cancel context.CancelFunc
}

// Config holds data facade configuration
type Config struct {
	// Hot stream settings
	HotExchanges []string `json:"hot_exchanges"` // ["binance", "okx", "coinbase", "kraken"]
	HotSetSize   int      `json:"hot_set_size"`  // Number of symbols for hot streams
	
	// TTL settings (seconds)
	TTLs map[string]int `json:"ttls"` // prices_hot: 5, prices_warm: 30, volumes_vadr: 120
	
	// Source authority rules
	Sources struct {
		PriceVolume   []string `json:"price_volume"`   // ["coingecko", "coinpaprika"]
		Microstructure []string `json:"microstructure"` // exchange-native only
		Fallback      []string `json:"fallback"`
	} `json:"sources"`
	
	// Reconciliation settings
	Reconcile struct {
		MaxDeviation  float64 `json:"max_deviation"` // 1% outlier threshold
		MinSources    int     `json:"min_sources"`   // Minimum sources required
		TrimmedMean   bool    `json:"trimmed_mean"`  // Use trimmed median
	} `json:"reconcile"`
}

// NewDataFacade creates a new data facade instance
func NewDataFacade(config *Config, cache CacheManager, reconciler Reconciler) *DataFacadeImpl {
	ctx, cancel := context.WithCancel(context.Background())
	
	df := &DataFacadeImpl{
		hotStreams: make(map[string]Stream),
		hotSet:     make(map[string]bool),
		cache:      cache,
		reconciler: reconciler,
		config:     config,
		ctx:        ctx,
		cancel:     cancel,
		health: FacadeHealth{
			HotStreams:   make(map[string]StreamHealth),
			WarmSources:  make(map[string]bool),
		},
	}
	
	// Initialize hot streams
	df.initializeHotStreams()
	
	return df
}

// initializeHotStreams sets up WebSocket connections to exchanges
func (df *DataFacadeImpl) initializeHotStreams() {
	df.mu.Lock()
	defer df.mu.Unlock()
	
	for _, exchange := range df.config.HotExchanges {
		stream, err := df.createExchangeStream(exchange)
		if err != nil {
			df.health.ErrorCount++
			df.health.LastError = fmt.Sprintf("Failed to create %s stream: %v", exchange, err)
			continue
		}
		df.hotStreams[exchange] = stream
		df.health.HotStreams[exchange] = StreamHealth{
			Exchange:  exchange,
			Connected: true,
		}
	}
}

// createExchangeStream creates a stream for a specific exchange
func (df *DataFacadeImpl) createExchangeStream(exchange string) (Stream, error) {
	// This would create actual WebSocket connections
	// For now, return a mock stream
	return NewMockStream(exchange), nil
}

// HotSubscribe subscribes to hot data streams for given symbols
func (df *DataFacadeImpl) HotSubscribe(symbols []string) (Stream, error) {
	df.mu.Lock()
	defer df.mu.Unlock()
	
	// Add symbols to hot set
	for _, symbol := range symbols {
		df.hotSet[symbol] = true
	}
	
	// Create multiplexed stream that combines all exchange streams
	multiplexer := NewMultiplexedStream(df.hotStreams, symbols)
	
	// Subscribe each exchange stream to the symbols
	for exchange, stream := range df.hotStreams {
		err := stream.Subscribe(symbols, []string{"trades", "books", "klines"})
		if err != nil {
			df.health.ErrorCount++
			df.health.LastError = fmt.Sprintf("Failed to subscribe %s: %v", exchange, err)
			
			// Update health status
			if health := df.health.HotStreams[exchange]; true {
				health.ErrorCount++
				health.LastError = err.Error()
				health.Connected = false
				df.health.HotStreams[exchange] = health
			}
			continue
		}
	}
	
	return multiplexer, nil
}

// WarmKlines retrieves kline data from cache or warm sources
func (df *DataFacadeImpl) WarmKlines(req KlineReq) (KlineResp, error) {
	// Check if symbol is in hot set
	df.mu.RLock()
	isHot := df.hotSet[req.Symbol]
	df.mu.RUnlock()
	
	// Determine TTL based on hot/warm status
	ttlKey := "prices_warm"
	if isHot {
		ttlKey = "prices_hot"
	}
	
	ttl := time.Duration(df.config.TTLs[ttlKey]) * time.Second
	
	// Try cache first
	cacheKey := fmt.Sprintf("klines:%s:%s:%d:%d", req.Symbol, req.TF, req.Since.Unix(), req.Until.Unix())
	
	if cached, found := df.cache.Get(cacheKey); found {
		if resp, ok := cached.(KlineResp); ok {
			// Update cache hit rate
			df.updateCacheHitRate(true)
			return resp, nil
		}
	}
	
	df.updateCacheHitRate(false)
	
	// Fetch from warm sources and reconcile
	bars, err := df.fetchAndReconcile(req)
	if err != nil {
		return KlineResp{}, fmt.Errorf("failed to fetch warm klines: %w", err)
	}
	
	// Create response with PIT integrity
	resp := KlineResp{
		Bars:      bars,
		PIT:       true,
		Source:    "reconciled",
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
	
	// Cache the result
	df.cache.Set(cacheKey, resp, ttl)
	
	return resp, nil
}

// L2Book retrieves order book from exchange-native sources only
func (df *DataFacadeImpl) L2Book(symbol string) (BookSnapshot, error) {
	// CRITICAL: Only exchange-native sources allowed for microstructure data
	// Never use aggregators for depth/spread data
	
	for _, exchange := range df.config.Sources.Microstructure {
		if stream, exists := df.hotStreams[exchange]; exists {
			// Try to get from hot stream first
			if book, err := df.getBookFromStream(stream, symbol); err == nil {
				return book, nil
			}
		}
		
		// Fall back to REST API for this exchange
		if book, err := df.fetchBookFromExchange(exchange, symbol); err == nil {
			return book, nil
		}
	}
	
	return BookSnapshot{}, fmt.Errorf("no exchange-native book data available for %s", symbol)
}

// fetchAndReconcile fetches data from multiple warm sources and reconciles
func (df *DataFacadeImpl) fetchAndReconcile(req KlineReq) ([]Bar, error) {
	sources := df.config.Sources.PriceVolume
	results := make(map[string][]Bar)
	
	// Fetch from each source
	for _, source := range sources {
		bars, err := df.fetchFromSource(source, req)
		if err != nil {
			df.health.WarmSources[source] = false
			continue
		}
		results[source] = bars
		df.health.WarmSources[source] = true
	}
	
	if len(results) == 0 {
		return nil, fmt.Errorf("no warm sources available")
	}
	
	// Reconcile using trimmed median
	reconciledBars, err := df.reconciler.ReconcileBars(results)
	if err != nil {
		return nil, fmt.Errorf("reconciliation failed: %w", err)
	}
	
	df.health.LastReconcile = time.Now()
	return reconciledBars, nil
}

// Health returns facade health status
func (df *DataFacadeImpl) Health() FacadeHealth {
	df.mu.RLock()
	defer df.mu.RUnlock()
	
	// Update stream health
	for exchange, stream := range df.hotStreams {
		df.health.HotStreams[exchange] = stream.Health()
	}
	
	return df.health
}

// Close shuts down the facade and all streams
func (df *DataFacadeImpl) Close() error {
	df.cancel()
	
	df.mu.Lock()
	defer df.mu.Unlock()
	
	var errs []error
	for exchange, stream := range df.hotStreams {
		if err := stream.Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", exchange, err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("errors closing streams: %v", errs)
	}
	
	return nil
}

// Helper methods (stubs for now - would be implemented with actual exchange APIs)

func (df *DataFacadeImpl) getBookFromStream(stream Stream, symbol string) (BookSnapshot, error) {
	// Implementation would read from stream's Books() channel
	return BookSnapshot{}, fmt.Errorf("not implemented")
}

func (df *DataFacadeImpl) fetchBookFromExchange(exchange, symbol string) (BookSnapshot, error) {
	// Implementation would make REST API call to exchange
	return BookSnapshot{}, fmt.Errorf("not implemented")
}

func (df *DataFacadeImpl) fetchFromSource(source string, req KlineReq) ([]Bar, error) {
	// Implementation would fetch from CoinGecko, CoinPaprika etc.
	return nil, fmt.Errorf("not implemented")
}

func (df *DataFacadeImpl) updateCacheHitRate(hit bool) {
	// Simple moving average update for cache hit rate
	if hit {
		df.health.CacheHitRate = (df.health.CacheHitRate*0.9) + (1.0*0.1)
	} else {
		df.health.CacheHitRate = (df.health.CacheHitRate * 0.9)
	}
}