package unit

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sawpanic/cryptorun/internal/data/validate"
)

// TestAnomalyCheckerCreation tests creating anomaly checker with various configurations
func TestAnomalyCheckerCreation(t *testing.T) {
	tests := []struct {
		name           string
		config         validate.AnomalyConfig
		expectedMAD    float64
		expectedWindow int
	}{
		{
			name: "default_config",
			config: validate.AnomalyConfig{
				// Use defaults
			},
			expectedMAD:    3.0,
			expectedWindow: 100,
		},
		{
			name: "hot_tier_config",
			config: validate.AnomalyConfig{
				MADThreshold:     2.5,
				WindowSize:       50,
				MinDataPoints:    10,
				PriceFields:      []string{"price", "close"},
				VolumeFields:     []string{"volume"},
				EnableQuarantine: true,
			},
			expectedMAD:    2.5,
			expectedWindow: 50,
		},
		{
			name: "warm_tier_config",
			config: validate.AnomalyConfig{
				MADThreshold:     3.5,
				WindowSize:       200,
				MinDataPoints:    30,
				PriceFields:      []string{"price", "open", "high", "low", "close"},
				VolumeFields:     []string{"volume", "base_volume", "quote_volume"},
				EnableQuarantine: false,
			},
			expectedMAD:    3.5,
			expectedWindow: 200,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checker := validate.NewAnomalyChecker(test.config)
			assert.NotNil(t, checker, "Checker should not be nil")
			
			// Test that defaults are applied
			metrics := checker.GetMetrics()
			assert.NotNil(t, metrics, "Metrics should be initialized")
			assert.Equal(t, int64(0), metrics.TotalChecks, "Initial checks should be zero")
		})
	}
}

// TestPriceAnomalyDetection tests price anomaly detection using MAD scores
func TestPriceAnomalyDetection(t *testing.T) {
	config := validate.AnomalyConfig{
		MADThreshold:     3.0,
		WindowSize:       50,
		MinDataPoints:    5, // Lower for testing
		PriceFields:      []string{"price"},
		EnableQuarantine: true,
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Feed normal price data to establish baseline
	normalPrices := []float64{100.0, 101.0, 99.0, 102.0, 98.0, 100.5, 99.5, 101.5, 98.5}
	
	for i, price := range normalPrices {
		data := map[string]interface{}{
			"price":     price,
			"timestamp": time.Now().Add(-time.Duration(len(normalPrices)-i) * time.Minute),
		}
		
		result := checker.CheckAnomaly(data, "test")
		assert.False(t, result.IsAnomaly, "Normal price %.1f should not be anomaly", price)
	}
	
	// Now test anomalous prices
	tests := []struct {
		name             string
		price            float64
		expectAnomaly    bool
		expectQuarantine bool
	}{
		{
			name:             "slightly_high_price",
			price:            110.0, // Within normal range
			expectAnomaly:    false,
			expectQuarantine: false,
		},
		{
			name:             "very_high_price",
			price:            200.0, // Way above normal
			expectAnomaly:    true,
			expectQuarantine: true,
		},
		{
			name:             "negative_price",
			price:            -10.0, // Invalid negative price
			expectAnomaly:    true,
			expectQuarantine: true,
		},
		{
			name:             "zero_price",
			price:            0.0, // Invalid zero price
			expectAnomaly:    true,
			expectQuarantine: true,
		},
		{
			name:             "extremely_low_price",
			price:            1.0, // Extremely low but positive
			expectAnomaly:    true,
			expectQuarantine: false, // Might not quarantine based on severity
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := map[string]interface{}{
				"price":     test.price,
				"timestamp": time.Now(),
			}
			
			result := checker.CheckAnomaly(data, "test")
			
			assert.Equal(t, test.expectAnomaly, result.IsAnomaly, 
				"Price %.1f anomaly detection mismatch", test.price)
				
			if test.expectAnomaly {
				assert.NotEmpty(t, result.Reason, "Anomaly should have reason")
				assert.Equal(t, "price", result.Field, "Should identify price field")
				assert.Equal(t, test.price, result.Value, "Should record anomalous value")
				
				if test.expectQuarantine {
					assert.True(t, result.ShouldQuarantine, 
						"Price %.1f should be quarantined", test.price)
				}
			}
		})
	}
}

// TestVolumeAnomalyDetection tests volume anomaly detection including spikes
func TestVolumeAnomalyDetection(t *testing.T) {
	config := validate.AnomalyConfig{
		MADThreshold:     3.0,
		SpikeThreshold:   5.0, // 5x median for spike detection
		WindowSize:       30,
		MinDataPoints:    5,
		VolumeFields:     []string{"volume"},
		EnableQuarantine: false, // Don't quarantine volume spikes
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Feed normal volume data
	normalVolumes := []float64{1.0, 1.2, 0.8, 1.1, 0.9, 1.3, 0.7, 1.0, 1.1, 0.9}
	
	for _, volume := range normalVolumes {
		data := map[string]interface{}{
			"volume":    volume,
			"timestamp": time.Now(),
		}
		
		result := checker.CheckAnomaly(data, "test")
		assert.False(t, result.IsAnomaly, "Normal volume %.1f should not be anomaly", volume)
	}
	
	tests := []struct {
		name             string
		volume           float64
		expectAnomaly    bool
		expectedType     validate.AnomalyType
		expectQuarantine bool
	}{
		{
			name:             "normal_volume",
			volume:           1.0,
			expectAnomaly:    false,
		},
		{
			name:             "volume_spike",
			volume:           10.0, // 10x normal volume (spike)
			expectAnomaly:    true,
			expectedType:     validate.AnomalyTypeSpike,
			expectQuarantine: false, // Spikes not quarantined by default
		},
		{
			name:             "extreme_volume_spike",
			volume:           100.0, // 100x normal volume
			expectAnomaly:    true,
			expectedType:     validate.AnomalyTypeSpike,
			expectQuarantine: false,
		},
		{
			name:             "negative_volume",
			volume:           -1.0, // Invalid negative volume
			expectAnomaly:    true,
			expectedType:     validate.AnomalyTypeCorruption,
			expectQuarantine: true, // Data corruption always quarantined
		},
		{
			name:             "zero_volume_allowed",
			volume:           0.0, // Zero volume is valid
			expectAnomaly:    false,
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := map[string]interface{}{
				"volume":    test.volume,
				"timestamp": time.Now(),
			}
			
			result := checker.CheckAnomaly(data, "test")
			
			assert.Equal(t, test.expectAnomaly, result.IsAnomaly,
				"Volume %.1f anomaly detection mismatch", test.volume)
				
			if test.expectAnomaly {
				assert.Equal(t, test.expectedType, result.AnomalyType,
					"Wrong anomaly type for volume %.1f", test.volume)
				assert.Equal(t, test.expectQuarantine, result.ShouldQuarantine,
					"Quarantine decision mismatch for volume %.1f", test.volume)
			}
		})
	}
}

// TestDataCorruptionDetection tests detection of obviously corrupted data
func TestDataCorruptionDetection(t *testing.T) {
	config := validate.AnomalyConfig{
		EnableQuarantine: true,
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	tests := []struct {
		name             string
		data             map[string]interface{}
		expectAnomaly    bool
		expectedType     validate.AnomalyType
		expectQuarantine bool
		expectedField    string
	}{
		{
			name: "normal_data",
			data: map[string]interface{}{
				"price":  100.0,
				"volume": 1.5,
			},
			expectAnomaly: false,
		},
		{
			name: "nan_price",
			data: map[string]interface{}{
				"price":  math.NaN(),
				"volume": 1.5,
			},
			expectAnomaly:    true,
			expectedType:     validate.AnomalyTypeCorruption,
			expectQuarantine: true,
			expectedField:    "price",
		},
		{
			name: "infinite_price",
			data: map[string]interface{}{
				"price":  math.Inf(1),
				"volume": 1.5,
			},
			expectAnomaly:    true,
			expectedType:     validate.AnomalyTypeCorruption,
			expectQuarantine: true,
			expectedField:    "price",
		},
		{
			name: "negative_infinite_volume",
			data: map[string]interface{}{
				"price":  100.0,
				"volume": math.Inf(-1),
			},
			expectAnomaly:    true,
			expectedType:     validate.AnomalyTypeCorruption,
			expectQuarantine: true,
			expectedField:    "volume",
		},
		{
			name: "negative_price",
			data: map[string]interface{}{
				"price":  -50.0,
				"volume": 1.5,
			},
			expectAnomaly:    true,
			expectedType:     validate.AnomalyTypeCorruption,
			expectQuarantine: true,
			expectedField:    "price",
		},
		{
			name: "zero_price",
			data: map[string]interface{}{
				"price":  0.0,
				"volume": 1.5,
			},
			expectAnomaly:    true,
			expectedType:     validate.AnomalyTypeCorruption,
			expectQuarantine: true,
			expectedField:    "price",
		},
		{
			name: "negative_volume",
			data: map[string]interface{}{
				"price":       100.0,
				"base_volume": -2.0,
			},
			expectAnomaly:    true,
			expectedType:     validate.AnomalyTypeCorruption,
			expectQuarantine: true,
			expectedField:    "base_volume",
		},
		{
			name: "multiple_price_fields",
			data: map[string]interface{}{
				"open":   100.0,
				"high":   105.0,
				"low":    -5.0, // Invalid negative low
				"close":  102.0,
				"volume": 1.5,
			},
			expectAnomaly:    true,
			expectedType:     validate.AnomalyTypeCorruption,
			expectQuarantine: true,
			expectedField:    "low",
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := checker.CheckAnomaly(test.data, "test")
			
			assert.Equal(t, test.expectAnomaly, result.IsAnomaly,
				"Corruption detection mismatch for %s", test.name)
				
			if test.expectAnomaly {
				assert.Equal(t, test.expectedType, result.AnomalyType,
					"Wrong anomaly type for %s", test.name)
				assert.Equal(t, test.expectQuarantine, result.ShouldQuarantine,
					"Quarantine decision mismatch for %s", test.name)
				assert.Equal(t, test.expectedField, result.Field,
					"Wrong field identified for %s", test.name)
				assert.Contains(t, result.Reason, "corruption",
					"Reason should mention corruption for %s", test.name)
			}
		})
	}
}

// TestMADCalculation tests Median Absolute Deviation calculation
func TestMADCalculation(t *testing.T) {
	config := validate.AnomalyConfig{
		MADThreshold:  3.0,
		WindowSize:    10,
		MinDataPoints: 3,
		PriceFields:   []string{"price"},
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Test with known data set where we can predict MAD
	knownPrices := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} // Median = 5.5
	// Absolute deviations: [4.5, 3.5, 2.5, 1.5, 0.5, 0.5, 1.5, 2.5, 3.5, 4.5]
	// MAD = median of deviations = 2.5
	
	for _, price := range knownPrices {
		data := map[string]interface{}{
			"price": price,
		}
		result := checker.CheckAnomaly(data, "test")
		// All values should be within 3 * 2.5 = 7.5 of median (5.5)
		// So range [5.5 - 7.5, 5.5 + 7.5] = [-2, 13]
		// All our values (1-10) should be normal
		assert.False(t, result.IsAnomaly, "Price %.1f should be normal", price)
	}
	
	// Now test values outside the expected range
	extremeValues := []float64{-5.0, 20.0} // Outside [-2, 13] range
	
	for _, price := range extremeValues {
		data := map[string]interface{}{
			"price": price,
		}
		result := checker.CheckAnomaly(data, "test")
		
		if price > 0 { // Positive extreme values should be MAD anomalies
			assert.True(t, result.IsAnomaly, "Price %.1f should be anomaly", price)
			assert.Equal(t, validate.AnomalyTypePrice, result.AnomalyType)
		} else { // Negative prices are corruption
			assert.True(t, result.IsAnomaly, "Price %.1f should be anomaly", price)
			assert.Equal(t, validate.AnomalyTypeCorruption, result.AnomalyType)
		}
	}
}

// TestAnomalyCheckerMetrics tests metrics tracking
func TestAnomalyCheckerMetrics(t *testing.T) {
	config := validate.AnomalyConfig{
		MADThreshold:     2.0, // Lower threshold for more anomalies
		WindowSize:       20,
		MinDataPoints:    3,
		PriceFields:      []string{"price"},
		EnableQuarantine: true,
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Initially metrics should be zero
	metrics := checker.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalChecks)
	assert.Equal(t, int64(0), metrics.AnomaliesFound)
	assert.Equal(t, int64(0), metrics.QuarantineCount)
	
	// Feed some normal data
	normalData := []float64{100, 101, 99, 102, 98}
	for _, price := range normalData {
		data := map[string]interface{}{"price": price}
		checker.CheckAnomaly(data, "test")
	}
	
	metrics = checker.GetMetrics()
	assert.Equal(t, int64(5), metrics.TotalChecks)
	assert.Equal(t, int64(0), metrics.AnomaliesFound) // No anomalies yet
	
	// Feed anomalous data
	anomalousData := []float64{-10.0, 1000.0} // Corruption and extreme value
	for _, price := range anomalousData {
		data := map[string]interface{}{"price": price}
		result := checker.CheckAnomaly(data, "test")
		assert.True(t, result.IsAnomaly, "Price %.1f should be anomaly", price)
	}
	
	metrics = checker.GetMetrics()
	assert.Equal(t, int64(7), metrics.TotalChecks) // 5 + 2
	assert.Equal(t, int64(2), metrics.AnomaliesFound) // 2 anomalies
	assert.Greater(t, metrics.QuarantineCount, int64(0)) // At least some quarantined
	assert.True(t, metrics.LastCheckTime.After(time.Now().Add(-time.Minute)))
}

// TestAnomalyCheckerReset tests resetting the checker state
func TestAnomalyCheckerReset(t *testing.T) {
	config := validate.AnomalyConfig{
		PriceFields:  []string{"price"},
		VolumeFields: []string{"volume"},
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Feed some data
	data := map[string]interface{}{
		"price":  100.0,
		"volume": 1.5,
	}
	checker.CheckAnomaly(data, "test")
	
	// Verify metrics are non-zero
	metrics := checker.GetMetrics()
	assert.Greater(t, metrics.TotalChecks, int64(0))
	
	// Reset and verify
	checker.Reset()
	metrics = checker.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalChecks)
	assert.Equal(t, int64(0), metrics.AnomaliesFound)
	assert.Equal(t, int64(0), metrics.QuarantineCount)
}

// TestAnomalyCheckFnWrapper tests the validation function wrapper
func TestAnomalyCheckFnWrapper(t *testing.T) {
	config := validate.AnomalyConfig{
		MADThreshold:     2.0,
		PriceFields:      []string{"price"},
		EnableQuarantine: true,
	}
	
	// Create the validation function
	validator := validate.AnomalyCheckFn(config, "test-tier")
	assert.NotNil(t, validator, "Validator function should not be nil")
	
	// Test with normal data - should pass
	normalData := map[string]interface{}{
		"price": 100.0,
	}
	err := validator(normalData)
	assert.NoError(t, err, "Normal data should pass validation")
	
	// Test with corrupted data - should fail if quarantined
	corruptData := map[string]interface{}{
		"price": -50.0, // Negative price - corruption
	}
	err = validator(corruptData)
	assert.Error(t, err, "Corrupted data should fail validation")
	assert.Contains(t, err.Error(), "anomaly detected", "Error should mention anomaly")
}

// TestConcurrentAnomalyChecking tests anomaly checking under concurrent access
func TestConcurrentAnomalyChecking(t *testing.T) {
	config := validate.AnomalyConfig{
		MADThreshold:  3.0,
		WindowSize:    100,
		MinDataPoints: 5,
		PriceFields:   []string{"price"},
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Feed initial data to establish baseline
	for i := 0; i < 10; i++ {
		data := map[string]interface{}{"price": 100.0 + float64(i)}
		checker.CheckAnomaly(data, "test")
	}
	
	// Run concurrent checks
	concurrency := 50
	done := make(chan bool, concurrency)
	
	for i := 0; i < concurrency; i++ {
		go func(index int) {
			defer func() { done <- true }()
			
			for j := 0; j < 10; j++ {
				data := map[string]interface{}{
					"price": 100.0 + float64(index*10+j),
				}
				result := checker.CheckAnomaly(data, "test")
				// Just verify we don't panic - result correctness tested elsewhere
				_ = result
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < concurrency; i++ {
		<-done
	}
	
	// Verify final metrics make sense
	metrics := checker.GetMetrics()
	expectedChecks := int64(10 + concurrency*10) // Initial + concurrent checks
	assert.Equal(t, expectedChecks, metrics.TotalChecks)
	
	t.Logf("Concurrent checks completed: %d total checks, %d anomalies found",
		metrics.TotalChecks, metrics.AnomaliesFound)
}