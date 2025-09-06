package derivs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// ETFProvider collects ETF flow data for crypto assets
type ETFProvider struct {
	httpClient *http.Client
	cache      map[string]*CachedETF
}

// NewETFProvider creates an ETF flow data provider
func NewETFProvider() *ETFProvider {
	return &ETFProvider{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cache:      make(map[string]*CachedETF),
	}
}

// ETFFlowSnapshot contains ETF flow data and tint calculation
type ETFFlowSnapshot struct {
	Symbol      string             `json:"symbol"`
	Timestamp   time.Time          `json:"timestamp"`
	ETFFlows    map[string]float64 `json:"etf_flows"`    // etf_ticker -> flow_usd_24h
	TotalFlow   float64            `json:"total_flow"`   // Net ETF flows (inflow positive)
	FlowTint    float64            `json:"flow_tint"`    // Normalized tint: -1 (outflow) to +1 (inflow)
	FlowVolume  float64            `json:"flow_volume"`  // Total flow volume (abs values)
	ETFList     []string           `json:"etf_list"`     // List of ETFs tracked
	DataSources map[string]string  `json:"data_sources"` // etf -> data_source
	CacheHit    bool               `json:"cache_hit"`
}

// CachedETF wraps ETF data with cache metadata
type CachedETF struct {
	Snapshot *ETFFlowSnapshot `json:"snapshot"`
	CachedAt time.Time        `json:"cached_at"`
	TTL      time.Duration    `json:"ttl"`
}

// GetETFFlowSnapshot retrieves ETF flow data and computes tint
func (ep *ETFProvider) GetETFFlowSnapshot(ctx context.Context, symbol string) (*ETFFlowSnapshot, error) {
	cacheKey := fmt.Sprintf("etf_%s", symbol)

	// Check cache first (ETF flows updated daily)
	if cached, exists := ep.cache[cacheKey]; exists {
		if time.Since(cached.CachedAt) < cached.TTL {
			cached.Snapshot.CacheHit = true
			return cached.Snapshot, nil
		}
		// Expired - remove from cache
		delete(ep.cache, cacheKey)
	}

	// Fetch fresh ETF data
	snapshot, err := ep.fetchETFSnapshot(ctx, symbol)
	if err != nil {
		return nil, err
	}

	// Cache the result (ETF data is relatively stable, updated daily)
	ep.cache[cacheKey] = &CachedETF{
		Snapshot: snapshot,
		CachedAt: time.Now(),
		TTL:      30 * time.Minute, // 30-minute cache for ETF data
	}

	snapshot.CacheHit = false
	return snapshot, nil
}

// fetchETFSnapshot collects ETF flow data from available sources
func (ep *ETFProvider) fetchETFSnapshot(ctx context.Context, symbol string) (*ETFFlowSnapshot, error) {
	snapshot := &ETFFlowSnapshot{
		Symbol:      symbol,
		Timestamp:   time.Now(),
		ETFFlows:    make(map[string]float64),
		DataSources: make(map[string]string),
		ETFList:     []string{},
	}

	// Get relevant ETFs for the symbol
	etfList := ep.getETFsForSymbol(symbol)
	if len(etfList) == 0 {
		// No ETFs available for this symbol - return zero flows
		snapshot.FlowTint = 0.0
		return snapshot, nil
	}

	snapshot.ETFList = etfList

	// For each relevant ETF, fetch flow data
	for _, etfTicker := range etfList {
		flow, err := ep.fetchETFFlow(ctx, etfTicker, symbol)
		if err != nil {
			// Log error but continue with other ETFs
			continue
		}

		snapshot.ETFFlows[etfTicker] = flow
		snapshot.DataSources[etfTicker] = "mock_etf_api" // In practice: Bloomberg, EDGAR, etc.
	}

	// Calculate aggregated metrics
	ep.calculateFlowMetrics(snapshot)

	return snapshot, nil
}

// getETFsForSymbol returns list of relevant ETF tickers for a crypto symbol
func (ep *ETFProvider) getETFsForSymbol(symbol string) []string {
	// Map crypto symbols to their relevant ETFs
	etfMappings := map[string][]string{
		"BTC": {
			"GBTC", // Grayscale Bitcoin Trust
			"BITB", // Bitwise Bitcoin ETF
			"IBIT", // iShares Bitcoin Trust
			"FBTC", // Fidelity Wise Origin Bitcoin Fund
			"BTCO", // Invesco Galaxy Bitcoin ETF
		},
		"ETH": {
			"ETHE", // Grayscale Ethereum Trust
			"ETHW", // iShares Ethereum Trust
			"FETH", // Fidelity Ethereum Fund
			"ETHV", // Valkyrie Ethereum Strategy ETF
		},
		// For other cryptos, return empty (no major ETFs yet)
	}

	if etfs, exists := etfMappings[strings.ToUpper(symbol)]; exists {
		return etfs
	}

	return []string{}
}

// fetchETFFlow gets flow data for a specific ETF (mocked for now)
func (ep *ETFProvider) fetchETFFlow(ctx context.Context, etfTicker, underlyingSymbol string) (float64, error) {
	// In practice, this would fetch from:
	// - SEC EDGAR filings for official holdings
	// - ETF provider APIs (if available)
	// - Financial data vendors (Bloomberg, Refinitiv)
	//
	// For now, we'll generate realistic mock data based on the ETF and market conditions

	// Mock flow based on ETF characteristics and recent crypto performance
	flow := ep.generateMockETFFlow(etfTicker, underlyingSymbol)

	return flow, nil
}

// generateMockETFFlow creates realistic mock ETF flow data
func (ep *ETFProvider) generateMockETFFlow(etfTicker, symbol string) float64 {
	// Generate deterministic but realistic flows
	hash := 0
	for _, c := range etfTicker + symbol {
		hash = hash*31 + int(c)
	}

	// Base flow magnitude (in millions USD)
	baseMagnitude := map[string]float64{
		"GBTC": 50.0, // Large flows for major ETFs
		"IBIT": 45.0,
		"BITB": 30.0,
		"FBTC": 25.0,
		"BTCO": 15.0,
		"ETHE": 20.0, // Smaller ETH ETF flows
		"ETHW": 15.0,
		"FETH": 10.0,
		"ETHV": 8.0,
	}

	magnitude := baseMagnitude[etfTicker]
	if magnitude == 0 {
		magnitude = 5.0 // Default for unknown ETFs
	}

	// Random component for flow direction and size
	randomValue := float64((hash%10000)-5000) / 10000.0 // -0.5 to +0.5

	// Apply some persistence (flows tend to continue in same direction)
	persistence := 0.3
	flow := magnitude * (randomValue + persistence*math.Copysign(0.2, randomValue))

	// Add some market regime influence
	// In bull markets, more inflows; in bear markets, more outflows
	// For mock purposes, assume neutral to slightly bullish market
	marketTint := 0.1
	flow += magnitude * marketTint

	return flow * 1e6 // Convert millions to actual USD
}

// calculateFlowMetrics computes aggregate flow metrics and tint
func (ep *ETFProvider) calculateFlowMetrics(snapshot *ETFFlowSnapshot) {
	snapshot.TotalFlow = 0.0
	snapshot.FlowVolume = 0.0

	// Sum all flows
	for _, flow := range snapshot.ETFFlows {
		snapshot.TotalFlow += flow
		snapshot.FlowVolume += math.Abs(flow)
	}

	// Calculate tint: normalize total flow by total volume
	// Tint ranges from -1 (all outflows) to +1 (all inflows)
	if snapshot.FlowVolume > 0 {
		snapshot.FlowTint = snapshot.TotalFlow / snapshot.FlowVolume

		// Clamp to [-1, +1] range
		snapshot.FlowTint = math.Max(-1.0, math.Min(1.0, snapshot.FlowTint))
	} else {
		snapshot.FlowTint = 0.0
	}
}

// Helper methods for ETF analysis

// HasNetInflows checks if there are net ETF inflows above threshold
func (efs *ETFFlowSnapshot) HasNetInflows(thresholdUSD float64) bool {
	return efs.TotalFlow > thresholdUSD
}

// HasNetOutflows checks if there are net ETF outflows below threshold
func (efs *ETFFlowSnapshot) HasNetOutflows(thresholdUSD float64) bool {
	return efs.TotalFlow < -thresholdUSD
}

// GetFlowIntensity returns flow intensity as percentage of typical volume
func (efs *ETFFlowSnapshot) GetFlowIntensity(typicalDailyVolumeUSD float64) float64 {
	if typicalDailyVolumeUSD > 0 {
		return efs.FlowVolume / typicalDailyVolumeUSD
	}
	return 0.0
}

// IsFlowTintBullish checks if ETF tint suggests bullish sentiment
func (efs *ETFFlowSnapshot) IsFlowTintBullish(threshold float64) bool {
	return efs.FlowTint > threshold
}

// IsFlowTintBearish checks if ETF tint suggests bearish sentiment
func (efs *ETFFlowSnapshot) IsFlowTintBearish(threshold float64) bool {
	return efs.FlowTint < -threshold
}

// GetDominantETF returns the ETF with the largest absolute flow
func (efs *ETFFlowSnapshot) GetDominantETF() (string, float64) {
	dominantETF := ""
	maxAbsFlow := 0.0

	for etf, flow := range efs.ETFFlows {
		absFlow := math.Abs(flow)
		if absFlow > maxAbsFlow {
			maxAbsFlow = absFlow
			dominantETF = etf
		}
	}

	return dominantETF, efs.ETFFlows[dominantETF]
}

// GetETFFlowSummary returns a formatted summary of ETF flows
func (efs *ETFFlowSnapshot) GetETFFlowSummary() string {
	if len(efs.ETFFlows) == 0 {
		return "No ETF data available"
	}

	summary := fmt.Sprintf("ETF Flows: Net $%.1fM", efs.TotalFlow/1e6)
	summary += fmt.Sprintf(" | Tint: %.2f", efs.FlowTint)

	dominantETF, dominantFlow := efs.GetDominantETF()
	if dominantETF != "" {
		summary += fmt.Sprintf(" | %s: $%.1fM", dominantETF, dominantFlow/1e6)
	}

	return summary
}
