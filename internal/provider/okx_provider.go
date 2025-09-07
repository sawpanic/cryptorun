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

// OKXProvider implements ExchangeProvider for OKX
type OKXProvider struct {
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

// OKXResponse wraps all OKX API responses
type OKXResponse struct {
	Code string      `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// OKXOrderBookData represents OKX order book data structure
type OKXOrderBookData struct {
	InstID string     `json:"instId"`
	Bids   [][]string `json:"bids"`
	Asks   [][]string `json:"asks"`
	TS     string     `json:"ts"`
}

// OKXTradeData represents OKX trade data structure
type OKXTradeData struct {
	InstID string `json:"instId"`
	TradeID string `json:"tradeId"`
	Px     string `json:"px"`
	Sz     string `json:"sz"`
	Side   string `json:"side"`
	TS     string `json:"ts"`
}

// OKXSystemStatusResponse represents OKX system status
type OKXSystemStatusResponse struct {
	State   string    `json:"state"`
	Title   string    `json:"title"`
	Href    string    `json:"href"`
	Begin   string    `json:"begin"`
	End     string    `json:"end"`
	PushTS  string    `json:"pushTs"`
}

// NewOKXProvider creates a new OKX provider
func NewOKXProvider(config ProviderConfig, metricsCallback func(string, interface{})) (ExchangeProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://www.okx.com"
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
	
	return &OKXProvider{
		config:          config,
		client:          client,
		rateLimiter:     rateLimiter,
		circuitBreaker:  circuitBreaker,
		cache:           cache,
		metricsCallback: metricsCallback,
	}, nil
}

func (op *OKXProvider) GetName() string {
	return op.config.Name
}

func (op *OKXProvider) GetVenue() string {
	return op.config.Venue
}

func (op *OKXProvider) GetSupportsDerivatives() bool {
	return true // OKX supports both spot and derivatives
}

func (op *OKXProvider) GetOrderBook(ctx context.Context, symbol string) (*OrderBookData, error) {
	// Convert symbol format (BTC-USD -> BTC-USDT)
	okxSymbol := op.convertSymbol(symbol)
	
	// Check cache first
	cacheKey := fmt.Sprintf("orderbook_%s", okxSymbol)
	if cached := op.cache.Get(cacheKey); cached != nil {
		op.emitMetric("okx_cache_hit", 1)
		return cached.(*OrderBookData), nil
	}
	op.emitMetric("okx_cache_miss", 1)
	
	// Rate limit
	if err := op.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	
	// Circuit breaker protection
	var orderBook *OrderBookData
	err := op.circuitBreaker.Call(func() error {
		var fetchErr error
		orderBook, fetchErr = op.fetchOrderBook(ctx, okxSymbol, symbol)
		return fetchErr
	})
	
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	op.cache.Set(cacheKey, orderBook, op.config.CacheConfig.TTL)
	
	return orderBook, nil
}

func (op *OKXProvider) GetTrades(ctx context.Context, symbol string, limit int) ([]TradeData, error) {
	// Convert symbol format
	okxSymbol := op.convertSymbol(symbol)
	
	// Check cache first
	cacheKey := fmt.Sprintf("trades_%s_%d", okxSymbol, limit)
	if cached := op.cache.Get(cacheKey); cached != nil {
		op.emitMetric("okx_cache_hit", 1)
		return cached.([]TradeData), nil
	}
	op.emitMetric("okx_cache_miss", 1)
	
	// Rate limit
	if err := op.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	
	// Circuit breaker protection
	var trades []TradeData
	err := op.circuitBreaker.Call(func() error {
		var fetchErr error
		trades, fetchErr = op.fetchTrades(ctx, okxSymbol, symbol, limit)
		return fetchErr
	})
	
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	op.cache.Set(cacheKey, trades, op.config.CacheConfig.TTL/2) // Shorter TTL for trades
	
	return trades, nil
}

func (op *OKXProvider) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]KlineData, error) {
	return nil, &ProviderError{
		Provider:    op.config.Name,
		Code:        ErrCodeInsufficientData,
		Message:     "klines not implemented for OKX provider",
		Temporary:   false,
	}
}

func (op *OKXProvider) GetFunding(ctx context.Context, symbol string) (*FundingData, error) {
	// OKX supports funding rates for perpetual contracts
	okxSymbol := op.convertSymbolToSwap(symbol)
	
	// Check cache first
	cacheKey := fmt.Sprintf("funding_%s", okxSymbol)
	if cached := op.cache.Get(cacheKey); cached != nil {
		op.emitMetric("okx_cache_hit", 1)
		return cached.(*FundingData), nil
	}
	
	// Rate limit
	if err := op.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	
	// Circuit breaker protection
	var funding *FundingData
	err := op.circuitBreaker.Call(func() error {
		var fetchErr error
		funding, fetchErr = op.fetchFunding(ctx, okxSymbol, symbol)
		return fetchErr
	})
	
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	op.cache.Set(cacheKey, funding, op.config.CacheConfig.TTL*2) // Longer TTL for funding
	
	return funding, nil
}

func (op *OKXProvider) GetOpenInterest(ctx context.Context, symbol string) (*OpenInterestData, error) {
	// OKX supports open interest for derivatives
	okxSymbol := op.convertSymbolToSwap(symbol)
	
	// Check cache first
	cacheKey := fmt.Sprintf("oi_%s", okxSymbol)
	if cached := op.cache.Get(cacheKey); cached != nil {
		op.emitMetric("okx_cache_hit", 1)
		return cached.(*OpenInterestData), nil
	}
	
	// Rate limit
	if err := op.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	
	// Circuit breaker protection
	var oi *OpenInterestData
	err := op.circuitBreaker.Call(func() error {
		var fetchErr error
		oi, fetchErr = op.fetchOpenInterest(ctx, okxSymbol, symbol)
		return fetchErr
	})
	
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	op.cache.Set(cacheKey, oi, op.config.CacheConfig.TTL)
	
	return oi, nil
}

func (op *OKXProvider) Health() ProviderHealth {
	op.mu.RLock()
	defer op.mu.RUnlock()
	
	// Calculate success rate
	var successRate float64
	if op.totalRequests > 0 {
		successRate = float64(op.successfulRequests) / float64(op.totalRequests)
	}
	
	// Determine if healthy
	healthy := op.started && 
		op.consecutiveFailures < 5 && 
		successRate >= 0.8 &&
		op.circuitBreaker.GetState() != CircuitOpen
	
	status := "healthy"
	if !healthy {
		status = fmt.Sprintf("unhealthy (failures: %d, success_rate: %.2f, circuit: %s)", 
			op.consecutiveFailures, successRate, op.circuitBreaker.GetState())
	}
	
	return ProviderHealth{
		Healthy:      healthy,
		Status:       status,
		ResponseTime: op.avgResponseTime,
		LastCheck:    op.lastHealthCheck,
		Metrics: ProviderMetrics{
			RequestCount:     op.totalRequests,
			ErrorCount:       op.totalRequests - op.successfulRequests,
			SuccessRate:      successRate,
			AvgResponseTime:  float64(op.avgResponseTime.Milliseconds()),
		},
	}
}

func (op *OKXProvider) GetLimits() ProviderLimits {
	return op.config.RateLimit
}

func (op *OKXProvider) Start(ctx context.Context) error {
	op.mu.Lock()
	defer op.mu.Unlock()
	
	if op.started {
		return nil
	}
	
	// Test connectivity with system status
	err := op.circuitBreaker.Call(func() error {
		return op.testConnectivity(ctx)
	})
	
	if err != nil {
		return fmt.Errorf("failed to connect to OKX API: %w", err)
	}
	
	op.started = true
	op.lastHealthCheck = time.Now()
	op.emitMetric("okx_provider_started", 1)
	
	return nil
}

func (op *OKXProvider) Stop(ctx context.Context) error {
	op.mu.Lock()
	defer op.mu.Unlock()
	
	op.started = false
	op.emitMetric("okx_provider_stopped", 1)
	
	return nil
}

// fetchOrderBook retrieves order book data from OKX API
func (op *OKXProvider) fetchOrderBook(ctx context.Context, okxSymbol, originalSymbol string) (*OrderBookData, error) {
	url := fmt.Sprintf("%s/api/v5/market/books?instId=%s&sz=100", op.config.BaseURL, okxSymbol)
	
	start := time.Now()
	resp, err := op.makeRequest(ctx, url)
	if err != nil {
		op.recordRequestResult(false, time.Since(start))
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		op.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("failed to read response: %v", err),
		}
	}
	
	var okxResp OKXResponse
	if err := json.Unmarshal(body, &okxResp); err != nil {
		op.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeInvalidData,
			Message:  fmt.Sprintf("failed to parse response: %v", err),
		}
	}
	
	if okxResp.Code != "0" {
		op.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("OKX API error: %s", okxResp.Msg),
		}
	}
	
	// Parse data array
	dataBytes, err := json.Marshal(okxResp.Data)
	if err != nil {
		op.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeInvalidData,
			Message:  "failed to parse data array",
		}
	}
	
	var okxBooks []OKXOrderBookData
	if err := json.Unmarshal(dataBytes, &okxBooks); err != nil {
		op.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeInvalidData,
			Message:  fmt.Sprintf("failed to parse order book data: %v", err),
		}
	}
	
	if len(okxBooks) == 0 {
		op.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeInsufficientData,
			Message:  "no order book data returned",
		}
	}
	
	op.recordRequestResult(true, time.Since(start))
	
	okxBook := okxBooks[0]
	
	// Convert to standard format
	orderBook := &OrderBookData{
		Venue:     op.config.Venue,
		Symbol:    originalSymbol,
		Timestamp: time.Now(),
		Bids:      make([]PriceLevel, 0, len(okxBook.Bids)),
		Asks:      make([]PriceLevel, 0, len(okxBook.Asks)),
		ProviderProof: ExchangeProof{
			SourceType: "exchange_native",
			Provider:   "okx",
			Checksum:  fmt.Sprintf("okx_orderbook_%s_%s", okxSymbol, okxBook.TS),
		},
	}
	
	// Parse bids
	for _, bid := range okxBook.Bids {
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
	for _, ask := range okxBook.Asks {
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

// fetchTrades retrieves recent trades from OKX API
func (op *OKXProvider) fetchTrades(ctx context.Context, okxSymbol, originalSymbol string, limit int) ([]TradeData, error) {
	url := fmt.Sprintf("%s/api/v5/market/trades?instId=%s&limit=%d", op.config.BaseURL, okxSymbol, limit)
	
	start := time.Now()
	resp, err := op.makeRequest(ctx, url)
	if err != nil {
		op.recordRequestResult(false, time.Since(start))
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		op.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("failed to read response: %v", err),
		}
	}
	
	var okxResp OKXResponse
	if err := json.Unmarshal(body, &okxResp); err != nil {
		op.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeInvalidData,
			Message:  fmt.Sprintf("failed to parse response: %v", err),
		}
	}
	
	if okxResp.Code != "0" {
		op.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("OKX API error: %s", okxResp.Msg),
		}
	}
	
	// Parse data array
	dataBytes, err := json.Marshal(okxResp.Data)
	if err != nil {
		op.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeInvalidData,
			Message:  "failed to parse data array",
		}
	}
	
	var okxTrades []OKXTradeData
	if err := json.Unmarshal(dataBytes, &okxTrades); err != nil {
		op.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeInvalidData,
			Message:  fmt.Sprintf("failed to parse trade data: %v", err),
		}
	}
	
	op.recordRequestResult(true, time.Since(start))
	
	// Convert to standard format
	trades := make([]TradeData, 0, len(okxTrades))
	for _, trade := range okxTrades {
		price, _ := strconv.ParseFloat(trade.Px, 64)
		size, _ := strconv.ParseFloat(trade.Sz, 64)
		
		// Convert timestamp from milliseconds
		ts, _ := strconv.ParseInt(trade.TS, 10, 64)
		timestamp := time.Unix(ts/1000, (ts%1000)*1000000)
		
		trades = append(trades, TradeData{
			TradeID:   trade.TradeID,
			Venue:     op.config.Venue,
			Symbol:    originalSymbol,
			Price:     price,
			Size:      size,
			Side:      trade.Side,
			Timestamp: timestamp,
		})
	}
	
	return trades, nil
}

// fetchFunding retrieves funding rate from OKX API
func (op *OKXProvider) fetchFunding(ctx context.Context, okxSymbol, originalSymbol string) (*FundingData, error) {
	url := fmt.Sprintf("%s/api/v5/public/funding-rate?instId=%s", op.config.BaseURL, okxSymbol)
	
	resp, err := op.makeRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Stub implementation - would parse actual funding rate data
	return &FundingData{
		Venue:        op.config.Venue,
		Symbol:       originalSymbol,
		FundingRate:  0.0001, // Stub value
		NextFunding:  time.Now().Add(8 * time.Hour),
		Timestamp:    time.Now(),
	}, nil
}

// fetchOpenInterest retrieves open interest from OKX API
func (op *OKXProvider) fetchOpenInterest(ctx context.Context, okxSymbol, originalSymbol string) (*OpenInterestData, error) {
	url := fmt.Sprintf("%s/api/v5/public/open-interest?instId=%s", op.config.BaseURL, okxSymbol)
	
	resp, err := op.makeRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Stub implementation - would parse actual OI data
	return &OpenInterestData{
		Venue:        op.config.Venue,
		Symbol:       originalSymbol,
		OpenInterest: 1000000, // Stub value
		Timestamp:    time.Now(),
	}, nil
}

// makeRequest makes an HTTP request with proper error handling
func (op *OKXProvider) makeRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("failed to create request: %v", err),
		}
	}
	
	resp, err := op.client.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeNetworkError,
			Message:  fmt.Sprintf("request failed: %v", err),
		}
	}
	
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, &ProviderError{
			Provider: op.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("API returned status %d", resp.StatusCode),
		}
	}
	
	return resp, nil
}

// testConnectivity tests the connection to OKX
func (op *OKXProvider) testConnectivity(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v5/system/time", op.config.BaseURL)
	
	resp, err := op.makeRequest(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	return nil
}

// convertSymbol converts standard symbol format to OKX spot format
// BTC-USD -> BTC-USDT
func (op *OKXProvider) convertSymbol(symbol string) string {
	parts := strings.Split(symbol, "-")
	if len(parts) != 2 {
		return symbol
	}
	
	base := parts[0]
	quote := parts[1]
	
	// Convert USD to USDT for OKX spot
	if quote == "USD" {
		quote = "USDT"
	}
	
	return fmt.Sprintf("%s-%s", base, quote)
}

// convertSymbolToSwap converts standard symbol format to OKX swap format
// BTC-USD -> BTC-USD-SWAP
func (op *OKXProvider) convertSymbolToSwap(symbol string) string {
	return fmt.Sprintf("%s-SWAP", symbol)
}

// recordRequestResult updates health metrics
func (op *OKXProvider) recordRequestResult(success bool, duration time.Duration) {
	op.mu.Lock()
	defer op.mu.Unlock()
	
	op.totalRequests++
	op.lastHealthCheck = time.Now()
	
	if success {
		op.successfulRequests++
		op.consecutiveFailures = 0
	} else {
		op.consecutiveFailures++
	}
	
	// Update average response time (simple moving average)
	if op.totalRequests == 1 {
		op.avgResponseTime = duration
	} else {
		// Weighted average with more weight on recent requests
		weight := 0.1
		op.avgResponseTime = time.Duration(float64(op.avgResponseTime)*(1-weight) + float64(duration)*weight)
	}
}

// emitMetric sends a metric if callback is configured
func (op *OKXProvider) emitMetric(metric string, value interface{}) {
	if op.metricsCallback != nil {
		op.metricsCallback(metric, value)
	}
}