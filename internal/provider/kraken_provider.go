package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// KrakenProvider implements ExchangeProvider for Kraken exchange
type KrakenProvider struct {
	config          ProviderConfig
	httpClient      *http.Client
	circuitBreaker  *CircuitBreaker
	rateLimiter     *RateLimiter
	cache           *ProviderCache
	
	// State management
	started         bool
	mu              sync.RWMutex
	
	// Metrics
	metricsCallback func(string, interface{})
	metrics         ProviderMetrics
	lastHealthCheck time.Time
}

// NewKrakenProvider creates a new Kraken provider
func NewKrakenProvider(config ProviderConfig, metricsCallback func(string, interface{})) (ExchangeProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.kraken.com/0/public"
	}
	
	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: config.RateLimit.Timeout,
	}
	
	// Create circuit breaker
	circuitBreaker := NewCircuitBreaker(
		fmt.Sprintf("kraken_%s", config.Name),
		config.CircuitBreaker,
	)
	circuitBreaker.SetMetricsCallback(metricsCallback)
	
	// Create rate limiter
	rateLimiter := NewRateLimiter(config.RateLimit)
	
	// Create cache
	cache := NewProviderCache(config.CacheConfig)
	
	provider := &KrakenProvider{
		config:          config,
		httpClient:      httpClient,
		circuitBreaker:  circuitBreaker,
		rateLimiter:     rateLimiter,
		cache:           cache,
		metricsCallback: metricsCallback,
		metrics: ProviderMetrics{
			RequestCount:    0,
			ErrorCount:      0,
			SuccessRate:     1.0,
			AvgResponseTime: 0,
		},
	}
	
	return provider, nil
}

// GetName returns the provider name
func (k *KrakenProvider) GetName() string {
	return k.config.Name
}

// GetVenue returns the venue name
func (k *KrakenProvider) GetVenue() string {
	return "kraken"
}

// GetSupportsDerivatives returns whether derivatives are supported
func (k *KrakenProvider) GetSupportsDerivatives() bool {
	return false // Kraken spot only in this implementation
}

// Start initializes the provider
func (k *KrakenProvider) Start(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	if k.started {
		return nil
	}
	
	// Test connectivity
	if err := k.testConnectivity(ctx); err != nil {
		return fmt.Errorf("kraken connectivity test failed: %w", err)
	}
	
	k.started = true
	k.emitMetric("provider_started", 1)
	
	return nil
}

// Stop shuts down the provider
func (k *KrakenProvider) Stop(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	if !k.started {
		return nil
	}
	
	k.started = false
	k.emitMetric("provider_stopped", 1)
	
	return nil
}

// GetOrderBook retrieves order book data from Kraken
func (k *KrakenProvider) GetOrderBook(ctx context.Context, symbol string) (*OrderBookData, error) {
	if !k.started {
		return nil, fmt.Errorf("provider not started")
	}
	
	// Check cache first
	if k.cache.Enabled() {
		cacheKey := fmt.Sprintf("orderbook_%s", symbol)
		if cached := k.cache.Get(cacheKey); cached != nil {
			if orderBook, ok := cached.(*OrderBookData); ok {
				k.emitMetric("cache_hit", 1)
				return orderBook, nil
			}
		}
	}
	
	// Execute with circuit breaker protection
	var orderBook *OrderBookData
	err := k.circuitBreaker.Call(func() error {
		var callErr error
		orderBook, callErr = k.fetchOrderBook(ctx, symbol)
		return callErr
	})
	
	if err != nil {
		k.recordError()
		return nil, err
	}
	
	k.recordSuccess()
	
	// Cache the result
	if k.cache.Enabled() {
		cacheKey := fmt.Sprintf("orderbook_%s", symbol)
		k.cache.Set(cacheKey, orderBook, k.config.CacheConfig.TTL)
	}
	
	return orderBook, nil
}

// fetchOrderBook makes the actual API call to fetch order book
func (k *KrakenProvider) fetchOrderBook(ctx context.Context, symbol string) (*OrderBookData, error) {
	// Apply rate limiting
	if err := k.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		k.emitMetric("api_request_duration_ms", duration.Milliseconds())
	}()
	
	// Convert symbol to Kraken format (BTC-USD -> XXBTZUSD)
	krakenSymbol := k.convertSymbol(symbol)
	
	url := fmt.Sprintf("%s/Depth?pair=%s&count=10", k.config.BaseURL, krakenSymbol)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", "CryptoRun/1.0")
	
	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, &ProviderError{
			Provider:    "kraken",
			Code:        fmt.Sprintf("HTTP_%d", resp.StatusCode),
			Message:     fmt.Sprintf("HTTP error: %d", resp.StatusCode),
			HTTPStatus:  resp.StatusCode,
			RateLimited: resp.StatusCode == 429,
			Temporary:   resp.StatusCode >= 500,
		}
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	// Parse Kraken response
	var krakenResp KrakenDepthResponse
	if err := json.Unmarshal(body, &krakenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if len(krakenResp.Error) > 0 {
		return nil, &ProviderError{
			Provider:    "kraken",
			Code:        ErrCodeInsufficientData,
			Message:     strings.Join(krakenResp.Error, "; "),
			Temporary:   true,
		}
	}
	
	// Convert to standard format
	orderBook, err := k.convertOrderBook(krakenResp, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to convert order book: %w", err)
	}
	
	// Add exchange proof
	orderBook.ProviderProof = CreateExchangeProof(
		"kraken", 
		url, 
		time.Since(start), 
		orderBook,
	)
	
	return orderBook, nil
}

// GetTrades retrieves recent trades
func (k *KrakenProvider) GetTrades(ctx context.Context, symbol string, limit int) ([]TradeData, error) {
	if !k.started {
		return nil, fmt.Errorf("provider not started")
	}
	
	var trades []TradeData
	err := k.circuitBreaker.Call(func() error {
		var callErr error
		trades, callErr = k.fetchTrades(ctx, symbol, limit)
		return callErr
	})
	
	if err != nil {
		k.recordError()
		return nil, err
	}
	
	k.recordSuccess()
	return trades, nil
}

// fetchTrades makes the actual API call to fetch trades
func (k *KrakenProvider) fetchTrades(ctx context.Context, symbol string, limit int) ([]TradeData, error) {
	if err := k.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	krakenSymbol := k.convertSymbol(symbol)
	url := fmt.Sprintf("%s/Trades?pair=%s&count=%d", k.config.BaseURL, krakenSymbol, limit)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var krakenResp KrakenTradesResponse
	if err := json.Unmarshal(body, &krakenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return k.convertTrades(krakenResp, symbol), nil
}

// GetKlines retrieves OHLCV data
func (k *KrakenProvider) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]KlineData, error) {
	if !k.started {
		return nil, fmt.Errorf("provider not started")
	}
	
	var klines []KlineData
	err := k.circuitBreaker.Call(func() error {
		var callErr error
		klines, callErr = k.fetchKlines(ctx, symbol, interval, limit)
		return callErr
	})
	
	if err != nil {
		k.recordError()
		return nil, err
	}
	
	k.recordSuccess()
	return klines, nil
}

// GetFunding returns funding data (not supported by Kraken spot)
func (k *KrakenProvider) GetFunding(ctx context.Context, symbol string) (*FundingData, error) {
	return nil, &ProviderError{
		Provider:    "kraken",
		Code:        ErrCodeInsufficientData,
		Message:     "funding data not available for spot markets",
		Temporary:   false,
	}
}

// GetOpenInterest returns open interest data (not supported by Kraken spot)
func (k *KrakenProvider) GetOpenInterest(ctx context.Context, symbol string) (*OpenInterestData, error) {
	return nil, &ProviderError{
		Provider:    "kraken",
		Code:        ErrCodeInsufficientData,
		Message:     "open interest not available for spot markets",
		Temporary:   false,
	}
}

// Health returns provider health status
func (k *KrakenProvider) Health() ProviderHealth {
	k.mu.RLock()
	defer k.mu.RUnlock()
	
	now := time.Now()
	healthy := k.started && (now.Sub(k.lastHealthCheck) < 2*time.Minute)
	
	// Update metrics
	k.updateHealthMetrics()
	
	health := ProviderHealth{
		Healthy:      healthy,
		Status:       k.getStatusString(healthy),
		LastCheck:    now,
		ResponseTime: time.Duration(k.metrics.AvgResponseTime) * time.Millisecond,
		CircuitState: k.circuitBreaker.GetState().String(),
		Metrics:      k.metrics,
	}
	
	k.lastHealthCheck = now
	return health
}

// GetLimits returns provider rate limits
func (k *KrakenProvider) GetLimits() ProviderLimits {
	return k.config.RateLimit
}

// Helper methods

func (k *KrakenProvider) testConnectivity(ctx context.Context) error {
	url := fmt.Sprintf("%s/SystemStatus", k.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	
	resp, err := k.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	return nil
}

func (k *KrakenProvider) convertSymbol(symbol string) string {
	// Convert standard format (BTC-USD) to Kraken format
	// This is a simplified conversion - real implementation would use mapping table
	parts := strings.Split(symbol, "-")
	if len(parts) != 2 {
		return symbol
	}
	
	base := parts[0]
	quote := parts[1]
	
	// Kraken uses different naming conventions
	if base == "BTC" {
		base = "XXBT"
	}
	if quote == "USD" {
		quote = "ZUSD"
	}
	
	return base + quote
}

func (k *KrakenProvider) convertOrderBook(krakenResp KrakenDepthResponse, symbol string) (*OrderBookData, error) {
	// Extract the order book data (Kraken returns map with symbol as key)
	var depthData KrakenDepthData
	for _, data := range krakenResp.Result {
		depthData = data
		break
	}
	
	if len(depthData.Bids) == 0 || len(depthData.Asks) == 0 {
		return nil, fmt.Errorf("insufficient order book data")
	}
	
	// Convert to standard format
	bids := make([]PriceLevel, len(depthData.Bids))
	for i, bid := range depthData.Bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		size, _ := strconv.ParseFloat(bid[1], 64)
		bids[i] = PriceLevel{Price: price, Size: size}
	}
	
	asks := make([]PriceLevel, len(depthData.Asks))
	for i, ask := range depthData.Asks {
		price, _ := strconv.ParseFloat(ask[0], 64)
		size, _ := strconv.ParseFloat(ask[1], 64)
		asks[i] = PriceLevel{Price: price, Size: size}
	}
	
	// Sort bids (highest first) and asks (lowest first)
	sort.Slice(bids, func(i, j int) bool { return bids[i].Price > bids[j].Price })
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })
	
	bestBid := bids[0]
	bestAsk := asks[0]
	midPrice := (bestBid.Price + bestAsk.Price) / 2
	spreadBps := ((bestAsk.Price - bestBid.Price) / midPrice) * 10000
	
	return &OrderBookData{
		Venue:         "kraken",
		Symbol:        symbol,
		Timestamp:     time.Now(),
		BestBid:       bestBid.Price,
		BestAsk:       bestAsk.Price,
		BestBidSize:   bestBid.Size,
		BestAskSize:   bestAsk.Size,
		MidPrice:      midPrice,
		SpreadBps:     spreadBps,
		Bids:          bids,
		Asks:          asks,
	}, nil
}

func (k *KrakenProvider) convertTrades(krakenResp KrakenTradesResponse, symbol string) []TradeData {
	var trades []TradeData
	
	// Extract trades from response (simplified)
	for _, tradeList := range krakenResp.Result {
		for _, trade := range tradeList {
			if len(trade) >= 4 {
				price, _ := strconv.ParseFloat(trade[0], 64)
				size, _ := strconv.ParseFloat(trade[1], 64)
				timestamp := parseKrakenTimestamp(trade[2])
				side := "buy"
				if trade[3] == "s" {
					side = "sell"
				}
				
				trades = append(trades, TradeData{
					Venue:     "kraken",
					Symbol:    symbol,
					Timestamp: timestamp,
					Price:     price,
					Size:      size,
					Side:      side,
				})
			}
		}
		break // Only process first symbol's trades
	}
	
	return trades
}

func (k *KrakenProvider) fetchKlines(ctx context.Context, symbol string, interval string, limit int) ([]KlineData, error) {
	// Simplified implementation - would need full Kraken OHLC API integration
	return []KlineData{}, nil
}

func (k *KrakenProvider) recordSuccess() {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	k.metrics.RequestCount++
	k.updateSuccessRate()
	k.emitMetric("provider_request_success", 1)
}

func (k *KrakenProvider) recordError() {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	k.metrics.RequestCount++
	k.metrics.ErrorCount++
	k.updateSuccessRate()
	k.emitMetric("provider_request_error", 1)
}

func (k *KrakenProvider) updateSuccessRate() {
	if k.metrics.RequestCount > 0 {
		successCount := k.metrics.RequestCount - k.metrics.ErrorCount
		k.metrics.SuccessRate = float64(successCount) / float64(k.metrics.RequestCount)
	}
}

func (k *KrakenProvider) updateHealthMetrics() {
	// Update cache hit rate
	if k.cache != nil {
		k.metrics.CacheHitRate = k.cache.GetHitRate()
	}
}

func (k *KrakenProvider) getStatusString(healthy bool) string {
	if !k.started {
		return "stopped"
	}
	if !healthy {
		return "unhealthy"
	}
	return "healthy"
}

func (k *KrakenProvider) emitMetric(metric string, value interface{}) {
	if k.metricsCallback != nil {
		k.metricsCallback(metric, value)
	}
}

// Helper function to parse Kraken timestamp
func parseKrakenTimestamp(timestampStr string) time.Time {
	if timestamp, err := strconv.ParseFloat(timestampStr, 64); err == nil {
		return time.Unix(int64(timestamp), 0)
	}
	return time.Now()
}

// Kraken API response structures

type KrakenDepthResponse struct {
	Error  []string                    `json:"error"`
	Result map[string]KrakenDepthData  `json:"result"`
}

type KrakenDepthData struct {
	Bids [][]string `json:"bids"`
	Asks [][]string `json:"asks"`
}

type KrakenTradesResponse struct {
	Error  []string                       `json:"error"`
	Result map[string][][]string         `json:"result"`
}