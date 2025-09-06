package factors

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"cryptorun/src/domain/catalyst"
	catalystsrc "cryptorun/src/infrastructure/catalyst"
)

// CatalystFactor computes catalyst heat for symbols
type CatalystFactor struct {
	client     *catalystsrc.CatalystClient
	calculator *catalyst.HeatCalculator
	config     catalyst.HeatConfig
}

// NewCatalystFactor creates catalyst factor calculator
func NewCatalystFactor(client *catalystsrc.CatalystClient, config catalyst.HeatConfig) *CatalystFactor {
	return &CatalystFactor{
		client:     client,
		calculator: catalyst.NewHeatCalculator(config),
		config:     config,
	}
}

// Calculate computes catalyst heat score for given symbols
func (cf *CatalystFactor) Calculate(ctx context.Context, symbols []string) (map[string]float64, error) {
	log.Info().
		Strs("symbols", symbols).
		Str("aggregation", cf.config.AggregationMethod).
		Msg("Computing catalyst heat factors")

	// Fetch events from all sources
	rawEvents, err := cf.client.GetEvents(ctx, symbols)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch catalyst events: %w", err)
	}

	// Convert raw events to domain events and group by symbol
	eventsBySymbol := cf.groupEventsBySymbol(rawEvents)

	// Calculate heat for each symbol
	results := make(map[string]float64)
	now := time.Now().UTC()

	for _, symbol := range symbols {
		events, exists := eventsBySymbol[symbol]
		if !exists {
			// No events for this symbol
			results[symbol] = 50.0 // Neutral score (midpoint of 0-100)
			continue
		}

		// Compute heat score
		heat := cf.calculator.Heat(events, now)
		results[symbol] = heat

		log.Debug().
			Str("symbol", symbol).
			Int("events", len(events)).
			Float64("heat", heat).
			Msg("Catalyst heat calculated for symbol")
	}

	log.Info().
		Int("total_events", len(rawEvents)).
		Int("symbols_processed", len(results)).
		Msg("Catalyst heat factors computed")

	return results, nil
}

// CalculateWithAnalysis computes heat and returns detailed analysis
func (cf *CatalystFactor) CalculateWithAnalysis(ctx context.Context, symbols []string) (map[string]float64, map[string]catalyst.HeatAnalysis, error) {
	// Get raw events
	rawEvents, err := cf.client.GetEvents(ctx, symbols)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch catalyst events: %w", err)
	}

	// Group by symbol
	eventsBySymbol := cf.groupEventsBySymbol(rawEvents)

	// Calculate heat and analysis for each symbol
	results := make(map[string]float64)
	analyses := make(map[string]catalyst.HeatAnalysis)
	now := time.Now().UTC()

	for _, symbol := range symbols {
		events, exists := eventsBySymbol[symbol]
		if !exists {
			results[symbol] = 50.0 // Neutral
			analyses[symbol] = catalyst.HeatAnalysis{
				TotalHeat:      50.0,
				EventCount:     0,
				BucketCounts:   make(map[string]int),
				TierCounts:     make(map[int]int),
				PolarityCounts: make(map[int]int),
				EventDetails:   []catalyst.EventHeatDetail{},
			}
			continue
		}

		// Compute heat and analysis
		heat := cf.calculator.Heat(events, now)
		analysis := cf.calculator.AnalyzeHeat(events, now)

		results[symbol] = heat
		analyses[symbol] = analysis
	}

	return results, analyses, nil
}

// groupEventsBySymbol organizes raw events by symbol after normalization
func (cf *CatalystFactor) groupEventsBySymbol(rawEvents []catalystsrc.RawEvent) map[string][]catalyst.Event {
	eventsBySymbol := make(map[string][]catalyst.Event)

	for _, rawEvent := range rawEvents {
		// Normalize symbol
		normalizedSymbol := cf.client.NormalizeSymbol(rawEvent.Symbol, rawEvent.Source)

		// Convert to domain event
		event := catalyst.Event{
			ID:         rawEvent.ID,
			Symbol:     normalizedSymbol,
			Title:      rawEvent.Title,
			Date:       rawEvent.Date,
			Tier:       rawEvent.Tier,
			Polarity:   rawEvent.Polarity,
			Source:     rawEvent.Source,
			Categories: rawEvent.Categories,
		}

		// Group by symbol
		eventsBySymbol[normalizedSymbol] = append(eventsBySymbol[normalizedSymbol], event)
	}

	return eventsBySymbol
}

// GetEventsForSymbol returns events for a specific symbol (useful for debugging)
func (cf *CatalystFactor) GetEventsForSymbol(ctx context.Context, symbol string) ([]catalyst.Event, error) {
	rawEvents, err := cf.client.GetEvents(ctx, []string{symbol})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch events for %s: %w", symbol, err)
	}

	eventsBySymbol := cf.groupEventsBySymbol(rawEvents)
	events, exists := eventsBySymbol[symbol]
	if !exists {
		return []catalyst.Event{}, nil
	}

	return events, nil
}

// ValidateConfiguration checks catalyst configuration for common issues
func (cf *CatalystFactor) ValidateConfiguration() error {
	config := cf.config

	// Check multiplier ranges
	if config.ImminentMultiplier < 0 || config.ImminentMultiplier > 2.0 {
		return fmt.Errorf("imminent multiplier %.2f outside reasonable range [0, 2.0]", config.ImminentMultiplier)
	}

	if config.NearTermMultiplier < 0 || config.NearTermMultiplier > 2.0 {
		return fmt.Errorf("near-term multiplier %.2f outside reasonable range [0, 2.0]", config.NearTermMultiplier)
	}

	if config.MediumMultiplier < 0 || config.MediumMultiplier > 2.0 {
		return fmt.Errorf("medium multiplier %.2f outside reasonable range [0, 2.0]", config.MediumMultiplier)
	}

	if config.DistantMultiplier < 0 || config.DistantMultiplier > 2.0 {
		return fmt.Errorf("distant multiplier %.2f outside reasonable range [0, 2.0]", config.DistantMultiplier)
	}

	// Check bucket boundaries
	if config.ImminentWeeks <= 0 || config.ImminentWeeks >= config.NearTermWeeks {
		return fmt.Errorf("imminent weeks %.1f must be positive and less than near-term %.1f",
			config.ImminentWeeks, config.NearTermWeeks)
	}

	if config.NearTermWeeks <= config.ImminentWeeks || config.NearTermWeeks >= config.MediumWeeks {
		return fmt.Errorf("near-term weeks %.1f must be between imminent %.1f and medium %.1f",
			config.NearTermWeeks, config.ImminentWeeks, config.MediumWeeks)
	}

	if config.MediumWeeks <= config.NearTermWeeks {
		return fmt.Errorf("medium weeks %.1f must be greater than near-term %.1f",
			config.MediumWeeks, config.NearTermWeeks)
	}

	// Check tier weights
	if config.MajorTierWeight < 0 || config.MajorTierWeight > 2.0 {
		return fmt.Errorf("major tier weight %.2f outside reasonable range [0, 2.0]", config.MajorTierWeight)
	}

	if config.MinorTierWeight < 0 || config.MinorTierWeight > 2.0 {
		return fmt.Errorf("minor tier weight %.2f outside reasonable range [0, 2.0]", config.MinorTierWeight)
	}

	if config.InfoTierWeight < 0 || config.InfoTierWeight > 2.0 {
		return fmt.Errorf("info tier weight %.2f outside reasonable range [0, 2.0]", config.InfoTierWeight)
	}

	// Check aggregation method
	if config.AggregationMethod != "max" && config.AggregationMethod != "smooth" {
		return fmt.Errorf("aggregation method '%s' must be 'max' or 'smooth'", config.AggregationMethod)
	}

	log.Info().
		Float64("imminent_mult", config.ImminentMultiplier).
		Float64("near_term_mult", config.NearTermMultiplier).
		Float64("medium_mult", config.MediumMultiplier).
		Float64("distant_mult", config.DistantMultiplier).
		Float64("imminent_weeks", config.ImminentWeeks).
		Float64("near_term_weeks", config.NearTermWeeks).
		Float64("medium_weeks", config.MediumWeeks).
		Str("aggregation", config.AggregationMethod).
		Msg("Catalyst configuration validated")

	return nil
}

// GetConfiguration returns current catalyst configuration
func (cf *CatalystFactor) GetConfiguration() catalyst.HeatConfig {
	return cf.config
}

// GetSupportedBuckets returns list of time buckets with their ranges
func (cf *CatalystFactor) GetSupportedBuckets() map[string]string {
	return map[string]string{
		"imminent":  fmt.Sprintf("0-%.0fw", cf.config.ImminentWeeks),
		"near-term": fmt.Sprintf("%.0f-%.0fw", cf.config.ImminentWeeks, cf.config.NearTermWeeks),
		"medium":    fmt.Sprintf("%.0f-%.0fw", cf.config.NearTermWeeks, cf.config.MediumWeeks),
		"distant":   fmt.Sprintf("%.0fw+", cf.config.MediumWeeks),
	}
}
