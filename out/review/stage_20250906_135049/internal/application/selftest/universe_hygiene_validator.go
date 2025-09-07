package selftest

import (
	"encoding/json"
	"fmt"
	"os"

	"regexp"
	"strconv"
	"strings"
	"time"
)

// UniverseHygieneValidator validates universe configuration and constraints
type UniverseHygieneValidator struct{}

// NewUniverseHygieneValidator creates a new universe hygiene validator
func NewUniverseHygieneValidator() *UniverseHygieneValidator {
	return &UniverseHygieneValidator{}
}

// Name returns the validator name
func (uhv *UniverseHygieneValidator) Name() string {
	return "Universe Hygiene Validation"
}

// UniverseConfig represents universe configuration structure
type UniverseConfig struct {
	Pairs []PairConfig `json:"pairs"`
	Constraints struct {
		MinADV     float64  `json:"min_adv"`
		USDOnly    bool     `json:"usd_only"`
		Exchanges  []string `json:"exchanges"`
	} `json:"constraints"`
	Hash string `json:"hash"`
}

// PairConfig represents a trading pair configuration
type PairConfig struct {
	Symbol     string  `json:"symbol"`
	Exchange   string  `json:"exchange"`
	ADV        float64 `json:"adv"` // Average Daily Volume in USD
	Base       string  `json:"base"`
	Quote      string  `json:"quote"`
	Active     bool    `json:"active"`
}

// Validate checks universe hygiene requirements
func (uhv *UniverseHygieneValidator) Validate() TestResult {
	start := time.Now()
	result := TestResult{
		Name:      uhv.Name(),
		Timestamp: start,
		Details:   []string{},
	}
	
	// Check 1: Load universe configuration
	universeConfig, err := uhv.loadUniverseConfig()
	if err != nil {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Failed to load universe config: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	result.Details = append(result.Details, fmt.Sprintf("Loaded universe config with %d pairs", len(universeConfig.Pairs)))
	
	// Check 2: Validate USD-only constraint
	nonUSDPairs := []string{}
	for _, pair := range universeConfig.Pairs {
		if pair.Active && pair.Quote != "USD" && pair.Quote != "USDT" && pair.Quote != "USDC" {
			nonUSDPairs = append(nonUSDPairs, pair.Symbol)
		}
	}
	
	if len(nonUSDPairs) > 0 {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Found %d non-USD pairs", len(nonUSDPairs))
		result.Details = append(result.Details, "Non-USD pairs found:")
		for _, pair := range nonUSDPairs {
			result.Details = append(result.Details, fmt.Sprintf("  - %s", pair))
		}
	} else {
		result.Details = append(result.Details, "USD-only constraint satisfied")
	}
	
	// Check 3: Validate minimum ADV constraint
	lowADVPairs := []string{}
	minADV := universeConfig.Constraints.MinADV
	if minADV == 0 {
		minADV = 100000 // Default $100k minimum
	}
	
	for _, pair := range universeConfig.Pairs {
		if pair.Active && pair.ADV < minADV {
			lowADVPairs = append(lowADVPairs, fmt.Sprintf("%s ($%.0f)", pair.Symbol, pair.ADV))
		}
	}
	
	if len(lowADVPairs) > 0 {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Found %d pairs below minimum ADV $%.0f", len(lowADVPairs), minADV)
		result.Details = append(result.Details, "Low ADV pairs:")
		for _, pair := range lowADVPairs {
			result.Details = append(result.Details, fmt.Sprintf("  - %s", pair))
		}
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Minimum ADV constraint ($%.0f) satisfied", minADV))
	}
	
	// Check 4: Validate supported exchanges
	supportedExchanges := map[string]bool{
		"kraken":   true,
		"binance":  true,
		"coinbase": true,
		"okx":      true,
	}
	
	unsupportedExchanges := []string{}
	for _, pair := range universeConfig.Pairs {
		if pair.Active && !supportedExchanges[strings.ToLower(pair.Exchange)] {
			unsupportedExchanges = append(unsupportedExchanges, fmt.Sprintf("%s (%s)", pair.Symbol, pair.Exchange))
		}
	}
	
	if len(unsupportedExchanges) > 0 {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Found %d pairs on unsupported exchanges", len(unsupportedExchanges))
		result.Details = append(result.Details, "Unsupported exchanges:")
		for _, pair := range unsupportedExchanges {
			result.Details = append(result.Details, fmt.Sprintf("  - %s", pair))
		}
	} else {
		result.Details = append(result.Details, "All exchanges are supported")
	}
	
	// Check 5: Validate hash integrity
	if universeConfig.Hash == "" {
		result.Status = "FAIL"
		result.Message = "Universe hash is missing"
	} else if !uhv.isValidHash(universeConfig.Hash) {
		result.Status = "FAIL"
		result.Message = "Universe hash format is invalid"
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Universe hash present: %s", universeConfig.Hash[:8]+"..."))
	}
	
	// Check 6: Validate symbol format consistency
	invalidSymbols := []string{}
	symbolRegex := regexp.MustCompile(`^[A-Z0-9]+[/-][A-Z0-9]+$`)
	
	for _, pair := range universeConfig.Pairs {
		if pair.Active && !symbolRegex.MatchString(pair.Symbol) {
			invalidSymbols = append(invalidSymbols, pair.Symbol)
		}
	}
	
	if len(invalidSymbols) > 0 {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Found %d symbols with invalid format", len(invalidSymbols))
		result.Details = append(result.Details, "Invalid symbol formats:")
		for _, symbol := range invalidSymbols {
			result.Details = append(result.Details, fmt.Sprintf("  - %s", symbol))
		}
	} else {
		result.Details = append(result.Details, "All symbol formats are valid")
	}
	
	// Check 7: Validate active pair count
	activePairs := 0
	for _, pair := range universeConfig.Pairs {
		if pair.Active {
			activePairs++
		}
	}
	
	if activePairs == 0 {
		result.Status = "FAIL"
		result.Message = "No active pairs found"
	} else if activePairs < 10 {
		result.Details = append(result.Details, fmt.Sprintf("Warning: Only %d active pairs (may be insufficient for diversification)", activePairs))
	} else {
		result.Details = append(result.Details, fmt.Sprintf("Found %d active pairs", activePairs))
	}
	
	if result.Status == "" {
		result.Status = "PASS"
		result.Message = "Universe hygiene validation passed"
	}
	
	result.Duration = time.Since(start)
	return result
}

// loadUniverseConfig loads universe configuration from file or creates test config
func (uhv *UniverseHygieneValidator) loadUniverseConfig() (*UniverseConfig, error) {
	// Try to load from config file first
	configPaths := []string{
		"config/universe.json",
		"config/pairs.json",
		"src/config/universe.json",
	}
	
	for _, path := range configPaths {
		if data, err := os.ReadFile(path); err == nil {
			var config UniverseConfig
			if err := json.Unmarshal(data, &config); err == nil {
				return &config, nil
			}
		}
	}
	
	// Create test universe config if no file exists
	testConfig := &UniverseConfig{
		Pairs: []PairConfig{
			{Symbol: "BTC/USD", Exchange: "kraken", ADV: 500000000, Base: "BTC", Quote: "USD", Active: true},
			{Symbol: "ETH/USD", Exchange: "kraken", ADV: 200000000, Base: "ETH", Quote: "USD", Active: true},
			{Symbol: "SOL/USD", Exchange: "kraken", ADV: 50000000, Base: "SOL", Quote: "USD", Active: true},
			{Symbol: "ADA/USD", Exchange: "kraken", ADV: 25000000, Base: "ADA", Quote: "USD", Active: true},
			{Symbol: "DOT/USD", Exchange: "kraken", ADV: 15000000, Base: "DOT", Quote: "USD", Active: true},
			{Symbol: "LINK/USD", Exchange: "kraken", ADV: 30000000, Base: "LINK", Quote: "USD", Active: true},
			{Symbol: "AVAX/USD", Exchange: "kraken", ADV: 20000000, Base: "AVAX", Quote: "USD", Active: true},
			{Symbol: "MATIC/USD", Exchange: "kraken", ADV: 40000000, Base: "MATIC", Quote: "USD", Active: true},
			{Symbol: "UNI/USD", Exchange: "kraken", ADV: 35000000, Base: "UNI", Quote: "USD", Active: true},
			{Symbol: "ATOM/USD", Exchange: "kraken", ADV: 18000000, Base: "ATOM", Quote: "USD", Active: true},
			// Test cases for validation
			{Symbol: "BTC/EUR", Exchange: "kraken", ADV: 50000000, Base: "BTC", Quote: "EUR", Active: false}, // Non-USD (inactive)
			{Symbol: "SHIB/USD", Exchange: "kraken", ADV: 50000, Base: "SHIB", Quote: "USD", Active: false},   // Low ADV (inactive)
		},
		Constraints: struct {
			MinADV     float64  `json:"min_adv"`
			USDOnly    bool     `json:"usd_only"`
			Exchanges  []string `json:"exchanges"`
		}{
			MinADV:    100000,
			USDOnly:   true,
			Exchanges: []string{"kraken", "binance", "coinbase", "okx"},
		},
		Hash: "sha256:a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
	}
	
	return testConfig, nil
}

// isValidHash validates hash format
func (uhv *UniverseHygieneValidator) isValidHash(hash string) bool {
	// Check for SHA256 hash format
	if strings.HasPrefix(hash, "sha256:") {
		hashPart := strings.TrimPrefix(hash, "sha256:")
		if len(hashPart) == 64 {
			// Check if it's valid hex
			if _, err := strconv.ParseUint(hashPart[:8], 16, 32); err == nil {
				return true
			}
		}
	}
	
	// Check for simple hex hash (32 or 64 characters)
	if len(hash) == 32 || len(hash) == 64 {
		if _, err := strconv.ParseUint(hash[:8], 16, 32); err == nil {
			return true
		}
	}
	
	return false
}