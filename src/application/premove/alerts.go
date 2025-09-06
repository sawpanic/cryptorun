package premove

import (
	"fmt"
	"sync"
	"time"
)

// AlertManager handles rate-limited alerts with manual override support
type AlertManager struct {
	mu               sync.RWMutex
	config           AlertConfig
	alertHistory     []AlertRecord
	hourlyBucket     map[int64]int // alerts per hour bucket
	dailyBucket      map[int64]int // alerts per day bucket
	manualOverrides  []ManualOverride
	currentOverride  *ManualOverride
	rateLimitedCount int64
	totalAlertsCount int64
}

// AlertConfig defines alert rate limiting and governance settings
type AlertConfig struct {
	MaxAlertsPerHour      int   `yaml:"per_hour" json:"per_hour"`
	MaxAlertsPerDay       int   `yaml:"per_day" json:"per_day"`
	VolatilityAllowance   int   `yaml:"volatility_allowance" json:"volatility_allowance"`
	ManualOverrideEnabled bool  `yaml:"enabled" json:"enabled"`
	OverrideDurationSec   int64 `yaml:"duration_s" json:"duration_s"`
	MaxOverridesPerDay    int   `yaml:"max_per_day" json:"max_per_day"`
}

// AlertRecord represents a single alert event
type AlertRecord struct {
	ID        string                 `json:"id"`
	Symbol    string                 `json:"symbol"`
	AlertType string                 `json:"alert_type"` // "pre_movement", "manual_override"
	Severity  string                 `json:"severity"`   // "low", "medium", "high", "critical"
	Score     float64                `json:"score"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Source    string                 `json:"source"` // "detector", "manual", "override"

	// Rate limiting context
	HourBucket    int64 `json:"hour_bucket"`
	DayBucket     int64 `json:"day_bucket"`
	IsRateLimited bool  `json:"is_rate_limited"`

	// Processing status
	Status string    `json:"status"` // "pending", "sent", "rate_limited", "suppressed"
	SentAt time.Time `json:"sent_at,omitempty"`
}

// ManualOverride represents a manual override of rate limiting
type ManualOverride struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id,omitempty"`
	Reason              string    `json:"reason"`
	StartTime           time.Time `json:"start_time"`
	EndTime             time.Time `json:"end_time"`
	IsActive            bool      `json:"is_active"`
	AlertsUnderOverride []string  `json:"alerts_under_override"`
}

// NewAlertManager creates a new alert manager with specified configuration
func NewAlertManager(config AlertConfig) *AlertManager {
	return &AlertManager{
		config:           config,
		alertHistory:     make([]AlertRecord, 0),
		hourlyBucket:     make(map[int64]int),
		dailyBucket:      make(map[int64]int),
		manualOverrides:  make([]ManualOverride, 0),
		currentOverride:  nil,
		rateLimitedCount: 0,
		totalAlertsCount: 0,
	}
}

// ProcessAlert processes an incoming alert with rate limiting
func (am *AlertManager) ProcessAlert(alert AlertRecord) (*AlertRecord, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Set timestamp if not provided
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	// Generate ID if not provided
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert_%d_%s", alert.Timestamp.Unix(), alert.Symbol)
	}

	// Calculate time buckets
	alert.HourBucket = alert.Timestamp.Unix() / 3600
	alert.DayBucket = alert.Timestamp.Unix() / 86400

	am.totalAlertsCount++

	// Check if manual override is active
	if am.isManualOverrideActive() {
		alert.Status = "sent"
		alert.Source = "override"
		alert.SentAt = time.Now()
		am.currentOverride.AlertsUnderOverride = append(am.currentOverride.AlertsUnderOverride, alert.ID)
	} else {
		// Apply rate limiting
		if am.isRateLimited(alert.Timestamp, alert.Severity) {
			alert.IsRateLimited = true
			alert.Status = "rate_limited"
			am.rateLimitedCount++
		} else {
			alert.Status = "sent"
			alert.SentAt = time.Now()

			// Update buckets
			am.hourlyBucket[alert.HourBucket]++
			am.dailyBucket[alert.DayBucket]++
		}
	}

	// Store alert in history
	am.alertHistory = append(am.alertHistory, alert)

	// Clean old bucket data (keep only last 48 hours and 7 days)
	am.cleanOldBuckets(alert.Timestamp)

	return &alert, nil
}

// isRateLimited checks if an alert should be rate limited
func (am *AlertManager) isRateLimited(timestamp time.Time, severity string) bool {
	hourBucket := timestamp.Unix() / 3600
	dayBucket := timestamp.Unix() / 86400

	// Get current counts
	hourlyCount := am.hourlyBucket[hourBucket]
	dailyCount := am.dailyBucket[dayBucket]

	// Calculate effective limits (with volatility allowance for high severity)
	hourlyLimit := am.config.MaxAlertsPerHour
	dailyLimit := am.config.MaxAlertsPerDay

	if severity == "high" || severity == "critical" {
		hourlyLimit += am.config.VolatilityAllowance
		dailyLimit += am.config.VolatilityAllowance
	}

	// Check limits
	return hourlyCount >= hourlyLimit || dailyCount >= dailyLimit
}

// isManualOverrideActive checks if a manual override is currently active
func (am *AlertManager) isManualOverrideActive() bool {
	if am.currentOverride == nil {
		return false
	}

	now := time.Now()
	if now.After(am.currentOverride.EndTime) {
		// Override has expired
		am.currentOverride.IsActive = false
		am.currentOverride = nil
		return false
	}

	return am.currentOverride.IsActive
}

// ActivateManualOverride activates a manual override for rate limiting
func (am *AlertManager) ActivateManualOverride(userID, reason string) (*ManualOverride, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if !am.config.ManualOverrideEnabled {
		return nil, fmt.Errorf("manual overrides are disabled")
	}

	// Check daily override limit
	today := time.Now().Unix() / 86400
	overridesToday := 0
	for _, override := range am.manualOverrides {
		if override.StartTime.Unix()/86400 == today {
			overridesToday++
		}
	}

	if overridesToday >= am.config.MaxOverridesPerDay {
		return nil, fmt.Errorf("daily override limit reached: %d/%d", overridesToday, am.config.MaxOverridesPerDay)
	}

	// Deactivate any existing override
	if am.currentOverride != nil {
		am.currentOverride.IsActive = false
	}

	// Create new override
	override := ManualOverride{
		ID:                  fmt.Sprintf("override_%d", time.Now().Unix()),
		UserID:              userID,
		Reason:              reason,
		StartTime:           time.Now(),
		EndTime:             time.Now().Add(time.Duration(am.config.OverrideDurationSec) * time.Second),
		IsActive:            true,
		AlertsUnderOverride: make([]string, 0),
	}

	am.manualOverrides = append(am.manualOverrides, override)
	am.currentOverride = &override

	return &override, nil
}

// DeactivateManualOverride manually deactivates the current override
func (am *AlertManager) DeactivateManualOverride() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.currentOverride == nil {
		return fmt.Errorf("no active manual override")
	}

	am.currentOverride.IsActive = false
	am.currentOverride.EndTime = time.Now()
	am.currentOverride = nil

	return nil
}

// cleanOldBuckets removes old bucket data to prevent memory leaks
func (am *AlertManager) cleanOldBuckets(currentTime time.Time) {
	currentHour := currentTime.Unix() / 3600
	currentDay := currentTime.Unix() / 86400

	// Keep only last 48 hours
	for bucket := range am.hourlyBucket {
		if bucket < currentHour-48 {
			delete(am.hourlyBucket, bucket)
		}
	}

	// Keep only last 7 days
	for bucket := range am.dailyBucket {
		if bucket < currentDay-7 {
			delete(am.dailyBucket, bucket)
		}
	}

	// Trim alert history to last 1000 alerts
	if len(am.alertHistory) > 1000 {
		am.alertHistory = am.alertHistory[len(am.alertHistory)-1000:]
	}
}

// GetAlertStats returns current alert statistics
func (am *AlertManager) GetAlertStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	now := time.Now()
	currentHour := now.Unix() / 3600
	currentDay := now.Unix() / 86400

	// Count recent alerts
	last24hCount := 0
	lastHourCount := 0
	rateLimitedLast24h := 0

	for _, alert := range am.alertHistory {
		alertHour := alert.Timestamp.Unix() / 3600
		alertDay := alert.Timestamp.Unix() / 86400

		if alertDay >= currentDay-1 {
			last24hCount++
			if alert.IsRateLimited {
				rateLimitedLast24h++
			}
		}

		if alertHour == currentHour {
			lastHourCount++
		}
	}

	return map[string]interface{}{
		"total_alerts":       am.totalAlertsCount,
		"rate_limited_total": am.rateLimitedCount,
		"last_24h":           last24hCount,
		"last_hour":          lastHourCount,
		"rate_limited_24h":   rateLimitedLast24h,
		"current_limits": map[string]interface{}{
			"hourly_limit": am.config.MaxAlertsPerHour,
			"daily_limit":  am.config.MaxAlertsPerDay,
			"hourly_used":  am.hourlyBucket[currentHour],
			"daily_used":   am.dailyBucket[currentDay],
		},
		"manual_override": map[string]interface{}{
			"enabled":          am.config.ManualOverrideEnabled,
			"active":           am.isManualOverrideActive(),
			"current_override": am.currentOverride,
		},
		"rates": map[string]interface{}{
			"rate_limit_percentage": (float64(am.rateLimitedCount) / float64(am.totalAlertsCount)) * 100.0,
		},
	}
}

// GetRecentAlerts returns recent alert records
func (am *AlertManager) GetRecentAlerts(limit int) []AlertRecord {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if limit <= 0 || limit > len(am.alertHistory) {
		limit = len(am.alertHistory)
	}

	start := len(am.alertHistory) - limit
	if start < 0 {
		start = 0
	}

	recent := make([]AlertRecord, limit)
	copy(recent, am.alertHistory[start:])

	// Reverse to get most recent first
	for i := 0; i < len(recent)/2; i++ {
		j := len(recent) - 1 - i
		recent[i], recent[j] = recent[j], recent[i]
	}

	return recent
}

// GetAlertsByTimeRange returns alerts within a specific time range
func (am *AlertManager) GetAlertsByTimeRange(start, end time.Time) []AlertRecord {
	am.mu.RLock()
	defer am.mu.RUnlock()

	filtered := make([]AlertRecord, 0)
	for _, alert := range am.alertHistory {
		if (alert.Timestamp.Equal(start) || alert.Timestamp.After(start)) &&
			(alert.Timestamp.Equal(end) || alert.Timestamp.Before(end)) {
			filtered = append(filtered, alert)
		}
	}

	return filtered
}

// CreatePreMovementAlert creates an alert for a pre-movement detection
func CreatePreMovementAlert(symbol string, score float64, reasons []string, metadata map[string]interface{}) AlertRecord {
	severity := "medium"
	if score >= 85 {
		severity = "high"
	} else if score >= 95 {
		severity = "critical"
	}

	message := fmt.Sprintf("Pre-movement detected for %s (score: %.1f)", symbol, score)
	if len(reasons) > 0 {
		message += fmt.Sprintf(" - %s", reasons[0])
	}

	return AlertRecord{
		Symbol:    symbol,
		AlertType: "pre_movement",
		Severity:  severity,
		Score:     score,
		Message:   message,
		Timestamp: time.Now(),
		Metadata:  metadata,
		Source:    "detector",
		Status:    "pending",
	}
}

// ValidateAlertConfig validates the alert configuration
func ValidateAlertConfig(config AlertConfig) error {
	if config.MaxAlertsPerHour <= 0 {
		return fmt.Errorf("max_alerts_per_hour must be positive")
	}

	if config.MaxAlertsPerDay <= 0 {
		return fmt.Errorf("max_alerts_per_day must be positive")
	}

	if config.MaxAlertsPerDay < config.MaxAlertsPerHour {
		return fmt.Errorf("daily limit (%d) must be >= hourly limit (%d)",
			config.MaxAlertsPerDay, config.MaxAlertsPerHour)
	}

	if config.VolatilityAllowance < 0 {
		return fmt.Errorf("volatility_allowance cannot be negative")
	}

	if config.OverrideDurationSec <= 0 {
		return fmt.Errorf("override_duration_s must be positive")
	}

	if config.MaxOverridesPerDay <= 0 {
		return fmt.Errorf("max_overrides_per_day must be positive")
	}

	return nil
}
