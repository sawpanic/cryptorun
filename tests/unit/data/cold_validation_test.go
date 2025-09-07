package data

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/data"
	"github.com/sawpanic/cryptorun/internal/quality"
)

func TestColdData_ValidationIntegration(t *testing.T) {
	t.Run("validation_enabled", func(t *testing.T) {
		config := data.ColdDataConfig{
			EnableCSV:   true,
			BasePath:    "data/cold",
			CacheExpiry: "1h",
			Quality: quality.QualityConfig{
				Validation: quality.ValidationConfig{
					Enable: true,
					Schema: quality.SchemaConfig{
						RequireOHLCV:     true,
						RequireTimestamp: true,
						RequireVenue:     true,
						RequireSymbol:    true,
					},
					Types: quality.TypeValidationConfig{
						SymbolRegex:    "^[A-Z]{3,10}(USD|USDT|USDC)$",
						VenueWhitelist: []string{"binance", "okx", "coinbase", "kraken"},
					},
				},
				Scoring: quality.ScoringConfig{
					Enable: true,
					Weights: quality.ScoringWeights{
						Freshness:    0.25,
						Completeness: 0.25,
						Consistency:  0.25,
						AnomalyFree:  0.25,
					},
					Thresholds: quality.ScoringThresholds{
						Excellent:  95,
						Good:       85,
						Acceptable: 70,
						Poor:       50,
					},
				},
				AnomalyDetection: quality.AnomalyDetectionConfig{
					Enable:        true,
					WindowSize:    24,
					Sensitivity:   2.5,
					MinDataPoints: 5,
					PriceAnomalies: quality.PriceAnomalyConfig{
						Enable:              true,
						MaxDeviationPercent: 20.0,
					},
					VolumeAnomalies: quality.VolumeAnomalyConfig{
						Enable:          true,
						SpikeMultiplier: 5.0,
					},
				},
			},
		}

		coldData, err := data.NewColdData(config)
		require.NoError(t, err)
		assert.NotNil(t, coldData)

		ctx := context.Background()

		// Test single envelope validation
		validEnvelope := &data.Envelope{
			Symbol:    "BTCUSD",
			Venue:     "kraken",
			Timestamp: time.Now(),
			PriceData: map[string]interface{}{
				"open":  50000.0,
				"high":  51000.0,
				"low":   49500.0,
				"close": 50500.0,
			},
			VolumeData: map[string]interface{}{
				"volume": 100.0,
			},
			SourceTier: data.TierCold,
		}

		result, err := coldData.ValidateEnvelope(ctx, validEnvelope)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Valid)
		assert.Greater(t, result.QualityScore, 0.0)
		assert.NotEmpty(t, result.QualityLevel)
	})

	t.Run("validation_disabled", func(t *testing.T) {
		config := data.ColdDataConfig{
			EnableCSV:   true,
			BasePath:    "data/cold",
			CacheExpiry: "1h",
			// Quality config with all features disabled
			Quality: quality.QualityConfig{
				Validation:       quality.ValidationConfig{Enable: false},
				Scoring:          quality.ScoringConfig{Enable: false},
				AnomalyDetection: quality.AnomalyDetectionConfig{Enable: false},
			},
		}

		coldData, err := data.NewColdData(config)
		require.NoError(t, err)

		ctx := context.Background()
		envelope := &data.Envelope{Symbol: "BTCUSD", Venue: "kraken"}

		// Should fail when validator is not initialized
		result, err := coldData.ValidateEnvelope(ctx, envelope)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "data validator not initialized")
	})

	t.Run("batch_validation", func(t *testing.T) {
		config := data.ColdDataConfig{
			EnableCSV: true,
			Quality: quality.QualityConfig{
				Validation: quality.ValidationConfig{
					Enable: true,
					Schema: quality.SchemaConfig{RequireSymbol: true, RequireVenue: true},
					Types: quality.TypeValidationConfig{
						SymbolRegex:    ".*",
						VenueWhitelist: []string{"kraken", "binance"},
					},
				},
				Scoring: quality.ScoringConfig{Enable: true},
			},
		}

		coldData, err := data.NewColdData(config)
		require.NoError(t, err)

		ctx := context.Background()
		envelopes := []*data.Envelope{
			{Symbol: "BTCUSD", Venue: "kraken", Timestamp: time.Now()},
			{Symbol: "ETHUSD", Venue: "binance", Timestamp: time.Now()},
			{Symbol: "", Venue: "kraken", Timestamp: time.Now()}, // Invalid - missing symbol
		}

		results, err := coldData.ValidateEnvelopes(ctx, envelopes)
		assert.NoError(t, err)
		require.Len(t, results, 3)

		assert.True(t, results[0].Valid)
		assert.True(t, results[1].Valid)
		assert.False(t, results[2].Valid) // Invalid envelope
		assert.Contains(t, results[2].Errors, "missing required field: symbol")
	})

	t.Run("quarantine_functionality", func(t *testing.T) {
		config := data.ColdDataConfig{
			EnableCSV: true,
			Quality: quality.QualityConfig{
				Validation: quality.ValidationConfig{
					Enable:              true,
					QuarantineThreshold: 2,
					RecoveryThreshold:   2,
					Schema:              quality.SchemaConfig{RequireSymbol: true},
					Types: quality.TypeValidationConfig{
						SymbolRegex:    ".*",
						VenueWhitelist: []string{"kraken"},
					},
				},
			},
		}

		coldData, err := data.NewColdData(config)
		require.NoError(t, err)

		ctx := context.Background()
		symbol := "BTCUSD"

		// Initially not quarantined
		assert.False(t, coldData.IsSymbolQuarantined(symbol))

		// Add failed validations
		failedEnvelope := &data.Envelope{
			Symbol: symbol,
			Venue:  "unknown_venue", // Not in whitelist
		}

		// Trigger quarantine
		for i := 0; i < 3; i++ {
			_, err := coldData.ValidateEnvelope(ctx, failedEnvelope)
			assert.NoError(t, err)
		}

		// Should be quarantined
		assert.True(t, coldData.IsSymbolQuarantined(symbol))

		// Check stats
		stats := coldData.GetValidationStats(symbol)
		assert.Greater(t, stats.FailedValidations, 0)
		assert.Greater(t, stats.ConsecutiveFails, 0)
		assert.True(t, stats.Quarantined)

		// Recovery
		validEnvelope := &data.Envelope{
			Symbol: symbol,
			Venue:  "kraken",
		}

		for i := 0; i < 2; i++ {
			_, err := coldData.ValidateEnvelope(ctx, validEnvelope)
			assert.NoError(t, err)
		}

		// Should be recovered
		assert.False(t, coldData.IsSymbolQuarantined(symbol))
	})

	t.Run("metrics_callback", func(t *testing.T) {
		config := data.ColdDataConfig{
			EnableCSV: true,
			Quality: quality.QualityConfig{
				Validation: quality.ValidationConfig{Enable: true},
				Scoring:    quality.ScoringConfig{Enable: true},
			},
		}

		coldData, err := data.NewColdData(config)
		require.NoError(t, err)

		// Set metrics callback
		var metricsReceived = make(map[string]float64)
		coldData.SetValidationMetricsCallback(func(metric string, value float64) {
			metricsReceived[metric] = value
		})

		ctx := context.Background()
		envelope := &data.Envelope{Symbol: "BTCUSD", Venue: "kraken"}

		_, err = coldData.ValidateEnvelope(ctx, envelope)
		assert.NoError(t, err)

		// Check that metrics were reported
		assert.Greater(t, len(metricsReceived), 0)
		assert.Contains(t, metricsReceived, "data_validation_success")
	})
}

func TestColdData_AnomalyDetection(t *testing.T) {
	config := data.ColdDataConfig{
		EnableCSV: true,
		Quality: quality.QualityConfig{
			AnomalyDetection: quality.AnomalyDetectionConfig{
				Enable:        true,
				WindowSize:    1, // 1 hour for testing
				Sensitivity:   2.0,
				MinDataPoints: 3,
				PriceAnomalies: quality.PriceAnomalyConfig{
					Enable:              true,
					MaxDeviationPercent: 15.0,
				},
				VolumeAnomalies: quality.VolumeAnomalyConfig{
					Enable:          true,
					SpikeMultiplier: 3.0,
				},
			},
			Validation: quality.ValidationConfig{
				Types: quality.TypeValidationConfig{
					SymbolRegex:    ".*",
					VenueWhitelist: []string{"kraken"},
				},
			},
		},
	}

	coldData, err := data.NewColdData(config)
	require.NoError(t, err)

	ctx := context.Background()
	symbol := "BTCUSD"

	// Add baseline data
	baselineEnvelopes := []*data.Envelope{
		{Symbol: symbol, Venue: "kraken", Timestamp: time.Now().Add(-30 * time.Minute),
			PriceData: map[string]interface{}{"close": 50000.0}, VolumeData: map[string]interface{}{"volume": 100.0}},
		{Symbol: symbol, Venue: "kraken", Timestamp: time.Now().Add(-20 * time.Minute),
			PriceData: map[string]interface{}{"close": 50100.0}, VolumeData: map[string]interface{}{"volume": 105.0}},
		{Symbol: symbol, Venue: "kraken", Timestamp: time.Now().Add(-10 * time.Minute),
			PriceData: map[string]interface{}{"close": 49900.0}, VolumeData: map[string]interface{}{"volume": 95.0}},
	}

	// Process baseline data
	for _, env := range baselineEnvelopes {
		_, err := coldData.ValidateEnvelope(ctx, env)
		assert.NoError(t, err)
	}

	t.Run("price_anomaly_detection", func(t *testing.T) {
		anomalyEnvelope := &data.Envelope{
			Symbol:    symbol,
			Venue:     "kraken",
			Timestamp: time.Now(),
			PriceData: map[string]interface{}{
				"close": 60000.0, // 20% above baseline
			},
			VolumeData: map[string]interface{}{"volume": 100.0},
		}

		result, err := coldData.ValidateEnvelope(ctx, anomalyEnvelope)
		assert.NoError(t, err)
		assert.Greater(t, len(result.Anomalies), 0)

		// Should have price anomaly
		priceAnomalyFound := false
		for _, anomaly := range result.Anomalies {
			if anomaly.Type == "price" {
				priceAnomalyFound = true
				assert.Greater(t, anomaly.Value, 15.0) // Above threshold
			}
		}
		assert.True(t, priceAnomalyFound)
	})

	t.Run("volume_spike_detection", func(t *testing.T) {
		spikeEnvelope := &data.Envelope{
			Symbol:    symbol,
			Venue:     "kraken",
			Timestamp: time.Now(),
			PriceData: map[string]interface{}{"close": 50000.0},
			VolumeData: map[string]interface{}{
				"volume": 350.0, // 3.5x baseline volume
			},
		}

		result, err := coldData.ValidateEnvelope(ctx, spikeEnvelope)
		assert.NoError(t, err)
		assert.Greater(t, len(result.Anomalies), 0)

		// Should have volume spike anomaly
		volumeAnomalyFound := false
		for _, anomaly := range result.Anomalies {
			if anomaly.Type == "volume" && strings.Contains(anomaly.Description, "spike") {
				volumeAnomalyFound = true
			}
		}
		assert.True(t, volumeAnomalyFound)
	})
}

func TestColdData_QualityScoring(t *testing.T) {
	config := data.ColdDataConfig{
		EnableCSV: true,
		Quality: quality.QualityConfig{
			MaxStalenessSeconds: map[string]int{
				"cold": 3600, // 1 hour
			},
			Scoring: quality.ScoringConfig{
				Enable: true,
				Weights: quality.ScoringWeights{
					Freshness:    0.4,
					Completeness: 0.3,
					Consistency:  0.2,
					AnomalyFree:  0.1,
				},
				Thresholds: quality.ScoringThresholds{
					Excellent:  95,
					Good:       85,
					Acceptable: 70,
					Poor:       50,
				},
			},
			Validation: quality.ValidationConfig{
				Schema: quality.SchemaConfig{RequireOHLCV: true},
				Types: quality.TypeValidationConfig{
					SymbolRegex:    ".*",
					VenueWhitelist: []string{"kraken"},
				},
			},
		},
	}

	coldData, err := data.NewColdData(config)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("high_quality_data", func(t *testing.T) {
		envelope := &data.Envelope{
			Symbol:     "BTCUSD",
			Venue:      "kraken",
			Timestamp:  time.Now().Add(-30 * time.Minute), // Fresh for cold tier
			SourceTier: data.TierCold,
			PriceData: map[string]interface{}{
				"open": 50000.0, "high": 51000.0, "low": 49500.0, "close": 50500.0,
			},
			VolumeData: map[string]interface{}{"volume": 100.0},
		}

		result, err := coldData.ValidateEnvelope(ctx, envelope)
		assert.NoError(t, err)
		assert.True(t, result.Valid)
		assert.Greater(t, result.QualityScore, 80.0)
		assert.Contains(t, []string{"excellent", "good"}, result.QualityLevel)
	})

	t.Run("poor_quality_data", func(t *testing.T) {
		envelope := &data.Envelope{
			Symbol:     "BTCUSD",
			Venue:      "kraken",
			Timestamp:  time.Now().Add(-2 * time.Hour), // Stale
			SourceTier: data.TierCold,
			PriceData: map[string]interface{}{
				"open":  50000.0,
				"high":  49000.0, // High < Low - inconsistent
				"low":   49500.0,
				"close": 50500.0,
			},
			// Missing volume data - incomplete
		}

		result, err := coldData.ValidateEnvelope(ctx, envelope)
		assert.NoError(t, err)
		assert.Less(t, result.QualityScore, 60.0)
		assert.Contains(t, []string{"poor", "critical"}, result.QualityLevel)
	})
}

func TestColdData_ValidationAndLoadFile(t *testing.T) {
	config := data.ColdDataConfig{
		EnableCSV:     true,
		DefaultFormat: "csv",
		BasePath:      "data/cold",
		Quality: quality.QualityConfig{
			Validation: quality.ValidationConfig{
				Enable:   true,
				FailFast: false, // Don't filter invalid data
				Types: quality.TypeValidationConfig{
					SymbolRegex:    ".*",
					VenueWhitelist: []string{"kraken", "test"},
				},
			},
			Scoring: quality.ScoringConfig{Enable: true},
		},
	}

	coldData, err := data.NewColdData(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Test the integrated validation and loading
	// Note: This uses the mock CSV reader that returns deterministic fake data
	envelopes, validationResults, err := coldData.ValidateAndLoadFile(ctx, "test.csv", "test", "TESTUSD")
	
	// Should succeed with mock data
	assert.NoError(t, err)
	assert.Greater(t, len(envelopes), 0)
	assert.Greater(t, len(validationResults), 0)
	assert.Len(t, validationResults, len(envelopes))

	// Check that validation was performed
	for _, result := range validationResults {
		assert.NotNil(t, result)
		assert.Greater(t, result.Metrics.ProcessingTimeMs, 0.0)
	}
}

func TestColdData_FailFastValidation(t *testing.T) {
	config := data.ColdDataConfig{
		EnableCSV: true,
		Quality: quality.QualityConfig{
			Validation: quality.ValidationConfig{
				Enable:   true,
				FailFast: true, // Filter out invalid data
				Schema:   quality.SchemaConfig{RequireSymbol: true},
				Types: quality.TypeValidationConfig{
					SymbolRegex:    "^[A-Z]+USD$", // Strict pattern
					VenueWhitelist: []string{"kraken"},
				},
			},
		},
	}

	coldData, err := data.NewColdData(config)
	require.NoError(t, err)

	// Create a mock that returns mixed valid/invalid data
	mixedEnvelopes := []*data.Envelope{
		{Symbol: "BTCUSD", Venue: "kraken"}, // Valid
		{Symbol: "INVALID", Venue: "kraken"}, // Invalid symbol
		{Symbol: "ETHUSD", Venue: "unknown"}, // Invalid venue
		{Symbol: "SOLUSD", Venue: "kraken"}, // Valid
	}

	ctx := context.Background()
	validationResults, err := coldData.ValidateEnvelopes(ctx, mixedEnvelopes)
	assert.NoError(t, err)
	require.Len(t, validationResults, 4)

	// Count valid and invalid results
	validCount := 0
	for _, result := range validationResults {
		if result.Valid {
			validCount++
		}
	}

	assert.Equal(t, 2, validCount) // Should have 2 valid envelopes
}