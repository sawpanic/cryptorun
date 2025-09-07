package guards

import (
	"fmt"
)

// EvaluateFreshnessGuard checks signal staleness with regime awareness
func EvaluateFreshnessGuard(inputs FreshnessInputs, config FreshnessConfig, regimeAware bool) GuardResult {
	// Select threshold profile based on regime and feature flag
	profile := "baseline"
	thresholds := config.Baseline

	if regimeAware && inputs.Regime == RegimeTrending {
		// Check safety conditions for trending profile
		vadrOK := inputs.VADR >= config.TrendingProfile.RequiresVADR
		spreadOK := !config.TrendingProfile.RequiresTightSpread ||
			inputs.SpreadBps <= config.TrendingProfile.SpreadThresholdBps

		if vadrOK && spreadOK {
			profile = "trending"
			thresholds = config.TrendingProfile
		}
	}

	// Apply safety constraints (hard limits override regime profiles)
	maxBarsAge := thresholds.MaxBarsAge
	if maxBarsAge > config.MaxBarsAgeAbs {
		maxBarsAge = config.MaxBarsAgeAbs
	}

	atrFactor := thresholds.ATRFactor
	if atrFactor < config.MinATRFactor {
		atrFactor = config.MinATRFactor
	}

	// Evaluation logic: Block if too old OR price moved too much
	ageExceeded := inputs.BarsAge > maxBarsAge

	var priceMovedTooMuch bool
	var atrMultiple float64
	if inputs.ATR1h > 0 {
		atrMultiple = inputs.PriceChange / inputs.ATR1h
		priceMovedTooMuch = atrMultiple > atrFactor
	}

	shouldBlock := ageExceeded || priceMovedTooMuch

	// Build result details
	details := map[string]interface{}{
		"bars_age":             inputs.BarsAge,
		"max_bars_age":         maxBarsAge,
		"price_change":         inputs.PriceChange,
		"atr_1h":               inputs.ATR1h,
		"atr_multiple":         atrMultiple,
		"atr_factor_limit":     atrFactor,
		"vadr":                 inputs.VADR,
		"spread_bps":           inputs.SpreadBps,
		"age_exceeded":         ageExceeded,
		"price_moved_too_much": priceMovedTooMuch,
	}

	// Add trending profile safety condition details
	if regimeAware && inputs.Regime == RegimeTrending {
		details["vadr_ok"] = inputs.VADR >= config.TrendingProfile.RequiresVADR
		details["spread_ok"] = !config.TrendingProfile.RequiresTightSpread ||
			inputs.SpreadBps <= config.TrendingProfile.SpreadThresholdBps
		details["vadr_required"] = config.TrendingProfile.RequiresVADR
		details["spread_threshold_bps"] = config.TrendingProfile.SpreadThresholdBps
	}

	// Generate reason string
	var reason string
	if !shouldBlock {
		reason = fmt.Sprintf("fresh (age=%d <= %d bars, price=%.2fx <= %.2fx ATR)",
			inputs.BarsAge, maxBarsAge, atrMultiple, atrFactor)
	} else if ageExceeded {
		reason = fmt.Sprintf("too_old (%d > %d bars)", inputs.BarsAge, maxBarsAge)
	} else if priceMovedTooMuch {
		reason = fmt.Sprintf("price_moved_too_much (%.2fx > %.2fx ATR)", atrMultiple, atrFactor)
	} else {
		reason = "stale_signal"
	}

	return GuardResult{
		Allow:   !shouldBlock,
		Reason:  reason,
		Profile: profile,
		Regime:  inputs.Regime,
		Details: details,
	}
}
