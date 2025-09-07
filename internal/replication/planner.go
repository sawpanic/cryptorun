package replication

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// PlannerConfig holds configuration for the replication planner
type PlannerConfig struct {
	MaxConcurrentSteps int           `json:"max_concurrent_steps"`
	DefaultWindow      time.Duration `json:"default_window"`
	MaxRetries         int           `json:"max_retries"`
	PlanTTL            time.Duration `json:"plan_ttl"`
	EnablePITValidation bool         `json:"enable_pit_validation"`
}

// RegionHealth represents the health status of a region
type RegionHealth struct {
	Region          Region        `json:"region"`
	Healthy         bool          `json:"healthy"`
	LastHealthCheck time.Time     `json:"last_health_check"`
	ReplicationLag  time.Duration `json:"replication_lag"`
	ErrorRate       float64       `json:"error_rate"`
	AvailableStorage int64        `json:"available_storage_gb"`
}

// ReplicationState represents the current state of replication
type ReplicationState struct {
	RegionHealth     map[Region]*RegionHealth `json:"region_health"`
	ActivePlans      []string                 `json:"active_plans"`
	LastSyncTimes    map[string]time.Time     `json:"last_sync_times"` // tier:from:to -> timestamp
	PendingWindows   []TimeRange              `json:"pending_windows"`
	InProgressSteps  []string                 `json:"in_progress_steps"`
}

// Planner creates and manages replication plans
type Planner struct {
	config   PlannerConfig
	ruleset  *RuleSet
	state    *ReplicationState
	metadata map[string]interface{}
}

// NewPlanner creates a new replication planner
func NewPlanner(config PlannerConfig, ruleset *RuleSet) *Planner {
	return &Planner{
		config:   config,
		ruleset:  ruleset,
		state:    &ReplicationState{
			RegionHealth:    make(map[Region]*RegionHealth),
			ActivePlans:     []string{},
			LastSyncTimes:   make(map[string]time.Time),
			PendingWindows:  []TimeRange{},
			InProgressSteps: []string{},
		},
		metadata: make(map[string]interface{}),
	}
}

// UpdateRegionHealth updates the health status of a region
func (p *Planner) UpdateRegionHealth(region Region, health *RegionHealth) {
	p.state.RegionHealth[region] = health
}

// IsRegionHealthy checks if a region is healthy for replication
func (p *Planner) IsRegionHealthy(region Region) bool {
	health, exists := p.state.RegionHealth[region]
	if !exists {
		return false // Unknown regions are considered unhealthy
	}
	
	return health.Healthy && 
		time.Since(health.LastHealthCheck) < 5*time.Minute &&
		health.ErrorRate < 0.1 // Less than 10% error rate
}

// BuildPlan creates a replication plan based on rules and current state
func (p *Planner) BuildPlan(tier Tier, window TimeRange, dryRun bool) (*Plan, error) {
	// Validate inputs
	if window.From.After(window.To) {
		return nil, fmt.Errorf("invalid time window: from %v is after to %v", window.From, window.To)
	}
	
	// Get relevant rules for the tier
	relevantRules := p.ruleset.FilterRulesByTier(tier)
	if len(relevantRules) == 0 {
		return nil, fmt.Errorf("no replication rules found for tier %s", tier)
	}
	
	// Sort rules by priority (higher priority first)
	sort.Slice(relevantRules, func(i, j int) bool {
		return relevantRules[i].Priority > relevantRules[j].Priority
	})
	
	planID := uuid.New().String()
	plan := &Plan{
		ID:        planID,
		Steps:     []Step{},
		CreatedAt: time.Now(),
		DryRun:    dryRun,
		Metadata: map[string]interface{}{
			"tier":           tier,
			"window_from":    window.From,
			"window_to":      window.To,
			"planner_config": p.config,
		},
	}
	
	stepCounter := 0
	
	// Build steps from rules
	for _, rule := range relevantRules {
		// Skip disabled rules or unhealthy source regions
		if !rule.Enabled || !p.IsRegionHealthy(rule.From) {
			continue
		}
		
		// Create steps for each destination region
		for _, toRegion := range rule.To {
			// Skip unhealthy destination regions
			if !p.IsRegionHealthy(toRegion) {
				continue
			}
			
			// Create time windows for the step based on tier
			stepWindows := p.createTimeWindows(rule.Tier, window)
			
			for _, stepWindow := range stepWindows {
				stepID := fmt.Sprintf("%s-step-%d", planID, stepCounter)
				stepCounter++
				
				step := Step{
					ID:       stepID,
					Tier:     rule.Tier,
					From:     rule.From,
					To:       toRegion,
					Window:   stepWindow,
					Priority: rule.Priority,
					EstimatedDuration: p.estimateStepDuration(rule.Tier, stepWindow),
					MaxRetries:        p.config.MaxRetries,
					Validator:         p.getValidatorsForTier(rule.Tier),
				}
				
				plan.Steps = append(plan.Steps, step)
			}
		}
	}
	
	// Sort steps by priority and estimated start time
	sort.Slice(plan.Steps, func(i, j int) bool {
		if plan.Steps[i].Priority != plan.Steps[j].Priority {
			return plan.Steps[i].Priority > plan.Steps[j].Priority
		}
		return plan.Steps[i].Window.From.Before(plan.Steps[j].Window.From)
	})
	
	// Update plan metadata
	plan.TotalSteps = len(plan.Steps)
	plan.EstimatedDuration = plan.GetTotalEstimatedDuration()
	
	// Validate the plan
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("generated plan is invalid: %w", err)
	}
	
	return plan, nil
}

// createTimeWindows creates appropriate time windows for a tier and overall window
func (p *Planner) createTimeWindows(tier Tier, overallWindow TimeRange) []TimeRange {
	var windows []TimeRange
	
	switch tier {
	case TierHot:
		// Hot tier uses small windows to minimize memory usage
		windowSize := 15 * time.Minute
		current := overallWindow.From
		
		for current.Before(overallWindow.To) {
			windowEnd := current.Add(windowSize)
			if windowEnd.After(overallWindow.To) {
				windowEnd = overallWindow.To
			}
			
			windows = append(windows, TimeRange{
				From: current,
				To:   windowEnd,
			})
			
			current = windowEnd
		}
		
	case TierWarm:
		// Warm tier uses hourly windows
		windowSize := time.Hour
		current := overallWindow.From
		
		for current.Before(overallWindow.To) {
			windowEnd := current.Add(windowSize)
			if windowEnd.After(overallWindow.To) {
				windowEnd = overallWindow.To
			}
			
			windows = append(windows, TimeRange{
				From: current,
				To:   windowEnd,
			})
			
			current = windowEnd
		}
		
	case TierCold:
		// Cold tier can handle larger windows
		windowSize := 6 * time.Hour
		current := overallWindow.From
		
		for current.Before(overallWindow.To) {
			windowEnd := current.Add(windowSize)
			if windowEnd.After(overallWindow.To) {
				windowEnd = overallWindow.To
			}
			
			windows = append(windows, TimeRange{
				From: current,
				To:   windowEnd,
			})
			
			current = windowEnd
		}
	}
	
	// If no windows were created, use the overall window
	if len(windows) == 0 {
		windows = append(windows, overallWindow)
	}
	
	return windows
}

// estimateStepDuration estimates how long a replication step will take
func (p *Planner) estimateStepDuration(tier Tier, window TimeRange) time.Duration {
	duration := window.To.Sub(window.From)
	
	// Base estimates by tier
	baseRates := map[Tier]time.Duration{
		TierHot:  100 * time.Millisecond, // Very fast for hot data
		TierWarm: 2 * time.Second,        // Moderate for aggregated data
		TierCold: 10 * time.Second,       // Slower for file operations
	}
	
	base := baseRates[tier]
	
	// Scale by window duration (larger windows take longer)
	scale := float64(duration) / float64(time.Hour)
	if scale < 0.1 {
		scale = 0.1 // Minimum scaling factor
	}
	
	estimated := time.Duration(float64(base) * scale)
	
	// Add network and processing overhead
	overhead := 5 * time.Second
	if tier == TierHot {
		overhead = 1 * time.Second // Less overhead for hot tier
	}
	
	return estimated + overhead
}

// getValidatorsForTier returns appropriate validation functions for a tier
func (p *Planner) getValidatorsForTier(tier Tier) []ValidateFn {
	var validators []ValidateFn
	
	// Common validators for all tiers
	validators = append(validators, ValidateSchema)
	validators = append(validators, ValidateTimestamps)
	
	switch tier {
	case TierHot:
		validators = append(validators, ValidateSequenceNumbers)
		validators = append(validators, ValidateFreshness(5*time.Second))
	case TierWarm:
		validators = append(validators, ValidateCompleteness)
		validators = append(validators, ValidateFreshness(60*time.Second))
	case TierCold:
		validators = append(validators, ValidateIntegrity)
		validators = append(validators, ValidatePartitioning)
	}
	
	return validators
}

// ValidateForExecution checks if a plan can be safely executed
func (p *Planner) ValidateForExecution(plan *Plan) error {
	if plan == nil {
		return fmt.Errorf("plan cannot be nil")
	}
	
	// Check plan age
	if time.Since(plan.CreatedAt) > p.config.PlanTTL {
		return fmt.Errorf("plan is too old (created %v ago)", time.Since(plan.CreatedAt))
	}
	
	// Validate all regions in the plan are healthy
	for _, step := range plan.Steps {
		if !p.IsRegionHealthy(step.From) {
			return fmt.Errorf("source region %s is unhealthy for step %s", step.From, step.ID)
		}
		if !p.IsRegionHealthy(step.To) {
			return fmt.Errorf("destination region %s is unhealthy for step %s", step.To, step.ID)
		}
	}
	
	// Check for conflicting steps with active plans
	for _, activeID := range p.state.ActivePlans {
		if activeID == plan.ID {
			continue // Skip self
		}
		
		// In a real implementation, we would check for resource conflicts
		// For now, we just limit concurrent plans
		if len(p.state.ActivePlans) >= p.config.MaxConcurrentSteps {
			return fmt.Errorf("too many active plans (limit: %d)", p.config.MaxConcurrentSteps)
		}
	}
	
	return nil
}

// MarkPlanActive marks a plan as actively executing
func (p *Planner) MarkPlanActive(planID string) {
	for _, activeID := range p.state.ActivePlans {
		if activeID == planID {
			return // Already active
		}
	}
	p.state.ActivePlans = append(p.state.ActivePlans, planID)
}

// MarkPlanComplete removes a plan from active status
func (p *Planner) MarkPlanComplete(planID string) {
	for i, activeID := range p.state.ActivePlans {
		if activeID == planID {
			// Remove from slice
			p.state.ActivePlans = append(p.state.ActivePlans[:i], p.state.ActivePlans[i+1:]...)
			return
		}
	}
}

// GetReplicationState returns a copy of the current replication state
func (p *Planner) GetReplicationState() *ReplicationState {
	// Create a deep copy to prevent external modifications
	state := &ReplicationState{
		RegionHealth:    make(map[Region]*RegionHealth),
		ActivePlans:     make([]string, len(p.state.ActivePlans)),
		LastSyncTimes:   make(map[string]time.Time),
		PendingWindows:  make([]TimeRange, len(p.state.PendingWindows)),
		InProgressSteps: make([]string, len(p.state.InProgressSteps)),
	}
	
	// Copy region health
	for region, health := range p.state.RegionHealth {
		healthCopy := *health
		state.RegionHealth[region] = &healthCopy
	}
	
	// Copy other fields
	copy(state.ActivePlans, p.state.ActivePlans)
	copy(state.PendingWindows, p.state.PendingWindows)
	copy(state.InProgressSteps, p.state.InProgressSteps)
	
	for key, value := range p.state.LastSyncTimes {
		state.LastSyncTimes[key] = value
	}
	
	return state
}

// Default validation functions (simplified implementations)

// ValidateSchema validates that required schema fields are present
func ValidateSchema(data map[string]interface{}) error {
	requiredFields := []string{"timestamp", "venue", "symbol"}
	
	for _, field := range requiredFields {
		if _, exists := data[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}
	
	return nil
}

// ValidateTimestamps validates timestamp format and range
func ValidateTimestamps(data map[string]interface{}) error {
	timestamp, exists := data["timestamp"]
	if !exists {
		return fmt.Errorf("timestamp field is required")
	}
	
	// Try to parse as time.Time
	switch ts := timestamp.(type) {
	case time.Time:
		if ts.IsZero() {
			return fmt.Errorf("timestamp cannot be zero")
		}
		if ts.After(time.Now().Add(time.Hour)) {
			return fmt.Errorf("timestamp too far in future")
		}
	case string:
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			return fmt.Errorf("invalid timestamp format: %w", err)
		}
	default:
		return fmt.Errorf("timestamp must be time.Time or RFC3339 string")
	}
	
	return nil
}

// ValidateSequenceNumbers validates sequence continuity for hot tier
func ValidateSequenceNumbers(data map[string]interface{}) error {
	if seq, exists := data["sequence"]; exists {
		if seqNum, ok := seq.(int64); ok && seqNum < 0 {
			return fmt.Errorf("sequence number cannot be negative")
		}
	}
	return nil
}

// ValidateFreshness returns a validator that checks data freshness
func ValidateFreshness(maxAge time.Duration) ValidateFn {
	return func(data map[string]interface{}) error {
		timestamp, exists := data["timestamp"]
		if !exists {
			return fmt.Errorf("timestamp required for freshness check")
		}
		
		var ts time.Time
		switch t := timestamp.(type) {
		case time.Time:
			ts = t
		case string:
			var err error
			if ts, err = time.Parse(time.RFC3339, t); err != nil {
				return fmt.Errorf("invalid timestamp for freshness check: %w", err)
			}
		default:
			return fmt.Errorf("timestamp must be time.Time or RFC3339 string")
		}
		
		age := time.Since(ts)
		if age > maxAge {
			return fmt.Errorf("data is too old: %v > %v", age, maxAge)
		}
		
		return nil
	}
}

// ValidateCompleteness checks data completeness for warm tier
func ValidateCompleteness(data map[string]interface{}) error {
	// Check for essential trading data fields
	essentialFields := []string{"price", "volume"}
	for _, field := range essentialFields {
		if val, exists := data[field]; !exists || val == nil {
			return fmt.Errorf("missing essential field for completeness: %s", field)
		}
	}
	return nil
}

// ValidateIntegrity checks data integrity for cold tier
func ValidateIntegrity(data map[string]interface{}) error {
	// Check for integrity hash if present
	if hash, exists := data["checksum"]; exists {
		if hashStr, ok := hash.(string); ok && len(hashStr) < 8 {
			return fmt.Errorf("invalid checksum format")
		}
	}
	return nil
}

// ValidatePartitioning checks partitioning constraints for cold tier
func ValidatePartitioning(data map[string]interface{}) error {
	// Ensure data has partition keys
	partitionKeys := []string{"venue", "symbol", "date"}
	for _, key := range partitionKeys {
		if _, exists := data[key]; !exists {
			return fmt.Errorf("missing partition key: %s", key)
		}
	}
	return nil
}