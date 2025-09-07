package kraken

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client provides Kraken API access with rate limiting and circuit breaking
type Client struct {
	httpClient   *http.Client
	baseURL      string
	wsURL        string
	rateLimiter  *RateLimiter
	metrics      MetricsCallback
	mu           sync.RWMutex
	wsConn       *websocket.Conn
	wsSubscriptions map[string]bool
}

// Config holds Kraken client configuration
type Config struct {
	BaseURL          string        `json:"base_url"`
	WebSocketURL     string        `json:"websocket_url"`
	RequestTimeout   time.Duration `json:"request_timeout"`
	RateLimitRPS     float64       `json:"rate_limit_rps"`
	MaxRetries       int           `json:"max_retries"`
	RetryBackoff     time.Duration `json:"retry_backoff"`
	UserAgent        string        `json:"user_agent"`
	EnableMetrics    bool          `json:"enable_metrics"`
}

// MetricsCallback is called when metrics are collected
type MetricsCallback func(metric string, value float64, tags map[string]string)

// NewClient creates a new Kraken API client with exchange-native validation
func NewClient(config Config) *Client {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.kraken.com"
	}
	if config.WebSocketURL == "" {
		config.WebSocketURL = "wss://ws.kraken.com"
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 10 * time.Second
	}
	if config.RateLimitRPS == 0 {
		config.RateLimitRPS = 1.0 // Kraken free tier: 1 RPS
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryBackoff == 0 {
		config.RetryBackoff = 1 * time.Second
	}
	if config.UserAgent == "" {
		config.UserAgent = "CryptoRun/3.2.1 (Exchange-Native)"
	}

	client := &Client{
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
		baseURL:         config.BaseURL,
		wsURL:           config.WebSocketURL,
		rateLimiter:     NewRateLimiter(config.RateLimitRPS),
		wsSubscriptions: make(map[string]bool),
	}

	return client
}

// SetMetricsCallback sets the metrics collection callback
func (c *Client) SetMetricsCallback(callback MetricsCallback) {
	c.metrics = callback
}

// Health checks Kraken API connectivity and rate limit status
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	start := time.Now()
	defer func() {
		if c.metrics != nil {
			c.metrics("kraken_health_check_duration_ms", float64(time.Since(start).Milliseconds()), 
				map[string]string{"provider": "kraken"})
		}
	}()

	// Test REST API connectivity
	resp, err := c.GetServerTime(ctx)
	if err != nil {
		if c.metrics != nil {
			c.metrics("kraken_health_check_failures_total", 1, 
				map[string]string{"provider": "kraken", "type": "rest"})
		}
		return &HealthStatus{
			Healthy: false,
			Errors:  []string{fmt.Sprintf("REST API error: %v", err)},
			Metrics: HealthMetrics{
				RateLimitRemaining: c.rateLimiter.Remaining(),
				LastRequestTime:    c.rateLimiter.LastRequest(),
			},
		}, nil
	}

	// Check WebSocket connection
	wsHealthy := c.isWebSocketHealthy()
	
	status := &HealthStatus{
		Healthy: true,
		Status:  "operational",
		Metrics: HealthMetrics{
			ServerTime:         resp.UnixTime,
			RateLimitRemaining: c.rateLimiter.Remaining(),
			LastRequestTime:    c.rateLimiter.LastRequest(),
			WebSocketHealthy:   wsHealthy,
		},
	}

	if !wsHealthy {
		status.Errors = append(status.Errors, "WebSocket connection unhealthy")
	}

	return status, nil
}

// GetServerTime retrieves Kraken server time for synchronization
func (c *Client) GetServerTime(ctx context.Context) (*ServerTimeResponse, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	url := fmt.Sprintf("%s/0/public/Time", c.baseURL)
	
	start := time.Now()
	resp, err := c.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		if c.metrics != nil {
			c.metrics("kraken_requests_total", 1, 
				map[string]string{"provider": "kraken", "endpoint": "Time", "status": "error"})
		}
		return nil, err
	}
	defer resp.Body.Close()

	if c.metrics != nil {
		c.metrics("kraken_requests_total", 1, 
			map[string]string{"provider": "kraken", "endpoint": "Time", "status": "success"})
		c.metrics("kraken_request_duration_ms", float64(time.Since(start).Milliseconds()),
			map[string]string{"provider": "kraken", "endpoint": "Time"})
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResp KrakenResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(apiResp.Error) > 0 {
		return nil, fmt.Errorf("API error: %v", apiResp.Error)
	}

	var timeResp ServerTimeResponse
	if err := json.Unmarshal(apiResp.Result, &timeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal time response: %w", err)
	}

	return &timeResp, nil
}

// GetTicker retrieves ticker information for USD pairs only
func (c *Client) GetTicker(ctx context.Context, pairs []string) (map[string]*TickerInfo, error) {
	// Validate USD-only pairs (exchange-native requirement)
	for _, pair := range pairs {
		if !isUSDPair(pair) {
			return nil, fmt.Errorf("non-USD pair not allowed: %s - USD pairs only", pair)
		}
	}

	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	pairStr := strings.Join(pairs, ",")
	url := fmt.Sprintf("%s/0/public/Ticker?pair=%s", c.baseURL, url.QueryEscape(pairStr))

	start := time.Now()
	resp, err := c.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		if c.metrics != nil {
			c.metrics("kraken_requests_total", 1,
				map[string]string{"provider": "kraken", "endpoint": "Ticker", "status": "error"})
		}
		return nil, err
	}
	defer resp.Body.Close()

	if c.metrics != nil {
		c.metrics("kraken_requests_total", 1,
			map[string]string{"provider": "kraken", "endpoint": "Ticker", "status": "success"})
		c.metrics("kraken_request_duration_ms", float64(time.Since(start).Milliseconds()),
			map[string]string{"provider": "kraken", "endpoint": "Ticker"})
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResp KrakenResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(apiResp.Error) > 0 {
		return nil, fmt.Errorf("API error: %v", apiResp.Error)
	}

	var tickers map[string]*TickerInfo
	if err := json.Unmarshal(apiResp.Result, &tickers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ticker response: %w", err)
	}

	// Normalize pair names and validate data
	normalized := make(map[string]*TickerInfo)
	for pair, ticker := range tickers {
		normalizedPair := normalizePairName(pair)
		
		// Validate ticker data
		if ticker.Ask == nil || len(ticker.Ask) < 2 {
			continue // Skip invalid ticker data
		}
		if ticker.Bid == nil || len(ticker.Bid) < 2 {
			continue
		}
		
		normalized[normalizedPair] = ticker
	}

	return normalized, nil
}

// GetOrderBook retrieves L2 order book for exchange-native microstructure analysis
func (c *Client) GetOrderBook(ctx context.Context, pair string, count int) (*OrderBookResponse, error) {
	if !isUSDPair(pair) {
		return nil, fmt.Errorf("non-USD pair not allowed: %s - USD pairs only", pair)
	}

	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	params := url.Values{}
	params.Set("pair", pair)
	if count > 0 {
		params.Set("count", strconv.Itoa(count))
	}

	url := fmt.Sprintf("%s/0/public/Depth?%s", c.baseURL, params.Encode())

	start := time.Now()
	resp, err := c.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		if c.metrics != nil {
			c.metrics("kraken_requests_total", 1,
				map[string]string{"provider": "kraken", "endpoint": "Depth", "status": "error"})
		}
		return nil, err
	}
	defer resp.Body.Close()

	if c.metrics != nil {
		c.metrics("kraken_requests_total", 1,
			map[string]string{"provider": "kraken", "endpoint": "Depth", "status": "success"})
		c.metrics("kraken_request_duration_ms", float64(time.Since(start).Milliseconds()),
			map[string]string{"provider": "kraken", "endpoint": "Depth"})
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResp KrakenResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(apiResp.Error) > 0 {
		return nil, fmt.Errorf("API error: %v", apiResp.Error)
	}

	var books map[string]*OrderBookData
	if err := json.Unmarshal(apiResp.Result, &books); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order book response: %w", err)
	}

	// Find the order book for our pair
	for pairName, book := range books {
		normalizedPair := normalizePairName(pairName)
		if normalizedPair == normalizePairName(pair) {
			return &OrderBookResponse{
				Pair: normalizedPair,
				Data: book,
			}, nil
		}
	}

	return nil, fmt.Errorf("order book not found for pair: %s", pair)
}

// Helper methods

func (c *Client) makeRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "CryptoRun/3.2.1 (Exchange-Native)")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

func (c *Client) isWebSocketHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.wsConn == nil {
		return false
	}

	// Simple ping test
	err := c.wsConn.WriteMessage(websocket.PingMessage, nil)
	return err == nil
}

// isUSDPair validates that the pair is USD-denominated (exchange-native requirement)
func isUSDPair(pair string) bool {
	upperPair := strings.ToUpper(pair)
	return strings.HasSuffix(upperPair, "USD") || 
		   strings.HasSuffix(upperPair, "ZUSD") || // Kraken format
		   strings.Contains(upperPair, "USD/") ||
		   strings.Contains(upperPair, "/USD")
}

// normalizePairName converts Kraken pair names to standard format
func normalizePairName(pair string) string {
	// Convert XXBTZUSD -> BTC-USD, etc.
	upperPair := strings.ToUpper(pair)
	
	// Handle Kraken's specific naming conventions
	if strings.HasPrefix(upperPair, "XXBT") {
		upperPair = strings.Replace(upperPair, "XXBT", "BTC", 1)
	}
	if strings.HasPrefix(upperPair, "XETH") {
		upperPair = strings.Replace(upperPair, "XETH", "ETH", 1)
	}
	if strings.HasSuffix(upperPair, "ZUSD") {
		upperPair = strings.Replace(upperPair, "ZUSD", "USD", 1)
	}
	
	// Convert to standard format: BTC-USD
	if len(upperPair) >= 6 && strings.HasSuffix(upperPair, "USD") {
		base := upperPair[:len(upperPair)-3]
		return fmt.Sprintf("%s-USD", base)
	}
	
	return upperPair
}