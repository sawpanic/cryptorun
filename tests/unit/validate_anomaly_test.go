package unit

import (
	"math"
	"testing"
	"time"

	"github.com/cryptorun/internal/data/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnomalyChecker_CheckAnomaly(t *testing.T) {
	// Create test configuration
	config := validate.AnomalyConfig{
		MADThreshold:   3.0,
		SpikeThreshold: 5.0,
		WindowSize:     20,
		MinDataPoints:  5,
		PriceFields:    []string{"price", "close"},
		VolumeFields:   []string{"volume", "base_volume"},
		EnableQuarantine: true,
	}
	
	checker := validate.NewAnomalyChecker(config)
	require.NotNil(t, checker)
	
	// Seed with normal data to build a baseline
	normalData := generateNormalDataPoints(50, 100.0, 1000.0) // price=100, volume=1000
	for _, data := range normalData {
		result := checker.CheckAnomaly(data, "hot")
		// Should not be anomalous during baseline building
		if result.IsAnomaly {
			t.Logf("Baseline data flagged as anomaly: %+v", result)
		}
	}
	
	tests := []struct {
		name            string
		data            map[string]interface{}
		tier            string
		expectAnomaly   bool
		expectQuarantine bool
		expectedType    validate.AnomalyType
		description     string
	}{
		{
			name: "normal_price_and_volume",
			data: map[string]interface{}{
				"price":  102.0,
				"volume": 1050.0,
			},
			tier:          "hot",
			expectAnomaly: false,
			description:   "Normal variations should not be flagged",
		},
		{
			name: "price_outlier_extreme",
			data: map[string]interface{}{
				"price":  500.0, // 5x normal price
				"volume": 1000.0,
			},
			tier:            "hot",
			expectAnomaly:   true,
			expectedType:    validate.AnomalyTypePrice,
			expectQuarantine: true,
			description:     "Extreme price outlier should be detected and quarantined",
		},
		{
			name: "volume_spike",
			data: map[string]interface{}{
				"price":  100.0,
				"volume": 6000.0, // 6x normal volume (exceeds spike threshold)
			},
			tier:          "hot",
			expectAnomaly: true,
			expectedType:  validate.AnomalyTypeSpike,
			expectQuarantine: false, // Volume spikes are often legitimate
			description:   "Volume spike should be detected but not quarantined",
		},
		{
			name: "negative_price_corruption",
			data: map[string]interface{}{
				"price":  -50.0,
				"volume": 1000.0,
			},
			tier:            "hot",
			expectAnomaly:   true,
			expectedType:    validate.AnomalyTypeCorruption,
			expectQuarantine: true,
			description:     "Negative price indicates data corruption",
		},
		{
			name: "negative_volume_corruption",
			data: map[string]interface{}{
				"price":  100.0,
				"volume": -100.0,
			},
			tier:            "hot",
			expectAnomaly:   true,
			expectedType:    validate.AnomalyTypeCorruption,
			expectQuarantine: true,
			description:     "Negative volume indicates data corruption",
		},
		{
			name: "nan_value_corruption",
			data: map[string]interface{}{
				"price":  math.NaN(),
				"volume": 1000.0,
			},
			tier:            "hot",
			expectAnomaly:   true,
			expectedType:    validate.AnomalyTypeCorruption,
			expectQuarantine: true,
			description:     "NaN values indicate data corruption",
		},
		{
			name: "infinite_value_corruption",
			data: map[string]interface{}{
				"price":  100.0,
				"volume": math.Inf(1),
			},
			tier:            "hot",
			expectAnomaly:   true,
			expectedType:    validate.AnomalyTypeCorruption,
			expectQuarantine: true,
			description:     "Infinite values indicate data corruption",
		},
		{
			name: "multiple_fields_normal",
			data: map[string]interface{}{
				"price":       101.0,
				"close":       100.5,
				"volume":      1100.0,
				"base_volume": 1050.0,
			},
			tier:          "hot",
			expectAnomaly: false,
			description:   "Normal values across multiple fields should be fine",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.CheckAnomaly(tt.data, tt.tier)
			require.NotNil(t, result)
			
			assert.Equal(t, tt.expectAnomaly, result.IsAnomaly, tt.description)
			
			if tt.expectAnomaly {
				assert.Equal(t, tt.expectedType, result.AnomalyType, "Anomaly type should match")
				assert.Equal(t, tt.expectQuarantine, result.ShouldQuarantine, "Quarantine decision should match")
				assert.NotEmpty(t, result.Reason, "Should provide reason for anomaly")
				assert.NotEmpty(t, result.Field, "Should identify the problematic field")
				assert.NotNil(t, result.Value, "Should capture the anomalous value")
				
				if result.AnomalyType == validate.AnomalyTypePrice || result.AnomalyType == validate.AnomalyTypeVolume {
					assert.Greater(t, math.Abs(result.MADScore), 0.0, "MAD score should be calculated")
					assert.NotNil(t, result.ExpectedRange, "Should provide expected range")
				}
			}
			
			// Validate result metadata
			assert.True(t, result.DetectedAt.After(time.Now().Add(-time.Second)))
			if result.Metadata != nil {
				assert.Equal(t, tt.tier, result.Metadata["tier"])
			}
		})
	}
}

func TestAnomalyChecker_MADCalculation(t *testing.T) {
	config := validate.AnomalyConfig{
		MADThreshold:  2.0,
		WindowSize:    10,
		MinDataPoints: 5,
		PriceFields:   []string{"price"},
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Test with known data where we can verify MAD calculation
	testData := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	
	// Feed the data
	for _, value := range testData {
		data := map[string]interface{}{"price": value}
		checker.CheckAnomaly(data, "test")
	}
	
	// Test outlier detection
	outlierData := map[string]interface{}{"price": 20.0} // Should be detected as outlier
	result := checker.CheckAnomaly(outlierData, "test")
	
	assert.True(t, result.IsAnomaly, "Value 20 should be detected as anomaly in 1-10 range")
	assert.Equal(t, validate.AnomalyTypePrice, result.AnomalyType)
	assert.Greater(t, math.Abs(result.MADScore), config.MADThreshold)
	
	// Verify MAD calculation manually
	// For data 1,2,3,4,5,6,7,8,9,10:
	// Median = 5.5
	// Deviations: |1-5.5|=4.5, |2-5.5|=3.5, ..., |10-5.5|=4.5
	// Sorted deviations: 0.5, 0.5, 1.5, 1.5, 2.5, 2.5, 3.5, 3.5, 4.5, 4.5
	// MAD = median of deviations = 2.5
	// For value 20: MAD score = |20 - 5.5| / 2.5 = 14.5 / 2.5 = 5.8
	expectedMADScore := 5.8
	assert.InDelta(t, expectedMADScore, math.Abs(result.MADScore), 0.1, "MAD score calculation should be accurate")
}

func TestAnomalyChecker_SpikeDetection(t *testing.T) {
	config := validate.AnomalyConfig{
		SpikeThreshold: 3.0, // 3x median
		WindowSize:     10,
		MinDataPoints:  5,
		VolumeFields:   []string{"volume"},
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Build baseline with consistent volume
	baseVolume := 1000.0
	for i := 0; i < 10; i++ {
		data := map[string]interface{}{"volume": baseVolume + float64(i)*10} // 1000, 1010, 1020, ...
		checker.CheckAnomaly(data, "test")
	}
	
	tests := []struct {
		name        string
		volume      float64
		expectSpike bool
		description string
	}{
		{
			name:        "normal_volume",
			volume:      1100.0,
			expectSpike: false,
			description: "Normal volume variation should not trigger spike detection",
		},
		{
			name:        "moderate_spike",
			volume:      2000.0, // ~2x median
			expectSpike: false,
			description: "2x median should not trigger spike (threshold is 3x)",
		},
		{
			name:        "volume_spike",
			volume:      4000.0, // ~4x median
			expectSpike: true,
			description: "4x median should trigger spike detection",
		},
		{
			name:        "extreme_spike",
			volume:      10000.0, // ~10x median
			expectSpike: true,
			description: "Extreme spike should be detected",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{"volume": tt.volume}
			result := checker.CheckAnomaly(data, "test")
			
			if tt.expectSpike {
				assert.True(t, result.IsAnomaly, tt.description)
				assert.Equal(t, validate.AnomalyTypeSpike, result.AnomalyType)
				assert.False(t, result.ShouldQuarantine, "Volume spikes should not be quarantined by default")
				assert.Contains(t, result.Reason, "spike")
				
				if result.Metadata != nil {
					spikeRatio, exists := result.Metadata["spike_ratio"]
					assert.True(t, exists, "Should include spike ratio in metadata")
					assert.Greater(t, spikeRatio, config.SpikeThreshold, "Spike ratio should exceed threshold")
				}
			} else {
				// Might still be anomalous due to MAD, but not a spike
				if result.IsAnomaly {
					assert.NotEqual(t, validate.AnomalyTypeSpike, result.AnomalyType, "Should not be classified as spike")
				}
			}
		})
	}
}

func TestAnomalyChecker_WindowSizeAndMinDataPoints(t *testing.T) {
	config := validate.AnomalyConfig{
		MADThreshold:  3.0,
		WindowSize:    5,  // Small window
		MinDataPoints: 3,  // Minimum required
		PriceFields:   []string{"price"},
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Test with insufficient data points
	t.Run("insufficient_data_points", func(t *testing.T) {
		// Feed only 2 data points (less than MinDataPoints)
		for i := 0; i < 2; i++ {
			data := map[string]interface{}{"price": 100.0 + float64(i)}
			result := checker.CheckAnomaly(data, "test")
			assert.False(t, result.IsAnomaly, "Should not detect anomalies with insufficient data points")
		}
	})
	
	// Test with minimum required data points
	t.Run("minimum_data_points", func(t *testing.T) {
		checker.Reset() // Clear previous data
		
		// Feed exactly minimum data points
		for i := 0; i < config.MinDataPoints; i++ {
			data := map[string]interface{}{"price": 100.0 + float64(i)}
			result := checker.CheckAnomaly(data, "test")
			
			if i < config.MinDataPoints-1 {
				assert.False(t, result.IsAnomaly, "Should not detect anomalies until minimum data points reached")
			}
		}
		
		// Now test with an outlier
		outlierData := map[string]interface{}{"price": 200.0}
		result := checker.CheckAnomaly(outlierData, "test")
		// With minimum data points, MAD calculation should work
		// Might or might not be anomalous depending on the data, but should not crash
		assert.NotNil(t, result)
	})
	
	// Test window size limitation
	t.Run("window_size_limitation", func(t *testing.T) {
		checker.Reset()
		
		// Feed more data points than window size
		for i := 0; i < config.WindowSize+5; i++ {
			data := map[string]interface{}{"price": 100.0 + float64(i)}
			checker.CheckAnomaly(data, "test")
		}
		
		// Window should be limited to WindowSize
		metrics := checker.GetMetrics()
		assert.True(t, metrics.TotalChecks >= config.WindowSize+5, "Should have processed all data points")
		
		// The internal window size should be respected (not directly testable without exposing internals)
		// But we can verify that anomaly detection still works
		outlierData := map[string]interface{}{"price": 300.0}
		result := checker.CheckAnomaly(outlierData, "test")
		assert.NotNil(t, result) // Should not panic
	})
}

func TestAnomalyChecker_SeverityLevels(t *testing.T) {
	config := validate.AnomalyConfig{
		MADThreshold:     2.0, // Lower threshold for testing
		WindowSize:       10,
		MinDataPoints:    5,
		PriceFields:      []string{"price"},
		EnableQuarantine: true,
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Build baseline
	for i := 0; i < 10; i++ {
		data := map[string]interface{}{"price": 100.0 + float64(i)}
		checker.CheckAnomaly(data, "test")
	}
	
	tests := []struct {
		name             string
		price            float64
		expectedSeverity string
		expectQuarantine bool
	}{
		{
			name:             "low_severity",
			price:            130.0,
			expectedSeverity: "low", // MAD score ~3-4
			expectQuarantine: false,
		},
		{
			name:             "medium_severity",
			price:            150.0,
			expectedSeverity: "medium", // MAD score ~3-4
			expectQuarantine: false,
		},
		{
			name:             "high_severity", 
			price:            200.0,
			expectedSeverity: "high", // MAD score ~4-5
			expectQuarantine: false,
		},
		{
			name:             "critical_severity",
			price:            300.0,
			expectedSeverity: "critical", // MAD score >5
			expectQuarantine: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{"price": tt.price}
			result := checker.CheckAnomaly(data, "test")
			
			if result.IsAnomaly {
				assert.Equal(t, tt.expectedSeverity, result.SeverityLevel, "Severity level should match expectation")
				assert.Equal(t, tt.expectQuarantine, result.ShouldQuarantine, "Quarantine decision should match expectation")
			}
		})
	}
}

func TestAnomalyChecker_CorruptionDetection(t *testing.T) {
	config := validate.AnomalyConfig{
		EnableQuarantine: true,
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	tests := []struct {
		name        string
		data        map[string]interface{}
		description string
	}{
		{
			name: "nan_price",
			data: map[string]interface{}{
				"price": math.NaN(),
			},
			description: "NaN price should be detected as corruption",
		},
		{
			name: "positive_infinity",
			data: map[string]interface{}{
				"volume": math.Inf(1),
			},
			description: "Positive infinity should be detected as corruption",
		},
		{
			name: "negative_infinity",
			data: map[string]interface{}{
				"price": math.Inf(-1),
			},
			description: "Negative infinity should be detected as corruption",
		},
		{
			name: "zero_price",
			data: map[string]interface{}{
				"price": 0.0,
			},
			description: "Zero price should be detected as corruption",
		},
		{
			name: "negative_close_price",
			data: map[string]interface{}{
				"close": -100.0,
			},
			description: "Negative close price should be detected as corruption",
		},
		{
			name: "negative_volume",
			data: map[string]interface{}{
				"volume": -500.0,
			},
			description: "Negative volume should be detected as corruption",
		},
		{
			name: "negative_base_volume",
			data: map[string]interface{}{
				"base_volume": -1000.0,
			},
			description: "Negative base volume should be detected as corruption",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.CheckAnomaly(tt.data, "test")
			
			assert.True(t, result.IsAnomaly, tt.description)
			assert.Equal(t, validate.AnomalyTypeCorruption, result.AnomalyType)
			assert.Equal(t, "critical", result.SeverityLevel)
			assert.True(t, result.ShouldQuarantine, "Corruption should always trigger quarantine")
			assert.Contains(t, result.Reason, "corruption")
		})
	}
}

func TestAnomalyChecker_Metrics(t *testing.T) {
	config := validate.AnomalyConfig{
		MADThreshold:     2.0,
		WindowSize:       10,
		MinDataPoints:    3,
		PriceFields:      []string{"price"},
		EnableQuarantine: true,
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Initial metrics should be zero
	initialMetrics := checker.GetMetrics()
	assert.Equal(t, int64(0), initialMetrics.TotalChecks)
	assert.Equal(t, int64(0), initialMetrics.AnomaliesFound)
	assert.Equal(t, int64(0), initialMetrics.QuarantineCount)
	
	// Process normal data
	normalCount := 5
	for i := 0; i < normalCount; i++ {
		data := map[string]interface{}{"price": 100.0 + float64(i)}
		checker.CheckAnomaly(data, "test")
	}
	
	// Process anomalous data
	anomalousCount := 2
	for i := 0; i < anomalousCount; i++ {
		data := map[string]interface{}{"price": math.NaN()} // Corruption
		result := checker.CheckAnomaly(data, "test")
		assert.True(t, result.ShouldQuarantine)
	}
	
	// Check final metrics
	finalMetrics := checker.GetMetrics()
	assert.Equal(t, int64(normalCount+anomalousCount), finalMetrics.TotalChecks)
	assert.Equal(t, int64(anomalousCount), finalMetrics.AnomaliesFound)
	assert.Equal(t, int64(anomalousCount), finalMetrics.QuarantineCount)
	assert.True(t, finalMetrics.LastCheckTime.After(time.Now().Add(-time.Second)))
}

func TestAnomalyChecker_Reset(t *testing.T) {
	config := validate.AnomalyConfig{
		WindowSize:    5,
		MinDataPoints: 3,
		PriceFields:   []string{"price"},
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Process some data
	for i := 0; i < 5; i++ {
		data := map[string]interface{}{"price": 100.0 + float64(i)}
		checker.CheckAnomaly(data, "test")
	}
	
	// Verify data was processed
	metrics := checker.GetMetrics()
	assert.Greater(t, metrics.TotalChecks, int64(0))
	
	// Reset
	checker.Reset()
	
	// Verify reset worked
	resetMetrics := checker.GetMetrics()
	assert.Equal(t, int64(0), resetMetrics.TotalChecks)
	assert.Equal(t, int64(0), resetMetrics.AnomaliesFound)
	assert.Equal(t, int64(0), resetMetrics.QuarantineCount)
	
	// Verify anomaly detection still works after reset
	data := map[string]interface{}{"price": math.NaN()}
	result := checker.CheckAnomaly(data, "test")
	assert.True(t, result.IsAnomaly) // Should still detect corruption
}

func TestAnomalyChecker_Performance(t *testing.T) {
	config := validate.AnomalyConfig{
		MADThreshold:  3.0,
		WindowSize:    100,
		MinDataPoints: 20,
		PriceFields:   []string{"price"},
		VolumeFields:  []string{"volume"},
	}
	
	checker := validate.NewAnomalyChecker(config)
	
	// Build baseline
	for i := 0; i < config.MinDataPoints; i++ {
		data := map[string]interface{}{
			"price":  100.0 + float64(i),
			"volume": 1000.0 + float64(i)*10,
		}
		checker.CheckAnomaly(data, "test")
	}
	
	// Benchmark performance
	testData := map[string]interface{}{
		"price":  102.0,
		"volume": 1050.0,
	}
	
	start := time.Now()
	iterations := 10000
	
	for i := 0; i < iterations; i++ {
		result := checker.CheckAnomaly(testData, "test")
		assert.NotNil(t, result)
	}
	
	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)
	
	// Should be reasonably fast - less than 100µs per check
	assert.True(t, avgDuration < 100*time.Microsecond,
		"Anomaly detection should be fast: %v per check", avgDuration)
	
	t.Logf("Performed %d anomaly checks in %v (avg: %v per check)",
		iterations, duration, avgDuration)
}

// Helper function to generate normal data points
func generateNormalDataPoints(count int, basePrice, baseVolume float64) []map[string]interface{} {
	data := make([]map[string]interface{}, count)
	for i := 0; i < count; i++ {
		// Add small random variations
		priceVariation := (float64(i%10) - 5.0) * 0.1 // ±0.5
		volumeVariation := (float64(i%10) - 5.0) * 10  // ±50
		
		data[i] = map[string]interface{}{
			"price":  basePrice + priceVariation,
			"volume": baseVolume + volumeVariation,
		}
	}
	return data
}