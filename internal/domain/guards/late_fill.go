package guards

import (
	"fmt"
)

// EvaluateLateFillGuard checks execution timing with regime awareness
func EvaluateLateFillGuard(inputs LateFillInputs, config LateFillConfig, regimeAware bool) GuardResult {
	// Calculate execution delay
	delay := inputs.ExecutionTime.Sub(inputs.SignalTime)
	delaySeconds := int(delay.Seconds())

	// Handle negative delays (clock skew)
	if delaySeconds < config.MinDelaySeconds {
		return GuardResult{
			Allow:   false,
			Reason:  fmt.Sprintf("clock_skew (delay=%ds < %ds)", delaySeconds, config.MinDelaySeconds),
			Profile: "baseline",
			Regime:  inputs.Regime,
			Details: map[string]interface{}{
				"delay_seconds": delaySeconds,
				"clock_skew":    true,
			},
		}
	}

	// Select threshold profile based on regime and feature flag
	profile := "baseline"
	thresholds := config.Baseline

	if regimeAware && inputs.Regime == RegimeTrending {
		// Check safety conditions for trending profile
		infraHealthOK := !config.TrendingProfile.RequiresInfraHealth ||
			inputs.InfraP99MS < 400.0
		atrProximityOK := !config.TrendingProfile.RequiresATRProximity ||
			inputs.ATRDistance <= config.TrendingProfile.ATRFactor

		if infraHealthOK && atrProximityOK {
			profile = "trending"
			thresholds = config.TrendingProfile
		}
	}

	// Apply safety constraints (hard limits override regime profiles)
	maxDelay := thresholds.MaxDelaySeconds
	if maxDelay > config.MaxDelaySecondsAbs {
		maxDelay = config.MaxDelaySecondsAbs
	}

	// Evaluation logic: Block if delay >= threshold
	shouldBlock := delaySeconds >= maxDelay

	// Build result details
	details := map[string]interface{}{
		"delay_seconds":    delaySeconds,
		"max_delay":        maxDelay,
		"infra_p99_ms":     inputs.InfraP99MS,
		"atr_distance":     inputs.ATRDistance,
		"atr_factor_limit": config.TrendingProfile.ATRFactor,
	}

	// Add trending profile safety condition details
	if regimeAware && inputs.Regime == RegimeTrending {
		details["infra_health_ok"] = !config.TrendingProfile.RequiresInfraHealth ||
			inputs.InfraP99MS < 400.0
		details["atr_proximity_ok"] = !config.TrendingProfile.RequiresATRProximity ||
			inputs.ATRDistance <= config.TrendingProfile.ATRFactor
	}

	// Generate reason string
	var reason string
	if !shouldBlock {
		reason = fmt.Sprintf("timing_ok (%ds < %ds)", delaySeconds, maxDelay)
	} else {
		reason = fmt.Sprintf("too_late (%ds >= %ds)", delaySeconds, maxDelay)
	}

	return GuardResult{
		Allow:   !shouldBlock,
		Reason:  reason,
		Profile: profile,
		Regime:  inputs.Regime,
		Details: details,
	}
}
