package microstructure

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/internal/microstructure/adapters"
)

// VenuePolicy enforces venue-specific policies and health monitoring
type VenuePolicy struct {
	config         *Config
	venueHealth    map[string]*VenueHealthStatus
	healthMutex    sync.RWMutex
	aggregateGuard *adapters.RuntimeAggregatorGuard
	
	// Policy configuration
	enableDeFiHooks bool
	defiHooks       *DeFiHooks
}

// VenuePolicyResult contains venue policy evaluation results
type VenuePolicyResult struct {
	Venue           string               `json:"venue"`
	Approved        bool                 `json:"approved"`
	Health          *VenueHealthStatus   `json:"health"`
	PolicyViolations []string            `json:"policy_violations,omitempty"`
	DeFiMetrics     *DeFiMetrics         `json:"defi_metrics,omitempty"`
	Recommendation  string               `json:"recommendation"`
	LastChecked     time.Time            `json:"last_checked"`
}

// DeFiMetrics contains decentralized finance liquidity metrics (optional)
type DeFiMetrics struct {
	OnChainLiquidity    float64   `json:"on_chain_liquidity"`    // Total on-chain liquidity (USD)
	DEXVenues           []string  `json:"dex_venues"`            // Active DEX venues
	LiquidityUtilization float64  `json:"liquidity_utilization"` // % of total liquidity utilized
	CrossChainSupport   bool      `json:"cross_chain_support"`   // Multi-chain support available
	LastUpdate          time.Time `json:"last_update"`           // When metrics were last updated
}

// DeFiHooks provides optional DeFi liquidity integration
type DeFiHooks struct {
	enabled          bool
	supportedChains  []string
	liquidityPools   map[string]float64 // pool -> liquidity USD
	lastUpdate       time.Time
	fetchInterval    time.Duration
}

// NewVenuePolicy creates a new venue policy enforcer
func NewVenuePolicy(config *Config) *VenuePolicy {
	return &VenuePolicy{
		config:         config,
		venueHealth:    make(map[string]*VenueHealthStatus),
		aggregateGuard: adapters.NewRuntimeAggregatorGuard(true),
		enableDeFiHooks: false, // Disabled by default
		defiHooks:      NewDeFiHooks(),
	}
}

// ValidateVenue validates a venue against policy requirements
func (vp *VenuePolicy) ValidateVenue(ctx context.Context, venue, symbol string) (*VenuePolicyResult, error) {
	result := &VenuePolicyResult{
		Venue:       venue,
		Approved:    false,
		LastChecked: time.Now(),
	}

	// 1. Aggregator ban enforcement
	if err := vp.aggregateGuard.CheckSource(venue); err != nil {
		result.PolicyViolations = append(result.PolicyViolations, 
			fmt.Sprintf("aggregator ban violation: %v", err))
		result.Recommendation = "reject"
		return result, nil
	}

	// 2. Supported venue check
	if !vp.IsVenueSupported(venue) {
		result.PolicyViolations = append(result.PolicyViolations,
			fmt.Sprintf("venue '%s' not in supported list: %v", venue, vp.config.SupportedVenues))
		result.Recommendation = "reject"
		return result, nil
	}

	// 3. USD-only pairs enforcement (per CryptoRun v3.2.1 constraints)
	if !vp.isUSDPair(symbol) {
		result.PolicyViolations = append(result.PolicyViolations,
			fmt.Sprintf("symbol '%s' is not a USD pair", symbol))
		result.Recommendation = "reject"
		return result, nil
	}

	// 4. Venue health check
	health, err := vp.GetVenueHealth(venue)
	if err != nil {
		// Initialize health if not found
		health = vp.initializeVenueHealth(venue)
	}
	result.Health = health

	// 5. DeFi hooks (if enabled)
	if vp.enableDeFiHooks {
		defiMetrics, err := vp.defiHooks.GetLiquidityMetrics(ctx, symbol)
		if err == nil {
			result.DeFiMetrics = defiMetrics
		}
	}

	// 6. Overall approval decision
	result.Approved = len(result.PolicyViolations) == 0 && health.Healthy
	result.Recommendation = vp.determineRecommendation(result)

	return result, nil
}

// IsVenueSupported checks if venue is in the supported list
func (vp *VenuePolicy) IsVenueSupported(venue string) bool {
	venueLower := strings.ToLower(venue)
	for _, supported := range vp.config.SupportedVenues {
		if strings.ToLower(supported) == venueLower {
			return true
		}
	}
	return false
}

// isUSDPair validates that the symbol is a USD pair
func (vp *VenuePolicy) isUSDPair(symbol string) bool {
	symbolUpper := strings.ToUpper(symbol)
	return strings.HasSuffix(symbolUpper, "USD") || 
		   strings.HasSuffix(symbolUpper, "USDT") ||
		   strings.HasSuffix(symbolUpper, "USDC")
}

// GetVenueHealth returns current venue health status
func (vp *VenuePolicy) GetVenueHealth(venue string) (*VenueHealthStatus, error) {
	vp.healthMutex.RLock()
	defer vp.healthMutex.RUnlock()

	health, exists := vp.venueHealth[venue]
	if !exists {
		return nil, fmt.Errorf("venue health not tracked for: %s", venue)
	}

	// Check if health data is stale
	if time.Since(health.LastUpdate) > 5*time.Minute {
		health.Healthy = false
		health.Recommendation = "avoid"
	}

	return health, nil
}

// UpdateVenueHealth updates venue health metrics
func (vp *VenuePolicy) UpdateVenueHealth(venue string, health VenueHealthStatus) error {
	vp.healthMutex.Lock()
	defer vp.healthMutex.Unlock()

	// Validate venue first
	if !vp.IsVenueSupported(venue) {
		return fmt.Errorf("cannot update health for unsupported venue: %s", venue)
	}

	health.LastUpdate = time.Now()
	vp.venueHealth[venue] = &health

	return nil
}

// initializeVenueHealth creates default health status for a new venue
func (vp *VenuePolicy) initializeVenueHealth(venue string) *VenueHealthStatus {
	health := &VenueHealthStatus{
		Healthy:        true,
		RejectRate:     0.0,
		LatencyP99Ms:   1000,
		ErrorRate:      0.0,
		LastUpdate:     time.Now(),
		Recommendation: "full_size",
		UptimePercent:  99.9,
	}

	vp.healthMutex.Lock()
	defer vp.healthMutex.Unlock()
	vp.venueHealth[venue] = health

	return health
}

// determineRecommendation determines action recommendation based on policy result
func (vp *VenuePolicy) determineRecommendation(result *VenuePolicyResult) string {
	if !result.Approved {
		return "reject"
	}

	if result.Health == nil {
		return "defer"
	}

	// Follow venue health recommendation
	return result.Health.Recommendation
}

// GetSupportedVenues returns list of supported venues
func (vp *VenuePolicy) GetSupportedVenues() []string {
	return vp.config.SupportedVenues
}

// GetPolicyStatus returns current policy enforcement status
func (vp *VenuePolicy) GetPolicyStatus() map[string]interface{} {
	vp.healthMutex.RLock()
	defer vp.healthMutex.RUnlock()

	healthSummary := make(map[string]string)
	for venue, health := range vp.venueHealth {
		if health.Healthy {
			healthSummary[venue] = "healthy"
		} else {
			healthSummary[venue] = "degraded"
		}
	}

	return map[string]interface{}{
		"supported_venues":     vp.config.SupportedVenues,
		"venue_health":         healthSummary,
		"aggregator_violations": len(vp.aggregateGuard.GetViolations()),
		"defi_hooks_enabled":   vp.enableDeFiHooks,
		"last_health_check":    time.Now(),
	}
}

// EnableDeFiHooks enables DeFi liquidity hooks (experimental)
func (vp *VenuePolicy) EnableDeFiHooks() {
	vp.enableDeFiHooks = true
	if vp.defiHooks != nil {
		vp.defiHooks.enabled = true
	}
}

// DisableDeFiHooks disables DeFi liquidity hooks
func (vp *VenuePolicy) DisableDeFiHooks() {
	vp.enableDeFiHooks = false
	if vp.defiHooks != nil {
		vp.defiHooks.enabled = false
	}
}

// DeFi Hooks Implementation (Optional/Experimental)

// NewDeFiHooks creates a new DeFi hooks manager
func NewDeFiHooks() *DeFiHooks {
	return &DeFiHooks{
		enabled:         false,
		supportedChains: []string{"ethereum", "polygon", "arbitrum"},
		liquidityPools:  make(map[string]float64),
		fetchInterval:   5 * time.Minute,
		lastUpdate:      time.Now(),
	}
}

// GetLiquidityMetrics fetches on-chain liquidity metrics (stub implementation)
func (dh *DeFiHooks) GetLiquidityMetrics(ctx context.Context, symbol string) (*DeFiMetrics, error) {
	if !dh.enabled {
		return nil, fmt.Errorf("DeFi hooks disabled")
	}

	// TODO: Implement actual on-chain data fetching
	// For now, return stub data
	metrics := &DeFiMetrics{
		OnChainLiquidity:    1000000.0, // $1M stub
		DEXVenues:          []string{"uniswap_v3", "curve", "balancer"},
		LiquidityUtilization: 0.15,    // 15% utilization
		CrossChainSupport:   true,
		LastUpdate:         time.Now(),
	}

	return metrics, nil
}

// RefreshLiquidityPools refreshes on-chain liquidity pool data (stub)
func (dh *DeFiHooks) RefreshLiquidityPools(ctx context.Context) error {
	if !dh.enabled {
		return fmt.Errorf("DeFi hooks disabled")
	}

	// TODO: Implement actual on-chain pool data fetching
	// For now, populate with stub data
	dh.liquidityPools = map[string]float64{
		"uniswap_v3_btc_usdc": 5000000.0,
		"curve_btc_wbtc":      2000000.0,
		"balancer_wbtc_eth":   1500000.0,
	}
	dh.lastUpdate = time.Now()

	return nil
}

// GetTotalOnChainLiquidity returns total tracked on-chain liquidity
func (dh *DeFiHooks) GetTotalOnChainLiquidity() float64 {
	if !dh.enabled {
		return 0.0
	}

	total := 0.0
	for _, liquidity := range dh.liquidityPools {
		total += liquidity
	}
	return total
}

// IsStaleData checks if DeFi data is stale and needs refresh
func (dh *DeFiHooks) IsStaleData() bool {
	return time.Since(dh.lastUpdate) > dh.fetchInterval
}

// VenueRiskProfile defines risk characteristics for each venue
type VenueRiskProfile struct {
	Venue              string    `json:"venue"`
	RiskScore          float64   `json:"risk_score"`          // 0.0-1.0 (higher = riskier)
	MaxPositionSize    float64   `json:"max_position_size"`   // Maximum position size (USD)
	RequiredCollateral float64   `json:"required_collateral"` // Collateral requirement multiplier
	KnownIssues        []string  `json:"known_issues,omitempty"`
	LastRiskAssessment time.Time `json:"last_risk_assessment"`
}

// GetVenueRiskProfile returns risk profile for a venue
func (vp *VenuePolicy) GetVenueRiskProfile(venue string) (*VenueRiskProfile, error) {
	// Default risk profiles for supported venues
	profiles := map[string]VenueRiskProfile{
		"binance": {
			Venue:           "binance",
			RiskScore:       0.2,
			MaxPositionSize: 10000000.0, // $10M
			RequiredCollateral: 1.1,
		},
		"okx": {
			Venue:           "okx", 
			RiskScore:       0.3,
			MaxPositionSize: 5000000.0, // $5M
			RequiredCollateral: 1.2,
		},
		"coinbase": {
			Venue:           "coinbase",
			RiskScore:       0.1,
			MaxPositionSize: 15000000.0, // $15M
			RequiredCollateral: 1.05,
		},
		"kraken": {
			Venue:           "kraken",
			RiskScore:       0.15,
			MaxPositionSize: 8000000.0, // $8M
			RequiredCollateral: 1.1,
		},
	}

	profile, exists := profiles[strings.ToLower(venue)]
	if !exists {
		return nil, fmt.Errorf("no risk profile available for venue: %s", venue)
	}

	profile.LastRiskAssessment = time.Now()
	return &profile, nil
}