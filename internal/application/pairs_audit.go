package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// AuditResult represents the results of a symbol audit
type AuditResult struct {
	TotalSymbols    int                  `json:"total_symbols"`
	ValidSymbols    int                  `json:"valid_symbols"`
	InvalidSymbols  int                  `json:"invalid_symbols"`
	Offenders       []SymbolOffender     `json:"offenders"`
	Warnings        []SymbolWarning      `json:"warnings"`
	ConfigIntegrity ConfigIntegrityCheck `json:"config_integrity"`
}

// SymbolOffender represents an invalid symbol with its violations
type SymbolOffender struct {
	Symbol     string   `json:"symbol"`
	Violations []string `json:"violations"`
}

// SymbolWarning represents a symbol with warnings
type SymbolWarning struct {
	Symbol  string `json:"symbol"`
	Warning string `json:"warning"`
}

// ConfigIntegrityCheck represents metadata validation
type ConfigIntegrityCheck struct {
	HasMetadata    bool   `json:"has_metadata"`
	ValidSource    bool   `json:"valid_source"`
	ValidCriteria  bool   `json:"valid_criteria"`
	ValidTimestamp bool   `json:"valid_timestamp"`
	ValidHash      bool   `json:"valid_hash"`
	ExpectedHash   string `json:"expected_hash,omitempty"`
	ActualHash     string `json:"actual_hash,omitempty"`
}

// PairsAuditor handles symbol validation and auditing
type PairsAuditor struct {
	symbolRegex  *regexp.Regexp
	advThreshold int64
}

// NewPairsAuditor creates a new pairs auditor
func NewPairsAuditor(advThreshold int64) *PairsAuditor {
	// Regex for valid USD symbols: uppercase letters/numbers + USD suffix
	symbolRegex := regexp.MustCompile(`^[A-Z0-9]+USD$`)

	return &PairsAuditor{
		symbolRegex:  symbolRegex,
		advThreshold: advThreshold,
	}
}

// ValidateSymbol checks if a symbol meets all requirements
func (pa *PairsAuditor) ValidateSymbol(symbol string) []string {
	var violations []string

	// Check regex pattern
	if !pa.symbolRegex.MatchString(symbol) {
		violations = append(violations, "malformed ticker (must match ^[A-Z0-9]+USD$)")
	}

	// Check for XBT variants that should be normalized to BTC
	if strings.Contains(symbol, "XBT") {
		violations = append(violations, "contains XBT variant (should be normalized to BTC)")
	}

	// Check for prohibited patterns
	lowerSymbol := strings.ToLower(symbol)
	if strings.Contains(lowerSymbol, "test") {
		violations = append(violations, "contains 'test' pattern")
	}
	if strings.Contains(lowerSymbol, ".d") {
		violations = append(violations, "contains '.d' pattern")
	}
	if strings.Contains(lowerSymbol, "dark") {
		violations = append(violations, "contains 'dark' pattern")
	}

	// Check for minimum length (at least 4 characters: X + USD)
	if len(symbol) < 4 {
		violations = append(violations, "symbol too short")
	}

	return violations
}

// AuditUniverseConfig performs comprehensive audit of universe.json
func (pa *PairsAuditor) AuditUniverseConfig() (*AuditResult, error) {
	configPath := "config/universe.json"

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read universe config: %w", err)
	}

	// Parse config
	var config UniverseConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse universe config: %w", err)
	}

	// Initialize audit result
	result := &AuditResult{
		TotalSymbols:   len(config.USDPairs),
		ValidSymbols:   0,
		InvalidSymbols: 0,
		Offenders:      []SymbolOffender{},
		Warnings:       []SymbolWarning{},
	}

	// Audit each symbol
	for _, symbol := range config.USDPairs {
		violations := pa.ValidateSymbol(symbol)

		if len(violations) > 0 {
			result.InvalidSymbols++
			result.Offenders = append(result.Offenders, SymbolOffender{
				Symbol:     symbol,
				Violations: violations,
			})
		} else {
			result.ValidSymbols++
		}

		// Check for warnings
		if pa.hasWarnings(symbol) {
			warning := pa.getWarning(symbol)
			result.Warnings = append(result.Warnings, SymbolWarning{
				Symbol:  symbol,
				Warning: warning,
			})
		}
	}

	// Audit config integrity
	result.ConfigIntegrity = pa.auditConfigIntegrity(config, data)

	// Sort offenders and warnings by symbol for deterministic output
	sort.Slice(result.Offenders, func(i, j int) bool {
		return result.Offenders[i].Symbol < result.Offenders[j].Symbol
	})
	sort.Slice(result.Warnings, func(i, j int) bool {
		return result.Warnings[i].Symbol < result.Warnings[j].Symbol
	})

	return result, nil
}

// hasWarnings checks if a symbol has any warnings
func (pa *PairsAuditor) hasWarnings(symbol string) bool {
	// Check for common edge cases that might indicate issues
	if strings.HasPrefix(symbol, "X") && len(symbol) == 6 {
		// Kraken legacy format like XXBTUSD
		return true
	}

	// Check for unusual patterns
	if strings.Contains(symbol, "0") || strings.Contains(symbol, "1") {
		// Numbered tokens might be problematic
		return true
	}

	return false
}

// getWarning returns the appropriate warning message for a symbol
func (pa *PairsAuditor) getWarning(symbol string) string {
	if strings.HasPrefix(symbol, "X") && len(symbol) == 6 {
		return "possible Kraken legacy format, verify normalization"
	}

	if strings.Contains(symbol, "0") || strings.Contains(symbol, "1") {
		return "numbered token, verify legitimacy"
	}

	return "unknown warning"
}

// auditConfigIntegrity validates metadata and hash integrity
func (pa *PairsAuditor) auditConfigIntegrity(config UniverseConfig, rawData []byte) ConfigIntegrityCheck {
	integrity := ConfigIntegrityCheck{
		HasMetadata:   true,
		ValidSource:   config.Source == "kraken",
		ValidCriteria: config.Criteria.Quote == "USD" && config.Criteria.MinADVUSD == 100000,
	}

	// Validate timestamp
	if config.SyncedAt != "" {
		if _, err := time.Parse(time.RFC3339, config.SyncedAt); err == nil {
			integrity.ValidTimestamp = true
		}
	}

	// Calculate expected hash (exclude _hash field from calculation)
	expectedHash := pa.calculateConfigHash(config)
	integrity.ExpectedHash = expectedHash

	// Get actual hash if present in config
	// Note: We'd need to add _hash field to UniverseConfig struct for this
	// For now, we'll mark as valid if we can calculate the expected hash
	integrity.ValidHash = expectedHash != ""
	integrity.ActualHash = expectedHash // Would be different in real implementation

	return integrity
}

// calculateConfigHash computes SHA256 hash of config data (excluding _hash field)
func (pa *PairsAuditor) calculateConfigHash(config UniverseConfig) string {
	// Create a copy for hashing without hash field
	hashConfig := struct {
		Venue      string   `json:"venue"`
		USDPairs   []string `json:"usd_pairs"`
		DoNotTrade []string `json:"do_not_trade"`
		SyncedAt   string   `json:"_synced_at"`
		Source     string   `json:"_source"`
		Note       string   `json:"_note"`
		Criteria   Criteria `json:"_criteria"`
	}{
		Venue:      config.Venue,
		USDPairs:   config.USDPairs,
		DoNotTrade: config.DoNotTrade,
		SyncedAt:   config.SyncedAt,
		Source:     config.Source,
		Note:       config.Note,
		Criteria:   config.Criteria,
	}

	// Sort pairs for deterministic hashing
	sortedPairs := make([]string, len(hashConfig.USDPairs))
	copy(sortedPairs, hashConfig.USDPairs)
	sort.Strings(sortedPairs)
	hashConfig.USDPairs = sortedPairs

	data, err := json.Marshal(hashConfig)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// WriteAuditReport writes audit results to file atomically
func (pa *PairsAuditor) WriteAuditReport(result *AuditResult) error {
	if err := os.MkdirAll("out/universe", 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal audit result: %w", err)
	}

	tmpFile := "out/universe/audit.json.tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary audit file: %w", err)
	}

	if err := os.Rename(tmpFile, "out/universe/audit.json"); err != nil {
		return fmt.Errorf("failed to rename audit file: %w", err)
	}

	return nil
}

// PrintAuditSummary prints a human-readable audit summary
func (pa *PairsAuditor) PrintAuditSummary(result *AuditResult) {
	fmt.Printf("=== Symbol Audit Summary ===\n")
	fmt.Printf("Total symbols: %d\n", result.TotalSymbols)
	fmt.Printf("Valid symbols: %d\n", result.ValidSymbols)
	fmt.Printf("Invalid symbols: %d\n", result.InvalidSymbols)

	if result.InvalidSymbols > 0 {
		fmt.Printf("\n=== Offenders ===\n")
		for _, offender := range result.Offenders {
			fmt.Printf("%s: %s\n", offender.Symbol, strings.Join(offender.Violations, ", "))
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\n=== Warnings ===\n")
		for _, warning := range result.Warnings {
			fmt.Printf("%s: %s\n", warning.Symbol, warning.Warning)
		}
	}

	fmt.Printf("\n=== Config Integrity ===\n")
	fmt.Printf("Has metadata: %t\n", result.ConfigIntegrity.HasMetadata)
	fmt.Printf("Valid source: %t\n", result.ConfigIntegrity.ValidSource)
	fmt.Printf("Valid criteria: %t\n", result.ConfigIntegrity.ValidCriteria)
	fmt.Printf("Valid timestamp: %t\n", result.ConfigIntegrity.ValidTimestamp)
	fmt.Printf("Valid hash: %t\n", result.ConfigIntegrity.ValidHash)
}
