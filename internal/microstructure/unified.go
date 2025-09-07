package microstructure

import (
	"context"
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/microstructure/adapters"
)

// UnifiedMicrostructureEvaluator provides unified spread, depth, and VADR calculations
// with venue policy enforcement and attribution tracking
type UnifiedMicrostructureEvaluator struct {
	spreadCalc     *SpreadCalculator
	depthCalc      *DepthCalculator
	vadrCalc       *VADRCalculator
	venuePolicy    *VenuePolicy
	config         *Config
	AggregateGuard *adapters.RuntimeAggregatorGuard // Exported for testing
}

// NewUnifiedMicrostructureEvaluator creates a new unified evaluator
func NewUnifiedMicrostructureEvaluator(config *Config) *UnifiedMicrostructureEvaluator {
	if config == nil {
		config = DefaultConfig()
	}

	return &UnifiedMicrostructureEvaluator{
		spreadCalc:     NewSpreadCalculator(config.SpreadWindowSeconds),
		depthCalc:      NewDepthCalculator(config.DepthWindowSeconds),
		vadrCalc:       NewVADRCalculator(),
		venuePolicy:    NewVenuePolicy(config),
		config:         config,
		AggregateGuard: adapters.NewRuntimeAggregatorGuard(true), // Strict mode
	}
}

// UnifiedResult contains unified microstructure evaluation results
type UnifiedResult struct {
	// Core metrics
	Spread *SpreadResult `json:"spread"`
	Depth  *DepthResult  `json:"depth"`
	VADR   *VADRResult   `json:"vadr"`

	// Venue and policy
	Venue       string             `json:"venue"`
	VenuePolicy *VenuePolicyResult `json:"venue_policy"`

	// Gate evaluation
	GateResults map[string]*GateResult `json:"gate_results"`
	
	// Overall assessment
	AllGatesPassed   bool     `json:"all_gates_passed"`
	FailureReasons   []string `json:"failure_reasons,omitempty"`
	RecommendedAction string  `json:"recommended_action"`

	// Attribution and metadata
	Attribution *AttributionData `json:"attribution"`
	Timestamp   time.Time        `json:"timestamp"`
	ProcessingMs int64           `json:"processing_ms"`
}

// AttributionData tracks data sources and processing metadata
type AttributionData struct {
	OrderBookSource   string        `json:"orderbook_source"`   // e.g., "binance"
	VolumeSource      string        `json:"volume_source"`      // e.g., "binance"
	DataAge           time.Duration `json:"data_age"`           // Age of underlying data
	QualityScore      float64       `json:"quality_score"`      // 0.0-1.0 data quality
	CacheHit          bool          `json:"cache_hit"`          // Data from cache vs live
	RateLimited       bool          `json:"rate_limited"`       // Rate limiting applied
	CircuitBreakerOpen bool         `json:"circuit_breaker_open"` // Circuit breaker status
	ProcessingPath    string        `json:"processing_path"`    // "unified_evaluator"
}

// GateResult contains individual gate evaluation result
type GateResult struct {
	Name        string  `json:"name"`        // "spread", "depth", "vadr"
	Passed      bool    `json:"passed"`      // Gate passed
	Value       float64 `json:"value"`       // Measured value
	Threshold   float64 `json:"threshold"`   // Required threshold
	Description string  `json:"description"` // Human-readable description
	Reason      string  `json:"reason"`      // Pass/fail reason
}

// EvaluateUnified performs comprehensive microstructure evaluation
func (u *UnifiedMicrostructureEvaluator) EvaluateUnified(
	ctx context.Context,
	symbol, venue string,
	orderbook *OrderBookSnapshot,
	vadrInput *VADRInput,
	tier *LiquidityTier,
) (*UnifiedResult, error) {
	
	startTime := time.Now()

	// Venue policy validation
	venueResult, err := u.venuePolicy.ValidateVenue(ctx, venue, symbol)
	if err != nil {
		return nil, fmt.Errorf("venue policy validation failed: %w", err)
	}

	// Aggregator ban enforcement
	if err := u.AggregateGuard.CheckSource(venue); err != nil {
		return nil, fmt.Errorf("aggregator ban violation: %w", err)
	}

	result := &UnifiedResult{
		Venue:        venue,
		VenuePolicy:  venueResult,
		GateResults:  make(map[string]*GateResult),
		Timestamp:    startTime,
		Attribution: &AttributionData{
			OrderBookSource: venue,
			VolumeSource:   venue,
			ProcessingPath: "unified_evaluator",
		},
	}

	// Calculate spread
	spreadResult, err := u.Spread(orderbook)
	if err != nil {
		result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("spread calculation failed: %v", err))
	} else {
		result.Spread = spreadResult
		
		// Evaluate spread gate
		spreadPassed, spreadReason := u.spreadCalc.ValidateSpreadRequirement(spreadResult, tier)
		result.GateResults["spread"] = &GateResult{
			Name:        "spread",
			Passed:      spreadPassed,
			Value:       spreadResult.RollingAvgBps,
			Threshold:   tier.SpreadCapBps,
			Description: u.spreadCalc.GetSpreadSummary(spreadResult),
			Reason:      spreadReason,
		}
	}

	// Calculate depth
	depthResult, err := u.Depth(orderbook)
	if err != nil {
		result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("depth calculation failed: %v", err))
	} else {
		result.Depth = depthResult
		
		// Evaluate depth gate
		depthPassed, depthReason := u.depthCalc.ValidateDepthRequirement(depthResult, tier)
		result.GateResults["depth"] = &GateResult{
			Name:        "depth",
			Passed:      depthPassed,
			Value:       depthResult.TotalDepthUSD,
			Threshold:   tier.DepthMinUSD,
			Description: u.depthCalc.GetDepthSummary(depthResult),
			Reason:      depthReason,
		}
	}

	// Calculate VADR
	vadrResult, err := u.VADR(vadrInput, tier)
	if err != nil {
		result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("VADR calculation failed: %v", err))
	} else {
		result.VADR = vadrResult
		
		// Evaluate VADR gate
		vadrPassed, vadrReason := u.vadrCalc.ValidateVADRRequirement(vadrResult)
		result.GateResults["vadr"] = &GateResult{
			Name:        "vadr",
			Passed:      vadrPassed,
			Value:       vadrResult.Current,
			Threshold:   vadrResult.EffectiveMin,
			Description: u.vadrCalc.GetVADRSummary(vadrResult),
			Reason:      vadrReason,
		}
	}

	// Overall gate assessment
	result.AllGatesPassed = true
	for _, gateResult := range result.GateResults {
		if !gateResult.Passed {
			result.AllGatesPassed = false
		}
	}

	// Recommended action based on results
	result.RecommendedAction = u.determineRecommendedAction(result)

	// Processing metadata
	result.ProcessingMs = time.Since(startTime).Milliseconds()
	result.Attribution.DataAge = time.Since(orderbook.Timestamp)
	result.Attribution.QualityScore = u.calculateQualityScore(result)

	return result, nil
}

// Spread calculates unified spread metrics
func (u *UnifiedMicrostructureEvaluator) Spread(orderbook *OrderBookSnapshot) (*SpreadResult, error) {
	// Venue validation
	if err := u.AggregateGuard.CheckSource(orderbook.Venue); err != nil {
		return nil, err
	}

	return u.spreadCalc.CalculateSpread(orderbook)
}

// Depth calculates unified depth metrics
func (u *UnifiedMicrostructureEvaluator) Depth(orderbook *OrderBookSnapshot) (*DepthResult, error) {
	// Venue validation
	if err := u.AggregateGuard.CheckSource(orderbook.Venue); err != nil {
		return nil, err
	}

	return u.depthCalc.CalculateDepth(orderbook)
}

// VADR calculates unified VADR metrics
func (u *UnifiedMicrostructureEvaluator) VADR(input *VADRInput, tier *LiquidityTier) (*VADRResult, error) {
	return u.vadrCalc.CalculateVADR(input, tier)
}

// GetSupportedVenues returns list of supported venues
func (u *UnifiedMicrostructureEvaluator) GetSupportedVenues() []string {
	return u.config.SupportedVenues
}

// IsVenueSupported checks if venue is supported
func (u *UnifiedMicrostructureEvaluator) IsVenueSupported(venue string) bool {
	return u.venuePolicy.IsVenueSupported(venue)
}

// GetVenueHealth returns current venue health status
func (u *UnifiedMicrostructureEvaluator) GetVenueHealth(venue string) (*VenueHealthStatus, error) {
	return u.venuePolicy.GetVenueHealth(venue)
}

// UpdateVenueHealth updates venue health metrics
func (u *UnifiedMicrostructureEvaluator) UpdateVenueHealth(venue string, health VenueHealthStatus) error {
	return u.venuePolicy.UpdateVenueHealth(venue, health)
}

// GetLiquidityTier determines liquidity tier based on ADV
func (u *UnifiedMicrostructureEvaluator) GetLiquidityTier(adv float64) *LiquidityTier {
	for _, tier := range u.config.LiquidityTiers {
		if adv >= tier.ADVMin && adv < tier.ADVMax {
			return &tier
		}
	}
	// Default to lowest tier if no match
	if len(u.config.LiquidityTiers) > 0 {
		return &u.config.LiquidityTiers[len(u.config.LiquidityTiers)-1]
	}
	return nil
}

// determineRecommendedAction determines action based on gate results and venue health
func (u *UnifiedMicrostructureEvaluator) determineRecommendedAction(result *UnifiedResult) string {
	if !result.AllGatesPassed {
		return "reject" // Hard rejection if any gate fails
	}

	if result.VenuePolicy != nil && !result.VenuePolicy.Approved {
		return "reject" // Venue policy failure
	}

	// Check venue health
	if result.VenuePolicy != nil && result.VenuePolicy.Health != nil {
		switch result.VenuePolicy.Health.Recommendation {
		case "avoid":
			return "reject"
		case "halve_size":
			return "halve_size"
		case "full_size":
			return "proceed"
		}
	}

	// Check data quality
	if result.Attribution.QualityScore < 0.7 {
		return "defer" // Low quality data
	}

	return "proceed"
}

// calculateQualityScore computes overall data quality score
func (u *UnifiedMicrostructureEvaluator) calculateQualityScore(result *UnifiedResult) float64 {
	scores := []float64{}

	// Spread quality
	if result.Spread != nil {
		switch result.Spread.DataQuality {
		case "excellent":
			scores = append(scores, 1.0)
		case "good":
			scores = append(scores, 0.8)
		case "sparse":
			scores = append(scores, 0.5)
		default:
			scores = append(scores, 0.3)
		}
	}

	// Data age penalty
	ageSeconds := result.Attribution.DataAge.Seconds()
	if ageSeconds < 1.0 {
		scores = append(scores, 1.0)
	} else if ageSeconds < 5.0 {
		scores = append(scores, 0.9)
	} else if ageSeconds < 10.0 {
		scores = append(scores, 0.7)
	} else {
		scores = append(scores, 0.3)
	}

	// VADR history adequacy
	if result.VADR != nil {
		if result.VADR.HistoryCount >= 200 {
			scores = append(scores, 1.0)
		} else if result.VADR.HistoryCount >= 50 {
			scores = append(scores, 0.8)
		} else {
			scores = append(scores, 0.5)
		}
	}

	// Calculate average
	if len(scores) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, score := range scores {
		sum += score
	}

	return sum / float64(len(scores))
}

// GetDiagnostics returns diagnostic information about the unified evaluator
func (u *UnifiedMicrostructureEvaluator) GetDiagnostics() map[string]interface{} {
	return map[string]interface{}{
		"supported_venues":    u.config.SupportedVenues,
		"aggregator_guard":    u.AggregateGuard.GetViolations(),
		"vadr_history_stats":  u.vadrCalc.GetVADRHistoryStats(),
		"spread_window_sec":   u.config.SpreadWindowSeconds,
		"depth_window_sec":    u.config.DepthWindowSeconds,
		"max_data_age_sec":    u.config.MaxDataAgeSeconds,
		"liquidity_tiers":     len(u.config.LiquidityTiers),
	}
}

// EvaluateSnapshot implements the basic Evaluator interface for compatibility
func (u *UnifiedMicrostructureEvaluator) EvaluateSnapshot(symbol string) (EvaluationResult, error) {
	// This is a compatibility shim - the real functionality is in EvaluateUnified
	return EvaluationResult{
		SpreadBps:      0.0, // Would need order book data
		DepthUSD:       0.0, // Would need order book data  
		VADR:           0.0, // Would need volume data
		BarCount:       0,
		DailyVolumeUSD: 0.0,
		ADX:            0.0,
		Hurst:          0.0,
		BarsFromTrigger: 0,
		LateFillDelay:   0,
		Healthy:         false,
	}, fmt.Errorf("EvaluateSnapshot not implemented - use EvaluateUnified instead")
}