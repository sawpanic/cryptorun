package catalyst

import (
	"fmt"
	"math"
	"time"
)

// TierDecayFunction computes time-decay multipliers for different event tiers
type TierDecayFunction struct {
	config TierDecayConfig
}

// TierDecayConfig holds configuration for tier-based decay calculations
type TierDecayConfig struct {
	// Half-life periods for each tier (how long until weight drops to 50%)
	ImminentHalfLife time.Duration `yaml:"imminent_half_life"` // Default: 2 days
	NearTermHalfLife time.Duration `yaml:"near_term_half_life"` // Default: 7 days  
	MediumHalfLife   time.Duration `yaml:"medium_half_life"`    // Default: 14 days
	DistantHalfLife  time.Duration `yaml:"distant_half_life"`   // Default: 30 days
	
	// Base multipliers for each tier at t=0
	ImminentBase float64 `yaml:"imminent_base"` // Default: 1.2
	NearTermBase float64 `yaml:"near_term_base"` // Default: 1.0
	MediumBase   float64 `yaml:"medium_base"`    // Default: 0.8
	DistantBase  float64 `yaml:"distant_base"`   // Default: 0.6
	
	// Future vs past event weighting
	FutureWeight float64 `yaml:"future_weight"` // Default: 1.0 (future events normal weight)
	PastWeight   float64 `yaml:"past_weight"`   // Default: 0.5 (past events half weight)
}

// DefaultTierDecayConfig returns sensible defaults for tier decay
func DefaultTierDecayConfig() TierDecayConfig {
	return TierDecayConfig{
		ImminentHalfLife: 2 * 24 * time.Hour,  // 2 days
		NearTermHalfLife: 7 * 24 * time.Hour,  // 1 week
		MediumHalfLife:   14 * 24 * time.Hour, // 2 weeks
		DistantHalfLife:  30 * 24 * time.Hour, // 1 month
		
		ImminentBase: 1.2, // 20% boost for imminent events
		NearTermBase: 1.0, // Baseline weight
		MediumBase:   0.8, // 20% reduction
		DistantBase:  0.6, // 40% reduction
		
		FutureWeight: 1.0, // Full weight for future events
		PastWeight:   0.5, // Half weight for past events
	}
}

// NewTierDecayFunction creates a new tier decay function
func NewTierDecayFunction(config TierDecayConfig) *TierDecayFunction {
	return &TierDecayFunction{
		config: config,
	}
}

// CalculateWeight computes the time-decayed weight for an event
func (tdf *TierDecayFunction) CalculateWeight(tier EventTier, eventTime, currentTime time.Time) float64 {
	// Get base weight and half-life for this tier
	baseWeight := tdf.getBaseWeight(tier)
	halfLife := tdf.getHalfLife(tier)
	
	// Calculate time difference
	timeDiff := eventTime.Sub(currentTime)
	absTimeDiff := time.Duration(math.Abs(float64(timeDiff)))
	
	// Calculate exponential decay: weight = base * e^(-ln(2) * t / half_life)
	decayFactor := math.Exp(-math.Ln2 * float64(absTimeDiff) / float64(halfLife))
	
	// Apply future vs past weighting
	futureOrPastWeight := tdf.config.FutureWeight
	if timeDiff < 0 { // Past event
		futureOrPastWeight = tdf.config.PastWeight
	}
	
	// Combine all factors
	finalWeight := baseWeight * decayFactor * futureOrPastWeight
	
	// Ensure non-negative
	return math.Max(0.0, finalWeight)
}

// getBaseWeight returns the base multiplier for a tier
func (tdf *TierDecayFunction) getBaseWeight(tier EventTier) float64 {
	switch tier {
	case TierImminent:
		return tdf.config.ImminentBase
	case TierNearTerm:
		return tdf.config.NearTermBase
	case TierMedium:
		return tdf.config.MediumBase
	case TierDistant:
		return tdf.config.DistantBase
	default:
		return 1.0 // Fallback
	}
}

// getHalfLife returns the half-life duration for a tier
func (tdf *TierDecayFunction) getHalfLife(tier EventTier) time.Duration {
	switch tier {
	case TierImminent:
		return tdf.config.ImminentHalfLife
	case TierNearTerm:
		return tdf.config.NearTermHalfLife
	case TierMedium:
		return tdf.config.MediumHalfLife
	case TierDistant:
		return tdf.config.DistantHalfLife
	default:
		return 7 * 24 * time.Hour // 1 week fallback
	}
}

// GetDecayMultiplier calculates decay multiplier for a given time delta and tier
func (tdf *TierDecayFunction) GetDecayMultiplier(tier EventTier, timeDelta time.Duration) float64 {
	halfLife := tdf.getHalfLife(tier)
	
	// Exponential decay: multiplier = e^(-ln(2) * t / half_life)
	return math.Exp(-math.Ln2 * math.Abs(float64(timeDelta)) / float64(halfLife))
}

// TierAnalysis provides analysis of tier weighting at a specific time
type TierAnalysis struct {
	Timestamp time.Time              `json:"timestamp"`
	TierCounts map[EventTier]int      `json:"tier_counts"`      // Number of events per tier
	TierWeights map[EventTier]float64 `json:"tier_weights"`     // Total weight per tier
	MaxWeight   float64               `json:"max_weight"`       // Highest individual event weight
	TotalWeight float64               `json:"total_weight"`     // Sum of all weights
	DominantTier EventTier            `json:"dominant_tier"`    // Tier with highest total weight
}

// AnalyzeTiers provides comprehensive tier analysis for a set of events
func (tdf *TierDecayFunction) AnalyzeTiers(events []CatalystEvent, currentTime time.Time) TierAnalysis {
	analysis := TierAnalysis{
		Timestamp:   currentTime,
		TierCounts:  make(map[EventTier]int),
		TierWeights: make(map[EventTier]float64),
		MaxWeight:   0.0,
		TotalWeight: 0.0,
		DominantTier: TierNearTerm, // Default
	}
	
	// Analyze each event
	for _, event := range events {
		weight := tdf.CalculateWeight(event.Tier, event.EventTime, currentTime)
		
		// Update counts
		analysis.TierCounts[event.Tier]++
		
		// Update weights
		analysis.TierWeights[event.Tier] += weight
		analysis.TotalWeight += weight
		
		// Track max weight
		if weight > analysis.MaxWeight {
			analysis.MaxWeight = weight
		}
	}
	
	// Find dominant tier
	maxTierWeight := 0.0
	for tier, weight := range analysis.TierWeights {
		if weight > maxTierWeight {
			maxTierWeight = weight
			analysis.DominantTier = tier
		}
	}
	
	return analysis
}

// ValidateTierConfiguration checks if tier configuration is sensible
func (tdf *TierDecayFunction) ValidateTierConfiguration() error {
	config := tdf.config
	
	// Check that half-lives are positive and in reasonable order
	if config.ImminentHalfLife <= 0 {
		return fmt.Errorf("imminent half-life must be positive")
	}
	if config.NearTermHalfLife <= 0 {
		return fmt.Errorf("near-term half-life must be positive")
	}
	if config.MediumHalfLife <= 0 {
		return fmt.Errorf("medium half-life must be positive")
	}
	if config.DistantHalfLife <= 0 {
		return fmt.Errorf("distant half-life must be positive")
	}
	
	// Check that half-lives increase with tier distance (optional warning)
	if config.ImminentHalfLife > config.NearTermHalfLife {
		// Could be a warning rather than error
	}
	
	// Check that base weights are non-negative
	if config.ImminentBase < 0 || config.NearTermBase < 0 || 
	   config.MediumBase < 0 || config.DistantBase < 0 {
		return fmt.Errorf("base weights must be non-negative")
	}
	
	// Check future/past weights
	if config.FutureWeight < 0 || config.PastWeight < 0 {
		return fmt.Errorf("future and past weights must be non-negative")
	}
	
	return nil
}

// GetTierDescription returns a human-readable description of a tier
func GetTierDescription(tier EventTier) string {
	switch tier {
	case TierImminent:
		return "Imminent (0-7 days): Earnings, major releases, scheduled events"
	case TierNearTerm:
		return "Near-term (7-30 days): Updates, forks, partnership announcements"
	case TierMedium:
		return "Medium-term (30-90 days): Roadmap milestones, regulatory decisions"
	case TierDistant:
		return "Distant (90+ days): Long-term developments, protocol upgrades"
	default:
		return "Unknown tier"
	}
}

// Example of how to calculate weighted tier signal
func CalculateWeightedTierSignal(events []CatalystEvent, currentTime time.Time, decayFunc *TierDecayFunction) float64 {
	if len(events) == 0 {
		return 0.0
	}
	
	totalWeight := 0.0
	maxPossibleWeight := 0.0
	
	for _, event := range events {
		weight := decayFunc.CalculateWeight(event.Tier, event.EventTime, currentTime)
		totalWeight += weight
		
		// Max possible weight would be imminent tier at t=0
		maxPossibleWeight += decayFunc.config.ImminentBase
	}
	
	// Normalize to 0-1 scale with logarithmic scaling to prevent dominance
	if maxPossibleWeight > 0 {
		signal := math.Log(1+totalWeight) / math.Log(1+maxPossibleWeight)
		return math.Max(0.0, math.Min(1.0, signal))
	}
	
	return 0.0
}