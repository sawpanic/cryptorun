package guards_test

import (
	"testing"

	"github.com/sawpanic/cryptorun/internal/domain/guards"
)

func TestFatigueGuard_BaselineProfile(t *testing.T) {
	config := guards.FatigueConfig{
		Baseline: guards.FatigueThresholds{
			Momentum24hThreshold: 12.0,
			RSI4hThreshold:       70.0,
			AccelerationOverride: 2.0,
			RequiresAccelRenewal: false,
		},
		MaxMomentum: 25.0,
		MaxRSI:      80.0,
	}

	tests := []struct {
		name     string
		inputs   guards.FatigueInputs
		expected bool
		reason   string
	}{
		{
			name: "allow_low_momentum",
			inputs: guards.FatigueInputs{
				Symbol:       "BTCUSD",
				Momentum24h:  8.0,  // < 12% threshold
				RSI4h:        75.0, // > 70 but momentum OK
				Acceleration: 1.0,
				AccelRenewal: false,
				Regime:       guards.RegimeChoppy,
			},
			expected: true,
			reason:   "momentum_ok",
		},
		{
			name: "allow_low_rsi",
			inputs: guards.FatigueInputs{
				Symbol:       "ETHUSD",
				Momentum24h:  15.0, // > 12% but RSI OK
				RSI4h:        65.0, // < 70 threshold
				Acceleration: 1.0,
				AccelRenewal: false,
				Regime:       guards.RegimeChoppy,
			},
			expected: true,
			reason:   "rsi_ok",
		},
		{
			name: "allow_acceleration_override",
			inputs: guards.FatigueInputs{
				Symbol:       "SOLUSD",
				Momentum24h:  15.0, // > 12%
				RSI4h:        75.0, // > 70
				Acceleration: 2.5,  // >= 2% override
				AccelRenewal: false,
				Regime:       guards.RegimeChoppy,
			},
			expected: true,
			reason:   "acceleration_override",
		},
		{
			name: "block_overextended",
			inputs: guards.FatigueInputs{
				Symbol:       "ADAUSD",
				Momentum24h:  15.0, // > 12%
				RSI4h:        75.0, // > 70
				Acceleration: 1.0,  // < 2% no override
				AccelRenewal: false,
				Regime:       guards.RegimeChoppy,
			},
			expected: false,
			reason:   "overextended",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guards.EvaluateFatigueGuard(tt.inputs, config, false)

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

func TestFatigueGuard_TrendingProfile(t *testing.T) {
	config := guards.FatigueConfig{
		Baseline: guards.FatigueThresholds{
			Momentum24hThreshold: 12.0,
			RSI4hThreshold:       70.0,
			AccelerationOverride: 2.0,
			RequiresAccelRenewal: false,
		},
		TrendingProfile: guards.FatigueThresholds{
			Momentum24hThreshold: 18.0, // Higher threshold
			RSI4hThreshold:       70.0,
			AccelerationOverride: 2.0,
			RequiresAccelRenewal: true, // Safety condition
		},
		MaxMomentum: 25.0,
		MaxRSI:      80.0,
	}

	tests := []struct {
		name          string
		inputs        guards.FatigueInputs
		regimeAware   bool
		expected      bool
		expectProfile string
		reason        string
	}{
		{
			name: "trending_regime_with_accel_renewal_allows_higher_momentum",
			inputs: guards.FatigueInputs{
				Symbol:       "BTCUSD",
				Momentum24h:  15.0, // Would block in baseline, OK in trending
				RSI4h:        65.0, // < 70
				Acceleration: 1.0,
				AccelRenewal: true, // Safety condition met
				Regime:       guards.RegimeTrending,
			},
			regimeAware:   true,
			expected:      true,
			expectProfile: "trending",
			reason:        "rsi_ok",
		},
		{
			name: "trending_regime_without_accel_renewal_uses_baseline",
			inputs: guards.FatigueInputs{
				Symbol:       "ETHUSD",
				Momentum24h:  15.0, // Would block in baseline
				RSI4h:        75.0, // > 70
				Acceleration: 1.0,
				AccelRenewal: false, // Safety condition NOT met
				Regime:       guards.RegimeTrending,
			},
			regimeAware:   true,
			expected:      false, // Uses baseline thresholds
			expectProfile: "baseline",
			reason:        "overextended",
		},
		{
			name: "regime_aware_disabled_always_uses_baseline",
			inputs: guards.FatigueInputs{
				Symbol:       "SOLUSD",
				Momentum24h:  15.0,
				RSI4h:        75.0,
				Acceleration: 1.0,
				AccelRenewal: true, // Even with accel renewal
				Regime:       guards.RegimeTrending,
			},
			regimeAware:   false, // Feature flag off
			expected:      false,
			expectProfile: "baseline",
			reason:        "overextended",
		},
		{
			name: "trending_allows_18_percent_momentum",
			inputs: guards.FatigueInputs{
				Symbol:       "MATICUSD",
				Momentum24h:  17.0, // Between baseline (12%) and trending (18%)
				RSI4h:        75.0, // > 70
				Acceleration: 1.0,  // < 2% override
				AccelRenewal: true, // Safety condition met
				Regime:       guards.RegimeTrending,
			},
			regimeAware:   true,
			expected:      false, // Still blocked due to RSI + momentum
			expectProfile: "trending",
			reason:        "overextended",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guards.EvaluateFatigueGuard(tt.inputs, config, tt.regimeAware)

			if result.Allow != tt.expected {
				t.Errorf("Expected allow=%v, got allow=%v", tt.expected, result.Allow)
			}

			if result.Profile != tt.expectProfile {
				t.Errorf("Expected profile=%s, got profile=%s", tt.expectProfile, result.Profile)
			}

			if !contains(result.Reason, tt.reason) {
				t.Errorf("Expected reason to contain '%s', got: %s", tt.reason, result.Reason)
			}
		})
	}
}

func TestFatigueGuard_SafetyConstraints(t *testing.T) {
	config := guards.FatigueConfig{
		Baseline: guards.FatigueThresholds{
			Momentum24hThreshold: 12.0,
			RSI4hThreshold:       70.0,
			AccelerationOverride: 2.0,
		},
		TrendingProfile: guards.FatigueThresholds{
			Momentum24hThreshold: 30.0, // Above safety limit
			RSI4hThreshold:       85.0, // Above safety limit
			AccelerationOverride: 2.0,
			RequiresAccelRenewal: true,
		},
		MaxMomentum: 25.0, // Safety constraint
		MaxRSI:      80.0, // Safety constraint
	}

	tests := []struct {
		name     string
		inputs   guards.FatigueInputs
		expected bool
		reason   string
	}{
		{
			name: "safety_constraint_limits_momentum_threshold",
			inputs: guards.FatigueInputs{
				Symbol:       "BTCUSD",
				Momentum24h:  27.0, // Above safety limit
				RSI4h:        65.0,
				Acceleration: 1.0,
				AccelRenewal: true,
				Regime:       guards.RegimeTrending,
			},
			expected: false, // Blocked by safety constraint
			reason:   "overextended",
		},
		{
			name: "safety_constraint_limits_rsi_threshold",
			inputs: guards.FatigueInputs{
				Symbol:       "ETHUSD",
				Momentum24h:  20.0,
				RSI4h:        82.0, // Above safety limit
				Acceleration: 1.0,
				AccelRenewal: true,
				Regime:       guards.RegimeTrending,
			},
			expected: false, // Blocked by safety constraint
			reason:   "overextended",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guards.EvaluateFatigueGuard(tt.inputs, config, true)

			if result.Allow != tt.expected {
				t.Errorf("Expected allow=%v, got allow=%v", tt.expected, result.Allow)
			}

			if !contains(result.Reason, tt.reason) {
				t.Errorf("Expected reason to contain '%s', got: %s", tt.reason, result.Reason)
			}

			// Verify safety constraints were applied
			details := result.Details
			momentumThreshold := details["momentum_threshold"].(float64)
			rsiThreshold := details["rsi_threshold"].(float64)

			if momentumThreshold > 25.0 {
				t.Errorf("Safety constraint violated: momentum threshold %v > 25.0", momentumThreshold)
			}

			if rsiThreshold > 80.0 {
				t.Errorf("Safety constraint violated: RSI threshold %v > 80.0", rsiThreshold)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
