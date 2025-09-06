package microstructure

import (
	"context"
	"fmt"
	"time"

	"cryptorun/internal/data/venue/types"
)

// Checker validates microstructure requirements for exchange-native assets
type Checker struct {
	config *Config
}

// Config defines microstructure validation thresholds
type Config struct {
	MaxSpreadBPS     float64 // Default: 50 bps
	MinDepthUSD      float64 // Default: $100,000
	MinVADR          float64 // Default: 1.75x
	RequireAllVenues bool    // Default: false (any venue passing is sufficient)
}

// DefaultConfig returns PRD-compliant microstructure requirements
func DefaultConfig() *Config {
	return &Config{
		MaxSpreadBPS:     50.0,   // < 50 bps spread requirement
		MinDepthUSD:      100000, // >= $100k depth within ±2%
		MinVADR:          1.75,   // >= 1.75× VADR requirement
		RequireAllVenues: false,  // Any venue passing is sufficient
	}
}

// NewChecker creates a microstructure checker with given config
func NewChecker(config *Config) *Checker {
	if config == nil {
		config = DefaultConfig()
	}
	return &Checker{config: config}
}

// ValidateOrderBook checks if orderbook meets microstructure requirements
func (c *Checker) ValidateOrderBook(ctx context.Context, orderBook *types.OrderBook, vadr, adv float64) *types.MicrostructureMetrics {
	metrics := &types.MicrostructureMetrics{
		Symbol:                orderBook.Symbol,
		Venue:                 orderBook.Venue,
		TimestampMono:         orderBook.TimestampMono,
		SpreadBPS:             orderBook.SpreadBPS,
		DepthUSDPlusMinus2Pct: orderBook.DepthUSDPlusMinus2Pct,
		VADR:                  vadr,
		ADV:                   adv,
		DataSource:            orderBook.Venue,
		CacheHit:              false, // Will be set by caller
		FetchLatencyMs:        0,     // Will be set by caller
	}

	// Validate spread requirement
	metrics.SpreadValid = orderBook.SpreadBPS < c.config.MaxSpreadBPS

	// Validate depth requirement
	metrics.DepthValid = orderBook.DepthUSDPlusMinus2Pct >= c.config.MinDepthUSD

	// Validate VADR requirement
	metrics.VADRValid = vadr >= c.config.MinVADR

	// Overall validation - all individual checks must pass
	metrics.OverallValid = metrics.SpreadValid && metrics.DepthValid && metrics.VADRValid

	return metrics
}

// GenerateProof creates a proof bundle for microstructure validation
func (c *Checker) GenerateProof(ctx context.Context, orderBook *types.OrderBook, metrics *types.MicrostructureMetrics) *types.ProofBundle {
	proofID := fmt.Sprintf("%s_%s_%d", orderBook.Symbol, orderBook.Venue, orderBook.TimestampMono.Unix())

	return &types.ProofBundle{
		AssetSymbol:           orderBook.Symbol,
		TimestampMono:         orderBook.TimestampMono,
		ProvenValid:           metrics.OverallValid,
		OrderBookSnapshot:     orderBook,
		MicrostructureMetrics: metrics,
		SpreadProof: types.ValidationProof{
			Metric:        "spread_bps",
			ActualValue:   orderBook.SpreadBPS,
			RequiredValue: c.config.MaxSpreadBPS,
			Operator:      "<",
			Passed:        metrics.SpreadValid,
			Evidence: fmt.Sprintf("Spread %.2f bps %s required max %.2f bps",
				orderBook.SpreadBPS,
				operatorResult(orderBook.SpreadBPS < c.config.MaxSpreadBPS),
				c.config.MaxSpreadBPS),
		},
		DepthProof: types.ValidationProof{
			Metric:        "depth_usd_plus_minus_2pct",
			ActualValue:   orderBook.DepthUSDPlusMinus2Pct,
			RequiredValue: c.config.MinDepthUSD,
			Operator:      ">=",
			Passed:        metrics.DepthValid,
			Evidence: fmt.Sprintf("Depth $%.0f %s required min $%.0f within ±2%%",
				orderBook.DepthUSDPlusMinus2Pct,
				operatorResult(orderBook.DepthUSDPlusMinus2Pct >= c.config.MinDepthUSD),
				c.config.MinDepthUSD),
		},
		VADRProof: types.ValidationProof{
			Metric:        "vadr",
			ActualValue:   metrics.VADR,
			RequiredValue: c.config.MinVADR,
			Operator:      ">=",
			Passed:        metrics.VADRValid,
			Evidence: fmt.Sprintf("VADR %.2fx %s required min %.2fx",
				metrics.VADR,
				operatorResult(metrics.VADR >= c.config.MinVADR),
				c.config.MinVADR),
		},
		ProofGeneratedAt: time.Now(),
		VenueUsed:        orderBook.Venue,
		ProofID:          proofID,
	}
}

// CheckAssetEligibility validates an asset across all available venues
func (c *Checker) CheckAssetEligibility(ctx context.Context, symbol string, venueClients map[string]VenueClient) (*AssetEligibilityResult, error) {
	result := &AssetEligibilityResult{
		Symbol:         symbol,
		CheckedAt:      time.Now(),
		VenueResults:   make(map[string]*types.MicrostructureMetrics),
		ProofBundles:   make(map[string]*types.ProofBundle),
		EligibleVenues: []string{},
	}

	// Check each venue
	for venueName, client := range venueClients {
		orderBook, err := client.FetchOrderBook(ctx, symbol)
		if err != nil {
			result.VenueErrors = append(result.VenueErrors, fmt.Sprintf("%s: %v", venueName, err))
			continue
		}

		// For demo purposes, using mock VADR/ADV values
		// In production, these would come from separate data sources
		vadr := 2.1   // Mock VADR > 1.75
		adv := 500000 // Mock ADV

		// Validate microstructure
		metrics := c.ValidateOrderBook(ctx, orderBook, vadr, adv)
		result.VenueResults[venueName] = metrics

		// Generate proof
		proof := c.GenerateProof(ctx, orderBook, metrics)
		result.ProofBundles[venueName] = proof

		// Track eligible venues
		if metrics.OverallValid {
			result.EligibleVenues = append(result.EligibleVenues, venueName)
		}
	}

	// Determine overall eligibility
	if c.config.RequireAllVenues {
		// All venues must pass
		result.OverallEligible = len(result.EligibleVenues) == len(venueClients) && len(result.VenueErrors) == 0
	} else {
		// Any venue passing is sufficient
		result.OverallEligible = len(result.EligibleVenues) > 0
	}

	return result, nil
}

// VenueClient interface for orderbook fetching
type VenueClient interface {
	FetchOrderBook(ctx context.Context, symbol string) (*types.OrderBook, error)
}

// AssetEligibilityResult contains microstructure check results across all venues
type AssetEligibilityResult struct {
	Symbol          string                                  `json:"symbol"`
	CheckedAt       time.Time                               `json:"checked_at"`
	OverallEligible bool                                    `json:"overall_eligible"`
	EligibleVenues  []string                                `json:"eligible_venues"`
	VenueResults    map[string]*types.MicrostructureMetrics `json:"venue_results"`
	ProofBundles    map[string]*types.ProofBundle           `json:"proof_bundles"`
	VenueErrors     []string                                `json:"venue_errors"`
}

// operatorResult returns a human-readable string for boolean operations
func operatorResult(passed bool) string {
	if passed {
		return "meets"
	}
	return "fails"
}
