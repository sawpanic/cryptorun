package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/interfaces"
	"github.com/sawpanic/cryptorun/internal/providers/guards"
)

// Adapter implements the Exchange interface for Binance
type Adapter struct {
	name       string
	baseURL    string
	httpClient *http.Client
	guard      *guards.ProviderGuard
	lastSeen   time.Time
	healthStats HealthStats
}

type HealthStats struct {
	RequestCount  int64
	ErrorCount    int64
	LastErrorTime time.Time
	AvgLatency    time.Duration
}

// NewAdapter creates a new Binance exchange adapter
func NewAdapter(name string) *Adapter {
	config := guards.ProviderConfig{
		Name:          name,
		SustainedRate: 20.0, // 20 RPS for Binance spot API
		BurstLimit:    50,   // Allow bursts up to 50 requests
		TTLSeconds:    5,    // 5 seconds for hot data
		MaxRetries:    3,
		FailureThresh: 0.1,  // 10% failure rate threshold
		WindowRequests: 100,
		ProbeInterval:  30,
	}

	return &Adapter{
		name:    name,
		baseURL: "https://api.binance.com/api/v3",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		guard:    guards.NewProviderGuard(config),
		lastSeen: time.Now(),
		healthStats: HealthStats{},
	}
}

// Name returns the exchange name
func (a *Adapter) Name() string {
	return a.name
}

// ConnectWS is not implemented for REST-only adapter
func (a *Adapter) ConnectWS(ctx context.Context) error {
	return fmt.Errorf("WebSocket not supported in REST-only mode")
}

// SubscribeTrades is not implemented for REST-only adapter
func (a *Adapter) SubscribeTrades(symbol string, callback interfaces.TradesCallback) error {
	return fmt.Errorf("WebSocket subscriptions not supported in REST-only mode")
}

// SubscribeBookL2 is not implemented for REST-only adapter
func (a *Adapter) SubscribeBookL2(symbol string, callback interfaces.BookL2Callback) error {
	return fmt.Errorf("WebSocket subscriptions not supported in REST-only mode")
}

// StreamKlines is not implemented for REST-only adapter
func (a *Adapter) StreamKlines(symbol string, interval string, callback interfaces.KlinesCallback) error {
	return fmt.Errorf("WebSocket subscriptions not supported in REST-only mode")
}

// GetKlines fetches historical kline data
func (a *Adapter) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]interfaces.Kline, error) {
	if limit <= 0 {
		limit = 100
	}
	
	// Normalize symbol for Binance (BTCUSDT format)
	normalizedSymbol := a.NormalizeSymbol(symbol)
	normalizedInterval := a.NormalizeInterval(interval)
	
	url := fmt.Sprintf("%s/klines?symbol=%s&interval=%s&limit=%d", 
		a.baseURL, normalizedSymbol, normalizedInterval, limit)
	
	req := guards.GuardedRequest{
		Method:  "GET",
		URL:     url,
		Headers: map[string]string{
			"Accept": "application/json",
		},
		CacheKey: fmt.Sprintf("binance_klines_%s_%s_%d", normalizedSymbol, normalizedInterval, limit),
	}
	
	start := time.Now()
	resp, err := a.guard.Execute(ctx, req, a.httpFetcher)
	a.updateHealthStats(err, time.Since(start))
	
	if err != nil {
		return nil, fmt.Errorf("binance klines request failed: %w", err)
	}
	
	var rawKlines [][]interface{}
	if err := json.Unmarshal(resp.Data, &rawKlines); err != nil {
		return nil, fmt.Errorf("failed to unmarshal klines: %w", err)
	}
	
	klines := make([]interfaces.Kline, 0, len(rawKlines))
	for _, raw := range rawKlines {
		if len(raw) < 11 {
			continue
		}
		
		kline, err := a.parseKline(normalizedSymbol, raw)
		if err != nil {
			continue // Skip invalid klines
		}
		
		klines = append(klines, kline)
	}
	
	return klines, nil
}

// GetTrades fetches recent trades (not implemented for momentum scanning)
func (a *Adapter) GetTrades(ctx context.Context, symbol string, limit int) ([]interfaces.Trade, error) {
	return nil, fmt.Errorf("GetTrades not implemented for Binance REST adapter")
}

// GetBookL2 fetches order book data
func (a *Adapter) GetBookL2(ctx context.Context, symbol string) (*interfaces.BookL2, error) {
	normalizedSymbol := a.NormalizeSymbol(symbol)
	
	url := fmt.Sprintf("%s/depth?symbol=%s&limit=100", a.baseURL, normalizedSymbol)
	
	req := guards.GuardedRequest{
		Method:  "GET",
		URL:     url,
		Headers: map[string]string{
			"Accept": "application/json",
		},
		CacheKey: fmt.Sprintf("binance_depth_%s", normalizedSymbol),
	}
	
	start := time.Now()
	resp, err := a.guard.Execute(ctx, req, a.httpFetcher)
	a.updateHealthStats(err, time.Since(start))
	
	if err != nil {
		return nil, fmt.Errorf("binance depth request failed: %w", err)
	}
	
	var rawBook struct {
		LastUpdateID int64       `json:"lastUpdateId"`
		Bids         [][]string  `json:"bids"`
		Asks         [][]string  `json:"asks"`
	}
	
	if err := json.Unmarshal(resp.Data, &rawBook); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order book: %w", err)
	}
	
	book := &interfaces.BookL2{
		Symbol:    symbol,
		Venue:     a.name,
		Timestamp: time.Now(),
		Sequence:  rawBook.LastUpdateID,
		Bids:      make([]interfaces.BookLevel, 0, len(rawBook.Bids)),
		Asks:      make([]interfaces.BookLevel, 0, len(rawBook.Asks)),
	}
	
	for _, bid := range rawBook.Bids {
		if len(bid) >= 2 {
			price, _ := strconv.ParseFloat(bid[0], 64)
			size, _ := strconv.ParseFloat(bid[1], 64)
			if price > 0 && size > 0 {
				book.Bids = append(book.Bids, interfaces.BookLevel{
					Price: price,
					Size:  size,
				})
			}
		}
	}
	
	for _, ask := range rawBook.Asks {
		if len(ask) >= 2 {
			price, _ := strconv.ParseFloat(ask[0], 64)
			size, _ := strconv.ParseFloat(ask[1], 64)
			if price > 0 && size > 0 {
				book.Asks = append(book.Asks, interfaces.BookLevel{
					Price: price,
					Size:  size,
				})
			}
		}
	}
	
	return book, nil
}

// NormalizeSymbol converts symbols to Binance format (e.g., BTCUSD -> BTCUSDT)
func (a *Adapter) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	
	// Convert common USD pairs to USDT
	if strings.HasSuffix(symbol, "USD") && !strings.HasSuffix(symbol, "USDT") {
		return strings.TrimSuffix(symbol, "USD") + "USDT"
	}
	
	return symbol
}

// NormalizeInterval converts time intervals to Binance format
func (a *Adapter) NormalizeInterval(interval string) string {
	// Binance intervals: 1s, 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M
	switch strings.ToLower(interval) {
	case "1min", "1m":
		return "1m"
	case "5min", "5m":
		return "5m"
	case "15min", "15m":
		return "15m"
	case "30min", "30m":
		return "30m"
	case "1hour", "1h", "60m":
		return "1h"
	case "4hour", "4h":
		return "4h"
	case "1day", "1d", "24h":
		return "1d"
	default:
		return "1h" // Default to 1 hour
	}
}

// Health returns the current health status
func (a *Adapter) Health() interfaces.HealthStatus {
	status := "healthy"
	recommendation := "use_primary"
	
	// Calculate error rate
	errorRate := float64(0)
	if a.healthStats.RequestCount > 0 {
		errorRate = float64(a.healthStats.ErrorCount) / float64(a.healthStats.RequestCount)
	}
	
	// Check if unhealthy
	if errorRate > 0.1 || time.Since(a.lastSeen) > 5*time.Minute {
		status = "degraded"
		recommendation = "use_fallback"
	}
	
	if errorRate > 0.5 || time.Since(a.lastSeen) > 15*time.Minute {
		status = "unhealthy"
		recommendation = "avoid"
	}
	
	return interfaces.HealthStatus{
		Venue:          a.name,
		Status:         status,
		LastSeen:       a.lastSeen,
		ErrorRate:      errorRate,
		P99Latency:     a.healthStats.AvgLatency,
		WSConnected:    false,
		RESTHealthy:    status == "healthy",
		Recommendation: recommendation,
	}
}

// Helper methods

func (a *Adapter) httpFetcher(ctx context.Context, req guards.GuardedRequest) (*guards.GuardedResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, nil)
	if err != nil {
		return nil, err
	}
	
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}
	
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	
	a.lastSeen = time.Now()
	
	return &guards.GuardedResponse{
		Data:   body,
		Cached: false,
		Age:    0,
	}, nil
}

func (a *Adapter) parseKline(symbol string, raw []interface{}) (interfaces.Kline, error) {
	if len(raw) < 11 {
		return interfaces.Kline{}, fmt.Errorf("insufficient kline data")
	}
	
	openTime, _ := raw[0].(float64)
	open, _ := strconv.ParseFloat(fmt.Sprintf("%v", raw[1]), 64)
	high, _ := strconv.ParseFloat(fmt.Sprintf("%v", raw[2]), 64)
	low, _ := strconv.ParseFloat(fmt.Sprintf("%v", raw[3]), 64)
	close, _ := strconv.ParseFloat(fmt.Sprintf("%v", raw[4]), 64)
	volume, _ := strconv.ParseFloat(fmt.Sprintf("%v", raw[5]), 64)
	// closeTime, _ := raw[6].(float64) // Skip close time for now
	quoteVolume, _ := strconv.ParseFloat(fmt.Sprintf("%v", raw[7]), 64)
	
	return interfaces.Kline{
		Symbol:    symbol,
		Venue:     a.name,
		Timestamp: time.Unix(int64(openTime)/1000, 0),
		Interval:  "1h", // Default interval - would need to be passed from request
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
		QuoteVol:  quoteVolume,
	}, nil
}

func (a *Adapter) updateHealthStats(err error, latency time.Duration) {
	a.healthStats.RequestCount++
	if err != nil {
		a.healthStats.ErrorCount++
		a.healthStats.LastErrorTime = time.Now()
	}
	
	// Simple running average for latency
	if a.healthStats.AvgLatency == 0 {
		a.healthStats.AvgLatency = latency
	} else {
		a.healthStats.AvgLatency = (a.healthStats.AvgLatency + latency) / 2
	}
}