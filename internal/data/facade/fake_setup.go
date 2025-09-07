package facade

import (
	"context"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/cache"
	"github.com/sawpanic/cryptorun/internal/data/exchanges/fake"
	"github.com/sawpanic/cryptorun/internal/data/interfaces"
	"github.com/sawpanic/cryptorun/internal/data/rl"
)

// Adapter types to bridge interface mismatches

type rateLimiterAdapter struct {
	impl *rl.RateLimiter
}

func (rla *rateLimiterAdapter) Allow(venue string) bool {
	return rla.impl.Allow(venue)
}

func (rla *rateLimiterAdapter) Wait(venue string) time.Duration {
	return rla.impl.Wait(venue)
}

func (rla *rateLimiterAdapter) UpdateBudget(venue string, remaining int64) {
	rla.impl.UpdateBudget(venue, remaining)
}

func (rla *rateLimiterAdapter) Status(venue string) RateLimitStatus {
	status := rla.impl.Status(venue)
	return RateLimitStatus{
		Venue:       status.Venue,
		Remaining:   status.Remaining,
		ResetTime:   status.ResetTime,
		Throttled:   status.Throttled,
		BackoffTime: status.BackoffTime,
	}
}

type cacheAdapter struct {
	impl *cache.TTLCache
}

func (ca *cacheAdapter) Get(key string) (interface{}, bool) {
	return ca.impl.Get(key)
}

func (ca *cacheAdapter) Set(key string, value interface{}, ttl time.Duration) {
	ca.impl.Set(key, value, ttl)
}

func (ca *cacheAdapter) Delete(key string) {
	// TTLCache handles expiration automatically, no explicit delete needed
}

func (ca *cacheAdapter) Stats() CacheStats {
	stats := ca.impl.Stats()
	return CacheStats{
		PricesHot: CacheTierStats{
			TTL:      stats.PricesHot.TTL,
			Hits:     stats.PricesHot.Hits,
			Misses:   stats.PricesHot.Misses,
			Entries:  stats.PricesHot.Entries,
			HitRatio: stats.PricesHot.HitRatio,
		},
		PricesWarm: CacheTierStats{
			TTL:      stats.PricesWarm.TTL,
			Hits:     stats.PricesWarm.Hits,
			Misses:   stats.PricesWarm.Misses,
			Entries:  stats.PricesWarm.Entries,
			HitRatio: stats.PricesWarm.HitRatio,
		},
		VolumesVADR: CacheTierStats{
			TTL:      stats.VolumesVADR.TTL,
			Hits:     stats.VolumesVADR.Hits,
			Misses:   stats.VolumesVADR.Misses,
			Entries:  stats.VolumesVADR.Entries,
			HitRatio: stats.VolumesVADR.HitRatio,
		},
		TokenMeta: CacheTierStats{
			TTL:      stats.TokenMeta.TTL,
			Hits:     stats.TokenMeta.Hits,
			Misses:   stats.TokenMeta.Misses,
			Entries:  stats.TokenMeta.Entries,
			HitRatio: stats.TokenMeta.HitRatio,
		},
		TotalEntries: stats.TotalEntries,
	}
}

func (ca *cacheAdapter) Clear() {
	ca.impl.Clear()
}

type exchangeAdapter struct {
	impl *fake.Adapter
}

func (ea *exchangeAdapter) Name() string {
	return ea.impl.Name()
}

func (ea *exchangeAdapter) ConnectWS(ctx context.Context) error {
	return ea.impl.ConnectWS(ctx)
}

func (ea *exchangeAdapter) SubscribeTrades(symbol string, callback TradesCallback) error {
	return ea.impl.SubscribeTrades(symbol, func(trades []interfaces.Trade) error {
		// Convert interfaces.Trade to local Trade
		localTrades := make([]Trade, len(trades))
		for i, trade := range trades {
			localTrades[i] = Trade{
				Symbol:    trade.Symbol,
				Venue:     trade.Venue,
				Timestamp: trade.Timestamp,
				Price:     trade.Price,
				Size:      trade.Size,
				Side:      trade.Side,
				TradeID:   trade.TradeID,
			}
		}
		return callback(localTrades)
	})
}

func (ea *exchangeAdapter) SubscribeBookL2(symbol string, callback BookL2Callback) error {
	return ea.impl.SubscribeBookL2(symbol, func(book *interfaces.BookL2) error {
		localBook := &BookL2{
			Symbol:    book.Symbol,
			Venue:     book.Venue,
			Timestamp: book.Timestamp,
			Sequence:  book.Sequence,
			Bids:      make([]BookLevel, len(book.Bids)),
			Asks:      make([]BookLevel, len(book.Asks)),
		}
		
		for i, bid := range book.Bids {
			localBook.Bids[i] = BookLevel{
				Price: bid.Price,
				Size:  bid.Size,
			}
		}
		
		for i, ask := range book.Asks {
			localBook.Asks[i] = BookLevel{
				Price: ask.Price,
				Size:  ask.Size,
			}
		}
		
		return callback(localBook)
	})
}

func (ea *exchangeAdapter) StreamKlines(symbol string, interval string, callback KlinesCallback) error {
	return ea.impl.StreamKlines(symbol, interval, func(klines []interfaces.Kline) error {
		// Convert interfaces.Kline to local Kline
		localKlines := make([]Kline, len(klines))
		for i, kline := range klines {
			localKlines[i] = Kline{
				Symbol:    kline.Symbol,
				Venue:     kline.Venue,
				Timestamp: kline.Timestamp,
				Interval:  kline.Interval,
				Open:      kline.Open,
				High:      kline.High,
				Low:       kline.Low,
				Close:     kline.Close,
				Volume:    kline.Volume,
				QuoteVol:  kline.QuoteVol,
			}
		}
		return callback(localKlines)
	})
}

func (ea *exchangeAdapter) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]Kline, error) {
	klines, err := ea.impl.GetKlines(ctx, symbol, interval, limit)
	if err != nil {
		return nil, err
	}
	
	// Convert interfaces.Kline to local Kline
	localKlines := make([]Kline, len(klines))
	for i, kline := range klines {
		localKlines[i] = Kline{
			Symbol:    kline.Symbol,
			Venue:     kline.Venue,
			Timestamp: kline.Timestamp,
			Interval:  kline.Interval,
			Open:      kline.Open,
			High:      kline.High,
			Low:       kline.Low,
			Close:     kline.Close,
			Volume:    kline.Volume,
			QuoteVol:  kline.QuoteVol,
		}
	}
	
	return localKlines, nil
}

func (ea *exchangeAdapter) GetTrades(ctx context.Context, symbol string, limit int) ([]Trade, error) {
	trades, err := ea.impl.GetTrades(ctx, symbol, limit)
	if err != nil {
		return nil, err
	}
	
	// Convert interfaces.Trade to local Trade
	localTrades := make([]Trade, len(trades))
	for i, trade := range trades {
		localTrades[i] = Trade{
			Symbol:    trade.Symbol,
			Venue:     trade.Venue,
			Timestamp: trade.Timestamp,
			Price:     trade.Price,
			Size:      trade.Size,
			Side:      trade.Side,
			TradeID:   trade.TradeID,
		}
	}
	
	return localTrades, nil
}

func (ea *exchangeAdapter) NormalizeSymbol(symbol string) string {
	return ea.impl.NormalizeSymbol(symbol)
}

func (ea *exchangeAdapter) NormalizeInterval(interval string) string {
	return ea.impl.NormalizeInterval(interval)
}

func (ea *exchangeAdapter) Health() HealthStatus {
	health := ea.impl.Health()
	return HealthStatus{
		Venue:          health.Venue,
		Status:         health.Status,
		LastSeen:       health.LastSeen,
		ErrorRate:      health.ErrorRate,
		P99Latency:     health.P99Latency,
		WSConnected:    health.WSConnected,
		RESTHealthy:    health.RESTHealthy,
		Recommendation: health.Recommendation,
	}
}

func (ea *exchangeAdapter) GetBookL2(ctx context.Context, symbol string) (*BookL2, error) {
	book, err := ea.impl.GetBookL2(ctx, symbol)
	if err != nil {
		return nil, err
	}
	
	// Convert interfaces.BookL2 to local BookL2
	localBook := &BookL2{
		Symbol:    book.Symbol,
		Venue:     book.Venue,
		Timestamp: book.Timestamp,
		Sequence:  book.Sequence,
		Bids:      make([]BookLevel, len(book.Bids)),
		Asks:      make([]BookLevel, len(book.Asks)),
	}
	
	for i, bid := range book.Bids {
		localBook.Bids[i] = BookLevel{
			Price: bid.Price,
			Size:  bid.Size,
		}
	}
	
	for i, ask := range book.Asks {
		localBook.Asks[i] = BookLevel{
			Price: ask.Price,
			Size:  ask.Size,
		}
	}
	
	return localBook, nil
}

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
	
	// Create facade with adapter
	facade := New(hotConfig, warmConfig, cacheConfig, &rateLimiterAdapter{impl: rateLimiter})
	
	// Set up TTL cache with adapter
	facade.cache = &cacheAdapter{impl: cache.NewTTLCache(cacheConfig.MaxEntries)}
	
	// PIT store disabled for offline testing to avoid import cycle
	// facade.pitStore = pit.NewStore("artifacts/pit")
	
	// Register fake exchanges with different characteristics using adapters
	krakenFake := fake.NewDeterministicAdapter("fake_kraken")
	krakenFake.SetVolatility(0.015) // Lower volatility for "Kraken"
	krakenFake.SetBasePrice("BTCUSD", 67800.0)
	krakenFake.SetBasePrice("ETHUSD", 3250.0)
	facade.exchanges["fake_kraken"] = &exchangeAdapter{impl: krakenFake}
	
	binanceFake := fake.NewDeterministicAdapter("fake_binance")
	binanceFake.SetVolatility(0.025) // Higher volatility for "Binance"
	binanceFake.SetTrendBias(0.1)    // Slight upward bias
	binanceFake.SetBasePrice("BTCUSD", 67600.0)
	binanceFake.SetBasePrice("ETHUSD", 3220.0)
	facade.exchanges["fake_binance"] = &exchangeAdapter{impl: binanceFake}
	
	coinbaseFake := fake.NewDeterministicAdapter("fake_coinbase")
	coinbaseFake.SetVolatility(0.020) // Moderate volatility
	coinbaseFake.SetTrendBias(-0.05)  // Slight downward bias
	coinbaseFake.SetBasePrice("BTCUSD", 67700.0)
	coinbaseFake.SetBasePrice("ETHUSD", 3240.0)
	facade.exchanges["fake_coinbase"] = &exchangeAdapter{impl: coinbaseFake}
	
	okxFake := fake.NewDeterministicAdapter("fake_okx")
	okxFake.SetVolatility(0.030) // Highest volatility
	okxFake.SetBasePrice("BTCUSD", 67550.0)
	okxFake.SetBasePrice("ETHUSD", 3210.0)
	facade.exchanges["fake_okx"] = &exchangeAdapter{impl: okxFake}
	
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
	facade := New(hotConfig, warmConfig, cacheConfig, &rateLimiterAdapter{impl: rateLimiter})
	
	// Set up minimal components with adapters
	facade.cache = &cacheAdapter{impl: cache.NewTTLCache(cacheConfig.MaxEntries)}
	// facade.pitStore = pit.NewStore("test/pit")  // Disabled for offline testing
	
	// Single test exchange with adapter
	testFake := fake.NewDeterministicAdapter("test")
	testFake.SetVolatility(0.01) // Low volatility for stable testing
	facade.exchanges["test"] = &exchangeAdapter{impl: testFake}
	
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
	facade := New(hotConfig, warmConfig, cacheConfig, &rateLimiterAdapter{impl: rateLimiter})
	
	// Set up high-performance components with adapters
	facade.cache = &cacheAdapter{impl: cache.NewTTLCache(cacheConfig.MaxEntries)}
	// No PIT store for benchmarking to reduce overhead
	
	// Multiple exchanges for load testing with adapters
	for _, venue := range []string{"bench_kraken", "bench_binance", "bench_coinbase"} {
		benchFake := fake.NewDeterministicAdapter(venue)
		benchFake.SetVolatility(0.05) // Higher volatility for more realistic load
		facade.exchanges[venue] = &exchangeAdapter{impl: benchFake}
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
			if adapter, ok := exchange.(*exchangeAdapter); ok {
				adapter.impl.SetTrendBias(0.2)
				adapter.impl.SetVolatility(0.035) // Higher vol in bull markets
			}
			_ = name
		}
	
	case "bear_market":
		// Downward bias with increased volatility
		for name, exchange := range facade.exchanges {
			if adapter, ok := exchange.(*exchangeAdapter); ok {
				adapter.impl.SetTrendBias(-0.3)
				adapter.impl.SetVolatility(0.045)
			}
			_ = name
		}
	
	case "sideways":
		// No trend bias, moderate volatility
		for name, exchange := range facade.exchanges {
			if adapter, ok := exchange.(*exchangeAdapter); ok {
				adapter.impl.SetTrendBias(0.0)
				adapter.impl.SetVolatility(0.020)
			}
			_ = name
		}
		
	case "high_vol":
		// High volatility, no trend bias
		for name, exchange := range facade.exchanges {
			if adapter, ok := exchange.(*exchangeAdapter); ok {
				adapter.impl.SetTrendBias(0.0)
				adapter.impl.SetVolatility(0.060) // Very high volatility
			}
			_ = name
		}
		
	case "stable":
		// Low volatility for testing
		for name, exchange := range facade.exchanges {
			if adapter, ok := exchange.(*exchangeAdapter); ok {
				adapter.impl.SetTrendBias(0.0)
				adapter.impl.SetVolatility(0.005) // Very low volatility
			}
			_ = name
		}
	}
}