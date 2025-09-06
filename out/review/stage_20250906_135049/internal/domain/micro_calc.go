package domain

import (
	"math"
)

// RoundBps implements HALF-UP rounding for basis points
// At exactly 0.5, rounds UP (not banker's rounding)
// Example: 49.5 -> 50, 49.4 -> 49, 50.5 -> 51
func RoundBps(x float64) int {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0 // Fail-safe for pathological inputs
	}
	
	// HALF-UP rounding: add 0.5 and truncate
	if x >= 0 {
		return int(x + 0.5)
	} else {
		return int(x - 0.5)
	}
}

// Depth2pcUSD computes USD liquidity depth within ±2% of mid price
// Uses precise rounding semantics:
// 1. Calculate mid = (bid + ask) / 2
// 2. Bounds = [mid * 0.98, mid * 1.02]
// 3. Sum price*size for levels within bounds
// 4. Round individual USD values to nearest cent, then sum
// 5. Round final result to nearest USD
func Depth2pcUSD(bid, ask float64, bidSizes, askSizes []float64, bidPrices, askPrices []float64) float64 {
	// Guard against invalid inputs
	if math.IsNaN(bid) || math.IsNaN(ask) || math.IsInf(bid, 0) || math.IsInf(ask, 0) {
		return 0.0 // Fail-safe
	}
	
	if bid <= 0 || ask <= 0 || bid >= ask {
		return 0.0 // Invalid price data
	}
	
	if len(bidSizes) != len(bidPrices) || len(askSizes) != len(askPrices) {
		return 0.0 // Mismatched arrays
	}
	
	// Calculate mid price and ±2% bounds
	mid := (bid + ask) / 2.0
	lowerBound := mid * 0.98  // -2%
	upperBound := mid * 1.02  // +2%
	
	var totalDepthUSD float64
	
	// Sum bid side within bounds
	for i := 0; i < len(bidPrices); i++ {
		price := bidPrices[i]
		size := bidSizes[i]
		
		// Skip invalid data
		if math.IsNaN(price) || math.IsNaN(size) || price <= 0 || size <= 0 {
			continue
		}
		
		// Check if within ±2% bounds
		if price >= lowerBound && price <= upperBound {
			// Round individual USD value to nearest cent, then add
			usdValue := price * size
			centValue := math.Round(usdValue * 100) / 100 // Round to cent
			totalDepthUSD += centValue
		}
	}
	
	// Sum ask side within bounds
	for i := 0; i < len(askPrices); i++ {
		price := askPrices[i]
		size := askSizes[i]
		
		// Skip invalid data
		if math.IsNaN(price) || math.IsNaN(size) || price <= 0 || size <= 0 {
			continue
		}
		
		// Check if within ±2% bounds
		if price >= lowerBound && price <= upperBound {
			// Round individual USD value to nearest cent, then add
			usdValue := price * size
			centValue := math.Round(usdValue * 100) / 100 // Round to cent
			totalDepthUSD += centValue
		}
	}
	
	// Round final result to nearest USD
	return math.Round(totalDepthUSD)
}

// ComputeSpreadBps calculates spread in basis points with HALF-UP rounding
// Formula: spread_bps = ((ask - bid) / mid) * 10000
// Applies RoundBps for consistent precision
func ComputeSpreadBps(bid, ask float64) int {
	if math.IsNaN(bid) || math.IsNaN(ask) || math.IsInf(bid, 0) || math.IsInf(ask, 0) {
		return 9999 // Fail-safe: return very high spread
	}
	
	if bid <= 0 || ask <= 0 || bid >= ask {
		return 9999 // Invalid: return very high spread
	}
	
	mid := (bid + ask) / 2.0
	if mid <= 0 {
		return 9999 // Safety check
	}
	
	spreadFraction := (ask - bid) / mid
	spreadBps := spreadFraction * 10000.0
	
	return RoundBps(spreadBps)
}

// GuardFinite ensures a float64 value is finite, returning fallback for NaN/Inf
func GuardFinite(value, fallback float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	return value
}

// GuardPositive ensures a value is positive, returning fallback if not
func GuardPositive(value, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}
	return GuardFinite(value, fallback)
}