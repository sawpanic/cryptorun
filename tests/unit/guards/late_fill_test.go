package guards_test

import (
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/guards"
)

func TestLateFillGuard_BaselineProfile(t *testing.T) {
	config := guards.LateFillConfig{
		Baseline: guards.LateFillThresholds{
			MaxDelaySeconds: 30,
		},
		MaxDelaySecondsAbs: 60,
		MinDelaySeconds:    0,
	}

	baseTime := time.Now()

	tests := []struct {
		name     string
		inputs   guards.LateFillInputs
		expected bool
		reason   string
	}{
		{
			name: "allow_quick_execution",
			inputs: guards.LateFillInputs{
				Symbol:        "BTCUSD",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(15 * time.Second), // 15s < 30s
				Regime:        guards.RegimeChoppy,
			},
			expected: true,
			reason:   "timing_ok",
		},
		{
			name: "allow_at_threshold",
			inputs: guards.LateFillInputs{
				Symbol:        "ETHUSD",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(30 * time.Second), // Exactly 30s
				Regime:        guards.RegimeChoppy,
			},
			expected: true,
			reason:   "timing_ok",
		},
		{
			name: "block_late_execution",
			inputs: guards.LateFillInputs{
				Symbol:        "SOLUSD",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(35 * time.Second), // 35s > 30s
				Regime:        guards.RegimeChoppy,
			},
			expected: false,
			reason:   "too_late",
		},
		{
			name: "block_negative_delay_clock_skew",
			inputs: guards.LateFillInputs{
				Symbol:        "ADAUSD",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(-5 * time.Second), // Negative delay
				Regime:        guards.RegimeChoppy,
			},
			expected: false,
			reason:   "clock_skew",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guards.EvaluateLateFillGuard(tt.inputs, config, false)

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

func TestLateFillGuard_TrendingProfile(t *testing.T) {
	config := guards.LateFillConfig{
		Baseline: guards.LateFillThresholds{
			MaxDelaySeconds: 30,
		},
		TrendingProfile: guards.LateFillThresholds{
			MaxDelaySeconds:      45,   // Higher limit in trending
			RequiresInfraHealth:  true, // Safety condition
			RequiresATRProximity: true, // Safety condition
			ATRFactor:            1.2,
		},
		MaxDelaySecondsAbs: 60,
		MinDelaySeconds:    0,
	}

	baseTime := time.Now()

	tests := []struct {
		name          string
		inputs        guards.LateFillInputs
		regimeAware   bool
		expected      bool
		expectProfile string
		reason        string
	}{
		{
			name: "trending_with_safety_conditions_met",
			inputs: guards.LateFillInputs{
				Symbol:        "BTCUSD",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(40 * time.Second), // Would block baseline, OK trending
				InfraP99MS:    350.0,                          // < 400ms (infra health OK)
				ATRDistance:   1.1,                            // < 1.2 ATR (proximity OK)
				Regime:        guards.RegimeTrending,
			},
			regimeAware:   true,
			expected:      true,
			expectProfile: "trending",
			reason:        "timing_ok",
		},
		{
			name: "trending_with_poor_infra_health_uses_baseline",
			inputs: guards.LateFillInputs{
				Symbol:        "ETHUSD",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(40 * time.Second), // Would block baseline
				InfraP99MS:    450.0,                          // > 400ms (infra health BAD)
				ATRDistance:   1.1,                            // < 1.2 ATR (proximity OK)
				Regime:        guards.RegimeTrending,
			},
			regimeAware:   true,
			expected:      false, // Uses baseline due to infra health
			expectProfile: "baseline",
			reason:        "too_late",
		},
		{
			name: "trending_with_poor_atr_proximity_uses_baseline",
			inputs: guards.LateFillInputs{
				Symbol:        "SOLUSD",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(40 * time.Second), // Would block baseline
				InfraP99MS:    350.0,                          // < 400ms (infra health OK)
				ATRDistance:   1.5,                            // > 1.2 ATR (proximity BAD)
				Regime:        guards.RegimeTrending,
			},
			regimeAware:   true,
			expected:      false, // Uses baseline due to ATR distance
			expectProfile: "baseline",
			reason:        "too_late",
		},
		{
			name: "regime_aware_disabled_uses_baseline",
			inputs: guards.LateFillInputs{
				Symbol:        "MATICUSD",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(40 * time.Second),
				InfraP99MS:    350.0, // Perfect conditions
				ATRDistance:   1.1,   // Perfect conditions
				Regime:        guards.RegimeTrending,
			},
			regimeAware:   false, // Feature flag off
			expected:      false,
			expectProfile: "baseline",
			reason:        "too_late",
		},
		{
			name: "trending_at_45_second_limit",
			inputs: guards.LateFillInputs{
				Symbol:        "AVAXUSD",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(45 * time.Second), // At trending limit
				InfraP99MS:    350.0,
				ATRDistance:   1.1,
				Regime:        guards.RegimeTrending,
			},
			regimeAware:   true,
			expected:      true,
			expectProfile: "trending",
			reason:        "timing_ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guards.EvaluateLateFillGuard(tt.inputs, config, tt.regimeAware)

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
				if _, ok := details["infra_health_ok"]; !ok {
					t.Error("Expected infra_health_ok in details")
				}
				if _, ok := details["atr_proximity_ok"]; !ok {
					t.Error("Expected atr_proximity_ok in details")
				}
			}
		})
	}
}

func TestLateFillGuard_SafetyConstraints(t *testing.T) {
	config := guards.LateFillConfig{
		Baseline: guards.LateFillThresholds{
			MaxDelaySeconds: 30,
		},
		TrendingProfile: guards.LateFillThresholds{
			MaxDelaySeconds:      90, // Above safety limit
			RequiresInfraHealth:  true,
			RequiresATRProximity: true,
			ATRFactor:            1.2,
		},
		MaxDelaySecondsAbs: 60, // Safety constraint
		MinDelaySeconds:    0,
	}

	baseTime := time.Now()

	tests := []struct {
		name     string
		inputs   guards.LateFillInputs
		expected bool
		reason   string
	}{
		{
			name: "safety_constraint_limits_delay",
			inputs: guards.LateFillInputs{
				Symbol:        "BTCUSD",
				SignalTime:    baseTime,
				ExecutionTime: baseTime.Add(70 * time.Second), // Above safety limit
				InfraP99MS:    350.0,                          // Perfect conditions
				ATRDistance:   1.1,                            // Perfect conditions
				Regime:        guards.RegimeTrending,
			},
			expected: false, // Blocked by safety constraint
			reason:   "too_late",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guards.EvaluateLateFillGuard(tt.inputs, config, true)

			if result.Allow != tt.expected {
				t.Errorf("Expected allow=%v, got allow=%v", tt.expected, result.Allow)
			}

			if !contains(result.Reason, tt.reason) {
				t.Errorf("Expected reason to contain '%s', got: %s", tt.reason, result.Reason)
			}

			// Verify safety constraint was applied
			details := result.Details
			maxDelay := details["max_delay"].(int)
			if maxDelay > 60 {
				t.Errorf("Safety constraint violated: max delay %v > 60s", maxDelay)
			}
		})
	}
}
