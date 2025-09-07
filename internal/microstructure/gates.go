package microstructure

import (
	"fmt"
	"time"
)

// GatesEngine orchestrates all microstructure gates with precedence rules
type GatesEngine struct {
	depthCalc    *DepthCalculator
	spreadCalc   *SpreadCalculator
	vadrCalc     *VADRCalculator
	venueMonitor *VenueHealthMonitor
}

// NewGatesEngine creates a comprehensive gates validation engine
func NewGatesEngine() *GatesEngine {
	return &GatesEngine{
		depthCalc:    NewDepthCalculator(60),  // 60s window
		spreadCalc:   NewSpreadCalculator(60), // 60s rolling
		vadrCalc:     NewVADRCalculator(),
		venueMonitor: NewVenueHealthMonitor(nil), // Use default config
	}
}

// GatesResult contains comprehensive gates validation results
type GatesResult struct {
	Symbol         string             `json:"symbol"`
	Timestamp      time.Time          `json:"timestamp"`
	OverallPass    bool               `json:"overall_pass"`   // All gates pass
	CriticalFails  []string           `json:"critical_fails"` // Critical failures that block entry
	WarningFails   []string           `json:"warning_fails"`  // Warnings that don't block
	DepthResult    *DepthResult       `json:"depth_result"`
	SpreadResult   *SpreadResult      `json:"spread_result"`
	VADRResult     *VADRResult        `json:"vadr_result"`
	VenueHealth    *VenueHealthResult `json:"venue_health"`
	FreshnessCheck *FreshnessResult   `json:"freshness_check"`
	Precedence     *PrecedenceResult  `json:"precedence"` // Precedence rule application
	Tier           *LiquidityTier     `json:"tier"`       // Applied tier
}

// PrecedenceResult shows how precedence rules were applied
type PrecedenceResult struct {
	WorstFeedMultiplier float64                  `json:"worst_feed_multiplier"` // Worst-feed wins
	FeedStaleness       map[string]time.Duration `json:"feed_staleness"`        // Per-venue staleness
	AppliedPolicy       string                   `json:"applied_policy"`        // Policy applied
	DegradedMode        bool                     `json:"degraded_mode"`         // Operating in degraded mode
	HealthyVenues       []string                 `json:"healthy_venues"`        // Venues still healthy
	UnhealthyVenues     []string                 `json:"unhealthy_venues"`      // Failed venues
}

// FreshnessResult contains data freshness validation
type FreshnessResult struct {
	Symbol           string                   `json:"symbol"`
	StalenessCheck   bool                     `json:"staleness_check"`    // Pass/fail staleness
	MaxStaleness     time.Duration            `json:"max_staleness"`      // Worst staleness found
	VenueStaleness   map[string]time.Duration `json:"venue_staleness"`    // Per venue
	AbortOnStaleness bool                     `json:"abort_on_staleness"` // Should abort gate
}

// GatesHealthConfig contains health monitoring configuration for gates
type GatesHealthConfig struct {
	MaxSpreadDivergence float64       // 0.5% max cross-venue spread divergence
	MaxHeartbeatGap     time.Duration // 10s max WS heartbeat gap
}

// VenueStatus tracks health metrics for a single venue
type VenueStatus struct {
	VenueName         string    `json:"venue_name"`
	IsHealthy         bool      `json:"is_healthy"`
	LastHeartbeat     time.Time `json:"last_heartbeat"`
	LastOrderbook     time.Time `json:"last_orderbook"`
	CurrentSpreadBps  float64   `json:"current_spread_bps"`
	RecentErrors      []string  `json:"recent_errors"`
	ConnectivityScore float64   `json:"connectivity_score"` // 0-1 score
}

// VenueHealthResult contains venue health evaluation
type VenueHealthResult struct {
	HealthyVenueCount   int                     `json:"healthy_venue_count"`
	TotalVenueCount     int                     `json:"total_venue_count"`
	DegradedMode        bool                    `json:"degraded_mode"`         // ≥1 healthy venue
	CrossCheckMode      bool                    `json:"cross_check_mode"`      // ≥2 venues for cross-checks
	MaxSpreadDivergence float64                 `json:"max_spread_divergence"` // Largest spread divergence
	VenueStatuses       map[string]*VenueStatus `json:"venue_statuses"`
	OverallHealthy      bool                    `json:"overall_healthy"`
}

// EvaluateAllGates runs comprehensive gates validation with precedence
func (ge *GatesEngine) EvaluateAllGates(symbol string, orderbook *OrderBookSnapshot, tier *LiquidityTier, vadrInput *VADRInput) (*GatesResult, error) {
	if orderbook == nil || tier == nil {
		return nil, fmt.Errorf("invalid inputs: orderbook=%v, tier=%v", orderbook, tier)
	}

	timestamp := time.Now()
	var criticalFails []string
	var warningFails []string

	// 1. Evaluate venue health first (precedence rule)
	venueHealth := ge.evaluateVenueHealth(symbol, orderbook)

	// Check for critical venue health failures
	if !venueHealth.OverallHealthy {
		if venueHealth.HealthyVenueCount == 0 {
			criticalFails = append(criticalFails, "no healthy venues available")
		} else if venueHealth.HealthyVenueCount < 2 {
			warningFails = append(warningFails, "operating in degraded mode (single venue)")
		}
	}

	// 2. Check data freshness (precedence rule)
	freshnessResult := ge.evaluateFreshness(symbol, orderbook)
	if freshnessResult.AbortOnStaleness {
		criticalFails = append(criticalFails, fmt.Sprintf("data too stale: %v", freshnessResult.MaxStaleness))
	}

	// 3. Apply precedence rules for subsequent checks
	precedenceResult := ge.applyPrecedenceRules(venueHealth, freshnessResult)

	// 4. Evaluate depth requirement (tiered by ADV)
	depthResult, err := ge.depthCalc.CalculateDepth(orderbook)
	if err != nil {
		criticalFails = append(criticalFails, fmt.Sprintf("depth calculation failed: %v", err))
	} else {
		depthPass, depthReason := ge.depthCalc.ValidateDepthRequirement(depthResult, tier)
		if !depthPass {
			criticalFails = append(criticalFails, depthReason)
		}
	}

	// 5. Evaluate spread requirement
	spreadResult, err := ge.spreadCalc.CalculateSpread(orderbook)
	if err != nil {
		criticalFails = append(criticalFails, fmt.Sprintf("spread calculation failed: %v", err))
	} else {
		spreadPass, spreadReason := ge.spreadCalc.ValidateSpreadRequirement(spreadResult, tier)
		if !spreadPass {
			criticalFails = append(criticalFails, spreadReason)
		}
	}

	// 6. Evaluate VADR requirement with precedence (max of p80, tier_min)
	var vadrResult *VADRResult
	if vadrInput != nil {
		vadrResult, err = ge.vadrCalc.CalculateVADR(vadrInput, tier)
		if err != nil {
			criticalFails = append(criticalFails, fmt.Sprintf("VADR calculation failed: %v", err))
		} else {
			vadrPass, vadrReason := ge.vadrCalc.ValidateVADRRequirement(vadrResult)
			if !vadrPass {
				criticalFails = append(criticalFails, vadrReason)
			}
		}
	} else {
		warningFails = append(warningFails, "VADR input not provided")
	}

	// Determine overall pass based on critical failures and precedence
	overallPass := len(criticalFails) == 0 && !precedenceResult.DegradedMode

	return &GatesResult{
		Symbol:         symbol,
		Timestamp:      timestamp,
		OverallPass:    overallPass,
		CriticalFails:  criticalFails,
		WarningFails:   warningFails,
		DepthResult:    depthResult,
		SpreadResult:   spreadResult,
		VADRResult:     vadrResult,
		VenueHealth:    venueHealth,
		FreshnessCheck: freshnessResult,
		Precedence:     precedenceResult,
		Tier:           tier,
	}, nil
}

// evaluateVenueHealth checks venue connectivity and spread divergence
func (ge *GatesEngine) evaluateVenueHealth(symbol string, orderbook *OrderBookSnapshot) *VenueHealthResult {
	// For now, simulate venue health based on orderbook quality
	// In production, this would track actual venue connections

	totalVenues := 1 // Based on single orderbook - would be multi-venue in prod
	healthyVenues := 0

	venueStatuses := make(map[string]*VenueStatus)

	// Evaluate primary venue (orderbook source)
	venueName := orderbook.Venue
	if venueName == "" {
		venueName = "primary"
	}

	isHealthy := true
	connectivityScore := 1.0
	recentErrors := []string{}

	// Check for basic health indicators
	if len(orderbook.Bids) == 0 || len(orderbook.Asks) == 0 {
		isHealthy = false
		connectivityScore = 0.0
		recentErrors = append(recentErrors, "incomplete orderbook")
	}

	// Check data freshness as health indicator
	dataAge := time.Since(orderbook.Timestamp)
	maxHeartbeatGap := 10 * time.Second // Default threshold
	if dataAge > maxHeartbeatGap {
		isHealthy = false
		connectivityScore *= 0.5
		recentErrors = append(recentErrors, fmt.Sprintf("stale data: %v", dataAge))
	}

	if isHealthy {
		healthyVenues++
	}

	venueStatuses[venueName] = &VenueStatus{
		VenueName:         venueName,
		IsHealthy:         isHealthy,
		LastHeartbeat:     orderbook.Timestamp,
		LastOrderbook:     orderbook.Timestamp,
		CurrentSpreadBps:  0.0, // Would calculate from orderbook
		RecentErrors:      recentErrors,
		ConnectivityScore: connectivityScore,
	}

	return &VenueHealthResult{
		HealthyVenueCount:   healthyVenues,
		TotalVenueCount:     totalVenues,
		DegradedMode:        healthyVenues < 2,
		CrossCheckMode:      healthyVenues >= 2,
		MaxSpreadDivergence: 0.0, // Would calculate cross-venue divergence
		VenueStatuses:       venueStatuses,
		OverallHealthy:      healthyVenues > 0,
	}
}

// evaluateFreshness validates data freshness and staleness
func (ge *GatesEngine) evaluateFreshness(symbol string, orderbook *OrderBookSnapshot) *FreshnessResult {
	maxAllowedStaleness := 5 * time.Minute // Configurable threshold
	dataAge := time.Since(orderbook.Timestamp)

	venueStaleness := map[string]time.Duration{
		orderbook.Venue: dataAge,
	}

	stalenessCheck := dataAge <= maxAllowedStaleness
	abortOnStaleness := dataAge > maxAllowedStaleness

	return &FreshnessResult{
		Symbol:           symbol,
		StalenessCheck:   stalenessCheck,
		MaxStaleness:     dataAge,
		VenueStaleness:   venueStaleness,
		AbortOnStaleness: abortOnStaleness,
	}
}

// applyPrecedenceRules implements freshness precedence (worst-feed wins)
func (ge *GatesEngine) applyPrecedenceRules(venueHealth *VenueHealthResult, freshness *FreshnessResult) *PrecedenceResult {
	// Worst-feed multiplier: use the stalest data's multiplier
	worstMultiplier := 1.0
	if freshness.MaxStaleness > time.Minute {
		// Apply staleness penalty
		minutesStale := freshness.MaxStaleness.Minutes()
		worstMultiplier = 1.0 + (minutesStale * 0.1) // 10% penalty per minute stale
	}

	// Determine applied policy
	appliedPolicy := "normal"
	degradedMode := venueHealth.DegradedMode

	if freshness.AbortOnStaleness {
		appliedPolicy = "abort_stale_data"
	} else if venueHealth.HealthyVenueCount == 0 {
		appliedPolicy = "abort_no_venues"
	} else if venueHealth.HealthyVenueCount == 1 {
		appliedPolicy = "degraded_single_venue"
		degradedMode = true
	} else {
		appliedPolicy = "normal_multi_venue"
	}

	// Extract venue lists
	var healthyVenues, unhealthyVenues []string
	for venueName, status := range venueHealth.VenueStatuses {
		if status.IsHealthy {
			healthyVenues = append(healthyVenues, venueName)
		} else {
			unhealthyVenues = append(unhealthyVenues, venueName)
		}
	}

	return &PrecedenceResult{
		WorstFeedMultiplier: worstMultiplier,
		FeedStaleness:       freshness.VenueStaleness,
		AppliedPolicy:       appliedPolicy,
		DegradedMode:        degradedMode,
		HealthyVenues:       healthyVenues,
		UnhealthyVenues:     unhealthyVenues,
	}
}

// GetGatesSummary returns human-readable gates summary
func (ge *GatesEngine) GetGatesSummary(result *GatesResult) string {
	if result == nil {
		return "no gates data"
	}

	status := "PASS"
	if !result.OverallPass {
		status = "FAIL"
	}

	summary := fmt.Sprintf("Gates: %s [%s tier]", status, result.Tier.Name)

	if len(result.CriticalFails) > 0 {
		summary += fmt.Sprintf(" - %d critical fails", len(result.CriticalFails))
	}

	if len(result.WarningFails) > 0 {
		summary += fmt.Sprintf(", %d warnings", len(result.WarningFails))
	}

	if result.Precedence != nil && result.Precedence.DegradedMode {
		summary += " [DEGRADED]"
	}

	return summary
}

// GetDetailedGatesSummary returns comprehensive gates analysis
func (ge *GatesEngine) GetDetailedGatesSummary(result *GatesResult) string {
	if result == nil {
		return "No gates evaluation performed"
	}

	summary := fmt.Sprintf("=== Gates Evaluation: %s ===\n", result.Symbol)
	summary += fmt.Sprintf("Overall: %s | Tier: %s | Timestamp: %s\n",
		map[bool]string{true: "PASS", false: "FAIL"}[result.OverallPass],
		result.Tier.Name,
		result.Timestamp.Format("15:04:05"))

	// Venue Health
	if result.VenueHealth != nil {
		summary += fmt.Sprintf("Venues: %d/%d healthy",
			result.VenueHealth.HealthyVenueCount,
			result.VenueHealth.TotalVenueCount)
		if result.VenueHealth.DegradedMode {
			summary += " [DEGRADED]"
		}
		summary += "\n"
	}

	// Individual Gate Results
	if result.DepthResult != nil {
		summary += fmt.Sprintf("Depth: $%.0f (≥$%.0f required)\n",
			result.DepthResult.TotalDepthUSD, result.Tier.DepthMinUSD)
	}

	if result.SpreadResult != nil {
		summary += fmt.Sprintf("Spread: %.1f bps (≤%.1f bps max)\n",
			result.SpreadResult.RollingAvgBps, result.Tier.SpreadCapBps)
	}

	if result.VADRResult != nil {
		summary += fmt.Sprintf("VADR: %.2f× (≥%.2f× required)\n",
			result.VADRResult.Current, result.VADRResult.EffectiveMin)
	}

	// Failures
	if len(result.CriticalFails) > 0 {
		summary += "CRITICAL FAILURES:\n"
		for _, fail := range result.CriticalFails {
			summary += fmt.Sprintf("  • %s\n", fail)
		}
	}

	if len(result.WarningFails) > 0 {
		summary += "WARNINGS:\n"
		for _, warn := range result.WarningFails {
			summary += fmt.Sprintf("  ⚠ %s\n", warn)
		}
	}

	return summary
}

// IsOperational checks if enough venues are healthy for operation
func (ge *GatesEngine) IsOperational() bool {
	// Check if venue monitor is operational
	// In practice, this would check actual venue health metrics
	return true // Simplified implementation
}

// UpdateVenueStatus updates health status for a venue
func (ge *GatesEngine) UpdateVenueStatus(venueName string, status *VenueStatus) {
	// Would update the actual venue health monitor
	// For now, simplified implementation
}

// ClearVenueHistory clears venue health history (useful for testing)
func (ge *GatesEngine) ClearVenueHistory() {
	// Would clear the actual venue health monitor history
	// For now, simplified implementation
}
