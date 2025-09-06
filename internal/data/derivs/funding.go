package derivs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FundingProvider collects funding rates from multiple free venues with z-score calculation
type FundingProvider struct {
	httpClient *http.Client
	cacheDir   string
	cache      map[string]*CachedFunding
}

// NewFundingProvider creates a funding rate provider
func NewFundingProvider() *FundingProvider {
	return &FundingProvider{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cacheDir:   "./cache/funding",
		cache:      make(map[string]*CachedFunding),
	}
}

// FundingSnapshot contains cross-venue funding data for z-score calculation
type FundingSnapshot struct {
	Symbol                   string             `json:"symbol"`
	Timestamp                time.Time          `json:"timestamp"`
	MonotonicTimestamp       int64              `json:"monotonic_timestamp"`
	VenueRates               map[string]float64 `json:"venue_rates"`                // venue -> current_rate
	VenueMedian              float64            `json:"venue_median"`               // Current cross-venue median
	SevenDayMean             float64            `json:"seven_day_mean"`             // μ from 7d rolling
	SevenDayStd              float64            `json:"seven_day_std"`              // σ from 7d rolling
	FundingZ                 float64            `json:"funding_z"`                  // (median - μ) / σ
	MaxVenueDivergence       float64            `json:"max_venue_divergence"`       // max |venue_rate - median|
	FundingDivergencePresent bool               `json:"funding_divergence_present"` // Entry gate requirement
	DataSources              map[string]string  `json:"data_sources"`               // venue -> api_used
	CacheHit                 bool               `json:"cache_hit"`
	SignatureHash            string             `json:"signature_hash"` // Data integrity hash
}

// CachedFunding wraps funding data with cache metadata
type CachedFunding struct {
	Snapshot *FundingSnapshot `json:"snapshot"`
	CachedAt time.Time        `json:"cached_at"`
	TTL      time.Duration    `json:"ttl"`
}

// HistoricalFundingData represents cached historical funding data for z-score calculation
type HistoricalFundingData struct {
	Symbol        string             `json:"symbol"`
	DataPoints    []FundingDataPoint `json:"data_points"` // 7d rolling window
	LastUpdated   time.Time          `json:"last_updated"`
	Source        string             `json:"source"`
	SignatureHash string             `json:"signature_hash"`
}

// FundingDataPoint represents a single funding rate observation
type FundingDataPoint struct {
	Timestamp   time.Time          `json:"timestamp"`
	VenueMedian float64            `json:"venue_median"`
	VenueRates  map[string]float64 `json:"venue_rates"`
}

// GetFundingSnapshot retrieves cross-venue funding rates and computes z-scores
func (fp *FundingProvider) GetFundingSnapshot(ctx context.Context, symbol string) (*FundingSnapshot, error) {
	cacheKey := fmt.Sprintf("funding_%s", symbol)

	// Check in-memory cache first
	if cached, exists := fp.cache[cacheKey]; exists {
		if time.Since(cached.CachedAt) < cached.TTL {
			cached.Snapshot.CacheHit = true
			return cached.Snapshot, nil
		}
		// Expired - remove from cache
		delete(fp.cache, cacheKey)
	}

	// Check disk cache (TTL ≥ 600s as required)
	diskSnapshot, err := fp.loadFromDiskCache(symbol)
	if err == nil && time.Since(diskSnapshot.Timestamp) < 10*time.Minute {
		diskSnapshot.CacheHit = true

		// Update in-memory cache
		fp.cache[cacheKey] = &CachedFunding{
			Snapshot: diskSnapshot,
			CachedAt: time.Now(),
			TTL:      10 * time.Minute,
		}

		return diskSnapshot, nil
	}

	// Fetch fresh data from multiple venues
	snapshot, err := fp.fetchFundingSnapshotWithZScore(ctx, symbol)
	if err != nil {
		return nil, err
	}

	// Cache both in-memory and disk
	fp.cache[cacheKey] = &CachedFunding{
		Snapshot: snapshot,
		CachedAt: time.Now(),
		TTL:      10 * time.Minute, // TTL ≥ 600s requirement
	}

	// Save to disk cache
	if err := fp.saveToDiskCache(snapshot); err != nil {
		// Log error but don't fail - in-memory cache still works
		fmt.Printf("Warning: failed to save funding cache for %s: %v\n", symbol, err)
	}

	snapshot.CacheHit = false
	return snapshot, nil
}

// fetchFundingSnapshotWithZScore collects funding rates and computes z-score
func (fp *FundingProvider) fetchFundingSnapshotWithZScore(ctx context.Context, symbol string) (*FundingSnapshot, error) {
	now := time.Now()
	snapshot := &FundingSnapshot{
		Symbol:             symbol,
		Timestamp:          now,
		MonotonicTimestamp: now.UnixNano(),
		VenueRates:         make(map[string]float64),
		DataSources:        make(map[string]string),
	}

	// Fetch from Binance (free, no key required)
	if rate, err := fp.fetchBinanceFunding(ctx, symbol); err == nil {
		snapshot.VenueRates["binance"] = rate
		snapshot.DataSources["binance"] = "binance_fapi_v1"
	}

	// Fetch from OKX (free, no key required)
	if rate, err := fp.fetchOKXFunding(ctx, symbol); err == nil {
		snapshot.VenueRates["okx"] = rate
		snapshot.DataSources["okx"] = "okx_api_v5"
	}

	// Fetch from Bybit (free, no key required)
	if rate, err := fp.fetchBybitFunding(ctx, symbol); err == nil {
		snapshot.VenueRates["bybit"] = rate
		snapshot.DataSources["bybit"] = "bybit_v5"
	}

	// Need at least 2 venues for meaningful calculation
	if len(snapshot.VenueRates) < 2 {
		return nil, fmt.Errorf("insufficient funding data: only %d venues available", len(snapshot.VenueRates))
	}

	// Step 1: Calculate venue median
	snapshot.VenueMedian = fp.calculateVenueMedian(snapshot.VenueRates)

	// Step 2: Load/update historical data and calculate 7d rolling stats
	if err := fp.calculateZScoreFromHistory(snapshot); err != nil {
		return nil, fmt.Errorf("failed to calculate z-score: %w", err)
	}

	// Step 3: Update historical cache with current data point
	if err := fp.updateHistoricalCache(snapshot); err != nil {
		// Log warning but don't fail - we have the z-score
		fmt.Printf("Warning: failed to update historical cache for %s: %v\n", symbol, err)
	}

	// Step 4: Calculate signature hash for data integrity
	snapshot.SignatureHash = fp.calculateSignatureHash(snapshot)

	return snapshot, nil
}

// fetchBinanceFunding gets funding rate from Binance futures API
func (fp *FundingProvider) fetchBinanceFunding(ctx context.Context, symbol string) (float64, error) {
	// Convert symbol to Binance format (e.g., BTC -> BTCUSDT)
	binanceSymbol := fp.toBinanceSymbol(symbol)

	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", binanceSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create Binance request: %w", err)
	}

	resp, err := fp.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Binance funding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("Binance API error: %d %s", resp.StatusCode, string(body))
	}

	var result BinanceFundingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode Binance response: %w", err)
	}

	return result.LastFundingRate, nil
}

// fetchOKXFunding gets funding rate from OKX API
func (fp *FundingProvider) fetchOKXFunding(ctx context.Context, symbol string) (float64, error) {
	// Convert symbol to OKX format (e.g., BTC -> BTC-USDT-SWAP)
	okxSymbol := fp.toOKXSymbol(symbol)

	url := fmt.Sprintf("https://www.okx.com/api/v5/public/funding-rate?instId=%s", okxSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create OKX request: %w", err)
	}

	resp, err := fp.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("OKX funding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("OKX API error: %d %s", resp.StatusCode, string(body))
	}

	var result OKXFundingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode OKX response: %w", err)
	}

	if len(result.Data) == 0 {
		return 0, fmt.Errorf("no OKX funding data available")
	}

	return result.Data[0].FundingRate, nil
}

// fetchBybitFunding gets funding rate from Bybit API
func (fp *FundingProvider) fetchBybitFunding(ctx context.Context, symbol string) (float64, error) {
	// Convert symbol to Bybit format (e.g., BTC -> BTCUSDT)
	bybitSymbol := fp.toBybitSymbol(symbol)

	url := fmt.Sprintf("https://api.bybit.com/v5/market/funding/history?category=linear&symbol=%s&limit=1", bybitSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create Bybit request: %w", err)
	}

	resp, err := fp.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Bybit funding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("Bybit API error: %d %s", resp.StatusCode, string(body))
	}

	var result BybitFundingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode Bybit response: %w", err)
	}

	if len(result.Result.List) == 0 {
		return 0, fmt.Errorf("no Bybit funding data available")
	}

	return result.Result.List[0].FundingRate, nil
}

// calculateVenueMedian computes the median funding rate across venues
func (fp *FundingProvider) calculateVenueMedian(venueRates map[string]float64) float64 {
	rates := make([]float64, 0, len(venueRates))
	for _, rate := range venueRates {
		rates = append(rates, rate)
	}

	sort.Float64s(rates)
	n := len(rates)

	if n%2 == 0 {
		return (rates[n/2-1] + rates[n/2]) / 2
	} else {
		return rates[n/2]
	}
}

// calculateZScoreFromHistory loads historical data and computes 7d rolling μ/σ for z-score
func (fp *FundingProvider) calculateZScoreFromHistory(snapshot *FundingSnapshot) error {
	// Load historical data from cache
	historical, err := fp.loadHistoricalData(snapshot.Symbol)
	if err != nil {
		// First time for this symbol - initialize with zeros
		snapshot.SevenDayMean = snapshot.VenueMedian
		snapshot.SevenDayStd = 0.01 // Small default std to avoid division by zero
		snapshot.FundingZ = 0.0
		snapshot.FundingDivergencePresent = false
		return nil
	}

	// Calculate 7d rolling statistics from historical medians
	if len(historical.DataPoints) < 2 {
		// Not enough history - use current as baseline
		snapshot.SevenDayMean = snapshot.VenueMedian
		snapshot.SevenDayStd = 0.01
		snapshot.FundingZ = 0.0
		snapshot.FundingDivergencePresent = false
		return nil
	}

	// Extract medians from last 7 days
	medians := make([]float64, 0, len(historical.DataPoints))
	cutoff := time.Now().Add(-7 * 24 * time.Hour)

	for _, point := range historical.DataPoints {
		if point.Timestamp.After(cutoff) {
			medians = append(medians, point.VenueMedian)
		}
	}

	if len(medians) < 2 {
		snapshot.SevenDayMean = snapshot.VenueMedian
		snapshot.SevenDayStd = 0.01
		snapshot.FundingZ = 0.0
		snapshot.FundingDivergencePresent = false
		return nil
	}

	// Calculate μ and σ
	sum := 0.0
	for _, median := range medians {
		sum += median
	}
	snapshot.SevenDayMean = sum / float64(len(medians))

	sumSq := 0.0
	for _, median := range medians {
		diff := median - snapshot.SevenDayMean
		sumSq += diff * diff
	}
	snapshot.SevenDayStd = math.Sqrt(sumSq / float64(len(medians)))

	// Avoid division by zero
	if snapshot.SevenDayStd < 1e-8 {
		snapshot.SevenDayStd = 1e-8
	}

	// Calculate z-score: (current_median - μ) / σ
	snapshot.FundingZ = (snapshot.VenueMedian - snapshot.SevenDayMean) / snapshot.SevenDayStd

	// Calculate max venue divergence from median
	snapshot.MaxVenueDivergence = 0.0
	for _, rate := range snapshot.VenueRates {
		divergence := math.Abs(rate - snapshot.VenueMedian)
		if divergence > snapshot.MaxVenueDivergence {
			snapshot.MaxVenueDivergence = divergence
		}
	}

	// Determine if funding divergence is present (entry gate requirement)
	// Rule: |z-score| >= 2.0 AND max venue divergence >= 0.005% (5bps)
	snapshot.FundingDivergencePresent = math.Abs(snapshot.FundingZ) >= 2.0 &&
		snapshot.MaxVenueDivergence >= 0.00005

	return nil
}

// loadHistoricalData loads 7d historical funding data from disk cache
func (fp *FundingProvider) loadHistoricalData(symbol string) (*HistoricalFundingData, error) {
	cacheFile := filepath.Join(fp.cacheDir, fmt.Sprintf("%s_history.json", strings.ToLower(symbol)))

	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("no historical data for %s", symbol)
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read historical cache: %w", err)
	}

	var historical HistoricalFundingData
	if err := json.Unmarshal(data, &historical); err != nil {
		return nil, fmt.Errorf("failed to parse historical cache: %w", err)
	}

	return &historical, nil
}

// updateHistoricalCache updates the 7d rolling cache with current data point
func (fp *FundingProvider) updateHistoricalCache(snapshot *FundingSnapshot) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(fp.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Load existing historical data or create new
	historical, err := fp.loadHistoricalData(snapshot.Symbol)
	if err != nil {
		// Create new historical data
		historical = &HistoricalFundingData{
			Symbol:      snapshot.Symbol,
			DataPoints:  []FundingDataPoint{},
			LastUpdated: snapshot.Timestamp,
			Source:      "cryptorun_funding_provider",
		}
	}

	// Add current data point
	newPoint := FundingDataPoint{
		Timestamp:   snapshot.Timestamp,
		VenueMedian: snapshot.VenueMedian,
		VenueRates:  make(map[string]float64),
	}

	// Copy venue rates
	for venue, rate := range snapshot.VenueRates {
		newPoint.VenueRates[venue] = rate
	}

	historical.DataPoints = append(historical.DataPoints, newPoint)
	historical.LastUpdated = snapshot.Timestamp

	// Keep only last 7 days + 1 day buffer for rolling calculation
	cutoff := time.Now().Add(-8 * 24 * time.Hour)
	filtered := make([]FundingDataPoint, 0, len(historical.DataPoints))

	for _, point := range historical.DataPoints {
		if point.Timestamp.After(cutoff) {
			filtered = append(filtered, point)
		}
	}

	historical.DataPoints = filtered
	historical.SignatureHash = fp.calculateHistoricalHash(historical)

	// Save to disk
	cacheFile := filepath.Join(fp.cacheDir, fmt.Sprintf("%s_history.json", strings.ToLower(snapshot.Symbol)))

	data, err := json.MarshalIndent(historical, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal historical data: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write historical cache: %w", err)
	}

	return nil
}

// loadFromDiskCache loads snapshot from disk cache
func (fp *FundingProvider) loadFromDiskCache(symbol string) (*FundingSnapshot, error) {
	cacheFile := filepath.Join(fp.cacheDir, fmt.Sprintf("%s.json", strings.ToLower(symbol)))

	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("no disk cache for %s", symbol)
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read disk cache: %w", err)
	}

	var snapshot FundingSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse disk cache: %w", err)
	}

	return &snapshot, nil
}

// saveToDiskCache saves snapshot to disk cache
func (fp *FundingProvider) saveToDiskCache(snapshot *FundingSnapshot) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(fp.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cacheFile := filepath.Join(fp.cacheDir, fmt.Sprintf("%s.json", strings.ToLower(snapshot.Symbol)))

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write disk cache: %w", err)
	}

	return nil
}

// calculateSignatureHash computes data integrity hash for snapshot
func (fp *FundingProvider) calculateSignatureHash(snapshot *FundingSnapshot) string {
	// Simple hash of key data fields
	hashInput := fmt.Sprintf("%s_%d_%.6f_%.6f_%.6f",
		snapshot.Symbol, snapshot.MonotonicTimestamp,
		snapshot.VenueMedian, snapshot.SevenDayMean, snapshot.SevenDayStd)

	// Simple hash function (in production would use crypto/sha256)
	hash := 0
	for _, c := range hashInput {
		hash = hash*31 + int(c)
	}

	return fmt.Sprintf("%x", hash)
}

// calculateHistoricalHash computes hash for historical data integrity
func (fp *FundingProvider) calculateHistoricalHash(historical *HistoricalFundingData) string {
	hashInput := fmt.Sprintf("%s_%d_%d",
		historical.Symbol, historical.LastUpdated.Unix(), len(historical.DataPoints))

	hash := 0
	for _, c := range hashInput {
		hash = hash*31 + int(c)
	}

	return fmt.Sprintf("%x", hash)
}

// Symbol conversion helpers for different venues

func (fp *FundingProvider) toBinanceSymbol(symbol string) string {
	// Most common mapping for perpetual futures
	return strings.ToUpper(symbol) + "USDT"
}

func (fp *FundingProvider) toOKXSymbol(symbol string) string {
	// OKX perpetual swap format
	return strings.ToUpper(symbol) + "-USDT-SWAP"
}

func (fp *FundingProvider) toBybitSymbol(symbol string) string {
	// Bybit linear contract format
	return strings.ToUpper(symbol) + "USDT"
}

// HasSignificantDivergence checks if funding divergence exceeds threshold for entry gate
func (fs *FundingSnapshot) HasSignificantDivergence(threshold float64) bool {
	return fs.FundingDivergencePresent && math.Abs(fs.FundingZ) >= threshold
}

// GetDivergentVenue returns the venue with highest funding divergence from median
func (fs *FundingSnapshot) GetDivergentVenue() (string, float64) {
	maxVenue := ""
	maxDivergence := 0.0

	for venue, rate := range fs.VenueRates {
		divergence := math.Abs(rate - fs.VenueMedian)
		if divergence > maxDivergence {
			maxDivergence = divergence
			maxVenue = venue
		}
	}

	return maxVenue, maxDivergence
}

// GetFundingZScore returns the calculated z-score
func (fs *FundingSnapshot) GetFundingZScore() float64 {
	return fs.FundingZ
}

// API Response structures

type BinanceFundingResponse struct {
	Symbol          string  `json:"symbol"`
	MarkPrice       string  `json:"markPrice"`
	IndexPrice      string  `json:"indexPrice"`
	EstimatedSettle string  `json:"estimatedSettlePrice"`
	LastFundingRate float64 `json:"lastFundingRate"`
	NextFundingTime int64   `json:"nextFundingTime"`
	InterestRate    string  `json:"interestRate"`
	Time            int64   `json:"time"`
}

type OKXFundingResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		InstType    string  `json:"instType"`
		InstId      string  `json:"instId"`
		FundingRate float64 `json:"fundingRate"`
		NextFunding string  `json:"nextFundingTime"`
		FundingTime string  `json:"fundingTime"`
	} `json:"data"`
}

type BybitFundingResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Category string `json:"category"`
		List     []struct {
			Symbol      string  `json:"symbol"`
			FundingRate float64 `json:"fundingRate"`
			FundingTime string  `json:"fundingRateTimestamp"`
		} `json:"list"`
	} `json:"result"`
}
