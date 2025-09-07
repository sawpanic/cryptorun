package microstructure

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"cryptorun/internal/microstructure/adapters"
)

// TieredGateCalculator implements venue-native tiered microstructure gates
type TieredGateCalculator struct {
	adapters    map[string]adapters.MicrostructureAdapter
	tierManager *LiquidityTierManager
	config      *TieredGateConfig
}

// TieredGateConfig holds configuration for tiered gates
type TieredGateConfig struct {
	// Fallback preferences (venue priority order)
	VenuePriority     []string      `yaml:"venue_priority"`      // ["binance", "okx", "coinbase"]
	CrossVenueEnabled bool          `yaml:"cross_venue_enabled"` // Enable cross-venue validation
	MaxVenueAge       time.Duration `yaml:"max_venue_age"`       // 30s max data age per venue

	// Precedence rules
	UseWorstFeedVADR   bool    `yaml:"use_worst_feed_vadr"`  // Use highest VADR requirement
	SpreadToleranceBps float64 `yaml:"spread_tolerance_bps"` // 5 bps cross-venue spread tolerance
	DepthTolerancePct  float64 `yaml:"depth_tolerance_pct"`  // 10% cross-venue depth tolerance

	// Quality thresholds
	MinDataQualityScore float64 `yaml:"min_data_quality_score"` // 0.8 minimum quality score
	MaxLatencyMs        int64   `yaml:"max_latency_ms"`         // 2000ms max response latency
	RequiredVenues      int     `yaml:"required_venues"`        // 1 minimum healthy venues
}

// DefaultTieredGateConfig returns production-ready configuration
func DefaultTieredGateConfig() *TieredGateConfig {
	return &TieredGateConfig{
		VenuePriority:       []string{"binance", "okx", "coinbase"},
		CrossVenueEnabled:   false, // Start simple, single-venue validation
		MaxVenueAge:         30 * time.Second,
		UseWorstFeedVADR:    true, // Conservative precedence rule
		SpreadToleranceBps:  5.0,  // Allow 5bps variance
		DepthTolerancePct:   0.10, // Allow 10% depth variance
		MinDataQualityScore: 0.8,  // Require good quality data
		MaxLatencyMs:        2000, // 2s max latency
		RequiredVenues:      1,    // Single venue required
	}
}

// NewTieredGateCalculator creates a new tiered gate calculator
func NewTieredGateCalculator(config *TieredGateConfig) *TieredGateCalculator {
	if config == nil {
		config = DefaultTieredGateConfig()
	}

	// Initialize adapters
	adapterFactory := adapters.NewAdapterFactory()
	adaptersMap := make(map[string]adapters.MicrostructureAdapter)

	for _, venue := range config.VenuePriority {
		if adapter, err := adapterFactory.CreateAdapter(venue); err == nil {
			adaptersMap[venue] = adapter
		}
	}

	return &TieredGateCalculator{
		adapters:    adaptersMap,
		tierManager: NewLiquidityTierManagerWithConfig(DefaultConfig().LiquidityTiers),
		config:      config,
	}
}

// TieredGateResult contains comprehensive gate evaluation with venue attribution
type TieredGateResult struct {
	Symbol    string         `json:"symbol"`
	Timestamp time.Time      `json:"timestamp"`
	Tier      *LiquidityTier `json:"tier"`
	ADV       float64        `json:"adv"`

	// Overall assessment
	AllGatesPass      bool   `json:"all_gates_pass"`
	RecommendedAction string `json:"recommended_action"` // proceed/halve_size/defer

	// Individual gate results
	DepthGate  *TieredDepthResult  `json:"depth_gate"`
	SpreadGate *TieredSpreadResult `json:"spread_gate"`
	VADRGate   *TieredVADRResult   `json:"vadr_gate"`

	// Venue health and precedence
	VenueResults  map[string]*VenueGateResult `json:"venue_results"`
	PrimaryVenue  string                      `json:"primary_venue"`  // Venue used for final decision
	FallbacksUsed []string                    `json:"fallbacks_used"` // Venues used as fallbacks
	DegradedMode  bool                        `json:"degraded_mode"`  // Operating with reduced venues

	// Failure analysis
	CriticalFailures []string `json:"critical_failures"`
	Warnings         []string `json:"warnings"`
	ProcessingTimeMs int64    `json:"processing_time_ms"`
}

// VenueGateResult contains per-venue gate evaluation
type VenueGateResult struct {
	Venue     string        `json:"venue"`
	Available bool          `json:"available"` // Venue responded successfully
	Latency   time.Duration `json:"latency"`   // Response latency
	DataAge   time.Duration `json:"data_age"`  // Data staleness
	Quality   string        `json:"quality"`   // Data quality assessment

	L1Data *adapters.L1Data `json:"l1_data,omitempty"`
	L2Data *adapters.L2Data `json:"l2_data,omitempty"`
	Error  string           `json:"error,omitempty"` // Error message if failed
}

// TieredDepthResult contains depth gate evaluation with tiered requirements
type TieredDepthResult struct {
	Required        float64            `json:"required"`         // Tier requirement (USD)
	Measured        float64            `json:"measured"`         // Best available measurement
	Pass            bool               `json:"pass"`             // Gate result
	VenueBreakdown  map[string]float64 `json:"venue_breakdown"`  // Per-venue depth measurements
	BestVenue       string             `json:"best_venue"`       // Venue with best depth
	QualityAdjusted float64            `json:"quality_adjusted"` // Depth adjusted for data quality
}

// TieredSpreadResult contains spread gate evaluation with tiered caps
type TieredSpreadResult struct {
	CapBps           float64            `json:"cap_bps"`            // Tier cap (basis points)
	MeasuredBps      float64            `json:"measured_bps"`       // Best available measurement
	Pass             bool               `json:"pass"`               // Gate result
	VenueBreakdown   map[string]float64 `json:"venue_breakdown"`    // Per-venue spread measurements
	BestVenue        string             `json:"best_venue"`         // Venue with tightest spread
	CrossVenueSpread float64            `json:"cross_venue_spread"` // Max spread divergence between venues
}

// TieredVADRResult contains VADR gate evaluation with precedence rules
type TieredVADRResult struct {
	RequiredMin    float64 `json:"required_min"`    // Base tier minimum
	EffectiveMin   float64 `json:"effective_min"`   // Applied minimum (max of p80, tier)
	Measured       float64 `json:"measured"`        // Best available measurement
	Pass           bool    `json:"pass"`            // Gate result
	P80Historical  float64 `json:"p80_historical"`  // 80th percentile from history
	PrecedenceRule string  `json:"precedence_rule"` // Applied precedence rule
}

// EvaluateTieredGates performs comprehensive tiered gate evaluation
func (tgc *TieredGateCalculator) EvaluateTieredGates(ctx context.Context, symbol string, adv float64, vadrInput *VADRInput) (*TieredGateResult, error) {
	startTime := time.Now()

	// Determine liquidity tier based on ADV
	tier, err := tgc.tierManager.GetTierByADV(adv)
	if err != nil {
		return nil, fmt.Errorf("failed to determine tier for ADV %.0f: %w", adv, err)
	}

	result := &TieredGateResult{
		Symbol:           symbol,
		Timestamp:        startTime,
		Tier:             tier,
		ADV:              adv,
		VenueResults:     make(map[string]*VenueGateResult),
		CriticalFailures: []string{},
		Warnings:         []string{},
	}

	// Gather data from all available venues
	venueData, err := tgc.gatherVenueData(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to gather venue data: %w", err)
	}

	result.VenueResults = venueData

	// Validate we have sufficient venue coverage
	healthyVenues := tgc.countHealthyVenues(venueData)
	if healthyVenues < tgc.config.RequiredVenues {
		result.CriticalFailures = append(result.CriticalFailures,
			fmt.Sprintf("insufficient healthy venues: %d < %d required", healthyVenues, tgc.config.RequiredVenues))
	}

	// Determine primary venue and fallbacks
	result.PrimaryVenue, result.FallbacksUsed = tgc.selectPrimaryVenue(venueData)
	result.DegradedMode = healthyVenues == 1

	// Evaluate individual gates
	result.DepthGate = tgc.evaluateDepthGate(venueData, tier)
	result.SpreadGate = tgc.evaluateSpreadGate(venueData, tier)

	if vadrInput != nil {
		result.VADRGate = tgc.evaluateVADRGate(vadrInput, tier)
	} else {
		result.Warnings = append(result.Warnings, "VADR input not provided")
	}

	// Determine overall result
	result.AllGatesPass = len(result.CriticalFailures) == 0 &&
		(result.DepthGate == nil || result.DepthGate.Pass) &&
		(result.SpreadGate == nil || result.SpreadGate.Pass) &&
		(result.VADRGate == nil || result.VADRGate.Pass)

	// Determine recommended action
	result.RecommendedAction = tgc.determineRecommendedAction(result)

	result.ProcessingTimeMs = time.Since(startTime).Milliseconds()

	return result, nil
}

// gatherVenueData collects L1/L2 data from all available venues
func (tgc *TieredGateCalculator) gatherVenueData(ctx context.Context, symbol string) (map[string]*VenueGateResult, error) {
	results := make(map[string]*VenueGateResult)

	for _, venue := range tgc.config.VenuePriority {
		adapter, exists := tgc.adapters[venue]
		if !exists {
			continue
		}

		venueResult := &VenueGateResult{
			Venue:     venue,
			Available: false,
		}

		// Measure latency
		startTime := time.Now()

		// Get L1 data
		l1Data, err := adapter.GetL1Data(ctx, symbol)
		if err != nil {
			venueResult.Error = fmt.Sprintf("L1 data failed: %v", err)
			results[venue] = venueResult
			continue
		}

		// Get L2 data
		l2Data, err := adapter.GetL2Data(ctx, symbol)
		if err != nil {
			venueResult.Error = fmt.Sprintf("L2 data failed: %v", err)
			results[venue] = venueResult
			continue
		}

		venueResult.Latency = time.Since(startTime)
		venueResult.Available = true
		venueResult.L1Data = l1Data
		venueResult.L2Data = l2Data
		venueResult.DataAge = l1Data.DataAge
		venueResult.Quality = l1Data.Quality

		// Validate latency
		if venueResult.Latency > time.Duration(tgc.config.MaxLatencyMs)*time.Millisecond {
			venueResult.Error = fmt.Sprintf("latency too high: %v", venueResult.Latency)
			venueResult.Available = false
		}

		// Validate data age
		if venueResult.DataAge > tgc.config.MaxVenueAge {
			venueResult.Error = fmt.Sprintf("data too stale: %v", venueResult.DataAge)
			venueResult.Available = false
		}

		results[venue] = venueResult
	}

	return results, nil
}

// evaluateDepthGate evaluates depth requirements across venues
func (tgc *TieredGateCalculator) evaluateDepthGate(venueData map[string]*VenueGateResult, tier *LiquidityTier) *TieredDepthResult {
	result := &TieredDepthResult{
		Required:       tier.DepthMinUSD,
		VenueBreakdown: make(map[string]float64),
	}

	bestDepth := 0.0
	bestVenue := ""

	// Collect depth measurements from all venues
	for venue, data := range venueData {
		if !data.Available || data.L2Data == nil {
			continue
		}

		depth := data.L2Data.TotalDepthUSD
		result.VenueBreakdown[venue] = depth

		// Apply quality adjustment
		qualityMultiplier := 1.0
		switch data.Quality {
		case "excellent":
			qualityMultiplier = 1.0
		case "good":
			qualityMultiplier = 0.95
		case "degraded":
			qualityMultiplier = 0.85
		}

		adjustedDepth := depth * qualityMultiplier

		if adjustedDepth > bestDepth {
			bestDepth = adjustedDepth
			bestVenue = venue
		}
	}

	result.Measured = bestDepth
	result.BestVenue = bestVenue
	result.QualityAdjusted = bestDepth
	result.Pass = bestDepth >= tier.DepthMinUSD

	return result
}

// evaluateSpreadGate evaluates spread requirements across venues
func (tgc *TieredGateCalculator) evaluateSpreadGate(venueData map[string]*VenueGateResult, tier *LiquidityTier) *TieredSpreadResult {
	result := &TieredSpreadResult{
		CapBps:         tier.SpreadCapBps,
		VenueBreakdown: make(map[string]float64),
	}

	bestSpread := math.Inf(1) // Start with infinity
	bestVenue := ""
	spreads := []float64{}

	// Collect spread measurements from all venues
	for venue, data := range venueData {
		if !data.Available || data.L1Data == nil {
			continue
		}

		spread := data.L1Data.SpreadBps
		result.VenueBreakdown[venue] = spread
		spreads = append(spreads, spread)

		if spread < bestSpread {
			bestSpread = spread
			bestVenue = venue
		}
	}

	result.MeasuredBps = bestSpread
	result.BestVenue = bestVenue
	result.Pass = bestSpread <= tier.SpreadCapBps

	// Calculate cross-venue spread divergence
	if len(spreads) > 1 {
		sort.Float64s(spreads)
		result.CrossVenueSpread = spreads[len(spreads)-1] - spreads[0]

		// Warn if divergence is high
		if result.CrossVenueSpread > tgc.config.SpreadToleranceBps {
			// This would be added to warnings in the calling function
		}
	}

	return result
}

// evaluateVADRGate evaluates VADR requirements with precedence rules
func (tgc *TieredGateCalculator) evaluateVADRGate(vadrInput *VADRInput, tier *LiquidityTier) *TieredVADRResult {
	result := &TieredVADRResult{
		RequiredMin: tier.VADRMinimum,
	}

	// Calculate current VADR
	if vadrInput.Volume > 0 && vadrInput.ADV > 0 {
		dailyRange := vadrInput.High - vadrInput.Low
		if dailyRange > 0 {
			result.Measured = (vadrInput.Volume * dailyRange) / vadrInput.ADV
		}
	}

	// Apply precedence rule: max(tier_min, p80_historical)
	// For now, mock p80 historical - in production this would come from historical data
	result.P80Historical = tier.VADRMinimum * 1.05 // Mock 5% higher than tier

	if tgc.config.UseWorstFeedVADR {
		result.EffectiveMin = math.Max(result.RequiredMin, result.P80Historical)
		result.PrecedenceRule = "max(tier_min, p80_historical)"
	} else {
		result.EffectiveMin = result.RequiredMin
		result.PrecedenceRule = "tier_min_only"
	}

	result.Pass = result.Measured >= result.EffectiveMin

	return result
}

// countHealthyVenues counts the number of available venues
func (tgc *TieredGateCalculator) countHealthyVenues(venueData map[string]*VenueGateResult) int {
	count := 0
	for _, data := range venueData {
		if data.Available {
			count++
		}
	}
	return count
}

// selectPrimaryVenue determines the primary venue and fallbacks used
func (tgc *TieredGateCalculator) selectPrimaryVenue(venueData map[string]*VenueGateResult) (string, []string) {
	// Select primary venue based on priority and availability
	for _, venue := range tgc.config.VenuePriority {
		if data, exists := venueData[venue]; exists && data.Available {
			// Determine fallbacks (other healthy venues)
			var fallbacks []string
			for _, otherVenue := range tgc.config.VenuePriority {
				if otherVenue != venue {
					if otherData, exists := venueData[otherVenue]; exists && otherData.Available {
						fallbacks = append(fallbacks, otherVenue)
					}
				}
			}

			return venue, fallbacks
		}
	}

	return "", []string{} // No healthy venues
}

// determineRecommendedAction determines the recommended trading action
func (tgc *TieredGateCalculator) determineRecommendedAction(result *TieredGateResult) string {
	// Critical failures = defer
	if len(result.CriticalFailures) > 0 {
		return "defer"
	}

	// Any gate failures = defer
	if !result.AllGatesPass {
		return "defer"
	}

	// Degraded mode = halve position size
	if result.DegradedMode {
		return "halve_size"
	}

	// All clear = proceed
	return "proceed"
}

// GetTieredGatesSummary returns a human-readable summary
func (tgc *TieredGateCalculator) GetTieredGatesSummary(result *TieredGateResult) string {
	if result == nil {
		return "No tiered gates data"
	}

	status := "PASS"
	if !result.AllGatesPass {
		status = "FAIL"
	}

	healthyVenues := tgc.countHealthyVenues(result.VenueResults)

	summary := fmt.Sprintf("Tiered Gates: %s [%s] %d venues",
		status, result.Tier.Name, healthyVenues)

	if result.DegradedMode {
		summary += " [DEGRADED]"
	}

	summary += fmt.Sprintf(" (%dms)", result.ProcessingTimeMs)

	return summary
}
