package metrics

import (
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"
)

// FreshnessCalculator implements "worst feed wins" penalty system
// Applies multipliers based on feed age to penalize stale data
type FreshnessCalculator struct {
	maxAge       time.Duration  // Maximum acceptable age before severe penalty
	warnAge      time.Duration  // Warning threshold for freshness
	penaltyBase  float64        // Base penalty multiplier
}

// NewFreshnessCalculator creates a freshness calculator with default settings
func NewFreshnessCalculator() *FreshnessCalculator {
	return &FreshnessCalculator{
		maxAge:      5 * time.Minute,    // 5 minutes max age
		warnAge:     1 * time.Minute,    // 1 minute warning
		penaltyBase: 0.1,                // 10% base penalty
	}
}

// FeedFreshness represents freshness data for a single feed
type FeedFreshness struct {
	Source      string        `json:"source"`
	LastUpdate  time.Time     `json:"last_update"`
	Age         time.Duration `json:"age"`
	Penalty     float64       `json:"penalty"`      // 0.0-1.0, higher = more penalty
	Status      string        `json:"status"`       // "fresh", "stale", "expired"
	Multiplier  float64       `json:"multiplier"`   // Final score multiplier
}

// FreshnessResult aggregates freshness across all feeds
type FreshnessResult struct {
	Feeds          []FeedFreshness `json:"feeds"`
	WorstAge       time.Duration   `json:"worst_age"`
	WorstSource    string          `json:"worst_source"`
	GlobalPenalty  float64         `json:"global_penalty"`
	GlobalMultiplier float64       `json:"global_multiplier"`
	Status         string          `json:"status"`
	Fresh          bool            `json:"fresh"`
}

// CalculateFreshness evaluates freshness across multiple data feeds
// Implements "worst feed wins" - the stalest feed determines overall penalty
func (fc *FreshnessCalculator) CalculateFreshness(feeds map[string]time.Time) FreshnessResult {
	now := time.Now()
	
	var feedFreshness []FeedFreshness
	var worstAge time.Duration
	var worstSource string
	var maxPenalty float64
	
	// Evaluate each feed
	for source, lastUpdate := range feeds {
		age := now.Sub(lastUpdate)
		penalty := fc.calculatePenalty(age)
		status := fc.getStatus(age)
		multiplier := 1.0 - penalty
		
		freshness := FeedFreshness{
			Source:     source,
			LastUpdate: lastUpdate,
			Age:        age,
			Penalty:    penalty,
			Status:     status,
			Multiplier: multiplier,
		}
		
		feedFreshness = append(feedFreshness, freshness)
		
		// Track worst feed
		if age > worstAge {
			worstAge = age
			worstSource = source
		}
		
		// Track maximum penalty (worst feed wins)
		if penalty > maxPenalty {
			maxPenalty = penalty
		}
	}
	
	// Global status based on worst feed
	globalStatus := fc.getStatus(worstAge)
	globalMultiplier := 1.0 - maxPenalty
	fresh := globalStatus == "fresh"
	
	result := FreshnessResult{
		Feeds:            feedFreshness,
		WorstAge:         worstAge,
		WorstSource:      worstSource,
		GlobalPenalty:    maxPenalty,
		GlobalMultiplier: globalMultiplier,
		Status:           globalStatus,
		Fresh:            fresh,
	}
	
	log.Debug().Dur("worst_age", worstAge).Str("worst_source", worstSource).
		Float64("penalty", maxPenalty).Float64("multiplier", globalMultiplier).
		Str("status", globalStatus).Msg("Freshness calculated")
	
	return result
}

// calculatePenalty computes penalty based on data age
func (fc *FreshnessCalculator) calculatePenalty(age time.Duration) float64 {
	if age <= fc.warnAge {
		return 0.0  // No penalty for fresh data
	}
	
	if age >= fc.maxAge {
		return 0.95  // Severe penalty for expired data (95% penalty)
	}
	
	// Exponential penalty between warning and max age
	ratio := float64(age-fc.warnAge) / float64(fc.maxAge-fc.warnAge)
	penalty := fc.penaltyBase * math.Exp(ratio*3) // Exponential growth
	
	// Cap penalty at 95%
	if penalty > 0.95 {
		penalty = 0.95
	}
	
	return penalty
}

// getStatus determines status based on age
func (fc *FreshnessCalculator) getStatus(age time.Duration) string {
	if age <= fc.warnAge {
		return "fresh"
	} else if age <= fc.maxAge {
		return "stale"
	} else {
		return "expired"
	}
}

// ApplyFreshnessMultiplier applies freshness penalty to a score
func (fc *FreshnessCalculator) ApplyFreshnessMultiplier(score float64, freshness FreshnessResult) float64 {
	adjustedScore := score * freshness.GlobalMultiplier
	
	log.Debug().Float64("original_score", score).
		Float64("multiplier", freshness.GlobalMultiplier).
		Float64("adjusted_score", adjustedScore).
		Str("status", freshness.Status).
		Msg("Freshness penalty applied")
	
	return adjustedScore
}

// ValidateFreshness checks if data meets freshness requirements
func (fc *FreshnessCalculator) ValidateFreshness(feeds map[string]time.Time, maxAge time.Duration) (bool, string) {
	if len(feeds) == 0 {
		return false, "no data feeds available"
	}
	
	now := time.Now()
	
	// Check each feed against maximum age
	for source, lastUpdate := range feeds {
		age := now.Sub(lastUpdate)
		if age > maxAge {
			return false, fmt.Sprintf("feed '%s' too stale: %s > %s", source, age, maxAge)
		}
	}
	
	return true, ""
}

// FreshnessGate implements entry gate based on data freshness
type FreshnessGate struct {
	calculator  *FreshnessCalculator
	maxAge      time.Duration
	minSources  int
}

// NewFreshnessGate creates a freshness gate with specified requirements
func NewFreshnessGate(maxAge time.Duration, minSources int) *FreshnessGate {
	return &FreshnessGate{
		calculator: NewFreshnessCalculator(),
		maxAge:     maxAge,
		minSources: minSources,
	}
}

// Evaluate checks if freshness requirements are met for entry
func (fg *FreshnessGate) Evaluate(feeds map[string]time.Time) (bool, FreshnessResult, string) {
	// Check minimum sources
	if len(feeds) < fg.minSources {
		result := FreshnessResult{
			Status: "insufficient_sources",
			Fresh:  false,
		}
		reason := fmt.Sprintf("insufficient sources: %d < %d", len(feeds), fg.minSources)
		return false, result, reason
	}
	
	// Calculate freshness
	freshness := fg.calculator.CalculateFreshness(feeds)
	
	// Check if worst feed exceeds maximum age
	if freshness.WorstAge > fg.maxAge {
		reason := fmt.Sprintf("data too stale: worst feed %s age %s > max %s", 
			freshness.WorstSource, freshness.WorstAge, fg.maxAge)
		return false, freshness, reason
	}
	
	// Check if global penalty is too high (>50% penalty)
	if freshness.GlobalPenalty > 0.5 {
		reason := fmt.Sprintf("freshness penalty too high: %.1f%%", freshness.GlobalPenalty*100)
		return false, freshness, reason
	}
	
	return true, freshness, ""
}

// FreshnessMonitor tracks feed freshness over time
type FreshnessMonitor struct {
	feeds       map[string]time.Time
	calculator  *FreshnessCalculator
	
	// Alerting thresholds
	alertAge    time.Duration
	alertPenalty float64
	
	// Metrics
	totalChecks   int64
	freshChecks   int64
	staleChecks   int64
	expiredChecks int64
}

// NewFreshnessMonitor creates a freshness monitor
func NewFreshnessMonitor() *FreshnessMonitor {
	return &FreshnessMonitor{
		feeds:        make(map[string]time.Time),
		calculator:   NewFreshnessCalculator(),
		alertAge:     2 * time.Minute,
		alertPenalty: 0.3, // 30% penalty threshold
	}
}

// UpdateFeed records the last update time for a feed
func (fm *FreshnessMonitor) UpdateFeed(source string, timestamp time.Time) {
	fm.feeds[source] = timestamp
}

// CheckFreshness evaluates current freshness and triggers alerts if needed
func (fm *FreshnessMonitor) CheckFreshness() FreshnessResult {
	fm.totalChecks++
	
	freshness := fm.calculator.CalculateFreshness(fm.feeds)
	
	// Update metrics
	switch freshness.Status {
	case "fresh":
		fm.freshChecks++
	case "stale":
		fm.staleChecks++
	case "expired":
		fm.expiredChecks++
	}
	
	// Trigger alerts if needed
	if freshness.WorstAge > fm.alertAge || freshness.GlobalPenalty > fm.alertPenalty {
		log.Warn().Dur("worst_age", freshness.WorstAge).
			Str("worst_source", freshness.WorstSource).
			Float64("penalty", freshness.GlobalPenalty).
			Msg("Freshness alert triggered")
	}
	
	return freshness
}

// GetMetrics returns freshness monitoring metrics
func (fm *FreshnessMonitor) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"total_checks":   fm.totalChecks,
		"fresh_checks":   fm.freshChecks,
		"stale_checks":   fm.staleChecks,
		"expired_checks": fm.expiredChecks,
		"fresh_ratio":    float64(fm.freshChecks) / float64(fm.totalChecks),
		"active_feeds":   len(fm.feeds),
	}
}