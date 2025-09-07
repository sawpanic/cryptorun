package kraken

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/sawpanic/cryptorun/internal/data/facade"
)

// Adapter implements facade.Exchange for Kraken
type Adapter struct {
	name       string
	baseURL    string
	wsURL      string
	httpClient *http.Client
	
	// WebSocket connection
	wsConn     *websocket.Conn
	wsConnected bool
	
	// Callbacks
	tradesCallbacks map[string]facade.TradesCallback
	bookCallbacks   map[string]facade.BookL2Callback
	klinesCallbacks map[string]facade.KlinesCallback
	
	// Health tracking
	lastSeen    time.Time
	errorCount  int64
	totalReqs   int64
	avgLatency  time.Duration
}

// NewAdapter creates a new Kraken exchange adapter
func NewAdapter() *Adapter {
	return &Adapter{
		name:            "kraken",
		baseURL:         "https://api.kraken.com",
		wsURL:           "wss://ws.kraken.com",
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		tradesCallbacks: make(map[string]facade.TradesCallback),
		bookCallbacks:   make(map[string]facade.BookL2Callback),
		klinesCallbacks: make(map[string]facade.KlinesCallback),
		lastSeen:        time.Now(),
	}
}

// Name returns the exchange name
func (a *Adapter) Name() string {
	return a.name
}

// ConnectWS establishes WebSocket connection
func (a *Adapter) ConnectWS(ctx context.Context) error {
	log.Info().Str("venue", a.name).Str("url", a.wsURL).Msg("Connecting to WebSocket")
	
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second
	
	conn, _, err := dialer.Dial(a.wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Kraken WebSocket: %w", err)
	}
	
	a.wsConn = conn
	a.wsConnected = true
	a.lastSeen = time.Now()
	
	// Start message handling goroutine
	go a.handleWebSocketMessages(ctx)
	
	log.Info().Str("venue", a.name).Msg("WebSocket connected successfully")
	return nil
}

// SubscribeTrades subscribes to trade updates for a symbol
func (a *Adapter) SubscribeTrades(symbol string, callback facade.TradesCallback) error {
	if !a.wsConnected {
		return fmt.Errorf("WebSocket not connected")
	}
	
	normalizedSymbol := a.NormalizeSymbol(symbol)
	a.tradesCallbacks[normalizedSymbol] = callback
	
	// Send subscription message
	subMsg := map[string]interface{}{
		"event": "subscribe",
		"pair":  []string{normalizedSymbol},
		"subscription": map[string]interface{}{
			"name": "trade",
		},
	}
	
	return a.wsConn.WriteJSON(subMsg)
}

// SubscribeBookL2 subscribes to orderbook updates
func (a *Adapter) SubscribeBookL2(symbol string, callback facade.BookL2Callback) error {
	if !a.wsConnected {
		return fmt.Errorf("WebSocket not connected")
	}
	
	normalizedSymbol := a.NormalizeSymbol(symbol)
	a.bookCallbacks[normalizedSymbol] = callback
	
	subMsg := map[string]interface{}{
		"event": "subscribe",
		"pair":  []string{normalizedSymbol},
		"subscription": map[string]interface{}{
			"name":  "book",
			"depth": 100,
		},
	}
	
	return a.wsConn.WriteJSON(subMsg)
}

// StreamKlines subscribes to kline/candlestick updates
func (a *Adapter) StreamKlines(symbol string, interval string, callback facade.KlinesCallback) error {
	if !a.wsConnected {
		return fmt.Errorf("WebSocket not connected")
	}
	
	normalizedSymbol := a.NormalizeSymbol(symbol)
	normalizedInterval := a.NormalizeInterval(interval)
	key := fmt.Sprintf("%s:%s", normalizedSymbol, normalizedInterval)
	a.klinesCallbacks[key] = callback
	
	subMsg := map[string]interface{}{
		"event": "subscribe",
		"pair":  []string{normalizedSymbol},
		"subscription": map[string]interface{}{
			"name":     "ohlc",
			"interval": normalizedInterval,
		},
	}
	
	return a.wsConn.WriteJSON(subMsg)
}

// GetKlines fetches historical klines via REST API
func (a *Adapter) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]facade.Kline, error) {
	normalizedSymbol := a.NormalizeSymbol(symbol)
	normalizedInterval := a.NormalizeInterval(interval)
	
	// Kraken OHLC endpoint
	url := fmt.Sprintf("%s/0/public/OHLC?pair=%s&interval=%s", 
		a.baseURL, normalizedSymbol, normalizedInterval)
	
	if limit > 0 && limit < 720 {
		// Kraken doesn't have a direct limit parameter, but we can use 'since'
		// For simplicity, we'll fetch all and slice
	}
	
	start := time.Now()
	resp, err := a.httpClient.Get(url)
	if err != nil {
		a.errorCount++
		return nil, fmt.Errorf("failed to fetch klines from Kraken: %w", err)
	}
	defer resp.Body.Close()
	
	// Update metrics
	a.totalReqs++
	latency := time.Since(start)
	a.avgLatency = time.Duration((int64(a.avgLatency)*int64(a.totalReqs-1) + int64(latency)) / int64(a.totalReqs))
	a.lastSeen = time.Now()
	
	if resp.StatusCode != http.StatusOK {
		a.errorCount++
		return nil, fmt.Errorf("Kraken API error: status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	var krakenResp KrakenOHLCResponse
	if err := json.Unmarshal(body, &krakenResp); err != nil {
		return nil, fmt.Errorf("failed to parse Kraken OHLC response: %w", err)
	}
	
	if len(krakenResp.Error) > 0 {
		return nil, fmt.Errorf("Kraken API error: %v", krakenResp.Error)
	}
	
	// Convert to normalized format
	var klines []facade.Kline
	for pairKey, ohlcData := range krakenResp.Result {
		if strings.Contains(pairKey, "last") {
			continue // Skip the "last" field
		}
		
		ohlcArray, ok := ohlcData.([]interface{})
		if !ok {
			continue
		}
		
		for _, item := range ohlcArray {
			itemArray, ok := item.([]interface{})
			if !ok || len(itemArray) < 8 {
				continue
			}
			
			timestamp := parseFloat64(itemArray[0])
			open := parseStringFloat(itemArray[1])
			high := parseStringFloat(itemArray[2])
			low := parseStringFloat(itemArray[3])
			close := parseStringFloat(itemArray[4])
			vwap := parseStringFloat(itemArray[5])  // Volume-weighted average price
			volume := parseStringFloat(itemArray[6])
			_ = parseFloat64(itemArray[7]) // count - not currently used
			
			kline := facade.Kline{
				Symbol:    symbol,
				Venue:     a.name,
				Timestamp: time.Unix(int64(timestamp), 0),
				Interval:  interval,
				Open:      open,
				High:      high,
				Low:       low,
				Close:     close,
				Volume:    volume,
				QuoteVol:  volume * vwap, // Approximate quote volume
			}
			
			klines = append(klines, kline)
		}
		break // Only process first pair
	}
	
	// Apply limit if specified
	if limit > 0 && len(klines) > limit {
		klines = klines[len(klines)-limit:] // Return most recent
	}
	
	log.Debug().Str("venue", a.name).Str("symbol", symbol).
		Int("count", len(klines)).Dur("latency", latency).
		Msg("Fetched klines via REST")
	
	return klines, nil
}

// GetTrades fetches recent trades via REST API
func (a *Adapter) GetTrades(ctx context.Context, symbol string, limit int) ([]facade.Trade, error) {
	normalizedSymbol := a.NormalizeSymbol(symbol)
	url := fmt.Sprintf("%s/0/public/Trades?pair=%s", a.baseURL, normalizedSymbol)
	
	start := time.Now()
	resp, err := a.httpClient.Get(url)
	if err != nil {
		a.errorCount++
		return nil, fmt.Errorf("failed to fetch trades from Kraken: %w", err)
	}
	defer resp.Body.Close()
	
	a.totalReqs++
	latency := time.Since(start)
	a.avgLatency = time.Duration((int64(a.avgLatency)*int64(a.totalReqs-1) + int64(latency)) / int64(a.totalReqs))
	a.lastSeen = time.Now()
	
	if resp.StatusCode != http.StatusOK {
		a.errorCount++
		return nil, fmt.Errorf("Kraken API error: status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	var krakenResp KrakenTradesResponse
	if err := json.Unmarshal(body, &krakenResp); err != nil {
		return nil, fmt.Errorf("failed to parse Kraken trades response: %w", err)
	}
	
	if len(krakenResp.Error) > 0 {
		return nil, fmt.Errorf("Kraken API error: %v", krakenResp.Error)
	}
	
	var trades []facade.Trade
	for pairKey, tradesData := range krakenResp.Result {
		if strings.Contains(pairKey, "last") {
			continue
		}
		
		tradesArray, ok := tradesData.([]interface{})
		if !ok {
			continue
		}
		
		for _, item := range tradesArray {
			itemArray, ok := item.([]interface{})
			if !ok || len(itemArray) < 6 {
				continue
			}
			
			price := parseStringFloat(itemArray[0])
			volume := parseStringFloat(itemArray[1])
			timestamp := parseFloat64(itemArray[2])
			side := "buy"
			if itemArray[3].(string) == "s" {
				side = "sell"
			}
			tradeType := itemArray[4].(string) // m = market, l = limit
			misc := itemArray[5].(string)
			
			trade := facade.Trade{
				Symbol:    symbol,
				Venue:     a.name,
				Timestamp: time.Unix(int64(timestamp), int64((timestamp-float64(int64(timestamp)))*1e9)),
				Price:     price,
				Size:      volume,
				Side:      side,
				TradeID:   fmt.Sprintf("%.6f_%s_%s", timestamp, tradeType, misc),
			}
			
			trades = append(trades, trade)
		}
		break
	}
	
	// Apply limit
	if limit > 0 && len(trades) > limit {
		trades = trades[len(trades)-limit:]
	}
	
	log.Debug().Str("venue", a.name).Str("symbol", symbol).
		Int("count", len(trades)).Dur("latency", latency).
		Msg("Fetched trades via REST")
	
	return trades, nil
}

// GetBookL2 fetches current orderbook via REST API
func (a *Adapter) GetBookL2(ctx context.Context, symbol string) (*facade.BookL2, error) {
	normalizedSymbol := a.NormalizeSymbol(symbol)
	url := fmt.Sprintf("%s/0/public/Depth?pair=%s&count=100", a.baseURL, normalizedSymbol)
	
	start := time.Now()
	resp, err := a.httpClient.Get(url)
	if err != nil {
		a.errorCount++
		return nil, fmt.Errorf("failed to fetch orderbook from Kraken: %w", err)
	}
	defer resp.Body.Close()
	
	a.totalReqs++
	latency := time.Since(start)
	a.avgLatency = time.Duration((int64(a.avgLatency)*int64(a.totalReqs-1) + int64(latency)) / int64(a.totalReqs))
	a.lastSeen = time.Now()
	
	if resp.StatusCode != http.StatusOK {
		a.errorCount++
		return nil, fmt.Errorf("Kraken API error: status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	var krakenResp KrakenDepthResponse
	if err := json.Unmarshal(body, &krakenResp); err != nil {
		return nil, fmt.Errorf("failed to parse Kraken depth response: %w", err)
	}
	
	if len(krakenResp.Error) > 0 {
		return nil, fmt.Errorf("Kraken API error: %v", krakenResp.Error)
	}
	
	// Extract orderbook data
	for _, depthData := range krakenResp.Result {
		depthMap, ok := depthData.(map[string]interface{})
		if !ok {
			continue
		}
		
		var bids, asks []facade.BookLevel
		
		// Parse bids
		if bidsData, exists := depthMap["bids"]; exists {
			bidsArray, ok := bidsData.([]interface{})
			if ok {
				for _, bid := range bidsArray {
					bidArray, ok := bid.([]interface{})
					if ok && len(bidArray) >= 2 {
						price := parseStringFloat(bidArray[0])
						size := parseStringFloat(bidArray[1])
						bids = append(bids, facade.BookLevel{
							Price: price,
							Size:  size,
						})
					}
				}
			}
		}
		
		// Parse asks
		if asksData, exists := depthMap["asks"]; exists {
			asksArray, ok := asksData.([]interface{})
			if ok {
				for _, ask := range asksArray {
					askArray, ok := ask.([]interface{})
					if ok && len(askArray) >= 2 {
						price := parseStringFloat(askArray[0])
						size := parseStringFloat(askArray[1])
						asks = append(asks, facade.BookLevel{
							Price: price,
							Size:  size,
						})
					}
				}
			}
		}
		
		book := &facade.BookL2{
			Symbol:    symbol,
			Venue:     a.name,
			Timestamp: time.Now(),
			Bids:      bids,
			Asks:      asks,
			Sequence:  0, // Kraken doesn't provide sequence numbers in REST API
		}
		
		log.Debug().Str("venue", a.name).Str("symbol", symbol).
			Int("bids", len(bids)).Int("asks", len(asks)).
			Dur("latency", latency).Msg("Fetched orderbook via REST")
		
		return book, nil
	}
	
	return nil, fmt.Errorf("no orderbook data found for symbol %s", symbol)
}

// NormalizeSymbol converts symbol to Kraken format (XBTUSD, ETHUSD)
func (a *Adapter) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "/", "")
	symbol = strings.ReplaceAll(symbol, "-", "")
	
	// Handle common symbol mappings
	switch {
	case strings.HasPrefix(symbol, "BTC"):
		return strings.Replace(symbol, "BTC", "XBT", 1)
	default:
		return symbol
	}
}

// NormalizeInterval converts interval to Kraken format
func (a *Adapter) NormalizeInterval(interval string) string {
	// Kraken uses minutes: 1, 5, 15, 30, 60, 240, 1440, 10080, 21600
	switch strings.ToLower(interval) {
	case "1m":
		return "1"
	case "5m":
		return "5"
	case "15m":
		return "15"
	case "30m":
		return "30"
	case "1h":
		return "60"
	case "4h":
		return "240"
	case "1d":
		return "1440"
	case "1w":
		return "10080"
	default:
		return "60" // Default to 1h
	}
}

// Health returns current adapter health status
func (a *Adapter) Health() facade.HealthStatus {
	now := time.Now()
	errorRate := 0.0
	if a.totalReqs > 0 {
		errorRate = float64(a.errorCount) / float64(a.totalReqs)
	}
	
	status := "healthy"
	recommendation := ""
	
	if errorRate > 0.1 {
		status = "degraded"
		recommendation = "high error rate"
	} else if now.Sub(a.lastSeen) > 30*time.Second {
		status = "degraded"
		recommendation = "connection timeout"
	} else if a.avgLatency > 2*time.Second {
		status = "degraded"
		recommendation = "high latency"
	}
	
	return facade.HealthStatus{
		Venue:        a.name,
		Status:       status,
		LastSeen:     a.lastSeen,
		ErrorRate:    errorRate,
		P99Latency:   a.avgLatency,
		WSConnected:  a.wsConnected,
		RESTHealthy:  errorRate < 0.05,
		Recommendation: recommendation,
	}
}

// WebSocket message handling
func (a *Adapter) handleWebSocketMessages(ctx context.Context) {
	defer func() {
		a.wsConnected = false
		if a.wsConn != nil {
			a.wsConn.Close()
		}
	}()
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := a.wsConn.ReadMessage()
			if err != nil {
				log.Warn().Str("venue", a.name).Err(err).Msg("WebSocket read error")
				return
			}
			
			a.processWebSocketMessage(message)
			a.lastSeen = time.Now()
		}
	}
}

func (a *Adapter) processWebSocketMessage(message []byte) {
	// Parse generic WebSocket message
	var msg interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Debug().Str("venue", a.name).Err(err).Msg("Failed to parse WebSocket message")
		return
	}
	
	// Handle different message types based on Kraken WebSocket format
	// This is a simplified implementation - real implementation would handle all message types
	log.Debug().Str("venue", a.name).RawJSON("message", message).Msg("WebSocket message received")
}

// Helper functions
func parseStringFloat(v interface{}) float64 {
	switch val := v.(type) {
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	case float64:
		return val
	}
	return 0.0
}

func parseFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 0.0
}

// Kraken API response structures
type KrakenOHLCResponse struct {
	Error  []string               `json:"error"`
	Result map[string]interface{} `json:"result"`
}

type KrakenTradesResponse struct {
	Error  []string               `json:"error"`
	Result map[string]interface{} `json:"result"`
}

type KrakenDepthResponse struct {
	Error  []string               `json:"error"`
	Result map[string]interface{} `json:"result"`
}