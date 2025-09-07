package facade

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Implementation of DataFacade interface

// Start initializes all components and begins hot tier streaming
func (f *Facade) Start(ctx context.Context) error {
	log.Info().Msg("Starting data facade")
	
	// Initialize exchanges
	for _, venue := range f.hotConfig.Venues {
		if exchange := f.exchanges[venue]; exchange != nil {
			if err := exchange.ConnectWS(ctx); err != nil {
				log.Warn().Str("venue", venue).Err(err).Msg("Failed to connect WebSocket")
				// Continue with other venues - degraded mode
			}
		}
	}
	
	// Initialize health monitoring
	go f.monitorHealth(ctx)
	
	log.Info().Int("hot_venues", len(f.hotConfig.Venues)).
		Int("warm_venues", len(f.warmConfig.Venues)).
		Msg("Data facade started")
	
	return nil
}

// Stop gracefully shuts down all connections
func (f *Facade) Stop() error {
	log.Info().Msg("Stopping data facade")
	
	// Stop all exchange connections
	var wg sync.WaitGroup
	for venue, exchange := range f.exchanges {
		wg.Add(1)
		go func(v string, ex Exchange) {
			defer wg.Done()
			log.Debug().Str("venue", v).Msg("Stopping exchange connection")
		}(venue, exchange)
	}
	
	wg.Wait()
	log.Info().Msg("Data facade stopped")
	return nil
}

// Hot tier implementations - WebSocket subscriptions

func (f *Facade) SubscribeTrades(ctx context.Context, venue string, symbol string, callback TradesCallback) error {
	exchange := f.exchanges[venue]
	if exchange == nil {
		return fmt.Errorf("unsupported venue: %s", venue)
	}
	
	log.Info().Str("venue", venue).Str("symbol", symbol).Msg("Subscribing to trades")
	
	// Rate limit check
	if !f.rateLimit.Allow(venue) {
		waitTime := f.rateLimit.Wait(venue)
		log.Warn().Str("venue", venue).Dur("wait", waitTime).Msg("Rate limited, waiting")
		time.Sleep(waitTime)
	}
	
	// Wrap callback to add PIT snapshots and attribution
	wrappedCallback := func(trades []Trade) error {
		// Update attribution
		f.updateAttribution(venue, "trades", len(trades))
		
		// Store PIT snapshot
		if f.pitStore != nil {
			for _, trade := range trades {
				f.pitStore.Snapshot("trades", trade.Timestamp, trade, venue)
			}
		}
		
		return callback(trades)
	}
	
	return exchange.SubscribeTrades(symbol, wrappedCallback)
}

func (f *Facade) SubscribeBookL2(ctx context.Context, venue string, symbol string, callback BookL2Callback) error {
	exchange := f.exchanges[venue]
	if exchange == nil {
		return fmt.Errorf("unsupported venue: %s", venue)
	}
	
	log.Info().Str("venue", venue).Str("symbol", symbol).Msg("Subscribing to L2 book")
	
	if !f.rateLimit.Allow(venue) {
		waitTime := f.rateLimit.Wait(venue)
		time.Sleep(waitTime)
	}
	
	wrappedCallback := func(book *BookL2) error {
		f.updateAttribution(venue, "book_l2", 1)
		
		if f.pitStore != nil {
			f.pitStore.Snapshot("book_l2", book.Timestamp, book, venue)
		}
		
		return callback(book)
	}
	
	return exchange.SubscribeBookL2(symbol, wrappedCallback)
}

func (f *Facade) StreamKlines(ctx context.Context, venue string, symbol string, interval string, callback KlinesCallback) error {
	exchange := f.exchanges[venue]
	if exchange == nil {
		return fmt.Errorf("unsupported venue: %s", venue)
	}
	
	log.Info().Str("venue", venue).Str("symbol", symbol).Str("interval", interval).Msg("Subscribing to klines")
	
	if !f.rateLimit.Allow(venue) {
		waitTime := f.rateLimit.Wait(venue)
		time.Sleep(waitTime)
	}
	
	wrappedCallback := func(klines []Kline) error {
		f.updateAttribution(venue, "klines", len(klines))
		
		if f.pitStore != nil {
			for _, kline := range klines {
				f.pitStore.Snapshot("klines", kline.Timestamp, kline, venue)
			}
		}
		
		return callback(klines)
	}
	
	return exchange.StreamKlines(symbol, interval, wrappedCallback)
}

// Warm tier implementations - REST API with caching

func (f *Facade) GetKlines(ctx context.Context, venue string, symbol string, interval string, limit int) ([]Kline, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("klines:%s:%s:%s:%d", venue, symbol, interval, limit)
	
	if cached, found := f.cache.Get(cacheKey); found {
		f.updateCacheHit(venue)
		if klines, ok := cached.([]Kline); ok {
			log.Debug().Str("venue", venue).Str("symbol", symbol).Msg("Cache hit for klines")
			return klines, nil
		}
	}
	
	f.updateCacheMiss(venue)
	
	// Rate limit check
	if !f.rateLimit.Allow(venue) {
		waitTime := f.rateLimit.Wait(venue)
		log.Warn().Str("venue", venue).Dur("wait", waitTime).Msg("Rate limited for REST call")
		time.Sleep(waitTime)
	}
	
	exchange := f.exchanges[venue]
	if exchange == nil {
		return nil, fmt.Errorf("unsupported venue: %s", venue)
	}
	
	start := time.Now()
	klines, err := exchange.GetKlines(ctx, symbol, interval, limit)
	if err != nil {
		f.updateHealthError(venue, err)
		return nil, err
	}
	
	// Update attribution and health
	latency := time.Since(start)
	f.updateAttribution(venue, "klines_rest", len(klines))
	f.updateHealthLatency(venue, latency)
	
	// Cache the result with appropriate TTL
	ttl := f.getTTLForDataType("klines")
	f.cache.Set(cacheKey, klines, ttl)
	
	// Store PIT snapshots
	if f.pitStore != nil {
		for _, kline := range klines {
			f.pitStore.Snapshot("klines", kline.Timestamp, kline, venue)
		}
	}
	
	log.Debug().Str("venue", venue).Str("symbol", symbol).
		Dur("latency", latency).Int("count", len(klines)).
		Msg("Fetched klines from REST API")
	
	return klines, nil
}

func (f *Facade) GetTrades(ctx context.Context, venue string, symbol string, limit int) ([]Trade, error) {
	cacheKey := fmt.Sprintf("trades:%s:%s:%d", venue, symbol, limit)
	
	if cached, found := f.cache.Get(cacheKey); found {
		f.updateCacheHit(venue)
		if trades, ok := cached.([]Trade); ok {
			return trades, nil
		}
	}
	
	f.updateCacheMiss(venue)
	
	if !f.rateLimit.Allow(venue) {
		waitTime := f.rateLimit.Wait(venue)
		time.Sleep(waitTime)
	}
	
	exchange := f.exchanges[venue]
	if exchange == nil {
		return nil, fmt.Errorf("unsupported venue: %s", venue)
	}
	
	start := time.Now()
	trades, err := exchange.GetTrades(ctx, symbol, limit)
	if err != nil {
		f.updateHealthError(venue, err)
		return nil, err
	}
	
	latency := time.Since(start)
	f.updateAttribution(venue, "trades_rest", len(trades))
	f.updateHealthLatency(venue, latency)
	
	ttl := f.getTTLForDataType("trades")
	f.cache.Set(cacheKey, trades, ttl)
	
	if f.pitStore != nil {
		for _, trade := range trades {
			f.pitStore.Snapshot("trades", trade.Timestamp, trade, venue)
		}
	}
	
	return trades, nil
}

func (f *Facade) GetBookL2(ctx context.Context, venue string, symbol string) (*BookL2, error) {
	cacheKey := fmt.Sprintf("book_l2:%s:%s", venue, symbol)
	
	if cached, found := f.cache.Get(cacheKey); found {
		f.updateCacheHit(venue)
		if book, ok := cached.(*BookL2); ok {
			return book, nil
		}
	}
	
	f.updateCacheMiss(venue)
	
	if !f.rateLimit.Allow(venue) {
		waitTime := f.rateLimit.Wait(venue)
		time.Sleep(waitTime)
	}
	
	exchange := f.exchanges[venue]
	if exchange == nil {
		return nil, fmt.Errorf("unsupported venue: %s", venue)
	}
	
	start := time.Now()
	book, err := exchange.GetBookL2(ctx, symbol)
	if err != nil {
		f.updateHealthError(venue, err)
		return nil, err
	}
	
	latency := time.Since(start)
	f.updateAttribution(venue, "book_l2_rest", 1)
	f.updateHealthLatency(venue, latency)
	
	ttl := f.getTTLForDataType("book_l2")
	f.cache.Set(cacheKey, book, ttl)
	
	if f.pitStore != nil {
		f.pitStore.Snapshot("book_l2", book.Timestamp, book, venue)
	}
	
	return book, nil
}

// Attribution and health methods

func (f *Facade) SourceAttribution(venue string) Attribution {
	if attr, exists := f.attribution[venue]; exists {
		return *attr
	}
	
	return Attribution{
		Venue:      venue,
		LastUpdate: time.Time{},
		Sources:    []string{},
	}
}

func (f *Facade) VenueHealth(venue string) HealthStatus {
	if health, exists := f.healthStats[venue]; exists {
		return *health
	}
	
	return HealthStatus{
		Venue:       venue,
		Status:      "unknown",
		LastSeen:    time.Time{},
		WSConnected: false,
		RESTHealthy: false,
	}
}

func (f *Facade) CacheStats() CacheStats {
	if f.cache != nil {
		return f.cache.Stats()
	}
	return CacheStats{}
}

// Helper methods

func (f *Facade) getTTLForDataType(dataType string) time.Duration {
	switch dataType {
	case "klines":
		return f.cacheConfig.PricesWarm
	case "trades":
		return f.cacheConfig.PricesHot
	case "book_l2":
		return f.cacheConfig.PricesHot
	default:
		return f.cacheConfig.PricesWarm
	}
}

func (f *Facade) updateAttribution(venue string, source string, count int) {
	if f.attribution[venue] == nil {
		f.attribution[venue] = &Attribution{
			Venue:   venue,
			Sources: []string{},
		}
	}
	
	attr := f.attribution[venue]
	attr.LastUpdate = time.Now()
	
	// Add source if not already present
	found := false
	for _, s := range attr.Sources {
		if s == source {
			found = true
			break
		}
	}
	if !found {
		attr.Sources = append(attr.Sources, source)
	}
}

func (f *Facade) updateCacheHit(venue string) {
	if attr := f.attribution[venue]; attr != nil {
		attr.CacheHits++
	}
}

func (f *Facade) updateCacheMiss(venue string) {
	if attr := f.attribution[venue]; attr != nil {
		attr.CacheMisses++
	}
}

func (f *Facade) updateHealthError(venue string, err error) {
	if f.healthStats[venue] == nil {
		f.healthStats[venue] = &HealthStatus{
			Venue: venue,
		}
	}
	
	health := f.healthStats[venue]
	health.ErrorRate = health.ErrorRate*0.9 + 0.1 // Exponential moving average
	health.Status = "degraded"
	
	log.Warn().Str("venue", venue).Err(err).Float64("error_rate", health.ErrorRate).
		Msg("Updated venue health due to error")
}

func (f *Facade) updateHealthLatency(venue string, latency time.Duration) {
	if f.healthStats[venue] == nil {
		f.healthStats[venue] = &HealthStatus{
			Venue: venue,
		}
	}
	
	health := f.healthStats[venue]
	health.LastSeen = time.Now()
	
	// Update P99 latency using exponential moving average
	if health.P99Latency == 0 {
		health.P99Latency = latency
	} else {
		health.P99Latency = time.Duration(float64(health.P99Latency)*0.9 + float64(latency)*0.1)
	}
	
	// Update status based on latency thresholds
	if latency > 2*time.Second {
		health.Status = "degraded"
		health.Recommendation = "halve size"
	} else if health.Status != "degraded" {
		health.Status = "healthy"
		health.Recommendation = ""
	}
}

func (f *Facade) monitorHealth(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.checkVenueHealth()
		}
	}
}

func (f *Facade) checkVenueHealth() {
	now := time.Now()
	
	for venue, health := range f.healthStats {
		if now.Sub(health.LastSeen) > 10*time.Second {
			health.Status = "degraded"
			health.Recommendation = "check connection"
			log.Warn().Str("venue", venue).Time("last_seen", health.LastSeen).
				Msg("Venue marked as degraded due to inactivity")
		}
	}
}