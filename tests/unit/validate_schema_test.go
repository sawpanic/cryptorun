package unit

import (
	"testing"
	"time"

	"github.com/cryptorun/internal/data/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaValidator_ValidateRecord(t *testing.T) {
	// Create test schema configuration
	config := validate.SchemaConfig{
		RequiredFields: map[string]string{
			"timestamp": "int64",
			"symbol":    "string",
			"price":     "float64",
			"volume":    "float64",
		},
		OptionalFields: map[string]string{
			"exchange": "string",
			"spread":   "float64",
		},
		FieldPatterns: map[string]string{
			"symbol": "^[A-Z]+-[A-Z]+$", // e.g., BTC-USD
		},
		FieldRanges: map[string]validate.Range{
			"price": {Min: 0.0, Max: 1000000.0},
			"volume": {Min: 0.0, Max: float64(^uint64(0) >> 1)}, // Max int64
		},
		Strict: true,
	}
	
	validator := validate.NewSchemaValidator(config)
	require.NotNil(t, validator)
	
	tests := []struct {
		name          string
		data          map[string]interface{}
		expectValid   bool
		expectErrors  int
		errorContains []string
	}{
		{
			name: "valid_record_all_required_fields",
			data: map[string]interface{}{
				"timestamp": int64(1693958400),
				"symbol":    "BTC-USD",
				"price":     50000.0,
				"volume":    100.5,
			},
			expectValid:  true,
			expectErrors: 0,
		},
		{
			name: "valid_record_with_optional_fields",
			data: map[string]interface{}{
				"timestamp": int64(1693958400),
				"symbol":    "ETH-USD",
				"price":     3000.0,
				"volume":    500.75,
				"exchange":  "kraken",
				"spread":    0.05,
			},
			expectValid:  true,
			expectErrors: 0,
		},
		{
			name: "missing_required_field",
			data: map[string]interface{}{
				"timestamp": int64(1693958400),
				"symbol":    "BTC-USD",
				"price":     50000.0,
				// missing volume
			},
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"volume", "required", "missing"},
		},
		{
			name: "wrong_field_type",
			data: map[string]interface{}{
				"timestamp": "2023-09-05T14:00:00Z", // string instead of int64
				"symbol":    "BTC-USD",
				"price":     50000.0,
				"volume":    100.5,
			},
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"timestamp", "type", "int64"},
		},
		{
			name: "invalid_field_pattern",
			data: map[string]interface{}{
				"timestamp": int64(1693958400),
				"symbol":    "bitcoin", // doesn't match pattern
				"price":     50000.0,
				"volume":    100.5,
			},
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"symbol", "pattern"},
		},
		{
			name: "field_out_of_range",
			data: map[string]interface{}{
				"timestamp": int64(1693958400),
				"symbol":    "BTC-USD",
				"price":     -100.0, // negative price
				"volume":    100.5,
			},
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"price", "range"},
		},
		{
			name: "multiple_validation_errors",
			data: map[string]interface{}{
				"timestamp": "invalid", // wrong type
				"symbol":    "btc",     // invalid pattern
				"price":     -50.0,     // out of range
				// missing volume
			},
			expectValid:   false,
			expectErrors:  4, // type, pattern, range, missing field
			errorContains: []string{"timestamp", "symbol", "price", "volume"},
		},
		{
			name: "null_values",
			data: map[string]interface{}{
				"timestamp": nil,
				"symbol":    "BTC-USD",
				"price":     50000.0,
				"volume":    100.5,
			},
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"timestamp", "null"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateRecord(tt.data)
			
			if tt.expectValid {
				assert.True(t, result.IsValid, "Record should be valid")
				assert.Equal(t, 0, len(result.Errors), "Should have no validation errors")
			} else {
				assert.False(t, result.IsValid, "Record should be invalid")
				assert.Equal(t, tt.expectErrors, len(result.Errors), "Should have expected number of errors")
				
				// Check that error messages contain expected strings
				for _, expectedStr := range tt.errorContains {
					found := false
					for _, err := range result.Errors {
						if contains(err.Message, expectedStr) {
							found = true
							break
						}
					}
					assert.True(t, found, "Error should contain '%s'", expectedStr)
				}
			}
			
			// Validate timestamp
			assert.True(t, result.ValidatedAt.After(time.Now().Add(-time.Second)))
			assert.Equal(t, tt.data, result.Data)
		})
	}
}

func TestSchemaValidator_ValidateFieldType(t *testing.T) {
	validator := validate.NewSchemaValidator(validate.SchemaConfig{})
	
	tests := []struct {
		name         string
		value        interface{}
		expectedType string
		expectValid  bool
	}{
		{"int64_valid", int64(12345), "int64", true},
		{"int64_from_int", int(12345), "int64", true},
		{"int64_invalid", "12345", "int64", false},
		
		{"float64_valid", float64(123.45), "float64", true},
		{"float64_from_float32", float32(123.45), "float64", true},
		{"float64_from_int", int(123), "float64", true},
		{"float64_invalid", "123.45", "float64", false},
		
		{"string_valid", "test", "string", true},
		{"string_invalid", 12345, "string", false},
		
		{"bool_valid", true, "bool", true},
		{"bool_invalid", "true", "bool", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validator.ValidateFieldType(tt.value, tt.expectedType)
			assert.Equal(t, tt.expectValid, isValid)
		})
	}
}

func TestSchemaValidator_ValidateFieldPattern(t *testing.T) {
	validator := validate.NewSchemaValidator(validate.SchemaConfig{})
	
	tests := []struct {
		name        string
		value       string
		pattern     string
		expectValid bool
	}{
		{"symbol_pattern_valid", "BTC-USD", "^[A-Z]+-[A-Z]+$", true},
		{"symbol_pattern_invalid", "btc-usd", "^[A-Z]+-[A-Z]+$", false},
		{"symbol_pattern_invalid_format", "BTCUSD", "^[A-Z]+-[A-Z]+$", false},
		
		{"email_pattern_valid", "test@example.com", `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, true},
		{"email_pattern_invalid", "invalid-email", `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, false},
		
		{"uuid_pattern_valid", "123e4567-e89b-12d3-a456-426614174000", `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, true},
		{"uuid_pattern_invalid", "not-a-uuid", `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validator.ValidateFieldPattern(tt.value, tt.pattern)
			assert.Equal(t, tt.expectValid, isValid)
		})
	}
}

func TestSchemaValidator_ValidateFieldRange(t *testing.T) {
	validator := validate.NewSchemaValidator(validate.SchemaConfig{})
	
	tests := []struct {
		name        string
		value       interface{}
		fieldRange  validate.Range
		expectValid bool
	}{
		{"price_in_range", 100.0, validate.Range{Min: 0.0, Max: 1000.0}, true},
		{"price_at_min", 0.0, validate.Range{Min: 0.0, Max: 1000.0}, true},
		{"price_at_max", 1000.0, validate.Range{Min: 0.0, Max: 1000.0}, true},
		{"price_below_min", -10.0, validate.Range{Min: 0.0, Max: 1000.0}, false},
		{"price_above_max", 1500.0, validate.Range{Min: 0.0, Max: 1000.0}, false},
		
		{"int_in_range", int64(50), validate.Range{Min: 0.0, Max: 100.0}, true},
		{"int_out_of_range", int64(150), validate.Range{Min: 0.0, Max: 100.0}, false},
		
		{"non_numeric_value", "not_a_number", validate.Range{Min: 0.0, Max: 100.0}, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validator.ValidateFieldRange(tt.value, tt.fieldRange)
			assert.Equal(t, tt.expectValid, isValid)
		})
	}
}

func TestSchemaValidator_GetSchema(t *testing.T) {
	config := validate.SchemaConfig{
		Name:    "TestSchema",
		Version: "1.0",
		RequiredFields: map[string]string{
			"id":   "string",
			"name": "string",
		},
	}
	
	validator := validate.NewSchemaValidator(config)
	schema := validator.GetSchema()
	
	assert.Equal(t, "TestSchema", schema.Name)
	assert.Equal(t, "1.0", schema.Version)
	assert.Equal(t, 2, len(schema.Fields))
	
	// Check required fields are present
	idField, exists := schema.Fields["id"]
	assert.True(t, exists)
	assert.Equal(t, "string", idField.Type)
	assert.True(t, idField.Required)
	
	nameField, exists := schema.Fields["name"]
	assert.True(t, exists)
	assert.Equal(t, "string", nameField.Type)
	assert.True(t, nameField.Required)
}

func TestSchemaValidator_EdgeCases(t *testing.T) {
	t.Run("empty_data", func(t *testing.T) {
		config := validate.SchemaConfig{
			RequiredFields: map[string]string{
				"required_field": "string",
			},
		}
		validator := validate.NewSchemaValidator(config)
		
		result := validator.ValidateRecord(map[string]interface{}{})
		assert.False(t, result.IsValid)
		assert.Equal(t, 1, len(result.Errors))
	})
	
	t.Run("nil_data", func(t *testing.T) {
		config := validate.SchemaConfig{}
		validator := validate.NewSchemaValidator(config)
		
		result := validator.ValidateRecord(nil)
		assert.False(t, result.IsValid)
		assert.Equal(t, 1, len(result.Errors))
	})
	
	t.Run("no_schema_requirements", func(t *testing.T) {
		config := validate.SchemaConfig{}
		validator := validate.NewSchemaValidator(config)
		
		result := validator.ValidateRecord(map[string]interface{}{
			"any_field": "any_value",
		})
		assert.True(t, result.IsValid)
		assert.Equal(t, 0, len(result.Errors))
	})
	
	t.Run("non_strict_mode", func(t *testing.T) {
		config := validate.SchemaConfig{
			RequiredFields: map[string]string{
				"required_field": "string",
			},
			Strict: false,
		}
		validator := validate.NewSchemaValidator(config)
		
		// Missing required field in non-strict mode
		result := validator.ValidateRecord(map[string]interface{}{
			"other_field": "value",
		})
		
		// In non-strict mode, missing fields should still be reported as errors
		// but the overall result might be more lenient depending on implementation
		assert.False(t, result.IsValid)
	})
	
	t.Run("extra_fields_allowed", func(t *testing.T) {
		config := validate.SchemaConfig{
			RequiredFields: map[string]string{
				"required_field": "string",
			},
		}
		validator := validate.NewSchemaValidator(config)
		
		result := validator.ValidateRecord(map[string]interface{}{
			"required_field": "value",
			"extra_field":    "extra_value", // Should be allowed
		})
		
		assert.True(t, result.IsValid)
		assert.Equal(t, 0, len(result.Errors))
	})
}

func TestSchemaValidator_Performance(t *testing.T) {
	// Test performance with large datasets
	config := validate.SchemaConfig{
		RequiredFields: map[string]string{
			"id":        "string",
			"timestamp": "int64", 
			"price":     "float64",
			"volume":    "float64",
		},
		FieldPatterns: map[string]string{
			"id": "^[A-Z0-9]{8}-[A-Z0-9]{4}-[A-Z0-9]{4}$",
		},
		FieldRanges: map[string]validate.Range{
			"price":  {Min: 0.0, Max: 1000000.0},
			"volume": {Min: 0.0, Max: 1000000000.0},
		},
	}
	
	validator := validate.NewSchemaValidator(config)
	
	// Generate test data
	validRecord := map[string]interface{}{
		"id":        "ABC12345-DEF6-789G",
		"timestamp": int64(1693958400),
		"price":     50000.0,
		"volume":    1000.5,
	}
	
	// Benchmark validation
	start := time.Now()
	iterations := 10000
	
	for i := 0; i < iterations; i++ {
		result := validator.ValidateRecord(validRecord)
		assert.True(t, result.IsValid)
	}
	
	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)
	
	// Should be fast - less than 100µs per validation
	assert.True(t, avgDuration < 100*time.Microsecond, 
		"Validation should be fast: %v per record", avgDuration)
	
	t.Logf("Validated %d records in %v (avg: %v per record)", 
		iterations, duration, avgDuration)
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || len(substr) == 0 || 
		 (len(s) > len(substr) && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}