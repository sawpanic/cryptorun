package unit

import (
	"context"
	"os"
	"testing"
	"time"

	"cryptorun/internal/domain/guards"
)

func TestRegimeAwareGuardsFeatureFlag(t *testing.T) {
	// Test that feature flag controls regime-aware behavior
	testCases := []struct {
		name          string
		envValue      string
		expectEnabled bool
	}{
		{"FlagTrue_EnablesRegimeAware", "true", true},
		{"FlagFalse_DisablesRegimeAware", "false", false},
		{"FlagEmpty_DisablesRegimeAware", "", false},
		{"FlagInvalid_DisablesRegimeAware", "invalid", false},
		{"Flag1_EnablesRegimeAware", "1", true},
		{"Flag0_DisablesRegimeAware", "0", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variable
			if tc.envValue != "" {
				os.Setenv("GUARDS_REGIME_AWARE", tc.envValue)
			} else {
				os.Unsetenv("GUARDS_REGIME_AWARE")
			}
			defer os.Unsetenv("GUARDS_REGIME_AWARE")

			// Create guards with mock profiles
			profiles := guards.RegimeProfiles{
				Trending: guards.RegimeProfile{
					Fatigue:   guards.FatigueProfile{MomentumThreshold: 18.0, RSIThreshold: 70.0, RequiresAccelRenewal: true},
					LateFill:  guards.LateFillProfile{MaxDelaySeconds: 45, RequiresInfraHealth: true, RequiresATRProximity: true, ATRFactor: 1.2},
					Freshness: guards.FreshnessProfile{MaxBarsAge: 3, RequiresVADR: 1.75, RequiresTightSpread: true, SpreadThresholdBps: 50.0},
				},
				Chop: guards.RegimeProfile{
					Fatigue:   guards.FatigueProfile{MomentumThreshold: 12.0, RSIThreshold: 70.0, RequiresAccelRenewal: false},
					LateFill:  guards.LateFillProfile{MaxDelaySeconds: 30, RequiresInfraHealth: false, RequiresATRProximity: false, ATRFactor: 1.0},
					Freshness: guards.FreshnessProfile{MaxBarsAge: 2, RequiresVADR: 0.0, RequiresTightSpread: false, SpreadThresholdBps: 100.0},
				},
				HighVol: guards.RegimeProfile{
					Fatigue:   guards.FatigueProfile{MomentumThreshold: 12.0, RSIThreshold: 70.0, RequiresAccelRenewal: false},
					LateFill:  guards.LateFillProfile{MaxDelaySeconds: 30, RequiresInfraHealth: false, RequiresATRProximity: false, ATRFactor: 1.0},
					Freshness: guards.FreshnessProfile{MaxBarsAge: 2, RequiresVADR: 0.0, RequiresTightSpread: false, SpreadThresholdBps: 100.0},
				},
			}

			safetyLimits := guards.SafetyLimits{
				MaxMomentumThreshold: 25.0,
				MaxRSIThreshold:      80.0,
				MaxDelaySecondsAbs:   60,
				MaxBarsAgeAbs:        5,
				MinATRFactor:         0.8,
			}

			regimeGuards := guards.NewRegimeAwareGuards(profiles, safetyLimits)

			if regimeGuards.IsEnabled() != tc.expectEnabled {
				t.Errorf("Expected regime aware enabled = %v, got %v", tc.expectEnabled, regimeGuards.IsEnabled())
			}
		})
	}
}

func TestFatigueGuardByRegime(t *testing.T) {
	// Enable regime-aware guards for testing
	os.Setenv("GUARDS_REGIME_AWARE", "true")
	defer os.Unsetenv("GUARDS_REGIME_AWARE")

	profiles := guards.RegimeProfiles{
		Trending: guards.RegimeProfile{
			Fatigue: guards.FatigueProfile{
				MomentumThreshold:    18.0, // Higher threshold for trending
				RSIThreshold:         70.0,
				RequiresAccelRenewal: true, // TRENDING-specific safety condition
			},
		},
		Chop: guards.RegimeProfile{
			Fatigue: guards.FatigueProfile{
				MomentumThreshold:    12.0, // Baseline threshold
				RSIThreshold:         70.0,
				RequiresAccelRenewal: false,
			},
		},
		HighVol: guards.RegimeProfile{
			Fatigue: guards.FatigueProfile{
				MomentumThreshold:    12.0, // Same as chop
				RSIThreshold:         70.0,
				RequiresAccelRenewal: false,
			},
		},
	}

	safetyLimits := guards.SafetyLimits{
		MaxMomentumThreshold: 25.0,
		MaxRSIThreshold:      80.0,
		MaxDelaySecondsAbs:   60,
		MaxBarsAgeAbs:        5,
		MinATRFactor:         0.8,
	}

	regimeGuards := guards.NewRegimeAwareGuards(profiles, safetyLimits)
	ctx := context.Background()

	testCases := []struct {
		name         string
		regime       string
		momentum24h  float64
		rsi4h        float64
		acceleration float64
		expectPassed bool
		expectReason string
	}{
		// TRENDING regime tests
		{
			name:         "TrendingRegime_HighMomentumWithAccel_ShouldPass",
			regime:       "TRENDING",
			momentum24h:  15.0, // Above chop threshold (12%) but below trending (18%)
			rsi4h:        75.0, // Above RSI threshold (70%)
			acceleration: 2.5,  // Positive acceleration
			expectPassed: true,
		},
		{
			name:         "TrendingRegime_HighMomentumNoAccel_ShouldFail",
			regime:       "TRENDING",
			momentum24h:  15.0,
			rsi4h:        75.0,
			acceleration: -0.5, // Negative acceleration
			expectPassed: false,
			expectReason: "trending_fatigue_requires_acceleration_renewal",
		},
		{
			name:         "TrendingRegime_VeryHighMomentum_ShouldFail",
			regime:       "TRENDING",
			momentum24h:  20.0, // Above trending threshold (18%)
			rsi4h:        75.0, // Above RSI threshold (70%)
			acceleration: 2.5,  // Positive acceleration
			expectPassed: false,
			expectReason: "fatigue_detected_momentum_20.0_rsi_75.0",
		},
		// CHOP regime tests
		{
			name:         "ChopRegime_ModerateGain_ShouldPass",
			regime:       "CHOP",
			momentum24h:  10.0, // Below chop threshold (12%)
			rsi4h:        65.0, // Below RSI threshold (70%)
			acceleration: 0.0,  // Acceleration not checked for chop
			expectPassed: true,
		},
		{
			name:         "ChopRegime_HighGain_ShouldFail",
			regime:       "CHOP",
			momentum24h:  15.0, // Above chop threshold (12%)
			rsi4h:        75.0, // Above RSI threshold (70%)
			acceleration: 0.0,  // Acceleration not checked for chop
			expectPassed: false,
			expectReason: "fatigue_detected_momentum_15.0_rsi_75.0",
		},
		// HIGH_VOL regime tests
		{
			name:         "HighVolRegime_ModerateGain_ShouldPass",
			regime:       "HIGH_VOL",
			momentum24h:  10.0,
			rsi4h:        65.0,
			acceleration: 0.0,
			expectPassed: true,
		},
		{
			name:         "HighVolRegime_HighGain_ShouldFail",
			regime:       "HIGH_VOL",
			momentum24h:  15.0,
			rsi4h:        75.0,
			acceleration: 0.0,
			expectPassed: false,
			expectReason: "fatigue_detected_momentum_15.0_rsi_75.0",
		},
		// Unknown regime tests
		{
			name:         "UnknownRegime_FallsBackToChop",
			regime:       "UNKNOWN",
			momentum24h:  15.0, // Above chop threshold
			rsi4h:        75.0, // Above RSI threshold
			acceleration: 0.0,
			expectPassed: false,
			expectReason: "fatigue_detected_momentum_15.0_rsi_75.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := guards.GuardInputs{
				Symbol:       "BTCUSD",
				Regime:       tc.regime,
				Momentum24h:  tc.momentum24h,
				RSI4h:        tc.rsi4h,
				Acceleration: tc.acceleration,
				Timestamp:    time.Now(),
			}

			result := regimeGuards.EvaluateFatigueGuard(ctx, inputs)

			if result.Passed != tc.expectPassed {
				t.Errorf("Expected passed = %v, got %v", tc.expectPassed, result.Passed)
			}

			if !tc.expectPassed && tc.expectReason != "" {
				if result.Reason != tc.expectReason {
					t.Errorf("Expected reason = %s, got %s", tc.expectReason, result.Reason)
				}
			}

			// Verify regime-aware flag is set
			if !result.RegimeAware {
				t.Error("Expected RegimeAware = true")
			}

			// Verify regime is recorded
			if result.Regime != tc.regime {
				t.Errorf("Expected regime = %s, got %s", tc.regime, result.Regime)
			}
		})
	}
}

func TestLateFillGuardByRegime(t *testing.T) {
	// Enable regime-aware guards for testing
	os.Setenv("GUARDS_REGIME_AWARE", "true")
	defer os.Unsetenv("GUARDS_REGIME_AWARE")

	profiles := guards.RegimeProfiles{
		Trending: guards.RegimeProfile{
			LateFill: guards.LateFillProfile{
				MaxDelaySeconds:      45,   // Relaxed for trending
				RequiresInfraHealth:  true, // TRENDING-specific condition
				RequiresATRProximity: true, // TRENDING-specific condition
				ATRFactor:            1.2,
			},
		},
		Chop: guards.RegimeProfile{
			LateFill: guards.LateFillProfile{
				MaxDelaySeconds:      30, // Baseline
				RequiresInfraHealth:  false,
				RequiresATRProximity: false,
				ATRFactor:            1.0,
			},
		},
	}

	safetyLimits := guards.SafetyLimits{
		MaxMomentumThreshold: 25.0,
		MaxRSIThreshold:      80.0,
		MaxDelaySecondsAbs:   60,
		MaxBarsAgeAbs:        5,
		MinATRFactor:         0.8,
	}

	regimeGuards := guards.NewRegimeAwareGuards(profiles, safetyLimits)
	ctx := context.Background()

	testCases := []struct {
		name            string
		regime          string
		fillDelay       time.Duration
		infraP99Latency time.Duration
		atrDistance     float64
		atr1h           float64
		expectPassed    bool
		expectReason    string
	}{
		// TRENDING regime tests
		{
			name:            "TrendingRegime_SlowFillGoodInfraATR_ShouldPass",
			regime:          "TRENDING",
			fillDelay:       40 * time.Second,       // Above chop limit (30s) but below trending (45s)
			infraP99Latency: 300 * time.Millisecond, // Good infrastructure (<400ms)
			atrDistance:     1.0,                    // Within ATR proximity
			atr1h:           1.0,
			expectPassed:    true,
		},
		{
			name:            "TrendingRegime_SlowFillBadInfra_ShouldFail",
			regime:          "TRENDING",
			fillDelay:       40 * time.Second,
			infraP99Latency: 500 * time.Millisecond, // Bad infrastructure (>400ms)
			atrDistance:     1.0,
			atr1h:           1.0,
			expectPassed:    false,
			expectReason:    "trending_late_fill_requires_infra_health_p99_500ms",
		},
		{
			name:            "TrendingRegime_SlowFillBadATR_ShouldFail",
			regime:          "TRENDING",
			fillDelay:       40 * time.Second,
			infraP99Latency: 300 * time.Millisecond,
			atrDistance:     1.5, // Too far from trigger (>1.2×ATR)
			atr1h:           1.0,
			expectPassed:    false,
			expectReason:    "trending_late_fill_requires_atr_proximity_1.50x",
		},
		{
			name:            "TrendingRegime_VerySlowFill_ShouldFail",
			regime:          "TRENDING",
			fillDelay:       50 * time.Second, // Above trending limit (45s)
			infraP99Latency: 300 * time.Millisecond,
			atrDistance:     1.0,
			atr1h:           1.0,
			expectPassed:    false,
			expectReason:    "late_fill_delay_50s_exceeds_45s",
		},
		// CHOP regime tests
		{
			name:            "ChopRegime_ModerateDelay_ShouldPass",
			regime:          "CHOP",
			fillDelay:       25 * time.Second,       // Below chop limit (30s)
			infraP99Latency: 500 * time.Millisecond, // Infrastructure not checked for chop
			atrDistance:     2.0,                    // ATR proximity not checked for chop
			atr1h:           1.0,
			expectPassed:    true,
		},
		{
			name:            "ChopRegime_SlowFill_ShouldFail",
			regime:          "CHOP",
			fillDelay:       35 * time.Second, // Above chop limit (30s)
			infraP99Latency: 300 * time.Millisecond,
			atrDistance:     1.0,
			atr1h:           1.0,
			expectPassed:    false,
			expectReason:    "late_fill_delay_35s_exceeds_30s",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := guards.GuardInputs{
				Symbol:          "BTCUSD",
				Regime:          tc.regime,
				FillDelay:       tc.fillDelay,
				InfraP99Latency: tc.infraP99Latency,
				ATRDistance:     tc.atrDistance,
				ATR1h:           tc.atr1h,
				Timestamp:       time.Now(),
			}

			result := regimeGuards.EvaluateLateFillGuard(ctx, inputs)

			if result.Passed != tc.expectPassed {
				t.Errorf("Expected passed = %v, got %v", tc.expectPassed, result.Passed)
			}

			if !tc.expectPassed && tc.expectReason != "" {
				if result.Reason != tc.expectReason {
					t.Errorf("Expected reason = %s, got %s", tc.expectReason, result.Reason)
				}
			}
		})
	}
}

func TestFreshnessGuardByRegime(t *testing.T) {
	// Enable regime-aware guards for testing
	os.Setenv("GUARDS_REGIME_AWARE", "true")
	defer os.Unsetenv("GUARDS_REGIME_AWARE")

	profiles := guards.RegimeProfiles{
		Trending: guards.RegimeProfile{
			Freshness: guards.FreshnessProfile{
				MaxBarsAge:          3,    // Relaxed for trending
				RequiresVADR:        1.75, // TRENDING-specific condition
				RequiresTightSpread: true, // TRENDING-specific condition
				SpreadThresholdBps:  50.0,
			},
		},
		Chop: guards.RegimeProfile{
			Freshness: guards.FreshnessProfile{
				MaxBarsAge:          2,   // Baseline
				RequiresVADR:        0.0, // Not required for chop
				RequiresTightSpread: false,
				SpreadThresholdBps:  100.0,
			},
		},
	}

	safetyLimits := guards.SafetyLimits{
		MaxMomentumThreshold: 25.0,
		MaxRSIThreshold:      80.0,
		MaxDelaySecondsAbs:   60,
		MaxBarsAgeAbs:        5,
		MinATRFactor:         0.8,
	}

	regimeGuards := guards.NewRegimeAwareGuards(profiles, safetyLimits)
	ctx := context.Background()

	testCases := []struct {
		name         string
		regime       string
		barsAge      int
		vadr         float64
		spreadBps    float64
		expectPassed bool
		expectReason string
	}{
		// TRENDING regime tests
		{
			name:         "TrendingRegime_SlightlyStaleGoodVADRSpread_ShouldPass",
			regime:       "TRENDING",
			barsAge:      3,    // Above chop limit (2) but within trending (3)
			vadr:         2.0,  // Above VADR requirement (1.75)
			spreadBps:    40.0, // Below spread threshold (50bps)
			expectPassed: true,
		},
		{
			name:         "TrendingRegime_SlightlyStaleBADVADR_ShouldFail",
			regime:       "TRENDING",
			barsAge:      3,
			vadr:         1.5, // Below VADR requirement (1.75)
			spreadBps:    40.0,
			expectPassed: false,
			expectReason: "trending_freshness_requires_vadr_1.75_got_1.50",
		},
		{
			name:         "TrendingRegime_SlightlyStaleBADSpread_ShouldFail",
			regime:       "TRENDING",
			barsAge:      3,
			vadr:         2.0,
			spreadBps:    60.0, // Above spread threshold (50bps)
			expectPassed: false,
			expectReason: "trending_freshness_requires_tight_spread_50.0bps_got_60.0bps",
		},
		{
			name:         "TrendingRegime_TooStale_ShouldFail",
			regime:       "TRENDING",
			barsAge:      4, // Above trending limit (3)
			vadr:         2.0,
			spreadBps:    40.0,
			expectPassed: false,
			expectReason: "stale_data_4_bars_exceeds_3",
		},
		// CHOP regime tests
		{
			name:         "ChopRegime_Fresh_ShouldPass",
			regime:       "CHOP",
			barsAge:      2,    // At chop limit (2)
			vadr:         1.0,  // VADR not checked for chop
			spreadBps:    80.0, // Spread not checked for chop
			expectPassed: true,
		},
		{
			name:         "ChopRegime_Stale_ShouldFail",
			regime:       "CHOP",
			barsAge:      3, // Above chop limit (2)
			vadr:         2.0,
			spreadBps:    40.0,
			expectPassed: false,
			expectReason: "stale_data_3_bars_exceeds_2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := guards.GuardInputs{
				Symbol:    "BTCUSD",
				Regime:    tc.regime,
				BarsAge:   tc.barsAge,
				VADR:      tc.vadr,
				SpreadBps: tc.spreadBps,
				Timestamp: time.Now(),
			}

			result := regimeGuards.EvaluateFreshnessGuard(ctx, inputs)

			if result.Passed != tc.expectPassed {
				t.Errorf("Expected passed = %v, got %v", tc.expectPassed, result.Passed)
			}

			if !tc.expectPassed && tc.expectReason != "" {
				if result.Reason != tc.expectReason {
					t.Errorf("Expected reason = %s, got %s", tc.expectReason, result.Reason)
				}
			}
		})
	}
}

func TestSafetyLimitsEnforcement(t *testing.T) {
	// Enable regime-aware guards for testing
	os.Setenv("GUARDS_REGIME_AWARE", "true")
	defer os.Unsetenv("GUARDS_REGIME_AWARE")

	// Create profiles with values that exceed safety limits
	profiles := guards.RegimeProfiles{
		Trending: guards.RegimeProfile{
			Fatigue: guards.FatigueProfile{
				MomentumThreshold: 30.0, // Above safety limit (25%)
				RSIThreshold:      85.0, // Above safety limit (80)
			},
			LateFill: guards.LateFillProfile{
				MaxDelaySeconds: 70, // Above safety limit (60s)
			},
			Freshness: guards.FreshnessProfile{
				MaxBarsAge: 10, // Above safety limit (5)
			},
		},
	}

	safetyLimits := guards.SafetyLimits{
		MaxMomentumThreshold: 25.0, // Should cap momentum threshold
		MaxRSIThreshold:      80.0, // Should cap RSI threshold
		MaxDelaySecondsAbs:   60,   // Should cap delay
		MaxBarsAgeAbs:        5,    // Should cap bars age
		MinATRFactor:         0.8,
	}

	regimeGuards := guards.NewRegimeAwareGuards(profiles, safetyLimits)
	ctx := context.Background()

	// Test fatigue guard safety limits
	t.Run("FatigueGuard_SafetyLimitsEnforced", func(t *testing.T) {
		inputs := guards.GuardInputs{
			Symbol:      "BTCUSD",
			Regime:      "TRENDING",
			Momentum24h: 27.0, // Above configured threshold (30%) but should be capped at 25%
			RSI4h:       82.0, // Above configured threshold (85) but should be capped at 80
			Timestamp:   time.Now(),
		}

		result := regimeGuards.EvaluateFatigueGuard(ctx, inputs)

		// Should fail because momentum and RSI are above safety limits
		if result.Passed {
			t.Error("Expected fatigue guard to fail due to safety limit enforcement")
		}

		// Should contain reference to safety-limited thresholds in threshold description
		expectedThresholdPattern := "momentum_25.0_rsi_80.0" // Safety limits applied
		if !contains(result.ThresholdUsed, expectedThresholdPattern) {
			t.Errorf("Expected threshold description to show safety limits, got: %s", result.ThresholdUsed)
		}
	})

	// Test late-fill guard safety limits
	t.Run("LateFillGuard_SafetyLimitsEnforced", func(t *testing.T) {
		inputs := guards.GuardInputs{
			Symbol:    "BTCUSD",
			Regime:    "TRENDING",
			FillDelay: 65 * time.Second, // Above configured limit (70s) but should be capped at 60s
			Timestamp: time.Now(),
		}

		result := regimeGuards.EvaluateLateFillGuard(ctx, inputs)

		// Should fail because delay exceeds safety limit
		if result.Passed {
			t.Error("Expected late-fill guard to fail due to safety limit enforcement")
		}

		expectedReason := "late_fill_delay_65s_exceeds_60s" // Safety limit applied
		if result.Reason != expectedReason {
			t.Errorf("Expected reason showing safety limit, got: %s", result.Reason)
		}
	})

	// Test freshness guard safety limits
	t.Run("FreshnessGuard_SafetyLimitsEnforced", func(t *testing.T) {
		inputs := guards.GuardInputs{
			Symbol:    "BTCUSD",
			Regime:    "TRENDING",
			BarsAge:   8, // Above configured limit (10) but should be capped at 5
			Timestamp: time.Now(),
		}

		result := regimeGuards.EvaluateFreshnessGuard(ctx, inputs)

		// Should fail because bars age exceeds safety limit
		if result.Passed {
			t.Error("Expected freshness guard to fail due to safety limit enforcement")
		}

		expectedReason := "stale_data_8_bars_exceeds_5" // Safety limit applied
		if result.Reason != expectedReason {
			t.Errorf("Expected reason showing safety limit, got: %s", result.Reason)
		}
	})
}

func TestLegacyBehaviorWhenDisabled(t *testing.T) {
	// Disable regime-aware guards
	os.Setenv("GUARDS_REGIME_AWARE", "false")
	defer os.Unsetenv("GUARDS_REGIME_AWARE")

	profiles := guards.RegimeProfiles{
		Trending: guards.RegimeProfile{
			Fatigue: guards.FatigueProfile{
				MomentumThreshold: 18.0, // Different from chop
			},
		},
		Chop: guards.RegimeProfile{
			Fatigue: guards.FatigueProfile{
				MomentumThreshold: 12.0, // Baseline/legacy
			},
		},
	}

	safetyLimits := guards.SafetyLimits{
		MaxMomentumThreshold: 25.0,
		MaxRSIThreshold:      80.0,
		MaxDelaySecondsAbs:   60,
		MaxBarsAgeAbs:        5,
		MinATRFactor:         0.8,
	}

	regimeGuards := guards.NewRegimeAwareGuards(profiles, safetyLimits)
	ctx := context.Background()

	// Test that all regimes use baseline (chop) thresholds when disabled
	testCases := []struct {
		name   string
		regime string
	}{
		{"TrendingRegime_UsesBaselineWhenDisabled", "TRENDING"},
		{"ChopRegime_UsesBaseline", "CHOP"},
		{"HighVolRegime_UsesBaselineWhenDisabled", "HIGH_VOL"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := guards.GuardInputs{
				Symbol:      "BTCUSD",
				Regime:      tc.regime,
				Momentum24h: 15.0, // Above chop baseline (12%) but below trending (18%)
				RSI4h:       75.0, // Above RSI threshold
				Timestamp:   time.Now(),
			}

			result := regimeGuards.EvaluateFatigueGuard(ctx, inputs)

			// Should fail for all regimes using baseline (chop) thresholds
			if result.Passed {
				t.Errorf("Expected guard to fail using baseline thresholds for regime %s", tc.regime)
			}

			// Should indicate regime-aware is disabled
			if result.RegimeAware {
				t.Error("Expected RegimeAware = false when feature disabled")
			}

			// Should use baseline threshold description
			if !contains(result.ThresholdUsed, "baseline_legacy") {
				t.Errorf("Expected baseline threshold description, got: %s", result.ThresholdUsed)
			}
		})
	}
}

func TestAllGuardsEvaluation(t *testing.T) {
	// Enable regime-aware guards for testing
	os.Setenv("GUARDS_REGIME_AWARE", "true")
	defer os.Unsetenv("GUARDS_REGIME_AWARE")

	profiles := guards.RegimeProfiles{
		Trending: guards.RegimeProfile{
			Fatigue:   guards.FatigueProfile{MomentumThreshold: 18.0, RSIThreshold: 70.0, RequiresAccelRenewal: true},
			LateFill:  guards.LateFillProfile{MaxDelaySeconds: 45, RequiresInfraHealth: true, RequiresATRProximity: true, ATRFactor: 1.2},
			Freshness: guards.FreshnessProfile{MaxBarsAge: 3, RequiresVADR: 1.75, RequiresTightSpread: true, SpreadThresholdBps: 50.0},
		},
	}

	safetyLimits := guards.SafetyLimits{
		MaxMomentumThreshold: 25.0,
		MaxRSIThreshold:      80.0,
		MaxDelaySecondsAbs:   60,
		MaxBarsAgeAbs:        5,
		MinATRFactor:         0.8,
	}

	regimeGuards := guards.NewRegimeAwareGuards(profiles, safetyLimits)
	ctx := context.Background()

	// Test all guards pass
	t.Run("AllGuards_Pass", func(t *testing.T) {
		inputs := guards.GuardInputs{
			Symbol:          "BTCUSD",
			Regime:          "TRENDING",
			Momentum24h:     10.0,                   // Below fatigue threshold
			RSI4h:           65.0,                   // Below RSI threshold
			Acceleration:    2.0,                    // Positive acceleration
			FillDelay:       25 * time.Second,       // Fast fill
			InfraP99Latency: 200 * time.Millisecond, // Good infrastructure
			ATRDistance:     1.0,                    // Within ATR
			ATR1h:           1.0,
			BarsAge:         2,    // Fresh
			VADR:            2.0,  // Above VADR requirement
			SpreadBps:       30.0, // Tight spread
			Timestamp:       time.Now(),
		}

		results := regimeGuards.EvaluateAllGuards(ctx, inputs)

		if len(results) != 3 {
			t.Errorf("Expected 3 guard results, got %d", len(results))
		}

		// All guards should pass
		for _, result := range results {
			if !result.Passed {
				t.Errorf("Guard %s failed with reason: %s", result.GuardType, result.Reason)
			}
		}
	})

	// Test one guard fails
	t.Run("OneGuard_Fails", func(t *testing.T) {
		inputs := guards.GuardInputs{
			Symbol:          "BTCUSD",
			Regime:          "TRENDING",
			Momentum24h:     20.0, // Above fatigue threshold - should fail
			RSI4h:           75.0, // Above RSI threshold - should fail
			Acceleration:    2.0,  // Positive acceleration
			FillDelay:       25 * time.Second,
			InfraP99Latency: 200 * time.Millisecond,
			ATRDistance:     1.0,
			ATR1h:           1.0,
			BarsAge:         2,
			VADR:            2.0,
			SpreadBps:       30.0,
			Timestamp:       time.Now(),
		}

		results := regimeGuards.EvaluateAllGuards(ctx, inputs)

		fatigueResult := findGuardResult(results, "fatigue")
		if fatigueResult == nil {
			t.Fatal("Fatigue guard result not found")
		}

		if fatigueResult.Passed {
			t.Error("Expected fatigue guard to fail")
		}

		// Other guards should still pass
		lateFillResult := findGuardResult(results, "late_fill")
		freshnessResult := findGuardResult(results, "freshness")

		if lateFillResult == nil || !lateFillResult.Passed {
			t.Error("Expected late-fill guard to pass")
		}

		if freshnessResult == nil || !freshnessResult.Passed {
			t.Error("Expected freshness guard to pass")
		}
	})
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			len(s) > len(substr) &&
				(s[1:len(substr)+1] == substr ||
					s[len(s)-len(substr)-1:len(s)-1] == substr))
}

func findGuardResult(results []guards.GuardResult, guardType string) *guards.GuardResult {
	for i, result := range results {
		if result.GuardType == guardType {
			return &results[i]
		}
	}
	return nil
}
