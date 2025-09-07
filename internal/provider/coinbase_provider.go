package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// CoinbaseProvider implements ExchangeProvider for Coinbase Advanced Trade
type CoinbaseProvider struct {
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

// CoinbaseOrderBookResponse represents Coinbase order book API response
type CoinbaseOrderBookResponse struct {
	PriceBook struct {
		ProductID string                    `json:"product_id"`
		Bids      []CoinbaseOrderBookLevel `json:"bids"`
		Asks      []CoinbaseOrderBookLevel `json:"asks"`
		Time      time.Time                `json:"time"`
	} `json:"pricebook"`
}

type CoinbaseOrderBookLevel struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

// CoinbaseTradeResponse represents Coinbase recent trades API response
type CoinbaseTradeResponse struct {
	Trades []CoinbaseTrade `json:"trades"`
}

type CoinbaseTrade struct {
	TradeID   string    `json:"trade_id"`
	ProductID string    `json:"product_id"`
	Price     string    `json:"price"`
	Size      string    `json:"size"`
	Side      string    `json:"side"`
	Time      time.Time `json:"time"`
}

// CoinbaseProductResponse represents Coinbase product information
type CoinbaseProductResponse struct {
	Products []CoinbaseProduct `json:"products"`
}

type CoinbaseProduct struct {
	ProductID     string `json:"product_id"`
	Price         string `json:"price"`
	PriceChange24h string `json:"price_change_24h"`
	Volume24h     string `json:"volume_24h"`
	Status        string `json:"status"`
}

// NewCoinbaseProvider creates a new Coinbase provider
func NewCoinbaseProvider(config ProviderConfig, metricsCallback func(string, interface{})) (ExchangeProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.coinbase.com"
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
	
	return &CoinbaseProvider{
		config:          config,
		client:          client,
		rateLimiter:     rateLimiter,
		circuitBreaker:  circuitBreaker,
		cache:           cache,
		metricsCallback: metricsCallback,
	}, nil
}

func (cp *CoinbaseProvider) GetName() string {
	return cp.config.Name
}

func (cp *CoinbaseProvider) GetVenue() string {
	return cp.config.Venue
}

func (cp *CoinbaseProvider) GetSupportsDerivatives() bool {
	return false // Spot-only for now
}

func (cp *CoinbaseProvider) GetOrderBook(ctx context.Context, symbol string) (*OrderBookData, error) {
	// Convert symbol format (BTC-USD -> BTC-USD, no change needed for Coinbase)
	coinbaseSymbol := cp.convertSymbol(symbol)
	
	// Check cache first
	cacheKey := fmt.Sprintf("orderbook_%s", coinbaseSymbol)
	if cached := cp.cache.Get(cacheKey); cached != nil {
		cp.emitMetric("coinbase_cache_hit", 1)
		return cached.(*OrderBookData), nil
	}
	cp.emitMetric("coinbase_cache_miss", 1)
	
	// Rate limit
	if err := cp.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	
	// Circuit breaker protection
	var orderBook *OrderBookData
	err := cp.circuitBreaker.Call(func() error {
		var fetchErr error
		orderBook, fetchErr = cp.fetchOrderBook(ctx, coinbaseSymbol, symbol)
		return fetchErr
	})
	
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	cp.cache.Set(cacheKey, orderBook, cp.config.CacheConfig.TTL)
	
	return orderBook, nil
}

func (cp *CoinbaseProvider) GetTrades(ctx context.Context, symbol string, limit int) ([]TradeData, error) {
	// Convert symbol format
	coinbaseSymbol := cp.convertSymbol(symbol)
	
	// Check cache first
	cacheKey := fmt.Sprintf("trades_%s_%d", coinbaseSymbol, limit)
	if cached := cp.cache.Get(cacheKey); cached != nil {
		cp.emitMetric("coinbase_cache_hit", 1)
		return cached.([]TradeData), nil
	}
	cp.emitMetric("coinbase_cache_miss", 1)
	
	// Rate limit
	if err := cp.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	
	// Circuit breaker protection
	var trades []TradeData
	err := cp.circuitBreaker.Call(func() error {
		var fetchErr error
		trades, fetchErr = cp.fetchTrades(ctx, coinbaseSymbol, symbol, limit)
		return fetchErr
	})
	
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	cp.cache.Set(cacheKey, trades, cp.config.CacheConfig.TTL/2) // Shorter TTL for trades
	
	return trades, nil
}

func (cp *CoinbaseProvider) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]KlineData, error) {
	return nil, &ProviderError{
		Provider:    cp.config.Name,
		Code:        ErrCodeInsufficientData,
		Message:     "klines not implemented for Coinbase provider",
		Temporary:   false,
	}
}

func (cp *CoinbaseProvider) GetFunding(ctx context.Context, symbol string) (*FundingData, error) {
	return nil, &ProviderError{
		Provider:    cp.config.Name,
		Code:        ErrCodeInsufficientData,
		Message:     "funding data not available for spot trading",
		Temporary:   false,
	}
}

func (cp *CoinbaseProvider) GetOpenInterest(ctx context.Context, symbol string) (*OpenInterestData, error) {
	return nil, &ProviderError{
		Provider:    cp.config.Name,
		Code:        ErrCodeInsufficientData,
		Message:     "open interest not available for spot trading",
		Temporary:   false,
	}
}

func (cp *CoinbaseProvider) Health() ProviderHealth {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	
	// Calculate success rate
	var successRate float64
	if cp.totalRequests > 0 {
		successRate = float64(cp.successfulRequests) / float64(cp.totalRequests)
	}
	
	// Determine if healthy
	healthy := cp.started && 
		cp.consecutiveFailures < 5 && 
		successRate >= 0.8 &&
		cp.circuitBreaker.GetState() != CircuitOpen
	
	status := "healthy"
	if !healthy {
		status = fmt.Sprintf("unhealthy (failures: %d, success_rate: %.2f, circuit: %s)", 
			cp.consecutiveFailures, successRate, cp.circuitBreaker.GetState())
	}
	
	return ProviderHealth{
		Healthy:      healthy,
		Status:       status,
		ResponseTime: cp.avgResponseTime,
		LastCheck:    cp.lastHealthCheck,
		Metrics: ProviderMetrics{
			RequestCount:     cp.totalRequests,
			ErrorCount:       cp.totalRequests - cp.successfulRequests,
			SuccessRate:      successRate,
			AvgResponseTime:  float64(cp.avgResponseTime.Milliseconds()),
		},
	}
}

func (cp *CoinbaseProvider) GetLimits() ProviderLimits {
	return cp.config.RateLimit
}

func (cp *CoinbaseProvider) Start(ctx context.Context) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	if cp.started {
		return nil
	}
	
	// Test connectivity with products endpoint
	err := cp.circuitBreaker.Call(func() error {
		return cp.testConnectivity(ctx)
	})
	
	if err != nil {
		return fmt.Errorf("failed to connect to Coinbase API: %w", err)
	}
	
	cp.started = true
	cp.lastHealthCheck = time.Now()
	cp.emitMetric("coinbase_provider_started", 1)
	
	return nil
}

func (cp *CoinbaseProvider) Stop(ctx context.Context) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	cp.started = false
	cp.emitMetric("coinbase_provider_stopped", 1)
	
	return nil
}

// fetchOrderBook retrieves order book data from Coinbase API
func (cp *CoinbaseProvider) fetchOrderBook(ctx context.Context, coinbaseSymbol, originalSymbol string) (*OrderBookData, error) {
	url := fmt.Sprintf("%s/api/v3/brokerage/product_book?product_id=%s&limit=100", cp.config.BaseURL, coinbaseSymbol)
	
	start := time.Now()
	resp, err := cp.makeRequest(ctx, url)
	if err != nil {
		cp.recordRequestResult(false, time.Since(start))
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		cp.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: cp.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("failed to read response: %v", err),
		}
	}
	
	var coinbaseResp CoinbaseOrderBookResponse
	if err := json.Unmarshal(body, &coinbaseResp); err != nil {
		cp.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: cp.config.Name,
			Code:     ErrCodeInvalidData,
			Message:  fmt.Sprintf("failed to parse order book: %v", err),
		}
	}
	
	cp.recordRequestResult(true, time.Since(start))
	
	// Convert to standard format
	orderBook := &OrderBookData{
		Venue:     cp.config.Venue,
		Symbol:    originalSymbol,
		Timestamp: coinbaseResp.PriceBook.Time,
		Bids:      make([]PriceLevel, 0, len(coinbaseResp.PriceBook.Bids)),
		Asks:      make([]PriceLevel, 0, len(coinbaseResp.PriceBook.Asks)),
		ProviderProof: ExchangeProof{
			SourceType: "exchange_native",
			Provider:   "coinbase",
			Checksum:  fmt.Sprintf("coinbase_orderbook_%s_%d", coinbaseSymbol, coinbaseResp.PriceBook.Time.Unix()),
		},
	}
	
	// Parse bids
	for _, bid := range coinbaseResp.PriceBook.Bids {
		price, _ := strconv.ParseFloat(bid.Price, 64)
		size, _ := strconv.ParseFloat(bid.Size, 64)
		orderBook.Bids = append(orderBook.Bids, PriceLevel{
			Price: price,
			Size:  size,
		})
	}
	
	// Parse asks
	for _, ask := range coinbaseResp.PriceBook.Asks {
		price, _ := strconv.ParseFloat(ask.Price, 64)
		size, _ := strconv.ParseFloat(ask.Size, 64)
		orderBook.Asks = append(orderBook.Asks, PriceLevel{
			Price: price,
			Size:  size,
		})
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

// fetchTrades retrieves recent trades from Coinbase API
func (cp *CoinbaseProvider) fetchTrades(ctx context.Context, coinbaseSymbol, originalSymbol string, limit int) ([]TradeData, error) {
	url := fmt.Sprintf("%s/api/v3/brokerage/products/%s/ticker?limit=%d", cp.config.BaseURL, coinbaseSymbol, limit)
	
	start := time.Now()
	resp, err := cp.makeRequest(ctx, url)
	if err != nil {
		cp.recordRequestResult(false, time.Since(start))
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		cp.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: cp.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("failed to read response: %v", err),
		}
	}
	
	var coinbaseResp CoinbaseTradeResponse
	if err := json.Unmarshal(body, &coinbaseResp); err != nil {
		cp.recordRequestResult(false, time.Since(start))
		return nil, &ProviderError{
			Provider: cp.config.Name,
			Code:     ErrCodeInvalidData,
			Message:  fmt.Sprintf("failed to parse trades: %v", err),
		}
	}
	
	cp.recordRequestResult(true, time.Since(start))
	
	// Convert to standard format
	trades := make([]TradeData, 0, len(coinbaseResp.Trades))
	for _, trade := range coinbaseResp.Trades {
		price, _ := strconv.ParseFloat(trade.Price, 64)
		size, _ := strconv.ParseFloat(trade.Size, 64)
		
		trades = append(trades, TradeData{
			TradeID:   trade.TradeID,
			Venue:     cp.config.Venue,
			Symbol:    originalSymbol,
			Price:     price,
			Size:      size,
			Side:      trade.Side,
			Timestamp: trade.Time,
		})
	}
	
	return trades, nil
}

// makeRequest makes an HTTP request with proper error handling
func (cp *CoinbaseProvider) makeRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, &ProviderError{
			Provider: cp.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("failed to create request: %v", err),
		}
	}
	
	// Add user agent for Coinbase API
	req.Header.Set("User-Agent", "CryptoRun/1.0")
	
	resp, err := cp.client.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Provider: cp.config.Name,
			Code:     ErrCodeNetworkError,
			Message:  fmt.Sprintf("request failed: %v", err),
		}
	}
	
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, &ProviderError{
			Provider: cp.config.Name,
			Code:     ErrCodeAPIError,
			Message:  fmt.Sprintf("API returned status %d", resp.StatusCode),
		}
	}
	
	return resp, nil
}

// testConnectivity tests the connection to Coinbase
func (cp *CoinbaseProvider) testConnectivity(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v3/brokerage/time", cp.config.BaseURL)
	
	resp, err := cp.makeRequest(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	return nil
}

// convertSymbol converts standard symbol format to Coinbase format
// BTC-USD -> BTC-USD (no change needed for most pairs)
func (cp *CoinbaseProvider) convertSymbol(symbol string) string {
	// Coinbase uses the same format as our standard (BTC-USD)
	return symbol
}

// recordRequestResult updates health metrics
func (cp *CoinbaseProvider) recordRequestResult(success bool, duration time.Duration) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	cp.totalRequests++
	cp.lastHealthCheck = time.Now()
	
	if success {
		cp.successfulRequests++
		cp.consecutiveFailures = 0
	} else {
		cp.consecutiveFailures++
	}
	
	// Update average response time (simple moving average)
	if cp.totalRequests == 1 {
		cp.avgResponseTime = duration
	} else {
		// Weighted average with more weight on recent requests
		weight := 0.1
		cp.avgResponseTime = time.Duration(float64(cp.avgResponseTime)*(1-weight) + float64(duration)*weight)
	}
}

// emitMetric sends a metric if callback is configured
func (cp *CoinbaseProvider) emitMetric(metric string, value interface{}) {
	if cp.metricsCallback != nil {
		cp.metricsCallback(metric, value)
	}
}