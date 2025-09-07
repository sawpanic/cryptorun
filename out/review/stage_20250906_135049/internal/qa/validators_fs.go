package qa

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
)

// ReadCSV reads a CSV file and returns rows as string slices
func ReadCSV(filepath string) ([][]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV records: %w", err)
	}

	return records, nil
}

// ReadJSON reads and parses a JSON file into the provided interface
func ReadJSON(filepath string, target interface{}) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

// WriteJSON writes an object to JSON file with proper formatting
func WriteJSON(filepath string, data interface{}) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(filepath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	return nil
}

// ValidateJSONSchema performs basic JSON schema validation
func ValidateJSONSchema(filepath string, schema interface{}) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JSON
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Basic structure validation using reflection
	if err := validateStructure(parsed, schema); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	return nil
}

// validateStructure performs basic structural validation using reflection
func validateStructure(data interface{}, schema interface{}) error {
	dataValue := reflect.ValueOf(data)
	schemaValue := reflect.ValueOf(schema)

	// Handle interface{} and pointer types
	for schemaValue.Kind() == reflect.Ptr || schemaValue.Kind() == reflect.Interface {
		if schemaValue.IsNil() {
			return nil
		}
		schemaValue = schemaValue.Elem()
	}

	switch schemaValue.Kind() {
	case reflect.Map:
		return validateMap(dataValue, schemaValue)
	case reflect.Slice:
		return validateSlice(dataValue, schemaValue)
	case reflect.Struct:
		return validateStruct(dataValue, schemaValue)
	default:
		// For basic types, just check compatibility
		return nil
	}
}

func validateMap(data, schema reflect.Value) error {
	if data.Kind() != reflect.Map {
		return fmt.Errorf("expected map, got %s", data.Kind())
	}

	// Basic validation - just check that it's a map
	return nil
}

func validateSlice(data, schema reflect.Value) error {
	if data.Kind() != reflect.Slice {
		return fmt.Errorf("expected slice, got %s", data.Kind())
	}

	// Basic validation - just check that it's a slice
	return nil
}

func validateStruct(data, schema reflect.Value) error {
	if data.Kind() != reflect.Map {
		return fmt.Errorf("expected object (map), got %s", data.Kind())
	}

	// For JSON validation, we expect maps from JSON parsing
	dataMap, ok := data.Interface().(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected string-keyed map")
	}

	schemaType := schema.Type()
	for i := 0; i < schema.NumField(); i++ {
		field := schemaType.Field(i)
		jsonTag := field.Tag.Get("json")
		
		fieldName := field.Name
		if jsonTag != "" && jsonTag != "-" {
			// Parse JSON tag (e.g., "name,omitempty")
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Check if required field exists
		if _, exists := dataMap[fieldName]; !exists {
			// Check if it's omitempty
			if jsonTag != "" && strings.Contains(jsonTag, "omitempty") {
				continue
			}
			return fmt.Errorf("required field missing: %s", fieldName)
		}
	}

	return nil
}

// ValidateCSVStructure validates CSV file structure
func ValidateCSVStructure(filepath string, requiredColumns []string) error {
	rows, err := ReadCSV(filepath)
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(rows) == 0 {
		return fmt.Errorf("CSV file is empty")
	}

	// Check header
	header := rows[0]
	for _, required := range requiredColumns {
		found := false
		for _, col := range header {
			if strings.EqualFold(col, required) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing required column: %s", required)
		}
	}

	// Check for data rows
	if len(rows) < 2 {
		return fmt.Errorf("CSV must have at least one data row")
	}

	return nil
}

// FileExists checks if a file exists and is readable
func FileExists(filepath string) error {
	info, err := os.Stat(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", filepath)
		}
		return fmt.Errorf("cannot access file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", filepath)
	}

	return nil
}

// ValidateFileSize ensures file is not empty and within reasonable limits
func ValidateFileSize(filepath string, minSize, maxSize int64) error {
	info, err := os.Stat(filepath)
	if err != nil {
		return fmt.Errorf("cannot stat file: %w", err)
	}

	size := info.Size()
	if size < minSize {
		return fmt.Errorf("file too small: %d bytes (minimum: %d)", size, minSize)
	}

	if maxSize > 0 && size > maxSize {
		return fmt.Errorf("file too large: %d bytes (maximum: %d)", size, maxSize)
	}

	return nil
}

// EnsureDirectory creates directory if it doesn't exist
func EnsureDirectory(dirpath string) error {
	return os.MkdirAll(dirpath, 0755)
}

// CleanupTestFiles removes test files and directories
func CleanupTestFiles(paths ...string) error {
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to cleanup %s: %w", path, err)
		}
	}
	return nil
}