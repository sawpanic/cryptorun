package microstructure

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sawpanic/cryptorun/internal/data/venue/binance"
	"github.com/sawpanic/cryptorun/internal/data/venue/coinbase"
	"github.com/sawpanic/cryptorun/internal/data/venue/okx"
	"github.com/sawpanic/cryptorun/internal/data/venue/types"
)

// Checker validates microstructure requirements across exchange-native venues
type Checker struct {
	binanceClient  *binance.OrderBookClient
	okxClient      *okx.OrderBookClient
	coinbaseClient *coinbase.OrderBookClient

	// Validation thresholds
	maxSpreadBPS float64
	minDepthUSD  float64
	minVADR      float64

	// Configuration
	requireAllVenues bool
	venues           []string
}

// NewChecker creates a microstructure validation checker
func NewChecker() *Checker {
	return &Checker{
		binanceClient:  binance.NewOrderBookClient(),
		okxClient:      okx.NewOrderBookClient(),
		coinbaseClient: coinbase.NewOrderBookClient(),

		// Default PRD requirements
		maxSpreadBPS: 50.0,   // < 50 bps
		minDepthUSD:  100000, // >= $100k @ ±2%
		minVADR:      1.75,   // >= 1.75x

		requireAllVenues: false,
		venues:           []string{"binance", "okx", "coinbase"},
	}
}

// SetThresholds updates validation thresholds
func (c *Checker) SetThresholds(maxSpreadBPS, minDepthUSD, minVADR float64) {
	c.maxSpreadBPS = maxSpreadBPS
	c.minDepthUSD = minDepthUSD
	c.minVADR = minVADR
}

// SetVenues configures which venues to check
func (c *Checker) SetVenues(venues []string, requireAll bool) {
	c.venues = venues
	c.requireAllVenues = requireAll
}

// ValidateAsset checks microstructure eligibility for a single asset
func (c *Checker) ValidateAsset(ctx context.Context, symbol string) (*ValidationResult, error) {
	log.Info().
		Str("symbol", symbol).
		Strs("venues", c.venues).
		Float64("max_spread_bps", c.maxSpreadBPS).
		Float64("min_depth_usd", c.minDepthUSD).
		Float64("min_vadr", c.minVADR).
		Msg("Starting microstructure validation")

	result := &ValidationResult{
		Symbol:         symbol,
		TimestampMono:  time.Now(),
		VenueResults:   make(map[string]*VenueValidation),
		EligibleVenues: []string{},
		FailedVenues:   []string{},
	}

	// Validate on each requested venue
	for _, venue := range c.venues {
		venueResult, err := c.validateOnVenue(ctx, symbol, venue)
		if err != nil {
			log.Error().
				Str("symbol", symbol).
				Str("venue", venue).
				Err(err).
				Msg("Failed to validate on venue")

			result.VenueResults[venue] = &VenueValidation{
				Venue: venue,
				Valid: false,
				Error: err.Error(),
			}
			result.FailedVenues = append(result.FailedVenues, venue)
			continue
		}

		result.VenueResults[venue] = venueResult

		if venueResult.Valid {
			result.EligibleVenues = append(result.EligibleVenues, venue)
			result.PassedVenueCount++
		} else {
			result.FailedVenues = append(result.FailedVenues, venue)
		}
		result.TotalVenueCount++
	}

	// Determine overall eligibility
	if c.requireAllVenues {
		result.OverallValid = result.PassedVenueCount == result.TotalVenueCount
	} else {
		result.OverallValid = result.PassedVenueCount > 0
	}

	log.Info().
		Str("symbol", symbol).
		Int("passed_venues", result.PassedVenueCount).
		Int("total_venues", result.TotalVenueCount).
		Bool("overall_valid", result.OverallValid).
		Strs("eligible_venues", result.EligibleVenues).
		Msg("Microstructure validation completed")

	return result, nil
}

// validateOnVenue performs validation on a specific venue
func (c *Checker) validateOnVenue(ctx context.Context, symbol, venue string) (*VenueValidation, error) {
	startTime := time.Now()

	// Fetch orderbook from venue
	var orderBook *types.OrderBook
	var err error

	switch venue {
	case "binance":
		orderBook, err = c.binanceClient.FetchOrderBook(ctx, symbol)
	case "okx":
		orderBook, err = c.okxClient.FetchOrderBook(ctx, symbol)
	case "coinbase":
		orderBook, err = c.coinbaseClient.FetchOrderBook(ctx, symbol)
	default:
		return nil, fmt.Errorf("unsupported venue: %s", venue)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch orderbook from %s: %w", venue, err)
	}

	// Calculate VADR (placeholder - would need historical volume data)
	vadr := c.calculateVADR(orderBook)

	// Create microstructure metrics
	metrics := &types.MicrostructureMetrics{
		Symbol:                symbol,
		Venue:                 venue,
		TimestampMono:         orderBook.TimestampMono,
		SpreadBPS:             orderBook.SpreadBPS,
		DepthUSDPlusMinus2Pct: orderBook.DepthUSDPlusMinus2Pct,
		VADR:                  vadr,
		DataSource:            venue,
		FetchLatencyMs:        time.Since(startTime).Milliseconds(),
	}

	// Validate against thresholds
	spreadValid := metrics.SpreadBPS < c.maxSpreadBPS
	depthValid := metrics.DepthUSDPlusMinus2Pct >= c.minDepthUSD
	vadrValid := metrics.VADR >= c.minVADR
	overallValid := spreadValid && depthValid && vadrValid

	metrics.SpreadValid = spreadValid
	metrics.DepthValid = depthValid
	metrics.VADRValid = vadrValid
	metrics.OverallValid = overallValid

	// Build failure reasons
	var failureReasons []string
	if !spreadValid {
		failureReasons = append(failureReasons,
			fmt.Sprintf("Spread %.1fbps > %.1fbps limit", metrics.SpreadBPS, c.maxSpreadBPS))
	}
	if !depthValid {
		failureReasons = append(failureReasons,
			fmt.Sprintf("Depth $%.0fk < $%.0fk limit", metrics.DepthUSDPlusMinus2Pct/1000, c.minDepthUSD/1000))
	}
	if !vadrValid {
		failureReasons = append(failureReasons,
			fmt.Sprintf("VADR %.2fx < %.2fx limit", metrics.VADR, c.minVADR))
	}

	return &VenueValidation{
		Venue:          venue,
		Valid:          overallValid,
		OrderBook:      orderBook,
		Metrics:        metrics,
		FailureReasons: failureReasons,
		FetchLatencyMs: time.Since(startTime).Milliseconds(),
	}, nil
}

// calculateVADR computes Volume-Adjusted Daily Range (placeholder implementation)
func (c *Checker) calculateVADR(orderBook *types.OrderBook) float64 {
	// TODO: Implement proper VADR calculation with historical volume data
	// For now, return a placeholder based on spread as a rough approximation
	// Real implementation would need 24h volume and daily range data

	// Rough approximation: wider spread = lower VADR
	if orderBook.SpreadBPS > 100 {
		return 1.0 // Low VADR for very wide spreads
	} else if orderBook.SpreadBPS > 50 {
		return 1.5 // Medium VADR for moderate spreads
	} else {
		return 2.0 // High VADR for tight spreads
	}
}

// ValidationResult contains the results of asset validation across venues
type ValidationResult struct {
	Symbol           string                      `json:"symbol"`
	TimestampMono    time.Time                   `json:"timestamp_mono"`
	OverallValid     bool                        `json:"overall_valid"`
	PassedVenueCount int                         `json:"passed_venue_count"`
	TotalVenueCount  int                         `json:"total_venue_count"`
	EligibleVenues   []string                    `json:"eligible_venues"`
	FailedVenues     []string                    `json:"failed_venues"`
	VenueResults     map[string]*VenueValidation `json:"venue_results"`
}

// VenueValidation contains validation results for a specific venue
type VenueValidation struct {
	Venue          string                       `json:"venue"`
	Valid          bool                         `json:"valid"`
	OrderBook      *types.OrderBook             `json:"order_book,omitempty"`
	Metrics        *types.MicrostructureMetrics `json:"metrics,omitempty"`
	FailureReasons []string                     `json:"failure_reasons,omitempty"`
	Error          string                       `json:"error,omitempty"`
	FetchLatencyMs int64                        `json:"fetch_latency_ms"`
}

// GetSummary returns a human-readable summary of validation results
func (r *ValidationResult) GetSummary() string {
	if r.OverallValid {
		return fmt.Sprintf("✅ ELIGIBLE - Passed on %d venue(s): %v",
			r.PassedVenueCount, r.EligibleVenues)
	} else {
		return fmt.Sprintf("❌ NOT ELIGIBLE - Failed on %d/%d venues",
			len(r.FailedVenues), r.TotalVenueCount)
	}
}

// GetDetailedReasons returns detailed failure reasons for UI display
func (r *ValidationResult) GetDetailedReasons() map[string][]string {
	reasons := make(map[string][]string)

	for venue, validation := range r.VenueResults {
		if !validation.Valid {
			if validation.Error != "" {
				reasons[venue] = []string{validation.Error}
			} else {
				reasons[venue] = validation.FailureReasons
			}
		}
	}

	return reasons
}
