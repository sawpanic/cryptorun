package gates

import (
	"context"
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/derivs"
	"github.com/sawpanic/cryptorun/internal/microstructure"
)

// Interfaces for dependency injection and testing
type FundingProviderInterface interface {
	GetFundingSnapshot(ctx context.Context, symbol string) (*derivs.FundingSnapshot, error)
}

type OIProviderInterface interface {
	GetOpenInterestSnapshot(ctx context.Context, symbol string, priceChange float64) (*derivs.OpenInterestSnapshot, error)
}

type ETFProviderInterface interface {
	GetETFFlowSnapshot(ctx context.Context, symbol string) (*derivs.ETFFlowSnapshot, error)
}

// EntryGateEvaluator enforces hard entry requirements with tiered microstructure gates
type EntryGateEvaluator struct {
	microEvaluator   microstructure.Evaluator
	tieredCalculator *microstructure.TieredGateCalculator // NEW: Tiered venue-native gates
	fundingProvider  FundingProviderInterface
	oiProvider       OIProviderInterface
	etfProvider      ETFProviderInterface
	config           *EntryGateConfig
}

// NewEntryGateEvaluator creates an entry gate evaluator with tiered microstructure gates
func NewEntryGateEvaluator(
	microEvaluator microstructure.Evaluator,
	fundingProvider FundingProviderInterface,
	oiProvider OIProviderInterface,
	etfProvider ETFProviderInterface,
) *EntryGateEvaluator {
	// Initialize tiered gate calculator with default configuration
	tieredCalculator := microstructure.NewTieredGateCalculator(microstructure.DefaultTieredGateConfig())

	return &EntryGateEvaluator{
		microEvaluator:   microEvaluator,
		tieredCalculator: tieredCalculator,
		fundingProvider:  fundingProvider,
		oiProvider:       oiProvider,
		etfProvider:      etfProvider,
		config:           DefaultEntryGateConfig(),
	}
}

// EntryGateConfig contains hard thresholds for entry gates
type EntryGateConfig struct {
	// Score gate
	MinCompositeScore float64 `yaml:"min_composite_score"` // ≥75

	// Microstructure gates
	MinVADR       float64 `yaml:"min_vadr"`        // ≥1.8×
	MaxSpreadBps  float64 `yaml:"max_spread_bps"`  // ≤50bps
	MinDepthUSD   float64 `yaml:"min_depth_usd"`   // ≥$100k within ±2%
	DepthRangePct float64 `yaml:"depth_range_pct"` // ±2%

	// Funding divergence gate
	MinFundingZScore         float64 `yaml:"min_funding_z_score"`        // ≥2.0 standard deviations
	RequireFundingDivergence bool    `yaml:"require_funding_divergence"` // Must have divergence

	// Movement threshold gates by regime
	MovementThresholds struct {
		Trending float64 `yaml:"trending"` // ≥2.5% for TRENDING regime
		Choppy   float64 `yaml:"choppy"`   // ≥3.0% for CHOP regime
		HighVol  float64 `yaml:"high_vol"` // ≥4.0% for HIGH_VOL regime
	} `yaml:"movement_thresholds"`

	// Volume surge gate (VADR requirement + bar count check)
	VolumeSurge struct {
		MinVADRMultiplier float64 `yaml:"min_vadr_multiplier"` // ≥1.75× VADR
		MinBarsRequired   int     `yaml:"min_bars_required"`   // ≥20 bars (freeze if less)
	} `yaml:"volume_surge"`

	// Liquidity gate
	MinDailyVolumeUSD float64 `yaml:"min_daily_volume_usd"` // ≥$500k daily volume

	// Trend quality gate (ADX OR Hurst)
	TrendQuality struct {
		MinADX   float64 `yaml:"min_adx"`   // ≥25 ADX
		MinHurst float64 `yaml:"min_hurst"` // ≥0.55 Hurst
	} `yaml:"trend_quality"`

	// Freshness gate
	Freshness struct {
		MaxBarsFromTrigger int           `yaml:"max_bars_from_trigger"` // ≤2 bars
		MaxLateFillDelay   time.Duration `yaml:"max_late_fill_delay"`   // ≤30 seconds
	} `yaml:"freshness"`

	// Optional: OI and ETF gates (can be disabled)
	EnableOIGate   bool    `yaml:"enable_oi_gate"`    // Enable OI residual check
	MinOIResidual  float64 `yaml:"min_oi_residual"`   // ≥$1M OI residual
	EnableETFGate  bool    `yaml:"enable_etf_gate"`   // Enable ETF flow check
	MinETFFlowTint float64 `yaml:"min_etf_flow_tint"` // ≥0.3 tint (positive flows)
}

// DefaultEntryGateConfig returns production-ready gate configuration
func DefaultEntryGateConfig() *EntryGateConfig {
	config := &EntryGateConfig{
		// Core gates (always enforced)
		MinCompositeScore: 75.0,
		MinVADR:           1.8,
		MaxSpreadBps:      50.0,
		MinDepthUSD:       100000.0, // $100k
		DepthRangePct:     2.0,      // ±2%

		// Funding divergence (always enforced)
		MinFundingZScore:         2.0,
		RequireFundingDivergence: true,

		// Liquidity gate
		MinDailyVolumeUSD: 500000.0, // $500k daily volume

		// Optional gates (can be disabled for symbols without data)
		EnableOIGate:   true,
		MinOIResidual:  1000000.0, // $1M
		EnableETFGate:  true,
		MinETFFlowTint: 0.3, // 30% net inflow tint
	}

	// Movement thresholds by regime
	config.MovementThresholds.Trending = 2.5 // ≥2.5% for TRENDING regime
	config.MovementThresholds.Choppy = 3.0   // ≥3.0% for CHOP regime
	config.MovementThresholds.HighVol = 4.0  // ≥4.0% for HIGH_VOL regime

	// Volume surge requirements
	config.VolumeSurge.MinVADRMultiplier = 1.75 // ≥1.75× VADR
	config.VolumeSurge.MinBarsRequired = 20     // ≥20 bars required

	// Trend quality thresholds (ADX OR Hurst)
	config.TrendQuality.MinADX = 25.0   // ≥25 ADX
	config.TrendQuality.MinHurst = 0.55 // ≥0.55 Hurst

	// Freshness requirements
	config.Freshness.MaxBarsFromTrigger = 2              // ≤2 bars from trigger
	config.Freshness.MaxLateFillDelay = 30 * time.Second // ≤30 seconds max late fill

	return config
}

// EntryGateResult contains the evaluation result and detailed reasoning
type EntryGateResult struct {
	Symbol           string                           `json:"symbol"`
	Timestamp        time.Time                        `json:"timestamp"`
	Passed           bool                             `json:"passed"`
	CompositeScore   float64                          `json:"composite_score"`
	GateResults      map[string]*GateCheck            `json:"gate_results"`       // gate_name -> result
	TieredGateResult *microstructure.TieredGateResult `json:"tiered_gate_result"` // NEW: Tiered microstructure results
	FailureReasons   []string                         `json:"failure_reasons"`    // List of failed gate descriptions
	PassedGates      []string                         `json:"passed_gates"`       // List of passed gate names
	EvaluationTimeMs int64                            `json:"evaluation_time_ms"`
}

// GateCheck represents the result of a single gate evaluation
type GateCheck struct {
	Name        string      `json:"name"`
	Passed      bool        `json:"passed"`
	Value       interface{} `json:"value"`       // Actual measured value
	Threshold   interface{} `json:"threshold"`   // Required threshold
	Description string      `json:"description"` // Human-readable description
}

// EvaluateEntry performs comprehensive entry gate evaluation with tiered microstructure gates
func (ege *EntryGateEvaluator) EvaluateEntry(ctx context.Context, symbol string, compositeScore float64, priceChange24h float64, regime string, adv float64) (*EntryGateResult, error) {
	startTime := time.Now()

	result := &EntryGateResult{
		Symbol:         symbol,
		Timestamp:      time.Now(),
		CompositeScore: compositeScore,
		GateResults:    make(map[string]*GateCheck),
		FailureReasons: []string{},
		PassedGates:    []string{},
	}

	// Gate 1: Composite Score ≥ 75
	scoreCheck := &GateCheck{
		Name:        "composite_score",
		Value:       compositeScore,
		Threshold:   ege.config.MinCompositeScore,
		Description: fmt.Sprintf("Composite score %.1f ≥ %.1f", compositeScore, ege.config.MinCompositeScore),
	}
	scoreCheck.Passed = compositeScore >= ege.config.MinCompositeScore
	result.GateResults["composite_score"] = scoreCheck

	if scoreCheck.Passed {
		result.PassedGates = append(result.PassedGates, "composite_score")
	} else {
		result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("Score %.1f below threshold %.1f", compositeScore, ege.config.MinCompositeScore))
	}

	// Gate 2: Tiered Microstructure Gates (venue-native with precedence)
	vadrInput := &microstructure.VADRInput{
		High:         priceChange24h * 1.1, // Rough approximation for demo
		Low:          priceChange24h * 0.9,
		Volume:       adv / 100.0, // Rough approximation
		ADV:          adv,
		CurrentPrice: 50000.0, // Mock price - would come from market data
	}

	tieredResult, err := ege.tieredCalculator.EvaluateTieredGates(ctx, symbol, adv, vadrInput)
	if err != nil {
		return nil, fmt.Errorf("tiered microstructure evaluation failed: %w", err)
	}
	result.TieredGateResult = tieredResult

	// Convert tiered results to traditional gate checks for backward compatibility
	if tieredResult.AllGatesPass {
		result.PassedGates = append(result.PassedGates, "tiered_microstructure")
	} else {
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Tiered gates failed: %s", tieredResult.RecommendedAction))
		for _, failure := range tieredResult.CriticalFailures {
			result.FailureReasons = append(result.FailureReasons, failure)
		}
	}

	// Create traditional microstructure gate checks from tiered results
	if tieredResult.DepthGate != nil {
		depthCheck := &GateCheck{
			Name:      "depth_tiered",
			Value:     tieredResult.DepthGate.Measured,
			Threshold: tieredResult.DepthGate.Required,
			Description: fmt.Sprintf("Tiered depth $%.0f ≥ $%.0f (%s, best: %s)",
				tieredResult.DepthGate.Measured, tieredResult.DepthGate.Required,
				tieredResult.Tier.Name, tieredResult.DepthGate.BestVenue),
			Passed: tieredResult.DepthGate.Pass,
		}
		result.GateResults["depth_tiered"] = depthCheck
	}

	if tieredResult.SpreadGate != nil {
		spreadCheck := &GateCheck{
			Name:      "spread_tiered",
			Value:     tieredResult.SpreadGate.MeasuredBps,
			Threshold: tieredResult.SpreadGate.CapBps,
			Description: fmt.Sprintf("Tiered spread %.1f bps ≤ %.1f bps (%s, best: %s)",
				tieredResult.SpreadGate.MeasuredBps, tieredResult.SpreadGate.CapBps,
				tieredResult.Tier.Name, tieredResult.SpreadGate.BestVenue),
			Passed: tieredResult.SpreadGate.Pass,
		}
		result.GateResults["spread_tiered"] = spreadCheck
	}

	if tieredResult.VADRGate != nil {
		vadrCheck := &GateCheck{
			Name:      "vadr_tiered",
			Value:     tieredResult.VADRGate.Measured,
			Threshold: tieredResult.VADRGate.EffectiveMin,
			Description: fmt.Sprintf("Tiered VADR %.2f× ≥ %.2f× (%s, rule: %s)",
				tieredResult.VADRGate.Measured, tieredResult.VADRGate.EffectiveMin,
				tieredResult.Tier.Name, tieredResult.VADRGate.PrecedenceRule),
			Passed: tieredResult.VADRGate.Pass,
		}
		result.GateResults["vadr_tiered"] = vadrCheck
	}

	// Legacy Gate 2: Basic Microstructure Gates (kept for comparison)
	microResult, err := ege.microEvaluator.EvaluateSnapshot(symbol)
	if err != nil {
		return nil, fmt.Errorf("microstructure evaluation failed: %w", err)
	}

	// VADR check
	vadrCheck := &GateCheck{
		Name:        "vadr",
		Value:       microResult.VADR,
		Threshold:   ege.config.MinVADR,
		Description: fmt.Sprintf("VADR %.2f× ≥ %.2f×", microResult.VADR, ege.config.MinVADR),
	}
	vadrCheck.Passed = microResult.VADR >= ege.config.MinVADR
	result.GateResults["vadr"] = vadrCheck

	// Spread check
	spreadCheck := &GateCheck{
		Name:        "spread",
		Value:       microResult.SpreadBps,
		Threshold:   ege.config.MaxSpreadBps,
		Description: fmt.Sprintf("Spread %.1f bps ≤ %.1f bps", microResult.SpreadBps, ege.config.MaxSpreadBps),
	}
	spreadCheck.Passed = microResult.SpreadBps <= ege.config.MaxSpreadBps
	result.GateResults["spread"] = spreadCheck

	// Depth check
	depthCheck := &GateCheck{
		Name:        "depth",
		Value:       microResult.DepthUSD,
		Threshold:   ege.config.MinDepthUSD,
		Description: fmt.Sprintf("Depth $%.0f ≥ $%.0f within ±%.1f%%", microResult.DepthUSD, ege.config.MinDepthUSD, ege.config.DepthRangePct),
	}
	depthCheck.Passed = microResult.DepthUSD >= ege.config.MinDepthUSD
	result.GateResults["depth"] = depthCheck

	// Update passed/failed lists
	for _, check := range []*GateCheck{vadrCheck, spreadCheck, depthCheck} {
		if check.Passed {
			result.PassedGates = append(result.PassedGates, check.Name)
		} else {
			result.FailureReasons = append(result.FailureReasons, check.Description+" FAILED")
		}
	}

	// Gate 3: Funding Divergence Present
	if ege.config.RequireFundingDivergence {
		fundingSnapshot, err := ege.fundingProvider.GetFundingSnapshot(ctx, symbol)
		if err != nil {
			// Funding data unavailable - this is a hard failure
			fundingCheck := &GateCheck{
				Name:        "funding_divergence",
				Value:       "unavailable",
				Threshold:   ege.config.MinFundingZScore,
				Description: "Funding divergence data unavailable",
				Passed:      false,
			}
			result.GateResults["funding_divergence"] = fundingCheck
			result.FailureReasons = append(result.FailureReasons, "Funding divergence data unavailable")
		} else {
			fundingCheck := &GateCheck{
				Name:        "funding_divergence",
				Value:       fundingSnapshot.MaxVenueDivergence,
				Threshold:   ege.config.MinFundingZScore,
				Description: fmt.Sprintf("Funding divergence %.2f ≥ %.2f", fundingSnapshot.MaxVenueDivergence, ege.config.MinFundingZScore),
			}
			fundingCheck.Passed = fundingSnapshot.FundingDivergencePresent &&
				fundingSnapshot.MaxVenueDivergence >= ege.config.MinFundingZScore
			result.GateResults["funding_divergence"] = fundingCheck

			if fundingCheck.Passed {
				result.PassedGates = append(result.PassedGates, "funding_divergence")
			} else {
				result.FailureReasons = append(result.FailureReasons,
					fmt.Sprintf("Insufficient funding divergence (max %.2f, need ≥%.2f)",
						fundingSnapshot.MaxVenueDivergence, ege.config.MinFundingZScore))
			}
		}
	}

	// Gate 4: Optional OI Gate
	if ege.config.EnableOIGate {
		oiSnapshot, err := ege.oiProvider.GetOpenInterestSnapshot(ctx, symbol, priceChange24h)
		if err != nil {
			// OI data unavailable - log but don't fail (optional gate)
			oiCheck := &GateCheck{
				Name:        "oi_residual",
				Value:       "unavailable",
				Threshold:   ege.config.MinOIResidual,
				Description: "OI data unavailable (optional)",
				Passed:      true, // Don't fail on missing optional data
			}
			result.GateResults["oi_residual"] = oiCheck
			result.PassedGates = append(result.PassedGates, "oi_residual")
		} else {
			oiCheck := &GateCheck{
				Name:        "oi_residual",
				Value:       oiSnapshot.OIResidual,
				Threshold:   ege.config.MinOIResidual,
				Description: fmt.Sprintf("OI residual $%.0f ≥ $%.0f", oiSnapshot.OIResidual, ege.config.MinOIResidual),
			}
			oiCheck.Passed = oiSnapshot.OIResidual >= ege.config.MinOIResidual
			result.GateResults["oi_residual"] = oiCheck

			if oiCheck.Passed {
				result.PassedGates = append(result.PassedGates, "oi_residual")
			} else {
				result.FailureReasons = append(result.FailureReasons,
					fmt.Sprintf("OI residual $%.0f below threshold $%.0f",
						oiSnapshot.OIResidual, ege.config.MinOIResidual))
			}
		}
	}

	// Gate 5: Optional ETF Gate
	if ege.config.EnableETFGate {
		etfSnapshot, err := ege.etfProvider.GetETFFlowSnapshot(ctx, symbol)
		if err != nil || len(etfSnapshot.ETFList) == 0 {
			// ETF data unavailable - pass by default (not all assets have ETFs)
			etfCheck := &GateCheck{
				Name:        "etf_flows",
				Value:       "unavailable",
				Threshold:   ege.config.MinETFFlowTint,
				Description: "ETF data unavailable (optional)",
				Passed:      true,
			}
			result.GateResults["etf_flows"] = etfCheck
			result.PassedGates = append(result.PassedGates, "etf_flows")
		} else {
			etfCheck := &GateCheck{
				Name:        "etf_flows",
				Value:       etfSnapshot.FlowTint,
				Threshold:   ege.config.MinETFFlowTint,
				Description: fmt.Sprintf("ETF tint %.2f ≥ %.2f", etfSnapshot.FlowTint, ege.config.MinETFFlowTint),
			}
			etfCheck.Passed = etfSnapshot.FlowTint >= ege.config.MinETFFlowTint
			result.GateResults["etf_flows"] = etfCheck

			if etfCheck.Passed {
				result.PassedGates = append(result.PassedGates, "etf_flows")
			} else {
				result.FailureReasons = append(result.FailureReasons,
					fmt.Sprintf("ETF tint %.2f below threshold %.2f",
						etfSnapshot.FlowTint, ege.config.MinETFFlowTint))
			}
		}
	}

	// Gate 6: Movement Threshold by Regime
	movementThreshold := ege.getMovementThresholdForRegime(regime)
	movementCheck := &GateCheck{
		Name:        "movement_threshold",
		Value:       priceChange24h,
		Threshold:   movementThreshold,
		Description: fmt.Sprintf("Movement %.1f%% ≥ %.1f%% (%s regime)", priceChange24h, movementThreshold, regime),
	}
	movementCheck.Passed = priceChange24h >= movementThreshold
	result.GateResults["movement_threshold"] = movementCheck

	if movementCheck.Passed {
		result.PassedGates = append(result.PassedGates, "movement_threshold")
	} else {
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Movement %.1f%% below threshold %.1f%% for %s regime",
				priceChange24h, movementThreshold, regime))
	}

	// Gate 7: Volume Surge (VADR ≥ 1.75× with bar count check)
	volumeCheck := &GateCheck{
		Name:      "volume_surge",
		Value:     microResult.VADR,
		Threshold: ege.config.VolumeSurge.MinVADRMultiplier,
		Description: fmt.Sprintf("VADR %.2f× ≥ %.2f× with %d bars (≥%d required)",
			microResult.VADR, ege.config.VolumeSurge.MinVADRMultiplier, microResult.BarCount, ege.config.VolumeSurge.MinBarsRequired),
	}
	volumeCheck.Passed = microResult.VADR >= ege.config.VolumeSurge.MinVADRMultiplier &&
		microResult.BarCount >= ege.config.VolumeSurge.MinBarsRequired
	result.GateResults["volume_surge"] = volumeCheck

	if volumeCheck.Passed {
		result.PassedGates = append(result.PassedGates, "volume_surge")
	} else {
		if microResult.VADR < ege.config.VolumeSurge.MinVADRMultiplier {
			result.FailureReasons = append(result.FailureReasons,
				fmt.Sprintf("VADR %.2f× below threshold %.2f×",
					microResult.VADR, ege.config.VolumeSurge.MinVADRMultiplier))
		}
		if microResult.BarCount < ege.config.VolumeSurge.MinBarsRequired {
			result.FailureReasons = append(result.FailureReasons,
				fmt.Sprintf("Insufficient bar count %d (need ≥%d) - frozen",
					microResult.BarCount, ege.config.VolumeSurge.MinBarsRequired))
		}
	}

	// Gate 8: Liquidity (≥$500k daily volume)
	liquidityCheck := &GateCheck{
		Name:        "liquidity",
		Value:       microResult.DailyVolumeUSD,
		Threshold:   ege.config.MinDailyVolumeUSD,
		Description: fmt.Sprintf("Daily volume $%.0f ≥ $%.0f", microResult.DailyVolumeUSD, ege.config.MinDailyVolumeUSD),
	}
	liquidityCheck.Passed = microResult.DailyVolumeUSD >= ege.config.MinDailyVolumeUSD
	result.GateResults["liquidity"] = liquidityCheck

	if liquidityCheck.Passed {
		result.PassedGates = append(result.PassedGates, "liquidity")
	} else {
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Daily volume $%.0f below threshold $%.0f",
				microResult.DailyVolumeUSD, ege.config.MinDailyVolumeUSD))
	}

	// Gate 9: Trend Quality (ADX > 25 OR Hurst > 0.55)
	trendQualityCheck := &GateCheck{
		Name:      "trend_quality",
		Value:     fmt.Sprintf("ADX=%.1f, Hurst=%.2f", microResult.ADX, microResult.Hurst),
		Threshold: fmt.Sprintf("ADX≥%.1f OR Hurst≥%.2f", ege.config.TrendQuality.MinADX, ege.config.TrendQuality.MinHurst),
		Description: fmt.Sprintf("ADX %.1f ≥ %.1f OR Hurst %.2f ≥ %.2f",
			microResult.ADX, ege.config.TrendQuality.MinADX, microResult.Hurst, ege.config.TrendQuality.MinHurst),
	}
	trendQualityCheck.Passed = microResult.ADX >= ege.config.TrendQuality.MinADX ||
		microResult.Hurst >= ege.config.TrendQuality.MinHurst
	result.GateResults["trend_quality"] = trendQualityCheck

	if trendQualityCheck.Passed {
		result.PassedGates = append(result.PassedGates, "trend_quality")
	} else {
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Weak trend quality: ADX %.1f < %.1f AND Hurst %.2f < %.2f",
				microResult.ADX, ege.config.TrendQuality.MinADX, microResult.Hurst, ege.config.TrendQuality.MinHurst))
	}

	// Gate 10: Freshness (within 2 bars of trigger, late-fill <30s)
	freshnessCheck := &GateCheck{
		Name:      "freshness",
		Value:     fmt.Sprintf("%d bars, %.1fs", microResult.BarsFromTrigger, microResult.LateFillDelay.Seconds()),
		Threshold: fmt.Sprintf("≤%d bars, ≤%.1fs", ege.config.Freshness.MaxBarsFromTrigger, ege.config.Freshness.MaxLateFillDelay.Seconds()),
		Description: fmt.Sprintf("Data age %d bars ≤ %d AND fill delay %.1fs ≤ %.1fs",
			microResult.BarsFromTrigger, ege.config.Freshness.MaxBarsFromTrigger,
			microResult.LateFillDelay.Seconds(), ege.config.Freshness.MaxLateFillDelay.Seconds()),
	}
	freshnessCheck.Passed = microResult.BarsFromTrigger <= ege.config.Freshness.MaxBarsFromTrigger &&
		microResult.LateFillDelay <= ege.config.Freshness.MaxLateFillDelay
	result.GateResults["freshness"] = freshnessCheck

	if freshnessCheck.Passed {
		result.PassedGates = append(result.PassedGates, "freshness")
	} else {
		if microResult.BarsFromTrigger > ege.config.Freshness.MaxBarsFromTrigger {
			result.FailureReasons = append(result.FailureReasons,
				fmt.Sprintf("Stale data: %d bars from trigger (max %d)",
					microResult.BarsFromTrigger, ege.config.Freshness.MaxBarsFromTrigger))
		}
		if microResult.LateFillDelay > ege.config.Freshness.MaxLateFillDelay {
			result.FailureReasons = append(result.FailureReasons,
				fmt.Sprintf("Late fill: %.1fs delay (max %.1fs)",
					microResult.LateFillDelay.Seconds(), ege.config.Freshness.MaxLateFillDelay.Seconds()))
		}
	}

	// Overall pass/fail determination
	result.Passed = len(result.FailureReasons) == 0

	result.EvaluationTimeMs = time.Since(startTime).Milliseconds()

	return result, nil
}

// getMovementThresholdForRegime returns the required movement threshold for the given regime
func (ege *EntryGateEvaluator) getMovementThresholdForRegime(regime string) float64 {
	switch regime {
	case "TRENDING":
		return ege.config.MovementThresholds.Trending
	case "CHOP":
		return ege.config.MovementThresholds.Choppy
	case "HIGH_VOL":
		return ege.config.MovementThresholds.HighVol
	default:
		// Default to highest threshold for unknown regimes (conservative)
		return ege.config.MovementThresholds.HighVol
	}
}

// GetGateSummary returns a concise summary of gate evaluation
func (egr *EntryGateResult) GetGateSummary() string {
	if egr.Passed {
		return fmt.Sprintf("✅ ENTRY CLEARED — %s (score: %.1f, %d/%d gates passed)",
			egr.Symbol, egr.CompositeScore, len(egr.PassedGates), len(egr.GateResults))
	} else {
		return fmt.Sprintf("❌ ENTRY BLOCKED — %s (%d failures: %s)",
			egr.Symbol, len(egr.FailureReasons), egr.FailureReasons[0])
	}
}

// GetDetailedReport returns a comprehensive gate evaluation report
func (egr *EntryGateResult) GetDetailedReport() string {
	report := fmt.Sprintf("Entry Gate Evaluation: %s (%.1f score)\n", egr.Symbol, egr.CompositeScore)
	report += fmt.Sprintf("Overall: %s | Evaluation: %dms\n\n",
		map[bool]string{true: "PASS ✅", false: "FAIL ❌"}[egr.Passed],
		egr.EvaluationTimeMs)

	// List all gate results
	gateOrder := []string{"composite_score", "vadr", "spread", "depth", "funding_divergence", "oi_residual", "etf_flows",
		"movement_threshold", "volume_surge", "liquidity", "trend_quality", "freshness"}

	for _, gateName := range gateOrder {
		if check, exists := egr.GateResults[gateName]; exists {
			status := map[bool]string{true: "✅", false: "❌"}[check.Passed]
			report += fmt.Sprintf("%s %s: %s\n", status, check.Name, check.Description)
		}
	}

	if len(egr.FailureReasons) > 0 {
		report += fmt.Sprintf("\nFailure Details:\n")
		for i, reason := range egr.FailureReasons {
			report += fmt.Sprintf("  %d. %s\n", i+1, reason)
		}
	}

	return report
}
