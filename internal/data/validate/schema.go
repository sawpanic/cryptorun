package validate

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// Schema defines the structure and validation rules for data
type Schema struct {
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	Fields  map[string]FieldSchema `json:"fields"`
}

// FieldSchema defines validation rules for a single field
type FieldSchema struct {
	Type        FieldType   `json:"type"`
	Required    bool        `json:"required"`
	Pattern     string      `json:"pattern,omitempty"`     // Regex pattern for string validation
	MinValue    *float64    `json:"min_value,omitempty"`   // Minimum value for numeric fields
	MaxValue    *float64    `json:"max_value,omitempty"`   // Maximum value for numeric fields
	MinLength   *int        `json:"min_length,omitempty"`  // Minimum length for strings/arrays
	MaxLength   *int        `json:"max_length,omitempty"`  // Maximum length for strings/arrays
	Enum        []interface{} `json:"enum,omitempty"`      // Allowed values
	Format      string      `json:"format,omitempty"`      // Format specification (e.g., "rfc3339", "email")
	Description string      `json:"description,omitempty"` // Field description
}

// FieldType represents the expected data type of a field
type FieldType string

const (
	FieldTypeString    FieldType = "string"
	FieldTypeInteger   FieldType = "integer"
	FieldTypeFloat     FieldType = "float"
	FieldTypeBoolean   FieldType = "boolean"
	FieldTypeTimestamp FieldType = "timestamp"
	FieldTypeArray     FieldType = "array"
	FieldTypeObject    FieldType = "object"
)

// SchemaValidator validates data against schemas
type SchemaValidator struct {
	schemas map[string]*Schema
	cache   map[string]*regexp.Regexp // Cache compiled regex patterns
}

// ValidationError represents a schema validation error
type ValidationError struct {
	Field   string `json:"field"`
	Value   interface{} `json:"value"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// Error implements the error interface
func (e ValidationError) Error() string {
	return fmt.Sprintf("field '%s' validation failed: %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult holds the result of schema validation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors"`
	Schema string            `json:"schema"`
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{
		schemas: make(map[string]*Schema),
		cache:   make(map[string]*regexp.Regexp),
	}
}

// RegisterSchema registers a schema for validation
func (sv *SchemaValidator) RegisterSchema(schema *Schema) error {
	if schema.Name == "" {
		return fmt.Errorf("schema name cannot be empty")
	}
	
	if schema.Version == "" {
		return fmt.Errorf("schema version cannot be empty")
	}
	
	// Validate schema fields
	if err := sv.validateSchema(schema); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}
	
	sv.schemas[schema.Name] = schema
	return nil
}

// ValidateData validates data against a registered schema
func (sv *SchemaValidator) ValidateData(schemaName string, data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
		Schema: schemaName,
	}
	
	schema, exists := sv.schemas[schemaName]
	if !exists {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "",
			Value:   nil,
			Rule:    "schema_not_found",
			Message: fmt.Sprintf("schema '%s' not found", schemaName),
		})
		return result
	}
	
	// Validate each field in the schema
	for fieldName, fieldSchema := range schema.Fields {
		value, exists := data[fieldName]
		
		// Check required fields
		if fieldSchema.Required && !exists {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fieldName,
				Value:   nil,
				Rule:    "required",
				Message: "field is required but missing",
			})
			continue
		}
		
		// Skip validation if field is not present and not required
		if !exists {
			continue
		}
		
		// Validate field value
		if fieldErr := sv.validateField(fieldName, value, fieldSchema); fieldErr != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *fieldErr)
		}
	}
	
	return result
}

// validateField validates a single field against its schema
func (sv *SchemaValidator) validateField(fieldName string, value interface{}, schema FieldSchema) *ValidationError {
	// Type validation
	if err := sv.validateFieldType(fieldName, value, schema.Type); err != nil {
		return err
	}
	
	// Pattern validation for strings
	if schema.Pattern != "" && schema.Type == FieldTypeString {
		if err := sv.validatePattern(fieldName, value, schema.Pattern); err != nil {
			return err
		}
	}
	
	// Numeric range validation
	if schema.MinValue != nil || schema.MaxValue != nil {
		if err := sv.validateNumericRange(fieldName, value, schema.MinValue, schema.MaxValue); err != nil {
			return err
		}
	}
	
	// Length validation
	if schema.MinLength != nil || schema.MaxLength != nil {
		if err := sv.validateLength(fieldName, value, schema.MinLength, schema.MaxLength); err != nil {
			return err
		}
	}
	
	// Enum validation
	if len(schema.Enum) > 0 {
		if err := sv.validateEnum(fieldName, value, schema.Enum); err != nil {
			return err
		}
	}
	
	// Format validation
	if schema.Format != "" {
		if err := sv.validateFormat(fieldName, value, schema.Format); err != nil {
			return err
		}
	}
	
	return nil
}

// validateFieldType validates the type of a field value
func (sv *SchemaValidator) validateFieldType(fieldName string, value interface{}, expectedType FieldType) *ValidationError {
	switch expectedType {
	case FieldTypeString:
		if _, ok := value.(string); !ok {
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "type",
				Message: "expected string type",
			}
		}
		
	case FieldTypeInteger:
		switch v := value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			// All integer types are valid
		case float64:
			// Allow float64 if it represents a whole number (JSON unmarshaling)
			if v != float64(int64(v)) {
				return &ValidationError{
					Field:   fieldName,
					Value:   value,
					Rule:    "type",
					Message: "expected integer, got float with decimal part",
				}
			}
		default:
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "type",
				Message: "expected integer type",
			}
		}
		
	case FieldTypeFloat:
		switch value.(type) {
		case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			// All numeric types can be treated as float
		default:
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "type",
				Message: "expected numeric type",
			}
		}
		
	case FieldTypeBoolean:
		if _, ok := value.(bool); !ok {
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "type",
				Message: "expected boolean type",
			}
		}
		
	case FieldTypeTimestamp:
		switch v := value.(type) {
		case time.Time:
			// Already a time.Time
		case string:
			// Try to parse as RFC3339
			if _, err := time.Parse(time.RFC3339, v); err != nil {
				return &ValidationError{
					Field:   fieldName,
					Value:   value,
					Rule:    "type",
					Message: "expected valid RFC3339 timestamp",
				}
			}
		case int64:
			// Unix timestamp - validate it's reasonable
			if v < 0 || v > 4102444800 { // Year 2100
				return &ValidationError{
					Field:   fieldName,
					Value:   value,
					Rule:    "type",
					Message: "unix timestamp out of reasonable range",
				}
			}
		default:
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "type",
				Message: "expected timestamp (time.Time, RFC3339 string, or unix timestamp)",
			}
		}
		
	case FieldTypeArray:
		rv := reflect.ValueOf(value)
		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "type",
				Message: "expected array/slice type",
			}
		}
		
	case FieldTypeObject:
		if _, ok := value.(map[string]interface{}); !ok {
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "type",
				Message: "expected object/map type",
			}
		}
	}
	
	return nil
}

// validatePattern validates a string value against a regex pattern
func (sv *SchemaValidator) validatePattern(fieldName string, value interface{}, pattern string) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "pattern",
			Message: "cannot validate pattern on non-string value",
		}
	}
	
	// Get compiled regex from cache or compile it
	regex, exists := sv.cache[pattern]
	if !exists {
		var err error
		regex, err = regexp.Compile(pattern)
		if err != nil {
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "pattern",
				Message: fmt.Sprintf("invalid regex pattern: %v", err),
			}
		}
		sv.cache[pattern] = regex
	}
	
	if !regex.MatchString(str) {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "pattern",
			Message: fmt.Sprintf("value does not match pattern '%s'", pattern),
		}
	}
	
	return nil
}

// validateNumericRange validates numeric values are within specified range
func (sv *SchemaValidator) validateNumericRange(fieldName string, value interface{}, min, max *float64) *ValidationError {
	var numVal float64
	var ok bool
	
	switch v := value.(type) {
	case float64:
		numVal = v
		ok = true
	case float32:
		numVal = float64(v)
		ok = true
	case int:
		numVal = float64(v)
		ok = true
	case int64:
		numVal = float64(v)
		ok = true
	case int32:
		numVal = float64(v)
		ok = true
	case uint:
		numVal = float64(v)
		ok = true
	case uint64:
		numVal = float64(v)
		ok = true
	case uint32:
		numVal = float64(v)
		ok = true
	}
	
	if !ok {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "range",
			Message: "cannot validate range on non-numeric value",
		}
	}
	
	if min != nil && numVal < *min {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "min_value",
			Message: fmt.Sprintf("value %v is less than minimum %v", numVal, *min),
		}
	}
	
	if max != nil && numVal > *max {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "max_value",
			Message: fmt.Sprintf("value %v is greater than maximum %v", numVal, *max),
		}
	}
	
	return nil
}

// validateLength validates the length of strings or arrays
func (sv *SchemaValidator) validateLength(fieldName string, value interface{}, minLen, maxLen *int) *ValidationError {
	var length int
	
	switch v := value.(type) {
	case string:
		length = len(v)
	case []interface{}:
		length = len(v)
	default:
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			length = rv.Len()
		} else {
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "length",
				Message: "cannot validate length on this type",
			}
		}
	}
	
	if minLen != nil && length < *minLen {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "min_length",
			Message: fmt.Sprintf("length %d is less than minimum %d", length, *minLen),
		}
	}
	
	if maxLen != nil && length > *maxLen {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "max_length",
			Message: fmt.Sprintf("length %d is greater than maximum %d", length, *maxLen),
		}
	}
	
	return nil
}

// validateEnum validates that value is in the allowed enum values
func (sv *SchemaValidator) validateEnum(fieldName string, value interface{}, enum []interface{}) *ValidationError {
	for _, allowed := range enum {
		if reflect.DeepEqual(value, allowed) {
			return nil
		}
	}
	
	return &ValidationError{
		Field:   fieldName,
		Value:   value,
		Rule:    "enum",
		Message: fmt.Sprintf("value not in allowed enum values %v", enum),
	}
}

// validateFormat validates specific formats
func (sv *SchemaValidator) validateFormat(fieldName string, value interface{}, format string) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "format",
			Message: "format validation only applies to strings",
		}
	}
	
	switch format {
	case "rfc3339":
		if _, err := time.Parse(time.RFC3339, str); err != nil {
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "format",
				Message: "invalid RFC3339 format",
			}
		}
		
	case "email":
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(str) {
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "format",
				Message: "invalid email format",
			}
		}
		
	case "uuid":
		uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
		if !uuidRegex.MatchString(strings.ToLower(str)) {
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "format",
				Message: "invalid UUID format",
			}
		}
		
	case "symbol":
		// Crypto symbol format validation
		symbolRegex := regexp.MustCompile(`^[A-Z]{3,10}(USD|USDT|USDC|BTC|ETH)$`)
		if !symbolRegex.MatchString(str) {
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "format",
				Message: "invalid crypto symbol format",
			}
		}
		
	case "venue":
		// Venue name validation
		allowedVenues := []string{"binance", "kraken", "okx", "coinbase"}
		validVenue := false
		for _, venue := range allowedVenues {
			if strings.ToLower(str) == venue {
				validVenue = true
				break
			}
		}
		if !validVenue {
			return &ValidationError{
				Field:   fieldName,
				Value:   value,
				Rule:    "format",
				Message: fmt.Sprintf("venue must be one of: %s", strings.Join(allowedVenues, ", ")),
			}
		}
		
	default:
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "format",
			Message: fmt.Sprintf("unknown format: %s", format),
		}
	}
	
	return nil
}

// validateSchema validates that a schema definition is valid
func (sv *SchemaValidator) validateSchema(schema *Schema) error {
	if len(schema.Fields) == 0 {
		return fmt.Errorf("schema must have at least one field")
	}
	
	for fieldName, fieldSchema := range schema.Fields {
		if fieldName == "" {
			return fmt.Errorf("field name cannot be empty")
		}
		
		// Validate field type
		validTypes := []FieldType{
			FieldTypeString, FieldTypeInteger, FieldTypeFloat,
			FieldTypeBoolean, FieldTypeTimestamp, FieldTypeArray, FieldTypeObject,
		}
		
		validType := false
		for _, validT := range validTypes {
			if fieldSchema.Type == validT {
				validType = true
				break
			}
		}
		
		if !validType {
			return fmt.Errorf("invalid field type '%s' for field '%s'", fieldSchema.Type, fieldName)
		}
		
		// Validate regex pattern if provided
		if fieldSchema.Pattern != "" {
			if _, err := regexp.Compile(fieldSchema.Pattern); err != nil {
				return fmt.Errorf("invalid regex pattern for field '%s': %v", fieldName, err)
			}
		}
		
		// Validate numeric ranges
		if fieldSchema.MinValue != nil && fieldSchema.MaxValue != nil {
			if *fieldSchema.MinValue > *fieldSchema.MaxValue {
				return fmt.Errorf("min_value cannot be greater than max_value for field '%s'", fieldName)
			}
		}
		
		// Validate length constraints
		if fieldSchema.MinLength != nil && fieldSchema.MaxLength != nil {
			if *fieldSchema.MinLength > *fieldSchema.MaxLength {
				return fmt.Errorf("min_length cannot be greater than max_length for field '%s'", fieldName)
			}
		}
		
		if fieldSchema.MinLength != nil && *fieldSchema.MinLength < 0 {
			return fmt.Errorf("min_length cannot be negative for field '%s'", fieldName)
		}
	}
	
	return nil
}

// GetRegisteredSchemas returns a list of all registered schema names
func (sv *SchemaValidator) GetRegisteredSchemas() []string {
	var names []string
	for name := range sv.schemas {
		names = append(names, name)
	}
	return names
}

// GetSchema returns a copy of a registered schema
func (sv *SchemaValidator) GetSchema(name string) (*Schema, error) {
	schema, exists := sv.schemas[name]
	if !exists {
		return nil, fmt.Errorf("schema '%s' not found", name)
	}
	
	// Return a deep copy to prevent external modification
	schemaCopy := *schema
	schemaCopy.Fields = make(map[string]FieldSchema)
	for k, v := range schema.Fields {
		schemaCopy.Fields[k] = v
	}
	
	return &schemaCopy, nil
}

// CreateCryptoDataSchema creates a standard schema for cryptocurrency data
func CreateCryptoDataSchema() *Schema {
	return &Schema{
		Name:    "crypto_data",
		Version: "1.0.0",
		Fields: map[string]FieldSchema{
			"timestamp": {
				Type:        FieldTypeTimestamp,
				Required:    true,
				Format:      "rfc3339",
				Description: "Data timestamp in RFC3339 format",
			},
			"venue": {
				Type:        FieldTypeString,
				Required:    true,
				Format:      "venue",
				Description: "Exchange venue name",
			},
			"symbol": {
				Type:        FieldTypeString,
				Required:    true,
				Format:      "symbol",
				Description: "Trading pair symbol",
			},
			"tier": {
				Type:        FieldTypeString,
				Required:    true,
				Enum:        []interface{}{"hot", "warm", "cold"},
				Description: "Data tier classification",
			},
			"price": {
				Type:        FieldTypeFloat,
				Required:    false,
				MinValue:    &[]float64{0.0}[0],
				Description: "Price value",
			},
			"volume": {
				Type:        FieldTypeFloat,
				Required:    false,
				MinValue:    &[]float64{0.0}[0],
				Description: "Volume value",
			},
			"sequence": {
				Type:        FieldTypeInteger,
				Required:    false,
				MinValue:    &[]float64{0.0}[0],
				Description: "Sequence number for ordering",
			},
		},
	}
}