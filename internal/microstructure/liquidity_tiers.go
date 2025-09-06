package microstructure

import (
	"fmt"
	"sort"
)

// LiquidityTierManager manages tiered liquidity requirements by ADV
// Tiers: Tier1 ($5M+ ADV), Tier2 ($1-5M ADV), Tier3 ($100k-1M ADV)
type LiquidityTierManager struct {
	tiers []LiquidityTier
}

// NewLiquidityTierManager creates a tier manager with default configuration
func NewLiquidityTierManager() *LiquidityTierManager {
	return &LiquidityTierManager{
		tiers: getDefaultLiquidityTiers(),
	}
}

// NewLiquidityTierManagerWithConfig creates a tier manager with custom tiers
func NewLiquidityTierManagerWithConfig(tiers []LiquidityTier) *LiquidityTierManager {
	// Sort tiers by ADV minimum (descending)
	sortedTiers := make([]LiquidityTier, len(tiers))
	copy(sortedTiers, tiers)
	
	sort.Slice(sortedTiers, func(i, j int) bool {
		return sortedTiers[i].ADVMin > sortedTiers[j].ADVMin
	})
	
	return &LiquidityTierManager{
		tiers: sortedTiers,
	}
}

// getDefaultLiquidityTiers returns the standard 3-tier structure
func getDefaultLiquidityTiers() []LiquidityTier {
	return []LiquidityTier{
		{
			Name:         "tier1",
			ADVMin:       5000000,  // $5M+ ADV
			ADVMax:       1e12,     // No upper limit
			DepthMinUSD:  150000,   // $150k depth within ±2%
			SpreadCapBps: 25,       // 25 bps max spread
			VADRMinimum:  1.85,     // 1.85× minimum VADR
			Description:  "High liquidity: Large caps, stablecoins (BTC, ETH, USDT, etc.)",
		},
		{
			Name:         "tier2",
			ADVMin:       1000000,  // $1-5M ADV
			ADVMax:       5000000,
			DepthMinUSD:  75000,    // $75k depth within ±2%
			SpreadCapBps: 50,       // 50 bps max spread
			VADRMinimum:  1.80,     // 1.80× minimum VADR
			Description:  "Medium liquidity: Mid caps (ADA, SOL, MATIC, etc.)",
		},
		{
			Name:         "tier3",
			ADVMin:       100000,   // $100k-1M ADV
			ADVMax:       1000000,
			DepthMinUSD:  25000,    // $25k depth within ±2%
			SpreadCapBps: 80,       // 80 bps max spread
			VADRMinimum:  1.75,     // 1.75× minimum VADR
			Description:  "Lower liquidity: Small caps and newer tokens",
		},
	}
}

// GetTierByADV determines the appropriate tier based on Average Daily Volume
func (ltm *LiquidityTierManager) GetTierByADV(adv float64) (*LiquidityTier, error) {
	if adv < 0 {
		return nil, fmt.Errorf("invalid ADV: %.2f (must be non-negative)", adv)
	}
	
	// Find the highest tier that the ADV qualifies for
	for _, tier := range ltm.tiers {
		if adv >= tier.ADVMin && adv <= tier.ADVMax {
			// Return a copy to prevent modification
			tierCopy := tier
			return &tierCopy, nil
		}
	}
	
	// If ADV is below all tiers, return the lowest tier with a warning
	if len(ltm.tiers) > 0 {
		lowestTier := ltm.tiers[len(ltm.tiers)-1]
		tierCopy := lowestTier
		return &tierCopy, fmt.Errorf("ADV %.0f below minimum tier requirement %.0f, using %s", 
			adv, tierCopy.ADVMin, tierCopy.Name)
	}
	
	return nil, fmt.Errorf("no tiers configured")
}

// GetTierByName retrieves a tier by its name
func (ltm *LiquidityTierManager) GetTierByName(name string) (*LiquidityTier, error) {
	for _, tier := range ltm.tiers {
		if tier.Name == name {
			tierCopy := tier
			return &tierCopy, nil
		}
	}
	
	return nil, fmt.Errorf("tier not found: %s", name)
}

// GetAllTiers returns all configured tiers
func (ltm *LiquidityTierManager) GetAllTiers() []LiquidityTier {
	result := make([]LiquidityTier, len(ltm.tiers))
	copy(result, ltm.tiers)
	return result
}

// ValidateSymbolForTier checks if a symbol meets tier requirements
func (ltm *LiquidityTierManager) ValidateSymbolForTier(symbol string, adv float64, depth, spread, vadr float64) (*TierValidationResult, error) {
	tier, err := ltm.GetTierByADV(adv)
	if err != nil {
		return nil, fmt.Errorf("failed to determine tier: %w", err)
	}
	
	validation := &TierValidationResult{
		Symbol:      symbol,
		ADV:         adv,
		AssignedTier: tier.Name,
		TierRequirements: *tier,
		Measurements: TierMeasurements{
			DepthUSD:  depth,
			SpreadBps: spread,
			VADR:      vadr,
		},
	}
	
	// Check each requirement
	validation.DepthPass = depth >= tier.DepthMinUSD
	validation.SpreadPass = spread <= tier.SpreadCapBps
	validation.VADRPass = vadr >= tier.VADRMinimum
	
	// Overall pass requires all gates to pass
	validation.OverallPass = validation.DepthPass && validation.SpreadPass && validation.VADRPass
	
	// Generate failure reasons
	if !validation.DepthPass {
		validation.FailureReasons = append(validation.FailureReasons,
			fmt.Sprintf("depth $%.0f < $%.0f required", depth, tier.DepthMinUSD))
	}
	
	if !validation.SpreadPass {
		validation.FailureReasons = append(validation.FailureReasons,
			fmt.Sprintf("spread %.1f bps > %.1f bps cap", spread, tier.SpreadCapBps))
	}
	
	if !validation.VADRPass {
		validation.FailureReasons = append(validation.FailureReasons,
			fmt.Sprintf("VADR %.3f < %.3f minimum", vadr, tier.VADRMinimum))
	}
	
	return validation, nil
}

// TierValidationResult contains validation results against tier requirements
type TierValidationResult struct {
	Symbol           string          `json:"symbol"`
	ADV              float64         `json:"adv"`
	AssignedTier     string          `json:"assigned_tier"`
	TierRequirements LiquidityTier   `json:"tier_requirements"`
	Measurements     TierMeasurements `json:"measurements"`
	
	// Gate results
	DepthPass    bool `json:"depth_pass"`
	SpreadPass   bool `json:"spread_pass"`
	VADRPass     bool `json:"vadr_pass"`
	OverallPass  bool `json:"overall_pass"`
	
	// Details
	FailureReasons []string `json:"failure_reasons,omitempty"`
}

// TierMeasurements contains actual measurements for tier validation
type TierMeasurements struct {
	DepthUSD  float64 `json:"depth_usd"`
	SpreadBps float64 `json:"spread_bps"`
	VADR      float64 `json:"vadr"`
}

// GetTierSummary returns a human-readable tier summary
func (ltm *LiquidityTierManager) GetTierSummary() []string {
	summary := make([]string, len(ltm.tiers))
	
	for i, tier := range ltm.tiers {
		advRange := fmt.Sprintf("$%.0f+", tier.ADVMin)
		if tier.ADVMax < 1e12 {
			advRange = fmt.Sprintf("$%.0f-$%.0f", tier.ADVMin, tier.ADVMax)
		}
		
		summary[i] = fmt.Sprintf("%s: %s ADV, $%.0fk depth, %.0f bps spread, %.2f× VADR",
			tier.Name,
			advRange,
			tier.DepthMinUSD/1000,
			tier.SpreadCapBps,
			tier.VADRMinimum)
	}
	
	return summary
}

// GetTierForSymbolClass returns appropriate tier for common symbol classes
func (ltm *LiquidityTierManager) GetTierForSymbolClass(symbolClass string) (*LiquidityTier, error) {
	var targetTier string
	
	switch symbolClass {
	case "major", "stablecoin", "btc", "eth":
		targetTier = "tier1"
	case "altcoin", "defi", "layer1":
		targetTier = "tier2"
	case "smallcap", "new", "memecoin":
		targetTier = "tier3"
	default:
		return nil, fmt.Errorf("unknown symbol class: %s", symbolClass)
	}
	
	return ltm.GetTierByName(targetTier)
}

// EstimatePositionSize estimates maximum position size for a tier
func (ltm *LiquidityTierManager) EstimatePositionSize(tier *LiquidityTier, venueHealthy bool) *PositionSizeEstimate {
	if tier == nil {
		return &PositionSizeEstimate{
			Error: "no tier provided",
		}
	}
	
	// Base position size on depth requirement
	baseSize := tier.DepthMinUSD * 0.8 // Use 80% of minimum depth as base
	
	// Apply venue health adjustment
	adjustedSize := baseSize
	sizeAdjustment := "full_size"
	
	if !venueHealthy {
		adjustedSize = baseSize * 0.5 // Halve size for unhealthy venues
		sizeAdjustment = "halve_size"
	}
	
	return &PositionSizeEstimate{
		TierName:        tier.Name,
		BaseSize:        baseSize,
		AdjustedSize:    adjustedSize,
		VenueHealthy:    venueHealthy,
		SizeAdjustment:  sizeAdjustment,
		DepthUtilization: (adjustedSize / tier.DepthMinUSD) * 100,
		Reasoning:       fmt.Sprintf("Base %.0f (80%% of $%.0fk depth), %s due to venue health", 
			baseSize, tier.DepthMinUSD/1000, sizeAdjustment),
	}
}

// PositionSizeEstimate contains position sizing recommendations
type PositionSizeEstimate struct {
	TierName         string  `json:"tier_name"`
	BaseSize         float64 `json:"base_size"`         // Base position size (USD)
	AdjustedSize     float64 `json:"adjusted_size"`     // Adjusted for venue health
	VenueHealthy     bool    `json:"venue_healthy"`     // Venue health status
	SizeAdjustment   string  `json:"size_adjustment"`   // "full_size", "halve_size"
	DepthUtilization float64 `json:"depth_utilization"` // % of depth being used
	Reasoning        string  `json:"reasoning"`         // Human-readable reasoning
	Error            string  `json:"error,omitempty"`   // Error message if any
}

// CompareSymbolTiers compares two symbols across tiers
func (ltm *LiquidityTierManager) CompareSymbolTiers(symbol1 string, adv1 float64, symbol2 string, adv2 float64) (*TierComparison, error) {
	tier1, err1 := ltm.GetTierByADV(adv1)
	tier2, err2 := ltm.GetTierByADV(adv2)
	
	comparison := &TierComparison{
		Symbol1: symbol1,
		Symbol2: symbol2,
		ADV1:    adv1,
		ADV2:    adv2,
	}
	
	if err1 != nil {
		comparison.Error = fmt.Sprintf("tier1 error: %v", err1)
		return comparison, err1
	}
	
	if err2 != nil {
		comparison.Error = fmt.Sprintf("tier2 error: %v", err2)
		return comparison, err2
	}
	
	comparison.Tier1 = tier1.Name
	comparison.Tier2 = tier2.Name
	comparison.SameTier = tier1.Name == tier2.Name
	
	// Determine which has better liquidity
	if adv1 > adv2 {
		comparison.BetterLiquidity = symbol1
		comparison.LiquidityAdvantage = (adv1 - adv2) / adv2 * 100
	} else if adv2 > adv1 {
		comparison.BetterLiquidity = symbol2
		comparison.LiquidityAdvantage = (adv2 - adv1) / adv1 * 100
	} else {
		comparison.BetterLiquidity = "equal"
		comparison.LiquidityAdvantage = 0
	}
	
	return comparison, nil
}

// TierComparison contains comparison results between two symbols
type TierComparison struct {
	Symbol1            string  `json:"symbol1"`
	Symbol2            string  `json:"symbol2"`
	ADV1               float64 `json:"adv1"`
	ADV2               float64 `json:"adv2"`
	Tier1              string  `json:"tier1"`
	Tier2              string  `json:"tier2"`
	SameTier           bool    `json:"same_tier"`
	BetterLiquidity    string  `json:"better_liquidity"`    // Which symbol has better liquidity
	LiquidityAdvantage float64 `json:"liquidity_advantage"` // Percentage advantage
	Error              string  `json:"error,omitempty"`
}