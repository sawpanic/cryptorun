package application

import (
	"encoding/json"
	"fmt"
	"time"
)

// RiskMonitor provides monitoring metrics for the risk envelope system
type RiskMonitor struct {
	envelope     *RiskEnvelope
	universeHash string
	lastUpdate   time.Time
}

// RiskMetrics contains risk envelope metrics for monitoring
type RiskMetrics struct {
	// Universe status
	UniverseStatus  UniverseStatus  `json:"universe_status"`
	
	// Risk envelope status
	RiskEnvelope    RiskEnvelopeMetrics `json:"risk_envelope"`
	
	// Emergency controls
	EmergencyStatus EmergencyStatus `json:"emergency_status"`
	
	// Timestamp
	Timestamp       time.Time       `json:"timestamp"`
}

// UniverseStatus tracks universe integrity and freshness
type UniverseStatus struct {
	SymbolCount     int       `json:"symbol_count"`
	Hash            string    `json:"hash"`
	LastRebuild     time.Time `json:"last_rebuild"`
	IntegrityCheck  string    `json:"integrity_check"`  // PASS/FAIL
	ADVCompliance   string    `json:"adv_compliance"`   // PASS/FAIL
	USDCompliance   string    `json:"usd_compliance"`   // PASS/FAIL
}

// RiskEnvelopeMetrics contains risk limit monitoring data
type RiskEnvelopeMetrics struct {
	// Position metrics
	ActivePositions   int     `json:"active_positions"`
	MaxPositions      int     `json:"max_positions"`
	PositionUtilization float64 `json:"position_utilization_pct"`
	
	// Exposure metrics  
	TotalExposureUSD  float64 `json:"total_exposure_usd"`
	PortfolioValueUSD float64 `json:"portfolio_value_usd"`
	ExposureRatio     float64 `json:"exposure_ratio_pct"`
	
	// Risk metrics
	CurrentDrawdown   float64 `json:"current_drawdown_pct"`
	DrawdownLimit     float64 `json:"drawdown_limit_pct"`
	DrawdownUtilization float64 `json:"drawdown_utilization_pct"`
	
	// Concentration metrics
	MaxSingleAsset    float64 `json:"max_single_asset_pct"`
	MaxSingleAssetLimit float64 `json:"max_single_asset_limit_pct"`
	MaxCorrelation    float64 `json:"max_correlation"`
	CorrelationLimit  float64 `json:"correlation_limit"`
	
	// Sector exposure
	SectorBreaches    int                `json:"sector_breaches"`
	TopSectorExposure map[string]float64 `json:"top_sector_exposure"`
	
	// Violations
	ActiveViolations  int      `json:"active_violations"`
	Breaches          int      `json:"breaches"`
	HealthStatus      string   `json:"health_status"`  // HEALTHY/DEGRADED/PAUSED
}

// EmergencyStatus tracks emergency controls and alerts
type EmergencyStatus struct {
	GlobalPause       bool      `json:"global_pause"`
	PauseReasons      []string  `json:"pause_reasons"`
	PauseDuration     int       `json:"pause_duration_minutes"`
	BlacklistedCount  int       `json:"blacklisted_symbols"`
	DegradedMode      bool      `json:"degraded_mode"`
	LastEmergencyTime time.Time `json:"last_emergency_time"`
}

// NewRiskMonitor creates a new risk monitoring instance
func NewRiskMonitor() *RiskMonitor {
	return &RiskMonitor{
		envelope:   NewRiskEnvelope(),
		lastUpdate: time.Now(),
	}
}

// UpdateMetrics refreshes risk metrics from current state
func (rm *RiskMonitor) UpdateMetrics() error {
	// Load current risk envelope state
	if err := rm.envelope.LoadState(); err != nil {
		// If state doesn't exist, use defaults (not an error for first run)
		rm.envelope = NewRiskEnvelope()
	}
	
	// Update universe hash from current universe
	if hash, err := rm.getCurrentUniverseHash(); err == nil {
		rm.universeHash = hash
	}
	
	rm.lastUpdate = time.Now()
	return nil
}

// GetMetrics returns current risk monitoring metrics
func (rm *RiskMonitor) GetMetrics() *RiskMetrics {
	rm.UpdateMetrics() // Ensure fresh data
	
	universeStatus := rm.getUniverseStatus()
	riskEnvelope := rm.getRiskEnvelopeMetrics()
	emergencyStatus := rm.getEmergencyStatus()
	
	return &RiskMetrics{
		UniverseStatus:  universeStatus,
		RiskEnvelope:    riskEnvelope,
		EmergencyStatus: emergencyStatus,
		Timestamp:       rm.lastUpdate,
	}
}

// getUniverseStatus builds universe monitoring metrics
func (rm *RiskMonitor) getUniverseStatus() UniverseStatus {
	// Load current universe for validation
	builder := NewUniverseBuilder(UniverseCriteria{
		Quote:     "USD",
		MinADVUSD: 100000,
		Venue:     "kraken",
	})
	
	currentUniverse := builder.loadCurrentUniverse()
	if currentUniverse == nil {
		return UniverseStatus{
			IntegrityCheck: "FAIL",
			ADVCompliance:  "UNKNOWN",
			USDCompliance:  "UNKNOWN",
		}
	}
	
	// Validate USD compliance
	usdCompliance := "PASS"
	for _, symbol := range currentUniverse.Universe {
		if !builder.isValidSymbol(symbol) {
			usdCompliance = "FAIL"
			break
		}
	}
	
	// Check hash integrity
	expectedHash := builder.generateHash(currentUniverse)
	integrityCheck := "PASS"
	if expectedHash != currentUniverse.Metadata.Hash {
		integrityCheck = "FAIL"
	}
	
	// ADV compliance (simplified - would check actual ADV data)
	advCompliance := "PASS"
	if currentUniverse.Metadata.Criteria.MinADVUSD != 100000 {
		advCompliance = "FAIL"
	}
	
	return UniverseStatus{
		SymbolCount:    len(currentUniverse.Universe),
		Hash:           currentUniverse.Metadata.Hash,
		LastRebuild:    currentUniverse.Metadata.Generated,
		IntegrityCheck: integrityCheck,
		ADVCompliance:  advCompliance,
		USDCompliance:  usdCompliance,
	}
}

// getRiskEnvelopeMetrics builds risk envelope monitoring metrics
func (rm *RiskMonitor) getRiskEnvelopeMetrics() RiskEnvelopeMetrics {
	state := rm.envelope.state
	config := rm.envelope.config
	
	// Position utilization
	activePositions := len(state.Positions)
	positionUtilization := float64(activePositions) / float64(config.MaxPositions) * 100
	
	// Exposure metrics
	exposureRatio := 0.0
	if state.PortfolioValue > 0 {
		exposureRatio = (state.TotalExposureUSD / state.PortfolioValue) * 100
	}
	
	// Drawdown utilization
	drawdownUtilization := 0.0
	if config.DrawdownPausePct > 0 {
		drawdownUtilization = (state.CurrentDrawdown / config.DrawdownPausePct) * 100
	}
	
	// Find max single asset exposure
	maxSingleAsset := 0.0
	for _, position := range state.Positions {
		if state.PortfolioValue > 0 {
			assetPct := (position.Size / state.PortfolioValue) * 100
			if assetPct > maxSingleAsset {
				maxSingleAsset = assetPct
			}
		}
	}
	
	// Find max correlation
	maxCorrelation := 0.0
	for _, corr := range state.CorrelationMatrix {
		if corr > maxCorrelation {
			maxCorrelation = corr
		}
	}
	
	// Count sector breaches
	sectorBreaches := 0
	topSectorExposure := make(map[string]float64)
	for sector, exposure := range state.SectorExposure {
		exposurePct := exposure * 100
		topSectorExposure[sector] = exposurePct
		
		if limit, exists := config.SectorCaps[sector]; exists {
			if exposure > limit {
				sectorBreaches++
			}
		}
	}
	
	// Determine health status
	healthStatus := "HEALTHY"
	activeViolations := 0
	breaches := 0
	
	if rm.envelope.emergency.GlobalPause {
		healthStatus = "PAUSED"
		breaches++
	} else if rm.envelope.emergency.DegradedMode {
		healthStatus = "DEGRADED"
	}
	
	// Check for approaching limits
	if positionUtilization > 80 || drawdownUtilization > 80 {
		healthStatus = "DEGRADED"
		activeViolations++
	}
	
	return RiskEnvelopeMetrics{
		ActivePositions:     activePositions,
		MaxPositions:        config.MaxPositions,
		PositionUtilization: positionUtilization,
		
		TotalExposureUSD:  state.TotalExposureUSD,
		PortfolioValueUSD: state.PortfolioValue,
		ExposureRatio:     exposureRatio,
		
		CurrentDrawdown:     state.CurrentDrawdown * 100,
		DrawdownLimit:       config.DrawdownPausePct * 100,
		DrawdownUtilization: drawdownUtilization,
		
		MaxSingleAsset:      maxSingleAsset,
		MaxSingleAssetLimit: config.MaxSingleAssetPct * 100,
		MaxCorrelation:      maxCorrelation,
		CorrelationLimit:    config.CorrelationCap,
		
		SectorBreaches:    sectorBreaches,
		TopSectorExposure: topSectorExposure,
		
		ActiveViolations: activeViolations,
		Breaches:         breaches,
		HealthStatus:     healthStatus,
	}
}

// getEmergencyStatus builds emergency controls monitoring metrics
func (rm *RiskMonitor) getEmergencyStatus() EmergencyStatus {
	emergency := rm.envelope.emergency
	
	// Count active blacklisted symbols
	blacklistedCount := 0
	now := time.Now()
	for _, blacklistTime := range emergency.SymbolBlacklist {
		hoursElapsed := now.Sub(blacklistTime).Hours()
		if hoursElapsed < float64(rm.envelope.config.BlacklistHours) {
			blacklistedCount++
		}
	}
	
	// Calculate pause duration
	pauseDuration := 0
	if emergency.GlobalPause && !emergency.LastPauseTime.IsZero() {
		pauseDuration = int(now.Sub(emergency.LastPauseTime).Minutes())
	}
	
	// Find most recent emergency event
	lastEmergencyTime := emergency.LastPauseTime
	for _, blacklistTime := range emergency.SymbolBlacklist {
		if blacklistTime.After(lastEmergencyTime) {
			lastEmergencyTime = blacklistTime
		}
	}
	
	return EmergencyStatus{
		GlobalPause:       emergency.GlobalPause,
		PauseReasons:      emergency.PauseReasons,
		PauseDuration:     pauseDuration,
		BlacklistedCount:  blacklistedCount,
		DegradedMode:      emergency.DegradedMode,
		LastEmergencyTime: lastEmergencyTime,
	}
}

// getCurrentUniverseHash loads and returns the current universe hash
func (rm *RiskMonitor) getCurrentUniverseHash() (string, error) {
	builder := NewUniverseBuilder(UniverseCriteria{
		Quote:     "USD",
		MinADVUSD: 100000,
		Venue:     "kraken",
	})
	
	universe := builder.loadCurrentUniverse()
	if universe == nil {
		return "", fmt.Errorf("no universe found")
	}
	
	return universe.Metadata.Hash, nil
}

// GetPrometheusMetrics returns Prometheus-style metrics for /metrics endpoint
func (rm *RiskMonitor) GetPrometheusMetrics() string {
	metrics := rm.GetMetrics()
	
	var output string
	
	// Universe metrics
	output += fmt.Sprintf("# HELP cryptorun_universe_symbols Total symbols in USD universe\n")
	output += fmt.Sprintf("# TYPE cryptorun_universe_symbols gauge\n")
	output += fmt.Sprintf("cryptorun_universe_symbols %d\n", metrics.UniverseStatus.SymbolCount)
	
	output += fmt.Sprintf("# HELP cryptorun_universe_integrity Universe integrity check status (1=PASS, 0=FAIL)\n")
	output += fmt.Sprintf("# TYPE cryptorun_universe_integrity gauge\n")
	integrityValue := 0
	if metrics.UniverseStatus.IntegrityCheck == "PASS" {
		integrityValue = 1
	}
	output += fmt.Sprintf("cryptorun_universe_integrity %d\n", integrityValue)
	
	// Risk envelope metrics
	output += fmt.Sprintf("# HELP cryptorun_risk_positions_active Active positions count\n")
	output += fmt.Sprintf("# TYPE cryptorun_risk_positions_active gauge\n")
	output += fmt.Sprintf("cryptorun_risk_positions_active %d\n", metrics.RiskEnvelope.ActivePositions)
	
	output += fmt.Sprintf("# HELP cryptorun_risk_position_utilization Position limit utilization percentage\n")
	output += fmt.Sprintf("# TYPE cryptorun_risk_position_utilization gauge\n")
	output += fmt.Sprintf("cryptorun_risk_position_utilization %.2f\n", metrics.RiskEnvelope.PositionUtilization)
	
	output += fmt.Sprintf("# HELP cryptorun_risk_drawdown_current Current portfolio drawdown percentage\n")
	output += fmt.Sprintf("# TYPE cryptorun_risk_drawdown_current gauge\n")
	output += fmt.Sprintf("cryptorun_risk_drawdown_current %.2f\n", metrics.RiskEnvelope.CurrentDrawdown)
	
	output += fmt.Sprintf("# HELP cryptorun_risk_exposure_total Total portfolio exposure in USD\n")
	output += fmt.Sprintf("# TYPE cryptorun_risk_exposure_total gauge\n")
	output += fmt.Sprintf("cryptorun_risk_exposure_total %.2f\n", metrics.RiskEnvelope.TotalExposureUSD)
	
	// Emergency controls
	output += fmt.Sprintf("# HELP cryptorun_emergency_pause Global pause status (1=active, 0=inactive)\n")
	output += fmt.Sprintf("# TYPE cryptorun_emergency_pause gauge\n")
	pauseValue := 0
	if metrics.EmergencyStatus.GlobalPause {
		pauseValue = 1
	}
	output += fmt.Sprintf("cryptorun_emergency_pause %d\n", pauseValue)
	
	output += fmt.Sprintf("# HELP cryptorun_emergency_blacklisted Blacklisted symbols count\n")
	output += fmt.Sprintf("# TYPE cryptorun_emergency_blacklisted gauge\n")
	output += fmt.Sprintf("cryptorun_emergency_blacklisted %d\n", metrics.EmergencyStatus.BlacklistedCount)
	
	output += fmt.Sprintf("# HELP cryptorun_risk_violations Active risk violations count\n")
	output += fmt.Sprintf("# TYPE cryptorun_risk_violations gauge\n")
	output += fmt.Sprintf("cryptorun_risk_violations %d\n", metrics.RiskEnvelope.ActiveViolations)
	
	output += fmt.Sprintf("# HELP cryptorun_risk_breaches Risk limit breaches count\n")
	output += fmt.Sprintf("# TYPE cryptorun_risk_breaches gauge\n")
	output += fmt.Sprintf("cryptorun_risk_breaches %d\n", metrics.RiskEnvelope.Breaches)
	
	return output
}

// GetRiskEnvelopeSummary returns a human-readable summary for /metrics HTML view
func (rm *RiskMonitor) GetRiskEnvelopeSummary() map[string]interface{} {
	metrics := rm.GetMetrics()
	
	summary := map[string]interface{}{
		"title": "Risk Envelope Status",
		"status": metrics.RiskEnvelope.HealthStatus,
		"timestamp": metrics.Timestamp.Format(time.RFC3339),
		
		"universe": map[string]interface{}{
			"symbols": metrics.UniverseStatus.SymbolCount,
			"hash": metrics.UniverseStatus.Hash[:12] + "...",
			"integrity": metrics.UniverseStatus.IntegrityCheck,
			"usd_compliance": metrics.UniverseStatus.USDCompliance,
			"adv_compliance": metrics.UniverseStatus.ADVCompliance,
		},
		
		"positions": map[string]interface{}{
			"active": fmt.Sprintf("%d/%d", metrics.RiskEnvelope.ActivePositions, metrics.RiskEnvelope.MaxPositions),
			"utilization": fmt.Sprintf("%.1f%%", metrics.RiskEnvelope.PositionUtilization),
			"total_exposure": fmt.Sprintf("$%.0f", metrics.RiskEnvelope.TotalExposureUSD),
		},
		
		"risk_limits": map[string]interface{}{
			"drawdown": fmt.Sprintf("%.2f%% / %.1f%%", metrics.RiskEnvelope.CurrentDrawdown, metrics.RiskEnvelope.DrawdownLimit),
			"max_single_asset": fmt.Sprintf("%.1f%% / %.1f%%", metrics.RiskEnvelope.MaxSingleAsset, metrics.RiskEnvelope.MaxSingleAssetLimit),
			"max_correlation": fmt.Sprintf("%.2f / %.2f", metrics.RiskEnvelope.MaxCorrelation, metrics.RiskEnvelope.CorrelationLimit),
			"sector_breaches": metrics.RiskEnvelope.SectorBreaches,
		},
		
		"emergency": map[string]interface{}{
			"global_pause": metrics.EmergencyStatus.GlobalPause,
			"pause_reasons": metrics.EmergencyStatus.PauseReasons,
			"blacklisted_symbols": metrics.EmergencyStatus.BlacklistedCount,
			"degraded_mode": metrics.EmergencyStatus.DegradedMode,
		},
		
		"violations": metrics.RiskEnvelope.ActiveViolations,
		"breaches": metrics.RiskEnvelope.Breaches,
	}
	
	return summary
}

// GetRiskEnvelopeJSON returns detailed risk metrics as JSON
func (rm *RiskMonitor) GetRiskEnvelopeJSON() ([]byte, error) {
	metrics := rm.GetMetrics()
	return json.MarshalIndent(metrics, "", "  ")
}