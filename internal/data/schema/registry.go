package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DataEnvelope represents the canonical schema for all data storage formats
type DataEnvelope struct {
	// Core fields
	Timestamp   time.Time `json:"timestamp" parquet:",timestamp(MILLIS)"`
	Venue       string    `json:"venue" parquet:",utf8"`
	Symbol      string    `json:"symbol" parquet:",utf8"`
	Tier        string    `json:"tier" parquet:",utf8"`
	
	// Provenance tracking
	OriginalSource    string  `json:"original_source" parquet:",utf8"`
	ConfidenceScore   float64 `json:"confidence_score" parquet:","`
	ProcessingDelay   int64   `json:"processing_delay_ms" parquet:","`
	
	// Market data
	BestBidPrice  float64 `json:"best_bid_price" parquet:","`
	BestAskPrice  float64 `json:"best_ask_price" parquet:","`
	BestBidQty    float64 `json:"best_bid_qty" parquet:","`
	BestAskQty    float64 `json:"best_ask_qty" parquet:","`
	MidPrice      float64 `json:"mid_price" parquet:","`
	SpreadBps     float64 `json:"spread_bps" parquet:","`
	
	// Point-in-time integrity
	RowCount      int64  `json:"row_count" parquet:","`
	MinTimestamp  int64  `json:"min_timestamp_unix" parquet:","`
	MaxTimestamp  int64  `json:"max_timestamp_unix" parquet:","`
	SchemaVersion string `json:"schema_version" parquet:",utf8"`
}

// SchemaRegistry manages data schemas and validation
type SchemaRegistry struct {
	schemaPath string
	schemas    map[string]*SchemaDefinition
}

// SchemaDefinition holds schema metadata and validation rules
type SchemaDefinition struct {
	Version     string                 `json:"version"`
	Entity      string                 `json:"entity"`
	Fields      []FieldDefinition      `json:"fields"`
	Required    []string               `json:"required"`
	Constraints map[string]interface{} `json:"constraints"`
	CreatedAt   time.Time              `json:"created_at"`
}

// FieldDefinition describes individual schema fields
type FieldDefinition struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Optional bool        `json:"optional"`
	Default  interface{} `json:"default,omitempty"`
}

// ValidationMode controls schema validation behavior
type ValidationMode int

const (
	ValidationStrict ValidationMode = iota // Fail on unknown fields
	ValidationWarn                         // Warn on unknown fields
	ValidationIgnore                       // Ignore unknown fields
)

// NewSchemaRegistry creates a new schema registry
func NewSchemaRegistry(schemaPath string) *SchemaRegistry {
	return &SchemaRegistry{
		schemaPath: schemaPath,
		schemas:    make(map[string]*SchemaDefinition),
	}
}

// LoadSchemas loads all schema definitions from the schema directory
func (r *SchemaRegistry) LoadSchemas() error {
	if _, err := os.Stat(r.schemaPath); os.IsNotExist(err) {
		if err := os.MkdirAll(r.schemaPath, 0755); err != nil {
			return fmt.Errorf("failed to create schema directory: %w", err)
		}
	}
	
	return filepath.Walk(r.schemaPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && (filepath.Ext(path) == ".json" || filepath.Ext(path) == ".yaml") {
			schema, err := r.loadSchemaFile(path)
			if err != nil {
				return fmt.Errorf("failed to load schema %s: %w", path, err)
			}
			
			key := fmt.Sprintf("%s:%s", schema.Entity, schema.Version)
			r.schemas[key] = schema
		}
		
		return nil
	})
}

// loadSchemaFile loads a single schema file
func (r *SchemaRegistry) loadSchemaFile(filePath string) (*SchemaDefinition, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	var schema SchemaDefinition
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	
	return &schema, nil
}

// GetSchema retrieves a schema by entity and version
func (r *SchemaRegistry) GetSchema(entity, version string) (*SchemaDefinition, error) {
	key := fmt.Sprintf("%s:%s", entity, version)
	schema, exists := r.schemas[key]
	if !exists {
		return nil, fmt.Errorf("schema not found: %s", key)
	}
	return schema, nil
}

// ValidateEnvelope validates a data envelope against schema
func (r *SchemaRegistry) ValidateEnvelope(data map[string]interface{}, entity, version string, mode ValidationMode) error {
	schema, err := r.GetSchema(entity, version)
	if err != nil {
		return err
	}
	
	return r.validateData(data, schema, mode)
}

// validateData performs the actual validation logic
func (r *SchemaRegistry) validateData(data map[string]interface{}, schema *SchemaDefinition, mode ValidationMode) error {
	// Check required fields
	for _, required := range schema.Required {
		if _, exists := data[required]; !exists {
			return fmt.Errorf("required field missing: %s", required)
		}
	}
	
	// Validate field types and constraints
	for _, field := range schema.Fields {
		value, exists := data[field.Name]
		if !exists {
			if !field.Optional && field.Default == nil {
				return fmt.Errorf("non-optional field missing: %s", field.Name)
			}
			continue
		}
		
		if err := r.validateFieldType(value, field.Type, field.Name); err != nil {
			return err
		}
	}
	
	// Handle unknown fields based on mode
	knownFields := make(map[string]bool)
	for _, field := range schema.Fields {
		knownFields[field.Name] = true
	}
	
	for fieldName := range data {
		if !knownFields[fieldName] {
			switch mode {
			case ValidationStrict:
				return fmt.Errorf("unknown field: %s", fieldName)
			case ValidationWarn:
				// In a real implementation, we'd log this warning
				fmt.Printf("Warning: unknown field %s\n", fieldName)
			case ValidationIgnore:
				// Do nothing
			}
		}
	}
	
	return nil
}

// validateFieldType checks if a value matches expected type
func (r *SchemaRegistry) validateFieldType(value interface{}, expectedType, fieldName string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field %s: expected string, got %T", fieldName, value)
		}
	case "float64":
		switch value.(type) {
		case float64, int, int64:
			// Accept numeric types
		default:
			return fmt.Errorf("field %s: expected numeric, got %T", fieldName, value)
		}
	case "int64":
		switch value.(type) {
		case int64, int:
			// Accept integer types
		default:
			return fmt.Errorf("field %s: expected integer, got %T", fieldName, value)
		}
	case "timestamp":
		switch value.(type) {
		case time.Time, string:
			// Accept time or ISO string
		default:
			return fmt.Errorf("field %s: expected timestamp, got %T", fieldName, value)
		}
	default:
		return fmt.Errorf("unknown field type: %s", expectedType)
	}
	
	return nil
}

// CreateDefaultSchemas creates default schema files for core entities
func (r *SchemaRegistry) CreateDefaultSchemas() error {
	envelopeSchema := &SchemaDefinition{
		Version: "1.0.0",
		Entity:  "envelope",
		Fields: []FieldDefinition{
			{Name: "timestamp", Type: "timestamp", Optional: false},
			{Name: "venue", Type: "string", Optional: false},
			{Name: "symbol", Type: "string", Optional: false},
			{Name: "tier", Type: "string", Optional: false},
			{Name: "original_source", Type: "string", Optional: false},
			{Name: "confidence_score", Type: "float64", Optional: false},
			{Name: "processing_delay_ms", Type: "int64", Optional: true, Default: 0},
			{Name: "best_bid_price", Type: "float64", Optional: false},
			{Name: "best_ask_price", Type: "float64", Optional: false},
			{Name: "best_bid_qty", Type: "float64", Optional: false},
			{Name: "best_ask_qty", Type: "float64", Optional: false},
			{Name: "mid_price", Type: "float64", Optional: false},
			{Name: "spread_bps", Type: "float64", Optional: false},
			{Name: "row_count", Type: "int64", Optional: true},
			{Name: "min_timestamp_unix", Type: "int64", Optional: true},
			{Name: "max_timestamp_unix", Type: "int64", Optional: true},
			{Name: "schema_version", Type: "string", Optional: true, Default: "1.0.0"},
		},
		Required: []string{"timestamp", "venue", "symbol", "tier", "original_source", "confidence_score",
			"best_bid_price", "best_ask_price", "best_bid_qty", "best_ask_qty", "mid_price", "spread_bps"},
		Constraints: map[string]interface{}{
			"confidence_score": map[string]float64{"min": 0.0, "max": 1.0},
			"spread_bps":       map[string]float64{"min": 0.0},
		},
		CreatedAt: time.Now(),
	}
	
	schemaData, err := json.MarshalIndent(envelopeSchema, "", "  ")
	if err != nil {
		return err
	}
	
	schemaFile := filepath.Join(r.schemaPath, "envelope_v1.0.0.json")
	return os.WriteFile(schemaFile, schemaData, 0644)
}

// GetSupportedVersions returns all available versions for an entity
func (r *SchemaRegistry) GetSupportedVersions(entity string) []string {
	var versions []string
	for key := range r.schemas {
		if entityName := key[:len(key)-6]; entityName == entity {
			version := key[len(entity)+1:]
			versions = append(versions, version)
		}
	}
	return versions
}

// ValidatePIT validates point-in-time integrity of a dataset
func ValidatePIT(data []DataEnvelope, expectedMinTime, expectedMaxTime time.Time, expectedRowCount int64) error {
	if int64(len(data)) != expectedRowCount {
		return fmt.Errorf("PIT integrity violation: expected %d rows, got %d", expectedRowCount, len(data))
	}
	
	if len(data) == 0 {
		return nil
	}
	
	minTime := data[0].Timestamp
	maxTime := data[0].Timestamp
	
	for _, envelope := range data {
		if envelope.Timestamp.Before(minTime) {
			minTime = envelope.Timestamp
		}
		if envelope.Timestamp.After(maxTime) {
			maxTime = envelope.Timestamp
		}
	}
	
	if minTime.Before(expectedMinTime) || maxTime.After(expectedMaxTime) {
		return fmt.Errorf("PIT integrity violation: time range [%v, %v] exceeds expected [%v, %v]",
			minTime, maxTime, expectedMinTime, expectedMaxTime)
	}
	
	return nil
}