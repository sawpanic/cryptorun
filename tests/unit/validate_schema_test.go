package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/sawpanic/cryptorun/internal/replication"
)

// TestValidateSchema tests schema validation functionality
func TestValidateSchema(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_complete_data",
			data: map[string]interface{}{
				"timestamp": time.Now(),
				"venue":     "kraken",
				"symbol":    "BTCUSD",
				"price":     45000.0,
				"volume":    1.5,
			},
			expectError: false,
		},
		{
			name: "missing_timestamp",
			data: map[string]interface{}{
				"venue":  "kraken",
				"symbol": "BTCUSD",
				"price":  45000.0,
			},
			expectError: true,
			errorMsg:    "missing required field: timestamp",
		},
		{
			name: "missing_venue",
			data: map[string]interface{}{
				"timestamp": time.Now(),
				"symbol":    "BTCUSD", 
				"price":     45000.0,
			},
			expectError: true,
			errorMsg:    "missing required field: venue",
		},
		{
			name: "missing_symbol",
			data: map[string]interface{}{
				"timestamp": time.Now(),
				"venue":     "kraken",
				"price":     45000.0,
			},
			expectError: true,
			errorMsg:    "missing required field: symbol",
		},
		{
			name: "empty_data",
			data: map[string]interface{}{},
			expectError: true,
			errorMsg:    "missing required field: timestamp",
		},
		{
			name: "nil_values",
			data: map[string]interface{}{
				"timestamp": nil,
				"venue":     "kraken",
				"symbol":    "BTCUSD",
			},
			expectError: true,
			errorMsg:    "missing required field: timestamp",
		},
		{
			name: "extra_fields_allowed",
			data: map[string]interface{}{
				"timestamp":   time.Now(),
				"venue":       "kraken",
				"symbol":      "BTCUSD",
				"price":       45000.0,
				"volume":      1.5,
				"extra_field": "should_be_ignored",
				"another":     123,
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := replication.ValidateSchema(test.data)
			
			if test.expectError {
				assert.Error(t, err, "Expected error for test: %s", test.name)
				if test.errorMsg != "" {
					assert.Contains(t, err.Error(), test.errorMsg, "Error message mismatch")
				}
			} else {
				assert.NoError(t, err, "Expected no error for test: %s", test.name)
			}
		})
	}
}

// TestValidateTimestamps tests timestamp validation functionality
func TestValidateTimestamps(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name        string
		data        map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_time_struct",
			data: map[string]interface{}{
				"timestamp": now,
			},
			expectError: false,
		},
		{
			name: "valid_rfc3339_string",
			data: map[string]interface{}{
				"timestamp": now.Format(time.RFC3339),
			},
			expectError: false,
		},
		{
			name: "missing_timestamp",
			data: map[string]interface{}{
				"venue": "kraken",
			},
			expectError: true,
			errorMsg:    "timestamp field is required",
		},
		{
			name: "zero_timestamp",
			data: map[string]interface{}{
				"timestamp": time.Time{},
			},
			expectError: true,
			errorMsg:    "timestamp cannot be zero",
		},
		{
			name: "future_timestamp_allowed",
			data: map[string]interface{}{
				"timestamp": now.Add(30 * time.Minute), // 30 minutes in future
			},
			expectError: false,
		},
		{
			name: "far_future_timestamp_rejected",
			data: map[string]interface{}{
				"timestamp": now.Add(2 * time.Hour), // 2 hours in future
			},
			expectError: true,
			errorMsg:    "timestamp too far in future",
		},
		{
			name: "invalid_string_format",
			data: map[string]interface{}{
				"timestamp": "2025-09-07 14:30:00", // Wrong format
			},
			expectError: true,
			errorMsg:    "invalid timestamp format",
		},
		{
			name: "invalid_timestamp_type",
			data: map[string]interface{}{
				"timestamp": 1694087400, // Unix timestamp as int
			},
			expectError: true,
			errorMsg:    "timestamp must be time.Time or RFC3339 string",
		},
		{
			name: "nil_timestamp",
			data: map[string]interface{}{
				"timestamp": nil,
			},
			expectError: true,
			errorMsg:    "timestamp must be time.Time or RFC3339 string",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := replication.ValidateTimestamps(test.data)
			
			if test.expectError {
				assert.Error(t, err, "Expected error for test: %s", test.name)
				if test.errorMsg != "" {
					assert.Contains(t, err.Error(), test.errorMsg, "Error message mismatch")
				}
			} else {
				assert.NoError(t, err, "Expected no error for test: %s", test.name)
			}
		})
	}
}

// TestValidateSequenceNumbers tests sequence number validation for hot tier
func TestValidateSequenceNumbers(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_positive_sequence",
			data: map[string]interface{}{
				"sequence": int64(12345),
			},
			expectError: false,
		},
		{
			name: "valid_zero_sequence",
			data: map[string]interface{}{
				"sequence": int64(0),
			},
			expectError: false,
		},
		{
			name: "missing_sequence_allowed",
			data: map[string]interface{}{
				"venue": "kraken",
			},
			expectError: false,
		},
		{
			name: "negative_sequence_rejected",
			data: map[string]interface{}{
				"sequence": int64(-1),
			},
			expectError: true,
			errorMsg:    "sequence number cannot be negative",
		},
		{
			name: "large_negative_sequence",
			data: map[string]interface{}{
				"sequence": int64(-999999),
			},
			expectError: true,
			errorMsg:    "sequence number cannot be negative",
		},
		{
			name: "non_int64_sequence_ignored",
			data: map[string]interface{}{
				"sequence": "12345", // String sequence ignored
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := replication.ValidateSequenceNumbers(test.data)
			
			if test.expectError {
				assert.Error(t, err, "Expected error for test: %s", test.name)
				if test.errorMsg != "" {
					assert.Contains(t, err.Error(), test.errorMsg, "Error message mismatch")
				}
			} else {
				assert.NoError(t, err, "Expected no error for test: %s", test.name)
			}
		})
	}
}

// TestValidateCompleteness tests data completeness validation for warm tier
func TestValidateCompleteness(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "complete_trading_data",
			data: map[string]interface{}{
				"price":  45000.0,
				"volume": 1.5,
				"venue":  "kraken",
			},
			expectError: false,
		},
		{
			name: "missing_price",
			data: map[string]interface{}{
				"volume": 1.5,
				"venue":  "kraken",
			},
			expectError: true,
			errorMsg:    "missing essential field for completeness: price",
		},
		{
			name: "missing_volume", 
			data: map[string]interface{}{
				"price": 45000.0,
				"venue": "kraken",
			},
			expectError: true,
			errorMsg:    "missing essential field for completeness: volume",
		},
		{
			name: "nil_price",
			data: map[string]interface{}{
				"price":  nil,
				"volume": 1.5,
			},
			expectError: true,
			errorMsg:    "missing essential field for completeness: price",
		},
		{
			name: "nil_volume",
			data: map[string]interface{}{
				"price":  45000.0,
				"volume": nil,
			},
			expectError: true,
			errorMsg:    "missing essential field for completeness: volume",
		},
		{
			name: "zero_values_allowed",
			data: map[string]interface{}{
				"price":  0.0,
				"volume": 0.0,
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := replication.ValidateCompleteness(test.data)
			
			if test.expectError {
				assert.Error(t, err, "Expected error for test: %s", test.name)
				if test.errorMsg != "" {
					assert.Contains(t, err.Error(), test.errorMsg, "Error message mismatch")
				}
			} else {
				assert.NoError(t, err, "Expected no error for test: %s", test.name)
			}
		})
	}
}

// TestValidateIntegrity tests data integrity validation for cold tier
func TestValidateIntegrity(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_checksum",
			data: map[string]interface{}{
				"checksum": "abc123def456",
				"price":    45000.0,
			},
			expectError: false,
		},
		{
			name: "missing_checksum_allowed",
			data: map[string]interface{}{
				"price":  45000.0,
				"volume": 1.5,
			},
			expectError: false,
		},
		{
			name: "valid_long_checksum",
			data: map[string]interface{}{
				"checksum": "1234567890abcdef1234567890abcdef12345678",
			},
			expectError: false,
		},
		{
			name: "short_checksum_rejected",
			data: map[string]interface{}{
				"checksum": "abc123", // Less than 8 characters
			},
			expectError: true,
			errorMsg:    "invalid checksum format",
		},
		{
			name: "empty_checksum_rejected",
			data: map[string]interface{}{
				"checksum": "",
			},
			expectError: true,
			errorMsg:    "invalid checksum format",
		},
		{
			name: "non_string_checksum_ignored",
			data: map[string]interface{}{
				"checksum": 12345678, // Non-string checksum ignored
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := replication.ValidateIntegrity(test.data)
			
			if test.expectError {
				assert.Error(t, err, "Expected error for test: %s", test.name)
				if test.errorMsg != "" {
					assert.Contains(t, err.Error(), test.errorMsg, "Error message mismatch")
				}
			} else {
				assert.NoError(t, err, "Expected no error for test: %s", test.name)
			}
		})
	}
}

// TestValidatePartitioning tests partitioning constraints for cold tier
func TestValidatePartitioning(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "complete_partition_keys",
			data: map[string]interface{}{
				"venue":  "kraken",
				"symbol": "BTCUSD",
				"date":   "2025-09-07",
				"price":  45000.0,
			},
			expectError: false,
		},
		{
			name: "missing_venue",
			data: map[string]interface{}{
				"symbol": "BTCUSD",
				"date":   "2025-09-07",
			},
			expectError: true,
			errorMsg:    "missing partition key: venue",
		},
		{
			name: "missing_symbol",
			data: map[string]interface{}{
				"venue": "kraken",
				"date":  "2025-09-07",
			},
			expectError: true,
			errorMsg:    "missing partition key: symbol",
		},
		{
			name: "missing_date",
			data: map[string]interface{}{
				"venue":  "kraken", 
				"symbol": "BTCUSD",
			},
			expectError: true,
			errorMsg:    "missing partition key: date",
		},
		{
			name: "extra_fields_allowed",
			data: map[string]interface{}{
				"venue":      "kraken",
				"symbol":     "BTCUSD",
				"date":       "2025-09-07",
				"price":      45000.0,
				"volume":     1.5,
				"extra_data": "ignored",
			},
			expectError: false,
		},
		{
			name: "all_missing",
			data: map[string]interface{}{
				"price": 45000.0,
			},
			expectError: true,
			errorMsg:    "missing partition key: venue",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := replication.ValidatePartitioning(test.data)
			
			if test.expectError {
				assert.Error(t, err, "Expected error for test: %s", test.name)
				if test.errorMsg != "" {
					assert.Contains(t, err.Error(), test.errorMsg, "Error message mismatch")
				}
			} else {
				assert.NoError(t, err, "Expected no error for test: %s", test.name)
			}
		})
	}
}

// TestValidationChain tests chaining multiple validators together
func TestValidationChain(t *testing.T) {
	now := time.Now()
	
	testData := map[string]interface{}{
		"timestamp": now,
		"venue":     "kraken",
		"symbol":    "BTCUSD",
		"price":     45000.0,
		"volume":    1.5,
		"sequence":  int64(12345),
		"checksum":  "abc123def456",
		"date":      now.Format("2006-01-02"),
	}
	
	// Chain of validators (simulates what planner would do)
	validators := []replication.ValidateFn{
		replication.ValidateSchema,
		replication.ValidateTimestamps,
		replication.ValidateSequenceNumbers,
		replication.ValidateCompleteness,
		replication.ValidateIntegrity,
		replication.ValidatePartitioning,
	}
	
	t.Run("all_validators_pass", func(t *testing.T) {
		for i, validator := range validators {
			err := validator(testData)
			assert.NoError(t, err, "Validator %d should pass", i)
		}
	})
	
	t.Run("early_validator_fails", func(t *testing.T) {
		// Remove required field to make schema validation fail
		badData := make(map[string]interface{})
		for k, v := range testData {
			badData[k] = v
		}
		delete(badData, "timestamp")
		
		// First validator (schema) should fail
		err := validators[0](badData)
		assert.Error(t, err, "Schema validator should fail")
		assert.Contains(t, err.Error(), "missing required field: timestamp")
		
		// But later validators would still work if called independently
		err = validators[2](badData) // sequence validator
		assert.NoError(t, err, "Sequence validator should pass independently")
	})
}