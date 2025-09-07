// Package premove contains execution quality tracking for monitoring fills, slippage, and venue performance
package premove

import (
	"sync"
	"time"
)

// ExecutionQualityTracker tracks fills, slippage >30bps tighten; recover after 20 good trades or 48h
type ExecutionQualityTracker struct {
	mu                          sync.Mutex
	slippageBpsTightenThreshold float64                // Default: 30bps
	goodTradesThreshold         int                    // Default: 20 trades for recovery
	recoveryWindowHours         int                    // Default: 48h for time-based recovery
	executions                  []ExecutionRecord      // Recent execution history
	venueStats                  map[string]*VenueStats // Per-venue execution statistics
	tightenedVenues             map[string]time.Time   // Venues with tightened thresholds
}

// ExecutionConfig holds configuration for execution quality tracking
type ExecutionConfig struct {
	SlippageBpsTightenThreshold float64 `yaml:"slippage_bps_tighten_threshold"` // Default: 30.0
	GoodTradesThreshold         int     `yaml:"good_trades_threshold"`          // Default: 20
	RecoveryWindowHours         int     `yaml:"recovery_window_hours"`          // Default: 48
	MaxHistorySize              int     `yaml:"max_history_size"`               // Default: 1000
}

// ExecutionRecord represents a single execution for quality tracking
type ExecutionRecord struct {
	ID            string    `json:"id"`
	Symbol        string    `json:"symbol"`
	Venue         string    `json:"venue"`
	Side          string    `json:"side"` // "buy" or "sell"
	Quantity      float64   `json:"quantity"`
	ExpectedPrice float64   `json:"expected_price"`
	ActualPrice   float64   `json:"actual_price"`
	SlippageBps   float64   `json:"slippage_bps"` // Basis points of slippage
	Timestamp     time.Time `json:"timestamp"`
	Quality       string    `json:"quality"` // "good", "bad", "acceptable"
}

// VenueStats tracks execution statistics for a specific venue
type VenueStats struct {
	TotalExecutions  int       `json:"total_executions"`
	GoodExecutions   int       `json:"good_executions"`
	BadExecutions    int       `json:"bad_executions"`
	AvgSlippageBps   float64   `json:"avg_slippage_bps"`
	WorstSlippageBps float64   `json:"worst_slippage_bps"`
	LastExecution    time.Time `json:"last_execution"`
	ConsecutiveGood  int       `json:"consecutive_good"` // For recovery tracking
	IsTightened      bool      `json:"is_tightened"`
	TightenedAt      time.Time `json:"tightened_at,omitempty"`
}

// ExecutionQualityMetrics provides aggregate execution quality information
type ExecutionQualityMetrics struct {
	TotalExecutions   int                    `json:"total_executions"`
	GoodExecutionRate float64                `json:"good_execution_rate_pct"`
	AvgSlippageBps    float64                `json:"avg_slippage_bps"`
	TightenedVenues   []string               `json:"tightened_venues"`
	VenueBreakdown    map[string]*VenueStats `json:"venue_breakdown"`
	RecentExecutions  []ExecutionRecord      `json:"recent_executions"`
	Recovery          RecoveryStatus         `json:"recovery"`
}

// RecoveryStatus tracks recovery progress for tightened venues
type RecoveryStatus struct {
	VenuesInRecovery int                         `json:"venues_in_recovery"`
	RecoveryProgress map[string]RecoveryProgress `json:"recovery_progress"`
}

// RecoveryProgress tracks individual venue recovery
type RecoveryProgress struct {
	ConsecutiveGood   int       `json:"consecutive_good"`
	RequiredGood      int       `json:"required_good"`
	TimeBasedRecovery time.Time `json:"time_based_recovery"`
	CanRecover        bool      `json:"can_recover"`
}

// NewExecutionQualityTracker creates an execution quality tracker with default configuration
func NewExecutionQualityTracker() *ExecutionQualityTracker {
	return &ExecutionQualityTracker{
		slippageBpsTightenThreshold: 30.0,
		goodTradesThreshold:         20,
		recoveryWindowHours:         48,
		executions:                  make([]ExecutionRecord, 0),
		venueStats:                  make(map[string]*VenueStats),
		tightenedVenues:             make(map[string]time.Time),
	}
}

// NewExecutionQualityTrackerWithConfig creates a tracker with custom configuration
func NewExecutionQualityTrackerWithConfig(config ExecutionConfig) *ExecutionQualityTracker {
	return &ExecutionQualityTracker{
		slippageBpsTightenThreshold: config.SlippageBpsTightenThreshold,
		goodTradesThreshold:         config.GoodTradesThreshold,
		recoveryWindowHours:         config.RecoveryWindowHours,
		executions:                  make([]ExecutionRecord, 0),
		venueStats:                  make(map[string]*VenueStats),
		tightenedVenues:             make(map[string]time.Time),
	}
}

// RecordExecution records a new execution and updates quality metrics
func (eq *ExecutionQualityTracker) RecordExecution(execution ExecutionRecord) error {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	// Calculate slippage in basis points
	if execution.ExpectedPrice > 0 {
		slippage := ((execution.ActualPrice - execution.ExpectedPrice) / execution.ExpectedPrice) * 10000
		if execution.Side == "sell" {
			slippage = -slippage // For sells, lower price is bad slippage
		}
		execution.SlippageBps = slippage
	}

	// Classify execution quality
	execution.Quality = eq.classifyExecution(execution)
	execution.Timestamp = time.Now()

	// Add to execution history
	eq.executions = append(eq.executions, execution)

	// Maintain history size limit
	if len(eq.executions) > 1000 {
		eq.executions = eq.executions[len(eq.executions)-1000:]
	}

	// Update venue statistics
	eq.updateVenueStats(execution)

	// Check if venue should be tightened
	if execution.Quality == "bad" && !eq.isVenueTightened(execution.Venue) {
		eq.tightenVenue(execution.Venue)
	}

	// Check for recovery conditions
	if execution.Quality == "good" && eq.isVenueTightened(execution.Venue) {
		eq.checkVenueRecovery(execution.Venue)
	}

	return nil
}

// classifyExecution determines the quality of an execution
func (eq *ExecutionQualityTracker) classifyExecution(execution ExecutionRecord) string {
	slippageBps := execution.SlippageBps

	// Bad execution: slippage > tighten threshold
	if slippageBps > eq.slippageBpsTightenThreshold {
		return "bad"
	}

	// Good execution: slippage <= 10bps
	if slippageBps <= 10.0 {
		return "good"
	}

	// Acceptable execution: between 10bps and tighten threshold
	return "acceptable"
}

// updateVenueStats updates statistics for the execution venue
func (eq *ExecutionQualityTracker) updateVenueStats(execution ExecutionRecord) {
	venue := execution.Venue
	if eq.venueStats[venue] == nil {
		eq.venueStats[venue] = &VenueStats{}
	}

	stats := eq.venueStats[venue]
	stats.TotalExecutions++
	stats.LastExecution = execution.Timestamp

	// Update quality counts
	switch execution.Quality {
	case "good":
		stats.GoodExecutions++
		stats.ConsecutiveGood++
	case "bad":
		stats.BadExecutions++
		stats.ConsecutiveGood = 0
	default:
		stats.ConsecutiveGood = 0
	}

	// Update slippage statistics
	totalSlippage := stats.AvgSlippageBps * float64(stats.TotalExecutions-1)
	stats.AvgSlippageBps = (totalSlippage + execution.SlippageBps) / float64(stats.TotalExecutions)

	if execution.SlippageBps > stats.WorstSlippageBps {
		stats.WorstSlippageBps = execution.SlippageBps
	}

	// Update tightened status
	stats.IsTightened = eq.isVenueTightened(venue)
	if stats.IsTightened {
		stats.TightenedAt = eq.tightenedVenues[venue]
	}
}

// tightenVenue marks a venue as having tightened thresholds
func (eq *ExecutionQualityTracker) tightenVenue(venue string) {
	eq.tightenedVenues[venue] = time.Now()
	if eq.venueStats[venue] != nil {
		eq.venueStats[venue].IsTightened = true
		eq.venueStats[venue].TightenedAt = time.Now()
		eq.venueStats[venue].ConsecutiveGood = 0 // Reset recovery counter
	}
}

// checkVenueRecovery checks if a venue can recover from tightened thresholds
func (eq *ExecutionQualityTracker) checkVenueRecovery(venue string) {
	stats := eq.venueStats[venue]
	if stats == nil {
		return
	}

	tightenedAt, exists := eq.tightenedVenues[venue]
	if !exists {
		return
	}

	// Check trade-based recovery: 20 consecutive good trades
	if stats.ConsecutiveGood >= eq.goodTradesThreshold {
		eq.recoverVenue(venue, "trade_based")
		return
	}

	// Check time-based recovery: 48 hours
	if time.Since(tightenedAt) >= time.Duration(eq.recoveryWindowHours)*time.Hour {
		eq.recoverVenue(venue, "time_based")
		return
	}
}

// recoverVenue removes tightened status from a venue
func (eq *ExecutionQualityTracker) recoverVenue(venue string, recoveryType string) {
	delete(eq.tightenedVenues, venue)
	if eq.venueStats[venue] != nil {
		eq.venueStats[venue].IsTightened = false
		eq.venueStats[venue].TightenedAt = time.Time{}
	}
}

// isVenueTightened checks if a venue currently has tightened thresholds
func (eq *ExecutionQualityTracker) isVenueTightened(venue string) bool {
	_, exists := eq.tightenedVenues[venue]
	return exists
}

// ShouldTightenThreshold checks if execution thresholds should be tightened for a venue
func (eq *ExecutionQualityTracker) ShouldTightenThreshold(venue string) bool {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	return eq.isVenueTightened(venue)
}

// GetExecutionMetrics returns comprehensive execution quality metrics
func (eq *ExecutionQualityTracker) GetExecutionMetrics() *ExecutionQualityMetrics {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	totalExecutions := len(eq.executions)
	goodExecutions := 0
	totalSlippage := 0.0

	// Calculate aggregate statistics
	for _, exec := range eq.executions {
		if exec.Quality == "good" {
			goodExecutions++
		}
		totalSlippage += exec.SlippageBps
	}

	goodExecutionRate := 0.0
	avgSlippage := 0.0
	if totalExecutions > 0 {
		goodExecutionRate = (float64(goodExecutions) / float64(totalExecutions)) * 100.0
		avgSlippage = totalSlippage / float64(totalExecutions)
	}

	// Get tightened venues list
	tightenedVenues := make([]string, 0, len(eq.tightenedVenues))
	for venue := range eq.tightenedVenues {
		tightenedVenues = append(tightenedVenues, venue)
	}

	// Build recovery status
	recoveryProgress := make(map[string]RecoveryProgress)
	for venue, tightenedAt := range eq.tightenedVenues {
		stats := eq.venueStats[venue]
		consecutiveGood := 0
		if stats != nil {
			consecutiveGood = stats.ConsecutiveGood
		}

		recoveryProgress[venue] = RecoveryProgress{
			ConsecutiveGood:   consecutiveGood,
			RequiredGood:      eq.goodTradesThreshold,
			TimeBasedRecovery: tightenedAt.Add(time.Duration(eq.recoveryWindowHours) * time.Hour),
			CanRecover:        consecutiveGood >= eq.goodTradesThreshold || time.Since(tightenedAt) >= time.Duration(eq.recoveryWindowHours)*time.Hour,
		}
	}

	recovery := RecoveryStatus{
		VenuesInRecovery: len(eq.tightenedVenues),
		RecoveryProgress: recoveryProgress,
	}

	// Get recent executions (last 10)
	recentCount := 10
	if len(eq.executions) < recentCount {
		recentCount = len(eq.executions)
	}
	recentExecutions := make([]ExecutionRecord, recentCount)
	if recentCount > 0 {
		copy(recentExecutions, eq.executions[len(eq.executions)-recentCount:])
	}

	return &ExecutionQualityMetrics{
		TotalExecutions:   totalExecutions,
		GoodExecutionRate: goodExecutionRate,
		AvgSlippageBps:    avgSlippage,
		TightenedVenues:   tightenedVenues,
		VenueBreakdown:    eq.venueStats,
		RecentExecutions:  recentExecutions,
		Recovery:          recovery,
	}
}

// GetVenueQuality returns execution quality status for a specific venue
func (eq *ExecutionQualityTracker) GetVenueQuality(venue string) *VenueStats {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	return eq.venueStats[venue]
}

// ResetVenueStats clears statistics for a venue (for testing/admin use)
func (eq *ExecutionQualityTracker) ResetVenueStats(venue string) {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	delete(eq.venueStats, venue)
	delete(eq.tightenedVenues, venue)
}

// toExecMetrics converts map[string]interface{} to *ExecutionMetrics for UI compatibility
func toExecMetrics(data map[string]interface{}) *ExecutionMetrics {
	metrics := &ExecutionMetrics{
		LastUpdated: time.Now(),
	}

	if val, ok := data["total_executions"].(int); ok {
		metrics.TotalExecutions = val
	} else if val, ok := data["total_executions"].(float64); ok {
		metrics.TotalExecutions = int(val)
	}

	if val, ok := data["successful_executions"].(int); ok {
		metrics.SuccessfulExecutions = val
	} else if val, ok := data["successful_executions"].(float64); ok {
		metrics.SuccessfulExecutions = int(val)
	}

	if val, ok := data["avg_slippage_bps"].(float64); ok {
		metrics.AvgSlippageBps = val
	}

	if val, ok := data["avg_fill_time_ms"].(float64); ok {
		metrics.AvgFillTimeMs = val
	}

	if val, ok := data["avg_quality_score"].(float64); ok {
		metrics.AvgQualityScore = val
	}

	if val, ok := data["acceptable_slippage_rate"].(float64); ok {
		metrics.AcceptableSlippageRate = val
	}

	if val, ok := data["in_recovery_mode"].(bool); ok {
		metrics.InRecoveryMode = val
	}

	if val, ok := data["consecutive_fails"].(int); ok {
		metrics.ConsecutiveFails = val
	} else if val, ok := data["consecutive_fails"].(float64); ok {
		metrics.ConsecutiveFails = int(val)
	}

	return metrics
}
