package regime

import (
	"fmt"
	"math"
	"sort"
	"time"

	regimeDomain "github.com/sawpanic/cryptorun/internal/domain/regime"
	"github.com/rs/zerolog/log"
)

// RegimeAnalyzer generates weekly regime performance reports
type RegimeAnalyzer struct {
	config KPIThresholds
}

// NewRegimeAnalyzer creates a new regime analyzer with default thresholds
func NewRegimeAnalyzer() *RegimeAnalyzer {
	return &RegimeAnalyzer{
		config: DefaultKPIThresholds,
	}
}

// NewRegimeAnalyzerWithConfig creates analyzer with custom KPI thresholds
func NewRegimeAnalyzerWithConfig(thresholds KPIThresholds) *RegimeAnalyzer {
	return &RegimeAnalyzer{
		config: thresholds,
	}
}

// GenerateReport creates comprehensive weekly regime analysis
func (ra *RegimeAnalyzer) GenerateReport(period ReportPeriod) (*RegimeReportData, error) {
	log.Info().
		Time("start", period.StartTime).
		Time("end", period.EndTime).
		Str("duration", period.Duration).
		Msg("Generating regime report")

	// Generate mock data for now - in production this would query real data sources
	flipHistory := ra.generateFlipHistory(period)
	exitStats := ra.generateExitStats(period)
	decileLifts := ra.generateDecileLifts(period)

	// Analyze KPI violations
	alerts := ra.analyzeKPIViolations(exitStats, decileLifts)

	report := &RegimeReportData{
		GeneratedAt: time.Now().UTC(),
		Period:      period,
		FlipHistory: flipHistory,
		ExitStats:   exitStats,
		DecileLifts: decileLifts,
		KPIAlerts:   alerts,
	}

	log.Info().
		Int("flips", len(flipHistory)).
		Int("regimes", len(exitStats)).
		Int("alerts", len(alerts)).
		Msg("Regime report generated")

	return report, nil
}

// generateFlipHistory creates realistic regime flip timeline for the period
func (ra *RegimeAnalyzer) generateFlipHistory(period ReportPeriod) []RegimeFlip {
	flips := []RegimeFlip{}

	// Generate 8-12 flips over 28 days (realistic regime switching frequency)
	currentTime := period.StartTime
	currentRegime := "choppy"

	regimes := []string{"choppy", "trending_bull", "high_vol"}

	for currentTime.Before(period.EndTime) {
		// Random duration between 8-72 hours (regime stability)
		durationHours := 12.0 + float64((len(flips)*13)%48)
		nextTime := currentTime.Add(time.Duration(durationHours * float64(time.Hour)))

		if nextTime.After(period.EndTime) {
			break
		}

		// Rotate to next regime
		nextRegime := regimes[(len(flips)+1)%len(regimes)]

		flip := RegimeFlip{
			Timestamp:     nextTime,
			FromRegime:    currentRegime,
			ToRegime:      nextRegime,
			DurationHours: durationHours,
			DetectorInputs: RegimeDetectorInputs{
				RealizedVol7d:   0.25 + float64((len(flips)*7)%30)/100.0,  // 0.25-0.55
				PctAbove20MA:    0.45 + float64((len(flips)*11)%30)/100.0, // 0.45-0.75
				BreadthThrust:   -0.1 + float64((len(flips)*13)%30)/100.0, // -0.1 to 0.2
				StabilityScore:  0.75 + float64((len(flips)*5)%20)/100.0,  // 0.75-0.95
				ConfidenceLevel: 0.80 + float64((len(flips)*3)%15)/100.0,  // 0.80-0.95
			},
			WeightChanges: ra.generateWeightChange(currentRegime, nextRegime),
		}

		flips = append(flips, flip)
		currentTime = nextTime
		currentRegime = nextRegime
	}

	return flips
}

// generateWeightChange shows before/after weight allocation for regime switch
func (ra *RegimeAnalyzer) generateWeightChange(fromRegime, toRegime string) WeightChange {
	before := ra.getRegimeWeights(fromRegime)
	after := ra.getRegimeWeights(toRegime)

	return WeightChange{
		Before: before,
		After:  after,
		Delta: regimeDomain.FactorWeights{
			Momentum:  after.Momentum - before.Momentum,
			Technical: after.Technical - before.Technical,
			Volume:    after.Volume - before.Volume,
			Quality:   after.Quality - before.Quality,
			Catalyst:  after.Catalyst - before.Catalyst,
		},
	}
}

// getRegimeWeights returns factor weights for a regime (matches config/regimes.yaml)
func (ra *RegimeAnalyzer) getRegimeWeights(regime string) regimeDomain.FactorWeights {
	switch regime {
	case "trending_bull":
		return regimeDomain.FactorWeights{Momentum: 50.0, Technical: 20.0, Volume: 15.0, Quality: 10.0, Catalyst: 5.0}
	case "choppy":
		return regimeDomain.FactorWeights{Momentum: 35.0, Technical: 30.0, Volume: 15.0, Quality: 15.0, Catalyst: 5.0}
	case "high_vol":
		return regimeDomain.FactorWeights{Momentum: 30.0, Technical: 25.0, Volume: 20.0, Quality: 20.0, Catalyst: 5.0}
	default:
		return regimeDomain.FactorWeights{Momentum: 35.0, Technical: 30.0, Volume: 15.0, Quality: 15.0, Catalyst: 5.0}
	}
}

// generateExitStats creates realistic exit distribution by regime
func (ra *RegimeAnalyzer) generateExitStats(period ReportPeriod) map[string]ExitStats {
	regimes := []string{"trending_bull", "choppy", "high_vol"}
	stats := make(map[string]ExitStats)

	for i, regime := range regimes {
		// Base exit counts (realistic for 28-day period)
		totalExits := 150 + (i * 50) // 150, 200, 250

		// Regime-specific exit patterns
		var timeLimit, hardStop, profitTarget int
		var avgReturn float64

		switch regime {
		case "trending_bull":
			timeLimit = int(float64(totalExits) * 0.25)    // 25% (good)
			hardStop = int(float64(totalExits) * 0.12)     // 12% (good)
			profitTarget = int(float64(totalExits) * 0.35) // 35% (excellent)
			avgReturn = 18.5
		case "choppy":
			timeLimit = int(float64(totalExits) * 0.45)    // 45% (warning)
			hardStop = int(float64(totalExits) * 0.25)     // 25% (critical)
			profitTarget = int(float64(totalExits) * 0.20) // 20% (miss)
			avgReturn = 8.2
		case "high_vol":
			timeLimit = int(float64(totalExits) * 0.35)    // 35% (acceptable)
			hardStop = int(float64(totalExits) * 0.15)     // 15% (good)
			profitTarget = int(float64(totalExits) * 0.28) // 28% (good)
			avgReturn = 22.1
		}

		momentumFade := int(float64(totalExits) * 0.12)
		venueHealth := int(float64(totalExits) * 0.08)
		other := totalExits - timeLimit - hardStop - profitTarget - momentumFade - venueHealth

		stats[regime] = ExitStats{
			Regime:          regime,
			TotalExits:      totalExits,
			TimeLimit:       timeLimit,
			HardStop:        hardStop,
			MomentumFade:    momentumFade,
			ProfitTarget:    profitTarget,
			VenueHealth:     venueHealth,
			Other:           other,
			TimeLimitPct:    float64(timeLimit) / float64(totalExits) * 100.0,
			HardStopPct:     float64(hardStop) / float64(totalExits) * 100.0,
			ProfitTargetPct: float64(profitTarget) / float64(totalExits) * 100.0,
			AvgHoldHours:    24.0 + (float64(i) * 8.0), // 24, 32, 40 hours
			AvgReturnPct:    avgReturn,
		}
	}

	return stats
}

// generateDecileLifts creates score→return analysis by regime
func (ra *RegimeAnalyzer) generateDecileLifts(period ReportPeriod) map[string]DecileLift {
	regimes := []string{"trending_bull", "choppy", "high_vol"}
	lifts := make(map[string]DecileLift)

	for i, regime := range regimes {
		deciles := make([]DecileBucket, 10)

		// Generate realistic decile performance
		for d := 0; d < 10; d++ {
			decile := d + 1

			// Score ranges (0-100 split into deciles)
			scoreMin := float64(d * 10)
			scoreMax := float64((d + 1) * 10)
			if d == 9 {
				scoreMax = 110.0 // Account for social cap
			}

			// Regime-specific performance patterns
			var baseReturn float64
			switch regime {
			case "trending_bull":
				baseReturn = 5.0 + float64(d)*2.5 // 5% to 27.5%
			case "choppy":
				baseReturn = -2.0 + float64(d)*1.8 // -2% to 14.2%
			case "high_vol":
				baseReturn = 3.0 + float64(d)*3.0 // 3% to 30%
			}

			count := 45 + (d * 5) // More candidates in higher deciles

			deciles[d] = DecileBucket{
				Decile:       decile,
				ScoreMin:     scoreMin,
				ScoreMax:     scoreMax,
				Count:        count,
				AvgScore:     scoreMin + 5.0,
				AvgReturn48h: baseReturn,
				HitRate:      0.45 + float64(d)*0.04, // 45% to 81%
				Sharpe:       0.3 + float64(d)*0.2,   // Increasing Sharpe
			}
		}

		// Calculate overall metrics
		correlation := 0.65 + float64(i)*0.1 // 0.65, 0.75, 0.85
		r2 := correlation * correlation
		topDecile := deciles[9].AvgReturn48h
		bottomDecile := deciles[0].AvgReturn48h
		liftRatio := topDecile / math.Max(bottomDecile, 1.0)

		lifts[regime] = DecileLift{
			Regime:      regime,
			Deciles:     deciles,
			Correlation: correlation,
			R2:          r2,
			Lift:        liftRatio,
		}
	}

	return lifts
}

// analyzeKPIViolations checks exit stats and decile lifts against thresholds
func (ra *RegimeAnalyzer) analyzeKPIViolations(exitStats map[string]ExitStats, decileLifts map[string]DecileLift) []KPIAlert {
	alerts := []KPIAlert{}

	for regime, stats := range exitStats {
		// Check time limit breach
		if stats.TimeLimitPct > ra.config.TimeLimitMax {
			severity := "warning"
			if stats.TimeLimitPct > ra.config.TimeLimitMax+10.0 {
				severity = "critical"
			}

			alerts = append(alerts, KPIAlert{
				Type:        "time_limit_breach",
				Regime:      regime,
				CurrentPct:  stats.TimeLimitPct,
				TargetPct:   ra.config.TimeLimitMax,
				Severity:    severity,
				Action:      "Tighten entry gates by +0.5pp to reduce position sizing",
				Description: fmt.Sprintf("%s regime: %.1f%% time-limit exits exceed target of %.1f%%", regime, stats.TimeLimitPct, ra.config.TimeLimitMax),
			})
		}

		// Check hard stop breach
		if stats.HardStopPct > ra.config.HardStopMax {
			severity := "critical" // Hard stops are always critical

			alerts = append(alerts, KPIAlert{
				Type:        "hard_stop_breach",
				Regime:      regime,
				CurrentPct:  stats.HardStopPct,
				TargetPct:   ra.config.HardStopMax,
				Severity:    severity,
				Action:      "Tighten entry gates by +0.5pp and review risk management",
				Description: fmt.Sprintf("%s regime: %.1f%% hard-stop exits exceed target of %.1f%%", regime, stats.HardStopPct, ra.config.HardStopMax),
			})
		}

		// Check profit target miss
		if stats.ProfitTargetPct < ra.config.ProfitTargetMin {
			alerts = append(alerts, KPIAlert{
				Type:        "profit_target_miss",
				Regime:      regime,
				CurrentPct:  stats.ProfitTargetPct,
				TargetPct:   ra.config.ProfitTargetMin,
				Severity:    "warning",
				Action:      "Review exit strategy and profit-taking thresholds",
				Description: fmt.Sprintf("%s regime: %.1f%% profit-target exits below target of %.1f%%", regime, stats.ProfitTargetPct, ra.config.ProfitTargetMin),
			})
		}
	}

	// Check decile lift violations
	for regime, lift := range decileLifts {
		if lift.Lift < ra.config.LiftMin {
			alerts = append(alerts, KPIAlert{
				Type:        "lift_degradation",
				Regime:      regime,
				CurrentPct:  lift.Lift,
				TargetPct:   ra.config.LiftMin,
				Severity:    "warning",
				Action:      "Review factor orthogonalization and score calibration",
				Description: fmt.Sprintf("%s regime: %.1fx lift below target of %.1fx", regime, lift.Lift, ra.config.LiftMin),
			})
		}

		if lift.Correlation < ra.config.CorrelationMin {
			alerts = append(alerts, KPIAlert{
				Type:        "correlation_degradation",
				Regime:      regime,
				CurrentPct:  lift.Correlation,
				TargetPct:   ra.config.CorrelationMin,
				Severity:    "warning",
				Action:      "Review factor weights and regime detection accuracy",
				Description: fmt.Sprintf("%s regime: %.2f correlation below target of %.2f", regime, lift.Correlation, ra.config.CorrelationMin),
			})
		}
	}

	// Sort alerts by severity (critical first)
	sort.Slice(alerts, func(i, j int) bool {
		if alerts[i].Severity != alerts[j].Severity {
			return alerts[i].Severity == "critical"
		}
		return alerts[i].Type < alerts[j].Type
	})

	return alerts
}
