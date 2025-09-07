package factors

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/sawpanic/cryptorun/internal/providers/defi"
)

// DeFiFactorInput contains the required data for DeFi factor calculations
type DeFiFactorInput struct {
	TokenSymbol    string    `json:"token_symbol"`    // USD token symbol (USDT, USDC, etc.)
	ProtocolList   []string  `json:"protocol_list"`   // Protocols to analyze
	TimestampStart int64     `json:"timestamp_start"` // Analysis start time
	TimestampEnd   int64     `json:"timestamp_end"`   // Analysis end time
}

// DeFiFactorResult contains the output of DeFi factor analysis
type DeFiFactorResult struct {
	// Core DeFi metrics
	DeFiScore          float64 `json:"defi_score"`           // Composite DeFi score (0-1)
	TVLMomentum        float64 `json:"tvl_momentum"`         // TVL change momentum score
	ProtocolDiversity  float64 `json:"protocol_diversity"`   // Protocol diversification score
	LiquidityDepth     float64 `json:"liquidity_depth"`      // Total liquidity depth score

	// TVL Analysis
	TotalTVL           float64 `json:"total_tvl"`            // Aggregated TVL across protocols
	TVLChange24h       float64 `json:"tvl_change_24h"`       // 24h TVL change percentage
	TVLChange7d        float64 `json:"tvl_change_7d"`        // 7d TVL change percentage
	TVLRank            int     `json:"tvl_rank"`             // TVL rank among analyzed tokens

	// Protocol Analysis  
	ProtocolCount      int     `json:"protocol_count"`       // Number of active protocols
	DominantProtocol   string  `json:"dominant_protocol"`    // Protocol with highest TVL
	ProtocolShare      float64 `json:"protocol_share"`       // Dominant protocol's TVL share
	
	// Volume & Activity
	TotalVolume24h     float64 `json:"total_volume_24h"`     // Aggregated 24h volume
	VolumeToTVLRatio   float64 `json:"volume_to_tvl_ratio"`  // Volume/TVL efficiency ratio
	ActivityScore      float64 `json:"activity_score"`       // Trading activity score

	// Yield Analysis (for lending protocols)
	WeightedSupplyAPY  float64 `json:"weighted_supply_apy"`  // TVL-weighted supply APY
	WeightedBorrowAPY  float64 `json:"weighted_borrow_apy"`  // TVL-weighted borrow APY
	YieldSpread        float64 `json:"yield_spread"`         // Borrow - Supply APY spread
	AvgUtilization     float64 `json:"avg_utilization"`      // Average utilization rate

	// Data Quality & Risk
	DataConsensus      float64 `json:"data_consensus"`       // Cross-provider consensus score
	QualityScore       float64 `json:"quality_score"`        // Data quality composite
	ConcentrationRisk  float64 `json:"concentration_risk"`   // Protocol concentration risk

	// Attribution
	ProviderCount      int       `json:"provider_count"`     // Number of data providers used
	LastUpdate         time.Time `json:"last_update"`        // Last data update time
	PITShift           int       `json:"pit_shift"`          // Point-in-time shift applied
}

// DeFiFactorConfig holds configuration for DeFi factor calculations
type DeFiFactorConfig struct {
	// Provider settings
	Providers          []string `yaml:"providers"`           // Data providers to use (thegraph, defillama)
	MaxProviders       int      `yaml:"max_providers"`       // Max providers per query
	TimeoutSeconds     int      `yaml:"timeout_seconds"`     // Request timeout
	
	// Analysis parameters  
	MinProtocols       int     `yaml:"min_protocols"`        // Minimum protocols required
	MaxProtocols       int     `yaml:"max_protocols"`        // Maximum protocols to analyze
	TVLThreshold       float64 `yaml:"tvl_threshold"`        // Minimum TVL for inclusion
	
	// Scoring weights
	WeightTVLMomentum  float64 `yaml:"weight_tvl_momentum"`  // TVL momentum weight
	WeightDiversity    float64 `yaml:"weight_diversity"`     // Diversity weight
	WeightActivity     float64 `yaml:"weight_activity"`      // Activity weight
	WeightYield        float64 `yaml:"weight_yield"`         // Yield weight
	
	// Risk parameters
	ConcentrationLimit float64 `yaml:"concentration_limit"`  // Max protocol concentration
	QualityThreshold   float64 `yaml:"quality_threshold"`    // Min quality score threshold
	
	// Point-in-time settings
	PITShiftHours      int     `yaml:"pit_shift_hours"`      // PIT shift in hours
}

// DefaultDeFiFactorConfig returns sensible defaults
func DefaultDeFiFactorConfig() DeFiFactorConfig {
	return DeFiFactorConfig{
		Providers:          []string{"thegraph", "defillama"},
		MaxProviders:       2,
		TimeoutSeconds:     30,
		MinProtocols:       2,
		MaxProtocols:       8,
		TVLThreshold:       100000.0, // $100K minimum TVL
		WeightTVLMomentum:  0.35,
		WeightDiversity:    0.25,
		WeightActivity:     0.25,
		WeightYield:        0.15,
		ConcentrationLimit: 0.80, // Max 80% in single protocol
		QualityThreshold:   0.70, // Min 70% quality score
		PITShiftHours:      0,    // No PIT shift by default
	}
}

// DeFiFactorCalculator computes DeFi factor scores
type DeFiFactorCalculator struct {
	config    DeFiFactorConfig
	providers map[string]defi.DeFiProvider
}

// NewDeFiFactorCalculator creates a new calculator
func NewDeFiFactorCalculator(config DeFiFactorConfig, providers map[string]defi.DeFiProvider) *DeFiFactorCalculator {
	return &DeFiFactorCalculator{
		config:    config,
		providers: providers,
	}
}

// Calculate computes DeFi factor score from input data
func (dfc *DeFiFactorCalculator) Calculate(ctx context.Context, input DeFiFactorInput) (*DeFiFactorResult, error) {
	// Enforce USD pairs only constraint
	if !isUSDTokenSymbol(input.TokenSymbol) {
		return nil, fmt.Errorf("non-USD token not allowed: %s - USD pairs only", input.TokenSymbol)
	}

	// Validate protocol list
	if len(input.ProtocolList) < dfc.config.MinProtocols {
		return nil, &ValidationError{
			Field:   "protocol_list",
			Message: "insufficient protocols for analysis",
			MinLen:  dfc.config.MinProtocols,
			ActLen:  len(input.ProtocolList),
		}
	}

	// Limit protocols to avoid excessive API calls
	protocols := input.ProtocolList
	if len(protocols) > dfc.config.MaxProtocols {
		protocols = protocols[:dfc.config.MaxProtocols]
	}

	// Collect metrics from all providers and protocols
	allMetrics, err := dfc.collectProtocolMetrics(ctx, input.TokenSymbol, protocols)
	if err != nil {
		return nil, fmt.Errorf("failed to collect protocol metrics: %w", err)
	}

	if len(allMetrics) == 0 {
		return &DeFiFactorResult{
			DeFiScore:     0.0,
			TVLMomentum:   0.0,
			QualityScore:  0.0,
			LastUpdate:    time.Now(),
			PITShift:      dfc.config.PITShiftHours,
		}, nil
	}

	// Calculate core DeFi metrics
	result := &DeFiFactorResult{
		LastUpdate: time.Now(),
		PITShift:   dfc.config.PITShiftHours,
	}

	// Aggregate TVL metrics
	dfc.calculateTVLMetrics(allMetrics, result)

	// Calculate protocol diversity metrics  
	dfc.calculateDiversityMetrics(allMetrics, result)

	// Calculate activity metrics
	dfc.calculateActivityMetrics(allMetrics, result)

	// Calculate yield metrics (for lending protocols)
	dfc.calculateYieldMetrics(allMetrics, result)

	// Calculate data quality and consensus
	dfc.calculateQualityMetrics(allMetrics, result)

	// Calculate final composite DeFi score
	result.DeFiScore = dfc.calculateCompositeScore(result)

	return result, nil
}

// collectProtocolMetrics gathers metrics from all configured providers
func (dfc *DeFiFactorCalculator) collectProtocolMetrics(ctx context.Context, tokenSymbol string, protocols []string) (map[string][]*defi.DeFiMetrics, error) {
	allMetrics := make(map[string][]*defi.DeFiMetrics)
	
	// Collect from each provider
	providerCount := 0
	for _, providerName := range dfc.config.Providers {
		if providerCount >= dfc.config.MaxProviders {
			break
		}
		
		provider, ok := dfc.providers[providerName]
		if !ok {
			continue
		}
		
		// Set timeout
		ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(dfc.config.TimeoutSeconds)*time.Second)
		defer cancel()
		
		// Collect metrics for each protocol
		for _, protocol := range protocols {
			// Try different metric types
			if tvlMetrics, err := provider.GetProtocolTVL(ctxTimeout, protocol, tokenSymbol); err == nil && tvlMetrics != nil {
				allMetrics[protocol] = append(allMetrics[protocol], tvlMetrics)
			}
			
			if lendingMetrics, err := provider.GetLendingMetrics(ctxTimeout, protocol, tokenSymbol); err == nil && lendingMetrics != nil {
				// Only add if different from TVL metrics (avoid duplicates)
				if len(allMetrics[protocol]) == 0 || allMetrics[protocol][0].BorrowAPY == 0 {
					allMetrics[protocol] = append(allMetrics[protocol], lendingMetrics)
				}
			}
		}
		
		providerCount++
	}
	
	return allMetrics, nil
}

// calculateTVLMetrics computes TVL-related metrics
func (dfc *DeFiFactorCalculator) calculateTVLMetrics(allMetrics map[string][]*defi.DeFiMetrics, result *DeFiFactorResult) {
	totalTVL := 0.0
	tvlChanges24h := make([]float64, 0)
	tvlChanges7d := make([]float64, 0)
	validProtocols := 0
	
	for protocol, metricsList := range allMetrics {
		if len(metricsList) == 0 {
			continue
		}
		
		// Use most recent metrics
		metrics := metricsList[len(metricsList)-1]
		
		// Skip protocols below TVL threshold
		if metrics.TVL < dfc.config.TVLThreshold {
			continue
		}
		
		totalTVL += metrics.TVL
		validProtocols++
		
		if metrics.TVLChange24h != 0 {
			tvlChanges24h = append(tvlChanges24h, metrics.TVLChange24h)
		}
		
		if metrics.TVLChange7d != 0 {
			tvlChanges7d = append(tvlChanges7d, metrics.TVLChange7d)
		}
		
		// Track dominant protocol
		if metrics.TVL > result.TotalTVL || result.DominantProtocol == "" {
			result.DominantProtocol = protocol
			result.ProtocolShare = metrics.TVL / totalTVL
		}
	}
	
	result.TotalTVL = totalTVL
	result.ProtocolCount = validProtocols
	
	// Calculate average TVL changes
	if len(tvlChanges24h) > 0 {
		sum := 0.0
		for _, change := range tvlChanges24h {
			sum += change
		}
		result.TVLChange24h = sum / float64(len(tvlChanges24h))
	}
	
	if len(tvlChanges7d) > 0 {
		sum := 0.0
		for _, change := range tvlChanges7d {
			sum += change
		}
		result.TVLChange7d = sum / float64(len(tvlChanges7d))
	}
	
	// Calculate TVL momentum score (0-1)
	momentum := 0.0
	if result.TVLChange24h > 0 {
		momentum += math.Min(result.TVLChange24h/20.0, 0.5) // Cap at 20% = 0.5 score
	}
	if result.TVLChange7d > 0 {
		momentum += math.Min(result.TVLChange7d/50.0, 0.5) // Cap at 50% = 0.5 score
	}
	result.TVLMomentum = math.Min(momentum, 1.0)
}

// calculateDiversityMetrics computes protocol diversity metrics
func (dfc *DeFiFactorCalculator) calculateDiversityMetrics(allMetrics map[string][]*defi.DeFiMetrics, result *DeFiFactorResult) {
	if result.ProtocolCount <= 1 {
		result.ProtocolDiversity = 0.0
		result.ConcentrationRisk = 1.0
		return
	}
	
	// Calculate protocol share distribution
	protocolTVLs := make([]float64, 0, result.ProtocolCount)
	totalTVL := result.TotalTVL
	
	for _, metricsList := range allMetrics {
		if len(metricsList) == 0 {
			continue
		}
		
		metrics := metricsList[len(metricsList)-1]
		if metrics.TVL >= dfc.config.TVLThreshold {
			protocolTVLs = append(protocolTVLs, metrics.TVL)
		}
	}
	
	// Calculate Herfindahl-Hirschman Index (concentration)
	hhi := 0.0
	for _, tvl := range protocolTVLs {
		share := tvl / totalTVL
		hhi += share * share
	}
	
	// Diversity score: 1 - normalized HHI
	maxHHI := 1.0 // Perfect concentration
	minHHI := 1.0 / float64(len(protocolTVLs)) // Perfect distribution
	normalizedHHI := (hhi - minHHI) / (maxHHI - minHHI)
	result.ProtocolDiversity = math.Max(0.0, 1.0-normalizedHHI)
	result.ConcentrationRisk = normalizedHHI
}

// calculateActivityMetrics computes trading activity metrics
func (dfc *DeFiFactorCalculator) calculateActivityMetrics(allMetrics map[string][]*defi.DeFiMetrics, result *DeFiFactorResult) {
	totalVolume := 0.0
	volumeCount := 0
	
	for _, metricsList := range allMetrics {
		if len(metricsList) == 0 {
			continue
		}
		
		metrics := metricsList[len(metricsList)-1]
		if metrics.PoolVolume24h > 0 {
			totalVolume += metrics.PoolVolume24h
			volumeCount++
		}
	}
	
	result.TotalVolume24h = totalVolume
	
	// Calculate Volume-to-TVL ratio
	if result.TotalTVL > 0 {
		result.VolumeToTVLRatio = totalVolume / result.TotalTVL
	}
	
	// Activity score based on volume efficiency
	// Higher volume/TVL ratios indicate more active protocols
	if result.VolumeToTVLRatio > 0 {
		// Normalize using log scale: ln(ratio + 1) / ln(2) caps at ~1.0 for 100% ratio
		result.ActivityScore = math.Min(math.Log(result.VolumeToTVLRatio+1)/math.Log(2), 1.0)
	}
}

// calculateYieldMetrics computes yield-related metrics for lending protocols
func (dfc *DeFiFactorCalculator) calculateYieldMetrics(allMetrics map[string][]*defi.DeFiMetrics, result *DeFiFactorResult) {
	totalSupplyAPY := 0.0
	totalBorrowAPY := 0.0
	totalUtilization := 0.0
	lendingProtocols := 0
	totalWeightedTVL := 0.0
	
	for _, metricsList := range allMetrics {
		if len(metricsList) == 0 {
			continue
		}
		
		metrics := metricsList[len(metricsList)-1]
		
		// Check if this is a lending protocol (has yield data)
		if metrics.SupplyAPY > 0 || metrics.BorrowAPY > 0 {
			weight := metrics.TVL // TVL-weighted average
			totalSupplyAPY += metrics.SupplyAPY * weight
			totalBorrowAPY += metrics.BorrowAPY * weight
			totalUtilization += metrics.UtilizationRate * weight
			totalWeightedTVL += weight
			lendingProtocols++
		}
	}
	
	if lendingProtocols > 0 && totalWeightedTVL > 0 {
		result.WeightedSupplyAPY = totalSupplyAPY / totalWeightedTVL
		result.WeightedBorrowAPY = totalBorrowAPY / totalWeightedTVL
		result.AvgUtilization = totalUtilization / totalWeightedTVL
		result.YieldSpread = result.WeightedBorrowAPY - result.WeightedSupplyAPY
	}
}

// calculateQualityMetrics computes data quality and consensus metrics
func (dfc *DeFiFactorCalculator) calculateQualityMetrics(allMetrics map[string][]*defi.DeFiMetrics, result *DeFiFactorResult) {
	totalConfidence := 0.0
	dataPoints := 0
	providerCounts := make(map[string]int)
	
	for _, metricsList := range allMetrics {
		for _, metrics := range metricsList {
			totalConfidence += metrics.ConfidenceScore
			dataPoints++
			providerCounts[metrics.DataSource]++
		}
	}
	
	result.ProviderCount = len(providerCounts)
	
	if dataPoints > 0 {
		result.QualityScore = totalConfidence / float64(dataPoints)
		
		// Consensus bonus for multiple providers
		if result.ProviderCount > 1 {
			consensusBonus := math.Min(float64(result.ProviderCount-1)*0.1, 0.2) // Up to 20% bonus
			result.DataConsensus = math.Min(result.QualityScore+consensusBonus, 1.0)
		} else {
			result.DataConsensus = result.QualityScore * 0.8 // Penalty for single provider
		}
	}
}

// calculateCompositeScore combines all metrics into final DeFi score
func (dfc *DeFiFactorCalculator) calculateCompositeScore(result *DeFiFactorResult) float64 {
	// Check minimum quality threshold
	if result.QualityScore < dfc.config.QualityThreshold {
		return 0.0 // Reject low-quality data
	}
	
	// Check concentration risk limit
	if result.ConcentrationRisk > dfc.config.ConcentrationLimit {
		return 0.0 // Reject overly concentrated positions
	}
	
	// Weighted composite score
	score := 0.0
	score += dfc.config.WeightTVLMomentum * result.TVLMomentum
	score += dfc.config.WeightDiversity * result.ProtocolDiversity
	score += dfc.config.WeightActivity * result.ActivityScore
	
	// Yield component (only for lending protocols)
	if result.WeightedSupplyAPY > 0 {
		yieldScore := math.Min(result.WeightedSupplyAPY/20.0, 1.0) // Normalize to 20% APY = 1.0
		score += dfc.config.WeightYield * yieldScore
	}
	
	// Apply quality multiplier
	score *= result.DataConsensus
	
	return math.Max(0.0, math.Min(1.0, score))
}

// Helper functions

// isUSDTokenSymbol validates USD token symbols
func isUSDTokenSymbol(symbol string) bool {
	usdTokens := []string{"USDT", "USDC", "BUSD", "DAI", "TUSD", "USDP", "FRAX", "GUSD"}
	for _, usd := range usdTokens {
		if symbol == usd {
			return true
		}
	}
	return false
}