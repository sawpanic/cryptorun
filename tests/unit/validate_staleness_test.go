package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/sawpanic/cryptorun/internal/replication"
)

// TestValidateFreshness tests freshness validation functionality
func TestValidateFreshness(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name        string
		maxAge      time.Duration
		timestamp   interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "fresh_data_time_struct",
			maxAge:      5 * time.Minute,
			timestamp:   now.Add(-1 * time.Minute), // 1 minute ago
			expectError: false,
		},
		{
			name:        "fresh_data_rfc3339",
			maxAge:      5 * time.Minute,
			timestamp:   now.Add(-2 * time.Minute).Format(time.RFC3339), // 2 minutes ago
			expectError: false,
		},
		{
			name:        "exactly_at_limit",
			maxAge:      5 * time.Minute,
			timestamp:   now.Add(-5 * time.Minute), // Exactly 5 minutes ago
			expectError: false, // Should be exactly at limit
		},
		{
			name:        "slightly_stale",
			maxAge:      5 * time.Minute,
			timestamp:   now.Add(-5*time.Minute - 1*time.Second), // 5 minutes 1 second ago
			expectError: true,
			errorMsg:    "data is too old",
		},
		{
			name:        "very_stale",
			maxAge:      1 * time.Minute,
			timestamp:   now.Add(-10 * time.Minute), // 10 minutes ago
			expectError: true,
			errorMsg:    "data is too old",
		},
		{
			name:        "future_data_allowed",
			maxAge:      5 * time.Minute,
			timestamp:   now.Add(1 * time.Minute), // 1 minute in future
			expectError: false,
		},
		{
			name:        "missing_timestamp",
			maxAge:      5 * time.Minute,
			timestamp:   nil,
			expectError: true,
			errorMsg:    "timestamp required for freshness check",
		},
		{
			name:        "invalid_timestamp_format",
			maxAge:      5 * time.Minute,
			timestamp:   "2025-09-07 14:30:00", // Wrong format
			expectError: true,
			errorMsg:    "invalid timestamp for freshness check",
		},
		{
			name:        "invalid_timestamp_type",
			maxAge:      5 * time.Minute,
			timestamp:   1694087400, // Unix timestamp as int
			expectError: true,
			errorMsg:    "timestamp must be time.Time or RFC3339 string",
		},
		{
			name:        "hot_tier_strict",
			maxAge:      5 * time.Second, // Hot tier: 5 second limit
			timestamp:   now.Add(-3 * time.Second),
			expectError: false,
		},
		{
			name:        "hot_tier_stale",
			maxAge:      5 * time.Second,
			timestamp:   now.Add(-10 * time.Second),
			expectError: true,
			errorMsg:    "data is too old",
		},
		{
			name:        "warm_tier_normal",
			maxAge:      60 * time.Second, // Warm tier: 60 second limit
			timestamp:   now.Add(-45 * time.Second),
			expectError: false,
		},
		{
			name:        "warm_tier_stale",
			maxAge:      60 * time.Second,
			timestamp:   now.Add(-90 * time.Second),
			expectError: true,
			errorMsg:    "data is too old",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create freshness validator with the specified max age
			validator := replication.ValidateFreshness(test.maxAge)
			
			// Prepare test data
			var data map[string]interface{}
			if test.timestamp != nil {
				data = map[string]interface{}{
					"timestamp": test.timestamp,
				}
			} else {
				data = map[string]interface{}{} // No timestamp
			}
			
			// Execute validation
			err := validator(data)
			
			if test.expectError {
				assert.Error(t, err, "Expected error for test: %s", test.name)
				if test.errorMsg != "" {
					assert.Contains(t, err.Error(), test.errorMsg, "Error message mismatch for %s", test.name)
				}
			} else {
				assert.NoError(t, err, "Expected no error for test: %s", test.name)
			}
		})
	}
}

// TestFreshnessValidatorCreation tests validator creation with different parameters
func TestFreshnessValidatorCreation(t *testing.T) {
	tests := []struct {
		name         string
		maxAge       time.Duration
		expectedType string
	}{
		{
			name:         "hot_tier_validator",
			maxAge:       5 * time.Second,
			expectedType: "hot",
		},
		{
			name:         "warm_tier_validator", 
			maxAge:       60 * time.Second,
			expectedType: "warm",
		},
		{
			name:         "cold_tier_validator",
			maxAge:       5 * time.Minute,
			expectedType: "cold",
		},
		{
			name:         "custom_validator",
			maxAge:       30 * time.Second,
			expectedType: "custom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := replication.ValidateFreshness(test.maxAge)
			assert.NotNil(t, validator, "Validator should not be nil")
			
			// Test with fresh data - should pass
			freshData := map[string]interface{}{
				"timestamp": time.Now(),
			}
			err := validator(freshData)
			assert.NoError(t, err, "Fresh data should pass validation")
			
			// Test with stale data - should fail
			staleData := map[string]interface{}{
				"timestamp": time.Now().Add(-test.maxAge - time.Second),
			}
			err = validator(staleData)
			assert.Error(t, err, "Stale data should fail validation")
		})
	}
}

// TestStalenessEdgeCases tests edge cases for staleness detection
func TestStalenessEdgeCases(t *testing.T) {
	now := time.Now()
	
	// Test various time precision scenarios
	t.Run("microsecond_precision", func(t *testing.T) {
		validator := replication.ValidateFreshness(1 * time.Second)
		
		// Just under the limit (should pass)
		timestamp := now.Add(-999 * time.Millisecond)
		data := map[string]interface{}{"timestamp": timestamp}
		err := validator(data)
		assert.NoError(t, err, "Just under limit should pass")
		
		// Just over the limit (should fail)
		timestamp = now.Add(-1001 * time.Millisecond)
		data = map[string]interface{}{"timestamp": timestamp}
		err = validator(data)
		assert.Error(t, err, "Just over limit should fail")
	})
	
	t.Run("zero_max_age", func(t *testing.T) {
		validator := replication.ValidateFreshness(0)
		
		// Even very recent data should fail with zero tolerance
		timestamp := now.Add(-1 * time.Nanosecond)
		data := map[string]interface{}{"timestamp": timestamp}
		err := validator(data)
		assert.Error(t, err, "Zero tolerance should reject any old data")
	})
	
	t.Run("negative_max_age", func(t *testing.T) {
		validator := replication.ValidateFreshness(-5 * time.Second)
		
		// Negative max age should reject all past data
		timestamp := now.Add(-1 * time.Millisecond)
		data := map[string]interface{}{"timestamp": timestamp}
		err := validator(data)
		assert.Error(t, err, "Negative max age should reject past data")
		
		// But future data might pass
		timestamp = now.Add(1 * time.Second)
		data = map[string]interface{}{"timestamp": timestamp}
		err = validator(data)
		assert.Error(t, err, "Should still fail due to negative comparison")
	})
	
	t.Run("very_large_max_age", func(t *testing.T) {
		validator := replication.ValidateFreshness(24 * time.Hour)
		
		// Very old data should pass with large tolerance
		timestamp := now.Add(-12 * time.Hour)
		data := map[string]interface{}{"timestamp": timestamp}
		err := validator(data)
		assert.NoError(t, err, "Old data should pass with large tolerance")
		
		// But beyond the limit should fail
		timestamp = now.Add(-25 * time.Hour)
		data = map[string]interface{}{"timestamp": timestamp}
		err = validator(data)
		assert.Error(t, err, "Data beyond large tolerance should fail")
	})
}

// TestMultipleTimestampFormats tests various timestamp formats
func TestMultipleTimestampFormats(t *testing.T) {
	now := time.Now()
	validator := replication.ValidateFreshness(5 * time.Minute)
	
	tests := []struct {
		name        string
		timestamp   interface{}
		expectError bool
	}{
		{
			name:        "time_struct",
			timestamp:   now.Add(-1 * time.Minute),
			expectError: false,
		},
		{
			name:        "rfc3339_string",
			timestamp:   now.Add(-1 * time.Minute).Format(time.RFC3339),
			expectError: false,
		},
		{
			name:        "rfc3339_nano_string",
			timestamp:   now.Add(-1 * time.Minute).Format(time.RFC3339Nano),
			expectError: false,
		},
		{
			name:        "iso8601_invalid",
			timestamp:   now.Add(-1 * time.Minute).Format("2006-01-02T15:04:05Z07:00"),
			expectError: false, // Should be valid RFC3339
		},
		{
			name:        "custom_format_invalid",
			timestamp:   now.Add(-1 * time.Minute).Format("2006-01-02 15:04:05"),
			expectError: true,
		},
		{
			name:        "unix_timestamp_invalid",
			timestamp:   now.Add(-1 * time.Minute).Unix(),
			expectError: true,
		},
		{
			name:        "unix_nano_timestamp_invalid",
			timestamp:   now.Add(-1 * time.Minute).UnixNano(),
			expectError: true,
		},
		{
			name:        "float_timestamp_invalid",
			timestamp:   float64(now.Add(-1 * time.Minute).Unix()),
			expectError: true,
		},
		{
			name:        "bool_timestamp_invalid",
			timestamp:   true,
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := map[string]interface{}{
				"timestamp": test.timestamp,
			}
			
			err := validator(data)
			
			if test.expectError {
				assert.Error(t, err, "Expected error for timestamp format: %s", test.name)
			} else {
				assert.NoError(t, err, "Expected no error for timestamp format: %s", test.name)
			}
		})
	}
}

// TestTierSpecificStalenessLimits tests realistic tier-specific staleness limits
func TestTierSpecificStalenessLimits(t *testing.T) {
	now := time.Now()
	
	tierLimits := map[string]time.Duration{
		"hot":  5 * time.Second,   // Hot tier: very strict
		"warm": 60 * time.Second,  // Warm tier: moderate
		"cold": 5 * time.Minute,   // Cold tier: relaxed
	}
	
	for tier, limit := range tierLimits {
		t.Run(tier+"_tier", func(t *testing.T) {
			validator := replication.ValidateFreshness(limit)
			
			// Test data within limit (should pass)
			withinLimit := limit / 2
			data := map[string]interface{}{
				"timestamp": now.Add(-withinLimit),
			}
			err := validator(data)
			assert.NoError(t, err, "%s tier: data within limit should pass", tier)
			
			// Test data at limit (should pass)
			data = map[string]interface{}{
				"timestamp": now.Add(-limit),
			}
			err = validator(data)
			assert.NoError(t, err, "%s tier: data at limit should pass", tier)
			
			// Test data beyond limit (should fail)
			beyondLimit := limit + time.Second
			data = map[string]interface{}{
				"timestamp": now.Add(-beyondLimit),
			}
			err = validator(data)
			assert.Error(t, err, "%s tier: data beyond limit should fail", tier)
			assert.Contains(t, err.Error(), "data is too old", "%s tier error message", tier)
		})
	}
}

// TestConcurrentFreshnessValidation tests validator under concurrent access
func TestConcurrentFreshnessValidation(t *testing.T) {
	validator := replication.ValidateFreshness(10 * time.Second)
	now := time.Now()
	
	// Run multiple validations concurrently
	concurrency := 100
	errors := make(chan error, concurrency)
	
	for i := 0; i < concurrency; i++ {
		go func(index int) {
			data := map[string]interface{}{
				"timestamp": now.Add(-time.Duration(index) * time.Second),
			}
			
			err := validator(data)
			errors <- err
		}(i)
	}
	
	// Collect results
	successCount := 0
	failureCount := 0
	
	for i := 0; i < concurrency; i++ {
		err := <-errors
		if err == nil {
			successCount++
		} else {
			failureCount++
		}
	}
	
	// Validate results
	assert.Greater(t, successCount, 0, "Should have some successful validations")
	assert.Greater(t, failureCount, 0, "Should have some failed validations (timestamps > 10s old)")
	assert.Equal(t, concurrency, successCount+failureCount, "All validations should complete")
	
	t.Logf("Concurrent validation results: %d successes, %d failures", successCount, failureCount)
}