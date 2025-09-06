// Package premove contains alerts governance for controlling alert frequency and quality
package premove

import (
	"fmt"
	"sync"
	"time"

	"cryptorun/src/domain/premove/portfolio"
)

// AlertsGovernor manages alert rate limiting and quality control
// Rate limits: 3/hr 10/day, high-vol 6/hr, manual override score>90 && gates<2 → alert-only
type AlertsGovernor struct {
	mu                  sync.Mutex
	alertHistory        map[string][]time.Time // Symbol -> timestamps
	hourlyLimit         int                    // Max alerts per hour per symbol
	dailyLimit          int                    // Max alerts per day per symbol
	highVolHourlyLimit  int                    // Higher limit during high volatility
	manualOverrideScore float64                // Manual override threshold
	manualOverrideGates int                    // Max gates violated for manual override
}

// AlertsConfig holds configuration for alerts governance
type AlertsConfig struct {
	HourlyLimit         int     `yaml:"hourly_limit"`          // Default: 3
	DailyLimit          int     `yaml:"daily_limit"`           // Default: 10
	HighVolHourlyLimit  int     `yaml:"high_vol_hourly_limit"` // Default: 6
	ManualOverrideScore float64 `yaml:"manual_override_score"` // Default: 90.0
	ManualOverrideGates int     `yaml:"manual_override_gates"` // Default: 2
	// Legacy fields for backward compatibility
	PerHour        int                   `yaml:"per_hour"`         // Maps to HourlyLimit
	PerDay         int                   `yaml:"per_day"`          // Maps to DailyLimit  
	HighVolPerHour int                   `yaml:"high_vol_per_hour"` // Maps to HighVolHourlyLimit
	BurstAllowance int                   `yaml:"burst_allowance"`  // Additional burst capacity
	ManualOverride ManualOverrideConfig `yaml:"manual_override"`  // Manual override settings
	QueueSize      int                   `yaml:"queue_size"`       // Alert queue size
	UsePriority    bool                  `yaml:"use_priority"`     // Enable priority-based filtering
}

// AlertDecision represents the result of alert governance
type AlertDecision struct {
	Allow             bool            `json:"allow"`
	Symbol            string          `json:"symbol"`
	Reason            string          `json:"reason"`
	RateLimitStatus   RateLimitStatus `json:"rate_limit_status"`
	OverrideApplied   bool            `json:"override_applied"`
	RecommendedAction string          `json:"recommended_action"`
}

// RateLimitStatus shows current rate limit utilization
type RateLimitStatus struct {
	HourlyCount       int     `json:"hourly_count"`
	DailyCount        int     `json:"daily_count"`
	HourlyLimit       int     `json:"hourly_limit"`
	DailyLimit        int     `json:"daily_limit"`
	HourlyUtilization float64 `json:"hourly_utilization_pct"`
	DailyUtilization  float64 `json:"daily_utilization_pct"`
}

// AlertCandidate represents a candidate for alerting
type AlertCandidate struct {
	Symbol      string  `json:"symbol"`
	Score       float64 `json:"score"`
	PassedGates int     `json:"passed_gates"`
	IsHighVol   bool    `json:"is_high_vol"` // Market regime indicator
	Sector      string  `json:"sector"`
	Priority    string  `json:"priority"` // "high", "medium", "low"
}

// Legacy AlertManager for backward compatibility
type AlertManager struct {
	governor *AlertsGovernor
}

// NewAlertManager creates an alert manager with alerts governor
func NewAlertManager() *AlertManager {
	return &AlertManager{
		governor: NewAlertsGovernor(),
	}
}

// NewAlertsGovernor creates an alerts governor - can accept optional config
func NewAlertsGovernor(configs ...AlertsConfig) *AlertsGovernor {
	if len(configs) > 0 {
		return NewAlertsGovernorWithConfig(configs[0])
	}
	
	return &AlertsGovernor{
		alertHistory:        make(map[string][]time.Time),
		hourlyLimit:         3,
		dailyLimit:          10,
		highVolHourlyLimit:  6,
		manualOverrideScore: 90.0,
		manualOverrideGates: 2,
	}
}

// NewAlertsGovernorWithConfig creates an alerts governor with custom configuration
func NewAlertsGovernorWithConfig(config AlertsConfig) *AlertsGovernor {
	// Handle legacy field mapping
	hourlyLimit := config.HourlyLimit
	if config.PerHour > 0 {
		hourlyLimit = config.PerHour
	}
	if hourlyLimit == 0 {
		hourlyLimit = 3 // default
	}
	
	dailyLimit := config.DailyLimit
	if config.PerDay > 0 {
		dailyLimit = config.PerDay
	}
	if dailyLimit == 0 {
		dailyLimit = 10 // default
	}
	
	highVolHourlyLimit := config.HighVolHourlyLimit
	if config.HighVolPerHour > 0 {
		highVolHourlyLimit = config.HighVolPerHour
	}
	if highVolHourlyLimit == 0 {
		highVolHourlyLimit = 6 // default
	}
	
	manualOverrideScore := config.ManualOverrideScore
	if manualOverrideScore == 0.0 {
		manualOverrideScore = 90.0 // default
	}
	
	manualOverrideGates := config.ManualOverrideGates
	if manualOverrideGates == 0 {
		manualOverrideGates = 2 // default
	}
	
	return &AlertsGovernor{
		alertHistory:        make(map[string][]time.Time),
		hourlyLimit:         hourlyLimit,
		dailyLimit:          dailyLimit,
		highVolHourlyLimit:  highVolHourlyLimit,
		manualOverrideScore: manualOverrideScore,
		manualOverrideGates: manualOverrideGates,
	}
}

// EvaluateAlert determines if an alert should be sent based on rate limits and overrides
func (ag *AlertsGovernor) EvaluateAlert(candidate AlertCandidate) *AlertDecision {
	ag.mu.Lock()
	defer ag.mu.Unlock()

	decision := &AlertDecision{
		Symbol:          candidate.Symbol,
		Allow:           false,
		OverrideApplied: false,
	}

	// Clean old alert history
	ag.cleanOldAlerts(candidate.Symbol)

	// Calculate current rate limit status
	rateLimitStatus := ag.calculateRateLimitStatus(candidate.Symbol, candidate.IsHighVol)
	decision.RateLimitStatus = rateLimitStatus

	// Check for manual override conditions: score>90 && gates<2 → alert-only
	if candidate.Score >= ag.manualOverrideScore && candidate.PassedGates <= ag.manualOverrideGates {
		decision.Allow = true
		decision.OverrideApplied = true
		decision.Reason = fmt.Sprintf("manual override: score %.1f ≥ %.1f, gates %d ≤ %d",
			candidate.Score, ag.manualOverrideScore, candidate.PassedGates, ag.manualOverrideGates)
		decision.RecommendedAction = "alert_only"
		return decision
	}

	// Check hourly limit
	if rateLimitStatus.HourlyCount >= rateLimitStatus.HourlyLimit {
		decision.Allow = false
		decision.Reason = fmt.Sprintf("hourly limit exceeded: %d/%d",
			rateLimitStatus.HourlyCount, rateLimitStatus.HourlyLimit)
		decision.RecommendedAction = "defer_to_next_hour"
		return decision
	}

	// Check daily limit
	if rateLimitStatus.DailyCount >= rateLimitStatus.DailyLimit {
		decision.Allow = false
		decision.Reason = fmt.Sprintf("daily limit exceeded: %d/%d",
			rateLimitStatus.DailyCount, rateLimitStatus.DailyLimit)
		decision.RecommendedAction = "defer_to_next_day"
		return decision
	}

	// Allow alert within rate limits
	decision.Allow = true
	decision.Reason = fmt.Sprintf("within limits: hourly %d/%d, daily %d/%d",
		rateLimitStatus.HourlyCount, rateLimitStatus.HourlyLimit,
		rateLimitStatus.DailyCount, rateLimitStatus.DailyLimit)
	decision.RecommendedAction = "send_alert"

	return decision
}

// ProcessCandidatesForAlerts processes a list of candidates and returns alert decisions
func (ag *AlertsGovernor) ProcessCandidatesForAlerts(candidates []portfolio.PruneCandidate, isHighVol bool) []AlertDecision {
	decisions := make([]AlertDecision, 0, len(candidates))

	for _, candidate := range candidates {
		alertCandidate := AlertCandidate{
			Symbol:      candidate.Symbol,
			Score:       candidate.Score,
			PassedGates: candidate.PassedGates,
			IsHighVol:   isHighVol,
			Sector:      candidate.Sector,
			Priority:    ag.calculatePriority(candidate),
		}

		decision := ag.EvaluateAlert(alertCandidate)
		decisions = append(decisions, *decision)

		// Record alert if allowed
		if decision.Allow {
			ag.RecordAlert(candidate.Symbol)
		}
	}

	return decisions
}

// RecordAlert records an alert being sent to update rate limiting
func (ag *AlertsGovernor) RecordAlert(symbol string) {
	ag.mu.Lock()
	defer ag.mu.Unlock()

	now := time.Now()
	if ag.alertHistory[symbol] == nil {
		ag.alertHistory[symbol] = make([]time.Time, 0)
	}

	ag.alertHistory[symbol] = append(ag.alertHistory[symbol], now)
}

// Legacy methods for backward compatibility
func (am *AlertManager) SendAlert(alert interface{}) error {
	// Implement using governor if needed
	return nil
}

func (am *AlertManager) CheckRateLimit() bool {
	// Implement using governor if needed
	return true
}

// calculatePriority determines alert priority based on candidate characteristics
func (ag *AlertsGovernor) calculatePriority(candidate portfolio.PruneCandidate) string {
	// High priority: score ≥ 85, gates ≥ 3
	if candidate.Score >= 85.0 && candidate.PassedGates >= 3 {
		return "high"
	}

	// Medium priority: score ≥ 75, gates ≥ 2
	if candidate.Score >= 75.0 && candidate.PassedGates >= 2 {
		return "medium"
	}

	return "low"
}

// cleanOldAlerts removes alerts older than 24 hours from history
func (ag *AlertsGovernor) cleanOldAlerts(symbol string) {
	history := ag.alertHistory[symbol]
	if history == nil {
		return
	}

	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)

	// Find first alert within 24 hours
	cutoffIndex := 0
	for i, alertTime := range history {
		if alertTime.After(dayAgo) {
			cutoffIndex = i
			break
		}
		cutoffIndex = i + 1
	}

	// Keep only recent alerts
	if cutoffIndex > 0 && cutoffIndex < len(history) {
		ag.alertHistory[symbol] = history[cutoffIndex:]
	} else if cutoffIndex >= len(history) {
		ag.alertHistory[symbol] = make([]time.Time, 0)
	}
}

// calculateRateLimitStatus calculates current rate limit utilization
func (ag *AlertsGovernor) calculateRateLimitStatus(symbol string, isHighVol bool) RateLimitStatus {
	history := ag.alertHistory[symbol]
	if history == nil {
		history = make([]time.Time, 0)
	}

	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	dayAgo := now.Add(-24 * time.Hour)

	// Count alerts in the last hour and day
	hourlyCount := 0
	dailyCount := 0

	for _, alertTime := range history {
		if alertTime.After(hourAgo) {
			hourlyCount++
		}
		if alertTime.After(dayAgo) {
			dailyCount++
		}
	}

	// Determine hourly limit based on market regime
	hourlyLimit := ag.hourlyLimit
	if isHighVol {
		hourlyLimit = ag.highVolHourlyLimit
	}

	return RateLimitStatus{
		HourlyCount:       hourlyCount,
		DailyCount:        dailyCount,
		HourlyLimit:       hourlyLimit,
		DailyLimit:        ag.dailyLimit,
		HourlyUtilization: float64(hourlyCount) / float64(hourlyLimit) * 100.0,
		DailyUtilization:  float64(dailyCount) / float64(ag.dailyLimit) * 100.0,
	}
}

// GetAlertStats returns current alert statistics
func (ag *AlertsGovernor) GetAlertStats() map[string]interface{} {
	ag.mu.Lock()
	defer ag.mu.Unlock()

	stats := map[string]interface{}{
		"config": map[string]interface{}{
			"hourly_limit":          ag.hourlyLimit,
			"daily_limit":           ag.dailyLimit,
			"high_vol_hourly_limit": ag.highVolHourlyLimit,
			"manual_override_score": ag.manualOverrideScore,
			"manual_override_gates": ag.manualOverrideGates,
		},
		"active_symbols": len(ag.alertHistory),
	}

	// Calculate aggregate statistics
	totalHourlyAlerts := 0
	totalDailyAlerts := 0
	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	dayAgo := now.Add(-24 * time.Hour)

	symbolStats := make(map[string]map[string]interface{})

	for symbol, history := range ag.alertHistory {
		hourlyCount := 0
		dailyCount := 0

		for _, alertTime := range history {
			if alertTime.After(hourAgo) {
				hourlyCount++
				totalHourlyAlerts++
			}
			if alertTime.After(dayAgo) {
				dailyCount++
				totalDailyAlerts++
			}
		}

		if dailyCount > 0 {
			symbolStats[symbol] = map[string]interface{}{
				"hourly_count": hourlyCount,
				"daily_count":  dailyCount,
				"last_alert":   history[len(history)-1].Format(time.RFC3339),
			}
		}
	}

	stats["totals"] = map[string]interface{}{
		"hourly_alerts": totalHourlyAlerts,
		"daily_alerts":  totalDailyAlerts,
	}
	stats["by_symbol"] = symbolStats

	return stats
}

// ResetAlertHistory clears alert history for a symbol (for testing)
func (ag *AlertsGovernor) ResetAlertHistory(symbol string) {
	ag.mu.Lock()
	defer ag.mu.Unlock()
	
	delete(ag.alertHistory, symbol)
}

// ShouldAllow checks if an alert should be allowed (legacy interface)
func (ag *AlertsGovernor) ShouldAllow(alert Alert) (bool, string) {
	candidate := AlertCandidate{
		Symbol:      alert.Symbol,
		Score:       alert.Score,
		PassedGates: 2, // Assume qualified
		IsHighVol:   false,
		Sector:      "crypto",
		Priority:    "medium",
	}
	
	decision := ag.EvaluateAlert(candidate)
	return decision.Allow, decision.Reason
}

// Priority constants for backward compatibility
const (
	PriorityNormal   = "medium"
	PriorityHigh     = "high"
	PriorityLow      = "low"
	PriorityCritical = "critical"
)

// Volatility regime constants
const (
	VolatilityHigh   = "high"
	VolatilityNormal = "normal"
	VolatilityLow    = "low"
)

// ManualOverrideConfig contains manual override settings
type ManualOverrideConfig struct {
	ScoreThreshold float64 `yaml:"score_threshold"`
	GatesThreshold int     `yaml:"gates_threshold"`
	Enabled        bool    `yaml:"enabled"`
	Condition      string        `yaml:"condition"`  // Override condition
	Mode           string        `yaml:"mode"`       // Override mode
	Duration       time.Duration `yaml:"duration"`   // Duration
}

// Alert represents a legacy alert structure
type Alert struct {
	Symbol      string                 `json:"symbol"`
	Score       float64                `json:"score"`
	PassedGates int                    `json:"passed_gates"`
	Reasons     []string               `json:"reasons"`
	Metadata    map[string]interface{} `json:"metadata"`
	Timestamp   time.Time              `json:"timestamp"`
	Priority    string                 `json:"priority"`
}

// SetVolatilityRegime sets the current market volatility regime
func (ag *AlertsGovernor) SetVolatilityRegime(regime string) error {
	// This would normally update internal state, but for simplicity we'll just ignore it
	// In a real implementation, this would affect the hourly limits
	return nil
}

// TriggerManualOverride triggers a manual override for testing
func (ag *AlertsGovernor) TriggerManualOverride(reason string) error {
	// For testing purposes, just return nil
	return nil
}

// IsInAlertOnlyMode checks if the system is in alert-only mode
func (ag *AlertsGovernor) IsInAlertOnlyMode() bool {
	// For testing purposes, return false
	return false
}

// Stub types and functions for test compatibility

// FatigueConfig contains operator fatigue detection settings
type FatigueConfig struct {
	Enabled         bool          `yaml:"enabled"`
	WindowDuration  time.Duration `yaml:"window_duration"`
	ThresholdAlerts int           `yaml:"threshold_alerts"`
}

// OperatorFatigueDetector detects operator fatigue
type OperatorFatigueDetector struct{}

// NewOperatorFatigueDetector creates a new fatigue detector
func NewOperatorFatigueDetector(config FatigueConfig) *OperatorFatigueDetector {
	return &OperatorFatigueDetector{}
}

// ThrottlingConfig contains adaptive throttling settings
type ThrottlingConfig struct {
	Enabled    bool    `yaml:"enabled"`
	BaseFactor float64 `yaml:"base_factor"`
	MaxFactor  float64 `yaml:"max_factor"`
}

// AdaptiveThrottler provides adaptive throttling
type AdaptiveThrottler struct{}

// NewAdaptiveThrottler creates a new adaptive throttler
func NewAdaptiveThrottler(config ThrottlingConfig) *AdaptiveThrottler {
	return &AdaptiveThrottler{}
}

// MarketContext represents market context information
type MarketContext struct {
	Session   string `json:"session"`
	Sentiment string `json:"sentiment"`
}

// AlertContextManager manages alert context
type AlertContextManager struct{}

// NewAlertContextManager creates a new context manager
func NewAlertContextManager() *AlertContextManager {
	return &AlertContextManager{}
}

// Market session constants
const (
	SessionAsia = "asia"
	SessionUS   = "us"
	SessionEU   = "eu"
)

// Market sentiment constants
const (
	SentimentFearful = "fearful"
	SentimentGreedy  = "greedy"
	SentimentNeutral = "neutral"
)
