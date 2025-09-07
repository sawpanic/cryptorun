package guards

// GuardEvaluator orchestrates all guard evaluations with regime awareness
type GuardEvaluator struct {
	config      GuardConfig
	regimeAware bool
}

// NewGuardEvaluator creates a new guard evaluator
func NewGuardEvaluator(config GuardConfig) *GuardEvaluator {
	return &GuardEvaluator{
		config:      config,
		regimeAware: config.RegimeAware,
	}
}

// EvaluateAllGuards runs all guards and returns combined result
func (ge *GuardEvaluator) EvaluateAllGuards(inputs AllGuardsInputs) AllGuardsResult {
	// Evaluate each guard independently
	fatigueResult := EvaluateFatigueGuard(inputs.Fatigue, ge.config.Fatigue, ge.regimeAware)
	lateFillResult := EvaluateLateFillGuard(inputs.LateFill, ge.config.LateFill, ge.regimeAware)
	freshnessResult := EvaluateFreshnessGuard(inputs.Freshness, ge.config.Freshness, ge.regimeAware)

	// Collect all results
	guardResults := map[string]GuardResult{
		"fatigue":   fatigueResult,
		"late_fill": lateFillResult,
		"freshness": freshnessResult,
	}

	// Determine overall result (all must pass)
	allowEntry := fatigueResult.Allow && lateFillResult.Allow && freshnessResult.Allow

	// Find first blocking guard (order matters for deterministic results)
	var blockReason, blockedBy, profile string
	var regime Regime

	if !allowEntry {
		// Check guards in order of priority
		guards := []struct {
			name   string
			result GuardResult
		}{
			{"fatigue", fatigueResult},
			{"late_fill", lateFillResult},
			{"freshness", freshnessResult},
		}

		for _, guard := range guards {
			if !guard.result.Allow {
				blockReason = guard.result.Reason
				blockedBy = guard.name
				profile = guard.result.Profile
				regime = guard.result.Regime
				break
			}
		}
	} else {
		// Use regime from any guard (they should all be the same)
		regime = fatigueResult.Regime

		// Determine profile used (preference: trending > baseline)
		if fatigueResult.Profile == "trending" || lateFillResult.Profile == "trending" ||
			freshnessResult.Profile == "trending" {
			profile = "trending"
		} else {
			profile = "baseline"
		}

		blockReason = "all_guards_passed"
	}

	return AllGuardsResult{
		AllowEntry:   allowEntry,
		BlockReason:  blockReason,
		BlockedBy:    blockedBy,
		Profile:      profile,
		Regime:       regime,
		GuardResults: guardResults,
	}
}

// GetEffectiveThresholds returns the thresholds that would be used for given regime
func (ge *GuardEvaluator) GetEffectiveThresholds(regime Regime, accelRenewal bool, infraP99MS, atrDistance, vadr, spreadBps float64) map[string]interface{} {
	// Determine which profiles would be active
	fatigueProfile := "baseline"
	if ge.regimeAware && regime == RegimeTrending &&
		ge.config.Fatigue.TrendingProfile.RequiresAccelRenewal && accelRenewal {
		fatigueProfile = "trending"
	}

	lateFillProfile := "baseline"
	if ge.regimeAware && regime == RegimeTrending {
		infraHealthOK := !ge.config.LateFill.TrendingProfile.RequiresInfraHealth ||
			infraP99MS < 400.0
		atrProximityOK := !ge.config.LateFill.TrendingProfile.RequiresATRProximity ||
			atrDistance <= ge.config.LateFill.TrendingProfile.ATRFactor
		if infraHealthOK && atrProximityOK {
			lateFillProfile = "trending"
		}
	}

	freshnessProfile := "baseline"
	if ge.regimeAware && regime == RegimeTrending {
		vadrOK := vadr >= ge.config.Freshness.TrendingProfile.RequiresVADR
		spreadOK := !ge.config.Freshness.TrendingProfile.RequiresTightSpread ||
			spreadBps <= ge.config.Freshness.TrendingProfile.SpreadThresholdBps
		if vadrOK && spreadOK {
			freshnessProfile = "trending"
		}
	}

	// Get effective thresholds for each guard
	fatigueThresholds := ge.config.Fatigue.Baseline
	if fatigueProfile == "trending" {
		fatigueThresholds = ge.config.Fatigue.TrendingProfile
	}

	lateFillThresholds := ge.config.LateFill.Baseline
	if lateFillProfile == "trending" {
		lateFillThresholds = ge.config.LateFill.TrendingProfile
	}

	freshnessThresholds := ge.config.Freshness.Baseline
	if freshnessProfile == "trending" {
		freshnessThresholds = ge.config.Freshness.TrendingProfile
	}

	return map[string]interface{}{
		"regime":            regime,
		"regime_aware":      ge.regimeAware,
		"fatigue_profile":   fatigueProfile,
		"late_fill_profile": lateFillProfile,
		"freshness_profile": freshnessProfile,
		"fatigue": map[string]interface{}{
			"momentum_24h_threshold": fatigueThresholds.Momentum24hThreshold,
			"rsi_4h_threshold":       fatigueThresholds.RSI4hThreshold,
			"acceleration_override":  fatigueThresholds.AccelerationOverride,
		},
		"late_fill": map[string]interface{}{
			"max_delay_seconds": lateFillThresholds.MaxDelaySeconds,
		},
		"freshness": map[string]interface{}{
			"max_bars_age": freshnessThresholds.MaxBarsAge,
			"atr_factor":   freshnessThresholds.ATRFactor,
		},
	}
}

// SetRegimeAware allows runtime toggling of regime awareness
func (ge *GuardEvaluator) SetRegimeAware(enabled bool) {
	ge.regimeAware = enabled
}

// IsRegimeAware returns current regime awareness setting
func (ge *GuardEvaluator) IsRegimeAware() bool {
	return ge.regimeAware
}
