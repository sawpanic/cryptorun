package gates

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/internal/microstructure"
)

// PolicyMatrix coordinates venue fallback, depeg guards, and risk-off toggles
type PolicyMatrix struct {
	config           *PolicyMatrixConfig
	venueRanking     []VenueRank
	depegMonitor     *DepegMonitor
	riskOffDetector  *RiskOffDetector
	matrixMutex      sync.RWMutex
	
	// State tracking
	activeVenues     map[string]VenueStatus
	depegAlerts      map[string]DepegAlert
	riskOffMode      bool
	lastUpdate       time.Time
}

// PolicyMatrixConfig contains policy matrix configuration
type PolicyMatrixConfig struct {
	// Venue fallback configuration
	VenueFallbackEnabled    bool              `yaml:"venue_fallback_enabled"`    // Enable venue fallback logic
	PrimaryVenues          []string          `yaml:"primary_venues"`            // Primary venue preferences ["kraken", "coinbase", "binance"]
	FallbackTimeout        time.Duration     `yaml:"fallback_timeout"`          // Timeout before fallback (5s)
	MinHealthyVenues       int               `yaml:"min_healthy_venues"`        // Minimum required healthy venues (1)
	VenueHealthChecks      VenueHealthConfig `yaml:"venue_health_checks"`       // Health check configuration
	
	// Depeg guard configuration
	DepegGuardEnabled      bool    `yaml:"depeg_guard_enabled"`       // Enable stablecoin depeg detection
	DepegThresholdBps      float64 `yaml:"depeg_threshold_bps"`       // Depeg threshold in basis points (100 bps = 1%)
	DepegMonitoredCoins    []string `yaml:"depeg_monitored_coins"`    // ["USDT", "USDC", "DAI"]
	DepegCooldownMinutes   int     `yaml:"depeg_cooldown_minutes"`    // Cooldown after depeg event (30 min)
	
	// Risk-off configuration  
	RiskOffTogglesEnabled  bool              `yaml:"risk_off_toggles_enabled"`  // Enable risk-off mode detection
	RiskOffThresholds      RiskOffThresholds `yaml:"risk_off_thresholds"`       // Risk-off detection thresholds
	RiskOffCooldownMinutes int               `yaml:"risk_off_cooldown_minutes"` // Cooldown duration (60 min)
	
	// Policy integration
	StrictMode             bool `yaml:"strict_mode"`             // Fail immediately on policy violations
	PolicyTimeoutSeconds   int  `yaml:"policy_timeout_seconds"` // Policy evaluation timeout (10s)
}

// VenueHealthConfig contains venue health check parameters
type VenueHealthConfig struct {
	LatencyThresholdMs     int64   `yaml:"latency_threshold_ms"`     // Max P99 latency (2000ms)
	ErrorRateThreshold     float64 `yaml:"error_rate_threshold"`     // Max error rate (5.0%)
	RejectRateThreshold    float64 `yaml:"reject_rate_threshold"`    // Max reject rate (10.0%)
	UptimeThreshold        float64 `yaml:"uptime_threshold"`         // Min uptime (95.0%)
	HealthCheckIntervalSec int     `yaml:"health_check_interval_sec"` // Check interval (30s)
}

// RiskOffThresholds contains risk-off detection parameters  
type RiskOffThresholds struct {
	VIXSpike               float64 `yaml:"vix_spike"`                // VIX spike threshold (>30)
	BTCDrop24h             float64 `yaml:"btc_drop_24h"`             // BTC 24h drop (>-15%)
	StablecoinVolumeSpike  float64 `yaml:"stablecoin_volume_spike"`  // Stablecoin volume spike (>3x)
	FundingRateExtreme     float64 `yaml:"funding_rate_extreme"`     // Extreme funding rates (>0.1%)
}

// VenueRank defines venue preference ordering
type VenueRank struct {
	Venue    string  `json:"venue"`
	Priority int     `json:"priority"`    // Lower = higher priority
	Weight   float64 `json:"weight"`      // Weight for fallback selection
	Reason   string  `json:"reason"`      // Why this venue is ranked here
}

// VenueStatus tracks individual venue operational status
type VenueStatus struct {
	Venue              string                                `json:"venue"`
	Healthy            bool                                  `json:"healthy"`
	Health             *microstructure.VenueHealthStatus    `json:"health"`
	LastHealthCheck    time.Time                             `json:"last_health_check"`
	ConsecutiveFailures int                                  `json:"consecutive_failures"`
	FallbackEligible   bool                                  `json:"fallback_eligible"`
}

// DepegAlert represents a stablecoin depeg event
type DepegAlert struct {
	Stablecoin      string    `json:"stablecoin"`       // "USDT", "USDC", etc.
	CurrentPrice    float64   `json:"current_price"`    // Current price vs USD
	DepegBps        float64   `json:"depeg_bps"`        // Depeg magnitude in basis points
	Timestamp       time.Time `json:"timestamp"`        // When depeg was detected
	AlertLevel      string    `json:"alert_level"`      // "warning", "critical"
	RecommendedAction string  `json:"recommended_action"` // "halt", "reduce_size", "monitor"
}

// RiskOffDetector monitors market conditions for risk-off events
type RiskOffDetector struct {
	config        *RiskOffThresholds
	currentState  RiskOffState
	lastUpdate    time.Time
	cooldownUntil time.Time
}

// RiskOffState contains current risk-off assessment
type RiskOffState struct {
	Active         bool      `json:"active"`
	TriggerReasons []string  `json:"trigger_reasons"`
	Confidence     float64   `json:"confidence"`     // 0.0-1.0
	Severity       string    `json:"severity"`       // "low", "medium", "high"
	Timestamp      time.Time `json:"timestamp"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// DepegMonitor tracks stablecoin depegging events
type DepegMonitor struct {
	config           *PolicyMatrixConfig
	monitoredCoins   map[string]float64 // coin -> last known price
	activeAlerts     map[string]DepegAlert
	lastPriceUpdate  time.Time
}

// NewPolicyMatrix creates a new policy matrix coordinator
func NewPolicyMatrix(config *PolicyMatrixConfig) *PolicyMatrix {
	if config == nil {
		config = DefaultPolicyMatrixConfig()
	}

	pm := &PolicyMatrix{
		config:       config,
		venueRanking: getDefaultVenueRanking(),
		depegMonitor: NewDepegMonitor(config),
		riskOffDetector: NewRiskOffDetector(&config.RiskOffThresholds),
		activeVenues: make(map[string]VenueStatus),
		depegAlerts:  make(map[string]DepegAlert),
		riskOffMode:  false,
		lastUpdate:   time.Now(),
	}

	// Initialize venue status for primary venues
	for _, venue := range config.PrimaryVenues {
		pm.activeVenues[venue] = VenueStatus{
			Venue:               venue,
			Healthy:             true,
			LastHealthCheck:     time.Now(),
			ConsecutiveFailures: 0,
			FallbackEligible:    true,
		}
	}

	return pm
}

// DefaultPolicyMatrixConfig returns production-ready policy matrix configuration
func DefaultPolicyMatrixConfig() *PolicyMatrixConfig {
	return &PolicyMatrixConfig{
		// Venue fallback
		VenueFallbackEnabled: true,
		PrimaryVenues:       []string{"kraken", "coinbase", "binance", "okx"}, // Kraken preferred
		FallbackTimeout:     5 * time.Second,
		MinHealthyVenues:    1,
		VenueHealthChecks: VenueHealthConfig{
			LatencyThresholdMs:     2000,  // 2s max P99
			ErrorRateThreshold:     5.0,   // 5% max error rate
			RejectRateThreshold:    10.0,  // 10% max reject rate
			UptimeThreshold:        95.0,  // 95% min uptime
			HealthCheckIntervalSec: 30,    // 30s checks
		},

		// Depeg guard
		DepegGuardEnabled:    true,
		DepegThresholdBps:    100.0,                          // 1% depeg threshold
		DepegMonitoredCoins:  []string{"USDT", "USDC", "DAI"}, 
		DepegCooldownMinutes: 30,                             // 30 min cooldown

		// Risk-off toggles
		RiskOffTogglesEnabled:  true,
		RiskOffCooldownMinutes: 60, // 1 hour cooldown
		RiskOffThresholds: RiskOffThresholds{
			VIXSpike:              30.0,  // VIX > 30
			BTCDrop24h:            -15.0, // BTC down >15% in 24h
			StablecoinVolumeSpike: 3.0,   // 3x volume spike
			FundingRateExtreme:    0.1,   // >0.1% funding rate
		},

		// Integration
		StrictMode:           false, // Allow graceful degradation
		PolicyTimeoutSeconds: 10,   // 10s timeout
	}
}

// getDefaultVenueRanking returns default venue preference ranking
func getDefaultVenueRanking() []VenueRank {
	return []VenueRank{
		{Venue: "kraken", Priority: 1, Weight: 0.4, Reason: "Preferred exchange with best USD pairs"},
		{Venue: "coinbase", Priority: 2, Weight: 0.3, Reason: "High liquidity, regulatory compliance"},
		{Venue: "binance", Priority: 3, Weight: 0.2, Reason: "Largest volume but higher risk"},
		{Venue: "okx", Priority: 4, Weight: 0.1, Reason: "Backup option for fallback"},
	}
}

// PolicyEvaluationResult contains comprehensive policy evaluation results
type PolicyEvaluationResult struct {
	Symbol                string                 `json:"symbol"`
	Venue                 string                 `json:"venue"`
	Timestamp             time.Time              `json:"timestamp"`
	
	// Policy decisions
	VenueApproved         bool                   `json:"venue_approved"`
	VenueRecommendation   string                 `json:"venue_recommendation"`   // "proceed", "fallback", "halt"
	FallbackVenue         string                 `json:"fallback_venue,omitempty"`
	
	// Policy checks
	DepegCheck            *DepegCheckResult      `json:"depeg_check"`
	RiskOffCheck          *RiskOffCheckResult    `json:"risk_off_check"`
	VenueHealthCheck      *VenueHealthResult     `json:"venue_health_check"`
	
	// Overall assessment
	PolicyPassed          bool                   `json:"policy_passed"`
	PolicyViolations      []string               `json:"policy_violations,omitempty"`
	RecommendedAction     string                 `json:"recommended_action"`
	ConfidenceScore       float64                `json:"confidence_score"` // 0.0-1.0
	
	// Processing metadata
	EvaluationTimeMs      int64                  `json:"evaluation_time_ms"`
	FallbacksAttempted    int                    `json:"fallbacks_attempted"`
}

// DepegCheckResult contains stablecoin depeg check results
type DepegCheckResult struct {
	Checked               bool                   `json:"checked"`
	DepegDetected         bool                   `json:"depeg_detected"`
	AffectedStablecoins   []string               `json:"affected_stablecoins,omitempty"`
	MaxDepegBps           float64                `json:"max_depeg_bps"`
	RecommendedAction     string                 `json:"recommended_action"`
	CooldownActive        bool                   `json:"cooldown_active"`
	CooldownExpiresAt     *time.Time             `json:"cooldown_expires_at,omitempty"`
}

// RiskOffCheckResult contains risk-off mode check results  
type RiskOffCheckResult struct {
	Checked               bool                   `json:"checked"`
	RiskOffActive         bool                   `json:"risk_off_active"`
	TriggerReasons        []string               `json:"trigger_reasons,omitempty"`
	Confidence            float64                `json:"confidence"`
	Severity              string                 `json:"severity"`
	RecommendedAction     string                 `json:"recommended_action"`
	CooldownActive        bool                   `json:"cooldown_active"`
	CooldownExpiresAt     *time.Time             `json:"cooldown_expires_at,omitempty"`
}

// VenueHealthResult contains venue health assessment results
type VenueHealthResult struct {
	PrimaryVenue          string                 `json:"primary_venue"`
	VenueHealthy          bool                   `json:"venue_healthy"`
	HealthMetrics         *microstructure.VenueHealthStatus `json:"health_metrics,omitempty"`
	FallbackRequired      bool                   `json:"fallback_required"`
	AvailableFallbacks    []string               `json:"available_fallbacks,omitempty"`
	HealthyVenueCount     int                    `json:"healthy_venue_count"`
}

// EvaluatePolicy performs comprehensive policy matrix evaluation
func (pm *PolicyMatrix) EvaluatePolicy(ctx context.Context, symbol, requestedVenue string) (*PolicyEvaluationResult, error) {
	startTime := time.Now()
	
	result := &PolicyEvaluationResult{
		Symbol:              symbol,
		Venue:               requestedVenue,
		Timestamp:           startTime,
		VenueApproved:       false,
		PolicyPassed:        false,
		PolicyViolations:    []string{},
		FallbacksAttempted:  0,
	}

	// 1. Depeg Guard Check
	if pm.config.DepegGuardEnabled {
		depegResult, err := pm.checkDepegGuard(ctx, symbol)
		if err != nil {
			return nil, fmt.Errorf("depeg guard check failed: %w", err)
		}
		result.DepegCheck = depegResult
		
		if depegResult.DepegDetected {
			result.PolicyViolations = append(result.PolicyViolations, 
				fmt.Sprintf("Stablecoin depeg detected: %v", depegResult.AffectedStablecoins))
		}
	}

	// 2. Risk-Off Mode Check
	if pm.config.RiskOffTogglesEnabled {
		riskOffResult, err := pm.checkRiskOffMode(ctx)
		if err != nil {
			return nil, fmt.Errorf("risk-off check failed: %w", err)
		}
		result.RiskOffCheck = riskOffResult
		
		if riskOffResult.RiskOffActive {
			result.PolicyViolations = append(result.PolicyViolations, 
				fmt.Sprintf("Risk-off mode active: %v", riskOffResult.TriggerReasons))
		}
	}

	// 3. Venue Health & Fallback Logic
	if pm.config.VenueFallbackEnabled {
		venueResult, err := pm.evaluateVenueHealth(ctx, requestedVenue)
		if err != nil {
			return nil, fmt.Errorf("venue health evaluation failed: %w", err)
		}
		result.VenueHealthCheck = venueResult
		
		if !venueResult.VenueHealthy && venueResult.FallbackRequired {
			// Attempt venue fallback
			fallbackVenue, fallbackAttempts, err := pm.attemptVenueFallback(ctx, requestedVenue)
			if err != nil {
				result.PolicyViolations = append(result.PolicyViolations, 
					fmt.Sprintf("Venue fallback failed: %v", err))
			} else {
				result.FallbackVenue = fallbackVenue
				result.VenueRecommendation = "fallback"
			}
			result.FallbacksAttempted = fallbackAttempts
		} else if venueResult.VenueHealthy {
			result.VenueApproved = true
			result.VenueRecommendation = "proceed"
		}
	} else {
		// No fallback enabled, just check if venue is approved
		result.VenueApproved = pm.isVenueApproved(requestedVenue)
		if result.VenueApproved {
			result.VenueRecommendation = "proceed"
		}
	}

	// 4. Overall Policy Decision
	result.PolicyPassed = len(result.PolicyViolations) == 0 && 
		(result.VenueApproved || result.FallbackVenue != "")
	
	result.RecommendedAction = pm.determineRecommendedAction(result)
	result.ConfidenceScore = pm.calculateConfidenceScore(result)
	result.EvaluationTimeMs = time.Since(startTime).Milliseconds()

	return result, nil
}

// checkDepegGuard evaluates stablecoin depeg conditions
func (pm *PolicyMatrix) checkDepegGuard(ctx context.Context, symbol string) (*DepegCheckResult, error) {
	result := &DepegCheckResult{
		Checked: true,
		DepegDetected: false,
		AffectedStablecoins: []string{},
		MaxDepegBps: 0.0,
		RecommendedAction: "proceed",
	}

	// Check if this is a stablecoin pair that needs monitoring
	if !pm.isStablecoinPair(symbol) {
		// Not a stablecoin pair, skip depeg check
		result.RecommendedAction = "proceed"
		return result, nil
	}

	// Update depeg monitor with latest prices (mock for demo)
	err := pm.depegMonitor.UpdatePrices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update depeg monitor: %w", err)
	}

	// Check for active depeg alerts
	pm.matrixMutex.RLock()
	for stablecoin, alert := range pm.depegAlerts {
		if alert.DepegBps >= pm.config.DepegThresholdBps {
			result.DepegDetected = true
			result.AffectedStablecoins = append(result.AffectedStablecoins, stablecoin)
			if alert.DepegBps > result.MaxDepegBps {
				result.MaxDepegBps = alert.DepegBps
			}
		}
	}
	pm.matrixMutex.RUnlock()

	// Determine recommended action based on depeg severity
	if result.DepegDetected {
		if result.MaxDepegBps >= 200.0 { // >2% depeg
			result.RecommendedAction = "halt"
		} else if result.MaxDepegBps >= 100.0 { // >1% depeg
			result.RecommendedAction = "reduce_size"
		} else {
			result.RecommendedAction = "monitor"
		}
	}

	return result, nil
}

// checkRiskOffMode evaluates risk-off market conditions
func (pm *PolicyMatrix) checkRiskOffMode(ctx context.Context) (*RiskOffCheckResult, error) {
	result := &RiskOffCheckResult{
		Checked: true,
		RiskOffActive: false,
		TriggerReasons: []string{},
		Confidence: 0.0,
		Severity: "low",
		RecommendedAction: "proceed",
	}

	// Update risk-off detector with latest market data (mock for demo)
	err := pm.riskOffDetector.UpdateMarketData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update risk-off detector: %w", err)
	}

	// Check current risk-off state
	currentState := pm.riskOffDetector.GetCurrentState()
	result.RiskOffActive = currentState.Active
	result.TriggerReasons = currentState.TriggerReasons
	result.Confidence = currentState.Confidence
	result.Severity = currentState.Severity

	// Check cooldown status
	if time.Now().Before(pm.riskOffDetector.cooldownUntil) {
		result.CooldownActive = true
		result.CooldownExpiresAt = &pm.riskOffDetector.cooldownUntil
	}

	// Determine recommended action
	if result.RiskOffActive && !result.CooldownActive {
		switch result.Severity {
		case "high":
			result.RecommendedAction = "halt"
		case "medium":
			result.RecommendedAction = "reduce_size"
		case "low":
			result.RecommendedAction = "monitor"
		}
	}

	return result, nil
}

// evaluateVenueHealth assesses venue health and determines fallback needs
func (pm *PolicyMatrix) evaluateVenueHealth(ctx context.Context, requestedVenue string) (*VenueHealthResult, error) {
	result := &VenueHealthResult{
		PrimaryVenue:       requestedVenue,
		VenueHealthy:       false,
		FallbackRequired:   false,
		AvailableFallbacks: []string{},
		HealthyVenueCount:  0,
	}

	// Get venue status
	pm.matrixMutex.RLock()
	venueStatus, exists := pm.activeVenues[requestedVenue]
	pm.matrixMutex.RUnlock()

	if !exists {
		// Venue not tracked, check if it's in our primary venues
		if pm.isPrimaryVenue(requestedVenue) {
			// Initialize venue status
			pm.matrixMutex.Lock()
			pm.activeVenues[requestedVenue] = VenueStatus{
				Venue:               requestedVenue,
				Healthy:             true, // Assume healthy until proven otherwise
				LastHealthCheck:     time.Now(),
				ConsecutiveFailures: 0,
				FallbackEligible:    true,
			}
			pm.matrixMutex.Unlock()
			venueStatus = pm.activeVenues[requestedVenue]
		} else {
			return nil, fmt.Errorf("venue %s not in primary venues list", requestedVenue)
		}
	}

	result.VenueHealthy = venueStatus.Healthy
	result.HealthMetrics = venueStatus.Health

	// Count healthy venues
	healthyCount := 0
	availableFallbacks := []string{}
	
	pm.matrixMutex.RLock()
	for venue, status := range pm.activeVenues {
		if status.Healthy {
			healthyCount++
			if venue != requestedVenue && status.FallbackEligible {
				availableFallbacks = append(availableFallbacks, venue)
			}
		}
	}
	pm.matrixMutex.RUnlock()

	result.HealthyVenueCount = healthyCount
	result.AvailableFallbacks = availableFallbacks

	// Determine if fallback is required
	if !result.VenueHealthy {
		result.FallbackRequired = true
	} else if healthyCount < pm.config.MinHealthyVenues {
		result.FallbackRequired = true // Preemptive fallback
	}

	return result, nil
}

// attemptVenueFallback tries to find a healthy fallback venue
func (pm *PolicyMatrix) attemptVenueFallback(ctx context.Context, failedVenue string) (string, int, error) {
	attempts := 0
	
	// Sort venues by priority (lower priority number = higher preference)
	for _, venueRank := range pm.venueRanking {
		if venueRank.Venue == failedVenue {
			continue // Skip the failed venue
		}
		
		attempts++
		
		// Check if this venue is healthy
		pm.matrixMutex.RLock()
		venueStatus, exists := pm.activeVenues[venueRank.Venue]
		pm.matrixMutex.RUnlock()
		
		if exists && venueStatus.Healthy && venueStatus.FallbackEligible {
			return venueRank.Venue, attempts, nil
		}
	}
	
	return "", attempts, fmt.Errorf("no healthy fallback venues available after %d attempts", attempts)
}

// Helper methods

func (pm *PolicyMatrix) isVenueApproved(venue string) bool {
	for _, primaryVenue := range pm.config.PrimaryVenues {
		if strings.ToLower(venue) == strings.ToLower(primaryVenue) {
			return true
		}
	}
	return false
}

func (pm *PolicyMatrix) isPrimaryVenue(venue string) bool {
	return pm.isVenueApproved(venue)
}

func (pm *PolicyMatrix) isStablecoinPair(symbol string) bool {
	symbolUpper := strings.ToUpper(symbol)
	for _, coin := range pm.config.DepegMonitoredCoins {
		if strings.Contains(symbolUpper, coin) {
			return true
		}
	}
	return false
}

func (pm *PolicyMatrix) determineRecommendedAction(result *PolicyEvaluationResult) string {
	if !result.PolicyPassed {
		// Check severity of violations
		if result.RiskOffCheck != nil && result.RiskOffCheck.RiskOffActive && result.RiskOffCheck.Severity == "high" {
			return "halt"
		}
		if result.DepegCheck != nil && result.DepegCheck.DepegDetected && result.DepegCheck.MaxDepegBps >= 200.0 {
			return "halt"
		}
		return "defer"
	}
	
	if result.FallbackVenue != "" {
		return "proceed_with_fallback"
	}
	
	return "proceed"
}

func (pm *PolicyMatrix) calculateConfidenceScore(result *PolicyEvaluationResult) float64 {
	score := 1.0
	
	// Reduce confidence for policy violations
	if len(result.PolicyViolations) > 0 {
		score -= float64(len(result.PolicyViolations)) * 0.2
	}
	
	// Reduce confidence for fallback usage
	if result.FallbackVenue != "" {
		score -= 0.1
	}
	
	// Reduce confidence for multiple fallback attempts
	if result.FallbacksAttempted > 1 {
		score -= float64(result.FallbacksAttempted-1) * 0.05
	}
	
	// Factor in risk-off confidence if available
	if result.RiskOffCheck != nil && result.RiskOffCheck.RiskOffActive {
		score *= result.RiskOffCheck.Confidence
	}
	
	if score < 0.0 {
		score = 0.0
	}
	
	return score
}

// UpdateVenueHealth updates the health status of a venue
func (pm *PolicyMatrix) UpdateVenueHealth(venue string, health *microstructure.VenueHealthStatus) error {
	pm.matrixMutex.Lock()
	defer pm.matrixMutex.Unlock()
	
	status, exists := pm.activeVenues[venue]
	if !exists {
		// Initialize new venue status
		status = VenueStatus{
			Venue:               venue,
			LastHealthCheck:     time.Now(),
			FallbackEligible:    pm.isPrimaryVenue(venue),
		}
	}
	
	status.Health = health
	status.Healthy = health.Healthy
	status.LastHealthCheck = time.Now()
	
	if !health.Healthy {
		status.ConsecutiveFailures++
	} else {
		status.ConsecutiveFailures = 0
	}
	
	// Disable fallback eligibility if too many consecutive failures
	if status.ConsecutiveFailures >= 5 {
		status.FallbackEligible = false
	}
	
	pm.activeVenues[venue] = status
	pm.lastUpdate = time.Now()
	
	return nil
}

// GetPolicyStatus returns current policy matrix status
func (pm *PolicyMatrix) GetPolicyStatus() map[string]interface{} {
	pm.matrixMutex.RLock()
	defer pm.matrixMutex.RUnlock()
	
	healthyVenues := 0
	for _, status := range pm.activeVenues {
		if status.Healthy {
			healthyVenues++
		}
	}
	
	return map[string]interface{}{
		"venue_fallback_enabled":    pm.config.VenueFallbackEnabled,
		"depeg_guard_enabled":       pm.config.DepegGuardEnabled,
		"risk_off_toggles_enabled":  pm.config.RiskOffTogglesEnabled,
		"healthy_venues":            healthyVenues,
		"total_venues":              len(pm.activeVenues),
		"risk_off_mode":             pm.riskOffMode,
		"active_depeg_alerts":       len(pm.depegAlerts),
		"last_update":               pm.lastUpdate,
		"primary_venues":            pm.config.PrimaryVenues,
	}
}

// Depeg Monitor Implementation

// NewDepegMonitor creates a new depeg monitor
func NewDepegMonitor(config *PolicyMatrixConfig) *DepegMonitor {
	return &DepegMonitor{
		config:          config,
		monitoredCoins:  make(map[string]float64),
		activeAlerts:    make(map[string]DepegAlert),
		lastPriceUpdate: time.Now(),
	}
}

// UpdatePrices updates stablecoin prices and checks for depegs (mock implementation)
func (dm *DepegMonitor) UpdatePrices(ctx context.Context) error {
	// Mock price updates - in production, this would fetch from APIs
	mockPrices := map[string]float64{
		"USDT": 1.0001, // Slight premium
		"USDC": 0.9999, // Slight discount  
		"DAI":  1.0005, // Slight premium
	}
	
	for coin, price := range mockPrices {
		dm.monitoredCoins[coin] = price
		
		// Check for depeg
		depegBps := (price - 1.0) * 10000 // Convert to basis points
		if abs(depegBps) >= dm.config.DepegThresholdBps {
			alert := DepegAlert{
				Stablecoin:    coin,
				CurrentPrice:  price,
				DepegBps:      depegBps,
				Timestamp:     time.Now(),
				AlertLevel:    "warning",
			}
			
			if abs(depegBps) >= 200.0 { // >2% depeg is critical
				alert.AlertLevel = "critical"
				alert.RecommendedAction = "halt"
			} else {
				alert.RecommendedAction = "monitor"
			}
			
			dm.activeAlerts[coin] = alert
		} else {
			// Remove alert if back to peg
			delete(dm.activeAlerts, coin)
		}
	}
	
	dm.lastPriceUpdate = time.Now()
	return nil
}

// Risk-Off Detector Implementation

// NewRiskOffDetector creates a new risk-off detector
func NewRiskOffDetector(thresholds *RiskOffThresholds) *RiskOffDetector {
	return &RiskOffDetector{
		config: thresholds,
		currentState: RiskOffState{
			Active:     false,
			Confidence: 0.0,
			Severity:   "low",
			Timestamp:  time.Now(),
		},
		lastUpdate: time.Now(),
	}
}

// UpdateMarketData updates market conditions and assesses risk-off state (mock implementation) 
func (rod *RiskOffDetector) UpdateMarketData(ctx context.Context) error {
	// Mock market data - in production, this would fetch real market indicators
	mockData := struct {
		VIX           float64
		BTCChange24h  float64
		StablecoinVol float64
		FundingRate   float64
	}{
		VIX:           25.0,  // Normal VIX
		BTCChange24h:  -5.0,  // Moderate decline
		StablecoinVol: 1.2,   // Normal volume
		FundingRate:   0.05,  // Normal funding
	}
	
	triggers := []string{}
	confidence := 0.0
	
	// Check each risk-off threshold
	if mockData.VIX > rod.config.VIXSpike {
		triggers = append(triggers, fmt.Sprintf("VIX spike: %.1f > %.1f", mockData.VIX, rod.config.VIXSpike))
		confidence += 0.3
	}
	
	if mockData.BTCChange24h < rod.config.BTCDrop24h {
		triggers = append(triggers, fmt.Sprintf("BTC drop: %.1f%% < %.1f%%", mockData.BTCChange24h, rod.config.BTCDrop24h))
		confidence += 0.4
	}
	
	if mockData.StablecoinVol > rod.config.StablecoinVolumeSpike {
		triggers = append(triggers, fmt.Sprintf("Stablecoin volume spike: %.1fx > %.1fx", mockData.StablecoinVol, rod.config.StablecoinVolumeSpike))
		confidence += 0.2
	}
	
	if abs(mockData.FundingRate) > rod.config.FundingRateExtreme {
		triggers = append(triggers, fmt.Sprintf("Extreme funding rate: %.3f%% > %.3f%%", mockData.FundingRate, rod.config.FundingRateExtreme))
		confidence += 0.1
	}
	
	// Update state
	rod.currentState.TriggerReasons = triggers
	rod.currentState.Confidence = confidence
	rod.currentState.Active = len(triggers) > 0 && confidence >= 0.3
	rod.currentState.Timestamp = time.Now()
	
	// Determine severity
	if confidence >= 0.7 {
		rod.currentState.Severity = "high"
	} else if confidence >= 0.4 {
		rod.currentState.Severity = "medium"
	} else {
		rod.currentState.Severity = "low"
	}
	
	rod.lastUpdate = time.Now()
	return nil
}

// GetCurrentState returns the current risk-off assessment
func (rod *RiskOffDetector) GetCurrentState() RiskOffState {
	return rod.currentState
}

// Helper function for absolute value (already defined in other test files)
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}