package datafacade

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/microstructure"
	regimeconfig "github.com/sawpanic/cryptorun/internal/config/regime"
	"github.com/sawpanic/cryptorun/internal/infrastructure/datafacade/cache"
	"github.com/sawpanic/cryptorun/internal/infrastructure/datafacade/fakes"
)

// DataFacade provides unified access to market data with caching and fallbacks
type DataFacade struct {
	// Cache layers
	cache *cache.LayeredCache
	
	// Data providers (ordered by preference)
	providers []DataProvider
	
	// Fake data for testing
	fakeProvider *fakes.DeterministicFakeProvider
	
	// Circuit breaker state
	mu             sync.RWMutex
	circuitState   map[string]*CircuitState
	
	// Configuration
	config FacadeConfig
}

// DataProvider defines the interface for data sources
type DataProvider interface {
	GetName() string
	IsHealthy() bool
	GetMicrostructureData(ctx context.Context, symbol string) (*microstructure.MicrostructureData, error)
	GetRegimeData(ctx context.Context) (*regimeconfig.MarketData, error)
	GetSupportedSymbols() []string
}

// CircuitState tracks circuit breaker status for each provider
type CircuitState struct {
	IsOpen        bool
	FailureCount  int
	LastFailure   time.Time
	NextRetry     time.Time
}

// FacadeConfig configures the data facade behavior
type FacadeConfig struct {
	// Cache TTLs
	HotTTL   time.Duration
	WarmTTL  time.Duration
	ColdTTL  time.Duration
	
	// Circuit breaker settings
	FailureThreshold    int
	RecoveryTimeout     time.Duration
	CircuitOpenTime     time.Duration
	
	// Behavior flags
	UseFakesForTesting  bool
	AllowStaleData      bool
	MaxStaleness        time.Duration
	
	// Supported symbols
	SupportedSymbols    []string
}

// NewDataFacade creates a new data facade with caching and circuit breakers
func NewDataFacade(config FacadeConfig) *DataFacade {
	layeredCache := cache.NewLayeredCache(config.HotTTL, config.WarmTTL, config.ColdTTL)
	
	// Initialize fake provider for testing
	fakeProvider := fakes.NewDeterministicFakeProvider(
		time.Now(),
		config.SupportedSymbols,
	)
	
	return &DataFacade{
		cache:        layeredCache,
		providers:    []DataProvider{}, // Will be populated via RegisterProvider
		fakeProvider: fakeProvider,
		circuitState: make(map[string]*CircuitState),
		config:       config,
	}
}

// RegisterProvider adds a data provider to the facade
func (df *DataFacade) RegisterProvider(provider DataProvider) {
	df.mu.Lock()
	defer df.mu.Unlock()
	
	df.providers = append(df.providers, provider)
	df.circuitState[provider.GetName()] = &CircuitState{
		IsOpen:       false,
		FailureCount: 0,
	}
}

// GetMicrostructureData retrieves microstructure data with caching and fallbacks
func (df *DataFacade) GetMicrostructureData(ctx context.Context, symbol string) (*microstructure.MicrostructureData, error) {
	_ = time.Now() // startTime for potential metrics
	
	// Check cache first
	cacheKey := cache.CacheKey("microstructure", symbol)
	if cachedData, _, found := df.cache.Get(cacheKey); found {
		if data, ok := cachedData.(*microstructure.MicrostructureData); ok {
			// Verify data is not too stale
			staleness := time.Since(data.Timestamp)
			if !df.config.AllowStaleData || staleness <= df.config.MaxStaleness {
				return data, nil
			}
		}
	}
	
	// Try live providers if cache miss or stale data
	if !df.config.UseFakesForTesting {
		data, err := df.tryLiveProviders(ctx, symbol)
		if err == nil {
			// Cache successful result
			df.cache.Set(cacheKey, data, cache.TierHot)
			return data, nil
		}
	}
	
	// Fall back to fake data
	fakeData := df.fakeProvider.GetMicrostructureData(symbol, time.Now())
	
	// Cache fake data with shorter TTL
	df.cache.Set(cacheKey, &fakeData, cache.TierWarm)
	
	return &fakeData, nil
}

// GetRegimeData retrieves market regime data with caching
func (df *DataFacade) GetRegimeData(ctx context.Context) (*regimeconfig.MarketData, error) {
	// Use time-bucketed cache key (4-hour buckets for regime data)
	cacheKey := cache.CacheKeyWithTimestamp(4*time.Hour, "regime", "market")
	
	if cachedData, _, found := df.cache.Get(cacheKey); found {
		if data, ok := cachedData.(*regimeconfig.MarketData); ok {
			return data, nil
		}
	}
	
	// Try live providers
	if !df.config.UseFakesForTesting {
		data, err := df.tryLiveProvidersForRegime(ctx)
		if err == nil {
			// Cache for longer since regime changes slowly
			df.cache.Set(cacheKey, data, cache.TierCold)
			return data, nil
		}
	}
	
	// Fall back to fake data
	fakeData := df.fakeProvider.GetRegimeData(time.Now())
	df.cache.Set(cacheKey, &fakeData, cache.TierCold)
	
	return &fakeData, nil
}

// GetBatchMicrostructureData retrieves data for multiple symbols efficiently
func (df *DataFacade) GetBatchMicrostructureData(ctx context.Context, symbols []string) (map[string]*microstructure.MicrostructureData, []error) {
	results := make(map[string]*microstructure.MicrostructureData)
	errors := []error{}
	
	// Check cache for all symbols first
	cacheMisses := []string{}
	
	for _, symbol := range symbols {
		cacheKey := cache.CacheKey("microstructure", symbol)
		if cachedData, _, found := df.cache.Get(cacheKey); found {
			if data, ok := cachedData.(*microstructure.MicrostructureData); ok {
				staleness := time.Since(data.Timestamp)
				if !df.config.AllowStaleData || staleness <= df.config.MaxStaleness {
					results[symbol] = data
					continue
				}
			}
		}
		cacheMisses = append(cacheMisses, symbol)
	}
	
	// Fetch missing symbols
	for _, symbol := range cacheMisses {
		data, err := df.GetMicrostructureData(ctx, symbol)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to get data for %s: %w", symbol, err))
		} else {
			results[symbol] = data
		}
	}
	
	return results, errors
}

// tryLiveProviders attempts to get data from live providers with circuit breaker logic
func (df *DataFacade) tryLiveProviders(ctx context.Context, symbol string) (*microstructure.MicrostructureData, error) {
	df.mu.RLock()
	providers := make([]DataProvider, len(df.providers))
	copy(providers, df.providers)
	df.mu.RUnlock()
	
	for _, provider := range providers {
		// Check circuit breaker
		if df.isCircuitOpen(provider.GetName()) {
			continue
		}
		
		// Check provider health
		if !provider.IsHealthy() {
			df.recordFailure(provider.GetName(), fmt.Errorf("provider unhealthy"))
			continue
		}
		
		// Try to get data
		data, err := provider.GetMicrostructureData(ctx, symbol)
		if err != nil {
			df.recordFailure(provider.GetName(), err)
			continue
		}
		
		// Success - reset circuit breaker
		df.recordSuccess(provider.GetName())
		return data, nil
	}
	
	return nil, fmt.Errorf("all providers failed for symbol %s", symbol)
}

// tryLiveProvidersForRegime attempts to get regime data from live providers
func (df *DataFacade) tryLiveProvidersForRegime(ctx context.Context) (*regimeconfig.MarketData, error) {
	df.mu.RLock()
	providers := make([]DataProvider, len(df.providers))
	copy(providers, df.providers)
	df.mu.RUnlock()
	
	for _, provider := range providers {
		if df.isCircuitOpen(provider.GetName()) {
			continue
		}
		
		if !provider.IsHealthy() {
			df.recordFailure(provider.GetName(), fmt.Errorf("provider unhealthy"))
			continue
		}
		
		data, err := provider.GetRegimeData(ctx)
		if err != nil {
			df.recordFailure(provider.GetName(), err)
			continue
		}
		
		df.recordSuccess(provider.GetName())
		return data, nil
	}
	
	return nil, fmt.Errorf("all providers failed for regime data")
}

// Circuit breaker methods

func (df *DataFacade) isCircuitOpen(providerName string) bool {
	df.mu.RLock()
	defer df.mu.RUnlock()
	
	state, exists := df.circuitState[providerName]
	if !exists {
		return false
	}
	
	if state.IsOpen {
		// Check if circuit should be closed (retry time reached)
		if time.Now().After(state.NextRetry) {
			return false
		}
		return true
	}
	
	return false
}

func (df *DataFacade) recordFailure(providerName string, err error) {
	df.mu.Lock()
	defer df.mu.Unlock()
	
	state, exists := df.circuitState[providerName]
	if !exists {
		state = &CircuitState{}
		df.circuitState[providerName] = state
	}
	
	state.FailureCount++
	state.LastFailure = time.Now()
	
	// Open circuit if failure threshold exceeded
	if state.FailureCount >= df.config.FailureThreshold {
		state.IsOpen = true
		state.NextRetry = time.Now().Add(df.config.CircuitOpenTime)
	}
}

func (df *DataFacade) recordSuccess(providerName string) {
	df.mu.Lock()
	defer df.mu.Unlock()
	
	state, exists := df.circuitState[providerName]
	if !exists {
		return
	}
	
	// Reset circuit breaker on success
	state.FailureCount = 0
	state.IsOpen = false
}

// GetCacheStats returns cache performance statistics
func (df *DataFacade) GetCacheStats() cache.LayeredStats {
	return df.cache.GetStats()
}

// GetCircuitStates returns the current state of all circuit breakers
func (df *DataFacade) GetCircuitStates() map[string]CircuitState {
	df.mu.RLock()
	defer df.mu.RUnlock()
	
	states := make(map[string]CircuitState)
	for name, state := range df.circuitState {
		states[name] = *state
	}
	
	return states
}

// GetSupportedSymbols returns the list of supported symbols
func (df *DataFacade) GetSupportedSymbols() []string {
	return df.config.SupportedSymbols
}

// ClearCache removes all cached data
func (df *DataFacade) ClearCache() {
	df.cache.Clear()
}

// Close shuts down the data facade and cleans up resources
func (df *DataFacade) Close() {
	df.cache.Close()
}

// DefaultFacadeConfig returns a reasonable default configuration
func DefaultFacadeConfig() FacadeConfig {
	return FacadeConfig{
		// Cache TTLs following CryptoRun hot/warm/cold pattern
		HotTTL:  30 * time.Second,  // Hot: Real-time data
		WarmTTL: 5 * time.Minute,   // Warm: Recent data
		ColdTTL: 1 * time.Hour,     // Cold: Historical data
		
		// Circuit breaker settings
		FailureThreshold: 3,               // 3 failures to open circuit
		RecoveryTimeout:  30 * time.Second, // 30s recovery window
		CircuitOpenTime:  5 * time.Minute, // 5 minutes open time
		
		// Behavior flags
		UseFakesForTesting: true,          // Use fakes by default for testing
		AllowStaleData:     true,          // Allow slightly stale data
		MaxStaleness:       60 * time.Second, // Max 1 minute staleness
		
		// Kraken USD pairs (CryptoRun standard)
		SupportedSymbols: []string{
			"BTC-USD", "ETH-USD", "ADA-USD", "DOT-USD", "LINK-USD",
			"LTC-USD", "XLM-USD", "XRP-USD", "SOL-USD", "MATIC-USD",
		},
	}
}

// FacadeMetrics provides operational metrics for the data facade
type FacadeMetrics struct {
	CacheStats      cache.LayeredStats
	CircuitStates   map[string]CircuitState
	RequestCounts   map[string]int64
	ErrorCounts     map[string]int64
	ResponseTimes   map[string]time.Duration
	LastUpdated     time.Time
}

// GetMetrics returns comprehensive facade metrics
func (df *DataFacade) GetMetrics() FacadeMetrics {
	return FacadeMetrics{
		CacheStats:    df.GetCacheStats(),
		CircuitStates: df.GetCircuitStates(),
		LastUpdated:   time.Now(),
		// TODO: Add request/error counting and response time tracking
	}
}