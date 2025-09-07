package datafacade

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/adapters"
	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/cache"
	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"
	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/middleware"
	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/pit"
)

// DataFacadeImpl implements the DataFacade interface
type DataFacadeImpl struct {
	// Venue adapters
	venues map[string]interfaces.VenueAdapter
	
	// Middleware components
	cache       interfaces.CacheLayer
	rateLimiter interfaces.RateLimiter
	circuitBreaker interfaces.CircuitBreaker
	pitStore    interfaces.PITStore
	
	// Active subscriptions
	subscriptions map[string]subscription
	subMutex      sync.RWMutex
	
	// Configuration
	config *Config
}

type subscription struct {
	cancel  context.CancelFunc
	channel interface{}
}

// Config holds data facade configuration
type Config struct {
	// Cache configuration
	CacheConfig CacheConfig `yaml:"cache"`
	
	// Rate limiting configuration  
	RateLimitConfig RateLimitConfig `yaml:"rate_limits"`
	
	// Circuit breaker configuration
	CircuitConfig CircuitConfig `yaml:"circuits"`
	
	// PIT store configuration
	PITConfig PITConfig `yaml:"pit"`
	
	// Venue configurations
	Venues map[string]VenueConfig `yaml:"venues"`
}

type CacheConfig struct {
	Redis RedisConfig `yaml:"redis"`
	TTLs  map[string]map[string]time.Duration `yaml:"ttls"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type RateLimitConfig struct {
	Venues map[string]VenueRateLimits `yaml:"venues"`
}

type VenueRateLimits struct {
	RequestsPerSecond int                `yaml:"requests_per_second"`
	BurstAllowance    int                `yaml:"burst_allowance"`
	WeightLimits      map[string]int     `yaml:"weight_limits"`
	DailyLimit        *int               `yaml:"daily_limit"`
	MonthlyLimit      *int               `yaml:"monthly_limit"`
}

type CircuitConfig struct {
	Venues map[string]middleware.VenueConfig `yaml:"venues"`
}

type PITConfig struct {
	BasePath    string `yaml:"base_path"`
	Compression bool   `yaml:"compression"`
	RetentionDays int  `yaml:"retention_days"`
}

type VenueConfig struct {
	BaseURL string `yaml:"base_url"`
	WSURL   string `yaml:"ws_url"`
	Enabled bool   `yaml:"enabled"`
}

// NewDataFacade creates a new data facade implementation
func NewDataFacade(config *Config) (*DataFacadeImpl, error) {
	facade := &DataFacadeImpl{
		venues:        make(map[string]interfaces.VenueAdapter),
		subscriptions: make(map[string]subscription),
		config:        config,
	}
	
	// Initialize cache layer
	redisConfig := config.CacheConfig.Redis
	prefixes := map[string]string{
		"trades":      "trades:",
		"klines":      "klines:",
		"orderbook":   "ob:",
		"funding":     "funding:",
		"openinterest": "oi:",
	}
	
	redisCache, err := cache.NewRedisCache(
		redisConfig.Addr,
		redisConfig.Password,
		redisConfig.DB,
		prefixes,
	)
	if err != nil {
		return nil, fmt.Errorf("create redis cache: %w", err)
	}
	facade.cache = redisCache
	
	// Initialize rate limiter
	facade.rateLimiter = middleware.NewTokenBucketRateLimiter()
	
	// Configure rate limits for each venue
	for venueName, limits := range config.RateLimitConfig.Venues {
		rateLimits := &interfaces.RateLimits{
			RequestsPerSecond: limits.RequestsPerSecond,
			BurstAllowance:    limits.BurstAllowance,
			WeightLimits:      limits.WeightLimits,
			DailyLimit:        limits.DailyLimit,
			MonthlyLimit:      limits.MonthlyLimit,
		}
		facade.rateLimiter.UpdateLimits(context.Background(), venueName, rateLimits)
	}
	
	// Initialize circuit breaker
	facade.circuitBreaker = middleware.NewCircuitBreakerImpl()
	
	// Initialize PIT store
	facade.pitStore = pit.NewFileBasedPITStore(
		config.PITConfig.BasePath,
		config.PITConfig.Compression,
	)
	
	// Load existing snapshots
	if err := facade.pitStore.LoadExistingSnapshots(); err != nil {
		// Log error but don't fail initialization
		fmt.Printf("Warning: failed to load existing snapshots: %v\n", err)
	}
	
	// Initialize venue adapters
	if err := facade.initializeVenues(); err != nil {
		return nil, fmt.Errorf("initialize venues: %w", err)
	}
	
	return facade, nil
}

func (f *DataFacadeImpl) initializeVenues() error {
	for venueName, venueConfig := range f.config.Venues {
		if !venueConfig.Enabled {
			continue
		}
		
		var adapter interfaces.VenueAdapter
		var err error
		
		switch venueName {
		case "binance":
			adapter = adapters.NewBinanceAdapter(
				venueConfig.BaseURL,
				venueConfig.WSURL,
				f.rateLimiter,
				f.circuitBreaker,
				f.cache,
			)
		case "okx":
			adapter = adapters.NewOKXAdapter(
				venueConfig.BaseURL,
				venueConfig.WSURL,
				f.rateLimiter,
				f.circuitBreaker,
				f.cache,
			)
		case "coinbase":
			adapter = adapters.NewCoinbaseAdapter(
				venueConfig.BaseURL,
				venueConfig.WSURL,
				f.rateLimiter,
				f.circuitBreaker,
				f.cache,
			)
		case "kraken":
			adapter = adapters.NewKrakenAdapter(
				venueConfig.BaseURL,
				venueConfig.WSURL,
				f.rateLimiter,
				f.circuitBreaker,
				f.cache,
			)
		default:
			return fmt.Errorf("unsupported venue: %s", venueName)
		}
		
		f.venues[venueName] = adapter
	}
	
	return nil
}

// HOT streaming methods (WebSocket data)

func (f *DataFacadeImpl) SubscribeToTrades(ctx context.Context, venue, symbol string) (<-chan interfaces.TradeEvent, error) {
	adapter, err := f.getAdapter(venue)
	if err != nil {
		return nil, err
	}
	
	key := fmt.Sprintf("trades:%s:%s", venue, symbol)
	
	// Check if already subscribed
	f.subMutex.RLock()
	if sub, exists := f.subscriptions[key]; exists {
		f.subMutex.RUnlock()
		return sub.channel.(<-chan interfaces.TradeEvent), nil
	}
	f.subMutex.RUnlock()
	
	// Create new subscription
	subCtx, cancel := context.WithCancel(ctx)
	ch, err := adapter.StreamTrades(subCtx, symbol)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stream trades: %w", err)
	}
	
	// Store subscription
	f.subMutex.Lock()
	f.subscriptions[key] = subscription{
		cancel:  cancel,
		channel: ch,
	}
	f.subMutex.Unlock()
	
	return ch, nil
}

func (f *DataFacadeImpl) SubscribeToKlines(ctx context.Context, venue, symbol, interval string) (<-chan interfaces.KlineEvent, error) {
	adapter, err := f.getAdapter(venue)
	if err != nil {
		return nil, err
	}
	
	key := fmt.Sprintf("klines:%s:%s:%s", venue, symbol, interval)
	
	// Check if already subscribed
	f.subMutex.RLock()
	if sub, exists := f.subscriptions[key]; exists {
		f.subMutex.RUnlock()
		return sub.channel.(<-chan interfaces.KlineEvent), nil
	}
	f.subMutex.RUnlock()
	
	// Create new subscription
	subCtx, cancel := context.WithCancel(ctx)
	ch, err := adapter.StreamKlines(subCtx, symbol, interval)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stream klines: %w", err)
	}
	
	// Store subscription
	f.subMutex.Lock()
	f.subscriptions[key] = subscription{
		cancel:  cancel,
		channel: ch,
	}
	f.subMutex.Unlock()
	
	return ch, nil
}

func (f *DataFacadeImpl) SubscribeToOrderBook(ctx context.Context, venue, symbol string, depth int) (<-chan interfaces.OrderBookEvent, error) {
	adapter, err := f.getAdapter(venue)
	if err != nil {
		return nil, err
	}
	
	key := fmt.Sprintf("orderbook:%s:%s", venue, symbol)
	
	// Check if already subscribed
	f.subMutex.RLock()
	if sub, exists := f.subscriptions[key]; exists {
		f.subMutex.RUnlock()
		return sub.channel.(<-chan interfaces.OrderBookEvent), nil
	}
	f.subMutex.RUnlock()
	
	// Create new subscription
	subCtx, cancel := context.WithCancel(ctx)
	ch, err := adapter.StreamOrderBook(subCtx, symbol, depth)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stream orderbook: %w", err)
	}
	
	// Store subscription
	f.subMutex.Lock()
	f.subscriptions[key] = subscription{
		cancel:  cancel,
		channel: ch,
	}
	f.subMutex.Unlock()
	
	return ch, nil
}

func (f *DataFacadeImpl) SubscribeToFunding(ctx context.Context, venue, symbol string) (<-chan interfaces.FundingEvent, error) {
	adapter, err := f.getAdapter(venue)
	if err != nil {
		return nil, err
	}
	
	key := fmt.Sprintf("funding:%s:%s", venue, symbol)
	
	// Check if already subscribed
	f.subMutex.RLock()
	if sub, exists := f.subscriptions[key]; exists {
		f.subMutex.RUnlock()
		return sub.channel.(<-chan interfaces.FundingEvent), nil
	}
	f.subMutex.RUnlock()
	
	// Create new subscription
	subCtx, cancel := context.WithCancel(ctx)
	ch, err := adapter.StreamFunding(subCtx, symbol)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stream funding: %w", err)
	}
	
	// Store subscription
	f.subMutex.Lock()
	f.subscriptions[key] = subscription{
		cancel:  cancel,
		channel: ch,
	}
	f.subMutex.Unlock()
	
	return ch, nil
}

func (f *DataFacadeImpl) SubscribeToOpenInterest(ctx context.Context, venue, symbol string) (<-chan interfaces.OpenInterestEvent, error) {
	adapter, err := f.getAdapter(venue)
	if err != nil {
		return nil, err
	}
	
	key := fmt.Sprintf("openinterest:%s:%s", venue, symbol)
	
	// Check if already subscribed
	f.subMutex.RLock()
	if sub, exists := f.subscriptions[key]; exists {
		f.subMutex.RUnlock()
		return sub.channel.(<-chan interfaces.OpenInterestEvent), nil
	}
	f.subMutex.RUnlock()
	
	// Create new subscription
	subCtx, cancel := context.WithCancel(ctx)
	ch, err := adapter.StreamOpenInterest(subCtx, symbol)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stream open interest: %w", err)
	}
	
	// Store subscription
	f.subMutex.Lock()
	f.subscriptions[key] = subscription{
		cancel:  cancel,
		channel: ch,
	}
	f.subMutex.Unlock()
	
	return ch, nil
}

// WARM pull methods (REST API with caching)

func (f *DataFacadeImpl) GetTrades(ctx context.Context, venue, symbol string, limit int) ([]interfaces.Trade, error) {
	adapter, err := f.getAdapter(venue)
	if err != nil {
		return nil, err
	}
	
	return adapter.GetTrades(ctx, symbol, limit)
}

func (f *DataFacadeImpl) GetKlines(ctx context.Context, venue, symbol, interval string, limit int) ([]interfaces.Kline, error) {
	adapter, err := f.getAdapter(venue)
	if err != nil {
		return nil, err
	}
	
	return adapter.GetKlines(ctx, symbol, interval, limit)
}

func (f *DataFacadeImpl) GetOrderBook(ctx context.Context, venue, symbol string, depth int) (*interfaces.OrderBookSnapshot, error) {
	adapter, err := f.getAdapter(venue)
	if err != nil {
		return nil, err
	}
	
	return adapter.GetOrderBook(ctx, symbol, depth)
}

func (f *DataFacadeImpl) GetFunding(ctx context.Context, venue, symbol string) (*interfaces.FundingRate, error) {
	adapter, err := f.getAdapter(venue)
	if err != nil {
		return nil, err
	}
	
	return adapter.GetFunding(ctx, symbol)
}

func (f *DataFacadeImpl) GetOpenInterest(ctx context.Context, venue, symbol string) (*interfaces.OpenInterest, error) {
	adapter, err := f.getAdapter(venue)
	if err != nil {
		return nil, err
	}
	
	return adapter.GetOpenInterest(ctx, symbol)
}

// Multi-venue aggregation methods

func (f *DataFacadeImpl) GetTradesMultiVenue(ctx context.Context, venues []string, symbol string, limit int) (map[string][]interfaces.Trade, error) {
	result := make(map[string][]interfaces.Trade)
	
	for _, venue := range venues {
		trades, err := f.GetTrades(ctx, venue, symbol, limit)
		if err != nil {
			// Log error but continue with other venues
			fmt.Printf("Warning: failed to get trades from %s: %v\n", venue, err)
			continue
		}
		result[venue] = trades
	}
	
	return result, nil
}

func (f *DataFacadeImpl) GetOrderBookMultiVenue(ctx context.Context, venues []string, symbol string, depth int) (map[string]*interfaces.OrderBookSnapshot, error) {
	result := make(map[string]*interfaces.OrderBookSnapshot)
	
	for _, venue := range venues {
		orderBook, err := f.GetOrderBook(ctx, venue, symbol, depth)
		if err != nil {
			// Log error but continue with other venues
			fmt.Printf("Warning: failed to get order book from %s: %v\n", venue, err)
			continue
		}
		result[venue] = orderBook
	}
	
	return result, nil
}

// PIT snapshot methods

func (f *DataFacadeImpl) CreateSnapshot(ctx context.Context, snapshotID string) error {
	// Collect data from all venues
	data := make(map[string]interface{})
	
	for venueName := range f.venues {
		venueData := make(map[string]interface{})
		
		// For now, create a simple snapshot with venue health
		health := map[string]interface{}{
			"timestamp": time.Now(),
			"venue":     venueName,
			"healthy":   true, // This would be determined by actual health checks
		}
		
		venueData["health"] = health
		data[venueName] = venueData
	}
	
	return f.pitStore.CreateSnapshot(ctx, snapshotID, data)
}

func (f *DataFacadeImpl) GetSnapshot(ctx context.Context, snapshotID string) (map[string]interface{}, error) {
	return f.pitStore.GetSnapshot(ctx, snapshotID)
}

func (f *DataFacadeImpl) ListSnapshots(ctx context.Context, filter interfaces.SnapshotFilter) ([]interfaces.SnapshotInfo, error) {
	return f.pitStore.ListSnapshots(ctx, filter)
}

func (f *DataFacadeImpl) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	return f.pitStore.DeleteSnapshot(ctx, snapshotID)
}

// Subscription management

func (f *DataFacadeImpl) Unsubscribe(ctx context.Context, venue, dataType, symbol string) error {
	key := fmt.Sprintf("%s:%s:%s", dataType, venue, symbol)
	
	f.subMutex.Lock()
	defer f.subMutex.Unlock()
	
	if sub, exists := f.subscriptions[key]; exists {
		sub.cancel()
		delete(f.subscriptions, key)
	}
	
	return nil
}

func (f *DataFacadeImpl) UnsubscribeAll(ctx context.Context) error {
	f.subMutex.Lock()
	defer f.subMutex.Unlock()
	
	for key, sub := range f.subscriptions {
		sub.cancel()
		delete(f.subscriptions, key)
	}
	
	return nil
}

// Health and monitoring

func (f *DataFacadeImpl) GetHealth(ctx context.Context) (*interfaces.HealthStatus, error) {
	health := &interfaces.HealthStatus{
		Timestamp: time.Now(),
		Overall:   "healthy",
		Venues:    make(map[string]interfaces.VenueHealth),
	}
	
	// Check each venue health
	for venueName := range f.venues {
		venueHealth := interfaces.VenueHealth{
			Name:      venueName,
			Healthy:   true,
			LastCheck: time.Now(),
		}
		
		// Check circuit breaker status
		if state, err := f.circuitBreaker.GetState(ctx, venueName); err == nil {
			venueHealth.CircuitState = state.State
		}
		
		health.Venues[venueName] = venueHealth
	}
	
	return health, nil
}

func (f *DataFacadeImpl) GetMetrics(ctx context.Context) (*interfaces.FacadeMetrics, error) {
	metrics := &interfaces.FacadeMetrics{
		Timestamp:      time.Now(),
		ActiveStreams:  len(f.subscriptions),
		TotalVenues:    len(f.venues),
		EnabledVenues:  0,
	}
	
	// Count enabled venues
	for _, config := range f.config.Venues {
		if config.Enabled {
			metrics.EnabledVenues++
		}
	}
	
	// Get cache metrics
	if cacheStats, err := f.cache.GetStats(ctx); err == nil {
		metrics.CacheStats = *cacheStats
	}
	
	return metrics, nil
}

// Configuration updates

func (f *DataFacadeImpl) UpdateRateLimits(ctx context.Context, venue string, limits *interfaces.RateLimits) error {
	return f.rateLimiter.UpdateLimits(ctx, venue, limits)
}

func (f *DataFacadeImpl) ForceCircuitOpen(ctx context.Context, venue string) error {
	return f.circuitBreaker.ForceOpen(ctx, venue)
}

func (f *DataFacadeImpl) ForceCircuitClose(ctx context.Context, venue string) error {
	return f.circuitBreaker.ForceClose(ctx, venue)
}

// Cleanup and shutdown

func (f *DataFacadeImpl) Shutdown(ctx context.Context) error {
	// Cancel all subscriptions
	f.UnsubscribeAll(ctx)
	
	// Close cache connection
	if redisCache, ok := f.cache.(*cache.RedisCache); ok {
		redisCache.Close()
	}
	
	// Clean up old snapshots
	if f.config.PITConfig.RetentionDays > 0 {
		f.pitStore.Cleanup(ctx, f.config.PITConfig.RetentionDays)
	}
	
	return nil
}

// Helper methods

func (f *DataFacadeImpl) getAdapter(venue string) (interfaces.VenueAdapter, error) {
	adapter, exists := f.venues[venue]
	if !exists {
		return nil, fmt.Errorf("venue not configured: %s", venue)
	}
	return adapter, nil
}

func (f *DataFacadeImpl) GetSupportedVenues() []string {
	venues := make([]string, 0, len(f.venues))
	for venue := range f.venues {
		venues = append(venues, venue)
	}
	return venues
}