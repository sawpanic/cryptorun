package microstructure

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// VenueHealthMonitor tracks venue operational health with real-time metrics
// Triggers: reject_rate>5%, p99_latency>2000ms, error_rate>3% → "halve_size"
type VenueHealthMonitor struct {
	mu             sync.RWMutex
	venues         map[string]*VenueMetrics
	config         *VenueHealthConfig
	windowDuration time.Duration
}

// VenueHealthConfig contains health monitoring configuration
type VenueHealthConfig struct {
	RejectRateThreshold float64       `yaml:"reject_rate_threshold"`  // 5.0% default
	LatencyThresholdMs  int64         `yaml:"latency_threshold_ms"`   // 2000ms default
	ErrorRateThreshold  float64       `yaml:"error_rate_threshold"`   // 3.0% default
	WindowDuration      time.Duration `yaml:"window_duration"`        // 15min default
	MinSamplesForHealth int           `yaml:"min_samples_for_health"` // 10 minimum
	MaxHistorySize      int           `yaml:"max_history_size"`       // 1000 max records
}

// NewVenueHealthMonitor creates a venue health monitor
func NewVenueHealthMonitor(config *VenueHealthConfig) *VenueHealthMonitor {
	if config == nil {
		config = defaultVenueHealthConfig()
	}

	return &VenueHealthMonitor{
		venues:         make(map[string]*VenueMetrics),
		config:         config,
		windowDuration: config.WindowDuration,
	}
}

// defaultVenueHealthConfig returns default health monitoring configuration
func defaultVenueHealthConfig() *VenueHealthConfig {
	return &VenueHealthConfig{
		RejectRateThreshold: 5.0,
		LatencyThresholdMs:  2000,
		ErrorRateThreshold:  3.0,
		WindowDuration:      15 * time.Minute,
		MinSamplesForHealth: 10,
		MaxHistorySize:      1000,
	}
}

// RecordRequest records an API request for health tracking
func (vhm *VenueHealthMonitor) RecordRequest(venue, endpoint string, latencyMs int64, success bool, statusCode int, errorCode string) {
	vhm.mu.Lock()
	defer vhm.mu.Unlock()

	if vhm.venues[venue] == nil {
		vhm.venues[venue] = &VenueMetrics{
			Venue:           venue,
			RecentRequests:  make([]RequestLog, 0, vhm.config.MaxHistorySize),
			RecentErrors:    make([]ErrorLog, 0, vhm.config.MaxHistorySize/2),
			HealthHistory:   make([]HealthPoint, 0, 100),
			LastHealthCheck: time.Now(),
		}
	}

	venueMetrics := vhm.venues[venue]

	// Record request
	request := RequestLog{
		Timestamp:  time.Now(),
		Endpoint:   endpoint,
		LatencyMs:  latencyMs,
		Success:    success,
		StatusCode: statusCode,
		ErrorCode:  errorCode,
	}

	venueMetrics.RecentRequests = append(venueMetrics.RecentRequests, request)

	// Trim history if needed
	if len(venueMetrics.RecentRequests) > vhm.config.MaxHistorySize {
		venueMetrics.RecentRequests = venueMetrics.RecentRequests[1:]
	}

	// Record error if request failed
	if !success {
		errorLog := ErrorLog{
			Timestamp:   time.Now(),
			Endpoint:    endpoint,
			ErrorType:   "api_error",
			ErrorCode:   errorCode,
			Message:     fmt.Sprintf("HTTP %d: %s", statusCode, errorCode),
			Recoverable: statusCode < 500, // Client errors are recoverable, server errors may not be
		}

		venueMetrics.RecentErrors = append(venueMetrics.RecentErrors, errorLog)

		// Trim error history
		if len(venueMetrics.RecentErrors) > vhm.config.MaxHistorySize/2 {
			venueMetrics.RecentErrors = venueMetrics.RecentErrors[1:]
		}
	}
}

// GetVenueHealth evaluates current venue health status
func (vhm *VenueHealthMonitor) GetVenueHealth(venue string) (*VenueHealthStatus, error) {
	vhm.mu.RLock()
	defer vhm.mu.RUnlock()

	venueMetrics, exists := vhm.venues[venue]
	if !exists {
		// Return default healthy status for unknown venues
		return &VenueHealthStatus{
			Healthy:        true,
			RejectRate:     0.0,
			LatencyP99Ms:   100,
			ErrorRate:      0.0,
			LastUpdate:     time.Now(),
			Recommendation: "full_size",
			UptimePercent:  100.0,
		}, nil
	}

	now := time.Now()
	windowStart := now.Add(-vhm.windowDuration)

	// Filter requests within window
	var windowRequests []RequestLog
	var windowErrors []ErrorLog

	for _, req := range venueMetrics.RecentRequests {
		if req.Timestamp.After(windowStart) {
			windowRequests = append(windowRequests, req)
		}
	}

	for _, err := range venueMetrics.RecentErrors {
		if err.Timestamp.After(windowStart) {
			windowErrors = append(windowErrors, err)
		}
	}

	// Calculate metrics
	health := &VenueHealthStatus{
		LastUpdate: now,
	}

	if len(windowRequests) < vhm.config.MinSamplesForHealth {
		// Insufficient data - assume healthy but note sparse data
		health.Healthy = true
		health.Recommendation = "full_size"
		health.UptimePercent = 100.0
		return health, nil
	}

	// Calculate reject rate (failed requests / total requests)
	rejectedCount := 0
	latencies := make([]int64, 0, len(windowRequests))

	for _, req := range windowRequests {
		if !req.Success {
			rejectedCount++
		}
		latencies = append(latencies, req.LatencyMs)
	}

	health.RejectRate = float64(rejectedCount) / float64(len(windowRequests)) * 100.0

	// Calculate P99 latency
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		p99Index := int(math.Ceil(0.99*float64(len(latencies)))) - 1
		if p99Index < 0 {
			p99Index = 0
		}
		if p99Index >= len(latencies) {
			p99Index = len(latencies) - 1
		}

		health.LatencyP99Ms = latencies[p99Index]
	}

	// Calculate error rate (errors / total requests)
	health.ErrorRate = float64(len(windowErrors)) / float64(len(windowRequests)) * 100.0

	// Calculate uptime (successful requests / total requests)
	successfulCount := len(windowRequests) - rejectedCount
	health.UptimePercent = float64(successfulCount) / float64(len(windowRequests)) * 100.0

	// Determine overall health status
	health.Healthy = true
	reasons := []string{}

	if health.RejectRate > vhm.config.RejectRateThreshold {
		health.Healthy = false
		reasons = append(reasons, fmt.Sprintf("reject_rate %.1f%% > %.1f%%",
			health.RejectRate, vhm.config.RejectRateThreshold))
	}

	if health.LatencyP99Ms > vhm.config.LatencyThresholdMs {
		health.Healthy = false
		reasons = append(reasons, fmt.Sprintf("p99_latency %dms > %dms",
			health.LatencyP99Ms, vhm.config.LatencyThresholdMs))
	}

	if health.ErrorRate > vhm.config.ErrorRateThreshold {
		health.Healthy = false
		reasons = append(reasons, fmt.Sprintf("error_rate %.1f%% > %.1f%%",
			health.ErrorRate, vhm.config.ErrorRateThreshold))
	}

	// Determine recommendation
	if health.Healthy {
		health.Recommendation = "full_size"
	} else if len(reasons) == 1 && health.ErrorRate <= vhm.config.ErrorRateThreshold*2 {
		// Minor issues - halve size
		health.Recommendation = "halve_size"
	} else {
		// Major issues - avoid venue
		health.Recommendation = "avoid"
	}

	// Record health point for history
	healthPoint := HealthPoint{
		Timestamp:    now,
		Healthy:      health.Healthy,
		RejectRate:   health.RejectRate,
		LatencyP99Ms: health.LatencyP99Ms,
		ErrorRate:    health.ErrorRate,
	}

	venueMetrics.HealthHistory = append(venueMetrics.HealthHistory, healthPoint)

	// Trim health history
	if len(venueMetrics.HealthHistory) > 100 {
		venueMetrics.HealthHistory = venueMetrics.HealthHistory[1:]
	}

	venueMetrics.LastHealthCheck = now

	return health, nil
}

// GetAllVenueHealth returns health status for all monitored venues
func (vhm *VenueHealthMonitor) GetAllVenueHealth() (map[string]*VenueHealthStatus, error) {
	vhm.mu.RLock()
	venueNames := make([]string, 0, len(vhm.venues))
	for venue := range vhm.venues {
		venueNames = append(venueNames, venue)
	}
	vhm.mu.RUnlock()

	result := make(map[string]*VenueHealthStatus)

	for _, venue := range venueNames {
		health, err := vhm.GetVenueHealth(venue)
		if err != nil {
			return nil, fmt.Errorf("failed to get health for venue %s: %w", venue, err)
		}
		result[venue] = health
	}

	return result, nil
}

// RecordOrderReject records an order rejection for health tracking
func (vhm *VenueHealthMonitor) RecordOrderReject(venue, reason string) {
	vhm.RecordRequest(venue, "order", 0, false, 400, reason)
}

// RecordConnectionError records a connection error
func (vhm *VenueHealthMonitor) RecordConnectionError(venue, errorType string) {
	vhm.RecordRequest(venue, "connection", 0, false, 500, errorType)
}

// IsVenueHealthy provides a simple boolean health check
func (vhm *VenueHealthMonitor) IsVenueHealthy(venue string) bool {
	health, err := vhm.GetVenueHealth(venue)
	if err != nil {
		return false // Assume unhealthy if we can't determine
	}
	return health.Healthy
}

// GetVenueRecommendation returns sizing recommendation for a venue
func (vhm *VenueHealthMonitor) GetVenueRecommendation(venue string) string {
	health, err := vhm.GetVenueHealth(venue)
	if err != nil {
		return "avoid"
	}
	return health.Recommendation
}

// GetHealthSummary returns a summary of all venue health
func (vhm *VenueHealthMonitor) GetHealthSummary() map[string]interface{} {
	vhm.mu.RLock()
	defer vhm.mu.RUnlock()

	summary := map[string]interface{}{
		"total_venues":     len(vhm.venues),
		"monitoring_since": time.Now().Add(-vhm.windowDuration),
		"window_duration":  vhm.windowDuration.String(),
		"thresholds": map[string]interface{}{
			"reject_rate_pct": vhm.config.RejectRateThreshold,
			"latency_p99_ms":  vhm.config.LatencyThresholdMs,
			"error_rate_pct":  vhm.config.ErrorRateThreshold,
		},
	}

	healthyCount := 0
	venueStats := make(map[string]interface{})

	for venue := range vhm.venues {
		health, err := vhm.GetVenueHealth(venue)
		if err != nil {
			continue
		}

		if health.Healthy {
			healthyCount++
		}

		venueStats[venue] = map[string]interface{}{
			"healthy":        health.Healthy,
			"recommendation": health.Recommendation,
			"reject_rate":    health.RejectRate,
			"latency_p99":    health.LatencyP99Ms,
			"error_rate":     health.ErrorRate,
			"uptime_pct":     health.UptimePercent,
		}
	}

	summary["healthy_venues"] = healthyCount
	summary["health_rate"] = float64(healthyCount) / float64(len(vhm.venues))
	summary["venues"] = venueStats

	return summary
}

// ClearHistory clears all venue health history (useful for testing)
func (vhm *VenueHealthMonitor) ClearHistory() {
	vhm.mu.Lock()
	defer vhm.mu.Unlock()

	vhm.venues = make(map[string]*VenueMetrics)
}
