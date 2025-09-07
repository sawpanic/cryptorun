package conformance_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// GuardsConfig represents guard configuration
type GuardsConfig struct {
	Guards struct {
		RegimeAware bool `yaml:"regime_aware"`

		Fatigue struct {
			Baseline struct {
				Momentum24hThreshold float64 `yaml:"momentum_24h_threshold"`
				RSI4hThreshold       float64 `yaml:"rsi_4h_threshold"`
			} `yaml:"baseline"`

			TrendingProfile struct {
				Momentum24hThreshold float64 `yaml:"momentum_24h_threshold"`
				RSI4hThreshold       float64 `yaml:"rsi_4h_threshold"`
				RequiresAccelRenewal bool    `yaml:"requires_accel_renewal"`
			} `yaml:"trending_profile"`

			MaxMomentum float64 `yaml:"max_momentum_threshold"`
			MaxRSI      float64 `yaml:"max_rsi_threshold"`
		} `yaml:"fatigue"`

		LateFill struct {
			Baseline struct {
				MaxDelaySeconds int `yaml:"max_delay_seconds"`
			} `yaml:"baseline"`

			TrendingProfile struct {
				MaxDelaySeconds      int     `yaml:"max_delay_seconds"`
				RequiresInfraHealth  bool    `yaml:"requires_infra_health"`
				RequiresATRProximity bool    `yaml:"requires_atr_proximity"`
				ATRFactor            float64 `yaml:"atr_factor"`
			} `yaml:"trending_profile"`

			MaxDelaySecondsAbs int `yaml:"max_delay_seconds_absolute"`
		} `yaml:"late_fill"`

		Freshness struct {
			Baseline struct {
				MaxBarsAge int     `yaml:"max_bars_age"`
				ATRFactor  float64 `yaml:"atr_factor"`
			} `yaml:"baseline"`

			TrendingProfile struct {
				MaxBarsAge          int     `yaml:"max_bars_age"`
				ATRFactor           float64 `yaml:"atr_factor"`
				RequiresVADR        float64 `yaml:"requires_vadr"`
				RequiresTightSpread bool    `yaml:"requires_tight_spread"`
				SpreadThresholdBps  float64 `yaml:"spread_threshold_bps"`
			} `yaml:"trending_profile"`

			MaxBarsAgeAbs int     `yaml:"max_bars_age_absolute"`
			MinATRFactor  float64 `yaml:"min_atr_factor"`
		} `yaml:"freshness"`
	} `yaml:"guards"`
}

// TestGuardRegimeBehaviorConformance verifies guards behavior table by regime
func TestGuardRegimeBehaviorConformance(t *testing.T) {
	// Load guards configuration
	configData, err := os.ReadFile("config/guards.yaml")
	if err != nil {
		t.Fatalf("Failed to read guards config: %v", err)
	}

	var config GuardsConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse guards config: %v", err)
	}

	// Test fatigue guard regime behavior
	t.Run("FatigueGuardRegimeBehavior", func(t *testing.T) {
		// Baseline (Chop/High-Vol): 12% momentum, 70 RSI threshold
		baseline := config.Guards.Fatigue.Baseline
		if baseline.Momentum24hThreshold != 12.0 {
			t.Errorf("CONFORMANCE VIOLATION: Fatigue baseline momentum threshold = %.1f, expected 12.0",
				baseline.Momentum24hThreshold)
		}

		if baseline.RSI4hThreshold != 70.0 {
			t.Errorf("CONFORMANCE VIOLATION: Fatigue baseline RSI threshold = %.1f, expected 70.0",
				baseline.RSI4hThreshold)
		}

		// Trending: 18% momentum ONLY when accel_renewal=true
		trending := config.Guards.Fatigue.TrendingProfile
		if trending.Momentum24hThreshold != 18.0 {
			t.Errorf("CONFORMANCE VIOLATION: Fatigue trending momentum threshold = %.1f, expected 18.0",
				trending.Momentum24hThreshold)
		}

		if trending.RSI4hThreshold != 70.0 {
			t.Errorf("CONFORMANCE VIOLATION: Fatigue trending RSI threshold = %.1f, expected 70.0",
				trending.RSI4hThreshold)
		}

		if !trending.RequiresAccelRenewal {
			t.Errorf("CONFORMANCE VIOLATION: Fatigue trending profile must require accel_renewal=true")
		}

		// Safety constraints: max 25% momentum, max 80 RSI
		if config.Guards.Fatigue.MaxMomentum != 25.0 {
			t.Errorf("CONFORMANCE VIOLATION: Fatigue max momentum = %.1f, expected 25.0",
				config.Guards.Fatigue.MaxMomentum)
		}

		if config.Guards.Fatigue.MaxRSI != 80.0 {
			t.Errorf("CONFORMANCE VIOLATION: Fatigue max RSI = %.1f, expected 80.0",
				config.Guards.Fatigue.MaxRSI)
		}
	})

	// Test late-fill guard regime behavior
	t.Run("LateFillGuardRegimeBehavior", func(t *testing.T) {
		// Baseline: 30s max execution delay
		baseline := config.Guards.LateFill.Baseline
		if baseline.MaxDelaySeconds != 30 {
			t.Errorf("CONFORMANCE VIOLATION: Late-fill baseline delay = %ds, expected 30s",
				baseline.MaxDelaySeconds)
		}

		// Trending: 45s ONLY when infra_p99 < 400ms AND atr_distance <= 1.2×ATR
		trending := config.Guards.LateFill.TrendingProfile
		if trending.MaxDelaySeconds != 45 {
			t.Errorf("CONFORMANCE VIOLATION: Late-fill trending delay = %ds, expected 45s",
				trending.MaxDelaySeconds)
		}

		if !trending.RequiresInfraHealth {
			t.Errorf("CONFORMANCE VIOLATION: Late-fill trending profile must require infra health check")
		}

		if !trending.RequiresATRProximity {
			t.Errorf("CONFORMANCE VIOLATION: Late-fill trending profile must require ATR proximity check")
		}

		if trending.ATRFactor != 1.2 {
			t.Errorf("CONFORMANCE VIOLATION: Late-fill trending ATR factor = %.1f, expected 1.2",
				trending.ATRFactor)
		}

		// Safety constraints: max 60s absolute
		if config.Guards.LateFill.MaxDelaySecondsAbs != 60 {
			t.Errorf("CONFORMANCE VIOLATION: Late-fill max delay absolute = %ds, expected 60s",
				config.Guards.LateFill.MaxDelaySecondsAbs)
		}
	})

	// Test freshness guard regime behavior
	t.Run("FreshnessGuardRegimeBehavior", func(t *testing.T) {
		// Baseline: 2 bars max age, 1.2×ATR price movement limit
		baseline := config.Guards.Freshness.Baseline
		if baseline.MaxBarsAge != 2 {
			t.Errorf("CONFORMANCE VIOLATION: Freshness baseline bars age = %d, expected 2",
				baseline.MaxBarsAge)
		}

		if baseline.ATRFactor != 1.2 {
			t.Errorf("CONFORMANCE VIOLATION: Freshness baseline ATR factor = %.1f, expected 1.2",
				baseline.ATRFactor)
		}

		// Trending: 3 bars ONLY when VADR >= 1.75× AND spread < 50bps
		trending := config.Guards.Freshness.TrendingProfile
		if trending.MaxBarsAge != 3 {
			t.Errorf("CONFORMANCE VIOLATION: Freshness trending bars age = %d, expected 3",
				trending.MaxBarsAge)
		}

		if trending.ATRFactor != 1.2 {
			t.Errorf("CONFORMANCE VIOLATION: Freshness trending ATR factor = %.1f, expected 1.2",
				trending.ATRFactor)
		}

		if trending.RequiresVADR != 1.75 {
			t.Errorf("CONFORMANCE VIOLATION: Freshness trending VADR requirement = %.2f, expected 1.75",
				trending.RequiresVADR)
		}

		if !trending.RequiresTightSpread {
			t.Errorf("CONFORMANCE VIOLATION: Freshness trending profile must require tight spread check")
		}

		if trending.SpreadThresholdBps != 50.0 {
			t.Errorf("CONFORMANCE VIOLATION: Freshness trending spread threshold = %.1f bps, expected 50.0",
				trending.SpreadThresholdBps)
		}

		// Safety constraints: max 5 bars absolute, min 0.8×ATR factor
		if config.Guards.Freshness.MaxBarsAgeAbs != 5 {
			t.Errorf("CONFORMANCE VIOLATION: Freshness max bars age absolute = %d, expected 5",
				config.Guards.Freshness.MaxBarsAgeAbs)
		}

		if config.Guards.Freshness.MinATRFactor != 0.8 {
			t.Errorf("CONFORMANCE VIOLATION: Freshness min ATR factor = %.1f, expected 0.8",
				config.Guards.Freshness.MinATRFactor)
		}
	})
}

// TestGuardSafetyConstraintsConformance verifies hard safety limits
func TestGuardSafetyConstraintsConformance(t *testing.T) {
	configData, err := os.ReadFile("config/guards.yaml")
	if err != nil {
		t.Fatalf("Failed to read guards config: %v", err)
	}

	var config GuardsConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse guards config: %v", err)
	}

	// Verify trending profiles never exceed safety constraints
	t.Run("TrendingProfileSafetyConstraints", func(t *testing.T) {
		// Fatigue trending profile must not exceed safety limits
		if config.Guards.Fatigue.TrendingProfile.Momentum24hThreshold > config.Guards.Fatigue.MaxMomentum {
			t.Errorf("CONFORMANCE VIOLATION: Fatigue trending momentum %.1f exceeds safety limit %.1f",
				config.Guards.Fatigue.TrendingProfile.Momentum24hThreshold,
				config.Guards.Fatigue.MaxMomentum)
		}

		if config.Guards.Fatigue.TrendingProfile.RSI4hThreshold > config.Guards.Fatigue.MaxRSI {
			t.Errorf("CONFORMANCE VIOLATION: Fatigue trending RSI %.1f exceeds safety limit %.1f",
				config.Guards.Fatigue.TrendingProfile.RSI4hThreshold,
				config.Guards.Fatigue.MaxRSI)
		}

		// Late-fill trending profile must not exceed safety limits
		if config.Guards.LateFill.TrendingProfile.MaxDelaySeconds > config.Guards.LateFill.MaxDelaySecondsAbs {
			t.Errorf("CONFORMANCE VIOLATION: Late-fill trending delay %ds exceeds safety limit %ds",
				config.Guards.LateFill.TrendingProfile.MaxDelaySeconds,
				config.Guards.LateFill.MaxDelaySecondsAbs)
		}

		// Freshness trending profile must not exceed safety limits
		if config.Guards.Freshness.TrendingProfile.MaxBarsAge > config.Guards.Freshness.MaxBarsAgeAbs {
			t.Errorf("CONFORMANCE VIOLATION: Freshness trending bars age %d exceeds safety limit %d",
				config.Guards.Freshness.TrendingProfile.MaxBarsAge,
				config.Guards.Freshness.MaxBarsAgeAbs)
		}

		if config.Guards.Freshness.TrendingProfile.ATRFactor < config.Guards.Freshness.MinATRFactor {
			t.Errorf("CONFORMANCE VIOLATION: Freshness trending ATR factor %.1f below safety limit %.1f",
				config.Guards.Freshness.TrendingProfile.ATRFactor,
				config.Guards.Freshness.MinATRFactor)
		}
	})
}
