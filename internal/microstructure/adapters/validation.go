package adapters

import (
	"fmt"
	"runtime"
	"strings"
)

// AGGREGATOR BAN ENFORCEMENT
// This package implements compile-time and runtime guards to prevent
// the use of aggregated microstructure data sources as per v3.2.1 constraints.

// BannedAggregators lists prohibited aggregator sources for microstructure data
var BannedAggregators = []string{
	"dexscreener",
	"coingecko",
	"coinmarketcap",
	"cmc",
	"nomics",
	"messari",
	"cryptocompare",
	"coinapi",
	"coinlayer",
	"fixer",
	"aggregate",
	"aggregated",
	"multi_exchange",
	"composite",
	"blended",
}

// AllowedExchanges lists the only permitted exchange-native sources
var AllowedExchanges = []string{
	"binance",
	"okx",
	"coinbase",
	"kraken", // Future support
}

// AggregatorBanError represents a violation of the aggregator ban
type AggregatorBanError struct {
	Source     string // Source that triggered the ban
	Function   string // Function where ban was triggered
	Reason     string // Reason for the ban
	StackTrace string // Call stack for debugging
}

// Error implements the error interface
func (e *AggregatorBanError) Error() string {
	return fmt.Sprintf("AGGREGATOR BAN VIOLATION: %s in %s - %s", e.Source, e.Function, e.Reason)
}

// GuardAgainstAggregator enforces the aggregator ban with compile-time detection
func GuardAgainstAggregator(source string) error {
	// Get caller information for better error reporting
	pc, file, line, ok := runtime.Caller(1)
	callerInfo := "unknown"
	if ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			callerInfo = fmt.Sprintf("%s:%d (%s)", file, line, fn.Name())
		}
	}

	// Check if source matches any banned aggregators
	sourceLower := strings.ToLower(source)
	for _, banned := range BannedAggregators {
		if strings.Contains(sourceLower, banned) {
			return &AggregatorBanError{
				Source:     source,
				Function:   callerInfo,
				Reason:     fmt.Sprintf("contains banned aggregator '%s'", banned),
				StackTrace: getStackTrace(),
			}
		}
	}

	return nil
}

// ValidateExchangeNativeSource ensures source is from allowed exchanges
func ValidateExchangeNativeSource(source string) error {
	if err := GuardAgainstAggregator(source); err != nil {
		return err
	}

	sourceLower := strings.ToLower(source)
	for _, allowed := range AllowedExchanges {
		if strings.Contains(sourceLower, allowed) {
			return nil // Valid exchange-native source
		}
	}

	// Get caller information
	pc, file, line, ok := runtime.Caller(1)
	callerInfo := "unknown"
	if ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			callerInfo = fmt.Sprintf("%s:%d (%s)", file, line, fn.Name())
		}
	}

	return &AggregatorBanError{
		Source:     source,
		Function:   callerInfo,
		Reason:     fmt.Sprintf("not from allowed exchanges: %v", AllowedExchanges),
		StackTrace: getStackTrace(),
	}
}

// CheckMicrostructureDataSource validates a data source for microstructure use
func CheckMicrostructureDataSource(source, endpoint string, data interface{}) error {
	// Primary source validation
	if err := ValidateExchangeNativeSource(source); err != nil {
		return fmt.Errorf("source validation failed: %w", err)
	}

	// Endpoint validation - look for aggregator patterns
	endpointLower := strings.ToLower(endpoint)
	suspiciousPatterns := []string{
		"/aggregated/",
		"/composite/",
		"/blended/",
		"/multi_exchange/",
		"/average/",
		"/weighted/",
		"/index/",
		"/combined/",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(endpointLower, pattern) {
			return &AggregatorBanError{
				Source:   source,
				Function: fmt.Sprintf("endpoint: %s", endpoint),
				Reason:   fmt.Sprintf("suspicious aggregation pattern: %s", pattern),
			}
		}
	}

	return nil
}

// getStackTrace returns a formatted stack trace for debugging
func getStackTrace() string {
	buf := make([]byte, 1024*8)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// Convenience functions for common validation patterns

// ValidateL1DataSource validates an L1 data source
func ValidateL1DataSource(source string) error {
	return CheckMicrostructureDataSource(source, "L1", nil)
}

// ValidateL2DataSource validates an L2 data source
func ValidateL2DataSource(source string) error {
	return CheckMicrostructureDataSource(source, "L2", nil)
}

// ValidateOrderBookSource validates an order book data source
func ValidateOrderBookSource(source string) error {
	return CheckMicrostructureDataSource(source, "orderbook", nil)
}

// ValidateTickerSource validates a ticker data source
func ValidateTickerSource(source string) error {
	return CheckMicrostructureDataSource(source, "ticker", nil)
}

// MustBeExchangeNative panics if source is not exchange-native
// Use this for critical paths where aggregator usage would be catastrophic
func MustBeExchangeNative(source string) {
	if err := ValidateExchangeNativeSource(source); err != nil {
		panic(fmt.Sprintf("CRITICAL: %v", err))
	}
}

// RuntimeAggregatorGuard provides runtime enforcement of aggregator ban
type RuntimeAggregatorGuard struct {
	enabled    bool
	violations []AggregatorBanError
	strictMode bool // Panic on violations vs log and continue
}

// NewRuntimeAggregatorGuard creates a new runtime guard
func NewRuntimeAggregatorGuard(strictMode bool) *RuntimeAggregatorGuard {
	return &RuntimeAggregatorGuard{
		enabled:    true,
		violations: make([]AggregatorBanError, 0),
		strictMode: strictMode,
	}
}

// CheckSource validates a source against the aggregator ban
func (g *RuntimeAggregatorGuard) CheckSource(source string) error {
	if !g.enabled {
		return nil
	}

	if err := ValidateExchangeNativeSource(source); err != nil {
		if banErr, ok := err.(*AggregatorBanError); ok {
			g.violations = append(g.violations, *banErr)

			if g.strictMode {
				panic(err)
			}
		}
		return err
	}

	return nil
}

// GetViolations returns all recorded violations
func (g *RuntimeAggregatorGuard) GetViolations() []AggregatorBanError {
	return g.violations
}

// ClearViolations clears the violation history
func (g *RuntimeAggregatorGuard) ClearViolations() {
	g.violations = make([]AggregatorBanError, 0)
}

// Disable temporarily disables the guard (for testing only)
func (g *RuntimeAggregatorGuard) Disable() {
	g.enabled = false
}

// Enable re-enables the guard
func (g *RuntimeAggregatorGuard) Enable() {
	g.enabled = true
}

// IsAggregatorBanned checks if a source is in the banned list without throwing errors
func IsAggregatorBanned(source string) bool {
	return GuardAgainstAggregator(source) != nil
}
