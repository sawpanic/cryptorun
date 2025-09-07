package bench

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"
)

// SpecPnLCalculator computes P&L using spec-compliant entry/exit logic
type SpecPnLCalculator struct {
	regime       string
	gatesConfig  GatesConfig
	guardsConfig GuardsConfig
	seriesSource SeriesSource
}

// SpecPnLResult represents spec-compliant P&L calculation
type SpecPnLResult struct {
	Symbol       string    `json:"symbol"`
	EntryTime    time.Time `json:"entry_time"`
	EntryPrice   float64   `json:"entry_price"`
	ExitTime     time.Time `json:"exit_time"`
	ExitPrice    float64   `json:"exit_price"`
	SpecPnLPct   float64   `json:"spec_pnl_pct"`
	Raw24hChange float64   `json:"raw_24h_change"`
	ExitReason   string    `json:"exit_reason"`
	SeriesSource string    `json:"series_source"`
	EntryValid   bool      `json:"entry_valid"`
	ExitValid    bool      `json:"exit_valid"`
}

// GatesConfig represents gate configuration for simulation
type GatesConfig struct {
	MinScore     float64 `json:"min_score"`
	MaxSpreadBps float64 `json:"max_spread_bps"`
	MinDepthUSD  float64 `json:"min_depth_usd"`
	MinVADR      float64 `json:"min_vadr"`
}

// GuardsConfig represents guard configuration for simulation
type GuardsConfig struct {
	FatigueThreshold float64 `json:"fatigue_threshold"`
	RSIThreshold     float64 `json:"rsi_threshold"`
	MaxBarsAge       int     `json:"max_bars_age"`
	MaxDelaySeconds  int     `json:"max_delay_seconds"`
	ATRFactor        float64 `json:"atr_factor"`
}

// SeriesSource provides exchange-native price data
type SeriesSource struct {
	ExchangeNativeFirst bool     `json:"exchange_native_first"`
	PreferredExchanges  []string `json:"preferred_exchanges"`
	FallbackAggregators []string `json:"fallback_aggregators"`
}

// NewSpecPnLCalculator creates a new spec-compliant P&L calculator
func NewSpecPnLCalculator(regime string, gatesConfig GatesConfig, guardsConfig GuardsConfig, seriesSource SeriesSource) *SpecPnLCalculator {
	return &SpecPnLCalculator{
		regime:       regime,
		gatesConfig:  gatesConfig,
		guardsConfig: guardsConfig,
		seriesSource: seriesSource,
	}
}

// CalculateSpecPnL computes spec-compliant P&L for a symbol
func (calc *SpecPnLCalculator) CalculateSpecPnL(ctx context.Context, symbol string, signalTime time.Time, raw24hChange float64) (*SpecPnLResult, error) {
	log.Debug().
		Str("symbol", symbol).
		Time("signal_time", signalTime).
		Str("regime", calc.regime).
		Msg("Starting spec-compliant P&L calculation")

	// Get price series from exchange-native source
	priceSeries, seriesLabel, err := calc.getPriceSeries(ctx, symbol, signalTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get price series for %s: %w", symbol, err)
	}

	// Find valid entry point (first bar after signal that passes all gates/guards)
	entryPoint, err := calc.findValidEntry(ctx, symbol, signalTime, priceSeries)
	if err != nil {
		return &SpecPnLResult{
			Symbol:       symbol,
			Raw24hChange: raw24hChange,
			SeriesSource: seriesLabel,
			EntryValid:   false,
			ExitValid:    false,
		}, nil
	}

	// Find exit point using hierarchy: hard stop, venue-health, 48h limit, accel-reversal, momentum fade, trailing/targets
	exitPoint, exitReason, err := calc.findValidExit(ctx, symbol, entryPoint, priceSeries)
	if err != nil {
		return &SpecPnLResult{
			Symbol:       symbol,
			EntryTime:    entryPoint.Timestamp,
			EntryPrice:   entryPoint.Price,
			Raw24hChange: raw24hChange,
			SeriesSource: seriesLabel,
			EntryValid:   true,
			ExitValid:    false,
		}, nil
	}

	// Calculate spec-compliant P&L
	specPnLPct := ((exitPoint.Price - entryPoint.Price) / entryPoint.Price) * 100.0

	result := &SpecPnLResult{
		Symbol:       symbol,
		EntryTime:    entryPoint.Timestamp,
		EntryPrice:   entryPoint.Price,
		ExitTime:     exitPoint.Timestamp,
		ExitPrice:    exitPoint.Price,
		SpecPnLPct:   specPnLPct,
		Raw24hChange: raw24hChange,
		ExitReason:   exitReason,
		SeriesSource: seriesLabel,
		EntryValid:   true,
		ExitValid:    true,
	}

	log.Debug().
		Str("symbol", symbol).
		Float64("spec_pnl_pct", specPnLPct).
		Float64("raw_24h_change", raw24hChange).
		Str("exit_reason", exitReason).
		Msg("Spec-compliant P&L calculation completed")

	return result, nil
}

// PriceBar represents a single price bar from exchange-native source
type PriceBar struct {
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Volume    float64   `json:"volume"`
	SpreadBps float64   `json:"spread_bps"`
	DepthUSD  float64   `json:"depth_usd"`
	VADR      float64   `json:"vadr"`
}

// getPriceSeries retrieves exchange-native price data with fallback labeling
func (calc *SpecPnLCalculator) getPriceSeries(ctx context.Context, symbol string, signalTime time.Time) ([]PriceBar, string, error) {
	// Try exchange-native sources first
	if calc.seriesSource.ExchangeNativeFirst {
		for _, exchange := range calc.seriesSource.PreferredExchanges {
			series, err := calc.fetchExchangeNativeSeries(ctx, symbol, signalTime, exchange)
			if err == nil && len(series) > 0 {
				return series, fmt.Sprintf("exchange_native_%s", exchange), nil
			}
			log.Warn().Err(err).Str("exchange", exchange).Str("symbol", symbol).
				Msg("Failed to fetch from exchange-native source")
		}
	}

	// Fallback to aggregators (must be labeled)
	for _, aggregator := range calc.seriesSource.FallbackAggregators {
		series, err := calc.fetchAggregatorSeries(ctx, symbol, signalTime, aggregator)
		if err == nil && len(series) > 0 {
			log.Warn().Str("aggregator", aggregator).Str("symbol", symbol).
				Msg("Using aggregator fallback for price series")
			return series, fmt.Sprintf("aggregator_fallback_%s", aggregator), nil
		}
	}

	return nil, "", fmt.Errorf("no price series available for %s", symbol)
}

// fetchExchangeNativeSeries fetches price data from exchange-native API
func (calc *SpecPnLCalculator) fetchExchangeNativeSeries(ctx context.Context, symbol string, signalTime time.Time, exchange string) ([]PriceBar, error) {
	// Mock implementation - in production would call actual exchange APIs
	startTime := signalTime.Add(-1 * time.Hour)
	_ = signalTime.Add(49 * time.Hour) // Cover 48h exit limit + buffer

	var bars []PriceBar
	basePrice := 100.0 // Mock base price

	// Generate mock exchange-native bars with realistic microstructure data
	for i := 0; i < 50; i++ {
		timestamp := startTime.Add(time.Duration(i) * time.Hour)

		// Simulate price movement
		priceMove := math.Sin(float64(i)*0.1) * 2.0 // ±2% movement
		price := basePrice * (1.0 + priceMove/100.0)

		bar := PriceBar{
			Timestamp: timestamp,
			Price:     price,
			High:      price * 1.01,
			Low:       price * 0.99,
			Volume:    1000000 + float64(i*10000),
			SpreadBps: 25.0 + float64(i%10),     // 25-35 bps
			DepthUSD:  120000 + float64(i*5000), // $120k-$365k
			VADR:      1.8 + float64(i%5)*0.1,   // 1.8-2.2x
		}

		bars = append(bars, bar)
	}

	log.Debug().Str("exchange", exchange).Str("symbol", symbol).Int("bars", len(bars)).
		Msg("Fetched exchange-native price series")

	return bars, nil
}

// fetchAggregatorSeries fetches price data from aggregator with fallback labeling
func (calc *SpecPnLCalculator) fetchAggregatorSeries(ctx context.Context, symbol string, signalTime time.Time, aggregator string) ([]PriceBar, error) {
	// Mock implementation - would fetch from aggregator APIs
	// Note: Aggregator data lacks microstructure details
	startTime := signalTime.Add(-1 * time.Hour)
	_ = signalTime.Add(49 * time.Hour)

	var bars []PriceBar
	basePrice := 100.0

	for i := 0; i < 50; i++ {
		timestamp := startTime.Add(time.Duration(i) * time.Hour)
		priceMove := math.Sin(float64(i)*0.1) * 2.0
		price := basePrice * (1.0 + priceMove/100.0)

		bar := PriceBar{
			Timestamp: timestamp,
			Price:     price,
			High:      price * 1.01,
			Low:       price * 0.99,
			Volume:    800000, // Lower quality volume data
			SpreadBps: 45.0,   // Wider spreads (estimated)
			DepthUSD:  80000,  // Lower depth estimates
			VADR:      1.5,    // Lower VADR estimates
		}

		bars = append(bars, bar)
	}

	log.Debug().Str("aggregator", aggregator).Str("symbol", symbol).Int("bars", len(bars)).
		Msg("Fetched aggregator fallback price series")

	return bars, nil
}

// findValidEntry finds first bar after signal that passes all gates/guards
func (calc *SpecPnLCalculator) findValidEntry(ctx context.Context, symbol string, signalTime time.Time, priceSeries []PriceBar) (*PriceBar, error) {
	for i, bar := range priceSeries {
		if bar.Timestamp.Before(signalTime) {
			continue // Skip bars before signal
		}

		// Check all gates/guards for this potential entry
		if calc.passesAllGatesAndGuards(ctx, symbol, bar, priceSeries, i) {
			log.Debug().Str("symbol", symbol).Time("entry_time", bar.Timestamp).
				Float64("entry_price", bar.Price).Msg("Found valid entry point")
			return &bar, nil
		}
	}

	return nil, fmt.Errorf("no valid entry found for %s after signal time", symbol)
}

// findValidExit finds exit using hierarchy: hard stop, venue-health, 48h limit, accel-reversal, momentum fade, trailing/targets
func (calc *SpecPnLCalculator) findValidExit(ctx context.Context, symbol string, entryPoint *PriceBar, priceSeries []PriceBar) (*PriceBar, string, error) {
	maxExitTime := entryPoint.Timestamp.Add(48 * time.Hour)

	for i, bar := range priceSeries {
		if bar.Timestamp.Before(entryPoint.Timestamp) || bar.Timestamp.Equal(entryPoint.Timestamp) {
			continue // Skip bars at/before entry
		}

		// Exit hierarchy (checked in priority order)

		// 1. Hard stop (5% loss)
		lossPercent := ((bar.Price - entryPoint.Price) / entryPoint.Price) * 100.0
		if lossPercent <= -5.0 {
			log.Debug().Str("symbol", symbol).Float64("loss_pct", lossPercent).
				Msg("Hard stop triggered")
			return &bar, "hard_stop", nil
		}

		// 2. Venue health exit (spread >100bps or depth <$50k)
		if bar.SpreadBps > 100.0 || bar.DepthUSD < 50000.0 {
			log.Debug().Str("symbol", symbol).Float64("spread_bps", bar.SpreadBps).
				Float64("depth_usd", bar.DepthUSD).Msg("Venue health exit triggered")
			return &bar, "venue_health", nil
		}

		// 3. 48h time limit
		if bar.Timestamp.After(maxExitTime) {
			log.Debug().Str("symbol", symbol).Time("exit_time", bar.Timestamp).
				Msg("48h time limit reached")
			return &bar, "time_limit_48h", nil
		}

		// 4. Acceleration reversal (momentum turning negative)
		if calc.detectAccelReversal(ctx, symbol, entryPoint, bar, priceSeries, i) {
			return &bar, "accel_reversal", nil
		}

		// 5. Momentum fade (RSI overbought)
		if calc.detectMomentumFade(ctx, symbol, bar, priceSeries, i) {
			return &bar, "momentum_fade", nil
		}

		// 6. Trailing stop/profit target (15% gain)
		gainPercent := ((bar.Price - entryPoint.Price) / entryPoint.Price) * 100.0
		if gainPercent >= 15.0 {
			log.Debug().Str("symbol", symbol).Float64("gain_pct", gainPercent).
				Msg("Profit target reached")
			return &bar, "profit_target", nil
		}
	}

	// If no exit condition met, use last available bar
	if len(priceSeries) > 0 {
		lastBar := priceSeries[len(priceSeries)-1]
		return &lastBar, "series_end", nil
	}

	return nil, "", fmt.Errorf("no exit point found for %s", symbol)
}

// passesAllGatesAndGuards checks if a bar passes all entry requirements
func (calc *SpecPnLCalculator) passesAllGatesAndGuards(ctx context.Context, symbol string, bar PriceBar, series []PriceBar, index int) bool {
	// Mock score (would be computed from actual factors)
	mockScore := 2.5

	// Gates checks
	if mockScore < calc.gatesConfig.MinScore {
		return false
	}
	if bar.SpreadBps > calc.gatesConfig.MaxSpreadBps {
		return false
	}
	if bar.DepthUSD < calc.gatesConfig.MinDepthUSD {
		return false
	}
	if bar.VADR < calc.gatesConfig.MinVADR {
		return false
	}

	// Guards checks (regime-aware)
	fatigueThreshold := calc.gatesConfig.MinScore // Simplified
	if calc.regime == "trending" {
		fatigueThreshold = calc.guardsConfig.FatigueThreshold * 1.5 // Relaxed for trending
	}

	// Mock momentum and RSI values
	momentum24h := 8.0 // Mock value
	rsi4h := 65.0      // Mock value

	if momentum24h > fatigueThreshold && rsi4h > calc.guardsConfig.RSIThreshold {
		return false // Fatigue guard
	}

	// Freshness guard (simplified)
	if index > calc.guardsConfig.MaxBarsAge {
		return false
	}

	return true
}

// detectAccelReversal checks for momentum acceleration reversal
func (calc *SpecPnLCalculator) detectAccelReversal(ctx context.Context, symbol string, entryPoint *PriceBar, currentBar PriceBar, series []PriceBar, index int) bool {
	if index < 3 {
		return false // Need at least 3 bars for acceleration calculation
	}

	// Calculate recent momentum change (simplified)
	prev3 := series[index-3].Price
	prev1 := series[index-1].Price
	current := currentBar.Price

	accel1 := (prev1 - prev3) / prev3
	accel2 := (current - prev1) / prev1

	// Reversal detected if acceleration turned negative
	return accel1 > 0 && accel2 < -0.02 // 2% deceleration threshold
}

// detectMomentumFade checks for momentum fade conditions
func (calc *SpecPnLCalculator) detectMomentumFade(ctx context.Context, symbol string, bar PriceBar, series []PriceBar, index int) bool {
	// Mock RSI calculation - would compute actual RSI
	mockRSI := 75.0 + float64(index%10) // Mock overbought condition

	return mockRSI > 80.0 // RSI overbought threshold
}
