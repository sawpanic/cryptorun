package catalyst

import (
	"fmt"
	"math"
	"time"
)

// EventTier represents the urgency/impact tier of a catalyst event
type EventTier string

const (
	TierImminent EventTier = "imminent"   // 0-7 days: earnings, major releases
	TierNearTerm EventTier = "near_term"  // 7-30 days: scheduled updates, forks
	TierMedium   EventTier = "medium"     // 30-90 days: roadmap milestones
	TierDistant  EventTier = "distant"    // 90+ days: long-term developments
)

// CatalystEvent represents a single catalyst event
type CatalystEvent struct {
	ID          string    `json:"id"`          // Unique identifier
	Symbol      string    `json:"symbol"`      // Asset symbol (e.g., "BTCUSD")
	Title       string    `json:"title"`       // Human-readable title
	Description string    `json:"description"` // Detailed description
	EventTime   time.Time `json:"event_time"`  // When the event occurs/occurred
	Tier        EventTier `json:"tier"`        // Impact tier
	Source      string    `json:"source"`      // Data source (must respect robots.txt)
	Confidence  float64   `json:"confidence"`  // 0.0-1.0 confidence in event
	
	// Metadata
	CreatedAt time.Time `json:"created_at"` // When event was added to registry
	UpdatedAt time.Time `json:"updated_at"` // Last update time
	Tags      []string  `json:"tags"`       // Optional tags for categorization
}

// EventRegistry manages catalyst events with time-decay functionality
type EventRegistry struct {
	events  map[string][]CatalystEvent // Symbol -> Events mapping
	sources []EventSource              // Configured data sources
	config  RegistryConfig             // Configuration
}

// RegistryConfig holds configuration for the event registry
type RegistryConfig struct {
	// Time decay parameters
	DecayHalfLife time.Duration `yaml:"decay_half_life"` // Half-life for time decay (default: 7 days)
	MaxLookAhead  time.Duration `yaml:"max_look_ahead"`  // Max future event horizon (default: 90 days)
	MaxLookBehind time.Duration `yaml:"max_look_behind"` // Max past event relevance (default: 30 days)
	
	// Tier weight multipliers
	TierWeights map[EventTier]float64 `yaml:"tier_weights"`
	
	// Source configuration
	MaxSourcesPerSymbol int           `yaml:"max_sources_per_symbol"` // Max events per symbol
	SourceTimeouts      time.Duration `yaml:"source_timeout"`         // HTTP timeout for sources
	RespectRobotsTxt    bool          `yaml:"respect_robots_txt"`     // Honor robots.txt (default: true)
}

// DefaultRegistryConfig returns sensible defaults
func DefaultRegistryConfig() RegistryConfig {
	return RegistryConfig{
		DecayHalfLife: 7 * 24 * time.Hour, // 7 days
		MaxLookAhead:  90 * 24 * time.Hour, // 90 days
		MaxLookBehind: 30 * 24 * time.Hour, // 30 days
		TierWeights: map[EventTier]float64{
			TierImminent: 1.2, // 20% boost for imminent events
			TierNearTerm: 1.0, // Baseline weight
			TierMedium:   0.8, // 20% reduction for medium-term
			TierDistant:  0.6, // 40% reduction for distant events
		},
		MaxSourcesPerSymbol: 50,
		SourceTimeouts:      30 * time.Second,
		RespectRobotsTxt:    true,
	}
}

// NewEventRegistry creates a new catalyst event registry
func NewEventRegistry(config RegistryConfig) *EventRegistry {
	return &EventRegistry{
		events:  make(map[string][]CatalystEvent),
		sources: make([]EventSource, 0),
		config:  config,
	}
}

// AddEvent adds a new catalyst event to the registry
func (er *EventRegistry) AddEvent(event CatalystEvent) error {
	// Validate event
	if err := er.validateEvent(event); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}
	
	// Set timestamps
	now := time.Now()
	event.CreatedAt = now
	event.UpdatedAt = now
	
	// Add to registry
	if er.events[event.Symbol] == nil {
		er.events[event.Symbol] = make([]CatalystEvent, 0)
	}
	
	er.events[event.Symbol] = append(er.events[event.Symbol], event)
	
	// Enforce per-symbol limits
	er.cleanupEventsForSymbol(event.Symbol)
	
	return nil
}

// GetEventsForSymbol retrieves active events for a symbol with time decay weighting
func (er *EventRegistry) GetEventsForSymbol(symbol string, atTime time.Time) []WeightedEvent {
	events, exists := er.events[symbol]
	if !exists {
		return nil
	}
	
	var weightedEvents []WeightedEvent
	
	for _, event := range events {
		// Check if event is within relevant time window
		if !er.isEventRelevant(event, atTime) {
			continue
		}
		
		// Calculate time-decay weight
		weight := er.calculateTimeDecayWeight(event, atTime)
		if weight <= 0 {
			continue
		}
		
		weightedEvents = append(weightedEvents, WeightedEvent{
			Event:  event,
			Weight: weight,
		})
	}
	
	return weightedEvents
}

// GetCatalystSignal computes aggregated catalyst signal for a symbol at a specific time
func (er *EventRegistry) GetCatalystSignal(symbol string, atTime time.Time) CatalystSignal {
	weightedEvents := er.GetEventsForSymbol(symbol, atTime)
	
	if len(weightedEvents) == 0 {
		return CatalystSignal{
			Symbol:      symbol,
			Timestamp:   atTime,
			Signal:      0.0,
			EventCount:  0,
			MaxWeight:   0.0,
			TotalWeight: 0.0,
		}
	}
	
	// Aggregate weights with diminishing returns
	totalWeight := 0.0
	maxWeight := 0.0
	signal := 0.0
	
	for _, we := range weightedEvents {
		totalWeight += we.Weight
		if we.Weight > maxWeight {
			maxWeight = we.Weight
		}
	}
	
	// Apply logarithmic scaling to prevent single events from dominating
	// signal = log(1 + totalWeight) / log(1 + maxPossibleWeight)
	maxPossibleWeight := er.config.TierWeights[TierImminent] * float64(len(weightedEvents))
	if maxPossibleWeight > 0 {
		signal = math.Log(1+totalWeight) / math.Log(1+maxPossibleWeight)
	}
	
	// Clamp to [0, 1] range
	signal = math.Max(0.0, math.Min(1.0, signal))
	
	return CatalystSignal{
		Symbol:      symbol,
		Timestamp:   atTime,
		Signal:      signal,
		EventCount:  len(weightedEvents),
		MaxWeight:   maxWeight,
		TotalWeight: totalWeight,
		Events:      weightedEvents,
	}
}

// WeightedEvent represents an event with its calculated time-decay weight
type WeightedEvent struct {
	Event  CatalystEvent `json:"event"`
	Weight float64       `json:"weight"` // Time-decayed weight
}

// CatalystSignal represents the aggregated catalyst signal for a symbol
type CatalystSignal struct {
	Symbol      string          `json:"symbol"`
	Timestamp   time.Time       `json:"timestamp"`
	Signal      float64         `json:"signal"`       // 0.0-1.0 aggregated signal
	EventCount  int             `json:"event_count"`  // Number of relevant events
	MaxWeight   float64         `json:"max_weight"`   // Highest individual event weight
	TotalWeight float64         `json:"total_weight"` // Sum of all event weights
	Events      []WeightedEvent `json:"events"`       // Individual weighted events
}

// calculateTimeDecayWeight computes the time-decay weight for an event
func (er *EventRegistry) calculateTimeDecayWeight(event CatalystEvent, atTime time.Time) float64 {
	// Get base tier weight
	tierWeight, exists := er.config.TierWeights[event.Tier]
	if !exists {
		tierWeight = 1.0 // Default weight
	}
	
	// Calculate time difference
	timeDiff := event.EventTime.Sub(atTime)
	absDiff := time.Duration(math.Abs(float64(timeDiff)))
	
	// Apply exponential decay: weight = e^(-ln(2) * t / half_life)
	decayFactor := math.Exp(-math.Ln2 * float64(absDiff) / float64(er.config.DecayHalfLife))
	
	// Apply confidence factor
	confidenceWeight := event.Confidence
	if confidenceWeight <= 0 {
		confidenceWeight = 1.0
	}
	
	// Combine all factors
	finalWeight := tierWeight * decayFactor * confidenceWeight
	
	return finalWeight
}

// isEventRelevant checks if an event is within the relevant time window
func (er *EventRegistry) isEventRelevant(event CatalystEvent, atTime time.Time) bool {
	timeDiff := event.EventTime.Sub(atTime)
	
	// Check future events (positive timeDiff)
	if timeDiff > 0 && timeDiff <= er.config.MaxLookAhead {
		return true
	}
	
	// Check past events (negative timeDiff)
	if timeDiff < 0 && -timeDiff <= er.config.MaxLookBehind {
		return true
	}
	
	return false
}

// validateEvent performs basic validation on a catalyst event
func (er *EventRegistry) validateEvent(event CatalystEvent) error {
	if event.ID == "" {
		return fmt.Errorf("event ID cannot be empty")
	}
	
	if event.Symbol == "" {
		return fmt.Errorf("event symbol cannot be empty")
	}
	
	if event.Title == "" {
		return fmt.Errorf("event title cannot be empty")
	}
	
	if event.EventTime.IsZero() {
		return fmt.Errorf("event time cannot be zero")
	}
	
	// Validate tier
	validTiers := []EventTier{TierImminent, TierNearTerm, TierMedium, TierDistant}
	validTier := false
	for _, tier := range validTiers {
		if event.Tier == tier {
			validTier = true
			break
		}
	}
	if !validTier {
		return fmt.Errorf("invalid event tier: %s", event.Tier)
	}
	
	// Validate confidence
	if event.Confidence < 0.0 || event.Confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0, got %.2f", event.Confidence)
	}
	
	return nil
}

// cleanupEventsForSymbol removes old/excess events for a symbol
func (er *EventRegistry) cleanupEventsForSymbol(symbol string) {
	events, exists := er.events[symbol]
	if !exists {
		return
	}
	
	now := time.Now()
	var validEvents []CatalystEvent
	
	// Filter out events outside the relevant time window
	for _, event := range events {
		if er.isEventRelevant(event, now) {
			validEvents = append(validEvents, event)
		}
	}
	
	// Enforce max events per symbol (keep most recent)
	if len(validEvents) > er.config.MaxSourcesPerSymbol {
		// Sort by creation time (most recent first)
		// For now, just truncate - in production, implement proper sorting
		validEvents = validEvents[:er.config.MaxSourcesPerSymbol]
	}
	
	er.events[symbol] = validEvents
}

// EventSource represents a data source for catalyst events
type EventSource interface {
	GetName() string
	GetEvents(symbol string) ([]CatalystEvent, error)
	RespectRobotsTxt() bool
}