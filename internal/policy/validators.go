package policy

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ReasonCode represents violation reason codes for clear error reporting
type ReasonCode string

const (
	ReasonNonUSDQuote         ReasonCode = "NON_USD_QUOTE"
	ReasonAggregatorBanned    ReasonCode = "AGGREGATOR_BANNED"
	ReasonStablecoinDepeg     ReasonCode = "STABLECOIN_DEPEG"
	ReasonGlobalPause         ReasonCode = "GLOBAL_PAUSE"
	ReasonSymbolBlacklisted   ReasonCode = "SYMBOL_BLACKLISTED"
	ReasonVenueNotPreferred   ReasonCode = "VENUE_NOT_PREFERRED"
	ReasonEmergencyControl    ReasonCode = "EMERGENCY_CONTROL"
)

// ValidationError contains detailed violation information
type ValidationError struct {
	Reason  ReasonCode
	Symbol  string
	Venue   string
	Message string
	Details map[string]interface{}
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s (symbol=%s, venue=%s)", e.Reason, e.Message, e.Symbol, e.Venue)
}

// PolicyValidator enforces CryptoRun v3.2.1 product policies
type PolicyValidator struct {
	globalPause           bool
	blacklist             map[string]bool
	emergencyControls     map[string]bool
	venuePreferenceOrder  []string
	stablecoinThreshold   float64
	aggregatorBlacklist   []string
}

// NewPolicyValidator creates a validator with default CryptoRun policies
func NewPolicyValidator() *PolicyValidator {
	return &PolicyValidator{
		globalPause:           false,
		blacklist:             make(map[string]bool),
		emergencyControls:     make(map[string]bool),
		venuePreferenceOrder:  []string{"kraken", "binance", "okx", "coinbase"},
		stablecoinThreshold:   0.005, // ±0.5% depeg threshold
		aggregatorBlacklist:   []string{"dexscreener", "coingecko", "cmc", "paprika", "etherscan", "moralis"},
	}
}

// ValidateUSDOnly enforces USD pairs only policy
func (pv *PolicyValidator) ValidateUSDOnly(symbol string) error {
	// Extract quote currency from symbol (assume BASEUSD format)
	if !strings.HasSuffix(strings.ToUpper(symbol), "USD") {
		return ValidationError{
			Reason:  ReasonNonUSDQuote,
			Symbol:  symbol,
			Message: fmt.Sprintf("Only USD pairs allowed, got %s", symbol),
			Details: map[string]interface{}{
				"policy":        "USD_ONLY",
				"violation":     "non-USD quote currency",
				"allowed_quote": "USD",
			},
		}
	}

	log.Debug().Str("symbol", symbol).Msg("USD-only validation passed")
	return nil
}

// ValidateVenuePreference checks venue against preference ordering
func (pv *PolicyValidator) ValidateVenuePreference(venue string, allowFallback bool) error {
	venueLower := strings.ToLower(venue)
	
	// Check if venue is in preference order
	for i, preferred := range pv.venuePreferenceOrder {
		if preferred == venueLower {
			if i > 0 && !allowFallback {
				return ValidationError{
					Reason:  ReasonVenueNotPreferred,
					Venue:   venue,
					Message: fmt.Sprintf("Venue %s not preferred, use %s instead", venue, pv.venuePreferenceOrder[0]),
					Details: map[string]interface{}{
						"policy":           "VENUE_PREFERENCE",
						"preferred_order":  pv.venuePreferenceOrder,
						"venue_position":   i,
						"allow_fallback":   allowFallback,
					},
				}
			}
			
			log.Debug().
				Str("venue", venue).
				Int("preference_rank", i).
				Bool("allow_fallback", allowFallback).
				Msg("Venue preference validation passed")
			return nil
		}
	}

	return ValidationError{
		Reason:  ReasonVenueNotPreferred,
		Venue:   venue,
		Message: fmt.Sprintf("Venue %s not in preference list", venue),
		Details: map[string]interface{}{
			"policy":          "VENUE_PREFERENCE",
			"preferred_order": pv.venuePreferenceOrder,
			"unknown_venue":   venue,
		},
	}
}

// ValidateAggregatorBan enforces exchange-native only policy for microstructure data
func (pv *PolicyValidator) ValidateAggregatorBan(dataSource string, dataType string) error {
	sourceLower := strings.ToLower(dataSource)
	
	// Check if data source is banned aggregator
	for _, banned := range pv.aggregatorBlacklist {
		if banned == sourceLower {
			// Allow aggregators for non-microstructure data
			if !isMicrostructureData(dataType) {
				log.Debug().
					Str("source", dataSource).
					Str("data_type", dataType).
					Msg("Aggregator allowed for non-microstructure data")
				return nil
			}
			
			return ValidationError{
				Reason:  ReasonAggregatorBanned,
				Venue:   dataSource,
				Message: fmt.Sprintf("Aggregator %s banned for microstructure data type %s", dataSource, dataType),
				Details: map[string]interface{}{
					"policy":              "EXCHANGE_NATIVE_ONLY",
					"banned_aggregators":  pv.aggregatorBlacklist,
					"violation":           "aggregator_microstructure_data",
					"data_type":          dataType,
					"allowed_venues":     pv.venuePreferenceOrder,
				},
			}
		}
	}

	log.Debug().
		Str("source", dataSource).
		Str("data_type", dataType).
		Msg("Aggregator ban validation passed")
	return nil
}

// ValidateStablecoinDepeg checks for stablecoin depeg beyond threshold
func (pv *PolicyValidator) ValidateStablecoinDepeg(symbol string, price float64) error {
	if !isStablecoin(symbol) {
		return nil // Not a stablecoin, skip validation
	}

	deviation := math.Abs(price - 1.0)
	if deviation > pv.stablecoinThreshold {
		return ValidationError{
			Reason:  ReasonStablecoinDepeg,
			Symbol:  symbol,
			Message: fmt.Sprintf("Stablecoin %s depegged: price=%.4f, deviation=%.4f%% > %.2f%%", 
				symbol, price, deviation*100, pv.stablecoinThreshold*100),
			Details: map[string]interface{}{
				"policy":         "STABLECOIN_PEG_GUARD",
				"current_price":  price,
				"peg_target":     1.0,
				"deviation":      deviation,
				"threshold":      pv.stablecoinThreshold,
				"deviation_pct":  deviation * 100,
				"threshold_pct":  pv.stablecoinThreshold * 100,
			},
		}
	}

	log.Debug().
		Str("symbol", symbol).
		Float64("price", price).
		Float64("deviation", deviation).
		Msg("Stablecoin depeg validation passed")
	return nil
}

// ValidateEmergencyControls checks global pause and blacklists
func (pv *PolicyValidator) ValidateEmergencyControls(symbol string, venue string) error {
	// Global pause check
	if pv.globalPause {
		return ValidationError{
			Reason:  ReasonGlobalPause,
			Symbol:  symbol,
			Venue:   venue,
			Message: "Global trading pause active",
			Details: map[string]interface{}{
				"policy":      "EMERGENCY_CONTROLS",
				"control":     "global_pause",
				"active":      true,
				"timestamp":   time.Now(),
			},
		}
	}

	// Symbol blacklist check
	if pv.blacklist[strings.ToUpper(symbol)] {
		return ValidationError{
			Reason:  ReasonSymbolBlacklisted,
			Symbol:  symbol,
			Venue:   venue,
			Message: fmt.Sprintf("Symbol %s is blacklisted", symbol),
			Details: map[string]interface{}{
				"policy":         "EMERGENCY_CONTROLS",
				"control":        "symbol_blacklist",
				"blacklisted":    true,
				"timestamp":      time.Now(),
			},
		}
	}

	// Venue-specific emergency control
	emergencyKey := fmt.Sprintf("%s:%s", venue, symbol)
	if pv.emergencyControls[emergencyKey] {
		return ValidationError{
			Reason:  ReasonEmergencyControl,
			Symbol:  symbol,
			Venue:   venue,
			Message: fmt.Sprintf("Emergency control active for %s on %s", symbol, venue),
			Details: map[string]interface{}{
				"policy":         "EMERGENCY_CONTROLS",
				"control":        "venue_symbol_block",
				"emergency_key":  emergencyKey,
				"active":         true,
				"timestamp":      time.Now(),
			},
		}
	}

	log.Debug().
		Str("symbol", symbol).
		Str("venue", venue).
		Bool("global_pause", pv.globalPause).
		Msg("Emergency controls validation passed")
	return nil
}

// ValidateAll runs all policy validations for a trading request
func (pv *PolicyValidator) ValidateAll(symbol, venue, dataSource, dataType string, price float64) error {
	validators := []func() error{
		func() error { return pv.ValidateEmergencyControls(symbol, venue) },
		func() error { return pv.ValidateUSDOnly(symbol) },
		func() error { return pv.ValidateVenuePreference(venue, true) }, // Allow fallback
		func() error { return pv.ValidateAggregatorBan(dataSource, dataType) },
		func() error { return pv.ValidateStablecoinDepeg(symbol, price) },
	}

	for _, validate := range validators {
		if err := validate(); err != nil {
			return err
		}
	}

	log.Info().
		Str("symbol", symbol).
		Str("venue", venue).
		Str("data_source", dataSource).
		Str("data_type", dataType).
		Float64("price", price).
		Msg("All policy validations passed")
	return nil
}

// Emergency control methods
func (pv *PolicyValidator) SetGlobalPause(paused bool) {
	pv.globalPause = paused
	log.Warn().Bool("paused", paused).Msg("Global pause state changed")
}

func (pv *PolicyValidator) AddToBlacklist(symbol string) {
	pv.blacklist[strings.ToUpper(symbol)] = true
	log.Warn().Str("symbol", symbol).Msg("Symbol added to blacklist")
}

func (pv *PolicyValidator) RemoveFromBlacklist(symbol string) {
	delete(pv.blacklist, strings.ToUpper(symbol))
	log.Info().Str("symbol", symbol).Msg("Symbol removed from blacklist")
}

func (pv *PolicyValidator) SetEmergencyControl(venue, symbol string, active bool) {
	key := fmt.Sprintf("%s:%s", venue, symbol)
	if active {
		pv.emergencyControls[key] = true
		log.Warn().Str("venue", venue).Str("symbol", symbol).Msg("Emergency control activated")
	} else {
		delete(pv.emergencyControls, key)
		log.Info().Str("venue", venue).Str("symbol", symbol).Msg("Emergency control deactivated")
	}
}

// GetStatus returns current policy validator status
func (pv *PolicyValidator) GetStatus() map[string]interface{} {
	blacklistedSymbols := make([]string, 0, len(pv.blacklist))
	for symbol := range pv.blacklist {
		blacklistedSymbols = append(blacklistedSymbols, symbol)
	}

	emergencyControls := make([]string, 0, len(pv.emergencyControls))
	for key := range pv.emergencyControls {
		emergencyControls = append(emergencyControls, key)
	}

	return map[string]interface{}{
		"global_pause":           pv.globalPause,
		"blacklisted_symbols":    blacklistedSymbols,
		"emergency_controls":     emergencyControls,
		"venue_preference_order": pv.venuePreferenceOrder,
		"stablecoin_threshold":   pv.stablecoinThreshold,
		"banned_aggregators":     pv.aggregatorBlacklist,
	}
}

// Helper functions
func isMicrostructureData(dataType string) bool {
	microstructureTypes := []string{"depth", "spread", "orderbook", "l1", "l2", "trades", "ticker"}
	dataTypeLower := strings.ToLower(dataType)
	for _, msType := range microstructureTypes {
		if strings.Contains(dataTypeLower, msType) {
			return true
		}
	}
	return false
}

func isStablecoin(symbol string) bool {
	stablecoins := []string{"USDT", "USDC", "BUSD", "DAI", "TUSD", "USDD", "FRAX"}
	symbolUpper := strings.ToUpper(symbol)
	for _, stable := range stablecoins {
		if strings.HasPrefix(symbolUpper, stable) {
			return true
		}
	}
	return false
}