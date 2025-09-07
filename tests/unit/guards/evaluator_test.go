package guards_test

import (
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/guards"
)

func TestGuardEvaluator_AllGuardsPass(t *testing.T) {
	config := guards.GuardConfig{
		RegimeAware: true,
		Fatigue: guards.FatigueConfig{
			Baseline: guards.FatigueThresholds{
				Momentum24hThreshold: 12.0,
				RSI4hThreshold:       70.0,
				AccelerationOverride: 2.0,
			},
			MaxMomentum: 25.0,
			MaxRSI:      80.0,
		},
		LateFill: guards.LateFillConfig{
			Baseline: guards.LateFillThresholds{
				MaxDelaySeconds: 30,
			},
			MaxDelaySecondsAbs: 60,
			MinDelaySeconds:    0,
		},
		Freshness: guards.FreshnessConfig{
			Baseline: guards.FreshnessThresholds{
				MaxBarsAge: 2,
				ATRFactor:  1.2,
			},
			MaxBarsAgeAbs: 5,
			MinATRFactor:  0.8,
		},
	}

	evaluator := guards.NewGuardEvaluator(config)
	baseTime := time.Now()

	inputs := guards.AllGuardsInputs{
		Fatigue: guards.FatigueInputs{
			Symbol:       "BTCUSD",
			Momentum24h:  8.0,  // < 12% (OK)
			RSI4h:        65.0, // < 70 (OK)
			Acceleration: 1.0,
			Regime:       guards.RegimeChoppy,
		},
		LateFill: guards.LateFillInputs{
			Symbol:        "BTCUSD",
			SignalTime:    baseTime,
			ExecutionTime: baseTime.Add(20 * time.Second), // < 30s (OK)
			Regime:        guards.RegimeChoppy,
		},
		Freshness: guards.FreshnessInputs{
			Symbol:      "BTCUSD",
			BarsAge:     1,     // < 2 (OK)
			PriceChange: 180.0, // Price change
			ATR1h:       200.0, // 0.9x ATR < 1.2x (OK)
			Regime:      guards.RegimeChoppy,
		},
	}

	result := evaluator.EvaluateAllGuards(inputs)

	if !result.AllowEntry {
		t.Errorf("Expected entry allowed when all guards pass, got blocked by: %s", result.BlockReason)
	}

	if result.BlockedBy != "" {
		t.Errorf("Expected no blocking guard, got: %s", result.BlockedBy)
	}

	if result.BlockReason != "all_guards_passed" {
		t.Errorf("Expected 'all_guards_passed', got: %s", result.BlockReason)
	}

	if result.Profile != "baseline" {
		t.Errorf("Expected baseline profile, got: %s", result.Profile)
	}

	// Verify individual guard results are included
	if len(result.GuardResults) != 3 {
		t.Errorf("Expected 3 guard results, got: %d", len(result.GuardResults))
	}

	guards := []string{"fatigue", "late_fill", "freshness"}
	for _, guardName := range guards {
		if guardResult, ok := result.GuardResults[guardName]; !ok {
			t.Errorf("Missing guard result for: %s", guardName)
		} else if !guardResult.Allow {
			t.Errorf("Guard %s should have passed but was blocked: %s", guardName, guardResult.Reason)
		}
	}
}

func TestGuardEvaluator_BlockingPrecedence(t *testing.T) {
	config := guards.GuardConfig{
		RegimeAware: false, // Use baseline only
		Fatigue: guards.FatigueConfig{
			Baseline: guards.FatigueThresholds{
				Momentum24hThreshold: 12.0,
				RSI4hThreshold:       70.0,
				AccelerationOverride: 2.0,
			},
			MaxMomentum: 25.0,
			MaxRSI:      80.0,
		},
		LateFill: guards.LateFillConfig{
			Baseline: guards.LateFillThresholds{
				MaxDelaySeconds: 30,
			},
			MaxDelaySecondsAbs: 60,
			MinDelaySeconds:    0,
		},
		Freshness: guards.FreshnessConfig{
			Baseline: guards.FreshnessThresholds{
				MaxBarsAge: 2,
				ATRFactor:  1.2,
			},
			MaxBarsAgeAbs: 5,
			MinATRFactor:  0.8,
		},
	}

	evaluator := guards.NewGuardEvaluator(config)
	baseTime := time.Now()

	tests := []struct {
		name            string
		inputs          guards.AllGuardsInputs
		expectedBlocked string
		reason          string
	}{
		{
			name: "fatigue_blocks_first",
			inputs: guards.AllGuardsInputs{
				Fatigue: guards.FatigueInputs{
					Symbol:       "BTCUSD",
					Momentum24h:  15.0, // > 12% (BLOCK)
					RSI4h:        75.0, // > 70 (BLOCK)
					Acceleration: 1.0,  // < 2% (no override)
					Regime:       guards.RegimeChoppy,
				},
				LateFill: guards.LateFillInputs{
					Symbol:        "BTCUSD",
					SignalTime:    baseTime,
					ExecutionTime: baseTime.Add(45 * time.Second), // Also blocks
					Regime:        guards.RegimeChoppy,
				},
				Freshness: guards.FreshnessInputs{
					Symbol:      "BTCUSD",
					BarsAge:     4, // Also blocks
					PriceChange: 180.0,
					ATR1h:       200.0,
					Regime:      guards.RegimeChoppy,
				},
			},
			expectedBlocked: "fatigue", // First in priority order
			reason:          "overextended",
		},
		{
			name: "late_fill_blocks_when_fatigue_passes",
			inputs: guards.AllGuardsInputs{
				Fatigue: guards.FatigueInputs{
					Symbol:       "ETHUSD",
					Momentum24h:  8.0,  // < 12% (OK)
					RSI4h:        65.0, // < 70 (OK)
					Acceleration: 1.0,
					Regime:       guards.RegimeChoppy,
				},
				LateFill: guards.LateFillInputs{
					Symbol:        "ETHUSD",
					SignalTime:    baseTime,
					ExecutionTime: baseTime.Add(45 * time.Second), // > 30s (BLOCK)
					Regime:        guards.RegimeChoppy,
				},
				Freshness: guards.FreshnessInputs{
					Symbol:      "ETHUSD",
					BarsAge:     4, // Also blocks
					PriceChange: 180.0,
					ATR1h:       200.0,
					Regime:      guards.RegimeChoppy,
				},
			},
			expectedBlocked: "late_fill", // Second in priority order
			reason:          "too_late",
		},
		{
			name: "freshness_blocks_when_others_pass",
			inputs: guards.AllGuardsInputs{
				Fatigue: guards.FatigueInputs{
					Symbol:       "SOLUSD",
					Momentum24h:  8.0,  // < 12% (OK)
					RSI4h:        65.0, // < 70 (OK)
					Acceleration: 1.0,
					Regime:       guards.RegimeChoppy,
				},
				LateFill: guards.LateFillInputs{
					Symbol:        "SOLUSD",
					SignalTime:    baseTime,
					ExecutionTime: baseTime.Add(20 * time.Second), // < 30s (OK)
					Regime:        guards.RegimeChoppy,
				},
				Freshness: guards.FreshnessInputs{
					Symbol:      "SOLUSD",
					BarsAge:     4, // > 2 (BLOCK)
					PriceChange: 180.0,
					ATR1h:       200.0,
					Regime:      guards.RegimeChoppy,
				},
			},
			expectedBlocked: "freshness", // Third in priority order
			reason:          "too_old",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluator.EvaluateAllGuards(tt.inputs)

			if result.AllowEntry {
				t.Errorf("Expected entry blocked, got allowed")
			}

			if result.BlockedBy != tt.expectedBlocked {
				t.Errorf("Expected blocked by %s, got: %s", tt.expectedBlocked, result.BlockedBy)
			}

			if !contains(result.BlockReason, tt.reason) {
				t.Errorf("Expected reason to contain '%s', got: %s", tt.reason, result.BlockReason)
			}
		})
	}
}

func TestGuardEvaluator_TrendingProfileSelection(t *testing.T) {
	config := guards.GuardConfig{
		RegimeAware: true,
		Fatigue: guards.FatigueConfig{
			Baseline: guards.FatigueThresholds{
				Momentum24hThreshold: 12.0,
				RSI4hThreshold:       70.0,
				AccelerationOverride: 2.0,
			},
			TrendingProfile: guards.FatigueThresholds{
				Momentum24hThreshold: 18.0, // Higher threshold
				RSI4hThreshold:       70.0,
				AccelerationOverride: 2.0,
				RequiresAccelRenewal: true, // Safety condition
			},
			MaxMomentum: 25.0,
			MaxRSI:      80.0,
		},
		LateFill: guards.LateFillConfig{
			Baseline: guards.LateFillThresholds{
				MaxDelaySeconds: 30,
			},
			TrendingProfile: guards.LateFillThresholds{
				MaxDelaySeconds:      45,   // Higher threshold
				RequiresInfraHealth:  true, // Safety condition
				RequiresATRProximity: true, // Safety condition
				ATRFactor:            1.2,
			},
			MaxDelaySecondsAbs: 60,
			MinDelaySeconds:    0,
		},
		Freshness: guards.FreshnessConfig{
			Baseline: guards.FreshnessThresholds{
				MaxBarsAge: 2,
				ATRFactor:  1.2,
			},
			TrendingProfile: guards.FreshnessThresholds{
				MaxBarsAge:          3, // Higher threshold
				ATRFactor:           1.2,
				RequiresVADR:        1.75, // Safety condition
				RequiresTightSpread: true, // Safety condition
				SpreadThresholdBps:  50.0,
			},
			MaxBarsAgeAbs: 5,
			MinATRFactor:  0.8,
		},
	}

	evaluator := guards.NewGuardEvaluator(config)
	baseTime := time.Now()

	inputs := guards.AllGuardsInputs{
		Fatigue: guards.FatigueInputs{
			Symbol:       "BTCUSD",
			Momentum24h:  15.0, // Would block baseline, OK trending
			RSI4h:        65.0, // < 70 (OK)
			Acceleration: 1.0,
			AccelRenewal: true, // Safety condition met
			Regime:       guards.RegimeTrending,
		},
		LateFill: guards.LateFillInputs{
			Symbol:        "BTCUSD",
			SignalTime:    baseTime,
			ExecutionTime: baseTime.Add(40 * time.Second), // Would block baseline, OK trending
			InfraP99MS:    350.0,                          // < 400ms (health OK)
			ATRDistance:   1.1,                            // < 1.2 (proximity OK)
			Regime:        guards.RegimeTrending,
		},
		Freshness: guards.FreshnessInputs{
			Symbol:      "BTCUSD",
			BarsAge:     3,     // Would block baseline, OK trending
			PriceChange: 180.0, // 0.9x ATR OK
			ATR1h:       200.0,
			VADR:        2.0,  // >= 1.75 (OK)
			SpreadBps:   40.0, // < 50 bps (OK)
			Regime:      guards.RegimeTrending,
		},
	}

	result := evaluator.EvaluateAllGuards(inputs)

	if !result.AllowEntry {
		t.Errorf("Expected entry allowed with trending profiles, got blocked by: %s", result.BlockReason)
	}

	if result.Profile != "trending" {
		t.Errorf("Expected trending profile, got: %s", result.Profile)
	}

	if result.Regime != guards.RegimeTrending {
		t.Errorf("Expected trending regime, got: %s", result.Regime)
	}

	// Verify individual guards used trending profiles
	for guardName, guardResult := range result.GuardResults {
		if guardResult.Profile != "trending" {
			t.Errorf("Guard %s should use trending profile, got: %s", guardName, guardResult.Profile)
		}
	}
}

func TestGuardEvaluator_EffectiveThresholds(t *testing.T) {
	config := guards.GuardConfig{
		RegimeAware: true,
		Fatigue: guards.FatigueConfig{
			Baseline: guards.FatigueThresholds{
				Momentum24hThreshold: 12.0,
				RSI4hThreshold:       70.0,
				AccelerationOverride: 2.0,
			},
			TrendingProfile: guards.FatigueThresholds{
				Momentum24hThreshold: 18.0,
				RSI4hThreshold:       70.0,
				AccelerationOverride: 2.0,
				RequiresAccelRenewal: true,
			},
			MaxMomentum: 25.0,
			MaxRSI:      80.0,
		},
		LateFill: guards.LateFillConfig{
			Baseline: guards.LateFillThresholds{
				MaxDelaySeconds: 30,
			},
			TrendingProfile: guards.LateFillThresholds{
				MaxDelaySeconds:      45,
				RequiresInfraHealth:  true,
				RequiresATRProximity: true,
				ATRFactor:            1.2,
			},
			MaxDelaySecondsAbs: 60,
			MinDelaySeconds:    0,
		},
		Freshness: guards.FreshnessConfig{
			Baseline: guards.FreshnessThresholds{
				MaxBarsAge: 2,
				ATRFactor:  1.2,
			},
			TrendingProfile: guards.FreshnessThresholds{
				MaxBarsAge:          3,
				ATRFactor:           1.2,
				RequiresVADR:        1.75,
				RequiresTightSpread: true,
				SpreadThresholdBps:  50.0,
			},
			MaxBarsAgeAbs: 5,
			MinATRFactor:  0.8,
		},
	}

	evaluator := guards.NewGuardEvaluator(config)

	tests := []struct {
		name              string
		regime            guards.Regime
		accelRenewal      bool
		infraP99MS        float64
		atrDistance       float64
		vadr              float64
		spreadBps         float64
		expectedFatigue   string
		expectedLateFill  string
		expectedFreshness string
	}{
		{
			name:              "choppy_regime_uses_baseline",
			regime:            guards.RegimeChoppy,
			accelRenewal:      true,
			infraP99MS:        350.0,
			atrDistance:       1.1,
			vadr:              2.0,
			spreadBps:         40.0,
			expectedFatigue:   "baseline",
			expectedLateFill:  "baseline",
			expectedFreshness: "baseline",
		},
		{
			name:              "trending_with_all_conditions_uses_trending",
			regime:            guards.RegimeTrending,
			accelRenewal:      true,  // Fatigue condition met
			infraP99MS:        350.0, // < 400 (late-fill condition met)
			atrDistance:       1.1,   // < 1.2 (late-fill condition met)
			vadr:              2.0,   // >= 1.75 (freshness condition met)
			spreadBps:         40.0,  // < 50 (freshness condition met)
			expectedFatigue:   "trending",
			expectedLateFill:  "trending",
			expectedFreshness: "trending",
		},
		{
			name:              "trending_with_missing_conditions_mixes_profiles",
			regime:            guards.RegimeTrending,
			accelRenewal:      false, // Fatigue condition NOT met
			infraP99MS:        450.0, // > 400 (late-fill condition NOT met)
			atrDistance:       1.5,   // > 1.2 (late-fill condition NOT met)
			vadr:              1.5,   // < 1.75 (freshness condition NOT met)
			spreadBps:         60.0,  // > 50 (freshness condition NOT met)
			expectedFatigue:   "baseline",
			expectedLateFill:  "baseline",
			expectedFreshness: "baseline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thresholds := evaluator.GetEffectiveThresholds(
				tt.regime, tt.accelRenewal, tt.infraP99MS, tt.atrDistance, tt.vadr, tt.spreadBps,
			)

			if thresholds["fatigue_profile"] != tt.expectedFatigue {
				t.Errorf("Expected fatigue profile %s, got: %s",
					tt.expectedFatigue, thresholds["fatigue_profile"])
			}

			if thresholds["late_fill_profile"] != tt.expectedLateFill {
				t.Errorf("Expected late-fill profile %s, got: %s",
					tt.expectedLateFill, thresholds["late_fill_profile"])
			}

			if thresholds["freshness_profile"] != tt.expectedFreshness {
				t.Errorf("Expected freshness profile %s, got: %s",
					tt.expectedFreshness, thresholds["freshness_profile"])
			}

			// Verify regime awareness flag is included
			if thresholds["regime_aware"] != true {
				t.Error("Expected regime_aware=true in thresholds")
			}
		})
	}
}
