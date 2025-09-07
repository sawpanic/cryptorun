package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"cryptorun/internal/providers/guards"
)

// BinanceAdapter wraps Binance API calls with provider guards
type BinanceAdapter struct {
	guard      *guards.ProviderGuard
	baseURL    string
	httpClient *http.Client
}

// NewBinanceAdapter creates a new Binance adapter with guards
func NewBinanceAdapter(config guards.ProviderConfig) *BinanceAdapter {
	if config.Name == "" {
		config.Name = "binance"
	}

	return &BinanceAdapter{
		guard:   guards.NewProviderGuard(config),
		baseURL: "https://api.binance.com/api/v3",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetTicker24h fetches 24h ticker statistics for a symbol
func (b *BinanceAdapter) GetTicker24h(ctx context.Context, symbol string) (*Ticker24hResponse, error) {
	params := ""
	if symbol != "" {
		params = fmt.Sprintf("symbol=%s", symbol)
	}

	url := fmt.Sprintf("%s/ticker/24hr", b.baseURL)
	if params != "" {
		url += "?" + params
	}

	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      url,
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: b.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/ticker/24hr?%s", params), nil, nil),
	}

	resp, err := b.guard.Execute(ctx, req, b.httpFetcher)
	if err != nil {
		return nil, err
	}

	var tickerResp Ticker24hResponse
	if symbol != "" {
		// Single symbol response
		var singleTicker Ticker24h
		if err := json.Unmarshal(resp.Data, &singleTicker); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ticker response: %w", err)
		}
		tickerResp.Data = []Ticker24h{singleTicker}
	} else {
		// All symbols response
		if err := json.Unmarshal(resp.Data, &tickerResp.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ticker response: %w", err)
		}
	}

	tickerResp.Cached = resp.Cached
	tickerResp.Age = resp.Age

	return &tickerResp, nil
}

// GetOrderBook fetches order book data for a symbol
func (b *BinanceAdapter) GetOrderBook(ctx context.Context, symbol string, limit int) (*OrderBookResponse, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	params := fmt.Sprintf("symbol=%s&limit=%d", symbol, limit)

	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/depth?%s", b.baseURL, params),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: b.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/depth?%s", params), nil, nil),
	}

	resp, err := b.guard.Execute(ctx, req, b.httpFetcher)
	if err != nil {
		return nil, err
	}

	var orderBook OrderBookResponse
	if err := json.Unmarshal(resp.Data, &orderBook); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order book response: %w", err)
	}

	orderBook.Cached = resp.Cached
	orderBook.Age = resp.Age

	return &orderBook, nil
}

// GetKlines fetches candlestick/kline data for a symbol
func (b *BinanceAdapter) GetKlines(ctx context.Context, symbol, interval string, limit int) (*KlinesResponse, error) {
	if limit <= 0 {
		limit = 500 // Default limit
	}

	params := fmt.Sprintf("symbol=%s&interval=%s&limit=%d", symbol, interval, limit)

	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/klines?%s", b.baseURL, params),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: b.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/klines?%s", params), nil, nil),
	}

	resp, err := b.guard.Execute(ctx, req, b.httpFetcher)
	if err != nil {
		return nil, err
	}

	var rawKlines [][]interface{}
	if err := json.Unmarshal(resp.Data, &rawKlines); err != nil {
		return nil, fmt.Errorf("failed to unmarshal klines response: %w", err)
	}

	// Convert raw klines to structured format
	klines := make([]Kline, len(rawKlines))
	for i, raw := range rawKlines {
		if len(raw) < 12 {
			continue
		}

		klines[i] = Kline{
			OpenTime:            int64(raw[0].(float64)),
			Open:                parseFloat(raw[1]),
			High:                parseFloat(raw[2]),
			Low:                 parseFloat(raw[3]),
			Close:               parseFloat(raw[4]),
			Volume:              parseFloat(raw[5]),
			CloseTime:           int64(raw[6].(float64)),
			BaseVolume:          parseFloat(raw[7]),
			TradeCount:          int64(raw[8].(float64)),
			TakerBuyBaseVolume:  parseFloat(raw[9]),
			TakerBuyQuoteVolume: parseFloat(raw[10]),
		}
	}

	return &KlinesResponse{
		Data:   klines,
		Cached: resp.Cached,
		Age:    resp.Age,
	}, nil
}

// GetExchangeInfo fetches exchange trading rules and symbol information
func (b *BinanceAdapter) GetExchangeInfo(ctx context.Context) (*ExchangeInfoResponse, error) {
	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/exchangeInfo", b.baseURL),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: b.guard.Cache().GenerateCacheKey("GET", "/exchangeInfo", nil, nil),
	}

	resp, err := b.guard.Execute(ctx, req, b.httpFetcher)
	if err != nil {
		return nil, err
	}

	var exchangeInfo ExchangeInfoResponse
	if err := json.Unmarshal(resp.Data, &exchangeInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal exchange info response: %w", err)
	}

	exchangeInfo.Cached = resp.Cached
	exchangeInfo.Age = resp.Age

	return &exchangeInfo, nil
}

// httpFetcher performs the actual HTTP request
func (b *BinanceAdapter) httpFetcher(ctx context.Context, req guards.GuardedRequest) (*guards.GuardedResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Add point-in-time headers if available
	b.guard.Cache().AddPITHeaders(req.CacheKey, req.Headers)
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := b.httpClient.Do(httpReq)
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

// Health returns the health status of the Binance provider
func (b *BinanceAdapter) Health() guards.ProviderHealth {
	return b.guard.Health()
}

// Response types for Binance API

type Ticker24hResponse struct {
	Data   []Ticker24h   `json:"data"`
	Cached bool          `json:"cached"`
	Age    time.Duration `json:"age"`
}

type Ticker24h struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	WeightedAvgPrice   string `json:"weightedAvgPrice"`
	PrevClosePrice     string `json:"prevClosePrice"`
	LastPrice          string `json:"lastPrice"`
	LastQty            string `json:"lastQty"`
	BidPrice           string `json:"bidPrice"`
	BidQty             string `json:"bidQty"`
	AskPrice           string `json:"askPrice"`
	AskQty             string `json:"askQty"`
	OpenPrice          string `json:"openPrice"`
	HighPrice          string `json:"highPrice"`
	LowPrice           string `json:"lowPrice"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
	OpenTime           int64  `json:"openTime"`
	CloseTime          int64  `json:"closeTime"`
	Count              int64  `json:"count"`
}

type OrderBookResponse struct {
	LastUpdateID int64         `json:"lastUpdateId"`
	Bids         [][2]string   `json:"bids"`
	Asks         [][2]string   `json:"asks"`
	Cached       bool          `json:"cached"`
	Age          time.Duration `json:"age"`
}

type KlinesResponse struct {
	Data   []Kline       `json:"data"`
	Cached bool          `json:"cached"`
	Age    time.Duration `json:"age"`
}

type Kline struct {
	OpenTime            int64   `json:"open_time"`
	Open                float64 `json:"open"`
	High                float64 `json:"high"`
	Low                 float64 `json:"low"`
	Close               float64 `json:"close"`
	Volume              float64 `json:"volume"`
	CloseTime           int64   `json:"close_time"`
	BaseVolume          float64 `json:"base_volume"`
	TradeCount          int64   `json:"trade_count"`
	TakerBuyBaseVolume  float64 `json:"taker_buy_base_volume"`
	TakerBuyQuoteVolume float64 `json:"taker_buy_quote_volume"`
}

type ExchangeInfoResponse struct {
	Timezone   string        `json:"timezone"`
	ServerTime int64         `json:"serverTime"`
	Symbols    []Symbol      `json:"symbols"`
	Cached     bool          `json:"cached"`
	Age        time.Duration `json:"age"`
}

type Symbol struct {
	Symbol     string   `json:"symbol"`
	Status     string   `json:"status"`
	BaseAsset  string   `json:"baseAsset"`
	QuoteAsset string   `json:"quoteAsset"`
	Filters    []Filter `json:"filters"`
}

type Filter struct {
	FilterType string `json:"filterType"`
	MinPrice   string `json:"minPrice,omitempty"`
	MaxPrice   string `json:"maxPrice,omitempty"`
	TickSize   string `json:"tickSize,omitempty"`
	MinQty     string `json:"minQty,omitempty"`
	MaxQty     string `json:"maxQty,omitempty"`
	StepSize   string `json:"stepSize,omitempty"`
}

// parseFloat safely converts interface{} to float64
func parseFloat(v interface{}) float64 {
	switch val := v.(type) {
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	}
	return 0
}
