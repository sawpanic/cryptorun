package catalyst

import (
	"math"
	"time"

	"github.com/rs/zerolog/log"
)

// Event represents a normalized catalyst event for heat calculation
type Event struct {
	ID         string    `json:"id"`
	Symbol     string    `json:"symbol"`
	Title      string    `json:"title"`
	Date       time.Time `json:"date"`
	Tier       int       `json:"tier"`     // 1=Major, 2=Minor, 3=Info
	Polarity   int       `json:"polarity"` // +1=Positive, -1=Negative, 0=Neutral
	Source     string    `json:"source"`
	Categories []string  `json:"categories"`
}

// HeatConfig defines time-decay buckets and tier weights
type HeatConfig struct {
	// Time-decay multipliers per bucket
	ImminentMultiplier float64 `yaml:"imminent_multiplier"`  // 0-4w: 1.2×
	NearTermMultiplier float64 `yaml:"near_term_multiplier"` // 4-8w: 1.0×
	MediumMultiplier   float64 `yaml:"medium_multiplier"`    // 8-16w: 0.8×
	DistantMultiplier  float64 `yaml:"distant_multiplier"`   // 16w+: 0.6×

	// Bucket boundaries in weeks
	ImminentWeeks float64 `yaml:"imminent_weeks"`  // 4.0
	NearTermWeeks float64 `yaml:"near_term_weeks"` // 8.0
	MediumWeeks   float64 `yaml:"medium_weeks"`    // 16.0

	// Tier weights (higher tier = more impact)
	MajorTierWeight float64 `yaml:"major_tier_weight"` // 1.0
	MinorTierWeight float64 `yaml:"minor_tier_weight"` // 0.6
	InfoTierWeight  float64 `yaml:"info_tier_weight"`  // 0.3

	// Aggregation method
	AggregationMethod string `yaml:"aggregation_method"` // "max" or "smooth"
}

// DefaultHeatConfig returns configuration matching PRD requirements
func DefaultHeatConfig() HeatConfig {
	return HeatConfig{
		ImminentMultiplier: 1.2, // 0-4w: 1.2×
		NearTermMultiplier: 1.0, // 4-8w: 1.0×
		MediumMultiplier:   0.8, // 8-16w: 0.8×
		DistantMultiplier:  0.6, // 16w+: 0.6×

		ImminentWeeks: 4.0,  // 4 weeks
		NearTermWeeks: 8.0,  // 8 weeks
		MediumWeeks:   16.0, // 16 weeks

		MajorTierWeight: 1.0, // Major events full weight
		MinorTierWeight: 0.6, // Minor events 60% weight
		InfoTierWeight:  0.3, // Info events 30% weight

		AggregationMethod: "smooth", // Smooth aggregation for multiple events
	}
}

// HeatCalculator computes catalyst heat scores with time-decay
type HeatCalculator struct {
	config HeatConfig
}

// NewHeatCalculator creates calculator with specified config
func NewHeatCalculator(config HeatConfig) *HeatCalculator {
	return &HeatCalculator{
		config: config,
	}
}

// Heat computes 0-100 catalyst heat score for events
func (hc *HeatCalculator) Heat(events []Event, now time.Time) float64 {
	if len(events) == 0 {
		return 0.0
	}

	var totalHeat float64
	eventCount := 0

	for _, event := range events {
		// Skip events in the past (they don't create forward-looking catalyst heat)
		if event.Date.Before(now) {
			continue
		}

		// Calculate time-to-event in weeks
		timeToEvent := event.Date.Sub(now)
		weeksToEvent := timeToEvent.Hours() / (24 * 7) // Convert to weeks

		// Get time-decay multiplier based on bucket
		decayMultiplier := hc.getTimeDecayMultiplier(weeksToEvent)

		// Get tier weight
		tierWeight := hc.getTierWeight(event.Tier)

		// Calculate base event heat
		baseHeat := decayMultiplier * tierWeight * 100.0 // Scale to 0-100 range

		// Apply polarity (negative events invert sign)
		eventHeat := baseHeat * float64(event.Polarity)

		// Aggregate heat based on method
		switch hc.config.AggregationMethod {
		case "max":
			// Take maximum absolute value
			if math.Abs(eventHeat) > math.Abs(totalHeat) {
				totalHeat = eventHeat
			}
		default: // "smooth"
			// Smooth aggregation with diminishing returns
			totalHeat += eventHeat * (1.0 / (1.0 + float64(eventCount)*0.2))
		}

		eventCount++

		log.Debug().
			Str("event_id", event.ID).
			Str("symbol", event.Symbol).
			Float64("weeks_to_event", weeksToEvent).
			Float64("decay_multiplier", decayMultiplier).
			Float64("tier_weight", tierWeight).
			Int("polarity", event.Polarity).
			Float64("event_heat", eventHeat).
			Msg("Catalyst event heat calculated")
	}

	// Normalize to 0-100 range and handle edge cases
	normalizedHeat := hc.normalizeHeat(totalHeat, eventCount)

	log.Info().
		Int("events", eventCount).
		Float64("raw_heat", totalHeat).
		Float64("normalized_heat", normalizedHeat).
		Str("aggregation", hc.config.AggregationMethod).
		Msg("Catalyst heat computed")

	return normalizedHeat
}

// getTimeDecayMultiplier returns multiplier based on time bucket
func (hc *HeatCalculator) getTimeDecayMultiplier(weeksToEvent float64) float64 {
	switch {
	case weeksToEvent <= hc.config.ImminentWeeks:
		return hc.config.ImminentMultiplier // 0-4w: 1.2×
	case weeksToEvent <= hc.config.NearTermWeeks:
		return hc.config.NearTermMultiplier // 4-8w: 1.0×
	case weeksToEvent <= hc.config.MediumWeeks:
		return hc.config.MediumMultiplier // 8-16w: 0.8×
	default:
		return hc.config.DistantMultiplier // 16w+: 0.6×
	}
}

// getTierWeight returns impact weight based on event tier
func (hc *HeatCalculator) getTierWeight(tier int) float64 {
	switch tier {
	case 1: // Major
		return hc.config.MajorTierWeight
	case 2: // Minor
		return hc.config.MinorTierWeight
	case 3: // Info
		return hc.config.InfoTierWeight
	default:
		return hc.config.InfoTierWeight // Default to info weight
	}
}

// normalizeHeat ensures heat score is in 0-100 range
func (hc *HeatCalculator) normalizeHeat(rawHeat float64, eventCount int) float64 {
	if eventCount == 0 {
		return 0.0
	}

	// Apply ceiling to prevent extreme scores
	maxHeat := 120.0  // Allow some headroom above 100
	minHeat := -120.0 // Symmetric range for negative events

	// Clamp to bounds
	normalizedHeat := math.Max(minHeat, math.Min(maxHeat, rawHeat))

	// Scale to 0-100 range (negative events become 0-50, positive become 50-100)
	if normalizedHeat >= 0 {
		// Positive: scale 0-120 to 50-100
		return 50.0 + (normalizedHeat/maxHeat)*50.0
	} else {
		// Negative: scale -120-0 to 0-50
		return 50.0 + (normalizedHeat/minHeat)*50.0
	}
}

// GetTimeBucket returns human-readable bucket for an event
func (hc *HeatCalculator) GetTimeBucket(event Event, now time.Time) string {
	timeToEvent := event.Date.Sub(now)
	weeksToEvent := timeToEvent.Hours() / (24 * 7)

	switch {
	case weeksToEvent <= 0:
		return "past"
	case weeksToEvent <= hc.config.ImminentWeeks:
		return "imminent" // 0-4w
	case weeksToEvent <= hc.config.NearTermWeeks:
		return "near-term" // 4-8w
	case weeksToEvent <= hc.config.MediumWeeks:
		return "medium" // 8-16w
	default:
		return "distant" // 16w+
	}
}

// AnalyzeHeat provides detailed breakdown of heat calculation
type HeatAnalysis struct {
	TotalHeat      float64           `json:"total_heat"`
	EventCount     int               `json:"event_count"`
	BucketCounts   map[string]int    `json:"bucket_counts"`
	TierCounts     map[int]int       `json:"tier_counts"`
	PolarityCounts map[int]int       `json:"polarity_counts"`
	EventDetails   []EventHeatDetail `json:"event_details"`
}

// EventHeatDetail shows heat contribution per event
type EventHeatDetail struct {
	Event           Event   `json:"event"`
	WeeksToEvent    float64 `json:"weeks_to_event"`
	TimeBucket      string  `json:"time_bucket"`
	DecayMultiplier float64 `json:"decay_multiplier"`
	TierWeight      float64 `json:"tier_weight"`
	EventHeat       float64 `json:"event_heat"`
}

// AnalyzeHeat provides detailed heat breakdown for debugging/analysis
func (hc *HeatCalculator) AnalyzeHeat(events []Event, now time.Time) HeatAnalysis {
	analysis := HeatAnalysis{
		BucketCounts:   make(map[string]int),
		TierCounts:     make(map[int]int),
		PolarityCounts: make(map[int]int),
		EventDetails:   []EventHeatDetail{},
	}

	var totalHeat float64
	eventCount := 0

	for _, event := range events {
		if event.Date.Before(now) {
			continue // Skip past events
		}

		timeToEvent := event.Date.Sub(now)
		weeksToEvent := timeToEvent.Hours() / (24 * 7)
		timeBucket := hc.GetTimeBucket(event, now)
		decayMultiplier := hc.getTimeDecayMultiplier(weeksToEvent)
		tierWeight := hc.getTierWeight(event.Tier)

		baseHeat := decayMultiplier * tierWeight * 100.0
		eventHeat := baseHeat * float64(event.Polarity)

		// Aggregate with same method as Heat()
		switch hc.config.AggregationMethod {
		case "max":
			if math.Abs(eventHeat) > math.Abs(totalHeat) {
				totalHeat = eventHeat
			}
		default: // "smooth"
			totalHeat += eventHeat * (1.0 / (1.0 + float64(eventCount)*0.2))
		}

		eventCount++

		// Update counters
		analysis.BucketCounts[timeBucket]++
		analysis.TierCounts[event.Tier]++
		analysis.PolarityCounts[event.Polarity]++

		// Add event detail
		analysis.EventDetails = append(analysis.EventDetails, EventHeatDetail{
			Event:           event,
			WeeksToEvent:    weeksToEvent,
			TimeBucket:      timeBucket,
			DecayMultiplier: decayMultiplier,
			TierWeight:      tierWeight,
			EventHeat:       eventHeat,
		})
	}

	analysis.TotalHeat = hc.normalizeHeat(totalHeat, eventCount)
	analysis.EventCount = eventCount

	return analysis
}
