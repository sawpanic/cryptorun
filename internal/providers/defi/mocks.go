package defi

import (
	"context"
	"fmt"
	"math"
	"time"
)

// MockDeFiProvider provides deterministic fake data for testing and development
type MockDeFiProvider struct {
	name                string
	protocolMetrics     map[string]*DeFiMetrics
	topTokens           []DeFiMetrics
	healthy             bool
	latencyMS           float64
	errorRate           float64
	supportedProtocols  []string
	simulateErrors      bool
	errorMessage        string
	requestCount        int
	lastUpdate          time.Time
}

// NewMockDeFiProvider creates a new mock DeFi provider with realistic data
func NewMockDeFiProvider(name string) *MockDeFiProvider {
	mock := &MockDeFiProvider{
		name:               name,
		protocolMetrics:    make(map[string]*DeFiMetrics),
		healthy:            true,
		latencyMS:          50.0 + float64(len(name))*10, // Vary by provider name
		errorRate:          0.01, // 1% error rate
		supportedProtocols: []string{},
		simulateErrors:     false,
		lastUpdate:         time.Now(),
	}
	
	// Populate with realistic mock data
	mock.populateProtocolData()
	mock.populateTopTokens()
	
	return mock
}

// GetProtocolTVL returns mock TVL metrics for a protocol/token
func (m *MockDeFiProvider) GetProtocolTVL(ctx context.Context, protocol string, tokenSymbol string) (*DeFiMetrics, error) {
	m.requestCount++
	
	// Simulate rate limiting or errors
	if m.simulateErrors {
		return nil, fmt.Errorf("mock error: %s", m.errorMessage)
	}
	
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	// Simulate network latency
	time.Sleep(time.Duration(m.latencyMS/2) * time.Millisecond)
	
	// Return protocol-specific metrics
	if baseMetrics, ok := m.protocolMetrics[protocol]; ok {
		metrics := *baseMetrics // Clone
		metrics.Protocol = protocol
		metrics.TokenSymbol = tokenSymbol
		metrics.DataSource = m.name
		metrics.Timestamp = time.Now()
		
		// Apply PIT shift if configured
		if metrics.PITShift > 0 {
			metrics.Timestamp = metrics.Timestamp.Add(-time.Duration(metrics.PITShift) * time.Hour)
		}
		
		return &metrics, nil
	}
	
	// Return nil for unknown protocols (not an error)
	return nil, nil
}

// GetPoolMetrics returns mock AMM pool metrics
func (m *MockDeFiProvider) GetPoolMetrics(ctx context.Context, protocol string, tokenA, tokenB string) (*DeFiMetrics, error) {
	m.requestCount++
	
	// For pools, use the TVL metrics but enhance with pool-specific data
	metrics, err := m.GetProtocolTVL(ctx, protocol, tokenA)
	if err != nil || metrics == nil {
		return metrics, err
	}
	
	// Add pool-specific enhancements
	if metrics.TVL > 0 {
		// Estimate pool liquidity as 90% of TVL
		metrics.PoolLiquidity = metrics.TVL * 0.9
		
		// Estimate fees as 0.3% of volume
		if metrics.PoolVolume24h > 0 {
			metrics.PoolFees24h = metrics.PoolVolume24h * 0.003
		}
	}
	
	return metrics, nil
}

// GetLendingMetrics returns mock lending protocol metrics
func (m *MockDeFiProvider) GetLendingMetrics(ctx context.Context, protocol string, tokenSymbol string) (*DeFiMetrics, error) {
	m.requestCount++
	
	// Only return lending data for lending protocols
	lendingProtocols := map[string]bool{
		"aave-v2":     true,
		"aave-v3":     true,
		"compound-v2": true,
		"compound-v3": true,
		"maker":       true,
	}
	
	if !lendingProtocols[protocol] {
		return nil, nil // Not a lending protocol
	}
	
	metrics, err := m.GetProtocolTVL(ctx, protocol, tokenSymbol)
	if err != nil || metrics == nil {
		return metrics, err
	}
	
	// Ensure lending-specific fields are populated
	if metrics.SupplyAPY == 0 && metrics.BorrowAPY == 0 {
		// Set realistic APYs if not already set
		baseAPY := 3.0 + float64(len(protocol))*0.5 // Vary by protocol
		metrics.SupplyAPY = baseAPY
		metrics.BorrowAPY = baseAPY * 1.8 // Borrow typically ~80% higher
		metrics.UtilizationRate = 0.65 + float64(len(tokenSymbol))*0.05 // Vary by token
		if metrics.UtilizationRate > 1.0 {
			metrics.UtilizationRate = 0.95
		}
	}
	
	return metrics, nil
}

// GetTopTVLTokens returns mock top TVL tokens
func (m *MockDeFiProvider) GetTopTVLTokens(ctx context.Context, limit int) ([]DeFiMetrics, error) {
	m.requestCount++
	
	if m.simulateErrors {
		return nil, fmt.Errorf("mock error: %s", m.errorMessage)
	}
	
	// Simulate network latency
	time.Sleep(time.Duration(m.latencyMS) * time.Millisecond)
	
	result := make([]DeFiMetrics, 0, limit)
	for i, token := range m.topTokens {
		if i >= limit {
			break
		}
		
		token.DataSource = m.name
		token.Timestamp = time.Now()
		result = append(result, token)
	}
	
	return result, nil
}

// Health returns mock provider health status
func (m *MockDeFiProvider) Health(ctx context.Context) (*ProviderHealth, error) {
	m.requestCount++
	
	return &ProviderHealth{
		Healthy:            m.healthy,
		DataSource:         m.name,
		LastUpdate:         m.lastUpdate,
		LatencyMS:          m.latencyMS,
		ErrorRate:          m.errorRate,
		SupportedProtocols: len(m.protocolMetrics),
		DataFreshness: map[string]time.Duration{
			"protocols": time.Since(m.lastUpdate),
			"tokens":    time.Since(m.lastUpdate),
		},
		Errors: m.getHealthErrors(),
	}, nil
}

// GetSupportedProtocols returns mock supported protocols
func (m *MockDeFiProvider) GetSupportedProtocols(ctx context.Context) ([]string, error) {
	protocols := make([]string, 0, len(m.protocolMetrics))
	for protocol := range m.protocolMetrics {
		protocols = append(protocols, protocol)
	}
	return protocols, nil
}

// Mock control methods for testing

// SetHealthy controls the health status
func (m *MockDeFiProvider) SetHealthy(healthy bool) {
	m.healthy = healthy
	if !healthy {
		m.errorRate = 0.20 // 20% error rate when unhealthy
		m.latencyMS *= 3   // Higher latency when unhealthy
	}
}

// SetLatency sets the simulated network latency
func (m *MockDeFiProvider) SetLatency(latencyMS float64) {
	m.latencyMS = latencyMS
}

// SimulateError enables error simulation
func (m *MockDeFiProvider) SimulateError(message string) {
	m.simulateErrors = true
	m.errorMessage = message
}

// ClearError disables error simulation
func (m *MockDeFiProvider) ClearError() {
	m.simulateErrors = false
	m.errorMessage = ""
}

// GetRequestCount returns the number of requests made to this provider
func (m *MockDeFiProvider) GetRequestCount() int {
	return m.requestCount
}

// ResetRequestCount resets the request counter
func (m *MockDeFiProvider) ResetRequestCount() {
	m.requestCount = 0
}

// AddProtocol adds or updates protocol metrics
func (m *MockDeFiProvider) AddProtocol(protocol string, metrics *DeFiMetrics) {
	m.protocolMetrics[protocol] = metrics
	m.lastUpdate = time.Now()
}

// RemoveProtocol removes protocol metrics
func (m *MockDeFiProvider) RemoveProtocol(protocol string) {
	delete(m.protocolMetrics, protocol)
	m.lastUpdate = time.Now()
}

// Private helper methods

func (m *MockDeFiProvider) populateProtocolData() {
	now := time.Now()
	
	// Uniswap V3 - Large AMM with high volume
	m.protocolMetrics["uniswap-v3"] = &DeFiMetrics{
		Timestamp:        now,
		Protocol:         "uniswap-v3",
		TVL:              2500000000.0, // $2.5B TVL
		TVLChange24h:     2.5,
		TVLChange7d:      8.2,
		PoolVolume24h:    1500000000.0, // $1.5B daily volume
		PoolLiquidity:    2200000000.0,
		PoolFees24h:      4500000.0, // $4.5M daily fees
		ConfidenceScore:  0.95,
		PITShift:         0,
	}
	
	// Aave V3 - Leading lending protocol
	m.protocolMetrics["aave-v3"] = &DeFiMetrics{
		Timestamp:        now,
		Protocol:         "aave-v3",
		TVL:              6000000000.0, // $6B TVL
		TVLChange24h:     1.8,
		TVLChange7d:      -3.2,
		SupplyAPY:        4.2,
		BorrowAPY:        7.8,
		UtilizationRate:  0.72,
		ConfidenceScore:  0.92,
		PITShift:         0,
	}
	
	// Curve - Stablecoin AMM
	m.protocolMetrics["curve"] = &DeFiMetrics{
		Timestamp:        now,
		Protocol:         "curve",
		TVL:              3200000000.0, // $3.2B TVL
		TVLChange24h:     0.8,
		TVLChange7d:      -1.5,
		PoolVolume24h:    800000000.0, // Lower volume, higher efficiency
		PoolLiquidity:    3000000000.0,
		PoolFees24h:      800000.0,
		ConfidenceScore:  0.88,
		PITShift:         0,
	}
	
	// Compound V2 - Established lending
	m.protocolMetrics["compound-v2"] = &DeFiMetrics{
		Timestamp:        now,
		Protocol:         "compound-v2",
		TVL:              1800000000.0, // $1.8B TVL
		TVLChange24h:     -0.5,
		TVLChange7d:      -5.2,
		SupplyAPY:        3.8,
		BorrowAPY:        6.9,
		UtilizationRate:  0.68,
		ConfidenceScore:  0.90,
		PITShift:         0,
	}
	
	// SushiSwap - Secondary AMM
	m.protocolMetrics["sushiswap"] = &DeFiMetrics{
		Timestamp:        now,
		Protocol:         "sushiswap",
		TVL:              1200000000.0, // $1.2B TVL
		TVLChange24h:     3.2,
		TVLChange7d:      12.8,
		PoolVolume24h:    400000000.0,
		PoolLiquidity:    1100000000.0,
		PoolFees24h:      1200000.0,
		ConfidenceScore:  0.85,
		PITShift:         0,
	}
	
	// MakerDAO - DAI stablecoin issuer
	m.protocolMetrics["makerdao"] = &DeFiMetrics{
		Timestamp:        now,
		Protocol:         "makerdao",
		TVL:              8500000000.0, // $8.5B TVL (highest)
		TVLChange24h:     0.2,
		TVLChange7d:      -2.1,
		SupplyAPY:        5.5, // DSR rate
		BorrowAPY:        8.2, // Stability fee
		UtilizationRate:  0.85,
		ConfidenceScore:  0.98, // Highest confidence
		PITShift:         0,
	}
	
	// Balancer V2 - Weighted pools
	m.protocolMetrics["balancer-v2"] = &DeFiMetrics{
		Timestamp:        now,
		Protocol:         "balancer-v2",
		TVL:              900000000.0, // $900M TVL
		TVLChange24h:     1.5,
		TVLChange7d:      6.8,
		PoolVolume24h:    180000000.0,
		PoolLiquidity:    850000000.0,
		PoolFees24h:      540000.0,
		ConfidenceScore:  0.82,
		PITShift:         0,
	}
	
	// Yearn Finance - Yield aggregator
	m.protocolMetrics["yearn-finance"] = &DeFiMetrics{
		Timestamp:        now,
		Protocol:         "yearn-finance",
		TVL:              450000000.0, // $450M TVL
		TVLChange24h:     -1.2,
		TVLChange7d:      3.5,
		SupplyAPY:        6.8, // Yield farming returns
		ConfidenceScore:  0.80,
		PITShift:         0,
	}
}

func (m *MockDeFiProvider) populateTopTokens() {
	now := time.Now()
	
	// Generate top USD tokens by TVL
	usdTokens := []struct {
		symbol string
		tvl    float64
		volume float64
	}{
		{"USDT", 12000000000.0, 8000000000.0},
		{"USDC", 10500000000.0, 6500000000.0},
		{"DAI", 4200000000.0, 2800000000.0},
		{"BUSD", 3800000000.0, 2200000000.0},
		{"FRAX", 1200000000.0, 400000000.0},
		{"TUSD", 800000000.0, 180000000.0},
		{"USDP", 650000000.0, 120000000.0},
		{"GUSD", 320000000.0, 45000000.0},
	}
	
	for i, token := range usdTokens {
		// Add some variance based on provider name
		variance := 1.0 + (float64(len(m.name))-6.0)*0.05 // ±5% based on provider name length
		
		metrics := DeFiMetrics{
			Timestamp:       now,
			Protocol:        "aggregated", // Cross-protocol aggregation
			TokenSymbol:     token.symbol,
			TVL:             token.tvl * variance,
			TVLChange24h:    -2.0 + float64(i)*0.8, // Vary from -2% to +4%
			TVLChange7d:     -5.0 + float64(i)*1.5, // Vary from -5% to +7%
			PoolVolume24h:   token.volume * variance,
			TVLRank:         i + 1,
			ConfidenceScore: 0.90 - float64(i)*0.02, // Decrease slightly by rank
			PITShift:        0,
		}
		
		m.topTokens = append(m.topTokens, metrics)
	}
}

func (m *MockDeFiProvider) getHealthErrors() []string {
	if m.healthy {
		return []string{}
	}
	
	errors := []string{
		fmt.Sprintf("High latency: %.1fms", m.latencyMS),
		fmt.Sprintf("Error rate: %.1f%%", m.errorRate*100),
	}
	
	if m.simulateErrors {
		errors = append(errors, fmt.Sprintf("Simulated error: %s", m.errorMessage))
	}
	
	return errors
}

// MockDeFiProviderFactory creates mock DeFi providers for testing
type MockDeFiProviderFactory struct {
	providers map[string]*MockDeFiProvider
}

// NewMockDeFiProviderFactory creates a new mock factory
func NewMockDeFiProviderFactory() *MockDeFiProviderFactory {
	return &MockDeFiProviderFactory{
		providers: make(map[string]*MockDeFiProvider),
	}
}

// CreateTheGraphProvider creates a mock The Graph provider
func (f *MockDeFiProviderFactory) CreateTheGraphProvider(config DeFiProviderConfig) (DeFiProvider, error) {
	mock := NewMockDeFiProvider("thegraph")
	mock.SetLatency(80.0) // Slightly higher latency for The Graph
	f.providers["thegraph"] = mock
	return mock, nil
}

// CreateDeFiLlamaProvider creates a mock DeFiLlama provider
func (f *MockDeFiProviderFactory) CreateDeFiLlamaProvider(config DeFiProviderConfig) (DeFiProvider, error) {
	mock := NewMockDeFiProvider("defillama")
	mock.SetLatency(120.0) // Higher latency for DeFiLlama
	f.providers["defillama"] = mock
	return mock, nil
}

// GetAvailableProviders returns available mock providers
func (f *MockDeFiProviderFactory) GetAvailableProviders() []string {
	return []string{"thegraph", "defillama"}
}

// GetProvider returns a specific mock provider (for testing control)
func (f *MockDeFiProviderFactory) GetProvider(name string) *MockDeFiProvider {
	return f.providers[name]
}

// MockDeFiAggregator provides mock aggregated DeFi metrics
type MockDeFiAggregator struct {
	providers map[string]DeFiProvider
	consensus float64
}

// NewMockDeFiAggregator creates a mock aggregator
func NewMockDeFiAggregator(providers map[string]DeFiProvider) *MockDeFiAggregator {
	return &MockDeFiAggregator{
		providers: providers,
		consensus: 0.95, // High consensus by default
	}
}

// AggregateLatest combines mock metrics from multiple providers
func (a *MockDeFiAggregator) AggregateLatest(ctx context.Context, tokenSymbol string, venues []string) (*AggregatedDeFiMetrics, error) {
	venueMetrics := make(map[string]*DeFiMetrics)
	
	// Collect metrics from requested venues
	for _, venue := range venues {
		if provider, ok := a.providers[venue]; ok {
			// For aggregation, use a common protocol like uniswap-v3
			if metrics, err := provider.GetProtocolTVL(ctx, "uniswap-v3", tokenSymbol); err == nil && metrics != nil {
				venueMetrics[venue] = metrics
			}
		}
	}
	
	if len(venueMetrics) == 0 {
		return nil, fmt.Errorf("no metrics available for token %s", tokenSymbol)
	}
	
	// Calculate aggregated values
	totalTVL := 0.0
	totalVolume := 0.0
	weightedTVLChange := 0.0
	totalWeight := 0.0
	
	for _, metrics := range venueMetrics {
		totalTVL += metrics.TVL
		totalVolume += metrics.PoolVolume24h
		weight := metrics.TVL
		weightedTVLChange += metrics.TVLChange24h * weight
		totalWeight += weight
	}
	
	if totalWeight > 0 {
		weightedTVLChange /= totalWeight
	}
	
	return &AggregatedDeFiMetrics{
		TokenSymbol:          tokenSymbol,
		Timestamp:            time.Now(),
		VenueCount:           len(venueMetrics),
		VenueMetrics:         venueMetrics,
		TotalTVL:             totalTVL,
		WeightedTVLChange24h: weightedTVLChange,
		TotalVolume24h:       totalVolume,
		TVLConsensus:         a.consensus,
		DataQuality:          a.consensus * 0.95, // Slightly lower than consensus
		OutlierVenues:        []string{}, // No outliers in mock data
	}, nil
}

// GetCrossVenueTVL calculates mock total TVL across venues
func (a *MockDeFiAggregator) GetCrossVenueTVL(ctx context.Context, tokenSymbol string) (float64, error) {
	aggregated, err := a.AggregateLatest(ctx, tokenSymbol, []string{"thegraph", "defillama"})
	if err != nil {
		return 0.0, err
	}
	return aggregated.TotalTVL, nil
}

// ValidateConsistency performs mock consistency validation
func (a *MockDeFiAggregator) ValidateConsistency(ctx context.Context, metrics map[string]*DeFiMetrics) (*ConsistencyReport, error) {
	if len(metrics) < 2 {
		return &ConsistencyReport{
			Timestamp:         time.Now(),
			VenueCount:        len(metrics),
			TVLConsistency:    0.0,
			VolumeConsistency: 0.0,
			OverallConsistency: 0.0,
			InsufficientData:   true,
			Recommendations:   []string{"Need at least 2 data sources for consistency analysis"},
		}, nil
	}
	
	// Mock consistency analysis
	tvlValues := make([]float64, 0, len(metrics))
	for _, m := range metrics {
		tvlValues = append(tvlValues, m.TVL)
	}
	
	// Calculate coefficient of variation as consistency metric
	mean := 0.0
	for _, v := range tvlValues {
		mean += v
	}
	mean /= float64(len(tvlValues))
	
	variance := 0.0
	for _, v := range tvlValues {
		diff := v - mean
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(len(tvlValues)))
	
	cv := stdDev / mean
	consistency := math.Max(0.0, 1.0-cv*10) // Convert CV to 0-1 consistency score
	
	return &ConsistencyReport{
		Timestamp:          time.Now(),
		VenueCount:         len(metrics),
		TVLConsistency:     consistency,
		VolumeConsistency:  consistency * 0.9, // Slightly lower for volume
		OverallConsistency: consistency * 0.95,
		Outliers:           make(map[string]OutlierInfo),
		OutlierThreshold:   2.0,
		InsufficientData:   false,
		StaleDataDetected:  false,
		HighVarianceWarning: cv > 0.1, // Warn if CV > 10%
		Recommendations:    []string{},
	}, nil
}

// SetConsensus sets the mock consensus score (for testing)
func (a *MockDeFiAggregator) SetConsensus(consensus float64) {
	a.consensus = math.Max(0.0, math.Min(1.0, consensus))
}