package etf

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ETFFlowProvider provides ETF flow tint calculations for BTC/ETH
// Uses free issuer dashboards and daily net creation/redemption data
type ETFFlowProvider struct {
	cacheMutex  sync.RWMutex
	memoryCache map[string]*ETFSnapshot
	cacheDir    string
	clientID    string
}

// ETFSnapshot represents daily ETF flow data with tint calculation
type ETFSnapshot struct {
	Symbol             string   `json:"symbol"`
	MonotonicTimestamp int64    `json:"monotonic_timestamp"`
	NetFlowUSD         float64  `json:"net_flow_usd"` // Daily net creation/redemption in USD
	ADV_USD_7d         float64  `json:"adv_usd_7d"`   // 7-day average daily volume in USD
	FlowTint           float64  `json:"flow_tint"`    // Clamped flow/ADV ratio
	ETFList            []string `json:"etf_list"`     // Contributing ETFs
	CacheHit           bool     `json:"cache_hit"`
	Source             string   `json:"source"`
	SignatureHash      string   `json:"signature_hash"`
}

// HistoricalETFData holds 7-day ADV calculation data
type HistoricalETFData struct {
	Symbol        string         `json:"symbol"`
	DataPoints    []ETFDataPoint `json:"data_points"` // 7d rolling window
	LastUpdated   int64          `json:"last_updated"`
	SignatureHash string         `json:"signature_hash"`
}

// ETFDataPoint represents a single day's ETF data
type ETFDataPoint struct {
	Timestamp  int64   `json:"timestamp"`
	VolumeUSD  float64 `json:"volume_usd"`
	NetFlowUSD float64 `json:"net_flow_usd"`
}

// NewETFFlowProvider creates a new ETF flow provider
func NewETFFlowProvider() *ETFFlowProvider {
	return &ETFFlowProvider{
		memoryCache: make(map[string]*ETFSnapshot),
		cacheDir:    "./cache/etf",
		clientID:    fmt.Sprintf("cryptorun-etf-%d", time.Now().Unix()),
	}
}

// GetETFFlowSnapshot returns ETF flow data with tint calculation
func (efp *ETFFlowProvider) GetETFFlowSnapshot(ctx context.Context, symbol string) (*ETFSnapshot, error) {
	// Check memory cache first
	efp.cacheMutex.RLock()
	if cached, exists := efp.memoryCache[symbol]; exists {
		if time.Now().Unix()-cached.MonotonicTimestamp < 86400 { // 24h TTL
			efp.cacheMutex.RUnlock()
			cached.CacheHit = true
			return cached, nil
		}
	}
	efp.cacheMutex.RUnlock()

	// Check disk cache
	if snapshot, err := efp.loadFromDiskCache(symbol); err == nil {
		if time.Now().Unix()-snapshot.MonotonicTimestamp < 86400 {
			// Update memory cache
			efp.cacheMutex.Lock()
			efp.memoryCache[symbol] = snapshot
			efp.cacheMutex.Unlock()

			snapshot.CacheHit = true
			return snapshot, nil
		}
	}

	// Fetch fresh data
	snapshot, err := efp.fetchETFFlowSnapshot(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ETF flow data: %w", err)
	}

	// Calculate flow tint
	if err := efp.calculateFlowTint(snapshot); err != nil {
		return nil, fmt.Errorf("failed to calculate flow tint: %w", err)
	}

	// Update historical data for ADV calculation
	if err := efp.updateHistoricalCache(symbol, snapshot); err != nil {
		// Log but don't fail - we can still return the snapshot
		fmt.Printf("Warning: failed to update ETF historical cache: %v\n", err)
	}

	// Cache the result
	efp.cacheMutex.Lock()
	efp.memoryCache[symbol] = snapshot
	efp.cacheMutex.Unlock()

	if err := efp.saveToDiskCache(snapshot); err != nil {
		fmt.Printf("Warning: failed to save ETF snapshot to disk: %v\n", err)
	}

	return snapshot, nil
}

// fetchETFFlowSnapshot fetches ETF data from free endpoints
func (efp *ETFFlowProvider) fetchETFFlowSnapshot(ctx context.Context, symbol string) (*ETFSnapshot, error) {
	timestamp := time.Now().Unix()

	// Mock implementation - in production, would fetch from:
	// - BlackRock iShares dashboard (IBIT)
	// - Grayscale dashboard (GBTC)
	// - Fidelity dashboard (FBTC)
	// - ARK dashboard (ARKB)
	// - VanEck dashboard (HODL)
	var netFlowUSD float64
	var etfList []string

	switch strings.ToUpper(symbol) {
	case "BTCUSD":
		// Mock: Sum of daily net creations across major BTC ETFs
		netFlowUSD = efp.mockBTCETFFlow()
		etfList = []string{"IBIT", "GBTC", "FBTC", "ARKB", "HODL"}
	case "ETHUSD":
		// Mock: Sum of daily net creations across ETH ETFs when available
		netFlowUSD = efp.mockETHETFFlow()
		etfList = []string{"ETHA", "ETH"} // Placeholder - ETH ETFs pending approval
	default:
		return nil, fmt.Errorf("ETF flow data not available for symbol: %s", symbol)
	}

	snapshot := &ETFSnapshot{
		Symbol:             symbol,
		MonotonicTimestamp: timestamp,
		NetFlowUSD:         netFlowUSD,
		ETFList:            etfList,
		CacheHit:           false,
		Source:             "issuer-dashboards",
		SignatureHash:      "", // Will be calculated after tint
	}

	return snapshot, nil
}

// calculateFlowTint calculates the flow tint with ADV normalization
func (efp *ETFFlowProvider) calculateFlowTint(snapshot *ETFSnapshot) error {
	// Load historical data for ADV calculation
	historical, err := efp.loadHistoricalData(snapshot.Symbol)
	if err != nil {
		// If no historical data, use conservative default ADV
		snapshot.ADV_USD_7d = efp.getDefaultADV(snapshot.Symbol)
	} else {
		snapshot.ADV_USD_7d = efp.calculate7DayADV(historical)
	}

	// Calculate tint: flow_USD / ADV_USD_7d, clamped to ±2%
	if snapshot.ADV_USD_7d > 0 {
		rawTint := snapshot.NetFlowUSD / snapshot.ADV_USD_7d
		snapshot.FlowTint = math.Max(-0.02, math.Min(0.02, rawTint))
	} else {
		snapshot.FlowTint = 0.0
	}

	// Calculate signature hash after all fields are set
	snapshot.SignatureHash = efp.calculateSignatureHash(snapshot)

	return nil
}

// calculate7DayADV computes the 7-day average daily volume
func (efp *ETFFlowProvider) calculate7DayADV(historical *HistoricalETFData) float64 {
	if len(historical.DataPoints) == 0 {
		return 0.0
	}

	var totalVolume float64
	validDays := 0

	// Use last 7 data points
	start := len(historical.DataPoints) - 7
	if start < 0 {
		start = 0
	}

	for i := start; i < len(historical.DataPoints); i++ {
		if historical.DataPoints[i].VolumeUSD > 0 {
			totalVolume += historical.DataPoints[i].VolumeUSD
			validDays++
		}
	}

	if validDays > 0 {
		return totalVolume / float64(validDays)
	}
	return 0.0
}

// getDefaultADV returns conservative default ADV when historical data is unavailable
func (efp *ETFFlowProvider) getDefaultADV(symbol string) float64 {
	switch strings.ToUpper(symbol) {
	case "BTCUSD":
		return 1_000_000_000.0 // $1B daily volume estimate for BTC ETFs
	case "ETHUSD":
		return 200_000_000.0 // $200M daily volume estimate for ETH ETFs
	default:
		return 100_000_000.0 // $100M default
	}
}

// Mock implementations for free ETF data (production would use actual dashboards)

func (efp *ETFFlowProvider) mockBTCETFFlow() float64 {
	// Mock daily net creation across major BTC ETFs
	// In production: aggregate from BlackRock, Grayscale, Fidelity dashboards
	now := time.Now()

	// Create deterministic but varying mock data based on day
	dayOffset := now.YearDay() % 30
	baseFlow := 50_000_000.0 // $50M base

	// Add some variation: ±$100M
	variation := math.Sin(float64(dayOffset)*0.2) * 100_000_000.0

	return baseFlow + variation
}

func (efp *ETFFlowProvider) mockETHETFFlow() float64 {
	// Mock for ETH ETFs (currently pending approval)
	now := time.Now()
	dayOffset := now.YearDay() % 20
	baseFlow := 10_000_000.0 // $10M base (smaller than BTC)

	variation := math.Cos(float64(dayOffset)*0.3) * 20_000_000.0

	return baseFlow + variation
}

// Cache management functions

func (efp *ETFFlowProvider) loadFromDiskCache(symbol string) (*ETFSnapshot, error) {
	filePath := filepath.Join(efp.cacheDir, fmt.Sprintf("%s.json", strings.ToLower(symbol)))

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var snapshot ETFSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

func (efp *ETFFlowProvider) saveToDiskCache(snapshot *ETFSnapshot) error {
	if err := os.MkdirAll(efp.cacheDir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(efp.cacheDir, fmt.Sprintf("%s.json", strings.ToLower(snapshot.Symbol)))

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

func (efp *ETFFlowProvider) loadHistoricalData(symbol string) (*HistoricalETFData, error) {
	filePath := filepath.Join(efp.cacheDir, fmt.Sprintf("%s_historical.json", strings.ToLower(symbol)))

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var historical HistoricalETFData
	if err := json.Unmarshal(data, &historical); err != nil {
		return nil, err
	}

	return &historical, nil
}

func (efp *ETFFlowProvider) updateHistoricalCache(symbol string, snapshot *ETFSnapshot) error {
	// Load existing historical data or create new
	historical, err := efp.loadHistoricalData(symbol)
	if err != nil {
		historical = &HistoricalETFData{
			Symbol:      symbol,
			DataPoints:  make([]ETFDataPoint, 0),
			LastUpdated: 0,
		}
	}

	// Add current data point
	dataPoint := ETFDataPoint{
		Timestamp:  snapshot.MonotonicTimestamp,
		VolumeUSD:  snapshot.ADV_USD_7d, // Approximation
		NetFlowUSD: snapshot.NetFlowUSD,
	}

	// Keep only last 8 days (7d + 1 buffer)
	historical.DataPoints = append(historical.DataPoints, dataPoint)
	if len(historical.DataPoints) > 8 {
		historical.DataPoints = historical.DataPoints[len(historical.DataPoints)-8:]
	}

	historical.LastUpdated = snapshot.MonotonicTimestamp
	historical.SignatureHash = efp.calculateHistoricalSignatureHash(historical)

	// Save back to disk
	filePath := filepath.Join(efp.cacheDir, fmt.Sprintf("%s_historical.json", strings.ToLower(symbol)))
	if err := os.MkdirAll(efp.cacheDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(historical, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// Data integrity functions

func (efp *ETFFlowProvider) calculateSignatureHash(snapshot *ETFSnapshot) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s|%d|%.6f|%.6f|%.6f|%s",
		snapshot.Symbol,
		snapshot.MonotonicTimestamp,
		snapshot.NetFlowUSD,
		snapshot.ADV_USD_7d,
		snapshot.FlowTint,
		strings.Join(snapshot.ETFList, ","))))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

func (efp *ETFFlowProvider) calculateHistoricalSignatureHash(historical *HistoricalETFData) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s|%d|%d",
		historical.Symbol,
		historical.LastUpdated,
		len(historical.DataPoints))))

	for _, point := range historical.DataPoints {
		h.Write([]byte(fmt.Sprintf("|%d|%.6f|%.6f",
			point.Timestamp,
			point.VolumeUSD,
			point.NetFlowUSD)))
	}

	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// IsFlowTintBullish checks if the ETF flow tint indicates bullish sentiment
func (snapshot *ETFSnapshot) IsFlowTintBullish(threshold float64) bool {
	return snapshot.FlowTint >= threshold
}

// GetFlowTintStrength returns a qualitative assessment of flow strength
func (snapshot *ETFSnapshot) GetFlowTintStrength() string {
	tint := snapshot.FlowTint

	if tint >= 0.015 {
		return "Very Strong Inflow"
	} else if tint >= 0.01 {
		return "Strong Inflow"
	} else if tint >= 0.005 {
		return "Moderate Inflow"
	} else if tint <= -0.015 {
		return "Very Strong Outflow"
	} else if tint <= -0.01 {
		return "Strong Outflow"
	} else if tint <= -0.005 {
		return "Moderate Outflow"
	} else {
		return "Neutral Flow"
	}
}
