// Coinbase exchange-native L1/L2 collector with health monitoring
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

// CoinbaseCollector implements exchange-native L1/L2 collection for Coinbase
type CoinbaseCollector struct {
	*BaseCollector

	// Coinbase-specific fields
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

// CoinbaseOrderBookResponse represents Coinbase order book data
type CoinbaseOrderBookResponse struct {
	Sequence int64      `json:"sequence"`
	Bids     [][]string `json:"bids"` // [price, size, num_orders]
	Asks     [][]string `json:"asks"`
}

// CoinbaseTickerResponse represents Coinbase ticker data
type CoinbaseTickerResponse struct {
	Ask     string    `json:"ask"`
	Bid     string    `json:"bid"`
	Price   string    `json:"price"`
	Size    string    `json:"size"`
	Time    time.Time `json:"time"`
	TradeID int64     `json:"trade_id"`
	Volume  string    `json:"volume"`
}

// CoinbaseStatsResponse represents Coinbase 24h stats
type CoinbaseStatsResponse struct {
	Open        string `json:"open"`         // Opening price
	High        string `json:"high"`         // Highest price
	Low         string `json:"low"`          // Lowest price
	Volume      string `json:"volume"`       // Volume
	Last        string `json:"last"`         // Last price
	Volume30Day string `json:"volume_30day"` // 30 day volume
}

// NewCoinbaseCollector creates a new Coinbase collector
func NewCoinbaseCollector(config *micro.CollectorConfig) (*CoinbaseCollector, error) {
	if config == nil {
		config = micro.DefaultConfig("coinbase")
	}

	base := NewBaseCollector(config)

	return &CoinbaseCollector{
		BaseCollector:   base,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		lastWindowStart: time.Now(),
	}, nil
}

// Start begins data collection for Coinbase
func (cc *CoinbaseCollector) Start(ctx context.Context) error {
	if err := cc.BaseCollector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start base collector: %w", err)
	}

	// Start Coinbase-specific monitoring
	cc.wg.Add(1)
	go cc.coinbaseMonitorWorker()

	return nil
}

// Stop gracefully shuts down the Coinbase collector
func (cc *CoinbaseCollector) Stop(ctx context.Context) error {
	return cc.BaseCollector.Stop(ctx)
}

// Subscribe to symbol updates (USD pairs only)
func (cc *CoinbaseCollector) Subscribe(symbols []string) error {
	cc.subscriptionsMutex.Lock()
	defer cc.subscriptionsMutex.Unlock()

	for _, symbol := range symbols {
		// Validate USD pairs only
		if !cc.isUSDPair(symbol) {
			return fmt.Errorf("non-USD pair not supported: %s (Coinbase collector only supports USD pairs)", symbol)
		}

		cc.subscriptions[symbol] = true
	}

	return nil
}

// Unsubscribe from symbol updates
func (cc *CoinbaseCollector) Unsubscribe(symbols []string) error {
	cc.subscriptionsMutex.Lock()
	defer cc.subscriptionsMutex.Unlock()

	for _, symbol := range symbols {
		delete(cc.subscriptions, symbol)
	}

	return nil
}

// coinbaseMonitorWorker monitors Coinbase and collects L1/L2 data
func (cc *CoinbaseCollector) coinbaseMonitorWorker() {
	defer cc.wg.Done()

	ticker := time.NewTicker(1 * time.Second) // Collect data every second
	defer ticker.Stop()

	for {
		select {
		case <-cc.ctx.Done():
			return
		case <-ticker.C:
			cc.collectData()
		}
	}
}

// collectData collects L1/L2 data for all subscribed symbols
func (cc *CoinbaseCollector) collectData() {
	cc.subscriptionsMutex.RLock()
	symbols := make([]string, 0, len(cc.subscriptions))
	for symbol := range cc.subscriptions {
		symbols = append(symbols, symbol)
	}
	cc.subscriptionsMutex.RUnlock()

	for _, symbol := range symbols {
		// Collect L1 data
		if l1Data, err := cc.fetchL1Data(symbol); err == nil {
			cc.updateL1Data(symbol, l1Data)
		} else {
			cc.recordError("L1", err)
		}

		// Collect L2 data
		if l2Data, err := cc.fetchL2Data(symbol); err == nil {
			cc.updateL2Data(symbol, l2Data)
		} else {
			cc.recordError("L2", err)
		}

		// Small delay between symbols to respect rate limits
		time.Sleep(100 * time.Millisecond)
	}
}

// fetchL1Data fetches L1 (best bid/ask) data from Coinbase
func (cc *CoinbaseCollector) fetchL1Data(symbol string) (*micro.L1Data, error) {
	startTime := time.Now()

	// Convert to Coinbase format (e.g., BTC-USD)
	coinbaseSymbol := cc.convertToCoinbaseSymbol(symbol)

	// Fetch ticker data which includes bid/ask and last price
	tickerURL := fmt.Sprintf("%s/products/%s/ticker", cc.config.BaseURL, coinbaseSymbol)

	req, err := http.NewRequestWithContext(cc.ctx, "GET", tickerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ticker request: %w", err)
	}

	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ticker API returned status %d", resp.StatusCode)
	}

	var ticker CoinbaseTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&ticker); err != nil {
		return nil, fmt.Errorf("failed to decode ticker response: %w", err)
	}

	// Parse numeric values
	bidPrice, _ := strconv.ParseFloat(ticker.Bid, 64)
	askPrice, _ := strconv.ParseFloat(ticker.Ask, 64)
	lastPrice, _ := strconv.ParseFloat(ticker.Price, 64)
	lastSize, _ := strconv.ParseFloat(ticker.Size, 64)

	// For bid/ask sizes, we need to get them from the order book since ticker doesn't include them
	var bidSize, askSize float64
	if bookData, err := cc.fetchOrderBookSnapshot(coinbaseSymbol); err == nil {
		if len(bookData.Bids) > 0 {
			bidSize, _ = strconv.ParseFloat(bookData.Bids[0][1], 64)
		}
		if len(bookData.Asks) > 0 {
			askSize, _ = strconv.ParseFloat(bookData.Asks[0][1], 64)
		}

		// Update sequence number from order book
		cc.checkSequenceGap(bookData.Sequence)
	} else {
		// Fallback: use last size as approximation
		bidSize = lastSize
		askSize = lastSize
	}

	// Calculate derived metrics
	spreadBps := cc.calculateSpreadBps(bidPrice, askPrice)
	midPrice := (bidPrice + askPrice) / 2

	latency := time.Since(startTime)
	cc.recordLatency(latency)

	// Assess data quality
	hasCompleteData := bidPrice > 0 && askPrice > 0 && lastPrice > 0
	quality := cc.assessDataQuality(time.Since(ticker.Time), hasCompleteData, false)

	return &micro.L1Data{
		Symbol:    symbol,
		Venue:     "coinbase",
		Timestamp: ticker.Time,
		BidPrice:  bidPrice,
		BidSize:   bidSize,
		AskPrice:  askPrice,
		AskSize:   askSize,
		LastPrice: lastPrice,
		SpreadBps: spreadBps,
		MidPrice:  midPrice,
		Sequence:  ticker.TradeID, // Use trade ID as sequence
		DataAge:   0,              // Will be set by base collector
		Quality:   quality,
	}, nil
}

// fetchL2Data fetches L2 (depth within ±2%) data from Coinbase
func (cc *CoinbaseCollector) fetchL2Data(symbol string) (*micro.L2Data, error) {
	startTime := time.Now()

	// Convert to Coinbase format
	coinbaseSymbol := cc.convertToCoinbaseSymbol(symbol)

	// Fetch order book with level 2 data
	bookData, err := cc.fetchOrderBookSnapshot(coinbaseSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order book: %w", err)
	}

	// Calculate mid price for ±2% calculation
	var bidPrice, askPrice float64
	if len(bookData.Bids) > 0 && len(bookData.Asks) > 0 {
		bidPrice, _ = strconv.ParseFloat(bookData.Bids[0][0], 64)
		askPrice, _ = strconv.ParseFloat(bookData.Asks[0][0], 64)
	}

	if bidPrice <= 0 || askPrice <= 0 {
		return nil, fmt.Errorf("invalid bid/ask prices")
	}

	midPrice := (bidPrice + askPrice) / 2

	// Calculate depth within ±2%
	bidDepthUSD, bidLevels := cc.calculateDepthUSD(bookData.Bids, midPrice, -0.02) // -2% for bids
	askDepthUSD, askLevels := cc.calculateDepthUSD(bookData.Asks, midPrice, 0.02)  // +2% for asks

	// Calculate liquidity gradient (depth@0.5% to depth@2% ratio)
	bidDepth05USD, _ := cc.calculateDepthUSD(bookData.Bids, midPrice, -0.005) // -0.5% for bids
	askDepth05USD, _ := cc.calculateDepthUSD(bookData.Asks, midPrice, 0.005)  // +0.5% for asks

	totalDepth05 := bidDepth05USD + askDepth05USD
	totalDepth2 := bidDepthUSD + askDepthUSD
	liquidityGradient := cc.calculateLiquidityGradient(totalDepth05, totalDepth2)

	// Calculate VADR inputs (approximated from current data)
	vadrInputVolume := (bidDepthUSD + askDepthUSD) / midPrice // Approximate volume
	vadrInputRange := askPrice - bidPrice                     // Current spread as range approximation

	latency := time.Since(startTime)
	cc.recordLatency(latency)

	// Assess data quality
	hasCompleteData := bidDepthUSD > 0 && askDepthUSD > 0 && bidLevels > 0 && askLevels > 0
	sequenceGap := cc.checkSequenceGap(bookData.Sequence)
	quality := cc.assessDataQuality(time.Since(time.Now()), hasCompleteData, sequenceGap)

	// Check if USD quote
	isUSDQuote := cc.isUSDPair(symbol)

	return &micro.L2Data{
		Symbol:            symbol,
		Venue:             "coinbase",
		Timestamp:         time.Now(),
		BidDepthUSD:       bidDepthUSD,
		AskDepthUSD:       askDepthUSD,
		TotalDepthUSD:     bidDepthUSD + askDepthUSD,
		BidLevels:         bidLevels,
		AskLevels:         askLevels,
		LiquidityGradient: liquidityGradient,
		VADRInputVolume:   vadrInputVolume,
		VADRInputRange:    vadrInputRange,
		Sequence:          bookData.Sequence,
		DataAge:           0, // Will be set by base collector
		Quality:           quality,
		IsUSDQuote:        isUSDQuote,
	}, nil
}

// fetchOrderBookSnapshot fetches order book snapshot from Coinbase
func (cc *CoinbaseCollector) fetchOrderBookSnapshot(coinbaseSymbol string) (*CoinbaseOrderBookResponse, error) {
	// Fetch order book with level 2 (full depth)
	bookURL := fmt.Sprintf("%s/products/%s/book?level=2", cc.config.BaseURL, coinbaseSymbol)

	req, err := http.NewRequestWithContext(cc.ctx, "GET", bookURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create book request: %w", err)
	}

	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch book: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("book API returned status %d", resp.StatusCode)
	}

	var book CoinbaseOrderBookResponse
	if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
		return nil, fmt.Errorf("failed to decode book response: %w", err)
	}

	return &book, nil
}

// calculateDepthUSD calculates total USD depth within a percentage range
func (cc *CoinbaseCollector) calculateDepthUSD(levels [][]string, midPrice float64, pctRange float64) (float64, int) {
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

// convertToCoinbaseSymbol converts standard symbol to Coinbase format
func (cc *CoinbaseCollector) convertToCoinbaseSymbol(symbol string) string {
	// Convert BTC/USD to BTC-USD, ETH/USD to ETH-USD, etc.
	parts := strings.Split(symbol, "/")
	if len(parts) != 2 {
		return symbol // Return as-is if not in expected format
	}

	base := strings.ToUpper(parts[0])
	quote := strings.ToUpper(parts[1])

	return base + "-" + quote
}

// isUSDPair checks if the symbol is a USD pair
func (cc *CoinbaseCollector) isUSDPair(symbol string) bool {
	return strings.HasSuffix(strings.ToUpper(symbol), "/USD") ||
		strings.HasSuffix(strings.ToUpper(symbol), "-USD")
}

// checkSequenceGap checks for sequence number gaps
func (cc *CoinbaseCollector) checkSequenceGap(currentSeq int64) bool {
	cc.statsMutex.Lock()
	defer cc.statsMutex.Unlock()

	if cc.lastSequenceNum > 0 && currentSeq > cc.lastSequenceNum+100 { // Allow for reasonable gap
		cc.sequenceGaps++
		cc.lastSequenceNum = currentSeq
		return true
	}

	cc.lastSequenceNum = currentSeq
	return false
}

// recordLatency records API call latency
func (cc *CoinbaseCollector) recordLatency(latency time.Duration) {
	cc.statsMutex.Lock()
	defer cc.statsMutex.Unlock()

	cc.windowLatencySum += latency
	if latency > cc.windowLatencyMax {
		cc.windowLatencyMax = latency
	}
	cc.windowMessageCount++
}

// recordError records an error occurrence
func (cc *CoinbaseCollector) recordError(operation string, err error) {
	cc.statsMutex.Lock()
	defer cc.statsMutex.Unlock()

	cc.windowErrorCount++
	cc.errorCount++

	// Log the error (in production, this would go to proper logging)
	fmt.Printf("Coinbase %s error: %v\n", operation, err)
}

// updateMetricsWindow overrides base implementation with Coinbase-specific metrics
func (cc *CoinbaseCollector) updateMetricsWindow(windowStart, windowEnd time.Time) {
	cc.statsMutex.Lock()

	windowDuration := windowEnd.Sub(windowStart)

	var avgLatencyMs float64
	if cc.windowMessageCount > 0 {
		avgLatencyMs = float64(cc.windowLatencySum.Milliseconds()) / float64(cc.windowMessageCount)
	}

	maxLatencyMs := cc.windowLatencyMax.Milliseconds()

	// Calculate quality score based on errors and latency
	qualityScore := 100.0
	if cc.windowMessageCount > 0 {
		errorRate := float64(cc.windowErrorCount) / float64(cc.windowMessageCount)
		qualityScore -= errorRate * 50 // Reduce by up to 50 points for errors
	}

	if avgLatencyMs > 1000 { // Penalize high latency
		qualityScore -= (avgLatencyMs - 1000) / 100
	}

	if qualityScore < 0 {
		qualityScore = 0
	}

	metrics := &micro.CollectorMetrics{
		Venue:            "coinbase",
		WindowStart:      windowStart,
		WindowEnd:        windowEnd,
		L1Messages:       cc.windowMessageCount, // Approximation: each fetch counts as message
		L2Messages:       cc.windowMessageCount,
		ErrorMessages:    cc.windowErrorCount,
		ProcessingTimeMs: windowDuration.Milliseconds(),
		AvgLatencyMs:     avgLatencyMs,
		MaxLatencyMs:     maxLatencyMs,
		StaleDataCount:   0,                   // Would need timestamp analysis to calculate properly
		IncompleteCount:  cc.windowErrorCount, // Approximation: errors often mean incomplete data
		QualityScore:     qualityScore,
	}

	// Reset window counters
	cc.windowMessageCount = 0
	cc.windowErrorCount = 0
	cc.windowLatencySum = 0
	cc.windowLatencyMax = 0
	cc.lastWindowStart = windowEnd

	cc.statsMutex.Unlock()

	cc.updateMetrics(metrics)
}

// updateRollingHealthStats overrides base implementation with Coinbase-specific health
func (cc *CoinbaseCollector) updateRollingHealthStats() {
	cc.statsMutex.Lock()

	now := time.Now()

	// Calculate error rate over last 60 seconds (approximated)
	totalMessages := cc.messageCount
	errorRate := 0.0
	if totalMessages > 0 {
		errorRate = float64(cc.errorCount) / float64(totalMessages)
	}

	// Calculate average latency
	avgLatency := time.Duration(0)
	if cc.messageCount > 0 {
		avgLatency = cc.latencySum / time.Duration(cc.messageCount)
	}

	// Determine health status
	status := micro.HealthGreen
	healthy := true
	recommendation := "proceed"

	if errorRate > cc.config.MaxErrorRate {
		status = micro.HealthRed
		healthy = false
		recommendation = "avoid"
	} else if avgLatency.Milliseconds() > cc.config.MaxLatencyP99Ms {
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
		Venue:            "coinbase",
		Timestamp:        now,
		Status:           status,
		Healthy:          healthy,
		Uptime:           uptime,
		HeartbeatAgeMs:   int64(dataFreshness / time.Millisecond),
		MessageGapRate:   float64(cc.sequenceGaps) / float64(cc.messageCount),
		WSReconnectCount: cc.wsReconnectCount,
		LatencyP50Ms:     avgLatency.Milliseconds() / 2,    // Approximation
		LatencyP99Ms:     int64(avgLatency.Milliseconds()), // Approximation
		ErrorRate:        errorRate,
		DataFreshness:    dataFreshness,
		DataCompleteness: 100.0 - (errorRate * 100),
		Recommendation:   recommendation,
	}

	cc.statsMutex.Unlock()

	cc.updateVenueHealth(health)
}
