package unit

import (
	"testing"
	"time"

	"github.com/cryptorun/internal/data/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStalenessValidator_CheckStaleness(t *testing.T) {
	// Create test configuration
	config := validate.StalenessConfig{
		MaxAge: map[string]time.Duration{
			"hot":  5 * time.Second,
			"warm": 60 * time.Second,
			"cold": 5 * time.Minute,
		},
		TimestampField:  "timestamp",
		TimestampFormat: "unix",
		ClockSkewTolerance: 2 * time.Second,
	}
	
	validator := validate.NewStalenessValidator(config)
	require.NotNil(t, validator)
	
	now := time.Now()
	
	tests := []struct {
		name          string
		data          map[string]interface{}
		tier          string
		expectStale   bool
		expectWarning bool
		description   string
	}{
		{
			name: "hot_tier_fresh_data",
			data: map[string]interface{}{
				"timestamp": now.Unix(),
				"price":     100.0,
			},
			tier:        "hot",
			expectStale: false,
			description: "Current timestamp should not be stale for hot tier",
		},
		{
			name: "hot_tier_stale_data",
			data: map[string]interface{}{
				"timestamp": now.Add(-10 * time.Second).Unix(),
				"price":     100.0,
			},
			tier:        "hot",
			expectStale: true,
			description: "10 second old data should be stale for hot tier (5s limit)",
		},
		{
			name: "warm_tier_fresh_data",
			data: map[string]interface{}{
				"timestamp": now.Add(-30 * time.Second).Unix(),
				"price":     100.0,
			},
			tier:        "warm",
			expectStale: false,
			description: "30 second old data should be fresh for warm tier (60s limit)",
		},
		{
			name: "warm_tier_stale_data",
			data: map[string]interface{}{
				"timestamp": now.Add(-90 * time.Second).Unix(),
				"price":     100.0,
			},
			tier:        "warm",
			expectStale: true,
			description: "90 second old data should be stale for warm tier (60s limit)",
		},
		{
			name: "cold_tier_fresh_data",
			data: map[string]interface{}{
				"timestamp": now.Add(-3 * time.Minute).Unix(),
				"price":     100.0,
			},
			tier:        "cold",
			expectStale: false,
			description: "3 minute old data should be fresh for cold tier (5m limit)",
		},
		{
			name: "cold_tier_stale_data",
			data: map[string]interface{}{
				"timestamp": now.Add(-10 * time.Minute).Unix(),
				"price":     100.0,
			},
			tier:        "cold",
			expectStale: true,
			description: "10 minute old data should be stale for cold tier (5m limit)",
		},
		{
			name: "missing_timestamp",
			data: map[string]interface{}{
				"price": 100.0,
			},
			tier:        "hot",
			expectStale: true,
			description: "Missing timestamp should be considered stale",
		},
		{
			name: "future_timestamp_within_tolerance",
			data: map[string]interface{}{
				"timestamp": now.Add(1 * time.Second).Unix(),
				"price":     100.0,
			},
			tier:        "hot",
			expectStale: false,
			description: "Future timestamp within tolerance should be acceptable",
		},
		{
			name: "future_timestamp_beyond_tolerance",
			data: map[string]interface{}{
				"timestamp": now.Add(5 * time.Second).Unix(),
				"price":     100.0,
			},
			tier:          "hot",
			expectStale:   false,
			expectWarning: true,
			description:   "Future timestamp beyond tolerance should generate warning",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.CheckStaleness(tt.data, tt.tier)
			require.NotNil(t, result)
			
			assert.Equal(t, tt.expectStale, result.IsStale, tt.description)
			
			if tt.expectWarning {
				assert.True(t, result.HasWarning, "Should have warning for future timestamp")
				assert.Contains(t, result.Warning, "future")
			}
			
			// Validate result fields
			assert.True(t, result.CheckedAt.After(now.Add(-time.Second)))
			assert.Equal(t, tt.tier, result.Tier)
			
			if !result.IsStale {
				assert.True(t, result.Age >= 0, "Age should be non-negative for non-stale data")
			}
		})
	}
}

func TestStalenessValidator_DifferentTimestampFormats(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name            string
		timestampFormat string
		timestampValue  interface{}
		expectValid     bool
	}{
		{
			name:            "unix_timestamp_int64",
			timestampFormat: "unix",
			timestampValue:  now.Unix(),
			expectValid:     true,
		},
		{
			name:            "unix_timestamp_int",
			timestampFormat: "unix",
			timestampValue:  int(now.Unix()),
			expectValid:     true,
		},
		{
			name:            "unix_timestamp_float64",
			timestampFormat: "unix",
			timestampValue:  float64(now.Unix()),
			expectValid:     true,
		},
		{
			name:            "unix_millis",
			timestampFormat: "unix_millis",
			timestampValue:  now.UnixMilli(),
			expectValid:     true,
		},
		{
			name:            "rfc3339_string",
			timestampFormat: "rfc3339",
			timestampValue:  now.Format(time.RFC3339),
			expectValid:     true,
		},
		{
			name:            "rfc3339_nano_string",
			timestampFormat: "rfc3339nano",
			timestampValue:  now.Format(time.RFC3339Nano),
			expectValid:     true,
		},
		{
			name:            "custom_format",
			timestampFormat: "2006-01-02 15:04:05",
			timestampValue:  now.Format("2006-01-02 15:04:05"),
			expectValid:     true,
		},
		{
			name:            "invalid_string_for_unix",
			timestampFormat: "unix",
			timestampValue:  "not_a_number",
			expectValid:     false,
		},
		{
			name:            "invalid_string_for_rfc3339",
			timestampFormat: "rfc3339",
			timestampValue:  "not_a_date",
			expectValid:     false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validate.StalenessConfig{
				MaxAge: map[string]time.Duration{
					"test": 60 * time.Second,
				},
				TimestampField:  "timestamp",
				TimestampFormat: tt.timestampFormat,
			}
			
			validator := validate.NewStalenessValidator(config)
			
			data := map[string]interface{}{
				"timestamp": tt.timestampValue,
			}
			
			result := validator.CheckStaleness(data, "test")
			
			if tt.expectValid {
				// Should not be stale due to parse error for valid timestamps
				assert.NotNil(t, result.ParsedTimestamp, "Should successfully parse timestamp")
				// Age might be high but parsing should work
			} else {
				// Invalid timestamps should be treated as stale
				assert.True(t, result.IsStale, "Invalid timestamp should be considered stale")
				assert.Contains(t, result.Reason, "parse", "Reason should mention parsing issue")
			}
		})
	}
}

func TestStalenessValidator_ClockSkewTolerance(t *testing.T) {
	config := validate.StalenessConfig{
		MaxAge: map[string]time.Duration{
			"hot": 5 * time.Second,
		},
		TimestampField:     "timestamp",
		TimestampFormat:    "unix",
		ClockSkewTolerance: 3 * time.Second,
	}
	
	validator := validate.NewStalenessValidator(config)
	now := time.Now()
	
	tests := []struct {
		name        string
		timestamp   time.Time
		expectStale bool
		description string
	}{
		{
			name:        "within_tolerance_past",
			timestamp:   now.Add(-2 * time.Second),
			expectStale: false,
			description: "Data 2s old should be fresh (5s limit - 3s tolerance = 2s effective)",
		},
		{
			name:        "within_tolerance_future",
			timestamp:   now.Add(2 * time.Second),
			expectStale: false,
			description: "Data 2s in future should be acceptable with tolerance",
		},
		{
			name:        "beyond_tolerance_past",
			timestamp:   now.Add(-9 * time.Second),
			expectStale: true,
			description: "Data 9s old should be stale (beyond 5s + 3s tolerance)",
		},
		{
			name:        "beyond_tolerance_future",
			timestamp:   now.Add(4 * time.Second),
			expectStale: false, // Future data isn't stale, just generates warning
			description: "Data 4s in future should generate warning but not be stale",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{
				"timestamp": tt.timestamp.Unix(),
			}
			
			result := validator.CheckStaleness(data, "hot")
			assert.Equal(t, tt.expectStale, result.IsStale, tt.description)
		})
	}
}

func TestStalenessValidator_MultipleTimestampFields(t *testing.T) {
	config := validate.StalenessConfig{
		MaxAge: map[string]time.Duration{
			"test": 60 * time.Second,
		},
		TimestampFields: []string{"timestamp", "created_at", "received_at"},
		TimestampFormat: "unix",
	}
	
	validator := validate.NewStalenessValidator(config)
	now := time.Now()
	
	tests := []struct {
		name        string
		data        map[string]interface{}
		expectStale bool
		description string
	}{
		{
			name: "first_field_available_and_fresh",
			data: map[string]interface{}{
				"timestamp": now.Unix(),
				"price":     100.0,
			},
			expectStale: false,
			description: "Should use first available timestamp field",
		},
		{
			name: "second_field_available_and_fresh",
			data: map[string]interface{}{
				"created_at": now.Unix(),
				"price":      100.0,
			},
			expectStale: false,
			description: "Should use second timestamp field if first is missing",
		},
		{
			name: "third_field_available_but_stale",
			data: map[string]interface{}{
				"received_at": now.Add(-2 * time.Minute).Unix(),
				"price":       100.0,
			},
			expectStale: true,
			description: "Should use third field and detect staleness",
		},
		{
			name: "multiple_fields_use_freshest",
			data: map[string]interface{}{
				"timestamp":   now.Add(-2 * time.Minute).Unix(), // Stale
				"created_at":  now.Unix(),                        // Fresh
				"received_at": now.Add(-1 * time.Minute).Unix(), // Stale
				"price":       100.0,
			},
			expectStale: false,
			description: "Should use the freshest available timestamp",
		},
		{
			name: "no_timestamp_fields",
			data: map[string]interface{}{
				"price": 100.0,
			},
			expectStale: true,
			description: "Should be stale if no timestamp fields are present",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.CheckStaleness(tt.data, "test")
			assert.Equal(t, tt.expectStale, result.IsStale, tt.description)
		})
	}
}

func TestStalenessValidator_TierSpecificLimits(t *testing.T) {
	config := validate.StalenessConfig{
		MaxAge: map[string]time.Duration{
			"realtime":   1 * time.Second,
			"hot":        5 * time.Second,
			"warm":       60 * time.Second,
			"cold":       5 * time.Minute,
			"historical": 24 * time.Hour,
		},
		TimestampField:  "timestamp",
		TimestampFormat: "unix",
	}
	
	validator := validate.NewStalenessValidator(config)
	now := time.Now()
	
	// Test same data age across different tiers
	dataAge := 30 * time.Second
	data := map[string]interface{}{
		"timestamp": now.Add(-dataAge).Unix(),
		"price":     100.0,
	}
	
	tests := []struct {
		tier        string
		expectStale bool
	}{
		{"realtime", true},   // 30s > 1s limit
		{"hot", true},        // 30s > 5s limit
		{"warm", false},      // 30s < 60s limit
		{"cold", false},      // 30s < 5m limit
		{"historical", false}, // 30s < 24h limit
		{"unknown", true},    // Unknown tier should default to stale
	}
	
	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			result := validator.CheckStaleness(data, tt.tier)
			assert.Equal(t, tt.expectStale, result.IsStale, 
				"Tier %s with %v old data", tt.tier, dataAge)
		})
	}
}

func TestStalenessValidator_EdgeCases(t *testing.T) {
	config := validate.StalenessConfig{
		MaxAge: map[string]time.Duration{
			"test": 60 * time.Second,
		},
		TimestampField:  "timestamp",
		TimestampFormat: "unix",
	}
	
	validator := validate.NewStalenessValidator(config)
	
	t.Run("nil_data", func(t *testing.T) {
		result := validator.CheckStaleness(nil, "test")
		assert.True(t, result.IsStale)
		assert.Contains(t, result.Reason, "nil")
	})
	
	t.Run("empty_data", func(t *testing.T) {
		result := validator.CheckStaleness(map[string]interface{}{}, "test")
		assert.True(t, result.IsStale)
		assert.Contains(t, result.Reason, "missing")
	})
	
	t.Run("zero_timestamp", func(t *testing.T) {
		data := map[string]interface{}{
			"timestamp": int64(0),
		}
		result := validator.CheckStaleness(data, "test")
		assert.True(t, result.IsStale)
		assert.Contains(t, result.Reason, "zero")
	})
	
	t.Run("negative_timestamp", func(t *testing.T) {
		data := map[string]interface{}{
			"timestamp": int64(-1),
		}
		result := validator.CheckStaleness(data, "test")
		assert.True(t, result.IsStale)
		assert.Contains(t, result.Reason, "invalid")
	})
}

func TestStalenessValidator_Performance(t *testing.T) {
	config := validate.StalenessConfig{
		MaxAge: map[string]time.Duration{
			"hot":  5 * time.Second,
			"warm": 60 * time.Second,
			"cold": 5 * time.Minute,
		},
		TimestampField:  "timestamp",
		TimestampFormat: "unix",
	}
	
	validator := validate.NewStalenessValidator(config)
	now := time.Now()
	
	// Test data
	data := map[string]interface{}{
		"timestamp": now.Unix(),
		"price":     50000.0,
		"volume":    1000.5,
	}
	
	// Benchmark staleness checking
	start := time.Now()
	iterations := 50000
	
	for i := 0; i < iterations; i++ {
		result := validator.CheckStaleness(data, "hot")
		assert.False(t, result.IsStale)
	}
	
	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)
	
	// Should be very fast - less than 10µs per check
	assert.True(t, avgDuration < 10*time.Microsecond,
		"Staleness check should be fast: %v per record", avgDuration)
	
	t.Logf("Performed %d staleness checks in %v (avg: %v per check)",
		iterations, duration, avgDuration)
}

func TestStalenessValidator_GetConfiguration(t *testing.T) {
	config := validate.StalenessConfig{
		MaxAge: map[string]time.Duration{
			"hot": 5 * time.Second,
		},
		TimestampField:     "timestamp",
		TimestampFormat:    "unix",
		ClockSkewTolerance: 2 * time.Second,
	}
	
	validator := validate.NewStalenessValidator(config)
	retrievedConfig := validator.GetConfiguration()
	
	assert.Equal(t, config.TimestampField, retrievedConfig.TimestampField)
	assert.Equal(t, config.TimestampFormat, retrievedConfig.TimestampFormat)
	assert.Equal(t, config.ClockSkewTolerance, retrievedConfig.ClockSkewTolerance)
	assert.Equal(t, len(config.MaxAge), len(retrievedConfig.MaxAge))
	
	for tier, maxAge := range config.MaxAge {
		assert.Equal(t, maxAge, retrievedConfig.MaxAge[tier])
	}
}