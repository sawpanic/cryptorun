package microstructure

import (
	"fmt"
	"math"
)

// DepthCalculator computes order book depth within ±2% price bounds
type DepthCalculator struct {
	windowSeconds int
	priceBounds   float64 // Default: 0.02 (2%)
}

// NewDepthCalculator creates a depth calculator with rolling window
func NewDepthCalculator(windowSeconds int) *DepthCalculator {
	return &DepthCalculator{
		windowSeconds: windowSeconds,
		priceBounds:   0.02, // ±2%
	}
}

// DepthResult contains depth calculation results
type DepthResult struct {
	BidDepthUSD   float64 `json:"bid_depth_usd"`   // Total bid liquidity within bounds
	AskDepthUSD   float64 `json:"ask_depth_usd"`   // Total ask liquidity within bounds
	TotalDepthUSD float64 `json:"total_depth_usd"` // Combined depth
	BidLevels     int     `json:"bid_levels"`      // Number of bid levels included
	AskLevels     int     `json:"ask_levels"`      // Number of ask levels included
	LastPrice     float64 `json:"last_price"`      // Reference price
	BidBound      float64 `json:"bid_bound"`       // Lower bound price (-2%)
	AskBound      float64 `json:"ask_bound"`       // Upper bound price (+2%)
}

// CalculateDepth computes depth within ±2% of last trade price
func (dc *DepthCalculator) CalculateDepth(orderbook *OrderBookSnapshot) (*DepthResult, error) {
	if orderbook == nil {
		return nil, fmt.Errorf("order book snapshot is nil")
	}

	if orderbook.LastPrice <= 0 {
		return nil, fmt.Errorf("invalid last price: %.6f", orderbook.LastPrice)
	}

	if len(orderbook.Bids) == 0 && len(orderbook.Asks) == 0 {
		return nil, fmt.Errorf("empty order book for %s", orderbook.Symbol)
	}

	lastPrice := orderbook.LastPrice
	bidBound := lastPrice * (1.0 - dc.priceBounds) // -2%
	askBound := lastPrice * (1.0 + dc.priceBounds) // +2%

	result := &DepthResult{
		LastPrice: lastPrice,
		BidBound:  bidBound,
		AskBound:  askBound,
	}

	// Calculate bid depth (prices >= bidBound)
	bidDepthUSD := 0.0
	bidLevels := 0
	for _, bid := range orderbook.Bids {
		if bid.Price >= bidBound {
			bidDepthUSD += bid.Price * bid.Size
			bidLevels++
		} else {
			// Bids are sorted descending, so we can break early
			break
		}
	}

	// Calculate ask depth (prices <= askBound)
	askDepthUSD := 0.0
	askLevels := 0
	for _, ask := range orderbook.Asks {
		if ask.Price <= askBound {
			askDepthUSD += ask.Price * ask.Size
			askLevels++
		} else {
			// Asks are sorted ascending, so we can break early
			break
		}
	}

	result.BidDepthUSD = bidDepthUSD
	result.AskDepthUSD = askDepthUSD
	result.TotalDepthUSD = bidDepthUSD + askDepthUSD
	result.BidLevels = bidLevels
	result.AskLevels = askLevels

	return result, nil
}

// ValidateDepthRequirement checks if depth meets tier requirements
func (dc *DepthCalculator) ValidateDepthRequirement(depthResult *DepthResult, tier *LiquidityTier) (bool, string) {
	if depthResult == nil || tier == nil {
		return false, "invalid inputs"
	}

	if depthResult.TotalDepthUSD >= tier.DepthMinUSD {
		return true, fmt.Sprintf("depth $%.0f ≥ $%.0f (%s)",
			depthResult.TotalDepthUSD, tier.DepthMinUSD, tier.Name)
	}

	return false, fmt.Sprintf("insufficient depth: $%.0f < $%.0f (%s requirement)",
		depthResult.TotalDepthUSD, tier.DepthMinUSD, tier.Name)
}

// GetDepthSummary returns a human-readable depth summary
func (dc *DepthCalculator) GetDepthSummary(depthResult *DepthResult) string {
	if depthResult == nil {
		return "no depth data"
	}

	return fmt.Sprintf("Depth: $%.0f total ($%.0f bids @ %d levels, $%.0f asks @ %d levels) within ±2%% of $%.4f",
		depthResult.TotalDepthUSD,
		depthResult.BidDepthUSD, depthResult.BidLevels,
		depthResult.AskDepthUSD, depthResult.AskLevels,
		depthResult.LastPrice)
}

// CalculateDepthBalance returns bid/ask depth balance ratio
func (dc *DepthCalculator) CalculateDepthBalance(depthResult *DepthResult) float64 {
	if depthResult == nil || depthResult.TotalDepthUSD == 0 {
		return 0.5 // Neutral if no data
	}

	// Returns 0.0-1.0, where 0.5 is perfectly balanced
	// <0.5 = ask-heavy, >0.5 = bid-heavy
	return depthResult.BidDepthUSD / depthResult.TotalDepthUSD
}

// EstimateMarketImpact estimates price impact for a given trade size
func (dc *DepthCalculator) EstimateMarketImpact(orderbook *OrderBookSnapshot, tradeSizeUSD float64, side string) (*MarketImpact, error) {
	if orderbook == nil {
		return nil, fmt.Errorf("order book snapshot is nil")
	}

	if tradeSizeUSD <= 0 {
		return nil, fmt.Errorf("invalid trade size: %.2f", tradeSizeUSD)
	}

	var levels []PriceLevel

	switch side {
	case "buy":
		levels = orderbook.Asks
	case "sell":
		levels = orderbook.Bids
	default:
		return nil, fmt.Errorf("invalid side: %s (must be 'buy' or 'sell')", side)
	}

	if len(levels) == 0 {
		return nil, fmt.Errorf("no %s side liquidity", side)
	}

	impact := &MarketImpact{
		Side:           side,
		RequestedUSD:   tradeSizeUSD,
		StartPrice:     orderbook.LastPrice,
		LevelsConsumed: 0,
	}

	remainingUSD := tradeSizeUSD
	totalCost := 0.0
	totalQuantity := 0.0

	for i, level := range levels {
		if remainingUSD <= 0 {
			break
		}

		levelValueUSD := level.Price * level.Size
		consumedUSD := math.Min(remainingUSD, levelValueUSD)
		consumedQuantity := consumedUSD / level.Price

		totalCost += consumedUSD
		totalQuantity += consumedQuantity
		remainingUSD -= consumedUSD
		impact.LevelsConsumed = i + 1
		impact.FinalPrice = level.Price
	}

	if remainingUSD > 0 {
		impact.InsufficientLiquidity = true
		impact.ShortfallUSD = remainingUSD
	}

	if totalQuantity > 0 {
		impact.AveragePrice = totalCost / totalQuantity
		impact.SlippageBps = math.Abs(impact.AveragePrice-impact.StartPrice) / impact.StartPrice * 10000
	}

	impact.FilledUSD = totalCost
	impact.FilledQuantity = totalQuantity

	return impact, nil
}

// MarketImpact contains market impact analysis results
type MarketImpact struct {
	Side                  string  `json:"side"`                    // "buy" or "sell"
	RequestedUSD          float64 `json:"requested_usd"`           // Requested trade size
	FilledUSD             float64 `json:"filled_usd"`              // Actually filled amount
	FilledQuantity        float64 `json:"filled_quantity"`         // Filled quantity in base units
	StartPrice            float64 `json:"start_price"`             // Starting reference price
	AveragePrice          float64 `json:"average_price"`           // Volume-weighted average price
	FinalPrice            float64 `json:"final_price"`             // Final level price consumed
	SlippageBps           float64 `json:"slippage_bps"`            // Slippage in basis points
	LevelsConsumed        int     `json:"levels_consumed"`         // Number of order book levels
	InsufficientLiquidity bool    `json:"insufficient_liquidity"`  // Could not fill completely
	ShortfallUSD          float64 `json:"shortfall_usd,omitempty"` // Unfilled amount if insufficient
}
