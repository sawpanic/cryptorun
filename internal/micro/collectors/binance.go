// Binance exchange-native L1/L2 collector with health monitoring
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

	"cryptorun/internal/micro"
)

// BinanceCollector implements exchange-native L1/L2 collection for Binance
type BinanceCollector struct {
	*BaseCollector

	// Binance-specific fields
	httpClient       *http.Client
	lastSequenceNum  int64
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

// BinanceOrderBookResponse represents Binance REST API order book response
type BinanceOrderBookResponse struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}

// BinanceTickerResponse represents Binance ticker data
type BinanceTickerResponse struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
	Time   int64  `json:"time"`
}

// NewBinanceCollector creates a new Binance collector
func NewBinanceCollector(config *micro.CollectorConfig) (*BinanceCollector, error) {
	if config == nil {
		config = micro.DefaultConfig("binance")
	}

	base := NewBaseCollector(config)

	return &BinanceCollector{
		BaseCollector:   base,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		lastWindowStart: time.Now(),
	}, nil
}

// Start begins data collection for Binance
func (bc *BinanceCollector) Start(ctx context.Context) error {
	if err := bc.BaseCollector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start base collector: %w", err)
	}

	// Start Binance-specific monitoring
	bc.wg.Add(1)
	go bc.binanceMonitorWorker()

	return nil
}

// Stop gracefully shuts down the Binance collector
func (bc *BinanceCollector) Stop(ctx context.Context) error {
	return bc.BaseCollector.Stop(ctx)
}

// Subscribe to symbol updates (USD pairs only)
func (bc *BinanceCollector) Subscribe(symbols []string) error {
	bc.subscriptionsMutex.Lock()
	defer bc.subscriptionsMutex.Unlock()

	for _, symbol := range symbols {
		// Validate USD pairs only
		if !bc.isUSDPair(symbol) {
			return fmt.Errorf("non-USD pair not supported: %s (Binance collector only supports USD pairs)", symbol)
		}

		bc.subscriptions[symbol] = true
	}

	return nil
}

// Unsubscribe from symbol updates
func (bc *BinanceCollector) Unsubscribe(symbols []string) error {
	bc.subscriptionsMutex.Lock()
	defer bc.subscriptionsMutex.Unlock()

	for _, symbol := range symbols {
		delete(bc.subscriptions, symbol)
	}

	return nil
}

// binanceMonitorWorker monitors Binance and collects L1/L2 data
func (bc *BinanceCollector) binanceMonitorWorker() {
	defer bc.wg.Done()

	ticker := time.NewTicker(1 * time.Second) // Collect data every second
	defer ticker.Stop()

	for {
		select {
		case <-bc.ctx.Done():
			return
		case <-ticker.C:
			bc.collectData()
		}
	}
}

// collectData collects L1/L2 data for all subscribed symbols
func (bc *BinanceCollector) collectData() {
	bc.subscriptionsMutex.RLock()
	symbols := make([]string, 0, len(bc.subscriptions))
	for symbol := range bc.subscriptions {
		symbols = append(symbols, symbol)
	}
	bc.subscriptionsMutex.RUnlock()

	for _, symbol := range symbols {
		// Collect L1 data
		if l1Data, err := bc.fetchL1Data(symbol); err == nil {
			bc.updateL1Data(symbol, l1Data)
		} else {
			bc.recordError("L1", err)
		}

		// Collect L2 data
		if l2Data, err := bc.fetchL2Data(symbol); err == nil {
			bc.updateL2Data(symbol, l2Data)
		} else {
			bc.recordError("L2", err)
		}

		// Small delay between symbols to respect rate limits
		time.Sleep(100 * time.Millisecond)
	}
}

// fetchL1Data fetches L1 (best bid/ask) data from Binance
func (bc *BinanceCollector) fetchL1Data(symbol string) (*micro.L1Data, error) {
	startTime := time.Now()

	// Convert to Binance format (e.g., BTCUSDT)
	binanceSymbol := bc.convertToBinanceSymbol(symbol)

	// Fetch ticker data for last price
	tickerURL := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", bc.config.BaseURL, binanceSymbol)

	req, err := http.NewRequestWithContext(bc.ctx, "GET", tickerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ticker request: %w", err)
	}

	resp, err := bc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ticker API returned status %d", resp.StatusCode)
	}

	var ticker BinanceTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&ticker); err != nil {
		return nil, fmt.Errorf("failed to decode ticker response: %w", err)
	}

	// Fetch order book for bid/ask
	bookURL := fmt.Sprintf("%s/api/v3/depth?symbol=%s&limit=5", bc.config.BaseURL, binanceSymbol)

	bookReq, err := http.NewRequestWithContext(bc.ctx, "GET", bookURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create book request: %w", err)
	}

	bookResp, err := bc.httpClient.Do(bookReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch book: %w", err)
	}
	defer bookResp.Body.Close()

	if bookResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("book API returned status %d", bookResp.StatusCode)
	}

	var book BinanceOrderBookResponse
	if err := json.NewDecoder(bookResp.Body).Decode(&book); err != nil {
		return nil, fmt.Errorf("failed to decode book response: %w", err)
	}

	// Parse data
	lastPrice, _ := strconv.ParseFloat(ticker.Price, 64)

	var bidPrice, bidSize, askPrice, askSize float64

	if len(book.Bids) > 0 {
		bidPrice, _ = strconv.ParseFloat(book.Bids[0][0], 64)
		bidSize, _ = strconv.ParseFloat(book.Bids[0][1], 64)
	}

	if len(book.Asks) > 0 {
		askPrice, _ = strconv.ParseFloat(book.Asks[0][0], 64)
		askSize, _ = strconv.ParseFloat(book.Asks[0][1], 64)
	}

	// Calculate derived metrics
	spreadBps := bc.calculateSpreadBps(bidPrice, askPrice)
	midPrice := (bidPrice + askPrice) / 2

	latency := time.Since(startTime)
	bc.recordLatency(latency)

	// Assess data quality
	hasCompleteData := bidPrice > 0 && askPrice > 0 && lastPrice > 0
	sequenceGap := bc.checkSequenceGap(book.LastUpdateID)
	quality := bc.assessDataQuality(time.Since(time.Now()), hasCompleteData, sequenceGap)

	return &micro.L1Data{
		Symbol:    symbol,
		Venue:     "binance",
		Timestamp: time.Now(),
		BidPrice:  bidPrice,
		BidSize:   bidSize,
		AskPrice:  askPrice,
		AskSize:   askSize,
		LastPrice: lastPrice,
		SpreadBps: spreadBps,
		MidPrice:  midPrice,
		Sequence:  book.LastUpdateID,
		DataAge:   0, // Will be set by base collector
		Quality:   quality,
	}, nil
}

// fetchL2Data fetches L2 (depth within ±2%) data from Binance
func (bc *BinanceCollector) fetchL2Data(symbol string) (*micro.L2Data, error) {
	startTime := time.Now()

	// Convert to Binance format
	binanceSymbol := bc.convertToBinanceSymbol(symbol)

	// Fetch order book with more depth (limit=100 for better ±2% calculation)
	bookURL := fmt.Sprintf("%s/api/v3/depth?symbol=%s&limit=100", bc.config.BaseURL, binanceSymbol)

	req, err := http.NewRequestWithContext(bc.ctx, "GET", bookURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create book request: %w", err)
	}

	resp, err := bc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch book: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("book API returned status %d", resp.StatusCode)
	}

	var book BinanceOrderBookResponse
	if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
		return nil, fmt.Errorf("failed to decode book response: %w", err)
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
	bidDepthUSD, bidLevels := bc.calculateDepthUSD(book.Bids, midPrice, -0.02) // -2% for bids
	askDepthUSD, askLevels := bc.calculateDepthUSD(book.Asks, midPrice, 0.02)  // +2% for asks

	// Calculate liquidity gradient (depth@0.5% to depth@2% ratio)
	bidDepth05USD, _ := bc.calculateDepthUSD(book.Bids, midPrice, -0.005) // -0.5% for bids
	askDepth05USD, _ := bc.calculateDepthUSD(book.Asks, midPrice, 0.005)  // +0.5% for asks

	totalDepth05 := bidDepth05USD + askDepth05USD
	totalDepth2 := bidDepthUSD + askDepthUSD
	liquidityGradient := bc.calculateLiquidityGradient(totalDepth05, totalDepth2)

	// Calculate VADR inputs (approximated from current data)
	vadrInputVolume := (bidDepthUSD + askDepthUSD) / midPrice // Approximate volume
	vadrInputRange := askPrice - bidPrice                     // Current spread as range approximation

	latency := time.Since(startTime)
	bc.recordLatency(latency)

	// Assess data quality
	hasCompleteData := bidDepthUSD > 0 && askDepthUSD > 0 && bidLevels > 0 && askLevels > 0
	sequenceGap := bc.checkSequenceGap(book.LastUpdateID)
	quality := bc.assessDataQuality(time.Since(time.Now()), hasCompleteData, sequenceGap)

	// Check if USD quote
	isUSDQuote := bc.isUSDPair(symbol)

	return &micro.L2Data{
		Symbol:            symbol,
		Venue:             "binance",
		Timestamp:         time.Now(),
		BidDepthUSD:       bidDepthUSD,
		AskDepthUSD:       askDepthUSD,
		TotalDepthUSD:     bidDepthUSD + askDepthUSD,
		BidLevels:         bidLevels,
		AskLevels:         askLevels,
		LiquidityGradient: liquidityGradient,
		VADRInputVolume:   vadrInputVolume,
		VADRInputRange:    vadrInputRange,
		Sequence:          book.LastUpdateID,
		DataAge:           0, // Will be set by base collector
		Quality:           quality,
		IsUSDQuote:        isUSDQuote,
	}, nil
}

// calculateDepthUSD calculates total USD depth within a percentage range
func (bc *BinanceCollector) calculateDepthUSD(levels [][]string, midPrice float64, pctRange float64) (float64, int) {
	if len(levels) == 0 || midPrice <= 0 {
		return 0, 0
	}

	targetPrice := midPrice * (1 + pctRange)
	totalDepth := 0.0
	levelCount := 0

	for _, level := range levels {
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

// convertToBinanceSymbol converts standard symbol to Binance format
func (bc *BinanceCollector) convertToBinanceSymbol(symbol string) string {
	// Convert BTC/USD to BTCUSDT, ETH/USD to ETHUSDT, etc.
	parts := strings.Split(symbol, "/")
	if len(parts) != 2 {
		return symbol // Return as-is if not in expected format
	}

	base := strings.ToUpper(parts[0])
	quote := strings.ToUpper(parts[1])

	// Binance typically uses USDT instead of USD
	if quote == "USD" {
		quote = "USDT"
	}

	return base + quote
}

// isUSDPair checks if the symbol is a USD pair
func (bc *BinanceCollector) isUSDPair(symbol string) bool {
	return strings.HasSuffix(strings.ToUpper(symbol), "/USD") ||
		strings.HasSuffix(strings.ToUpper(symbol), "USD") ||
		strings.HasSuffix(strings.ToUpper(symbol), "/USDT") ||
		strings.HasSuffix(strings.ToUpper(symbol), "USDT")
}

// checkSequenceGap checks for sequence number gaps
func (bc *BinanceCollector) checkSequenceGap(currentSeq int64) bool {
	bc.statsMutex.Lock()
	defer bc.statsMutex.Unlock()

	if bc.lastSequenceNum > 0 && currentSeq > bc.lastSequenceNum+1 {
		bc.sequenceGaps++
		bc.lastSequenceNum = currentSeq
		return true
	}

	bc.lastSequenceNum = currentSeq
	return false
}

// recordLatency records API call latency
func (bc *BinanceCollector) recordLatency(latency time.Duration) {
	bc.statsMutex.Lock()
	defer bc.statsMutex.Unlock()

	bc.windowLatencySum += latency
	if latency > bc.windowLatencyMax {
		bc.windowLatencyMax = latency
	}
	bc.windowMessageCount++
}

// recordError records an error occurrence
func (bc *BinanceCollector) recordError(operation string, err error) {
	bc.statsMutex.Lock()
	defer bc.statsMutex.Unlock()

	bc.windowErrorCount++
	bc.errorCount++

	// Log the error (in production, this would go to proper logging)
	fmt.Printf("Binance %s error: %v\n", operation, err)
}

// updateMetricsWindow overrides base implementation with Binance-specific metrics
func (bc *BinanceCollector) updateMetricsWindow(windowStart, windowEnd time.Time) {
	bc.statsMutex.Lock()

	windowDuration := windowEnd.Sub(windowStart)

	var avgLatencyMs float64
	if bc.windowMessageCount > 0 {
		avgLatencyMs = float64(bc.windowLatencySum.Milliseconds()) / float64(bc.windowMessageCount)
	}

	maxLatencyMs := bc.windowLatencyMax.Milliseconds()

	// Calculate quality score based on errors and latency
	qualityScore := 100.0
	if bc.windowMessageCount > 0 {
		errorRate := float64(bc.windowErrorCount) / float64(bc.windowMessageCount)
		qualityScore -= errorRate * 50 // Reduce by up to 50 points for errors
	}

	if avgLatencyMs > 1000 { // Penalize high latency
		qualityScore -= (avgLatencyMs - 1000) / 100
	}

	if qualityScore < 0 {
		qualityScore = 0
	}

	metrics := &micro.CollectorMetrics{
		Venue:            "binance",
		WindowStart:      windowStart,
		WindowEnd:        windowEnd,
		L1Messages:       bc.windowMessageCount, // Approximation: each fetch counts as message
		L2Messages:       bc.windowMessageCount,
		ErrorMessages:    bc.windowErrorCount,
		ProcessingTimeMs: windowDuration.Milliseconds(),
		AvgLatencyMs:     avgLatencyMs,
		MaxLatencyMs:     maxLatencyMs,
		StaleDataCount:   0,                   // Would need timestamp analysis to calculate properly
		IncompleteCount:  bc.windowErrorCount, // Approximation: errors often mean incomplete data
		QualityScore:     qualityScore,
	}

	// Reset window counters
	bc.windowMessageCount = 0
	bc.windowErrorCount = 0
	bc.windowLatencySum = 0
	bc.windowLatencyMax = 0
	bc.lastWindowStart = windowEnd

	bc.statsMutex.Unlock()

	bc.updateMetrics(metrics)
}

// updateRollingHealthStats overrides base implementation with Binance-specific health
func (bc *BinanceCollector) updateRollingHealthStats() {
	bc.statsMutex.Lock()

	now := time.Now()

	// Calculate error rate over last 60 seconds (approximated)
	totalMessages := bc.messageCount
	errorRate := 0.0
	if totalMessages > 0 {
		errorRate = float64(bc.errorCount) / float64(totalMessages)
	}

	// Calculate average latency
	avgLatency := time.Duration(0)
	if bc.messageCount > 0 {
		avgLatency = bc.latencySum / time.Duration(bc.messageCount)
	}

	// Determine health status
	status := micro.HealthGreen
	healthy := true
	recommendation := "proceed"

	if errorRate > bc.config.MaxErrorRate {
		status = micro.HealthRed
		healthy = false
		recommendation = "avoid"
	} else if avgLatency.Milliseconds() > bc.config.MaxLatencyP99Ms {
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
		Venue:            "binance",
		Timestamp:        now,
		Status:           status,
		Healthy:          healthy,
		Uptime:           uptime,
		HeartbeatAgeMs:   int64(dataFreshness / time.Millisecond),
		MessageGapRate:   float64(bc.sequenceGaps) / float64(bc.messageCount),
		WSReconnectCount: bc.wsReconnectCount,
		LatencyP50Ms:     avgLatency.Milliseconds() / 2,    // Approximation
		LatencyP99Ms:     int64(avgLatency.Milliseconds()), // Approximation
		ErrorRate:        errorRate,
		DataFreshness:    dataFreshness,
		DataCompleteness: 100.0 - (errorRate * 100),
		Recommendation:   recommendation,
	}

	bc.statsMutex.Unlock()

	bc.updateVenueHealth(health)
}
