package microstructure

import (
	"context"
	"fmt"
	"time"
)

// MicrostructureEvaluator is the main implementation of the Evaluator interface
type MicrostructureEvaluator struct {
	config         *Config
	depthCalc      *DepthCalculator
	spreadCalc     *SpreadCalculator
	vadrCalc       *VADRCalculator
	tierManager    *LiquidityTierManager
	healthMonitor  *VenueHealthMonitor
}

// NewMicrostructureEvaluator creates a new microstructure evaluator
func NewMicrostructureEvaluator(config *Config) *MicrostructureEvaluator {
	if config == nil {
		config = DefaultConfig()
	}

	return &MicrostructureEvaluator{
		config:        config,
		depthCalc:     NewDepthCalculator(config.DepthWindowSeconds),
		spreadCalc:    NewSpreadCalculator(config.SpreadWindowSeconds),
		vadrCalc:      NewVADRCalculator(),
		tierManager:   NewLiquidityTierManagerWithConfig(config.LiquidityTiers),
		healthMonitor: NewVenueHealthMonitor(&VenueHealthConfig{
			RejectRateThreshold: config.RejectRateThreshold,
			LatencyThresholdMs:  config.LatencyThresholdMs,
			ErrorRateThreshold:  config.ErrorRateThreshold,
			WindowDuration:      15 * time.Minute,
			MinSamplesForHealth: 10,
			MaxHistorySize:      1000,
		}),
	}
}

// EvaluateGates performs comprehensive gate evaluation for a symbol/venue
func (me *MicrostructureEvaluator) EvaluateGates(ctx context.Context, symbol, venue string, orderbook *OrderBookSnapshot, adv float64) (*GateReport, error) {
	startTime := time.Now()
	
	// Validate inputs
	if err := me.validateInputs(symbol, venue, orderbook, adv); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Determine liquidity tier
	tier, err := me.tierManager.GetTierByADV(adv)
	if err != nil {
		return nil, fmt.Errorf("failed to determine tier for ADV %.0f: %w", adv, err)
	}

	// Initialize gate report
	report := &GateReport{
		Symbol:    symbol,
		Venue:     venue,
		Timestamp: orderbook.Timestamp,
		Details: GateDetails{
			LiquidityTier: tier.Name,
			ADV:           adv,
		},
	}

	// Evaluate depth gate
	depthResult, err := me.depthCalc.CalculateDepth(orderbook)
	if err != nil {
		return nil, fmt.Errorf("depth calculation failed: %w", err)
	}

	report.DepthOK, _ = me.depthCalc.ValidateDepthRequirement(depthResult, tier)
	report.Details.BidDepthUSD = depthResult.BidDepthUSD
	report.Details.AskDepthUSD = depthResult.AskDepthUSD
	report.Details.TotalDepthUSD = depthResult.TotalDepthUSD
	report.Details.DepthRequiredUSD = tier.DepthMinUSD

	// Evaluate spread gate
	spreadResult, err := me.spreadCalc.CalculateSpread(orderbook)
	if err != nil {
		return nil, fmt.Errorf("spread calculation failed: %w", err)
	}

	report.SpreadOK, _ = me.spreadCalc.ValidateSpreadRequirement(spreadResult, tier)
	report.Details.SpreadBps = spreadResult.Current.SpreadBps
	report.Details.SpreadCapBps = tier.SpreadCapBps

	// Evaluate VADR gate (requires additional input)
	vadrInput := &VADRInput{
		High:         orderbook.LastPrice * 1.02, // Approximate 24h high
		Low:          orderbook.LastPrice * 0.98, // Approximate 24h low
		Volume:       adv / orderbook.LastPrice,  // Approximate volume
		ADV:          adv,
		CurrentPrice: orderbook.LastPrice,
	}

	vadrResult, err := me.vadrCalc.CalculateVADR(vadrInput, tier)
	if err != nil {
		// VADR calculation can fail with insufficient data - mark as failed
		report.VadrOK = false
		report.Details.VADRCurrent = 0.0
		report.Details.VADRMinimum = tier.VADRMinimum
	} else {
		report.VadrOK = vadrResult.PassesGate
		report.Details.VADRCurrent = vadrResult.Current
		report.Details.VADRMinimum = vadrResult.EffectiveMin
	}

	// Evaluate venue health
	venueHealth, err := me.healthMonitor.GetVenueHealth(venue)
	if err != nil {
		return nil, fmt.Errorf("venue health check failed: %w", err)
	}

	report.Details.VenueHealth = *venueHealth

	// Determine overall execution feasibility
	report.ExecutionFeasible = report.DepthOK && report.SpreadOK && report.VadrOK

	// Generate failure reasons
	if !report.DepthOK {
		report.FailureReasons = append(report.FailureReasons,
			fmt.Sprintf("insufficient depth: $%.0f < $%.0f required (%s)",
				depthResult.TotalDepthUSD, tier.DepthMinUSD, tier.Name))
	}

	if !report.SpreadOK {
		report.FailureReasons = append(report.FailureReasons,
			fmt.Sprintf("spread too wide: %.1f bps > %.1f bps cap (%s)",
				spreadResult.Current.SpreadBps, tier.SpreadCapBps, tier.Name))
	}

	if !report.VadrOK {
		report.FailureReasons = append(report.FailureReasons,
			fmt.Sprintf("VADR insufficient: %.3f < %.3f required (%s)",
				report.Details.VADRCurrent, report.Details.VADRMinimum, tier.Name))
	}

	// Determine recommended action
	if report.ExecutionFeasible && venueHealth.Healthy {
		report.RecommendedAction = "proceed"
	} else if report.ExecutionFeasible && venueHealth.Recommendation == "halve_size" {
		report.RecommendedAction = "halve_size"
	} else {
		report.RecommendedAction = "defer"
	}

	// Add venue health issues to failure reasons
	if !venueHealth.Healthy {
		report.FailureReasons = append(report.FailureReasons,
			fmt.Sprintf("venue %s unhealthy: %s", venue, venueHealth.Recommendation))
	}

	// Set processing metadata
	report.Details.DataAge = time.Since(orderbook.Timestamp)
	report.Details.ProcessingMs = time.Since(startTime).Milliseconds()

	// Assess data quality
	report.Details.DataQuality = me.assessDataQuality(orderbook, depthResult, spreadResult)

	return report, nil
}

// GetLiquidityTier determines tier based on ADV
func (me *MicrostructureEvaluator) GetLiquidityTier(adv float64) *LiquidityTier {
	tier, _ := me.tierManager.GetTierByADV(adv)
	return tier
}

// UpdateVenueHealth updates venue health metrics
func (me *MicrostructureEvaluator) UpdateVenueHealth(venue string, health VenueHealthStatus) error {
	// This would typically update internal venue health tracking
	// For now, we'll record synthetic requests to simulate the health status
	if !health.Healthy {
		me.healthMonitor.RecordRequest(venue, "synthetic", health.LatencyP99Ms, false, 500, "simulated_error")
	} else {
		me.healthMonitor.RecordRequest(venue, "synthetic", health.LatencyP99Ms, true, 200, "")
	}
	return nil
}

// GetVenueHealth retrieves current venue health status
func (me *MicrostructureEvaluator) GetVenueHealth(venue string) (*VenueHealthStatus, error) {
	return me.healthMonitor.GetVenueHealth(venue)
}

// validateInputs performs input validation
func (me *MicrostructureEvaluator) validateInputs(symbol, venue string, orderbook *OrderBookSnapshot, adv float64) error {
	if symbol == "" {
		return fmt.Errorf("symbol is required")
	}

	if venue == "" {
		return fmt.Errorf("venue is required")
	}

	if orderbook == nil {
		return fmt.Errorf("orderbook snapshot is required")
	}

	if adv <= 0 {
		return fmt.Errorf("ADV must be positive, got %.2f", adv)
	}

	// Check if venue is supported
	supported := false
	for _, supportedVenue := range me.config.SupportedVenues {
		if venue == supportedVenue {
			supported = true
			break
		}
	}
	if !supported {
		return fmt.Errorf("venue %s not supported, must be one of: %v", venue, me.config.SupportedVenues)
	}

	// Check data freshness
	dataAge := time.Since(orderbook.Timestamp)
	if dataAge > time.Duration(me.config.MaxDataAgeSeconds)*time.Second {
		return fmt.Errorf("orderbook data too stale: %v old (max %ds)",
			dataAge, me.config.MaxDataAgeSeconds)
	}

	// Check minimum book levels
	if len(orderbook.Bids) < me.config.MinBookLevels || len(orderbook.Asks) < me.config.MinBookLevels {
		return fmt.Errorf("insufficient order book levels: %d bids, %d asks (min %d each)",
			len(orderbook.Bids), len(orderbook.Asks), me.config.MinBookLevels)
	}

	return nil
}

// assessDataQuality evaluates overall data quality
func (me *MicrostructureEvaluator) assessDataQuality(orderbook *OrderBookSnapshot, depth *DepthResult, spread *SpreadResult) string {
	score := 0
	maxScore := 5

	// Age quality
	if time.Since(orderbook.Timestamp) < 2*time.Second {
		score++
	}

	// Book quality
	if orderbook.Metadata.BookQuality == "full" {
		score++
	} else if orderbook.Metadata.BookQuality == "partial" {
		score += 0 // neutral
	}

	// Depth levels
	totalLevels := depth.BidLevels + depth.AskLevels
	if totalLevels >= 20 {
		score++
	} else if totalLevels >= 10 {
		score += 0 // neutral
	}

	// Spread stability
	if spread.SampleCount > 10 && spread.StdDevBps < 5.0 {
		score++
	}

	// Overall balance
	if depth.BidDepthUSD > 0 && depth.AskDepthUSD > 0 {
		balance := depth.BidDepthUSD / (depth.BidDepthUSD + depth.AskDepthUSD)
		if balance > 0.3 && balance < 0.7 { // Reasonably balanced
			score++
		}
	}

	switch {
	case score >= 4:
		return "excellent"
	case score >= 3:
		return "good"
	default:
		return "degraded"
	}
}

// GetEvaluatorStats returns evaluator performance statistics
func (me *MicrostructureEvaluator) GetEvaluatorStats() map[string]interface{} {
	allVenueHealth, _ := me.healthMonitor.GetAllVenueHealth()
	
	healthyVenues := 0
	for _, health := range allVenueHealth {
		if health.Healthy {
			healthyVenues++
		}
	}

	return map[string]interface{}{
		"config": map[string]interface{}{
			"supported_venues": me.config.SupportedVenues,
			"spread_window_s":  me.config.SpreadWindowSeconds,
			"depth_window_s":   me.config.DepthWindowSeconds,
			"max_data_age_s":   me.config.MaxDataAgeSeconds,
		},
		"venues": map[string]interface{}{
			"total_monitored": len(allVenueHealth),
			"healthy_count":   healthyVenues,
			"health_rate":     float64(healthyVenues) / float64(len(allVenueHealth)),
		},
		"tiers": map[string]interface{}{
			"tier_count": len(me.config.LiquidityTiers),
			"tier_names": me.getTierNames(),
		},
		"vadr_history": me.vadrCalc.GetVADRHistoryStats(),
	}
}

// getTierNames returns list of configured tier names
func (me *MicrostructureEvaluator) getTierNames() []string {
	names := make([]string, len(me.config.LiquidityTiers))
	for i, tier := range me.config.LiquidityTiers {
		names[i] = tier.Name
	}
	return names
}

// RecordVenueRequest records an API request for venue health tracking
func (me *MicrostructureEvaluator) RecordVenueRequest(venue, endpoint string, latencyMs int64, success bool, statusCode int, errorCode string) {
	me.healthMonitor.RecordRequest(venue, endpoint, latencyMs, success, statusCode, errorCode)
}

// ClearHistory clears all historical data (useful for testing)
func (me *MicrostructureEvaluator) ClearHistory() {
	me.spreadCalc.ClearHistory()
	me.vadrCalc.ClearHistory()
	me.healthMonitor.ClearHistory()
}