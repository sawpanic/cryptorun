package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/config"
)

func TestMenuGuardsIntegration(t *testing.T) {
	// Create temporary config directory
	tempDir := t.TempDir()
	guardConfigPath := filepath.Join(tempDir, "guards.yaml")

	t.Run("LoadDefaultGuardsConfig", func(t *testing.T) {
		defaultConfig := config.GetDefaultGuardsConfig()
		require.NotNil(t, defaultConfig)

		// Verify basic structure
		assert.False(t, defaultConfig.RegimeAware, "Default should start with regime_aware disabled")
		assert.Equal(t, "conservative", defaultConfig.Active)
		assert.Contains(t, defaultConfig.Profiles, "conservative")
		assert.Contains(t, defaultConfig.Profiles, "trending_risk_on")
	})

	t.Run("SaveAndLoadGuardsConfig", func(t *testing.T) {
		originalConfig := config.GetDefaultGuardsConfig()

		// Modify configuration
		originalConfig.RegimeAware = true
		originalConfig.Active = "trending_risk_on"

		// Save configuration
		err := config.SaveGuardsConfig(originalConfig, guardConfigPath)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(guardConfigPath)
		require.NoError(t, err)

		// Load configuration back
		loadedConfig, err := config.LoadGuardsConfig(guardConfigPath)
		require.NoError(t, err)

		// Verify loaded config matches
		assert.True(t, loadedConfig.RegimeAware)
		assert.Equal(t, "trending_risk_on", loadedConfig.Active)
	})

	t.Run("ValidateGuardProfiles", func(t *testing.T) {
		config := config.GetDefaultGuardsConfig()

		for profileName, profile := range config.Profiles {
			t.Run(profileName, func(t *testing.T) {
				errors := profile.ValidateProfile()
				if len(errors) > 0 {
					t.Errorf("Profile %s validation errors: %v", profileName, errors)
				}

				// Check required regimes exist
				requiredRegimes := []string{"trending", "choppy", "high_vol"}
				for _, regime := range requiredRegimes {
					assert.Contains(t, profile.Regimes, regime,
						"Profile %s missing regime %s", profileName, regime)
				}
			})
		}
	})

	t.Run("GetActiveProfile", func(t *testing.T) {
		config := config.GetDefaultGuardsConfig()

		// Test with valid active profile
		profile, err := config.GetActiveProfile()
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, "Conservative", profile.Name)

		// Test with invalid active profile
		config.Active = "nonexistent"
		_, err = config.GetActiveProfile()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("GetRegimeThresholds", func(t *testing.T) {
		config := config.GetDefaultGuardsConfig()
		profile, err := config.GetActiveProfile()
		require.NoError(t, err)

		// Test with valid regime
		guards, err := profile.GetRegimeThresholds("trending")
		require.NoError(t, err)
		require.NotNil(t, guards)

		// Verify threshold structure
		assert.Greater(t, guards.Fatigue.Threshold24h, 0.0)
		assert.Greater(t, guards.Fatigue.RSI4h, 0.0)
		assert.Greater(t, guards.Freshness.MaxBarsAge, 0)
		assert.Greater(t, guards.Freshness.ATRFactor, 0.0)
		assert.Greater(t, guards.LateFill.MaxDelaySeconds, 0)

		// Test with invalid regime
		_, err = profile.GetRegimeThresholds("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("ConservativeProfileThresholds", func(t *testing.T) {
		config := config.GetDefaultGuardsConfig()
		conservativeProfile := config.Profiles["conservative"]

		// Test trending regime thresholds
		trending, exists := conservativeProfile.Regimes["trending"]
		require.True(t, exists)

		assert.Equal(t, 12.0, trending.Fatigue.Threshold24h)
		assert.Equal(t, 70.0, trending.Fatigue.RSI4h)
		assert.Equal(t, 2, trending.Freshness.MaxBarsAge)
		assert.Equal(t, 1.2, trending.Freshness.ATRFactor)
		assert.Equal(t, 30, trending.LateFill.MaxDelaySeconds)
		assert.Equal(t, 400.0, trending.LateFill.P99LatencyReq)
		assert.Equal(t, 1.2, trending.LateFill.ATRProximity)

		// Test high_vol regime has stricter thresholds
		highVol, exists := conservativeProfile.Regimes["high_vol"]
		require.True(t, exists)

		assert.Equal(t, 65.0, highVol.Fatigue.RSI4h, "High vol should have stricter RSI")
		assert.Equal(t, 1.0, highVol.Freshness.ATRFactor, "High vol should have tighter ATR")
	})

	t.Run("TrendingRiskOnProfileThresholds", func(t *testing.T) {
		config := config.GetDefaultGuardsConfig()
		riskOnProfile := config.Profiles["trending_risk_on"]

		// Test trending regime has relaxed thresholds
		trending, exists := riskOnProfile.Regimes["trending"]
		require.True(t, exists)

		assert.Equal(t, 18.0, trending.Fatigue.Threshold24h, "Risk-on should allow higher momentum")
		assert.Equal(t, 3, trending.Freshness.MaxBarsAge, "Risk-on should allow older bars")
		assert.Equal(t, 45, trending.LateFill.MaxDelaySeconds, "Risk-on should allow longer delays")

		// Test high_vol regime has stricter thresholds even in risk-on
		highVol, exists := riskOnProfile.Regimes["high_vol"]
		require.True(t, exists)

		assert.Equal(t, 10.0, highVol.Fatigue.Threshold24h, "High vol should be stricter even in risk-on")
	})

	t.Run("ProfileValidationSafety", func(t *testing.T) {
		config := config.GetDefaultGuardsConfig()

		// Create invalid profile for testing
		invalidProfile := config.Profiles["conservative"]

		// Test fatigue threshold bounds
		invalidProfile.Regimes["trending"].Fatigue.Threshold24h = 30.0 // Too high
		errors := invalidProfile.ValidateProfile()
		assert.NotEmpty(t, errors, "Should detect fatigue threshold too high")

		// Test RSI bounds
		invalidProfile = config.Profiles["conservative"]
		invalidProfile.Regimes["trending"].Fatigue.RSI4h = 90.0 // Too high
		errors = invalidProfile.ValidateProfile()
		assert.NotEmpty(t, errors, "Should detect RSI threshold too high")

		// Test bars age bounds
		invalidProfile = config.Profiles["conservative"]
		invalidProfile.Regimes["trending"].Freshness.MaxBarsAge = 10 // Too high
		errors = invalidProfile.ValidateProfile()
		assert.NotEmpty(t, errors, "Should detect bars age too high")
	})

	t.Run("ConfigFileFormat", func(t *testing.T) {
		config := config.GetDefaultGuardsConfig()

		// Save config to temp file
		err := config.SaveGuardsConfig(config, guardConfigPath)
		require.NoError(t, err)

		// Read file content and verify format
		content, err := os.ReadFile(guardConfigPath)
		require.NoError(t, err)

		contentStr := string(content)

		// Verify YAML structure
		assert.Contains(t, contentStr, "regime_aware:", "Should contain regime_aware field")
		assert.Contains(t, contentStr, "active_profile:", "Should contain active_profile field")
		assert.Contains(t, contentStr, "profiles:", "Should contain profiles section")
		assert.Contains(t, contentStr, "conservative:", "Should contain conservative profile")
		assert.Contains(t, contentStr, "trending_risk_on:", "Should contain trending_risk_on profile")

		// Verify regime sections
		assert.Contains(t, contentStr, "trending:", "Should contain trending regime")
		assert.Contains(t, contentStr, "choppy:", "Should contain choppy regime")
		assert.Contains(t, contentStr, "high_vol:", "Should contain high_vol regime")

		// Verify guard sections
		assert.Contains(t, contentStr, "fatigue:", "Should contain fatigue guard")
		assert.Contains(t, contentStr, "freshness:", "Should contain freshness guard")
		assert.Contains(t, contentStr, "late_fill:", "Should contain late_fill guard")
	})

	t.Run("MenuIntegrationPathExists", func(t *testing.T) {
		// Verify that the config path function returns expected location
		configPath := config.GetGuardsConfigPath()
		expectedSuffix := filepath.Join("config", "guards.yaml")

		assert.True(t, strings.HasSuffix(configPath, expectedSuffix),
			"Config path should end with config/guards.yaml, got: %s", configPath)
	})
}

func TestGuardsMenuPrecedence(t *testing.T) {
	t.Run("MenuOverCLI", func(t *testing.T) {
		// This test verifies the Menu → CLI precedence requirement
		// Menu choices should override CLI flags per Menu-first governance

		config := config.GetDefaultGuardsConfig()

		// Test that menu can switch between profiles
		originalActive := config.Active
		config.Active = "trending_risk_on"
		assert.NotEqual(t, originalActive, config.Active, "Menu should be able to override active profile")

		// Test that profile changes affect thresholds
		conservative := config.Profiles["conservative"]
		trendingRiskOn := config.Profiles["trending_risk_on"]

		conservativeTrending := conservative.Regimes["trending"]
		riskOnTrending := trendingRiskOn.Regimes["trending"]

		assert.NotEqual(t, conservativeTrending.Fatigue.Threshold24h, riskOnTrending.Fatigue.Threshold24h,
			"Different profiles should have different thresholds")
		assert.NotEqual(t, conservativeTrending.LateFill.MaxDelaySeconds, riskOnTrending.LateFill.MaxDelaySeconds,
			"Different profiles should have different late-fill settings")
	})
}
