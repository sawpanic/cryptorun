package guards_test

import (
	"testing"

	"cryptorun/internal/domain/guards"
)

func TestFreshnessGuard_BaselineProfile(t *testing.T) {
	config := guards.FreshnessConfig{
		Baseline: guards.FreshnessThresholds{
			MaxBarsAge: 2,
			ATRFactor:  1.2,
		},
		MaxBarsAgeAbs: 5,
		MinATRFactor:  0.8,
	}

	tests := []struct {
		name     string
		inputs   guards.FreshnessInputs
		expected bool
		reason   string
	}{
		{
			name: "allow_fresh_signal",
			inputs: guards.FreshnessInputs{
				Symbol:      "BTCUSD",
				BarsAge:     1,     // < 2 bars
				PriceChange: 180.0, // Price change
				ATR1h:       200.0, // 0.9x ATR < 1.2x limit
				Regime:      guards.RegimeChoppy,
			},
			expected: true,
			reason:   "fresh",
		},
		{
			name: "allow_at_age_threshold",
			inputs: guards.FreshnessInputs{
				Symbol:      "ETHUSD",
				BarsAge:     2,     // Exactly 2 bars
				PriceChange: 120.0, // Price change
				ATR1h:       100.0, // Exactly 1.2x ATR
				Regime:      guards.RegimeChoppy,
			},
			expected: true,
			reason:   "fresh",
		},
		{
			name: "block_too_old",
			inputs: guards.FreshnessInputs{
				Symbol:      "SOLUSD",
				BarsAge:     3,     // > 2 bars
				PriceChange: 80.0,  // Price movement OK
				ATR1h:       100.0, // 0.8x ATR < 1.2x limit
				Regime:      guards.RegimeChoppy,
			},
			expected: false,
			reason:   "too_old",
		},
		{
			name: "block_price_moved_too_much",
			inputs: guards.FreshnessInputs{
				Symbol:      "ADAUSD",
				BarsAge:     1,     // Age OK
				PriceChange: 250.0, // Price change
				ATR1h:       200.0, // 1.25x ATR > 1.2x limit
				Regime:      guards.RegimeChoppy,
			},
			expected: false,
			reason:   "price_moved_too_much",
		},
		{
			name: "handle_zero_atr",
			inputs: guards.FreshnessInputs{
				Symbol:      "MATICUSD",
				BarsAge:     1, // Age OK
				PriceChange: 100.0,
				ATR1h:       0.0, // Zero ATR edge case
				Regime:      guards.RegimeChoppy,
			},
			expected: true, // Should not block on zero ATR
			reason:   "fresh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guards.EvaluateFreshnessGuard(tt.inputs, config, false)

			if result.Allow != tt.expected {
				t.Errorf("Expected allow=%v, got allow=%v", tt.expected, result.Allow)
			}

			if !contains(result.Reason, tt.reason) {
				t.Errorf("Expected reason to contain '%s', got: %s", tt.reason, result.Reason)
			}

			if result.Profile != "baseline" {
				t.Errorf("Expected baseline profile, got: %s", result.Profile)
			}
		})
	}
}

func TestFreshnessGuard_TrendingProfile(t *testing.T) {
	config := guards.FreshnessConfig{
		Baseline: guards.FreshnessThresholds{
			MaxBarsAge: 2,
			ATRFactor:  1.2,
		},
		TrendingProfile: guards.FreshnessThresholds{
			MaxBarsAge:          3,    // Higher limit in trending
			ATRFactor:           1.2,  // Keep ATR factor same
			RequiresVADR:        1.75, // Safety condition
			RequiresTightSpread: true, // Safety condition
			SpreadThresholdBps:  50.0, // 50 bps threshold
		},
		MaxBarsAgeAbs: 5,
		MinATRFactor:  0.8,
	}

	tests := []struct {
		name          string
		inputs        guards.FreshnessInputs
		regimeAware   bool
		expected      bool
		expectProfile string
		reason        string
	}{
		{
			name: "trending_with_safety_conditions_met",
			inputs: guards.FreshnessInputs{
				Symbol:      "BTCUSD",
				BarsAge:     3, // Would block baseline, OK trending
				PriceChange: 180.0,
				ATR1h:       200.0, // 0.9x ATR OK
				VADR:        2.0,   // >= 1.75 (VADR OK)
				SpreadBps:   40.0,  // < 50 bps (spread OK)
				Regime:      guards.RegimeTrending,
			},
			regimeAware:   true,
			expected:      true,
			expectProfile: "trending",
			reason:        "fresh",
		},
		{
			name: "trending_with_poor_vadr_uses_baseline",
			inputs: guards.FreshnessInputs{
				Symbol:      "ETHUSD",
				BarsAge:     3, // Would block baseline
				PriceChange: 180.0,
				ATR1h:       200.0,
				VADR:        1.5,  // < 1.75 (VADR BAD)
				SpreadBps:   40.0, // < 50 bps (spread OK)
				Regime:      guards.RegimeTrending,
			},
			regimeAware:   true,
			expected:      false, // Uses baseline due to VADR
			expectProfile: "baseline",
			reason:        "too_old",
		},
		{
			name: "trending_with_wide_spread_uses_baseline",
			inputs: guards.FreshnessInputs{
				Symbol:      "SOLUSD",
				BarsAge:     3, // Would block baseline
				PriceChange: 180.0,
				ATR1h:       200.0,
				VADR:        2.0,  // >= 1.75 (VADR OK)
				SpreadBps:   60.0, // > 50 bps (spread BAD)
				Regime:      guards.RegimeTrending,
			},
			regimeAware:   true,
			expected:      false, // Uses baseline due to spread
			expectProfile: "baseline",
			reason:        "too_old",
		},
		{
			name: "regime_aware_disabled_uses_baseline",
			inputs: guards.FreshnessInputs{
				Symbol:      "MATICUSD",
				BarsAge:     3, // Would block baseline
				PriceChange: 180.0,
				ATR1h:       200.0,
				VADR:        2.0,  // Perfect conditions
				SpreadBps:   40.0, // Perfect conditions
				Regime:      guards.RegimeTrending,
			},
			regimeAware:   false, // Feature flag off
			expected:      false,
			expectProfile: "baseline",
			reason:        "too_old",
		},
		{
			name: "trending_profile_respects_atr_factor",
			inputs: guards.FreshnessInputs{
				Symbol:      "AVAXUSD",
				BarsAge:     2,     // Age OK for both profiles
				PriceChange: 250.0, // Price change
				ATR1h:       200.0, // 1.25x ATR > 1.2x limit
				VADR:        2.0,   // Perfect VADR
				SpreadBps:   40.0,  // Perfect spread
				Regime:      guards.RegimeTrending,
			},
			regimeAware:   true,
			expected:      false, // Still blocked by ATR factor
			expectProfile: "trending",
			reason:        "price_moved_too_much",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guards.EvaluateFreshnessGuard(tt.inputs, config, tt.regimeAware)

			if result.Allow != tt.expected {
				t.Errorf("Expected allow=%v, got allow=%v", tt.expected, result.Allow)
			}

			if result.Profile != tt.expectProfile {
				t.Errorf("Expected profile=%s, got profile=%s", tt.expectProfile, result.Profile)
			}

			if !contains(result.Reason, tt.reason) {
				t.Errorf("Expected reason to contain '%s', got: %s", tt.reason, result.Reason)
			}

			// Verify safety condition details are included
			if tt.regimeAware && tt.inputs.Regime == guards.RegimeTrending {
				details := result.Details
				if _, ok := details["vadr_ok"]; !ok {
					t.Error("Expected vadr_ok in details")
				}
				if _, ok := details["spread_ok"]; !ok {
					t.Error("Expected spread_ok in details")
				}
			}
		})
	}
}

func TestFreshnessGuard_SafetyConstraints(t *testing.T) {
	config := guards.FreshnessConfig{
		Baseline: guards.FreshnessThresholds{
			MaxBarsAge: 2,
			ATRFactor:  1.2,
		},
		TrendingProfile: guards.FreshnessThresholds{
			MaxBarsAge:          10,  // Above safety limit
			ATRFactor:           0.5, // Below safety limit
			RequiresVADR:        1.75,
			RequiresTightSpread: true,
			SpreadThresholdBps:  50.0,
		},
		MaxBarsAgeAbs: 5,   // Safety constraint
		MinATRFactor:  0.8, // Safety constraint
	}

	tests := []struct {
		name     string
		inputs   guards.FreshnessInputs
		expected bool
		reason   string
	}{
		{
			name: "safety_constraint_limits_max_bars_age",
			inputs: guards.FreshnessInputs{
				Symbol:      "BTCUSD",
				BarsAge:     4, // Between trending (10) and safety (5) limits
				PriceChange: 180.0,
				ATR1h:       200.0,
				VADR:        2.0,  // Perfect conditions
				SpreadBps:   40.0, // Perfect conditions
				Regime:      guards.RegimeTrending,
			},
			expected: true, // Allowed up to safety limit
			reason:   "fresh",
		},
		{
			name: "safety_constraint_enforces_min_atr_factor",
			inputs: guards.FreshnessInputs{
				Symbol:      "ETHUSD",
				BarsAge:     2,     // Age OK
				PriceChange: 180.0, // Price change
				ATR1h:       200.0, // 0.9x ATR should be OK with min 0.8x
				VADR:        2.0,   // Perfect conditions
				SpreadBps:   40.0,  // Perfect conditions
				Regime:      guards.RegimeTrending,
			},
			expected: true, // Allowed due to safety constraint
			reason:   "fresh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guards.EvaluateFreshnessGuard(tt.inputs, config, true)

			if result.Allow != tt.expected {
				t.Errorf("Expected allow=%v, got allow=%v", tt.expected, result.Allow)
			}

			if !contains(result.Reason, tt.reason) {
				t.Errorf("Expected reason to contain '%s', got: %s", tt.reason, result.Reason)
			}

			// Verify safety constraints were applied
			details := result.Details
			maxBarsAge := details["max_bars_age"].(int)
			atrFactorLimit := details["atr_factor_limit"].(float64)

			if maxBarsAge > 5 {
				t.Errorf("Safety constraint violated: max bars age %v > 5", maxBarsAge)
			}

			if atrFactorLimit < 0.8 {
				t.Errorf("Safety constraint violated: ATR factor %v < 0.8", atrFactorLimit)
			}
		})
	}
}
