package microstructure

import (
	"fmt"
	"math"
	"time"
)

// SpreadCalculator computes bid-ask spreads with rolling averages
type SpreadCalculator struct {
	windowSeconds int
	history       []SpreadPoint
	maxHistory    int
}

// NewSpreadCalculator creates a spread calculator with 60s rolling window
func NewSpreadCalculator(windowSeconds int) *SpreadCalculator {
	maxHistory := windowSeconds * 2 // Keep 2× window for safety
	if maxHistory < 100 {
		maxHistory = 100
	}

	return &SpreadCalculator{
		windowSeconds: windowSeconds,
		maxHistory:    maxHistory,
		history:       make([]SpreadPoint, 0, maxHistory),
	}
}

// SpreadPoint tracks spread at a specific point in time
type SpreadPoint struct {
	Timestamp time.Time `json:"timestamp"`
	BidPrice  float64   `json:"bid_price"`
	AskPrice  float64   `json:"ask_price"`
	SpreadAbs float64   `json:"spread_abs"` // Absolute spread (ask - bid)
	SpreadBps float64   `json:"spread_bps"` // Spread in basis points
	MidPrice  float64   `json:"mid_price"`  // (bid + ask) / 2
	LastPrice float64   `json:"last_price"` // Last trade price
}

// SpreadResult contains comprehensive spread analysis
type SpreadResult struct {
	Current       SpreadPoint `json:"current"`         // Latest measurement
	RollingAvgBps float64     `json:"rolling_avg_bps"` // 60s rolling average
	MinBps        float64     `json:"min_bps"`         // Minimum in window
	MaxBps        float64     `json:"max_bps"`         // Maximum in window
	StdDevBps     float64     `json:"std_dev_bps"`     // Standard deviation
	SampleCount   int         `json:"sample_count"`    // Samples in window
	WindowSeconds int         `json:"window_seconds"`  // Rolling window size
	DataQuality   string      `json:"data_quality"`    // "excellent", "good", "sparse"
}

// CalculateSpread computes current spread and updates rolling average
func (sc *SpreadCalculator) CalculateSpread(orderbook *OrderBookSnapshot) (*SpreadResult, error) {
	if orderbook == nil {
		return nil, fmt.Errorf("order book snapshot is nil")
	}

	if len(orderbook.Bids) == 0 || len(orderbook.Asks) == 0 {
		return nil, fmt.Errorf("incomplete order book: %d bids, %d asks",
			len(orderbook.Bids), len(orderbook.Asks))
	}

	// Get best bid and ask
	bestBid := orderbook.Bids[0] // Highest bid
	bestAsk := orderbook.Asks[0] // Lowest ask

	if bestBid.Price <= 0 || bestAsk.Price <= 0 {
		return nil, fmt.Errorf("invalid prices: bid=%.6f, ask=%.6f",
			bestBid.Price, bestAsk.Price)
	}

	if bestAsk.Price <= bestBid.Price {
		return nil, fmt.Errorf("crossed book: bid=%.6f >= ask=%.6f",
			bestBid.Price, bestAsk.Price)
	}

	// Calculate current spread
	spreadAbs := bestAsk.Price - bestBid.Price
	midPrice := (bestBid.Price + bestAsk.Price) / 2.0
	spreadBps := (spreadAbs / midPrice) * 10000.0

	currentPoint := SpreadPoint{
		Timestamp: orderbook.Timestamp,
		BidPrice:  bestBid.Price,
		AskPrice:  bestAsk.Price,
		SpreadAbs: spreadAbs,
		SpreadBps: spreadBps,
		MidPrice:  midPrice,
		LastPrice: orderbook.LastPrice,
	}

	// Add to history
	sc.addToHistory(currentPoint)

	// Calculate rolling statistics
	result := sc.calculateRollingStats(currentPoint)

	return result, nil
}

// addToHistory adds a spread point and manages history size
func (sc *SpreadCalculator) addToHistory(point SpreadPoint) {
	sc.history = append(sc.history, point)

	// Trim to max history size
	if len(sc.history) > sc.maxHistory {
		sc.history = sc.history[1:]
	}
}

// calculateRollingStats computes rolling statistics over the window
func (sc *SpreadCalculator) calculateRollingStats(current SpreadPoint) *SpreadResult {
	cutoff := current.Timestamp.Add(-time.Duration(sc.windowSeconds) * time.Second)

	// Filter to window
	var windowPoints []SpreadPoint
	for _, point := range sc.history {
		if point.Timestamp.After(cutoff) {
			windowPoints = append(windowPoints, point)
		}
	}

	if len(windowPoints) == 0 {
		// No history, use current point
		return &SpreadResult{
			Current:       current,
			RollingAvgBps: current.SpreadBps,
			MinBps:        current.SpreadBps,
			MaxBps:        current.SpreadBps,
			StdDevBps:     0.0,
			SampleCount:   1,
			WindowSeconds: sc.windowSeconds,
			DataQuality:   "sparse",
		}
	}

	// Calculate statistics
	sum := 0.0
	minBps := math.Inf(1)
	maxBps := math.Inf(-1)

	for _, point := range windowPoints {
		sum += point.SpreadBps
		if point.SpreadBps < minBps {
			minBps = point.SpreadBps
		}
		if point.SpreadBps > maxBps {
			maxBps = point.SpreadBps
		}
	}

	avgBps := sum / float64(len(windowPoints))

	// Calculate standard deviation
	sumSquares := 0.0
	for _, point := range windowPoints {
		diff := point.SpreadBps - avgBps
		sumSquares += diff * diff
	}
	stdDevBps := math.Sqrt(sumSquares / float64(len(windowPoints)))

	// Assess data quality
	dataQuality := "excellent"
	samplesPerSecond := float64(len(windowPoints)) / float64(sc.windowSeconds)
	if samplesPerSecond < 0.1 {
		dataQuality = "sparse"
	} else if samplesPerSecond < 0.5 {
		dataQuality = "good"
	}

	return &SpreadResult{
		Current:       current,
		RollingAvgBps: avgBps,
		MinBps:        minBps,
		MaxBps:        maxBps,
		StdDevBps:     stdDevBps,
		SampleCount:   len(windowPoints),
		WindowSeconds: sc.windowSeconds,
		DataQuality:   dataQuality,
	}
}

// ValidateSpreadRequirement checks if spread meets tier cap
func (sc *SpreadCalculator) ValidateSpreadRequirement(spreadResult *SpreadResult, tier *LiquidityTier) (bool, string) {
	if spreadResult == nil || tier == nil {
		return false, "invalid inputs"
	}

	// Use rolling average for stability, fall back to current if no history
	spreadToCheck := spreadResult.RollingAvgBps
	if spreadResult.SampleCount <= 1 {
		spreadToCheck = spreadResult.Current.SpreadBps
	}

	if spreadToCheck <= tier.SpreadCapBps {
		return true, fmt.Sprintf("spread %.1f bps ≤ %.1f bps (%s cap)",
			spreadToCheck, tier.SpreadCapBps, tier.Name)
	}

	return false, fmt.Sprintf("spread too wide: %.1f bps > %.1f bps (%s cap)",
		spreadToCheck, tier.SpreadCapBps, tier.Name)
}

// GetSpreadSummary returns human-readable spread summary
func (sc *SpreadCalculator) GetSpreadSummary(spreadResult *SpreadResult) string {
	if spreadResult == nil {
		return "no spread data"
	}

	if spreadResult.SampleCount <= 1 {
		return fmt.Sprintf("Spread: %.1f bps (bid: $%.4f, ask: $%.4f, mid: $%.4f)",
			spreadResult.Current.SpreadBps,
			spreadResult.Current.BidPrice,
			spreadResult.Current.AskPrice,
			spreadResult.Current.MidPrice)
	}

	return fmt.Sprintf("Spread: %.1f bps avg (current: %.1f, range: %.1f-%.1f, %d samples)",
		spreadResult.RollingAvgBps,
		spreadResult.Current.SpreadBps,
		spreadResult.MinBps,
		spreadResult.MaxBps,
		spreadResult.SampleCount)
}

// IsSpreadStable checks if spread is stable within acceptable variance
func (sc *SpreadCalculator) IsSpreadStable(spreadResult *SpreadResult, maxStdDevBps float64) bool {
	if spreadResult == nil || spreadResult.SampleCount < 10 {
		return false // Need sufficient samples
	}

	return spreadResult.StdDevBps <= maxStdDevBps
}

// EstimateExecutionCost estimates total execution cost including spread impact
func (sc *SpreadCalculator) EstimateExecutionCost(spreadResult *SpreadResult, tradeSizeUSD float64, side string) *ExecutionCost {
	if spreadResult == nil {
		return &ExecutionCost{
			Error: "no spread data available",
		}
	}

	current := spreadResult.Current

	var executionPrice, impactBps float64

	switch side {
	case "buy":
		executionPrice = current.AskPrice
		// For small trades, main cost is crossing the spread
		impactBps = (executionPrice - current.MidPrice) / current.MidPrice * 10000
	case "sell":
		executionPrice = current.BidPrice
		impactBps = (current.MidPrice - executionPrice) / current.MidPrice * 10000
	default:
		return &ExecutionCost{
			Error: fmt.Sprintf("invalid side: %s", side),
		}
	}

	quantity := tradeSizeUSD / executionPrice
	totalCost := quantity * executionPrice

	return &ExecutionCost{
		Side:           side,
		TradeSizeUSD:   tradeSizeUSD,
		ExecutionPrice: executionPrice,
		MidPrice:       current.MidPrice,
		Quantity:       quantity,
		TotalCost:      totalCost,
		ImpactBps:      impactBps,
		SpreadBps:      current.SpreadBps,
		Timestamp:      current.Timestamp,
	}
}

// ExecutionCost contains execution cost estimation
type ExecutionCost struct {
	Side           string    `json:"side"`
	TradeSizeUSD   float64   `json:"trade_size_usd"`
	ExecutionPrice float64   `json:"execution_price"`
	MidPrice       float64   `json:"mid_price"`
	Quantity       float64   `json:"quantity"`
	TotalCost      float64   `json:"total_cost"`
	ImpactBps      float64   `json:"impact_bps"` // Price impact in bps
	SpreadBps      float64   `json:"spread_bps"` // Current spread
	Timestamp      time.Time `json:"timestamp"`
	Error          string    `json:"error,omitempty"`
}

// ClearHistory clears spread history (useful for testing)
func (sc *SpreadCalculator) ClearHistory() {
	sc.history = sc.history[:0]
}
