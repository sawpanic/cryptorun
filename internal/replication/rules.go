package replication

import (
	"fmt"
	"time"
)

// Tier represents the data tier for replication rules
type Tier string

const (
	TierHot  Tier = "hot"
	TierWarm Tier = "warm"
	TierCold Tier = "cold"
)

// Mode represents the replication mode
type Mode string

const (
	ActiveActive  Mode = "active-active"
	ActivePassive Mode = "active-passive"
)

// Region represents a geographic region
type Region string

// Common regions used in CryptoRun
const (
	RegionUSEast1 Region = "us-east-1"
	RegionUSWest2 Region = "us-west-2"
	RegionEUWest1 Region = "eu-west-1"
)

// Rule defines a replication rule for a specific tier and region pair
type Rule struct {
	Tier     Tier          `json:"tier"`
	Mode     Mode          `json:"mode"`
	From     Region        `json:"from"`
	To       []Region      `json:"to"`
	LagSLO   time.Duration `json:"lag_slo"`   // e.g., warm<=60s, cold<=5m
	Priority int           `json:"priority"`  // Higher priority rules processed first
	Enabled  bool          `json:"enabled"`
}

// TimeRange represents a time window for replication operations
type TimeRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// ValidateFn represents a validation function for replication data
type ValidateFn func(data map[string]interface{}) error

// Step represents a single replication step in a plan
type Step struct {
	ID        string       `json:"id"`
	Tier      Tier         `json:"tier"`
	From      Region       `json:"from"`
	To        Region       `json:"to"`
	Window    TimeRange    `json:"window"`
	Validator []ValidateFn `json:"-"` // Not serialized - function pointers
	Priority  int          `json:"priority"`
	EstimatedDuration time.Duration `json:"estimated_duration"`
	MaxRetries        int           `json:"max_retries"`
}

// Plan represents a complete replication plan with ordered steps
type Plan struct {
	ID          string    `json:"id"`
	Steps       []Step    `json:"steps"`
	CreatedAt   time.Time `json:"created_at"`
	TotalSteps  int       `json:"total_steps"`
	EstimatedDuration time.Duration `json:"estimated_duration"`
	DryRun      bool      `json:"dry_run"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// RuleSet manages a collection of replication rules
type RuleSet struct {
	Rules    []Rule    `json:"rules"`
	Version  string    `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks if a replication rule is valid
func (r *Rule) Validate() error {
	if r.Tier == "" {
		return fmt.Errorf("tier cannot be empty")
	}
	
	if r.Mode == "" {
		return fmt.Errorf("mode cannot be empty")
	}
	
	if r.From == "" {
		return fmt.Errorf("from region cannot be empty")
	}
	
	if len(r.To) == 0 {
		return fmt.Errorf("to regions cannot be empty")
	}
	
	if r.LagSLO <= 0 {
		return fmt.Errorf("lag SLO must be positive")
	}
	
	// Validate tier-specific constraints
	switch r.Tier {
	case TierHot:
		if r.LagSLO > 5*time.Second {
			return fmt.Errorf("hot tier lag SLO cannot exceed 5 seconds")
		}
		if r.Mode != ActiveActive {
			return fmt.Errorf("hot tier must use active-active mode")
		}
	case TierWarm:
		if r.LagSLO > 5*time.Minute {
			return fmt.Errorf("warm tier lag SLO cannot exceed 5 minutes")
		}
	case TierCold:
		if r.LagSLO > 30*time.Minute {
			return fmt.Errorf("cold tier lag SLO cannot exceed 30 minutes")
		}
	default:
		return fmt.Errorf("unknown tier: %s", r.Tier)
	}
	
	return nil
}

// GetSLOForTier returns the default SLO for a given tier
func GetSLOForTier(tier Tier) time.Duration {
	switch tier {
	case TierHot:
		return 500 * time.Millisecond
	case TierWarm:
		return 60 * time.Second
	case TierCold:
		return 5 * time.Minute
	default:
		return time.Minute // Default fallback
	}
}

// GetDefaultModeForTier returns the default replication mode for a tier
func GetDefaultModeForTier(tier Tier) Mode {
	switch tier {
	case TierHot:
		return ActiveActive
	case TierWarm, TierCold:
		return ActivePassive
	default:
		return ActivePassive
	}
}

// FilterRulesByTier returns rules matching the specified tier
func (rs *RuleSet) FilterRulesByTier(tier Tier) []Rule {
	var filtered []Rule
	for _, rule := range rs.Rules {
		if rule.Tier == tier && rule.Enabled {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

// FilterRulesByRegion returns rules involving the specified region
func (rs *RuleSet) FilterRulesByRegion(region Region) []Rule {
	var filtered []Rule
	for _, rule := range rs.Rules {
		if !rule.Enabled {
			continue
		}
		
		// Check if region is source
		if rule.From == region {
			filtered = append(filtered, rule)
			continue
		}
		
		// Check if region is destination
		for _, to := range rule.To {
			if to == region {
				filtered = append(filtered, rule)
				break
			}
		}
	}
	return filtered
}

// Validate checks if all rules in the ruleset are valid
func (rs *RuleSet) Validate() error {
	if len(rs.Rules) == 0 {
		return fmt.Errorf("ruleset cannot be empty")
	}
	
	for i, rule := range rs.Rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("rule %d is invalid: %w", i, err)
		}
	}
	
	return nil
}

// EstimateDuration estimates the duration for a replication step
func (s *Step) EstimateDuration(dataSize int64) time.Duration {
	// Base duration estimates by tier (simplified model)
	baseDurations := map[Tier]time.Duration{
		TierHot:  100 * time.Millisecond,
		TierWarm: 5 * time.Second,
		TierCold: 30 * time.Second,
	}
	
	base := baseDurations[s.Tier]
	
	// Scale by data size (rough estimate: 1MB per second)
	if dataSize > 0 {
		dataDuration := time.Duration(dataSize/1024/1024) * time.Second
		base += dataDuration
	}
	
	// Add network latency buffer
	base += 2 * time.Second
	
	return base
}

// Validate checks if a replication step is valid
func (s *Step) Validate() error {
	if s.Tier == "" {
		return fmt.Errorf("step tier cannot be empty")
	}
	
	if s.From == "" {
		return fmt.Errorf("step from region cannot be empty")
	}
	
	if s.To == "" {
		return fmt.Errorf("step to region cannot be empty")
	}
	
	if s.From == s.To {
		return fmt.Errorf("step from and to regions cannot be the same")
	}
	
	if s.Window.From.After(s.Window.To) {
		return fmt.Errorf("step window from cannot be after to")
	}
	
	return nil
}

// Validate checks if a replication plan is valid
func (p *Plan) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("plan ID cannot be empty")
	}
	
	if len(p.Steps) == 0 {
		return fmt.Errorf("plan must have at least one step")
	}
	
	if p.TotalSteps != len(p.Steps) {
		return fmt.Errorf("plan total steps mismatch: expected %d, got %d", p.TotalSteps, len(p.Steps))
	}
	
	for i, step := range p.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("step %d is invalid: %w", i, err)
		}
	}
	
	return nil
}

// GetStepsByTier returns all steps for a specific tier
func (p *Plan) GetStepsByTier(tier Tier) []Step {
	var filtered []Step
	for _, step := range p.Steps {
		if step.Tier == tier {
			filtered = append(filtered, step)
		}
	}
	return filtered
}

// GetTotalEstimatedDuration calculates total estimated duration for all steps
func (p *Plan) GetTotalEstimatedDuration() time.Duration {
	var total time.Duration
	for _, step := range p.Steps {
		total += step.EstimatedDuration
	}
	return total
}