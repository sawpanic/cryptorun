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

// CoinbaseProvider implements the Provider interface for Coinbase
type CoinbaseProvider struct {
	name        string
	baseURL     string
	client      *http.Client
	rateLimiter *RateLimiter
}

// NewCoinbaseProvider creates a new Coinbase provider with free/keyless endpoints
func NewCoinbaseProvider() *CoinbaseProvider {
	return &CoinbaseProvider{
		name:    "coinbase",
		baseURL: "https://api.exchange.coinbase.com", // Coinbase Pro/Advanced API
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: NewRateLimiter(100, 5), // Conservative rate limit for public endpoints
	}
}

func (c *CoinbaseProvider) Name() string {
	return c.name
}

func (c *CoinbaseProvider) HasCapability(cap Capability) bool {
	switch cap {
	case CapabilitySpotTrades, CapabilityOrderBookL2, CapabilityKlineData:
		return true
	case CapabilityFunding: // Coinbase doesn't offer perpetual futures
		return false
	case CapabilitySupplyReserves, CapabilityWhaleDetection, CapabilityCVD:
		return false // Not available via free APIs
	}
	return false
}

func (c *CoinbaseProvider) Probe(ctx context.Context) (*ProbeResult, error) {
	start := time.Now()
	
	// Use server time endpoint as a lightweight health check
	endpoint := "/time"
	url := c.baseURL + endpoint
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &ProbeResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}, nil
	}
	
	resp, err := c.client.Do(req)
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

func (c *CoinbaseProvider) GetSpotTrades(ctx context.Context, req *SpotTradesRequest) (*SpotTradesResponse, error) {
	start := time.Now()
	
	if !c.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", c.name)
	}
	
	// Convert symbol to Coinbase format (e.g., BTCUSDT -> BTC-USD)
	productId := c.convertSymbolToCoinbase(req.Symbol)
	
	endpoint := fmt.Sprintf("/products/%s/trades", productId)
	params := url.Values{}
	params.Set("limit", strconv.Itoa(req.Limit))
	
	fullURL := fmt.Sprintf("%s%s?%s", c.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := c.client.Do(httpReq)
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
		TradeID int    `json:"trade_id"`
		Price   string `json:"price"`
		Size    string `json:"size"`
		Side    string `json:"side"`
		Time    string `json:"time"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	trades := make([]SpotTrade, len(result))
	for i, t := range result {
		price, _ := strconv.ParseFloat(t.Price, 64)
		volume, _ := strconv.ParseFloat(t.Size, 64)
		
		timestamp, _ := time.Parse(time.RFC3339, t.Time)
		
		trades[i] = SpotTrade{
			Symbol:    req.Symbol,
			Price:     price,
			Volume:    volume,
			Side:      t.Side,
			Timestamp: timestamp,
			TradeID:   strconv.Itoa(t.TradeID),
		}
	}
	
	return &SpotTradesResponse{
		Data: trades,
		Provenance: Provenance{
			Venue:     c.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

func (c *CoinbaseProvider) GetOrderBookL2(ctx context.Context, req *OrderBookRequest) (*OrderBookResponse, error) {
	start := time.Now()
	
	if !c.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", c.name)
	}
	
	productId := c.convertSymbolToCoinbase(req.Symbol)
	
	endpoint := fmt.Sprintf("/products/%s/book", productId)
	params := url.Values{}
	params.Set("level", "2") // Level 2 order book
	
	fullURL := fmt.Sprintf("%s%s?%s", c.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := c.client.Do(httpReq)
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
		Sequence int64      `json:"sequence"`
		Bids     [][]string `json:"bids"`
		Asks     [][]string `json:"asks"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	// Convert bids and asks, limit to requested amount
	maxEntries := req.Limit
	if maxEntries <= 0 {
		maxEntries = 100 // Default limit
	}
	
	bidCount := len(result.Bids)
	if bidCount > maxEntries {
		bidCount = maxEntries
	}
	bids := make([]OrderBookEntry, bidCount)
	for i := 0; i < bidCount; i++ {
		if len(result.Bids[i]) >= 2 {
			price, _ := strconv.ParseFloat(result.Bids[i][0], 64)
			size, _ := strconv.ParseFloat(result.Bids[i][1], 64)
			bids[i] = OrderBookEntry{Price: price, Size: size}
		}
	}
	
	askCount := len(result.Asks)
	if askCount > maxEntries {
		askCount = maxEntries
	}
	asks := make([]OrderBookEntry, askCount)
	for i := 0; i < askCount; i++ {
		if len(result.Asks[i]) >= 2 {
			price, _ := strconv.ParseFloat(result.Asks[i][0], 64)
			size, _ := strconv.ParseFloat(result.Asks[i][1], 64)
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
			Venue:     c.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

func (c *CoinbaseProvider) GetKlineData(ctx context.Context, req *KlineRequest) (*KlineResponse, error) {
	start := time.Now()
	
	if !c.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", c.name)
	}
	
	productId := c.convertSymbolToCoinbase(req.Symbol)
	
	endpoint := fmt.Sprintf("/products/%s/candles", productId)
	params := url.Values{}
	params.Set("granularity", c.convertIntervalToCoinbase(req.Interval))
	
	fullURL := fmt.Sprintf("%s%s?%s", c.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := c.client.Do(httpReq)
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
	
	var result [][]float64
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	// Limit results to requested amount and reverse (Coinbase returns newest first)
	maxEntries := req.Limit
	if maxEntries <= 0 || maxEntries > len(result) {
		maxEntries = len(result)
	}
	
	klines := make([]Kline, maxEntries)
	for i := 0; i < maxEntries; i++ {
		k := result[len(result)-maxEntries+i] // Reverse order to get oldest first
		if len(k) >= 6 {
			// Coinbase candle format: [time, low, high, open, close, volume]
			timestamp := int64(k[0])
			low := k[1]
			high := k[2]
			open := k[3]
			close := k[4]
			volume := k[5]
			
			openTime := time.Unix(timestamp, 0)
			closeTime := openTime.Add(c.getIntervalDuration(req.Interval))
			
			klines[i] = Kline{
				Symbol:    req.Symbol,
				Interval:  req.Interval,
				OpenTime:  openTime,
				CloseTime: closeTime,
				Open:      open,
				High:      high,
				Low:       low,
				Close:     close,
				Volume:    volume,
			}
		}
	}
	
	return &KlineResponse{
		Data: klines,
		Provenance: Provenance{
			Venue:     c.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

// Helper functions for Coinbase symbol and interval conversion
func (c *CoinbaseProvider) convertSymbolToCoinbase(symbol string) string {
	// Convert BTCUSDT -> BTC-USD, ETHUSDT -> ETH-USD, etc.
	if strings.HasSuffix(symbol, "USDT") {
		base := strings.TrimSuffix(symbol, "USDT")
		return fmt.Sprintf("%s-USD", base)
	}
	if strings.HasSuffix(symbol, "USD") {
		base := strings.TrimSuffix(symbol, "USD")
		return fmt.Sprintf("%s-USD", base)
	}
	return symbol
}

func (c *CoinbaseProvider) convertIntervalToCoinbase(interval string) string {
	// Convert standard intervals to Coinbase granularity (seconds)
	switch interval {
	case "1m":
		return "60"
	case "5m":
		return "300"
	case "15m":
		return "900"
	case "1h":
		return "3600"
	case "4h":
		return "14400"
	case "1d":
		return "86400"
	default:
		return "3600" // Default to 1 hour
	}
}

func (c *CoinbaseProvider) getIntervalDuration(interval string) time.Duration {
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

// Coinbase doesn't support these capabilities via free APIs
func (c *CoinbaseProvider) GetFundingHistory(ctx context.Context, req *FundingRequest) (*FundingResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (c *CoinbaseProvider) GetSupplyReserves(ctx context.Context, req *SupplyRequest) (*SupplyResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (c *CoinbaseProvider) GetWhaleDetection(ctx context.Context, req *WhaleRequest) (*WhaleResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (c *CoinbaseProvider) GetCVD(ctx context.Context, req *CVDRequest) (*CVDResponse, error) {
	return nil, ErrCapabilityNotSupported
}