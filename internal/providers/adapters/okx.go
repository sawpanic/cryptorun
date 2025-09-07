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

// OKXAdapter wraps OKX API calls with provider guards
type OKXAdapter struct {
	guard      *guards.ProviderGuard
	baseURL    string
	httpClient *http.Client
}

// NewOKXAdapter creates a new OKX adapter with guards
func NewOKXAdapter(config guards.ProviderConfig) *OKXAdapter {
	if config.Name == "" {
		config.Name = "okx"
	}

	return &OKXAdapter{
		guard:   guards.NewProviderGuard(config),
		baseURL: "https://www.okx.com/api/v5",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetTicker fetches ticker information for an instrument
func (o *OKXAdapter) GetTicker(ctx context.Context, instId string) (*OKXTickerResponse, error) {
	params := ""
	if instId != "" {
		params = fmt.Sprintf("instId=%s", instId)
	}

	url := fmt.Sprintf("%s/market/ticker", o.baseURL)
	if params != "" {
		url += "?" + params
	}

	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      url,
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: o.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/market/ticker?%s", params), nil, nil),
	}

	resp, err := o.guard.Execute(ctx, req, o.httpFetcher)
	if err != nil {
		return nil, err
	}

	var tickerResp OKXTickerResponse
	if err := json.Unmarshal(resp.Data, &tickerResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ticker response: %w", err)
	}

	tickerResp.Cached = resp.Cached
	tickerResp.Age = resp.Age

	return &tickerResp, nil
}

// GetOrderBook fetches order book data for an instrument
func (o *OKXAdapter) GetOrderBook(ctx context.Context, instId string, sz int) (*OKXOrderBookResponse, error) {
	if sz <= 0 {
		sz = 20 // Default size
	}

	params := fmt.Sprintf("instId=%s&sz=%d", instId, sz)

	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/market/books?%s", o.baseURL, params),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: o.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/market/books?%s", params), nil, nil),
	}

	resp, err := o.guard.Execute(ctx, req, o.httpFetcher)
	if err != nil {
		return nil, err
	}

	var orderBookResp OKXOrderBookResponse
	if err := json.Unmarshal(resp.Data, &orderBookResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order book response: %w", err)
	}

	orderBookResp.Cached = resp.Cached
	orderBookResp.Age = resp.Age

	return &orderBookResp, nil
}

// GetCandles fetches candlestick data for an instrument
func (o *OKXAdapter) GetCandles(ctx context.Context, instId, bar string, limit int) (*OKXCandlesResponse, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	params := fmt.Sprintf("instId=%s&bar=%s&limit=%d", instId, bar, limit)

	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/market/candles?%s", o.baseURL, params),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: o.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/market/candles?%s", params), nil, nil),
	}

	resp, err := o.guard.Execute(ctx, req, o.httpFetcher)
	if err != nil {
		return nil, err
	}

	var rawResp struct {
		Code string     `json:"code"`
		Msg  string     `json:"msg"`
		Data [][]string `json:"data"`
	}

	if err := json.Unmarshal(resp.Data, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal candles response: %w", err)
	}

	if rawResp.Code != "0" {
		return nil, fmt.Errorf("OKX API error: %s", rawResp.Msg)
	}

	// Convert string arrays to structured candles
	candles := make([]OKXCandle, len(rawResp.Data))
	for i, raw := range rawResp.Data {
		if len(raw) >= 6 {
			candles[i] = OKXCandle{
				Timestamp: raw[0],
				Open:      raw[1],
				High:      raw[2],
				Low:       raw[3],
				Close:     raw[4],
				Volume:    raw[5],
			}
			if len(raw) >= 7 {
				candles[i].VolumeCcy = raw[6]
			}
		}
	}

	return &OKXCandlesResponse{
		Code:   rawResp.Code,
		Msg:    rawResp.Msg,
		Data:   candles,
		Cached: resp.Cached,
		Age:    resp.Age,
	}, nil
}

// GetInstruments fetches available trading instruments
func (o *OKXAdapter) GetInstruments(ctx context.Context, instType string) (*OKXInstrumentsResponse, error) {
	params := ""
	if instType != "" {
		params = fmt.Sprintf("instType=%s", instType)
	}

	url := fmt.Sprintf("%s/public/instruments", o.baseURL)
	if params != "" {
		url += "?" + params
	}

	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      url,
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: o.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/public/instruments?%s", params), nil, nil),
	}

	resp, err := o.guard.Execute(ctx, req, o.httpFetcher)
	if err != nil {
		return nil, err
	}

	var instrumentsResp OKXInstrumentsResponse
	if err := json.Unmarshal(resp.Data, &instrumentsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instruments response: %w", err)
	}

	instrumentsResp.Cached = resp.Cached
	instrumentsResp.Age = resp.Age

	return &instrumentsResp, nil
}

// httpFetcher performs the actual HTTP request
func (o *OKXAdapter) httpFetcher(ctx context.Context, req guards.GuardedRequest) (*guards.GuardedResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Add point-in-time headers if available
	o.guard.Cache().AddPITHeaders(req.CacheKey, req.Headers)
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := o.httpClient.Do(httpReq)
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

// Health returns the health status of the OKX provider
func (o *OKXAdapter) Health() guards.ProviderHealth {
	return o.guard.Health()
}

// Response types for OKX API

type OKXTickerResponse struct {
	Code   string        `json:"code"`
	Msg    string        `json:"msg"`
	Data   []OKXTicker   `json:"data"`
	Cached bool          `json:"cached"`
	Age    time.Duration `json:"age"`
}

type OKXTicker struct {
	InstType  string `json:"instType"`
	InstId    string `json:"instId"`
	Last      string `json:"last"`
	LastSz    string `json:"lastSz"`
	AskPx     string `json:"askPx"`
	AskSz     string `json:"askSz"`
	BidPx     string `json:"bidPx"`
	BidSz     string `json:"bidSz"`
	Open24h   string `json:"open24h"`
	High24h   string `json:"high24h"`
	Low24h    string `json:"low24h"`
	Vol24h    string `json:"vol24h"`
	VolCcy24h string `json:"volCcy24h"`
	SodUtc0   string `json:"sodUtc0"`
	SodUtc8   string `json:"sodUtc8"`
	Timestamp string `json:"ts"`
}

type OKXOrderBookResponse struct {
	Code   string         `json:"code"`
	Msg    string         `json:"msg"`
	Data   []OKXOrderBook `json:"data"`
	Cached bool           `json:"cached"`
	Age    time.Duration  `json:"age"`
}

type OKXOrderBook struct {
	Asks      [][]string `json:"asks"`
	Bids      [][]string `json:"bids"`
	Timestamp string     `json:"ts"`
}

type OKXCandlesResponse struct {
	Code   string        `json:"code"`
	Msg    string        `json:"msg"`
	Data   []OKXCandle   `json:"data"`
	Cached bool          `json:"cached"`
	Age    time.Duration `json:"age"`
}

type OKXCandle struct {
	Timestamp string `json:"timestamp"`
	Open      string `json:"open"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Close     string `json:"close"`
	Volume    string `json:"volume"`
	VolumeCcy string `json:"volume_ccy,omitempty"`
}

type OKXInstrumentsResponse struct {
	Code   string          `json:"code"`
	Msg    string          `json:"msg"`
	Data   []OKXInstrument `json:"data"`
	Cached bool            `json:"cached"`
	Age    time.Duration   `json:"age"`
}

type OKXInstrument struct {
	InstType  string `json:"instType"`
	InstId    string `json:"instId"`
	Uly       string `json:"uly"`
	BaseCcy   string `json:"baseCcy"`
	QuoteCcy  string `json:"quoteCcy"`
	SettleCcy string `json:"settleCcy"`
	CtVal     string `json:"ctVal"`
	CtMult    string `json:"ctMult"`
	CtValCcy  string `json:"ctValCcy"`
	OptType   string `json:"optType"`
	Strike    string `json:"stk"`
	ListTime  string `json:"listTime"`
	ExpTime   string `json:"expTime"`
	TickSz    string `json:"tickSz"`
	LotSz     string `json:"lotSz"`
	MinSz     string `json:"minSz"`
	State     string `json:"state"`
}
