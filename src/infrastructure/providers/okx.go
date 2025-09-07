package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// OKXProvider implements the Provider interface for OKX
type OKXProvider struct {
	name        string
	baseURL     string
	client      *http.Client
	rateLimiter *RateLimiter
}

// NewOKXProvider creates a new OKX provider with free/keyless endpoints
func NewOKXProvider() *OKXProvider {
	return &OKXProvider{
		name:    "okx",
		baseURL: "https://www.okx.com",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: NewRateLimiter(60, 10), // 60 requests per 2 seconds, 10 RPS
	}
}

func (o *OKXProvider) Name() string {
	return o.name
}

func (o *OKXProvider) HasCapability(cap Capability) bool {
	switch cap {
	case CapabilityFunding, CapabilitySpotTrades, CapabilityOrderBookL2, CapabilityKlineData:
		return true
	case CapabilitySupplyReserves, CapabilityWhaleDetection, CapabilityCVD:
		return false // Not available via free APIs
	}
	return false
}

func (o *OKXProvider) Probe(ctx context.Context) (*ProbeResult, error) {
	start := time.Now()
	
	// Use server time endpoint as a lightweight health check
	endpoint := "/api/v5/public/time"
	url := o.baseURL + endpoint
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &ProbeResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}, nil
	}
	
	resp, err := o.client.Do(req)
	if err != nil {
		return &ProbeResult{
			Success:   false,
			Error:     err.Error(),
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		}, nil
	}
	defer resp.Body.Close()
	
	success := resp.StatusCode == http.StatusOK
	errorMsg := ""
	if !success {
		errorMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	
	return &ProbeResult{
		Success:   success,
		Error:     errorMsg,
		LatencyMs: int(time.Since(start).Milliseconds()),
		Timestamp: time.Now(),
	}, nil
}

func (o *OKXProvider) GetFundingHistory(ctx context.Context, req *FundingRequest) (*FundingResponse, error) {
	start := time.Now()
	
	if !o.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", o.name)
	}
	
	// Convert symbol to OKX format (e.g., BTCUSDT -> BTC-USDT-SWAP)
	instId := o.convertSymbolToOKX(req.Symbol)
	
	endpoint := "/api/v5/public/funding-history"
	params := url.Values{}
	params.Set("instId", instId)
	params.Set("limit", strconv.Itoa(req.Limit))
	
	fullURL := fmt.Sprintf("%s%s?%s", o.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var result struct {
		Code string `json:"code"`
		Data []struct {
			InstId      string `json:"instId"`
			FundingRate string `json:"fundingRate"`
			FundingTime string `json:"fundingTime"`
			MarkPx      string `json:"markPx"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	if result.Code != "0" {
		return nil, fmt.Errorf("OKX API error: code %s", result.Code)
	}
	
	rates := make([]FundingRate, len(result.Data))
	for i, r := range result.Data {
		rate, _ := strconv.ParseFloat(r.FundingRate, 64)
		markPrice, _ := strconv.ParseFloat(r.MarkPx, 64)
		timestamp, _ := strconv.ParseInt(r.FundingTime, 10, 64)
		
		rates[i] = FundingRate{
			Symbol:    o.convertSymbolFromOKX(r.InstId),
			Rate:      rate,
			Timestamp: time.Unix(timestamp/1000, 0),
			MarkPrice: markPrice,
		}
	}
	
	return &FundingResponse{
		Data: rates,
		Provenance: Provenance{
			Venue:     o.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

func (o *OKXProvider) GetSpotTrades(ctx context.Context, req *SpotTradesRequest) (*SpotTradesResponse, error) {
	start := time.Now()
	
	if !o.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", o.name)
	}
	
	// Convert symbol to OKX format (e.g., BTCUSDT -> BTC-USDT)
	instId := o.convertSpotSymbolToOKX(req.Symbol)
	
	endpoint := "/api/v5/market/trades"
	params := url.Values{}
	params.Set("instId", instId)
	params.Set("limit", strconv.Itoa(req.Limit))
	
	fullURL := fmt.Sprintf("%s%s?%s", o.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var result struct {
		Code string `json:"code"`
		Data []struct {
			InstId  string `json:"instId"`
			TradeId string `json:"tradeId"`
			Px      string `json:"px"`
			Sz      string `json:"sz"`
			Side    string `json:"side"`
			Ts      string `json:"ts"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	if result.Code != "0" {
		return nil, fmt.Errorf("OKX API error: code %s", result.Code)
	}
	
	trades := make([]SpotTrade, len(result.Data))
	for i, t := range result.Data {
		price, _ := strconv.ParseFloat(t.Px, 64)
		volume, _ := strconv.ParseFloat(t.Sz, 64)
		timestamp, _ := strconv.ParseInt(t.Ts, 10, 64)
		
		trades[i] = SpotTrade{
			Symbol:    req.Symbol,
			Price:     price,
			Volume:    volume,
			Side:      t.Side,
			Timestamp: time.Unix(timestamp/1000, 0),
			TradeID:   t.TradeId,
		}
	}
	
	return &SpotTradesResponse{
		Data: trades,
		Provenance: Provenance{
			Venue:     o.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

func (o *OKXProvider) GetOrderBookL2(ctx context.Context, req *OrderBookRequest) (*OrderBookResponse, error) {
	start := time.Now()
	
	if !o.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", o.name)
	}
	
	instId := o.convertSpotSymbolToOKX(req.Symbol)
	
	endpoint := "/api/v5/market/books"
	params := url.Values{}
	params.Set("instId", instId)
	params.Set("sz", strconv.Itoa(req.Limit))
	
	fullURL := fmt.Sprintf("%s%s?%s", o.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var result struct {
		Code string `json:"code"`
		Data []struct {
			Asks [][]string `json:"asks"`
			Bids [][]string `json:"bids"`
			Ts   string     `json:"ts"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	if result.Code != "0" || len(result.Data) == 0 {
		return nil, fmt.Errorf("OKX API error or no data")
	}
	
	data := result.Data[0]
	
	bids := make([]OrderBookEntry, len(data.Bids))
	for i, bid := range data.Bids {
		if len(bid) >= 2 {
			price, _ := strconv.ParseFloat(bid[0], 64)
			size, _ := strconv.ParseFloat(bid[1], 64)
			bids[i] = OrderBookEntry{Price: price, Size: size}
		}
	}
	
	asks := make([]OrderBookEntry, len(data.Asks))
	for i, ask := range data.Asks {
		if len(ask) >= 2 {
			price, _ := strconv.ParseFloat(ask[0], 64)
			size, _ := strconv.ParseFloat(ask[1], 64)
			asks[i] = OrderBookEntry{Price: price, Size: size}
		}
	}
	
	timestamp := time.Now()
	if ts, err := strconv.ParseInt(data.Ts, 10, 64); err == nil {
		timestamp = time.Unix(ts/1000, 0)
	}
	
	return &OrderBookResponse{
		Data: &OrderBookL2{
			Symbol:    req.Symbol,
			Bids:      bids,
			Asks:      asks,
			Timestamp: timestamp,
		},
		Provenance: Provenance{
			Venue:     o.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

func (o *OKXProvider) GetKlineData(ctx context.Context, req *KlineRequest) (*KlineResponse, error) {
	start := time.Now()
	
	if !o.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", o.name)
	}
	
	instId := o.convertSpotSymbolToOKX(req.Symbol)
	
	endpoint := "/api/v5/market/candles"
	params := url.Values{}
	params.Set("instId", instId)
	params.Set("bar", o.convertIntervalToOKX(req.Interval))
	params.Set("limit", strconv.Itoa(req.Limit))
	
	fullURL := fmt.Sprintf("%s%s?%s", o.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var result struct {
		Code string     `json:"code"`
		Data [][]string `json:"data"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	if result.Code != "0" {
		return nil, fmt.Errorf("OKX API error: code %s", result.Code)
	}
	
	klines := make([]Kline, len(result.Data))
	for i, k := range result.Data {
		if len(k) >= 7 {
			// OKX candlestick format: [ts, o, h, l, c, vol, volCcy, ...]
			timestamp, _ := strconv.ParseInt(k[0], 10, 64)
			open, _ := strconv.ParseFloat(k[1], 64)
			high, _ := strconv.ParseFloat(k[2], 64)
			low, _ := strconv.ParseFloat(k[3], 64)
			closePrice, _ := strconv.ParseFloat(k[4], 64)
			volume, _ := strconv.ParseFloat(k[5], 64)
			quoteVolume, _ := strconv.ParseFloat(k[6], 64)
			
			openTime := time.Unix(timestamp/1000, 0)
			closeTime := openTime.Add(o.getIntervalDuration(req.Interval))
			
			klines[i] = Kline{
				Symbol:      req.Symbol,
				Interval:    req.Interval,
				OpenTime:    openTime,
				CloseTime:   closeTime,
				Open:        open,
				High:        high,
				Low:         low,
				Close:       closePrice,
				Volume:      volume,
				QuoteVolume: quoteVolume,
			}
		}
	}
	
	return &KlineResponse{
		Data: klines,
		Provenance: Provenance{
			Venue:     o.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

// Helper functions for OKX symbol and interval conversion
func (o *OKXProvider) convertSymbolToOKX(symbol string) string {
	// Convert BTCUSDT -> BTC-USDT-SWAP
	if strings.HasSuffix(symbol, "USDT") {
		base := strings.TrimSuffix(symbol, "USDT")
		return fmt.Sprintf("%s-USDT-SWAP", base)
	}
	return symbol
}

func (o *OKXProvider) convertSpotSymbolToOKX(symbol string) string {
	// Convert BTCUSDT -> BTC-USDT
	if strings.HasSuffix(symbol, "USDT") {
		base := strings.TrimSuffix(symbol, "USDT")
		return fmt.Sprintf("%s-USDT", base)
	}
	return symbol
}

func (o *OKXProvider) convertSymbolFromOKX(instId string) string {
	// Convert BTC-USDT-SWAP -> BTCUSDT or BTC-USDT -> BTCUSDT
	parts := strings.Split(instId, "-")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return instId
}

func (o *OKXProvider) convertIntervalToOKX(interval string) string {
	// Convert standard intervals to OKX format
	switch interval {
	case "1m":
		return "1m"
	case "5m":
		return "5m"
	case "15m":
		return "15m"
	case "1h":
		return "1H"
	case "4h":
		return "4H"
	case "1d":
		return "1D"
	default:
		return interval
	}
}

func (o *OKXProvider) getIntervalDuration(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return time.Hour
	}
}

// Unsupported capabilities return errors
func (o *OKXProvider) GetSupplyReserves(ctx context.Context, req *SupplyRequest) (*SupplyResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (o *OKXProvider) GetWhaleDetection(ctx context.Context, req *WhaleRequest) (*WhaleResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (o *OKXProvider) GetCVD(ctx context.Context, req *CVDRequest) (*CVDResponse, error) {
	return nil, ErrCapabilityNotSupported
}