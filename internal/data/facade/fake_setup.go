package facade

import (
	"time"

	"github.com/sawpanic/cryptorun/internal/data/cache"
	"github.com/sawpanic/cryptorun/internal/data/exchanges/fake"
	"github.com/sawpanic/cryptorun/internal/data/rl"
)

// NewOfflineFacade creates a data facade configured for offline development with deterministic fake data
func NewOfflineFacade() *Facade {
	// Default configurations for offline development
	hotConfig := HotConfig{
		Venues:       []string{"fake_kraken", "fake_binance", "fake_coinbase"},
		MaxPairs:     50,
		ReconnectSec: 30,
		BufferSize:   1000,
		Timeout:      10 * time.Second,
	}
	
	warmConfig := WarmConfig{
		Venues:       []string{"fake_kraken", "fake_binance", "fake_coinbase", "fake_okx"},
		DefaultTTL:   30 * time.Second,
		MaxRetries:   3,
		BackoffBase:  100 * time.Millisecond,
		RequestLimit: 100,
	}
	
	cacheConfig := CacheConfig{
		PricesHot:   5 * time.Second,    // Hot prices cache for 5s
		PricesWarm:  30 * time.Second,   // Warm prices cache for 30s
		VolumesVADR: 120 * time.Second,  // Volume/VADR cache for 2min
		TokenMeta:   24 * time.Hour,     // Token metadata cache for 24h
		MaxEntries:  10000,              // Maximum cache entries
	}
	
	// Create rate limiter
	rateLimiter := rl.NewRateLimiter()
	
	// Create facade
	facade := New(hotConfig, warmConfig, cacheConfig, rateLimiter)
	
	// Set up TTL cache
	facade.cache = cache.NewTTLCache(cacheConfig.MaxEntries)
	
	// PIT store disabled for offline testing to avoid import cycle
	// facade.pitStore = pit.NewStore("artifacts/pit")
	
	// Register fake exchanges with different characteristics
	krakenFake := fake.NewDeterministicAdapter("fake_kraken")
	krakenFake.SetVolatility(0.015) // Lower volatility for "Kraken"
	krakenFake.SetBasePrice("BTCUSD", 67800.0)
	krakenFake.SetBasePrice("ETHUSD", 3250.0)
	facade.exchanges["fake_kraken"] = krakenFake
	
	binanceFake := fake.NewDeterministicAdapter("fake_binance")
	binanceFake.SetVolatility(0.025) // Higher volatility for "Binance"
	binanceFake.SetTrendBias(0.1)    // Slight upward bias
	binanceFake.SetBasePrice("BTCUSD", 67600.0)
	binanceFake.SetBasePrice("ETHUSD", 3220.0)
	facade.exchanges["fake_binance"] = binanceFake
	
	coinbaseFake := fake.NewDeterministicAdapter("fake_coinbase")
	coinbaseFake.SetVolatility(0.020) // Moderate volatility
	coinbaseFake.SetTrendBias(-0.05)  // Slight downward bias
	coinbaseFake.SetBasePrice("BTCUSD", 67700.0)
	coinbaseFake.SetBasePrice("ETHUSD", 3240.0)
	facade.exchanges["fake_coinbase"] = coinbaseFake
	
	okxFake := fake.NewDeterministicAdapter("fake_okx")
	okxFake.SetVolatility(0.030) // Highest volatility
	okxFake.SetBasePrice("BTCUSD", 67550.0)
	okxFake.SetBasePrice("ETHUSD", 3210.0)
	facade.exchanges["fake_okx"] = okxFake
	
	return facade
}

// NewTestingFacade creates a simplified facade for unit testing with minimal setup
func NewTestingFacade() *Facade {
	hotConfig := HotConfig{
		Venues:       []string{"test"},
		MaxPairs:     10,
		ReconnectSec: 30,
		BufferSize:   100,
		Timeout:      5 * time.Second,
	}
	
	warmConfig := WarmConfig{
		Venues:       []string{"test"},
		DefaultTTL:   10 * time.Second,
		MaxRetries:   1,
		BackoffBase:  50 * time.Millisecond,
		RequestLimit: 10,
	}
	
	cacheConfig := CacheConfig{
		PricesHot:   2 * time.Second,
		PricesWarm:  10 * time.Second,
		VolumesVADR: 30 * time.Second,
		TokenMeta:   time.Hour,
		MaxEntries:  100,
	}
	
	rateLimiter := rl.NewRateLimiter()
	facade := New(hotConfig, warmConfig, cacheConfig, rateLimiter)
	
	// Set up minimal components
	facade.cache = cache.NewTTLCache(cacheConfig.MaxEntries)
	// facade.pitStore = pit.NewStore("test/pit")  // Disabled for offline testing
	
	// Single test exchange
	testFake := fake.NewDeterministicAdapter("test")
	testFake.SetVolatility(0.01) // Low volatility for stable testing
	facade.exchanges["test"] = testFake
	
	return facade
}

// NewBenchmarkFacade creates a facade configured for performance benchmarking
func NewBenchmarkFacade() *Facade {
	// High-performance configuration
	hotConfig := HotConfig{
		Venues:       []string{"bench_kraken", "bench_binance"},
		MaxPairs:     100,
		ReconnectSec: 10,
		BufferSize:   5000,
		Timeout:      2 * time.Second,
	}
	
	warmConfig := WarmConfig{
		Venues:       []string{"bench_kraken", "bench_binance", "bench_coinbase"},
		DefaultTTL:   1 * time.Second,  // Very short TTL for benchmarking
		MaxRetries:   1,                // Minimal retries
		BackoffBase:  10 * time.Millisecond,
		RequestLimit: 1000,
	}
	
	cacheConfig := CacheConfig{
		PricesHot:   500 * time.Millisecond,
		PricesWarm:  2 * time.Second,
		VolumesVADR: 5 * time.Second,
		TokenMeta:   time.Hour,
		MaxEntries:  50000, // Large cache for benchmarking
	}
	
	rateLimiter := rl.NewRateLimiter()
	facade := New(hotConfig, warmConfig, cacheConfig, rateLimiter)
	
	// Set up high-performance components
	facade.cache = cache.NewTTLCache(cacheConfig.MaxEntries)
	// No PIT store for benchmarking to reduce overhead
	
	// Multiple exchanges for load testing
	for _, venue := range []string{"bench_kraken", "bench_binance", "bench_coinbase"} {
		benchFake := fake.NewDeterministicAdapter(venue)
		benchFake.SetVolatility(0.05) // Higher volatility for more realistic load
		facade.exchanges[venue] = benchFake
	}
	
	return facade
}

// GetFakeUniverseSymbols returns a list of symbols supported by fake exchanges for testing
func GetFakeUniverseSymbols() []string {
	return []string{
		"BTCUSD", "ETHUSD", "SOLUSD", "ADAUSD", "LINKUSD",
		"DOTUSD", "MATICUSD", "AVAXUSD", "UNIUSD", "LTCUSD",
		"XRPUSD", "ALGOUSD", "ATOMUSD", "NEARUSD", "FTMUSD",
		"MANAUSD", "SANDUSD", "ENJUSD", "GALAUSD", "CHZUSD",
	}
}

// SetupRealisticScenario configures fake exchanges with realistic market conditions
func SetupRealisticScenario(facade *Facade, scenario string) {
	switch scenario {
	case "bull_market":
		// All exchanges have upward bias with varying volatility
		for name, exchange := range facade.exchanges {
			if fake, ok := exchange.(*fake.Adapter); ok {
				fake.SetTrendBias(0.2)
				fake.SetVolatility(0.035) // Higher vol in bull markets
			}
			_ = name
		}
	
	case "bear_market":
		// Downward bias with increased volatility
		for name, exchange := range facade.exchanges {
			if fake, ok := exchange.(*fake.Adapter); ok {
				fake.SetTrendBias(-0.3)
				fake.SetVolatility(0.045)
			}
			_ = name
		}
	
	case "sideways":
		// No trend bias, moderate volatility
		for name, exchange := range facade.exchanges {
			if fake, ok := exchange.(*fake.Adapter); ok {
				fake.SetTrendBias(0.0)
				fake.SetVolatility(0.020)
			}
			_ = name
		}
		
	case "high_vol":
		// High volatility, no trend bias
		for name, exchange := range facade.exchanges {
			if fake, ok := exchange.(*fake.Adapter); ok {
				fake.SetTrendBias(0.0)
				fake.SetVolatility(0.060) // Very high volatility
			}
			_ = name
		}
		
	case "stable":
		// Low volatility for testing
		for name, exchange := range facade.exchanges {
			if fake, ok := exchange.(*fake.Adapter); ok {
				fake.SetTrendBias(0.0)
				fake.SetVolatility(0.005) // Very low volatility
			}
			_ = name
		}
	}
}