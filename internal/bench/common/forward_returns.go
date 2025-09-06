package common

import (
	"context"
	"fmt"
	"time"
)

// ForwardReturnsCalculator computes forward returns for benchmark analysis
type ForwardReturnsCalculator struct {
	priceProvider PriceProvider
}

// NewForwardReturnsCalculator creates a forward returns calculator
func NewForwardReturnsCalculator(priceProvider PriceProvider) *ForwardReturnsCalculator {
	return &ForwardReturnsCalculator{
		priceProvider: priceProvider,
	}
}

// WindowSpec defines a time window for forward return calculation
type WindowSpec struct {
	Name     string        `json:"name"` // "1h", "4h", "12h", "24h", "7d"
	Duration time.Duration `json:"duration"`
}

// ParseWindows converts window strings to WindowSpec objects
func ParseWindows(windowStrs []string) ([]WindowSpec, error) {
	specs := make([]WindowSpec, 0, len(windowStrs))

	for _, ws := range windowStrs {
		var duration time.Duration
		var err error

		switch ws {
		case "1h":
			duration = time.Hour
		case "4h":
			duration = 4 * time.Hour
		case "12h":
			duration = 12 * time.Hour
		case "24h":
			duration = 24 * time.Hour
		case "7d":
			duration = 7 * 24 * time.Hour
		default:
			return nil, fmt.Errorf("unsupported window: %s", ws)
		}

		specs = append(specs, WindowSpec{
			Name:     ws,
			Duration: duration,
		})
	}

	return specs, nil
}

// ForwardReturn represents a return calculation for a specific window
type ForwardReturn struct {
	Symbol      string             `json:"symbol"`
	BaseTime    time.Time          `json:"base_time"`
	Windows     map[string]float64 `json:"windows"` // window_name -> return
	BasePrice   float64            `json:"base_price"`
	FinalPrices map[string]float64 `json:"final_prices"` // window_name -> final_price
}

// CalculateForwardReturns computes returns for all assets and windows
func (frc *ForwardReturnsCalculator) CalculateForwardReturns(
	ctx context.Context,
	assets []Asset,
	windows []WindowSpec,
	baseTime time.Time,
) ([]ForwardReturn, error) {

	results := make([]ForwardReturn, 0, len(assets))

	for _, asset := range assets {
		// Get base price
		basePrice, err := frc.priceProvider.GetPrice(ctx, asset.Symbol, baseTime)
		if err != nil {
			// Skip assets where we can't get base price
			continue
		}

		result := ForwardReturn{
			Symbol:      asset.Symbol,
			BaseTime:    baseTime,
			BasePrice:   basePrice,
			Windows:     make(map[string]float64),
			FinalPrices: make(map[string]float64),
		}

		// Calculate returns for each window
		for _, window := range windows {
			finalTime := baseTime.Add(window.Duration)
			finalPrice, err := frc.priceProvider.GetPrice(ctx, asset.Symbol, finalTime)
			if err != nil {
				// Skip this window for this asset
				continue
			}

			// Calculate return as (final - base) / base
			returnValue := (finalPrice - basePrice) / basePrice

			result.Windows[window.Name] = returnValue
			result.FinalPrices[window.Name] = finalPrice
		}

		// Only include if we got at least one window
		if len(result.Windows) > 0 {
			results = append(results, result)
		}
	}

	return results, nil
}

// HitRule defines what constitutes a "hit" for benchmark evaluation
type HitRule struct {
	Type      string  `json:"type"`       // "top_k" or "threshold"
	Value     float64 `json:"value"`      // k for top_k, threshold for threshold
	WindowMin int     `json:"window_min"` // Minimum windows required
}

// DefaultHitRule returns the standard hit rule used in TopGainers bench
func DefaultHitRule() HitRule {
	return HitRule{
		Type:      "threshold",
		Value:     0.02, // 2% threshold
		WindowMin: 1,    // At least 1 window must hit
	}
}

// EvaluateHits determines which assets are "hits" based on the rule
func EvaluateHits(returns []ForwardReturn, rule HitRule) map[string]map[string]bool {
	hits := make(map[string]map[string]bool) // symbol -> window -> hit

	for _, ret := range returns {
		hits[ret.Symbol] = make(map[string]bool)

		for window, returnVal := range ret.Windows {
			switch rule.Type {
			case "threshold":
				hits[ret.Symbol][window] = returnVal >= rule.Value
			case "top_k":
				// For top_k, we'd need to rank all assets - simplified for now
				hits[ret.Symbol][window] = returnVal > 0.05 // 5% threshold as proxy
			default:
				hits[ret.Symbol][window] = false
			}
		}
	}

	return hits
}

// PriceProvider interface for getting historical prices
type PriceProvider interface {
	GetPrice(ctx context.Context, symbol string, timestamp time.Time) (float64, error)
}

// MockPriceProvider for testing
type MockPriceProvider struct {
	prices map[string]map[time.Time]float64
}

// NewMockPriceProvider creates a mock price provider
func NewMockPriceProvider() *MockPriceProvider {
	return &MockPriceProvider{
		prices: make(map[string]map[time.Time]float64),
	}
}

// SetPrice sets a price for a symbol at a specific time
func (mpp *MockPriceProvider) SetPrice(symbol string, timestamp time.Time, price float64) {
	if mpp.prices[symbol] == nil {
		mpp.prices[symbol] = make(map[time.Time]float64)
	}
	mpp.prices[symbol][timestamp] = price
}

// GetPrice retrieves a price for a symbol at a specific time
func (mpp *MockPriceProvider) GetPrice(ctx context.Context, symbol string, timestamp time.Time) (float64, error) {
	if symbolPrices, exists := mpp.prices[symbol]; exists {
		if price, exists := symbolPrices[timestamp]; exists {
			return price, nil
		}
	}
	return 0, fmt.Errorf("no price data for %s at %v", symbol, timestamp)
}
