package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BinanceProvider implements ExchangeProvider for Binance
type BinanceProvider struct {
	config          ProviderConfig
	client          *http.Client
	rateLimiter     *RateLimiter
	circuitBreaker  *CircuitBreaker
	cache           *ProviderCache
	metricsCallback func(string, interface{})
	started         bool
	mu              sync.RWMutex
	
	// Health tracking
	lastHealthCheck time.Time
	consecutiveFailures int
	avgResponseTime time.Duration
	totalRequests   int64
	successfulRequests int64
}

// BinanceOrderBookResponse represents Binance order book API response
type BinanceOrderBookResponse struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids        [][]string `json:"bids"`
	Asks        [][]string `json:"asks"`
}

// BinanceTradeResponse represents Binance recent trades API response
type BinanceTradeResponse []struct {
	ID           int64  `json:"id"`
	Price        string `json:"price"`
	Qty          string `json:"qty"`
	QuoteQty     string `json:"quoteQty"`
	Time         int64  `json:"time"`
	IsBuyerMaker bool   `json:"isBuyerMaker"`
}

// BinanceSystemStatusResponse represents Binance system status
type BinanceSystemStatusResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

// NewBinanceProvider creates a new Binance provider
func NewBinanceProvider(config ProviderConfig, metricsCallback func(string, interface{})) (ExchangeProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.binance.com"
	}
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: config.RateLimit.Timeout,
	}
	
	// Initialize rate limiter
	rateLimiter := NewRateLimiter(config.RateLimit)
	
	// Initialize circuit breaker
	circuitBreaker := NewCircuitBreaker(config.Name, config.CircuitBreaker)
	if metricsCallback != nil {
		circuitBreaker.SetMetricsCallback(metricsCallback)
	}
	
	// Initialize cache
	cache := NewProviderCache(config.CacheConfig)
	
	return &BinanceProvider{
		config:          config,
		client:          client,
		rateLimiter:     rateLimiter,
		circuitBreaker:  circuitBreaker,
		cache:           cache,
		metricsCallback: metricsCallback,
	}, nil
}

func (bp *BinanceProvider) GetName() string {
	return bp.config.Name
}

func (bp *BinanceProvider) GetVenue() string {
	return bp.config.Venue
}

func (bp *BinanceProvider) GetSupportsDerivatives() bool {
	return false // Spot-only for now
}

func (bp *BinanceProvider) GetOrderBook(ctx context.Context, symbol string) (*OrderBookData, error) {
	// Convert symbol format (BTC-USD -> BTCUSDT)
	binanceSymbol := bp.convertSymbol(symbol)
	
	// Check cache first
	cacheKey := fmt.Sprintf("orderbook_%s", binanceSymbol)
	if cached := bp.cache.Get(cacheKey); cached != nil {
		bp.emitMetric("binance_cache_hit", 1)
		return cached.(*OrderBookData), nil
	}
	bp.emitMetric("binance_cache_miss", 1)
	
	// Rate limit
	if err := bp.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	
	// Circuit breaker protection
	var orderBook *OrderBookData
	err := bp.circuitBreaker.Call(func() error {
		var fetchErr error
		orderBook, fetchErr = bp.fetchOrderBook(ctx, binanceSymbol, symbol)
		return fetchErr
	})
	
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	bp.cache.Set(cacheKey, orderBook, bp.config.CacheConfig.TTL)
	
	return orderBook, nil
}

func (bp *BinanceProvider) GetTrades(ctx context.Context, symbol string, limit int) ([]TradeData, error) {
	// Convert symbol format
	binanceSymbol := bp.convertSymbol(symbol)
	
	// Check cache first
	cacheKey := fmt.Sprintf("trades_%s_%d", binanceSymbol, limit)
	if cached := bp.cache.Get(cacheKey); cached != nil {
		bp.emitMetric("binance_cache_hit", 1)
		return cached.([]TradeData), nil
	}
	bp.emitMetric("binance_cache_miss", 1)
	
	// Rate limit
	if err := bp.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	
	// Circuit breaker protection
	var trades []TradeData
	err := bp.circuitBreaker.Call(func() error {
		var fetchErr error
		trades, fetchErr = bp.fetchTrades(ctx, binanceSymbol, symbol, limit)
		return fetchErr
	})
	
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	bp.cache.Set(cacheKey, trades, bp.config.CacheConfig.TTL/2) // Shorter TTL for trades
	
	return trades, nil
}

func (bp *BinanceProvider) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]KlineData, error) {
	return nil, &ProviderError{
		Provider:    bp.config.Name,
		Code:        ErrCodeInsufficientData,
		Message:     "klines not implemented for Binance provider",
		Temporary:   false,
	}
}

func (bp *BinanceProvider) GetFunding(ctx context.Context, symbol string) (*FundingData, error) {
	return nil, &ProviderError{
		Provider:    bp.config.Name,
		Code:        ErrCodeInsufficientData,
		Message:     "funding data not available for spot trading",
		Temporary:   false,
	}
}

func (bp *BinanceProvider) GetOpenInterest(ctx context.Context, symbol string) (*OpenInterestData, error) {
	return nil, &ProviderError{
		Provider:    bp.config.Name,
		Code:        ErrCodeInsufficientData,
		Message:     "open interest not available for spot trading",
		Temporary:   false,
	}
}

func (bp *BinanceProvider) Health() ProviderHealth {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	
	// Calculate success rate
	var successRate float64
	if bp.totalRequests > 0 {
		successRate = float64(bp.successfulRequests) / float64(bp.totalRequests)
	}
	
	// Determine if healthy
	healthy := bp.started && 
		bp.consecutiveFailures < 5 && 
		successRate >= 0.8 &&
		bp.circuitBreaker.GetState() != CircuitOpen
	
	status := "healthy"
	if !healthy {
		status = fmt.Sprintf("unhealthy (failures: %d, success_rate: %.2f, circuit: %s)", 
			bp.consecutiveFailures, successRate, bp.circuitBreaker.GetState())
	}
	
	return ProviderHealth{
		Healthy:      healthy,
		Status:       status,
		ResponseTime: bp.avgResponseTime,
		LastCheck:    bp.lastHealthCheck,
		Metrics: ProviderMetrics{
			RequestCount:     bp.totalRequests,
			ErrorCount:       bp.totalRequests - bp.successfulRequests,
			SuccessRate:      successRate,
			AvgResponseTime:  float64(bp.avgResponseTime.Milliseconds()),
		},
	}
}

func (bp *BinanceProvider) GetLimits() ProviderLimits {
	return bp.config.RateLimit
}

func (bp *BinanceProvider) Start(ctx context.Context) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	if bp.started {
		return nil
	}
	
	// Test connectivity with system status
	err := bp.circuitBreaker.Call(func() error {
		return bp.testConnectivity(ctx)
	})
	
	if err != nil {
		return fmt.Errorf("failed to connect to Binance API: %w", err)
	}
	
	bp.started = true
	bp.lastHealthCheck = time.Now()
	bp.emitMetric("binance_provider_started", 1)
	
	return nil
}

func (bp *BinanceProvider) Stop(ctx context.Context) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	bp.started = false
	bp.emitMetric("binance_provider_stopped", 1)
	
	return nil
}

// fetchOrderBook retrieves order book data from Binance API
func (bp *BinanceProvider) fetchOrderBook(ctx context.Context, binanceSymbol, originalSymbol string) (*OrderBookData, error) {
	url := fmt.Sprintf("%s/api/v3/depth?symbol=%s&limit=100", bp.config.BaseURL, binanceSymbol)
	
	start := time.Now()
	resp, err := bp.makeRequest(ctx, url)
	if err != nil {
		bp.recordRequestResult(false, time.Since(start))
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		bp.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: bp.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("failed to read response: %v", err),
		}
	}
	
	var binanceResp BinanceOrderBookResponse
	if err := json.Unmarshal(body, &binanceResp); err != nil {
		bp.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: bp.config.Name,
			Code:     ErrCodeInvalidData,
			Message:  fmt.Sprintf("failed to parse order book: %v", err),
		}
	}
	
	bp.recordRequestResult(true, time.Since(start))
	
	// Convert to standard format
	orderBook := &OrderBookData{
		Venue:     bp.config.Venue,
		Symbol:    originalSymbol,
		Timestamp: time.Now(),
		Bids:      make([]PriceLevel, 0, len(binanceResp.Bids)),
		Asks:      make([]PriceLevel, 0, len(binanceResp.Asks)),
		ProviderProof: ExchangeProof{
			SourceType: "exchange_native",
			Provider:   "binance",
			Checksum:  fmt.Sprintf("binance_orderbook_%d", binanceResp.LastUpdateID),
		},
	}
	
	// Parse bids
	for _, bid := range binanceResp.Bids {
		if len(bid) >= 2 {
			price, _ := strconv.ParseFloat(bid[0], 64)
			size, _ := strconv.ParseFloat(bid[1], 64)
			orderBook.Bids = append(orderBook.Bids, PriceLevel{
				Price: price,
				Size:  size,
			})
		}
	}
	
	// Parse asks
	for _, ask := range binanceResp.Asks {
		if len(ask) >= 2 {
			price, _ := strconv.ParseFloat(ask[0], 64)
			size, _ := strconv.ParseFloat(ask[1], 64)
			orderBook.Asks = append(orderBook.Asks, PriceLevel{
				Price: price,
				Size:  size,
			})
		}
	}
	
	// Set best bid/ask
	if len(orderBook.Bids) > 0 {
		orderBook.BestBid = orderBook.Bids[0].Price
	}
	if len(orderBook.Asks) > 0 {
		orderBook.BestAsk = orderBook.Asks[0].Price
	}
	
	return orderBook, nil
}

// fetchTrades retrieves recent trades from Binance API
func (bp *BinanceProvider) fetchTrades(ctx context.Context, binanceSymbol, originalSymbol string, limit int) ([]TradeData, error) {
	url := fmt.Sprintf("%s/api/v3/trades?symbol=%s&limit=%d", bp.config.BaseURL, binanceSymbol, limit)
	
	start := time.Now()
	resp, err := bp.makeRequest(ctx, url)
	if err != nil {
		bp.recordRequestResult(false, time.Since(start))
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		bp.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: bp.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("failed to read response: %v", err),
		}
	}
	
	var binanceResp BinanceTradeResponse
	if err := json.Unmarshal(body, &binanceResp); err != nil {
		bp.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: bp.config.Name,
			Code:     ErrCodeInvalidData,
			Message:  fmt.Sprintf("failed to parse trades: %v", err),
		}
	}
	
	bp.recordRequestResult(true, time.Since(start))
	
	// Convert to standard format
	trades := make([]TradeData, 0, len(binanceResp))
	for _, trade := range binanceResp {
		price, _ := strconv.ParseFloat(trade.Price, 64)
		size, _ := strconv.ParseFloat(trade.Qty, 64)
		
		side := "buy"
		if trade.IsBuyerMaker {
			side = "sell"
		}
		
		trades = append(trades, TradeData{
			TradeID:   fmt.Sprintf("%d", trade.ID),
			Venue:     bp.config.Venue,
			Symbol:    originalSymbol,
			Price:     price,
			Size:      size,
			Side:      side,
			Timestamp: time.Unix(trade.Time/1000, (trade.Time%1000)*1000000),
		})
	}
	
	return trades, nil
}

// makeRequest makes an HTTP request with proper error handling
func (bp *BinanceProvider) makeRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, &ProviderError{
			Provider: bp.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("failed to create request: %v", err),
		}
	}
	
	resp, err := bp.client.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Provider: bp.config.Name,
			Code:     ErrCodeNetworkError,
			Message:  fmt.Sprintf("request failed: %v", err),
		}
	}
	
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, &ProviderError{
			Provider: bp.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("API returned status %d", resp.StatusCode),
		}
	}
	
	return resp, nil
}

// testConnectivity tests the connection to Binance
func (bp *BinanceProvider) testConnectivity(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v3/ping", bp.config.BaseURL)
	
	resp, err := bp.makeRequest(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	return nil
}

// convertSymbol converts standard symbol format to Binance format
// BTC-USD -> BTCUSDT, ETH-USD -> ETHUSDT
func (bp *BinanceProvider) convertSymbol(symbol string) string {
	parts := strings.Split(symbol, "-")
	if len(parts) != 2 {
		return symbol
	}
	
	base := parts[0]
	quote := parts[1]
	
	// Convert USD to USDT for Binance
	if quote == "USD" {
		quote = "USDT"
	}
	
	return base + quote
}

// recordRequestResult updates health metrics
func (bp *BinanceProvider) recordRequestResult(success bool, duration time.Duration) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	bp.totalRequests++
	bp.lastHealthCheck = time.Now()
	
	if success {
		bp.successfulRequests++
		bp.consecutiveFailures = 0
	} else {
		bp.consecutiveFailures++
	}
	
	// Update average response time (simple moving average)
	if bp.totalRequests == 1 {
		bp.avgResponseTime = duration
	} else {
		// Weighted average with more weight on recent requests
		weight := 0.1
		bp.avgResponseTime = time.Duration(float64(bp.avgResponseTime)*(1-weight) + float64(duration)*weight)
	}
}

// emitMetric sends a metric if callback is configured
func (bp *BinanceProvider) emitMetric(metric string, value interface{}) {
	if bp.metricsCallback != nil {
		bp.metricsCallback(metric, value)
	}
}