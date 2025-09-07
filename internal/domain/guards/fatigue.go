package guards

import (
	"fmt"
)

// EvaluateFatigueGuard checks if position is overextended with regime awareness
func EvaluateFatigueGuard(inputs FatigueInputs, config FatigueConfig, regimeAware bool) GuardResult {
	// Select threshold profile based on regime and feature flag
	profile := "baseline"
	thresholds := config.Baseline

	if regimeAware && inputs.Regime == RegimeTrending {
		// Trending profile ONLY if accel_renewal=true (safety condition)
		if config.TrendingProfile.RequiresAccelRenewal && inputs.AccelRenewal {
			profile = "trending"
			thresholds = config.TrendingProfile
		}
	}

	// Apply safety constraints (hard limits override regime profiles)
	momentumThreshold := thresholds.Momentum24hThreshold
	if momentumThreshold > config.MaxMomentum {
		momentumThreshold = config.MaxMomentum
	}

	rsiThreshold := thresholds.RSI4hThreshold
	if rsiThreshold > config.MaxRSI {
		rsiThreshold = config.MaxRSI
	}

	// Evaluation logic: Block if momentum > threshold AND RSI > threshold
	momentumHigh := inputs.Momentum24h > momentumThreshold
	rsiHigh := inputs.RSI4h > rsiThreshold
	accelerationOverride := inputs.Acceleration >= thresholds.AccelerationOverride

	// Block if both conditions met AND no acceleration override
	shouldBlock := momentumHigh && rsiHigh && !accelerationOverride

	// Build result details
	details := map[string]interface{}{
		"momentum_24h":          inputs.Momentum24h,
		"momentum_threshold":    momentumThreshold,
		"rsi_4h":                inputs.RSI4h,
		"rsi_threshold":         rsiThreshold,
		"acceleration":          inputs.Acceleration,
		"acceleration_override": thresholds.AccelerationOverride,
		"accel_renewal":         inputs.AccelRenewal,
		"momentum_high":         momentumHigh,
		"rsi_high":              rsiHigh,
		"acceleration_saves":    accelerationOverride,
	}

	// Generate reason string
	var reason string
	if !shouldBlock {
		if !momentumHigh {
			reason = fmt.Sprintf("momentum_ok (%.1f%% <= %.1f%%)",
				inputs.Momentum24h, momentumThreshold)
		} else if !rsiHigh {
			reason = fmt.Sprintf("rsi_ok (%.1f <= %.1f)",
				inputs.RSI4h, rsiThreshold)
		} else if accelerationOverride {
			reason = fmt.Sprintf("acceleration_override (%.1f%% >= %.1f%%)",
				inputs.Acceleration, thresholds.AccelerationOverride)
		} else {
			reason = "conditions_not_met"
		}
	} else {
		reason = fmt.Sprintf("overextended (24h=%.1f%% > %.1f%%, RSI=%.1f > %.1f, accel=%.1f%% < %.1f%%)",
			inputs.Momentum24h, momentumThreshold, inputs.RSI4h, rsiThreshold,
			inputs.Acceleration, thresholds.AccelerationOverride)
	}

	return GuardResult{
		Allow:   !shouldBlock,
		Reason:  reason,
		Profile: profile,
		Regime:  inputs.Regime,
		Details: details,
	}
}
