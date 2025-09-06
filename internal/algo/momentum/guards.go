package momentum

import (
	"time"
)

// ApplyGuards applies fatigue, freshness, and late-fill guards
func (mc *MomentumCore) ApplyGuards(data map[string][]MarketData, result *MomentumResult) GuardResults {
	guards := GuardResults{
		Fatigue:   mc.ApplyFatigueGuard(data, result),
		Freshness: mc.ApplyFreshnessGuard(data, result),
		LateFill:  mc.ApplyLateFillGuard(data, result),
	}
	return guards
}

// ApplyFatigueGuard implements fatigue guard: block if 24h > +12% & RSI(4h) > 70 unless accel ↑
func (mc *MomentumCore) ApplyFatigueGuard(data map[string][]MarketData, result *MomentumResult) GuardResult {
	// Get 24h data
	tf24h, exists := data["24h"]
	if !exists || len(tf24h) < 2 {
		return GuardResult{
			Pass:   true,
			Value:  0.0,
			Reason: "insufficient 24h data",
		}
	}

	// Calculate 24h return
	current := tf24h[len(tf24h)-1].Close
	previous := tf24h[len(tf24h)-2].Close
	if previous == 0 {
		return GuardResult{
			Pass:   true,
			Value:  0.0,
			Reason: "invalid 24h price data",
		}
	}

	return24h := (current - previous) / previous * 100.0

	// Check if 24h return exceeds threshold
	if return24h <= mc.config.Fatigue.Return24hThreshold {
		return GuardResult{
			Pass:   true,
			Value:  return24h,
			Reason: "24h return below fatigue threshold",
		}
	}

	// Get 4h data for RSI calculation
	tf4h, exists := data["4h"]
	if !exists || len(tf4h) < 15 {
		return GuardResult{
			Pass:   true,
			Value:  return24h,
			Reason: "insufficient 4h data for RSI",
		}
	}

	// Calculate RSI(4h)
	rsi4h := calculateRSI(tf4h, 14)

	// Check RSI threshold
	if rsi4h <= mc.config.Fatigue.RSI4hThreshold {
		return GuardResult{
			Pass:   true,
			Value:  return24h,
			Reason: "RSI below fatigue threshold",
		}
	}

	// Check acceleration renewal if enabled
	if mc.config.Fatigue.AccelRenewal && result.Acceleration4h > 0 {
		return GuardResult{
			Pass:   true,
			Value:  return24h,
			Reason: "acceleration renewal override",
		}
	}

	// Fatigue guard triggered
	return GuardResult{
		Pass:   false,
		Value:  return24h,
		Reason: "fatigue guard: 24h return excessive with high RSI",
	}
}

// ApplyFreshnessGuard implements freshness guard: ≤2 bars old & within 1.2×ATR(1h)
func (mc *MomentumCore) ApplyFreshnessGuard(data map[string][]MarketData, result *MomentumResult) GuardResult {
	tf1h, exists := data["1h"]
	if !exists || len(tf1h) < mc.config.Freshness.ATRWindow+2 {
		return GuardResult{
			Pass:   false,
			Value:  0.0,
			Reason: "insufficient 1h data for freshness check",
		}
	}

	// Check data freshness (assume data is ordered by time)
	latestBar := tf1h[len(tf1h)-1]
	now := time.Now()

	// Calculate bars age (assuming 1h bars)
	barsAge := int(now.Sub(latestBar.Timestamp).Hours())

	if barsAge > mc.config.Freshness.MaxBarsAge {
		return GuardResult{
			Pass:   false,
			Value:  float64(barsAge),
			Reason: "data too old for fresh signal",
		}
	}

	// Calculate ATR and check price movement
	atr := calculateATR(tf1h, mc.config.Freshness.ATRWindow)
	if atr == 0 {
		return GuardResult{
			Pass:   false,
			Value:  0.0,
			Reason: "invalid ATR calculation",
		}
	}

	// Check if latest price movement is within acceptable range
	if len(tf1h) >= 2 {
		priceMove := tf1h[len(tf1h)-1].Close - tf1h[len(tf1h)-2].Close
		maxMove := atr * mc.config.Freshness.ATRFactor

		if abs(priceMove) > maxMove {
			return GuardResult{
				Pass:   false,
				Value:  abs(priceMove) / atr,
				Reason: "price movement exceeds ATR threshold",
			}
		}
	}

	return GuardResult{
		Pass:   true,
		Value:  float64(barsAge),
		Reason: "data fresh and within ATR range",
	}
}

// ApplyLateFillGuard implements late-fill guard: reject fills >30s after signal bar close
func (mc *MomentumCore) ApplyLateFillGuard(data map[string][]MarketData, result *MomentumResult) GuardResult {
	tf1h, exists := data["1h"]
	if !exists || len(tf1h) == 0 {
		return GuardResult{
			Pass:   false,
			Value:  0.0,
			Reason: "no 1h data for late-fill check",
		}
	}

	// Get the latest signal bar
	latestBar := tf1h[len(tf1h)-1]

	// Calculate time since bar close (assuming hourly bars close on the hour)
	barCloseTime := latestBar.Timestamp.Truncate(time.Hour).Add(time.Hour)
	now := time.Now()
	timeSinceClose := now.Sub(barCloseTime)

	maxDelay := time.Duration(mc.config.LateFill.MaxDelaySeconds) * time.Second

	if timeSinceClose > maxDelay {
		return GuardResult{
			Pass:   false,
			Value:  timeSinceClose.Seconds(),
			Reason: "signal too late after bar close",
		}
	}

	return GuardResult{
		Pass:   true,
		Value:  timeSinceClose.Seconds(),
		Reason: "signal timing acceptable",
	}
}

// abs returns absolute value of float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
