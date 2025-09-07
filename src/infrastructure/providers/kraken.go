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

// KrakenProvider implements the Provider interface for Kraken
type KrakenProvider struct {
	name        string
	baseURL     string
	client      *http.Client
	rateLimiter *RateLimiter
}

// NewKrakenProvider creates a new Kraken provider with free/keyless endpoints
func NewKrakenProvider() *KrakenProvider {
	return &KrakenProvider{
		name:    "kraken",
		baseURL: "https://api.kraken.com",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: NewRateLimiter(60, 1), // 60 calls per minute, 1 RPS
	}
}

func (k *KrakenProvider) Name() string {
	return k.name
}

func (k *KrakenProvider) HasCapability(cap Capability) bool {
	switch cap {
	case CapabilitySpotTrades, CapabilityOrderBookL2, CapabilityKlineData:
		return true
	case CapabilityFunding: // Kraken doesn't offer perpetual futures via public API
		return false
	case CapabilitySupplyReserves, CapabilityWhaleDetection, CapabilityCVD:
		return false // Not available via free APIs
	}
	return false
}

func (k *KrakenProvider) Probe(ctx context.Context) (*ProbeResult, error) {
	start := time.Now()
	
	// Use server time endpoint as a lightweight health check
	endpoint := "/0/public/Time"
	url := k.baseURL + endpoint
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &ProbeResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}, nil
	}
	
	resp, err := k.client.Do(req)
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

func (k *KrakenProvider) GetSpotTrades(ctx context.Context, req *SpotTradesRequest) (*SpotTradesResponse, error) {
	start := time.Now()
	
	if !k.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", k.name)
	}
	
	// Convert symbol to Kraken format (e.g., BTCUSDT -> XBTUSD)
	pair := k.convertSymbolToKraken(req.Symbol)
	
	endpoint := "/0/public/Trades"
	params := url.Values{}
	params.Set("pair", pair)
	params.Set("count", strconv.Itoa(req.Limit))
	
	fullURL := fmt.Sprintf("%s%s?%s", k.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := k.client.Do(httpReq)
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
		Error  []string               `json:"error"`
		Result map[string][][]string `json:"result"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	if len(result.Error) > 0 {
		return nil, fmt.Errorf("Kraken API error: %v", result.Error)
	}
	
	// Get trades from result (first key is the pair)
	var tradeData [][]string
	for _, trades := range result.Result {
		tradeData = trades
		break
	}
	
	if tradeData == nil {
		return nil, fmt.Errorf("no trade data found in response")
	}
	
	trades := make([]SpotTrade, len(tradeData))
	for i, t := range tradeData {
		if len(t) >= 6 {
			// Kraken trade format: [price, volume, time, side, type, misc]
			price, _ := strconv.ParseFloat(t[0], 64)
			volume, _ := strconv.ParseFloat(t[1], 64)
			timestamp, _ := strconv.ParseFloat(t[2], 64)
			side := "buy"
			if t[3] == "s" {
				side = "sell"
			}
			
			trades[i] = SpotTrade{
				Symbol:    req.Symbol,
				Price:     price,
				Volume:    volume,
				Side:      side,
				Timestamp: time.Unix(int64(timestamp), 0),
				TradeID:   fmt.Sprintf("%d", i), // Kraken doesn't provide trade IDs in public API
			}
		}
	}
	
	return &SpotTradesResponse{
		Data: trades,
		Provenance: Provenance{
			Venue:     k.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

func (k *KrakenProvider) GetOrderBookL2(ctx context.Context, req *OrderBookRequest) (*OrderBookResponse, error) {
	start := time.Now()
	
	if !k.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", k.name)
	}
	
	pair := k.convertSymbolToKraken(req.Symbol)
	
	endpoint := "/0/public/Depth"
	params := url.Values{}
	params.Set("pair", pair)
	params.Set("count", strconv.Itoa(req.Limit))
	
	fullURL := fmt.Sprintf("%s%s?%s", k.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := k.client.Do(httpReq)
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
		Error  []string `json:"error"`
		Result map[string]struct {
			Asks [][]string `json:"asks"`
			Bids [][]string `json:"bids"`
		} `json:"result"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	if len(result.Error) > 0 {
		return nil, fmt.Errorf("Kraken API error: %v", result.Error)
	}
	
	// Get order book from result (first key is the pair)
	var bookData struct {
		Asks [][]string `json:"asks"`
		Bids [][]string `json:"bids"`
	}
	for _, book := range result.Result {
		bookData = book
		break
	}
	
	bids := make([]OrderBookEntry, len(bookData.Bids))
	for i, bid := range bookData.Bids {
		if len(bid) >= 2 {
			price, _ := strconv.ParseFloat(bid[0], 64)
			size, _ := strconv.ParseFloat(bid[1], 64)
			bids[i] = OrderBookEntry{Price: price, Size: size}
		}
	}
	
	asks := make([]OrderBookEntry, len(bookData.Asks))
	for i, ask := range bookData.Asks {
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
			Venue:     k.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

func (k *KrakenProvider) GetKlineData(ctx context.Context, req *KlineRequest) (*KlineResponse, error) {
	start := time.Now()
	
	if !k.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", k.name)
	}
	
	pair := k.convertSymbolToKraken(req.Symbol)
	
	endpoint := "/0/public/OHLC"
	params := url.Values{}
	params.Set("pair", pair)
	params.Set("interval", k.convertIntervalToKraken(req.Interval))
	
	fullURL := fmt.Sprintf("%s%s?%s", k.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := k.client.Do(httpReq)
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
		Error  []string                     `json:"error"`
		Result map[string][][]interface{} `json:"result"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	if len(result.Error) > 0 {
		return nil, fmt.Errorf("Kraken API error: %v", result.Error)
	}
	
	// Get OHLC data from result (first key is the pair)
	var ohlcData [][]interface{}
	for key, data := range result.Result {
		if !strings.Contains(key, "last") { // Skip "last" field
			ohlcData = data
			break
		}
	}
	
	if ohlcData == nil {
		return nil, fmt.Errorf("no OHLC data found in response")
	}
	
	// Limit results to requested amount
	maxEntries := req.Limit
	if maxEntries <= 0 || maxEntries > len(ohlcData) {
		maxEntries = len(ohlcData)
	}
	
	klines := make([]Kline, maxEntries)
	for i := 0; i < maxEntries; i++ {
		ohlc := ohlcData[len(ohlcData)-maxEntries+i] // Get most recent entries
		if len(ohlc) >= 7 {
			// Kraken OHLC format: [time, open, high, low, close, vwap, volume, count]
			timestamp, _ := ohlc[0].(float64)
			open, _ := strconv.ParseFloat(ohlc[1].(string), 64)
			high, _ := strconv.ParseFloat(ohlc[2].(string), 64)
			low, _ := strconv.ParseFloat(ohlc[3].(string), 64)
			closePrice, _ := strconv.ParseFloat(ohlc[4].(string), 64)
			volume, _ := strconv.ParseFloat(ohlc[6].(string), 64)
			
			openTime := time.Unix(int64(timestamp), 0)
			closeTime := openTime.Add(k.getIntervalDuration(req.Interval))
			
			klines[i] = Kline{
				Symbol:    req.Symbol,
				Interval:  req.Interval,
				OpenTime:  openTime,
				CloseTime: closeTime,
				Open:      open,
				High:      high,
				Low:       low,
				Close:     closePrice,
				Volume:    volume,
			}
		}
	}
	
	return &KlineResponse{
		Data: klines,
		Provenance: Provenance{
			Venue:     k.name,
			Endpoint:  endpoint,
			Window:    req.Limit,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

// Helper functions for Kraken symbol and interval conversion
func (k *KrakenProvider) convertSymbolToKraken(symbol string) string {
	// Convert standard symbols to Kraken pairs
	switch symbol {
	case "BTCUSDT", "BTCUSD":
		return "XBTUSD"
	case "ETHUSDT", "ETHUSD":
		return "ETHUSD"
	case "ADAUSDT", "ADAUSD":
		return "ADAUSD"
	case "DOTUSDT", "DOTUSD":
		return "DOTUSD"
	case "LINKUSDT", "LINKUSD":
		return "LINKUSD"
	case "LTCUSDT", "LTCUSD":
		return "LTCUSD"
	case "XRPUSDT", "XRPUSD":
		return "XRPUSD"
	default:
		// Try to convert XXXUSDT -> XXXUSD for other pairs
		if strings.HasSuffix(symbol, "USDT") {
			base := strings.TrimSuffix(symbol, "USDT")
			return fmt.Sprintf("%sUSD", base)
		}
		return symbol
	}
}

func (k *KrakenProvider) convertIntervalToKraken(interval string) string {
	// Convert standard intervals to Kraken intervals (in minutes)
	switch interval {
	case "1m":
		return "1"
	case "5m":
		return "5"
	case "15m":
		return "15"
	case "1h":
		return "60"
	case "4h":
		return "240"
	case "1d":
		return "1440"
	default:
		return "60" // Default to 1 hour
	}
}

func (k *KrakenProvider) getIntervalDuration(interval string) time.Duration {
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

// Kraken doesn't support these capabilities via free APIs
func (k *KrakenProvider) GetFundingHistory(ctx context.Context, req *FundingRequest) (*FundingResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (k *KrakenProvider) GetSupplyReserves(ctx context.Context, req *SupplyRequest) (*SupplyResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (k *KrakenProvider) GetWhaleDetection(ctx context.Context, req *WhaleRequest) (*WhaleResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (k *KrakenProvider) GetCVD(ctx context.Context, req *CVDRequest) (*CVDResponse, error) {
	return nil, ErrCapabilityNotSupported
}