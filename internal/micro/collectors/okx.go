// OKX exchange-native L1/L2 collector with health monitoring
package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/internal/micro"
)

// OKXCollector implements exchange-native L1/L2 collection for OKX
type OKXCollector struct {
	*BaseCollector

	// OKX-specific fields
	httpClient       *http.Client
	lastSequenceNum  string
	wsReconnectCount int
	messageCount     int64
	errorCount       int64
	latencySum       time.Duration
	latencyMax       time.Duration
	sequenceGaps     int64

	// Performance tracking
	lastWindowStart    time.Time
	windowMessageCount int64
	windowErrorCount   int64
	windowLatencySum   time.Duration
	windowLatencyMax   time.Duration

	// Mutex for stats
	statsMutex sync.Mutex
}

// OKXResponse represents the base OKX API response structure
type OKXResponse struct {
	Code string      `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// OKXOrderBookData represents OKX order book data
type OKXOrderBookData struct {
	Asks [][]string `json:"asks"` // [price, size, liquidated_orders, order_count]
	Bids [][]string `json:"bids"`
	TS   string     `json:"ts"` // Timestamp
}

// OKXTickerData represents OKX ticker data
type OKXTickerData struct {
	InstID  string `json:"instId"`  // Instrument ID
	Last    string `json:"last"`    // Last price
	LastSz  string `json:"lastSz"`  // Last size
	AskPx   string `json:"askPx"`   // Ask price
	AskSz   string `json:"askSz"`   // Ask size
	BidPx   string `json:"bidPx"`   // Bid price
	BidSz   string `json:"bidSz"`   // Bid size
	Open24h string `json:"open24h"` // 24h open
	High24h string `json:"high24h"` // 24h high
	Low24h  string `json:"low24h"`  // 24h low
	TS      string `json:"ts"`      // Timestamp
}

// NewOKXCollector creates a new OKX collector
func NewOKXCollector(config *micro.CollectorConfig) (*OKXCollector, error) {
	if config == nil {
		config = micro.DefaultConfig("okx")
	}

	base := NewBaseCollector(config)

	return &OKXCollector{
		BaseCollector:   base,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		lastWindowStart: time.Now(),
	}, nil
}

// Start begins data collection for OKX
func (oc *OKXCollector) Start(ctx context.Context) error {
	if err := oc.BaseCollector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start base collector: %w", err)
	}

	// Start OKX-specific monitoring
	oc.wg.Add(1)
	go oc.okxMonitorWorker()

	return nil
}

// Stop gracefully shuts down the OKX collector
func (oc *OKXCollector) Stop(ctx context.Context) error {
	return oc.BaseCollector.Stop(ctx)
}

// Subscribe to symbol updates (USD pairs only)
func (oc *OKXCollector) Subscribe(symbols []string) error {
	oc.subscriptionsMutex.Lock()
	defer oc.subscriptionsMutex.Unlock()

	for _, symbol := range symbols {
		// Validate USD pairs only
		if !oc.isUSDPair(symbol) {
			return fmt.Errorf("non-USD pair not supported: %s (OKX collector only supports USD pairs)", symbol)
		}

		oc.subscriptions[symbol] = true
	}

	return nil
}

// Unsubscribe from symbol updates
func (oc *OKXCollector) Unsubscribe(symbols []string) error {
	oc.subscriptionsMutex.Lock()
	defer oc.subscriptionsMutex.Unlock()

	for _, symbol := range symbols {
		delete(oc.subscriptions, symbol)
	}

	return nil
}

// okxMonitorWorker monitors OKX and collects L1/L2 data
func (oc *OKXCollector) okxMonitorWorker() {
	defer oc.wg.Done()

	ticker := time.NewTicker(1 * time.Second) // Collect data every second
	defer ticker.Stop()

	for {
		select {
		case <-oc.ctx.Done():
			return
		case <-ticker.C:
			oc.collectData()
		}
	}
}

// collectData collects L1/L2 data for all subscribed symbols
func (oc *OKXCollector) collectData() {
	oc.subscriptionsMutex.RLock()
	symbols := make([]string, 0, len(oc.subscriptions))
	for symbol := range oc.subscriptions {
		symbols = append(symbols, symbol)
	}
	oc.subscriptionsMutex.RUnlock()

	for _, symbol := range symbols {
		// Collect L1 data
		if l1Data, err := oc.fetchL1Data(symbol); err == nil {
			oc.updateL1Data(symbol, l1Data)
		} else {
			oc.recordError("L1", err)
		}

		// Collect L2 data
		if l2Data, err := oc.fetchL2Data(symbol); err == nil {
			oc.updateL2Data(symbol, l2Data)
		} else {
			oc.recordError("L2", err)
		}

		// Small delay between symbols to respect rate limits
		time.Sleep(100 * time.Millisecond)
	}
}

// fetchL1Data fetches L1 (best bid/ask) data from OKX
func (oc *OKXCollector) fetchL1Data(symbol string) (*micro.L1Data, error) {
	startTime := time.Now()

	// Convert to OKX format (e.g., BTC-USDT)
	okxSymbol := oc.convertToOKXSymbol(symbol)

	// Fetch ticker data which includes bid/ask and last price
	tickerURL := fmt.Sprintf("%s/api/v5/market/ticker?instId=%s", oc.config.BaseURL, okxSymbol)

	req, err := http.NewRequestWithContext(oc.ctx, "GET", tickerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ticker request: %w", err)
	}

	resp, err := oc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ticker API returned status %d", resp.StatusCode)
	}

	var response OKXResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode ticker response: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("OKX API error: %s - %s", response.Code, response.Msg)
	}

	// Parse the data array
	dataArray, ok := response.Data.([]interface{})
	if !ok || len(dataArray) == 0 {
		return nil, fmt.Errorf("invalid ticker data format")
	}

	// Convert to OKXTickerData
	dataBytes, err := json.Marshal(dataArray[0])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ticker data: %w", err)
	}

	var ticker OKXTickerData
	if err := json.Unmarshal(dataBytes, &ticker); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ticker data: %w", err)
	}

	// Parse numeric values
	bidPrice, _ := strconv.ParseFloat(ticker.BidPx, 64)
	bidSize, _ := strconv.ParseFloat(ticker.BidSz, 64)
	askPrice, _ := strconv.ParseFloat(ticker.AskPx, 64)
	askSize, _ := strconv.ParseFloat(ticker.AskSz, 64)
	lastPrice, _ := strconv.ParseFloat(ticker.Last, 64)

	// Calculate derived metrics
	spreadBps := oc.calculateSpreadBps(bidPrice, askPrice)
	midPrice := (bidPrice + askPrice) / 2

	latency := time.Since(startTime)
	oc.recordLatency(latency)

	// Assess data quality
	hasCompleteData := bidPrice > 0 && askPrice > 0 && lastPrice > 0
	sequenceGap := oc.checkSequenceGap(ticker.TS)
	quality := oc.assessDataQuality(time.Since(time.Now()), hasCompleteData, sequenceGap)

	// Parse timestamp
	ts, _ := strconv.ParseInt(ticker.TS, 10, 64)
	timestamp := time.Unix(ts/1000, (ts%1000)*1000000) // OKX uses milliseconds

	return &micro.L1Data{
		Symbol:    symbol,
		Venue:     "okx",
		Timestamp: timestamp,
		BidPrice:  bidPrice,
		BidSize:   bidSize,
		AskPrice:  askPrice,
		AskSize:   askSize,
		LastPrice: lastPrice,
		SpreadBps: spreadBps,
		MidPrice:  midPrice,
		Sequence:  ts, // Use timestamp as sequence
		DataAge:   0,  // Will be set by base collector
		Quality:   quality,
	}, nil
}

// fetchL2Data fetches L2 (depth within ±2%) data from OKX
func (oc *OKXCollector) fetchL2Data(symbol string) (*micro.L2Data, error) {
	startTime := time.Now()

	// Convert to OKX format
	okxSymbol := oc.convertToOKXSymbol(symbol)

	// Fetch order book with depth (sz parameter for depth level)
	bookURL := fmt.Sprintf("%s/api/v5/market/books?instId=%s&sz=100", oc.config.BaseURL, okxSymbol)

	req, err := http.NewRequestWithContext(oc.ctx, "GET", bookURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create book request: %w", err)
	}

	resp, err := oc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch book: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("book API returned status %d", resp.StatusCode)
	}

	var response OKXResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode book response: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("OKX API error: %s - %s", response.Code, response.Msg)
	}

	// Parse the data array
	dataArray, ok := response.Data.([]interface{})
	if !ok || len(dataArray) == 0 {
		return nil, fmt.Errorf("invalid book data format")
	}

	// Convert to OKXOrderBookData
	dataBytes, err := json.Marshal(dataArray[0])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal book data: %w", err)
	}

	var book OKXOrderBookData
	if err := json.Unmarshal(dataBytes, &book); err != nil {
		return nil, fmt.Errorf("failed to unmarshal book data: %w", err)
	}

	// Calculate mid price for ±2% calculation
	var bidPrice, askPrice float64
	if len(book.Bids) > 0 && len(book.Asks) > 0 {
		bidPrice, _ = strconv.ParseFloat(book.Bids[0][0], 64)
		askPrice, _ = strconv.ParseFloat(book.Asks[0][0], 64)
	}

	if bidPrice <= 0 || askPrice <= 0 {
		return nil, fmt.Errorf("invalid bid/ask prices")
	}

	midPrice := (bidPrice + askPrice) / 2

	// Calculate depth within ±2%
	bidDepthUSD, bidLevels := oc.calculateDepthUSD(book.Bids, midPrice, -0.02) // -2% for bids
	askDepthUSD, askLevels := oc.calculateDepthUSD(book.Asks, midPrice, 0.02)  // +2% for asks

	// Calculate liquidity gradient (depth@0.5% to depth@2% ratio)
	bidDepth05USD, _ := oc.calculateDepthUSD(book.Bids, midPrice, -0.005) // -0.5% for bids
	askDepth05USD, _ := oc.calculateDepthUSD(book.Asks, midPrice, 0.005)  // +0.5% for asks

	totalDepth05 := bidDepth05USD + askDepth05USD
	totalDepth2 := bidDepthUSD + askDepthUSD
	liquidityGradient := oc.calculateLiquidityGradient(totalDepth05, totalDepth2)

	// Calculate VADR inputs (approximated from current data)
	vadrInputVolume := (bidDepthUSD + askDepthUSD) / midPrice // Approximate volume
	vadrInputRange := askPrice - bidPrice                     // Current spread as range approximation

	latency := time.Since(startTime)
	oc.recordLatency(latency)

	// Assess data quality
	hasCompleteData := bidDepthUSD > 0 && askDepthUSD > 0 && bidLevels > 0 && askLevels > 0
	sequenceGap := oc.checkSequenceGap(book.TS)
	quality := oc.assessDataQuality(time.Since(time.Now()), hasCompleteData, sequenceGap)

	// Check if USD quote
	isUSDQuote := oc.isUSDPair(symbol)

	// Parse timestamp
	ts, _ := strconv.ParseInt(book.TS, 10, 64)
	timestamp := time.Unix(ts/1000, (ts%1000)*1000000) // OKX uses milliseconds

	return &micro.L2Data{
		Symbol:            symbol,
		Venue:             "okx",
		Timestamp:         timestamp,
		BidDepthUSD:       bidDepthUSD,
		AskDepthUSD:       askDepthUSD,
		TotalDepthUSD:     bidDepthUSD + askDepthUSD,
		BidLevels:         bidLevels,
		AskLevels:         askLevels,
		LiquidityGradient: liquidityGradient,
		VADRInputVolume:   vadrInputVolume,
		VADRInputRange:    vadrInputRange,
		Sequence:          ts, // Use timestamp as sequence
		DataAge:           0,  // Will be set by base collector
		Quality:           quality,
		IsUSDQuote:        isUSDQuote,
	}, nil
}

// calculateDepthUSD calculates total USD depth within a percentage range
func (oc *OKXCollector) calculateDepthUSD(levels [][]string, midPrice float64, pctRange float64) (float64, int) {
	if len(levels) == 0 || midPrice <= 0 {
		return 0, 0
	}

	targetPrice := midPrice * (1 + pctRange)
	totalDepth := 0.0
	levelCount := 0

	for _, level := range levels {
		if len(level) < 2 {
			continue
		}

		price, _ := strconv.ParseFloat(level[0], 64)
		size, _ := strconv.ParseFloat(level[1], 64)

		// Check if price is within range
		var withinRange bool
		if pctRange < 0 { // Bids (price should be >= target)
			withinRange = price >= targetPrice
		} else { // Asks (price should be <= target)
			withinRange = price <= targetPrice
		}

		if withinRange {
			totalDepth += price * size // USD value
			levelCount++
		} else {
			break // Levels are sorted, so we can break early
		}
	}

	return totalDepth, levelCount
}

// convertToOKXSymbol converts standard symbol to OKX format
func (oc *OKXCollector) convertToOKXSymbol(symbol string) string {
	// Convert BTC/USD to BTC-USDT, ETH/USD to ETH-USDT, etc.
	parts := strings.Split(symbol, "/")
	if len(parts) != 2 {
		return symbol // Return as-is if not in expected format
	}

	base := strings.ToUpper(parts[0])
	quote := strings.ToUpper(parts[1])

	// OKX typically uses USDT instead of USD for spot trading
	if quote == "USD" {
		quote = "USDT"
	}

	return base + "-" + quote
}

// isUSDPair checks if the symbol is a USD pair
func (oc *OKXCollector) isUSDPair(symbol string) bool {
	return strings.HasSuffix(strings.ToUpper(symbol), "/USD") ||
		strings.HasSuffix(strings.ToUpper(symbol), "USD") ||
		strings.HasSuffix(strings.ToUpper(symbol), "/USDT") ||
		strings.HasSuffix(strings.ToUpper(symbol), "-USDT")
}

// checkSequenceGap checks for sequence number gaps (using timestamp for OKX)
func (oc *OKXCollector) checkSequenceGap(currentTSStr string) bool {
	oc.statsMutex.Lock()
	defer oc.statsMutex.Unlock()

	if oc.lastSequenceNum != "" && currentTSStr != "" {
		currentTS, _ := strconv.ParseInt(currentTSStr, 10, 64)
		lastTS, _ := strconv.ParseInt(oc.lastSequenceNum, 10, 64)

		// Consider a gap if timestamp difference is > 5 seconds
		if currentTS > lastTS+5000 { // 5000ms = 5s
			oc.sequenceGaps++
			oc.lastSequenceNum = currentTSStr
			return true
		}
	}

	oc.lastSequenceNum = currentTSStr
	return false
}

// recordLatency records API call latency
func (oc *OKXCollector) recordLatency(latency time.Duration) {
	oc.statsMutex.Lock()
	defer oc.statsMutex.Unlock()

	oc.windowLatencySum += latency
	if latency > oc.windowLatencyMax {
		oc.windowLatencyMax = latency
	}
	oc.windowMessageCount++
}

// recordError records an error occurrence
func (oc *OKXCollector) recordError(operation string, err error) {
	oc.statsMutex.Lock()
	defer oc.statsMutex.Unlock()

	oc.windowErrorCount++
	oc.errorCount++

	// Log the error (in production, this would go to proper logging)
	fmt.Printf("OKX %s error: %v\n", operation, err)
}

// updateMetricsWindow overrides base implementation with OKX-specific metrics
func (oc *OKXCollector) updateMetricsWindow(windowStart, windowEnd time.Time) {
	oc.statsMutex.Lock()

	windowDuration := windowEnd.Sub(windowStart)

	var avgLatencyMs float64
	if oc.windowMessageCount > 0 {
		avgLatencyMs = float64(oc.windowLatencySum.Milliseconds()) / float64(oc.windowMessageCount)
	}

	maxLatencyMs := oc.windowLatencyMax.Milliseconds()

	// Calculate quality score based on errors and latency
	qualityScore := 100.0
	if oc.windowMessageCount > 0 {
		errorRate := float64(oc.windowErrorCount) / float64(oc.windowMessageCount)
		qualityScore -= errorRate * 50 // Reduce by up to 50 points for errors
	}

	if avgLatencyMs > 1000 { // Penalize high latency
		qualityScore -= (avgLatencyMs - 1000) / 100
	}

	if qualityScore < 0 {
		qualityScore = 0
	}

	metrics := &micro.CollectorMetrics{
		Venue:            "okx",
		WindowStart:      windowStart,
		WindowEnd:        windowEnd,
		L1Messages:       oc.windowMessageCount, // Approximation: each fetch counts as message
		L2Messages:       oc.windowMessageCount,
		ErrorMessages:    oc.windowErrorCount,
		ProcessingTimeMs: windowDuration.Milliseconds(),
		AvgLatencyMs:     avgLatencyMs,
		MaxLatencyMs:     maxLatencyMs,
		StaleDataCount:   0,                   // Would need timestamp analysis to calculate properly
		IncompleteCount:  oc.windowErrorCount, // Approximation: errors often mean incomplete data
		QualityScore:     qualityScore,
	}

	// Reset window counters
	oc.windowMessageCount = 0
	oc.windowErrorCount = 0
	oc.windowLatencySum = 0
	oc.windowLatencyMax = 0
	oc.lastWindowStart = windowEnd

	oc.statsMutex.Unlock()

	oc.updateMetrics(metrics)
}

// updateRollingHealthStats overrides base implementation with OKX-specific health
func (oc *OKXCollector) updateRollingHealthStats() {
	oc.statsMutex.Lock()

	now := time.Now()

	// Calculate error rate over last 60 seconds (approximated)
	totalMessages := oc.messageCount
	errorRate := 0.0
	if totalMessages > 0 {
		errorRate = float64(oc.errorCount) / float64(totalMessages)
	}

	// Calculate average latency
	avgLatency := time.Duration(0)
	if oc.messageCount > 0 {
		avgLatency = oc.latencySum / time.Duration(oc.messageCount)
	}

	// Determine health status
	status := micro.HealthGreen
	healthy := true
	recommendation := "proceed"

	if errorRate > oc.config.MaxErrorRate {
		status = micro.HealthRed
		healthy = false
		recommendation = "avoid"
	} else if avgLatency.Milliseconds() > oc.config.MaxLatencyP99Ms {
		status = micro.HealthYellow
		recommendation = "halve_size"
	}

	// Calculate uptime (simplified - assume healthy means up)
	uptime := 100.0
	if !healthy {
		uptime = 85.0 // Approximation for unhealthy state
	}

	// Data freshness (approximated based on last successful fetch)
	dataFreshness := 2 * time.Second // Approximation

	health := &micro.VenueHealth{
		Venue:            "okx",
		Timestamp:        now,
		Status:           status,
		Healthy:          healthy,
		Uptime:           uptime,
		HeartbeatAgeMs:   int64(dataFreshness / time.Millisecond),
		MessageGapRate:   float64(oc.sequenceGaps) / float64(oc.messageCount),
		WSReconnectCount: oc.wsReconnectCount,
		LatencyP50Ms:     avgLatency.Milliseconds() / 2,    // Approximation
		LatencyP99Ms:     int64(avgLatency.Milliseconds()), // Approximation
		ErrorRate:        errorRate,
		DataFreshness:    dataFreshness,
		DataCompleteness: 100.0 - (errorRate * 100),
		Recommendation:   recommendation,
	}

	oc.statsMutex.Unlock()

	oc.updateVenueHealth(health)
}
