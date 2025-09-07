package datafacade

import (
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/adapters"
	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/cache"
	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"
	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/middleware"
	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/pit"
)

// Factory creates and configures data facade components
type Factory struct {
	config *Config
}

// NewFactory creates a new data facade factory
func NewFactory(config *Config) *Factory {
	return &Factory{config: config}
}

// CreateDataFacade creates a fully configured data facade
func (f *Factory) CreateDataFacade() (*DataFacadeImpl, error) {
	// Create cache layer
	cache, err := f.createCache()
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}
	
	// Create rate limiter
	rateLimiter := f.createRateLimiter()
	
	// Create circuit breaker
	circuitBreaker := f.createCircuitBreaker()
	
	// Create PIT store
	pitStore := f.createPITStore()
	
	// Create venue adapters
	venues, err := f.createVenueAdapters(cache, rateLimiter, circuitBreaker)
	if err != nil {
		return nil, fmt.Errorf("create venue adapters: %w", err)
	}
	
	facade := &DataFacadeImpl{
		venues:         venues,
		cache:          cache,
		rateLimiter:    rateLimiter,
		circuitBreaker: circuitBreaker,
		pitStore:       pitStore,
		subscriptions:  make(map[string]subscription),
		config:         f.config,
	}
	
	// Load existing snapshots
	if err := pitStore.LoadExistingSnapshots(); err != nil {
		fmt.Printf("Warning: failed to load existing snapshots: %v\n", err)
	}
	
	return facade, nil
}

func (f *Factory) createCache() (interfaces.CacheLayer, error) {
	redisConfig := f.config.CacheConfig.Redis
	prefixes := map[string]string{
		"trades":       "trades:",
		"klines":       "klines:",
		"orderbook":    "ob:",
		"funding":      "funding:",
		"openinterest": "oi:",
	}
	
	return cache.NewRedisCache(
		redisConfig.Addr,
		redisConfig.Password,
		redisConfig.DB,
		prefixes,
	)
}

func (f *Factory) createRateLimiter() interfaces.RateLimiter {
	rateLimiter := middleware.NewTokenBucketRateLimiter()
	
	// Configure rate limits for each venue
	for venueName, limits := range f.config.RateLimitConfig.Venues {
		rateLimits := &interfaces.RateLimits{
			RequestsPerSecond: limits.RequestsPerSecond,
			BurstAllowance:    limits.BurstAllowance,
			WeightLimits:      limits.WeightLimits,
			DailyLimit:        limits.DailyLimit,
			MonthlyLimit:      limits.MonthlyLimit,
		}
		rateLimiter.UpdateLimits(nil, venueName, rateLimits)
	}
	
	return rateLimiter
}

func (f *Factory) createCircuitBreaker() interfaces.CircuitBreaker {
	circuitBreaker := middleware.NewCircuitBreakerImpl()
	
	// Configure circuit breakers for each venue
	for venueName, venueConfig := range f.config.CircuitConfig.Venues {
		// Configure HTTP circuit breaker
		httpConfig := middleware.CircuitBreakerConfig{
			FailureThreshold: venueConfig.HTTP.FailureThreshold,
			SuccessThreshold: venueConfig.HTTP.SuccessThreshold,
			Timeout:          venueConfig.HTTP.Timeout,
			MaxRequests:      venueConfig.HTTP.MaxRequests,
		}
		circuitBreaker.ConfigureBreaker(venueName+"_http", httpConfig)
		
		// Configure WebSocket circuit breaker
		wsConfig := middleware.CircuitBreakerConfig{
			FailureThreshold: venueConfig.WebSocket.FailureThreshold,
			SuccessThreshold: venueConfig.WebSocket.SuccessThreshold,
			Timeout:          venueConfig.WebSocket.Timeout,
			MaxRequests:      venueConfig.WebSocket.MaxRequests,
		}
		circuitBreaker.ConfigureBreaker(venueName+"_websocket", wsConfig)
	}
	
	return circuitBreaker
}

func (f *Factory) createPITStore() interfaces.PITStore {
	return pit.NewFileBasedPITStore(
		f.config.PITConfig.BasePath,
		f.config.PITConfig.Compression,
	)
}

func (f *Factory) createVenueAdapters(cache interfaces.CacheLayer, rateLimiter interfaces.RateLimiter, 
	circuitBreaker interfaces.CircuitBreaker) (map[string]interfaces.VenueAdapter, error) {
	
	venues := make(map[string]interfaces.VenueAdapter)
	
	for venueName, venueConfig := range f.config.Venues {
		if !venueConfig.Enabled {
			continue
		}
		
		var adapter interfaces.VenueAdapter
		
		switch venueName {
		case "binance":
			adapter = adapters.NewBinanceAdapter(
				rateLimiter,
				circuitBreaker,
			)
			
		case "okx":
			adapter = adapters.NewOKXAdapter(
				venueConfig.BaseURL,
				venueConfig.WSURL,
				rateLimiter,
				circuitBreaker,
				cache,
			)
			
		case "coinbase":
			adapter = adapters.NewCoinbaseAdapter(
				venueConfig.BaseURL,
				venueConfig.WSURL,
				rateLimiter,
				circuitBreaker,
				cache,
			)
			
		case "kraken":
			adapter = adapters.NewKrakenAdapter(
				venueConfig.BaseURL,
				venueConfig.WSURL,
				rateLimiter,
				circuitBreaker,
				cache,
			)
			
		default:
			return nil, fmt.Errorf("unsupported venue: %s", venueName)
		}
		
		venues[venueName] = adapter
	}
	
	return venues, nil
}

// DefaultConfig creates a default configuration for the data facade
func DefaultConfig() *Config {
	return &Config{
		CacheConfig: CacheConfig{
			Redis: RedisConfig{
				Addr:     "localhost:6379",
				Password: "",
				DB:       0,
			},
			TTLs: map[string]map[string]time.Duration{
				"default": {
					"trades":       30 * time.Second,
					"klines":       60 * time.Second,
					"orderbook_l1": 5 * time.Second,
					"orderbook_l2": 10 * time.Second,
					"funding":      300 * time.Second,
					"openinterest": 60 * time.Second,
				},
			},
		},
		RateLimitConfig: RateLimitConfig{
			Venues: map[string]VenueRateLimits{
				"binance": {
					RequestsPerSecond: 20,
					BurstAllowance:    10,
					WeightLimits: map[string]int{
						"trades":       1,
						"klines":       1,
						"orderbook":    1,
						"funding":      1,
						"openinterest": 1,
					},
					DailyLimit:   intPtr(160000),
					MonthlyLimit: intPtr(5000000),
				},
				"okx": {
					RequestsPerSecond: 10,
					BurstAllowance:    5,
					WeightLimits:      map[string]int{},
					DailyLimit:        intPtr(50000),
					MonthlyLimit:      intPtr(1500000),
				},
				"coinbase": {
					RequestsPerSecond: 5,
					BurstAllowance:    3,
					WeightLimits:      map[string]int{},
					DailyLimit:        intPtr(10000),
					MonthlyLimit:      intPtr(300000),
				},
				"kraken": {
					RequestsPerSecond: 1,
					BurstAllowance:    1,
					WeightLimits:      map[string]int{},
					DailyLimit:        intPtr(5000),
					MonthlyLimit:      intPtr(150000),
				},
			},
		},
		CircuitConfig: CircuitConfig{
			Venues: map[string]middleware.VenueConfig{
				"binance": {
					HTTP: struct {
						FailureThreshold int
						SuccessThreshold int
						Timeout          time.Duration
						MaxRequests      int
					}{
						FailureThreshold: 5,
						SuccessThreshold: 3,
						Timeout:          30 * time.Second,
						MaxRequests:      3,
					},
					WebSocket: struct {
						FailureThreshold int
						SuccessThreshold int
						Timeout          time.Duration
						MaxRequests      int
					}{
						FailureThreshold: 3,
						SuccessThreshold: 2,
						Timeout:          60 * time.Second,
						MaxRequests:      2,
					},
				},
				"okx": {
					HTTP: struct {
						FailureThreshold int
						SuccessThreshold int
						Timeout          time.Duration
						MaxRequests      int
					}{
						FailureThreshold: 5,
						SuccessThreshold: 3,
						Timeout:          30 * time.Second,
						MaxRequests:      3,
					},
					WebSocket: struct {
						FailureThreshold int
						SuccessThreshold int
						Timeout          time.Duration
						MaxRequests      int
					}{
						FailureThreshold: 3,
						SuccessThreshold: 2,
						Timeout:          60 * time.Second,
						MaxRequests:      2,
					},
				},
				"coinbase": {
					HTTP: struct {
						FailureThreshold int
						SuccessThreshold int
						Timeout          time.Duration
						MaxRequests      int
					}{
						FailureThreshold: 8,
						SuccessThreshold: 4,
						Timeout:          45 * time.Second,
						MaxRequests:      2,
					},
					WebSocket: struct {
						FailureThreshold int
						SuccessThreshold int
						Timeout          time.Duration
						MaxRequests      int
					}{
						FailureThreshold: 3,
						SuccessThreshold: 2,
						Timeout:          60 * time.Second,
						MaxRequests:      2,
					},
				},
				"kraken": {
					HTTP: struct {
						FailureThreshold int
						SuccessThreshold int
						Timeout          time.Duration
						MaxRequests      int
					}{
						FailureThreshold: 3,
						SuccessThreshold: 2,
						Timeout:          60 * time.Second,
						MaxRequests:      1,
					},
					WebSocket: struct {
						FailureThreshold int
						SuccessThreshold int
						Timeout          time.Duration
						MaxRequests      int
					}{
						FailureThreshold: 2,
						SuccessThreshold: 1,
						Timeout:          120 * time.Second,
						MaxRequests:      1,
					},
				},
			},
		},
		PITConfig: PITConfig{
			BasePath:      "./data/pit",
			Compression:   true,
			RetentionDays: 30,
		},
		Venues: map[string]VenueConfig{
			"binance": {
				BaseURL: "https://api.binance.com",
				WSURL:   "wss://stream.binance.com:9443/ws",
				Enabled: true,
			},
			"okx": {
				BaseURL: "https://www.okx.com",
				WSURL:   "wss://ws.okx.com:8443",
				Enabled: true,
			},
			"coinbase": {
				BaseURL: "https://api.exchange.coinbase.com",
				WSURL:   "wss://ws-feed.exchange.coinbase.com",
				Enabled: true,
			},
			"kraken": {
				BaseURL: "https://api.kraken.com",
				WSURL:   "wss://ws.kraken.com",
				Enabled: true,
			},
		},
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}