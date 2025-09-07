package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sawpanic/cryptorun/internal/providers/guards"
)

// CoinbaseAdapter wraps Coinbase API calls with provider guards
type CoinbaseAdapter struct {
	guard      *guards.ProviderGuard
	baseURL    string
	httpClient *http.Client
}

// NewCoinbaseAdapter creates a new Coinbase adapter with guards
func NewCoinbaseAdapter(config guards.ProviderConfig) *CoinbaseAdapter {
	if config.Name == "" {
		config.Name = "coinbase"
	}

	return &CoinbaseAdapter{
		guard:   guards.NewProviderGuard(config),
		baseURL: "https://api.exchange.coinbase.com",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetTicker fetches 24hr stats for a product
func (c *CoinbaseAdapter) GetTicker(ctx context.Context, productId string) (*CoinbaseTickerResponse, error) {
	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/products/%s/ticker", c.baseURL, productId),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: c.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/products/%s/ticker", productId), nil, nil),
	}

	resp, err := c.guard.Execute(ctx, req, c.httpFetcher)
	if err != nil {
		return nil, err
	}

	var tickerResp CoinbaseTickerResponse
	if err := json.Unmarshal(resp.Data, &tickerResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ticker response: %w", err)
	}

	tickerResp.Cached = resp.Cached
	tickerResp.Age = resp.Age

	return &tickerResp, nil
}

// Get24hrStats fetches 24hr statistics for a product
func (c *CoinbaseAdapter) Get24hrStats(ctx context.Context, productId string) (*CoinbaseStatsResponse, error) {
	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/products/%s/stats", c.baseURL, productId),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: c.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/products/%s/stats", productId), nil, nil),
	}

	resp, err := c.guard.Execute(ctx, req, c.httpFetcher)
	if err != nil {
		return nil, err
	}

	var statsResp CoinbaseStatsResponse
	if err := json.Unmarshal(resp.Data, &statsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stats response: %w", err)
	}

	statsResp.Cached = resp.Cached
	statsResp.Age = resp.Age

	return &statsResp, nil
}

// GetOrderBook fetches order book (level 2) for a product
func (c *CoinbaseAdapter) GetOrderBook(ctx context.Context, productId string, level int) (*CoinbaseOrderBookResponse, error) {
	if level <= 0 {
		level = 2 // Default level 2
	}

	params := fmt.Sprintf("level=%d", level)

	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/products/%s/book?%s", c.baseURL, productId, params),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: c.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/products/%s/book?%s", productId, params), nil, nil),
	}

	resp, err := c.guard.Execute(ctx, req, c.httpFetcher)
	if err != nil {
		return nil, err
	}

	var orderBookResp CoinbaseOrderBookResponse
	if err := json.Unmarshal(resp.Data, &orderBookResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order book response: %w", err)
	}

	orderBookResp.Cached = resp.Cached
	orderBookResp.Age = resp.Age

	return &orderBookResp, nil
}

// GetCandles fetches historic rates (candles) for a product
func (c *CoinbaseAdapter) GetCandles(ctx context.Context, productId string, granularity int, start, end time.Time) (*CoinbaseCandlesResponse, error) {
	params := fmt.Sprintf("granularity=%d", granularity)
	if !start.IsZero() {
		params += fmt.Sprintf("&start=%s", start.Format(time.RFC3339))
	}
	if !end.IsZero() {
		params += fmt.Sprintf("&end=%s", end.Format(time.RFC3339))
	}

	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/products/%s/candles?%s", c.baseURL, productId, params),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: c.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/products/%s/candles?%s", productId, params), nil, nil),
	}

	resp, err := c.guard.Execute(ctx, req, c.httpFetcher)
	if err != nil {
		return nil, err
	}

	var rawCandles [][]float64
	if err := json.Unmarshal(resp.Data, &rawCandles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal candles response: %w", err)
	}

	// Convert raw candles to structured format
	candles := make([]CoinbaseCandle, len(rawCandles))
	for i, raw := range rawCandles {
		if len(raw) >= 6 {
			candles[i] = CoinbaseCandle{
				Time:   int64(raw[0]),
				Low:    raw[1],
				High:   raw[2],
				Open:   raw[3],
				Close:  raw[4],
				Volume: raw[5],
			}
		}
	}

	return &CoinbaseCandlesResponse{
		Data:   candles,
		Cached: resp.Cached,
		Age:    resp.Age,
	}, nil
}

// GetProducts fetches available trading pairs
func (c *CoinbaseAdapter) GetProducts(ctx context.Context) (*CoinbaseProductsResponse, error) {
	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/products", c.baseURL),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: c.guard.Cache().GenerateCacheKey("GET", "/products", nil, nil),
	}

	resp, err := c.guard.Execute(ctx, req, c.httpFetcher)
	if err != nil {
		return nil, err
	}

	var productsResp CoinbaseProductsResponse
	if err := json.Unmarshal(resp.Data, &productsResp.Data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal products response: %w", err)
	}

	productsResp.Cached = resp.Cached
	productsResp.Age = resp.Age

	return &productsResp, nil
}

// httpFetcher performs the actual HTTP request
func (c *CoinbaseAdapter) httpFetcher(ctx context.Context, req guards.GuardedRequest) (*guards.GuardedResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Add point-in-time headers if available
	c.guard.Cache().AddPITHeaders(req.CacheKey, req.Headers)
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &guards.GuardedResponse{
		Data:       body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Cached:     false,
	}, nil
}

// Health returns the health status of the Coinbase provider
func (c *CoinbaseAdapter) Health() guards.ProviderHealth {
	return c.guard.Health()
}

// Response types for Coinbase API

type CoinbaseTickerResponse struct {
	TradeId int64         `json:"trade_id"`
	Price   string        `json:"price"`
	Size    string        `json:"size"`
	Time    string        `json:"time"`
	Bid     string        `json:"bid"`
	Ask     string        `json:"ask"`
	Volume  string        `json:"volume"`
	Cached  bool          `json:"cached"`
	Age     time.Duration `json:"age"`
}

type CoinbaseStatsResponse struct {
	Open        string        `json:"open"`
	High        string        `json:"high"`
	Low         string        `json:"low"`
	Volume      string        `json:"volume"`
	Last        string        `json:"last"`
	Volume30Day string        `json:"volume_30day"`
	Cached      bool          `json:"cached"`
	Age         time.Duration `json:"age"`
}

type CoinbaseOrderBookResponse struct {
	Sequence int64         `json:"sequence"`
	Bids     [][]string    `json:"bids"`
	Asks     [][]string    `json:"asks"`
	Cached   bool          `json:"cached"`
	Age      time.Duration `json:"age"`
}

type CoinbaseCandlesResponse struct {
	Data   []CoinbaseCandle `json:"data"`
	Cached bool             `json:"cached"`
	Age    time.Duration    `json:"age"`
}

type CoinbaseCandle struct {
	Time   int64   `json:"time"`
	Low    float64 `json:"low"`
	High   float64 `json:"high"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type CoinbaseProductsResponse struct {
	Data   []CoinbaseProduct `json:"data"`
	Cached bool              `json:"cached"`
	Age    time.Duration     `json:"age"`
}

type CoinbaseProduct struct {
	Id              string `json:"id"`
	BaseCurrency    string `json:"base_currency"`
	QuoteCurrency   string `json:"quote_currency"`
	BaseMinSize     string `json:"base_min_size"`
	BaseMaxSize     string `json:"base_max_size"`
	QuoteIncrement  string `json:"quote_increment"`
	BaseIncrement   string `json:"base_increment"`
	DisplayName     string `json:"display_name"`
	MinMarketFunds  string `json:"min_market_funds"`
	MaxMarketFunds  string `json:"max_market_funds"`
	MarginEnabled   bool   `json:"margin_enabled"`
	PostOnly        bool   `json:"post_only"`
	LimitOnly       bool   `json:"limit_only"`
	CancelOnly      bool   `json:"cancel_only"`
	TradingDisabled bool   `json:"trading_disabled"`
	Status          string `json:"status"`
	StatusMessage   string `json:"status_message"`
}
