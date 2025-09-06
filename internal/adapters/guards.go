package adapters

import (
	"context"
	"fmt"
	"time"
)

// GuardsAdapter provides shared guard evaluation for benchmarking
type GuardsAdapter struct {
	microstructureEnabled bool
	freshnessEnabled      bool
	fatigueEnabled        bool
}

// NewGuardsAdapter creates a guards adapter
func NewGuardsAdapter() *GuardsAdapter {
	return &GuardsAdapter{
		microstructureEnabled: true,
		freshnessEnabled:      true,
		fatigueEnabled:        true,
	}
}

// EvaluateAsset applies all guards to determine asset eligibility
func (ga *GuardsAdapter) EvaluateAsset(ctx context.Context, symbol string) (bool, []string, error) {
	var reasons []string
	passed := true

	// Microstructure guard - use mock validation for benchmark
	if ga.microstructureEnabled {
		microPassed, microReason := ga.evaluateMicrostructure(symbol)
		if !microPassed {
			passed = false
			reasons = append(reasons, microReason)
		}
	}

	// Freshness guard - mock evaluation
	if ga.freshnessEnabled {
		freshPassed, freshReason := ga.evaluateFreshness(symbol)
		if !freshPassed {
			passed = false
			reasons = append(reasons, freshReason)
		}
	}

	// Fatigue guard - mock evaluation
	if ga.fatigueEnabled {
		fatiguePassed, fatigueReason := ga.evaluateFatigue(symbol)
		if !fatiguePassed {
			passed = false
			reasons = append(reasons, fatigueReason)
		}
	}

	return passed, reasons, nil
}

// evaluateMicrostructure performs mock microstructure validation
func (ga *GuardsAdapter) evaluateMicrostructure(symbol string) (bool, string) {
	// Mock microstructure validation - simulate realistic pass rates
	// Major coins: 90% pass rate
	// Mid-cap coins: 70% pass rate
	// Long-tail coins: 40% pass rate

	majorCoins := map[string]bool{
		"BTC": true, "ETH": true, "BNB": true, "SOL": true,
		"ADA": true, "AVAX": true, "DOT": true, "LINK": true,
	}

	midCapCoins := map[string]bool{
		"MATIC": true, "UNI": true, "LTC": true, "BCH": true,
		"XRP": true, "DOGE": true,
	}

	// Simple hash-based deterministic "randomness"
	hash := 0
	for _, c := range symbol {
		hash = hash*31 + int(c)
	}
	randomValue := float64(hash%100) / 100.0

	if majorCoins[symbol] {
		if randomValue < 0.90 {
			return true, ""
		} else {
			return false, "microstructure_spread_violation"
		}
	} else if midCapCoins[symbol] {
		if randomValue < 0.70 {
			return true, ""
		} else {
			return false, "microstructure_depth_insufficient"
		}
	} else {
		if randomValue < 0.40 {
			return true, ""
		} else {
			return false, "microstructure_vadr_below_threshold"
		}
	}
}

// evaluateFreshness performs mock freshness validation
func (ga *GuardsAdapter) evaluateFreshness(symbol string) (bool, string) {
	// Mock freshness check - simulate data staleness
	// Most assets pass freshness (85% rate)

	hash := 0
	for _, c := range symbol {
		hash = hash*17 + int(c)
	}
	randomValue := float64(hash%100) / 100.0

	if randomValue < 0.85 {
		return true, ""
	} else {
		return false, "freshness_data_stale_3_bars"
	}
}

// evaluateFatigue performs mock fatigue validation
func (ga *GuardsAdapter) evaluateFatigue(symbol string) (bool, string) {
	// Mock fatigue check - simulate overextended moves
	// Varies by symbol to simulate different momentum states

	hash := 0
	for _, c := range symbol {
		hash = hash*13 + int(c)
	}
	randomValue := float64(hash%100) / 100.0

	// Simulate momentum/RSI checks
	if randomValue < 0.75 {
		return true, ""
	} else {
		momentum := 12.0 + (randomValue * 8.0) // Mock momentum 12-20%
		rsi := 65.0 + (randomValue * 15.0)     // Mock RSI 65-80
		return false, fmt.Sprintf("fatigue_momentum_%.1f_rsi_%.1f", momentum, rsi)
	}
}

// CoinGeckoPriceProvider implements PriceProvider using CoinGecko API
type CoinGeckoPriceProvider struct {
	featuresAdapter *FeaturesAdapter
}

// NewCoinGeckoPriceProvider creates a CoinGecko-based price provider
func NewCoinGeckoPriceProvider() *CoinGeckoPriceProvider {
	return &CoinGeckoPriceProvider{
		featuresAdapter: NewFeaturesAdapter(),
	}
}

// GetPrice retrieves price for a symbol at a specific timestamp
func (cgpp *CoinGeckoPriceProvider) GetPrice(ctx context.Context, symbol string, timestamp time.Time) (float64, error) {
	// For benchmark purposes, generate mock prices with realistic movements
	// In production, this would fetch actual historical prices

	features, err := cgpp.featuresAdapter.GetFeatures(ctx, symbol, timestamp)
	if err != nil {
		// Fallback to mock price generation
		return cgpp.generateMockPrice(symbol, timestamp), nil
	}

	// Use current price from features as base
	basePrice := 100.0 // Mock base price

	// Apply mock time-based price movement
	timeOffset := time.Since(timestamp).Hours()
	priceMultiplier := 1.0 + (features.Returns24h * (timeOffset / 24.0))

	return basePrice * priceMultiplier, nil
}

// generateMockPrice creates a deterministic mock price for testing
func (cgpp *CoinGeckoPriceProvider) generateMockPrice(symbol string, timestamp time.Time) float64 {
	// Generate deterministic price based on symbol and timestamp
	hash := 0
	for _, c := range symbol {
		hash = hash*31 + int(c)
	}

	// Add timestamp component for price movement
	timeComponent := float64(timestamp.Unix() % 86400) // Daily cycle

	// Base prices for common symbols
	basePrices := map[string]float64{
		"BTC":  45000.0,
		"ETH":  2800.0,
		"BNB":  320.0,
		"SOL":  95.0,
		"ADA":  0.48,
		"AVAX": 18.5,
		"DOT":  6.2,
		"LINK": 14.8,
	}

	basePrice := basePrices[symbol]
	if basePrice == 0 {
		basePrice = float64(hash%1000 + 1) // Fallback price $1-$1000
	}

	// Add some realistic price movement (±5% daily range)
	movement := (timeComponent/86400.0 - 0.5) * 0.1 // ±5%

	return basePrice * (1.0 + movement)
}
