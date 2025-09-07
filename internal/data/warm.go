package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cryptorun/internal/providers/guards"
)

// WarmData implements REST API + cache data tier
type WarmData struct {
	guards map[string]*guards.ProviderGuard // venue -> guard
	client *http.Client

	// Configuration
	defaultTTL time.Duration
}

// RestEndpoint configuration for venue REST APIs
type RestEndpoint struct {
	BaseURL       string
	OrderBookPath string
	PricePath     string
	Headers       map[string]string
}

// WarmConfig holds configuration for warm tier
type WarmConfig struct {
	DefaultTTLSeconds int                              `json:"default_ttl_seconds"`
	Endpoints         map[string]RestEndpoint          `json:"endpoints"`
	GuardConfigs      map[string]guards.ProviderConfig `json:"guard_configs"`
}

// NewWarmData creates a new warm data tier
func NewWarmData(config WarmConfig) *WarmData {
	w := &WarmData{
		guards: make(map[string]*guards.ProviderGuard),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		defaultTTL: time.Duration(config.DefaultTTLSeconds) * time.Second,
	}

	// Initialize guards for each venue
	for venue, guardConfig := range config.GuardConfigs {
		w.guards[venue] = guards.NewProviderGuard(guardConfig)
	}

	return w
}

// GetOrderBook retrieves order book via REST API with caching
func (w *WarmData) GetOrderBook(ctx context.Context, venue, symbol string) (*Envelope, error) {
	guard, exists := w.guards[venue]
	if !exists {
		return nil, fmt.Errorf("no guard configured for venue: %s", venue)
	}

	// Create request
	cacheKey := fmt.Sprintf("orderbook:%s:%s", venue, symbol)
	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      w.buildOrderBookURL(venue, symbol),
		Headers:  w.buildHeaders(venue),
		CacheKey: cacheKey,
	}

	// Execute with provider guard
	resp, err := guard.Execute(ctx, req, w.fetchHTTP)
	if err != nil {
		return nil, fmt.Errorf("warm tier fetch failed for %s %s: %w", venue, symbol, err)
	}

	// Convert to envelope
	envelope, err := w.convertToEnvelope(venue, symbol, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to convert response to envelope: %w", err)
	}

	return envelope, nil
}

// GetPriceData retrieves price data via REST API with caching
func (w *WarmData) GetPriceData(ctx context.Context, venue, symbol string) (*Envelope, error) {
	// For warm tier, price data uses same endpoint as order book (simplified)
	return w.GetOrderBook(ctx, venue, symbol)
}

// IsAvailable checks if warm data is available for venue
func (w *WarmData) IsAvailable(ctx context.Context, venue string) bool {
	guard, exists := w.guards[venue]
	if !exists {
		return false
	}

	// Check circuit breaker status
	health := guard.Health()
	return !health.CircuitOpen
}

// SetCacheTTL updates cache TTL for a venue (implementation depends on cache internals)
func (w *WarmData) SetCacheTTL(venue string, ttlSeconds int) {
	// Note: Current guard implementation doesn't expose TTL modification
	// This would require enhancing the guard interface
}

// InvalidateCache removes cached data for venue/symbol
func (w *WarmData) InvalidateCache(venue, symbol string) error {
	guard, exists := w.guards[venue]
	if !exists {
		return fmt.Errorf("no guard configured for venue: %s", venue)
	}

	// Note: Current guard implementation doesn't expose cache invalidation
	// This would require enhancing the guard interface
	_ = guard // Silence unused variable
	return fmt.Errorf("cache invalidation not yet implemented")
}

// GetCacheStats returns aggregated cache statistics
func (w *WarmData) GetCacheStats() CacheStats {
	var totalHits, totalMisses int64
	var totalHitRate float64
	venueCount := 0
	lastUpdated := time.Now()

	for _, guard := range w.guards {
		health := guard.Health()
		totalHitRate += health.CacheHitRate
		totalHits += health.RequestCount // Simplified - actual hits would need guard enhancement
		venueCount++

		if health.LastSuccess.After(lastUpdated) {
			lastUpdated = health.LastSuccess
		}
	}

	var avgHitRate float64
	if venueCount > 0 {
		avgHitRate = totalHitRate / float64(venueCount)
	}

	return CacheStats{
		HitRate:     avgHitRate,
		MissCount:   totalMisses,
		ErrorCount:  0, // Would need guard enhancement
		LastUpdated: lastUpdated,
	}
}

// buildOrderBookURL constructs REST API URL for order book
func (w *WarmData) buildOrderBookURL(venue, symbol string) string {
	// Simplified URL building - would use actual endpoint configuration
	switch venue {
	case "binance":
		return fmt.Sprintf("https://api.binance.com/api/v3/depth?symbol=%s&limit=100", symbol)
	case "okx":
		return fmt.Sprintf("https://www.okx.com/api/v5/market/books?instId=%s&sz=100", symbol)
	case "coinbase":
		return fmt.Sprintf("https://api.exchange.coinbase.com/products/%s/book?level=2", symbol)
	case "kraken":
		return fmt.Sprintf("https://api.kraken.com/0/public/Depth?pair=%s&count=100", symbol)
	default:
		return fmt.Sprintf("https://api.%s.com/orderbook/%s", venue, symbol)
	}
}

// buildHeaders constructs HTTP headers for venue
func (w *WarmData) buildHeaders(venue string) map[string]string {
	headers := map[string]string{
		"User-Agent":   "CryptoRun/3.2.1",
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}

	// Add venue-specific headers if needed
	switch venue {
	case "okx":
		headers["OK-ACCESS-KEY"] = "" // Would be loaded from config
	case "coinbase":
		headers["CB-VERSION"] = "2021-06-04"
	}

	return headers
}

// fetchHTTP performs the actual HTTP request
func (w *WarmData) fetchHTTP(ctx context.Context, req guards.GuardedRequest) (*guards.GuardedResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Execute request
	resp, err := w.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &guards.GuardedResponse{
		Data:       body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Cached:     false,
		Age:        0,
	}, nil
}

// convertToEnvelope converts REST response to standard envelope
func (w *WarmData) convertToEnvelope(venue, symbol string, resp *guards.GuardedResponse) (*Envelope, error) {
	now := time.Now()

	envelope := NewEnvelope(venue, symbol, TierWarm,
		WithCacheHit(resp.Cached),
		WithConfidenceScore(0.85), // Lower confidence than hot tier
	)

	envelope.Provenance.OriginalSource = fmt.Sprintf("%s_rest", venue)
	envelope.Provenance.LatencyMS = int64(resp.Age.Milliseconds())
	envelope.Provenance.RetrievedAt = now
	envelope.Provenance.RetryCount = resp.RetryCount

	// Parse venue-specific response format
	orderBookData, err := w.parseOrderBookResponse(venue, resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse order book response: %w", err)
	}

	envelope.OrderBook = orderBookData
	envelope.Checksum = envelope.GenerateChecksum(orderBookData, "order_book")

	return envelope, nil
}

// parseOrderBookResponse parses venue-specific JSON responses
func (w *WarmData) parseOrderBookResponse(venue string, data []byte) (interface{}, error) {
	// Simplified parsing - would implement venue-specific parsers
	var genericResponse map[string]interface{}
	if err := json.Unmarshal(data, &genericResponse); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Add venue and timestamp
	genericResponse["venue"] = venue
	genericResponse["parsed_at"] = time.Now()

	return genericResponse, nil
}

// GetHealthStatus returns health status of all venue guards
func (w *WarmData) GetHealthStatus() map[string]guards.ProviderHealth {
	status := make(map[string]guards.ProviderHealth)

	for venue, guard := range w.guards {
		status[venue] = guard.Health()
	}

	return status
}
