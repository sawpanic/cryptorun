package guards

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// RegimeAwareGuards provides regime-specific guard evaluation with feature flag control
type RegimeAwareGuards struct {
	enabled      bool
	profiles     RegimeProfiles
	safetyLimits SafetyLimits
}

// RegimeProfiles defines guard thresholds per market regime
type RegimeProfiles struct {
	Trending RegimeProfile `yaml:"trending"`
	Chop     RegimeProfile `yaml:"chop"`
	HighVol  RegimeProfile `yaml:"high_vol"`
}

// RegimeProfile contains guard thresholds for a specific regime
type RegimeProfile struct {
	Fatigue   FatigueProfile   `yaml:"fatigue"`
	LateFill  LateFillProfile  `yaml:"late_fill"`
	Freshness FreshnessProfile `yaml:"freshness"`
}

// FatigueProfile defines fatigue guard parameters per regime
type FatigueProfile struct {
	MomentumThreshold    float64 `yaml:"momentum_threshold"`     // Base momentum threshold %
	RSIThreshold         float64 `yaml:"rsi_threshold"`          // RSI overbought threshold
	RequiresAccelRenewal bool    `yaml:"requires_accel_renewal"` // Trending-only: requires acceleration renewal
}

// LateFillProfile defines late-fill guard parameters per regime
type LateFillProfile struct {
	MaxDelaySeconds      int     `yaml:"max_delay_seconds"`      // Maximum fill delay in seconds
	RequiresInfraHealth  bool    `yaml:"requires_infra_health"`  // Trending-only: requires p99 < 400ms
	RequiresATRProximity bool    `yaml:"requires_atr_proximity"` // Trending-only: within 1.2×ATR
	ATRFactor            float64 `yaml:"atr_factor"`             // ATR proximity factor
}

// FreshnessProfile defines freshness guard parameters per regime
type FreshnessProfile struct {
	MaxBarsAge          int     `yaml:"max_bars_age"`          // Maximum data age in bars
	RequiresVADR        float64 `yaml:"requires_vadr"`         // Trending-only: minimum VADR
	RequiresTightSpread bool    `yaml:"requires_tight_spread"` // Trending-only: spread requirement
	SpreadThresholdBps  float64 `yaml:"spread_threshold_bps"`  // Maximum spread in basis points
}

// SafetyLimits define absolute safety constraints that regimes cannot exceed
type SafetyLimits struct {
	MaxMomentumThreshold float64 `yaml:"max_momentum_threshold"` // Absolute max momentum (25%)
	MaxRSIThreshold      float64 `yaml:"max_rsi_threshold"`      // Absolute max RSI (80)
	MaxDelaySecondsAbs   int     `yaml:"max_delay_seconds_abs"`  // Absolute max delay (60s)
	MaxBarsAgeAbs        int     `yaml:"max_bars_age_abs"`       // Absolute max bars age (5)
	MinATRFactor         float64 `yaml:"min_atr_factor"`         // Minimum ATR factor (0.8)
}

// GuardInputs contains all data needed for guard evaluation
type GuardInputs struct {
	Symbol          string        `json:"symbol"`
	Regime          string        `json:"regime"`
	Momentum24h     float64       `json:"momentum_24h"`
	RSI4h           float64       `json:"rsi_4h"`
	Acceleration    float64       `json:"acceleration"`
	FillDelay       time.Duration `json:"fill_delay"`
	InfraP99Latency time.Duration `json:"infra_p99_latency"`
	ATRDistance     float64       `json:"atr_distance"`
	ATR1h           float64       `json:"atr_1h"`
	BarsAge         int           `json:"bars_age"`
	VADR            float64       `json:"vadr"`
	SpreadBps       float64       `json:"spread_bps"`
	Timestamp       time.Time     `json:"timestamp"`
}

// RegimeGuardResult contains the result of regime-aware guard evaluation
type RegimeGuardResult struct {
	Passed        bool   `json:"passed"`
	GuardType     string `json:"guard_type"`
	Regime        string `json:"regime"`
	Reason        string `json:"reason,omitempty"`
	ThresholdUsed string `json:"threshold_used,omitempty"`
	RegimeAware   bool   `json:"regime_aware"`
}

// NewRegimeAwareGuards creates a new regime-aware guards evaluator
func NewRegimeAwareGuards(profiles RegimeProfiles, safetyLimits SafetyLimits) *RegimeAwareGuards {
	// Check feature flag from environment
	enabled := false
	if flagValue := os.Getenv("GUARDS_REGIME_AWARE"); flagValue != "" {
		if parsed, err := strconv.ParseBool(flagValue); err == nil {
			enabled = parsed
		}
	}

	return &RegimeAwareGuards{
		enabled:      enabled,
		profiles:     profiles,
		safetyLimits: safetyLimits,
	}
}

// IsEnabled returns whether regime-aware guards are enabled
func (g *RegimeAwareGuards) IsEnabled() bool {
	return g.enabled
}

// EvaluateFatigueGuard checks fatigue conditions with regime-aware thresholds
func (g *RegimeAwareGuards) EvaluateFatigueGuard(ctx context.Context, inputs GuardInputs) RegimeGuardResult {
	result := RegimeGuardResult{
		GuardType:   "fatigue",
		Regime:      inputs.Regime,
		RegimeAware: g.enabled,
	}

	var profile FatigueProfile
	var thresholdDesc string

	if g.enabled {
		// Use regime-specific thresholds
		switch inputs.Regime {
		case "TRENDING":
			profile = g.profiles.Trending.Fatigue
			thresholdDesc = "trending_profile"

			// TRENDING-specific safety condition: requires accel_renewal
			if profile.RequiresAccelRenewal && inputs.Acceleration <= 0 {
				result.Passed = false
				result.Reason = "trending_fatigue_requires_acceleration_renewal"
				result.ThresholdUsed = thresholdDesc
				return result
			}

		case "CHOP":
			profile = g.profiles.Chop.Fatigue
			thresholdDesc = "chop_profile"
		case "HIGH_VOL":
			profile = g.profiles.HighVol.Fatigue
			thresholdDesc = "high_vol_profile"
		default:
			// Fallback to chop for unknown regimes
			profile = g.profiles.Chop.Fatigue
			thresholdDesc = "chop_profile_fallback"
		}
	} else {
		// Use baseline thresholds (legacy behavior)
		profile = g.profiles.Chop.Fatigue // Chop = baseline
		thresholdDesc = "baseline_legacy"
	}

	// Enforce safety limits
	momentumThreshold := profile.MomentumThreshold
	rsiThreshold := profile.RSIThreshold

	if momentumThreshold > g.safetyLimits.MaxMomentumThreshold {
		momentumThreshold = g.safetyLimits.MaxMomentumThreshold
		log.Warn().Float64("requested", profile.MomentumThreshold).
			Float64("capped", momentumThreshold).
			Msg("Fatigue momentum threshold capped by safety limit")
	}

	if rsiThreshold > g.safetyLimits.MaxRSIThreshold {
		rsiThreshold = g.safetyLimits.MaxRSIThreshold
		log.Warn().Float64("requested", profile.RSIThreshold).
			Float64("capped", rsiThreshold).
			Msg("Fatigue RSI threshold capped by safety limit")
	}

	// Evaluate fatigue conditions
	momentumFatigued := inputs.Momentum24h > momentumThreshold
	rsiFatigued := inputs.RSI4h > rsiThreshold

	if momentumFatigued && rsiFatigued {
		result.Passed = false
		result.Reason = fmt.Sprintf("fatigue_detected_momentum_%.1f_rsi_%.1f", inputs.Momentum24h, inputs.RSI4h)
	} else {
		result.Passed = true
	}

	result.ThresholdUsed = fmt.Sprintf("%s_momentum_%.1f_rsi_%.1f", thresholdDesc, momentumThreshold, rsiThreshold)

	log.Debug().
		Str("symbol", inputs.Symbol).
		Str("regime", inputs.Regime).
		Bool("regime_aware", g.enabled).
		Float64("momentum_24h", inputs.Momentum24h).
		Float64("momentum_threshold", momentumThreshold).
		Float64("rsi_4h", inputs.RSI4h).
		Float64("rsi_threshold", rsiThreshold).
		Bool("passed", result.Passed).
		Msg("Fatigue guard evaluation completed")

	return result
}

// EvaluateLateFillGuard checks late-fill conditions with regime-aware thresholds
func (g *RegimeAwareGuards) EvaluateLateFillGuard(ctx context.Context, inputs GuardInputs) RegimeGuardResult {
	result := RegimeGuardResult{
		GuardType:   "late_fill",
		Regime:      inputs.Regime,
		RegimeAware: g.enabled,
	}

	var profile LateFillProfile
	var thresholdDesc string

	if g.enabled {
		// Use regime-specific thresholds
		switch inputs.Regime {
		case "TRENDING":
			profile = g.profiles.Trending.LateFill
			thresholdDesc = "trending_profile"

			// TRENDING-specific safety conditions
			if profile.RequiresInfraHealth && inputs.InfraP99Latency > 400*time.Millisecond {
				result.Passed = false
				result.Reason = fmt.Sprintf("trending_late_fill_requires_infra_health_p99_%dms", inputs.InfraP99Latency/time.Millisecond)
				result.ThresholdUsed = thresholdDesc
				return result
			}

			if profile.RequiresATRProximity && inputs.ATRDistance > profile.ATRFactor*inputs.ATR1h {
				result.Passed = false
				result.Reason = fmt.Sprintf("trending_late_fill_requires_atr_proximity_%.2fx", inputs.ATRDistance/inputs.ATR1h)
				result.ThresholdUsed = thresholdDesc
				return result
			}

		case "CHOP":
			profile = g.profiles.Chop.LateFill
			thresholdDesc = "chop_profile"
		case "HIGH_VOL":
			profile = g.profiles.HighVol.LateFill
			thresholdDesc = "high_vol_profile"
		default:
			profile = g.profiles.Chop.LateFill
			thresholdDesc = "chop_profile_fallback"
		}
	} else {
		// Use baseline thresholds (legacy behavior)
		profile = g.profiles.Chop.LateFill
		thresholdDesc = "baseline_legacy"
	}

	// Enforce safety limits
	maxDelaySeconds := profile.MaxDelaySeconds
	if maxDelaySeconds > g.safetyLimits.MaxDelaySecondsAbs {
		maxDelaySeconds = g.safetyLimits.MaxDelaySecondsAbs
		log.Warn().Int("requested", profile.MaxDelaySeconds).
			Int("capped", maxDelaySeconds).
			Msg("Late-fill delay threshold capped by safety limit")
	}

	// Evaluate late-fill condition
	fillDelaySeconds := int(inputs.FillDelay.Seconds())

	if fillDelaySeconds > maxDelaySeconds {
		result.Passed = false
		result.Reason = fmt.Sprintf("late_fill_delay_%ds_exceeds_%ds", fillDelaySeconds, maxDelaySeconds)
	} else {
		result.Passed = true
	}

	result.ThresholdUsed = fmt.Sprintf("%s_max_delay_%ds", thresholdDesc, maxDelaySeconds)

	log.Debug().
		Str("symbol", inputs.Symbol).
		Str("regime", inputs.Regime).
		Bool("regime_aware", g.enabled).
		Int("fill_delay_seconds", fillDelaySeconds).
		Int("max_delay_seconds", maxDelaySeconds).
		Bool("passed", result.Passed).
		Msg("Late-fill guard evaluation completed")

	return result
}

// EvaluateFreshnessGuard checks freshness conditions with regime-aware thresholds
func (g *RegimeAwareGuards) EvaluateFreshnessGuard(ctx context.Context, inputs GuardInputs) RegimeGuardResult {
	result := RegimeGuardResult{
		GuardType:   "freshness",
		Regime:      inputs.Regime,
		RegimeAware: g.enabled,
	}

	var profile FreshnessProfile
	var thresholdDesc string

	if g.enabled {
		// Use regime-specific thresholds
		switch inputs.Regime {
		case "TRENDING":
			profile = g.profiles.Trending.Freshness
			thresholdDesc = "trending_profile"

			// TRENDING-specific safety conditions
			if inputs.VADR < profile.RequiresVADR {
				result.Passed = false
				result.Reason = fmt.Sprintf("trending_freshness_requires_vadr_%.2f_got_%.2f", profile.RequiresVADR, inputs.VADR)
				result.ThresholdUsed = thresholdDesc
				return result
			}

			if profile.RequiresTightSpread && inputs.SpreadBps > profile.SpreadThresholdBps {
				result.Passed = false
				result.Reason = fmt.Sprintf("trending_freshness_requires_tight_spread_%.1fbps_got_%.1fbps", profile.SpreadThresholdBps, inputs.SpreadBps)
				result.ThresholdUsed = thresholdDesc
				return result
			}

		case "CHOP":
			profile = g.profiles.Chop.Freshness
			thresholdDesc = "chop_profile"
		case "HIGH_VOL":
			profile = g.profiles.HighVol.Freshness
			thresholdDesc = "high_vol_profile"
		default:
			profile = g.profiles.Chop.Freshness
			thresholdDesc = "chop_profile_fallback"
		}
	} else {
		// Use baseline thresholds (legacy behavior)
		profile = g.profiles.Chop.Freshness
		thresholdDesc = "baseline_legacy"
	}

	// Enforce safety limits
	maxBarsAge := profile.MaxBarsAge
	if maxBarsAge > g.safetyLimits.MaxBarsAgeAbs {
		maxBarsAge = g.safetyLimits.MaxBarsAgeAbs
		log.Warn().Int("requested", profile.MaxBarsAge).
			Int("capped", maxBarsAge).
			Msg("Freshness bars age threshold capped by safety limit")
	}

	// Evaluate freshness condition
	if inputs.BarsAge > maxBarsAge {
		result.Passed = false
		result.Reason = fmt.Sprintf("stale_data_%d_bars_exceeds_%d", inputs.BarsAge, maxBarsAge)
	} else {
		result.Passed = true
	}

	result.ThresholdUsed = fmt.Sprintf("%s_max_bars_%d", thresholdDesc, maxBarsAge)

	log.Debug().
		Str("symbol", inputs.Symbol).
		Str("regime", inputs.Regime).
		Bool("regime_aware", g.enabled).
		Int("bars_age", inputs.BarsAge).
		Int("max_bars_age", maxBarsAge).
		Float64("vadr", inputs.VADR).
		Float64("spread_bps", inputs.SpreadBps).
		Bool("passed", result.Passed).
		Msg("Freshness guard evaluation completed")

	return result
}

// EvaluateAllGuards runs all guard checks and returns aggregate result
func (g *RegimeAwareGuards) EvaluateAllGuards(ctx context.Context, inputs GuardInputs) []RegimeGuardResult {
	results := make([]RegimeGuardResult, 0, 3)

	// Evaluate each guard type
	results = append(results, g.EvaluateFatigueGuard(ctx, inputs))
	results = append(results, g.EvaluateLateFillGuard(ctx, inputs))
	results = append(results, g.EvaluateFreshnessGuard(ctx, inputs))

	// Log aggregate result
	allPassed := true
	for _, result := range results {
		if !result.Passed {
			allPassed = false
		}
	}

	log.Info().
		Str("symbol", inputs.Symbol).
		Str("regime", inputs.Regime).
		Bool("regime_aware", g.enabled).
		Bool("all_guards_passed", allPassed).
		Int("total_guards", len(results)).
		Msg("All guards evaluation completed")

	return results
}
