package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// BinanceProvider implements the Provider interface for Binance
type BinanceProvider struct {
	name        string
	baseURL     string
	futuresURL  string
	client      *http.Client
	rateLimiter *RateLimiter
}

// NewBinanceProvider creates a new Binance provider with free/keyless endpoints
func NewBinanceProvider() *BinanceProvider {
	return &BinanceProvider{
		name:       "binance",
		baseURL:    "https://api.binance.com",    // Spot API
		futuresURL: "https://fapi.binance.com",   // Futures API
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: NewRateLimiter(1200, 20), // 1200 weight limit, 20 RPS
	}
}

func (b *BinanceProvider) Name() string {
	return b.name
}

func (b *BinanceProvider) HasCapability(cap Capability) bool {
	switch cap {
	case CapabilityFunding, CapabilitySpotTrades, CapabilityOrderBookL2, CapabilityKlineData:
		return true
	case CapabilitySupplyReserves, CapabilityWhaleDetection, CapabilityCVD:
		return false // Not available via free APIs
	}
	return false
}

func (b *BinanceProvider) Probe(ctx context.Context) (*ProbeResult, error) {
	start := time.Now()
	
	// Use server time endpoint as a lightweight health check
	endpoint := "/api/v3/time"
	url := b.baseURL + endpoint
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &ProbeResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}, nil
	}
	
	resp, err := b.client.Do(req)
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

func (b *BinanceProvider) GetFundingHistory(ctx context.Context, req *FundingRequest) (*FundingResponse, error) {
	start := time.Now()
	
	if !b.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", b.name)
	}
	
	endpoint := "/fapi/v1/fundingHistory"
	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("limit", strconv.Itoa(req.Limit))
	
	fullURL := fmt.Sprintf("%s%s?%s", b.futuresURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := b.client.Do(httpReq)
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
	
	var result []struct {
		Symbol      string `json:"symbol"`
		FundingRate string `json:"fundingRate"`
		FundingTime int64  `json:"fundingTime"`
		MarkPrice   string `json:"markPrice"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	rates := make([]FundingRate, len(result))
	for i, r := range result {
		rate, _ := strconv.ParseFloat(r.FundingRate, 64)
		markPrice, _ := strconv.ParseFloat(r.MarkPrice, 64)
		
		rates[i] = FundingRate{
			Symbol:    r.Symbol,
			Rate:      rate,
			Timestamp: time.Unix(r.FundingTime/1000, 0),
			MarkPrice: markPrice,
		}
	}
	
	return &FundingResponse{
		Data: rates,
		Provenance: Provenance{
			Venue:     b.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

func (b *BinanceProvider) GetSpotTrades(ctx context.Context, req *SpotTradesRequest) (*SpotTradesResponse, error) {
	start := time.Now()
	
	if !b.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", b.name)
	}
	
	endpoint := "/api/v3/trades"
	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("limit", strconv.Itoa(req.Limit))
	
	fullURL := fmt.Sprintf("%s%s?%s", b.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := b.client.Do(httpReq)
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
	
	var result []struct {
		ID           int64  `json:"id"`
		Price        string `json:"price"`
		Qty          string `json:"qty"`
		QuoteQty     string `json:"quoteQty"`
		Time         int64  `json:"time"`
		IsBuyerMaker bool   `json:"isBuyerMaker"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	trades := make([]SpotTrade, len(result))
	for i, r := range result {
		price, _ := strconv.ParseFloat(r.Price, 64)
		volume, _ := strconv.ParseFloat(r.Qty, 64)
		
		side := "buy"
		if r.IsBuyerMaker {
			side = "sell" // If buyer is maker, trade was a sell
		}
		
		trades[i] = SpotTrade{
			Symbol:    req.Symbol,
			Price:     price,
			Volume:    volume,
			Side:      side,
			Timestamp: time.Unix(r.Time/1000, 0),
			TradeID:   strconv.FormatInt(r.ID, 10),
		}
	}
	
	return &SpotTradesResponse{
		Data: trades,
		Provenance: Provenance{
			Venue:     b.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

func (b *BinanceProvider) GetOrderBookL2(ctx context.Context, req *OrderBookRequest) (*OrderBookResponse, error) {
	start := time.Now()
	
	if !b.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", b.name)
	}
	
	endpoint := "/api/v3/depth"
	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("limit", strconv.Itoa(req.Limit))
	
	fullURL := fmt.Sprintf("%s%s?%s", b.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := b.client.Do(httpReq)
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
		LastUpdateID int64      `json:"lastUpdateId"`
		Bids         [][]string `json:"bids"`
		Asks         [][]string `json:"asks"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	// Convert bids and asks
	bids := make([]OrderBookEntry, len(result.Bids))
	for i, bid := range result.Bids {
		if len(bid) >= 2 {
			price, _ := strconv.ParseFloat(bid[0], 64)
			size, _ := strconv.ParseFloat(bid[1], 64)
			bids[i] = OrderBookEntry{Price: price, Size: size}
		}
	}
	
	asks := make([]OrderBookEntry, len(result.Asks))
	for i, ask := range result.Asks {
		if len(ask) >= 2 {
			price, _ := strconv.ParseFloat(ask[0], 64)
			size, _ := strconv.ParseFloat(ask[1], 64)
			asks[i] = OrderBookEntry{Price: price, Size: size}
		}
	}
	
	return &OrderBookResponse{
		Data: &OrderBookL2{
			Symbol:    req.Symbol,
			Bids:      bids,
			Asks:      asks,
			Timestamp: time.Now(),
		},
		Provenance: Provenance{
			Venue:     b.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

func (b *BinanceProvider) GetKlineData(ctx context.Context, req *KlineRequest) (*KlineResponse, error) {
	start := time.Now()
	
	if !b.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", b.name)
	}
	
	endpoint := "/api/v3/klines"
	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("interval", req.Interval)
	params.Set("limit", strconv.Itoa(req.Limit))
	
	fullURL := fmt.Sprintf("%s%s?%s", b.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := b.client.Do(httpReq)
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
	
	var result [][]interface{}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	klines := make([]Kline, len(result))
	for i, k := range result {
		if len(k) >= 12 {
			// Binance kline format: [openTime, open, high, low, close, volume, closeTime, quoteVolume, ...]
			openTime, _ := k[0].(float64)
			open, _ := strconv.ParseFloat(k[1].(string), 64)
			high, _ := strconv.ParseFloat(k[2].(string), 64)
			low, _ := strconv.ParseFloat(k[3].(string), 64)
			closePrice, _ := strconv.ParseFloat(k[4].(string), 64)
			volume, _ := strconv.ParseFloat(k[5].(string), 64)
			closeTime, _ := k[6].(float64)
			quoteVolume, _ := strconv.ParseFloat(k[7].(string), 64)
			
			klines[i] = Kline{
				Symbol:      req.Symbol,
				Interval:    req.Interval,
				OpenTime:    time.Unix(int64(openTime)/1000, 0),
				CloseTime:   time.Unix(int64(closeTime)/1000, 0),
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
			Venue:     b.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

// Unsupported capabilities return errors
func (b *BinanceProvider) GetSupplyReserves(ctx context.Context, req *SupplyRequest) (*SupplyResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (b *BinanceProvider) GetWhaleDetection(ctx context.Context, req *WhaleRequest) (*WhaleResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (b *BinanceProvider) GetCVD(ctx context.Context, req *CVDRequest) (*CVDResponse, error) {
	return nil, ErrCapabilityNotSupported
}

// RateLimiter implementation for Binance weight-based rate limiting
type RateLimiter struct {
	tokens     int
	maxTokens  int
	refillRate int
	lastRefill time.Time
}

func NewRateLimiter(maxTokens, refillRatePerSecond int) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRatePerSecond,
		lastRefill: time.Now(),
	}
}

func (rl *RateLimiter) Allow() bool {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	
	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed * float64(rl.refillRate))
	if tokensToAdd > 0 {
		rl.tokens = min(rl.maxTokens, rl.tokens+tokensToAdd)
		rl.lastRefill = now
	}
	
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}