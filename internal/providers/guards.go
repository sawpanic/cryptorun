// Package providers enforces exchange-native data source requirements
// and prevents aggregator usage for microstructure data
package providers

import (
	"fmt"
	"reflect"
	"strings"
)

// ExchangeNativeGuard enforces exchange-native data source requirements
// CRITICAL: Prevents aggregator usage for depth/spread/order book data
type ExchangeNativeGuard struct {
	allowedExchanges map[string]bool
	bannedSources    map[string]bool
}

// NewExchangeNativeGuard creates a new guard with CryptoRun v3.2.1 requirements
func NewExchangeNativeGuard() *ExchangeNativeGuard {
	return &ExchangeNativeGuard{
		allowedExchanges: map[string]bool{
			"binance":  true,
			"okx":      true, 
			"coinbase": true,
			"kraken":   true,
		},
		bannedSources: map[string]bool{
			// Aggregators BANNED for microstructure data
			"coingecko":    true,
			"coinpaprika":  true,
			"dexscreener":  true,
			"defillama":    true,
			"coinmarketcap": true,
			"cryptocompare": true,
			"messari":      true,
			"nomics":       true,
			"coinapi":      true,
			
			// Internal aggregation patterns also banned
			"aggregated":   true,
			"composite":    true,
			"blended":      true,
			"averaged":     true,
			"merged":       true,
		},
	}
}

// ValidateDataSource ensures data source compliance for microstructure data
func (g *ExchangeNativeGuard) ValidateDataSource(source string, dataType DataType) error {
	sourceLower := strings.ToLower(source)
	
	// For microstructure data, only exchange-native sources allowed
	if dataType == DataTypeMicrostructure {
		if g.bannedSources[sourceLower] {
			return &AggregatorViolationError{
				Source:   source,
				DataType: dataType,
				Reason:   fmt.Sprintf("Aggregator '%s' banned for microstructure data", source),
			}
		}
		
		if !g.allowedExchanges[sourceLower] {
			return &AggregatorViolationError{
				Source:   source,
				DataType: dataType,
				Reason:   fmt.Sprintf("Source '%s' not in allowed exchange list", source),
			}
		}
	}
	
	return nil
}

// ValidateProvider checks if a provider interface violates exchange-native requirements
func (g *ExchangeNativeGuard) ValidateProvider(provider interface{}) error {
	providerType := reflect.TypeOf(provider)
	_ = reflect.ValueOf(provider) // Currently unused but available for future validation
	
	if providerType == nil {
		return fmt.Errorf("provider is nil")
	}
	
	// Check provider name/type for banned patterns
	providerName := strings.ToLower(providerType.String())
	for bannedSource := range g.bannedSources {
		if strings.Contains(providerName, bannedSource) {
			return &AggregatorViolationError{
				Source:   providerName,
				DataType: DataTypeMicrostructure,
				Reason:   fmt.Sprintf("Provider type contains banned pattern: %s", bannedSource),
			}
		}
	}
	
	// Check for methods that suggest aggregation
	if providerType.Kind() == reflect.Interface || providerType.Kind() == reflect.Ptr {
		if providerType.Kind() == reflect.Ptr {
			providerType = providerType.Elem()
		}
		
		for i := 0; i < providerType.NumMethod(); i++ {
			method := providerType.Method(i)
			methodName := strings.ToLower(method.Name)
			
			// Flag methods that suggest data aggregation
			if strings.Contains(methodName, "aggregate") ||
			   strings.Contains(methodName, "blend") ||
			   strings.Contains(methodName, "merge") ||
			   strings.Contains(methodName, "combine") ||
			   strings.Contains(methodName, "composite") {
				return &AggregatorViolationError{
					Source:   providerName,
					DataType: DataTypeMicrostructure,
					Reason:   fmt.Sprintf("Provider has aggregation method: %s", method.Name),
				}
			}
		}
	}
	
	return nil
}

// ValidateDataStructure checks if data structures contain aggregated sources
func (g *ExchangeNativeGuard) ValidateDataStructure(data interface{}, dataType DataType) error {
	return g.validateStructRecursive(reflect.ValueOf(data), dataType, "")
}

func (g *ExchangeNativeGuard) validateStructRecursive(v reflect.Value, dataType DataType, fieldPath string) error {
	if !v.IsValid() {
		return nil
	}
	
	// Handle pointers and interfaces
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	
	switch v.Kind() {
	case reflect.String:
		str := v.String()
		if err := g.ValidateDataSource(str, dataType); err != nil {
			return fmt.Errorf("field %s: %w", fieldPath, err)
		}
		
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			fieldValue := v.Field(i)
			
			if !fieldValue.CanInterface() {
				continue
			}
			
			newPath := field.Name
			if fieldPath != "" {
				newPath = fieldPath + "." + field.Name
			}
			
			// Check specific field names for banned sources
			fieldName := strings.ToLower(field.Name)
			if strings.Contains(fieldName, "source") ||
			   strings.Contains(fieldName, "provider") ||
			   strings.Contains(fieldName, "venue") {
				if fieldValue.Kind() == reflect.String {
					str := fieldValue.String()
					if err := g.ValidateDataSource(str, dataType); err != nil {
						return fmt.Errorf("field %s: %w", newPath, err)
					}
				}
			}
			
			if err := g.validateStructRecursive(fieldValue, dataType, newPath); err != nil {
				return err
			}
		}
		
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			newPath := fmt.Sprintf("%s[%d]", fieldPath, i)
			if fieldPath == "" {
				newPath = fmt.Sprintf("[%d]", i)
			}
			
			if err := g.validateStructRecursive(v.Index(i), dataType, newPath); err != nil {
				return err
			}
		}
		
	case reflect.Map:
		for _, key := range v.MapKeys() {
			keyStr := fmt.Sprintf("%v", key.Interface())
			mapValue := v.MapIndex(key)
			
			newPath := fmt.Sprintf("%s[%s]", fieldPath, keyStr)
			if fieldPath == "" {
				newPath = fmt.Sprintf("[%s]", keyStr)
			}
			
			// Check map keys for banned sources
			if err := g.ValidateDataSource(keyStr, dataType); err != nil {
				return fmt.Errorf("map key %s: %w", newPath, err)
			}
			
			if err := g.validateStructRecursive(mapValue, dataType, newPath); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// GetAllowedExchanges returns the list of allowed exchange-native sources
func (g *ExchangeNativeGuard) GetAllowedExchanges() []string {
	exchanges := make([]string, 0, len(g.allowedExchanges))
	for exchange := range g.allowedExchanges {
		exchanges = append(exchanges, exchange)
	}
	return exchanges
}

// GetBannedSources returns the list of banned aggregator sources
func (g *ExchangeNativeGuard) GetBannedSources() []string {
	sources := make([]string, 0, len(g.bannedSources))
	for source := range g.bannedSources {
		sources = append(sources, source)
	}
	return sources
}

// IsExchangeNative checks if a source is exchange-native
func (g *ExchangeNativeGuard) IsExchangeNative(source string) bool {
	sourceLower := strings.ToLower(source)
	return g.allowedExchanges[sourceLower] && !g.bannedSources[sourceLower]
}

// DataType represents different types of market data
type DataType int

const (
	DataTypeUnknown DataType = iota
	DataTypeMicrostructure // Depth, spread, order book - EXCHANGE-NATIVE ONLY
	DataTypePricing        // Price, volume - aggregators allowed
	DataTypeFunding        // Funding rates, basis - exchange-native preferred
	DataTypeSocial         // Social sentiment - aggregators allowed
)

// String returns the string representation of DataType
func (dt DataType) String() string {
	switch dt {
	case DataTypeMicrostructure:
		return "microstructure"
	case DataTypePricing:
		return "pricing"
	case DataTypeFunding:
		return "funding"
	case DataTypeSocial:
		return "social"
	default:
		return "unknown"
	}
}

// AggregatorViolationError represents a violation of exchange-native requirements
type AggregatorViolationError struct {
	Source   string   `json:"source"`
	DataType DataType `json:"data_type"`
	Reason   string   `json:"reason"`
}

// Error implements the error interface
func (e *AggregatorViolationError) Error() string {
	return fmt.Sprintf("aggregator violation: %s (source: %s, type: %s)", 
		e.Reason, e.Source, e.DataType.String())
}

// IsAggregatorViolation checks if an error is an aggregator violation
func IsAggregatorViolation(err error) bool {
	_, ok := err.(*AggregatorViolationError)
	return ok
}

// CompileTimeGuard ensures aggregator protection is checked at compile time
// This function should be used in unit tests to catch violations early
func CompileTimeGuard() {
	guard := NewExchangeNativeGuard()
	
	// Example banned sources that should trigger failures
	bannedExamples := []string{
		"coingecko",
		"dexscreener", 
		"aggregated_data",
		"composite_feed",
	}
	
	for _, source := range bannedExamples {
		if err := guard.ValidateDataSource(source, DataTypeMicrostructure); err == nil {
			panic(fmt.Sprintf("Compile-time guard failed: %s should be banned", source))
		}
	}
	
	// Example allowed sources that should pass
	allowedExamples := []string{
		"kraken",
		"binance",
		"coinbase",
		"okx",
	}
	
	for _, source := range allowedExamples {
		if err := guard.ValidateDataSource(source, DataTypeMicrostructure); err != nil {
			panic(fmt.Sprintf("Compile-time guard failed: %s should be allowed", source))
		}
	}
}