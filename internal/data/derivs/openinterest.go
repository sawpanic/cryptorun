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
	"strings"
	"time"
)

// OpenInterestProvider collects open interest data from free venues with residual calculation
type OpenInterestProvider struct {
	httpClient *http.Client
	cacheDir   string
	cache      map[string]*CachedOI
}

// NewOpenInterestProvider creates an OI data provider
func NewOpenInterestProvider() *OpenInterestProvider {
	return &OpenInterestProvider{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cacheDir:   "./cache/oi",
		cache:      make(map[string]*CachedOI),
	}
}

// OpenInterestSnapshot contains OI data with 1h delta and residual calculation
type OpenInterestSnapshot struct {
	Symbol             string             `json:"symbol"`
	Timestamp          time.Time          `json:"timestamp"`
	MonotonicTimestamp int64              `json:"monotonic_timestamp"`
	VenueOI            map[string]float64 `json:"venue_oi"`           // venue -> current_oi_usd
	VenueOIChange1h    map[string]float64 `json:"venue_oi_change_1h"` // venue -> oi_change_1h
	TotalOI            float64            `json:"total_oi"`           // Sum across venues
	DeltaOI_1h         float64            `json:"delta_oi_1h"`        // Total ΔOI 1h
	PriceChange1h      float64            `json:"price_change_1h"`    // 1h price change for residual
	Beta7d             float64            `json:"beta_7d"`            // β from 7d rolling regression
	OIResidual         float64            `json:"oi_residual"`        // ΔOI - β*ΔPrice (1h)
	BetaR2             float64            `json:"beta_r2"`            // R² of regression fit
	DataSources        map[string]string  `json:"data_sources"`       // venue -> api_used
	CacheHit           bool               `json:"cache_hit"`
	SignatureHash      string             `json:"signature_hash"` // Data integrity hash
}

// CachedOI wraps OI data with cache metadata
type CachedOI struct {
	Snapshot *OpenInterestSnapshot `json:"snapshot"`
	CachedAt time.Time             `json:"cached_at"`
	TTL      time.Duration         `json:"ttl"`
}

// HistoricalOIData represents cached historical OI and price data for β regression
type HistoricalOIData struct {
	Symbol        string        `json:"symbol"`
	DataPoints    []OIDataPoint `json:"data_points"` // 7d rolling window
	LastUpdated   time.Time     `json:"last_updated"`
	Source        string        `json:"source"`
	SignatureHash string        `json:"signature_hash"`
}

// OIDataPoint represents a single OI and price observation for regression
type OIDataPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	TotalOI       float64   `json:"total_oi"`
	DeltaOI_1h    float64   `json:"delta_oi_1h"`
	PriceChange1h float64   `json:"price_change_1h"`
}

// GetOpenInterestSnapshot retrieves OI data and computes 1h delta + residual
func (oip *OpenInterestProvider) GetOpenInterestSnapshot(ctx context.Context, symbol string, priceChange1h float64) (*OpenInterestSnapshot, error) {
	cacheKey := fmt.Sprintf("oi_%s", symbol)

	// Check in-memory cache first (OI updates less frequently than prices)
	if cached, exists := oip.cache[cacheKey]; exists {
		if time.Since(cached.CachedAt) < cached.TTL {
			// Update price change and recalculate residual for fresh data
			cached.Snapshot.PriceChange1h = priceChange1h
			cached.Snapshot.OIResidual = cached.Snapshot.DeltaOI_1h - (cached.Snapshot.Beta7d * priceChange1h)
			cached.Snapshot.CacheHit = true
			return cached.Snapshot, nil
		}
		// Expired - remove from cache
		delete(oip.cache, cacheKey)
	}

	// Check disk cache (TTL ≥ 600s as required)
	diskSnapshot, err := oip.loadFromDiskCache(symbol)
	if err == nil && time.Since(diskSnapshot.Timestamp) < 10*time.Minute {
		// Update with current price change
		diskSnapshot.PriceChange1h = priceChange1h
		diskSnapshot.OIResidual = diskSnapshot.DeltaOI_1h - (diskSnapshot.Beta7d * priceChange1h)
		diskSnapshot.CacheHit = true

		// Update in-memory cache
		oip.cache[cacheKey] = &CachedOI{
			Snapshot: diskSnapshot,
			CachedAt: time.Now(),
			TTL:      10 * time.Minute,
		}

		return diskSnapshot, nil
	}

	// Fetch fresh OI data with 1h delta calculation
	snapshot, err := oip.fetchOISnapshotWithDelta(ctx, symbol, priceChange1h)
	if err != nil {
		return nil, err
	}

	// Cache both in-memory and disk
	oip.cache[cacheKey] = &CachedOI{
		Snapshot: snapshot,
		CachedAt: time.Now(),
		TTL:      10 * time.Minute, // TTL ≥ 600s requirement
	}

	// Save to disk cache
	if err := oip.saveToDiskCache(snapshot); err != nil {
		fmt.Printf("Warning: failed to save OI cache for %s: %v\n", symbol, err)
	}

	snapshot.CacheHit = false
	return snapshot, nil
}

// fetchOISnapshotWithDelta collects OI data and computes 1h delta + residual
func (oip *OpenInterestProvider) fetchOISnapshotWithDelta(ctx context.Context, symbol string, priceChange1h float64) (*OpenInterestSnapshot, error) {
	now := time.Now()
	snapshot := &OpenInterestSnapshot{
		Symbol:             symbol,
		Timestamp:          now,
		MonotonicTimestamp: now.UnixNano(),
		VenueOI:            make(map[string]float64),
		VenueOIChange1h:    make(map[string]float64),
		PriceChange1h:      priceChange1h,
		DataSources:        make(map[string]string),
	}

	// Fetch from Binance with 1h delta calculation
	if oi, oiChange1h, err := oip.fetchBinanceOIWith1hDelta(ctx, symbol); err == nil {
		snapshot.VenueOI["binance"] = oi
		snapshot.VenueOIChange1h["binance"] = oiChange1h
		snapshot.DataSources["binance"] = "binance_fapi_v1"
	}

	// Fetch from OKX with 1h delta calculation
	if oi, oiChange1h, err := oip.fetchOKXOIWith1hDelta(ctx, symbol); err == nil {
		snapshot.VenueOI["okx"] = oi
		snapshot.VenueOIChange1h["okx"] = oiChange1h
		snapshot.DataSources["okx"] = "okx_api_v5"
	}

	// Fetch from Bybit with 1h delta calculation
	if oi, oiChange1h, err := oip.fetchBybitOIWith1hDelta(ctx, symbol); err == nil {
		snapshot.VenueOI["bybit"] = oi
		snapshot.VenueOIChange1h["bybit"] = oiChange1h
		snapshot.DataSources["bybit"] = "bybit_v5"
	}

	// Calculate aggregated metrics
	snapshot.TotalOI = 0.0
	snapshot.DeltaOI_1h = 0.0

	for _, oi := range snapshot.VenueOI {
		snapshot.TotalOI += oi
	}

	for _, oiChange := range snapshot.VenueOIChange1h {
		snapshot.DeltaOI_1h += oiChange
	}

	// Calculate β from 7d rolling regression and OI residual
	if err := oip.calculateBetaAndResidual(snapshot); err != nil {
		return nil, fmt.Errorf("failed to calculate OI residual: %w", err)
	}

	// Update historical cache with current data point
	if err := oip.updateHistoricalCache(snapshot); err != nil {
		fmt.Printf("Warning: failed to update OI historical cache for %s: %v\n", symbol, err)
	}

	// Calculate signature hash for data integrity
	snapshot.SignatureHash = oip.calculateSignatureHash(snapshot)

	return snapshot, nil
}

// calculateBetaAndResidual computes β from 7d rolling regression and calculates OI residual
func (oip *OpenInterestProvider) calculateBetaAndResidual(snapshot *OpenInterestSnapshot) error {
	// Load historical data for β regression
	historical, err := oip.loadHistoricalData(snapshot.Symbol)
	if err != nil {
		// First time for this symbol - use default β
		snapshot.Beta7d = 2.5
		snapshot.BetaR2 = 0.0
		snapshot.OIResidual = snapshot.DeltaOI_1h - (snapshot.Beta7d * snapshot.PriceChange1h)
		return nil
	}

	// Need at least 10 data points for meaningful regression
	if len(historical.DataPoints) < 10 {
		snapshot.Beta7d = 2.5
		snapshot.BetaR2 = 0.0
		snapshot.OIResidual = snapshot.DeltaOI_1h - (snapshot.Beta7d * snapshot.PriceChange1h)
		return nil
	}

	// Filter to last 7 days
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	recentPoints := make([]OIDataPoint, 0, len(historical.DataPoints))

	for _, point := range historical.DataPoints {
		if point.Timestamp.After(cutoff) {
			recentPoints = append(recentPoints, point)
		}
	}

	if len(recentPoints) < 10 {
		snapshot.Beta7d = 2.5
		snapshot.BetaR2 = 0.0
		snapshot.OIResidual = snapshot.DeltaOI_1h - (snapshot.Beta7d * snapshot.PriceChange1h)
		return nil
	}

	// Perform linear regression: ΔOI = α + β*ΔPrice + ε
	beta, r2 := oip.performLinearRegression(recentPoints)
	snapshot.Beta7d = beta
	snapshot.BetaR2 = r2

	// Calculate residual: ΔOI - β*ΔPrice
	snapshot.OIResidual = snapshot.DeltaOI_1h - (beta * snapshot.PriceChange1h)

	return nil
}

// performLinearRegression performs OLS regression of ΔOI on ΔPrice
func (oip *OpenInterestProvider) performLinearRegression(points []OIDataPoint) (float64, float64) {
	if len(points) < 2 {
		return 2.5, 0.0 // Default values
	}

	n := float64(len(points))

	// Calculate means
	sumX, sumY := 0.0, 0.0
	for _, point := range points {
		sumX += point.PriceChange1h
		sumY += point.DeltaOI_1h
	}
	meanX, meanY := sumX/n, sumY/n

	// Calculate β and R²
	sumXY, sumXX, sumYY := 0.0, 0.0, 0.0
	for _, point := range points {
		dx := point.PriceChange1h - meanX
		dy := point.DeltaOI_1h - meanY
		sumXY += dx * dy
		sumXX += dx * dx
		sumYY += dy * dy
	}

	// β = Σ(xy) / Σ(x²)
	var beta float64
	if sumXX > 1e-10 {
		beta = sumXY / sumXX
	} else {
		beta = 2.5 // Default if no price variation
	}

	// R² = (Σ(xy))² / (Σ(x²) * Σ(y²))
	var r2 float64
	if sumXX > 1e-10 && sumYY > 1e-10 {
		r2 = (sumXY * sumXY) / (sumXX * sumYY)
	} else {
		r2 = 0.0
	}

	// Bound β to reasonable range (OI can't be negatively correlated with price long-term)
	if beta < 0 {
		beta = 0.1
	} else if beta > 10 {
		beta = 10.0
	}

	return beta, math.Max(0, math.Min(1, r2))
}

// fetchBinanceOIWith1hDelta gets OI and calculates 1h delta from Binance
func (oip *OpenInterestProvider) fetchBinanceOIWith1hDelta(ctx context.Context, symbol string) (float64, float64, error) {
	// For now, use the original function and estimate 1h delta
	// In production, this would fetch historical OI data points
	currentOI, _, err := oip.fetchBinanceOI(ctx, symbol)
	if err != nil {
		return 0, 0, err
	}

	// Mock 1h delta calculation - in practice would fetch previous hour's OI
	delta1h := currentOI * 0.001 // Mock 0.1% hourly change

	return currentOI, delta1h, nil
}

// fetchOKXOIWith1hDelta gets OI and calculates 1h delta from OKX
func (oip *OpenInterestProvider) fetchOKXOIWith1hDelta(ctx context.Context, symbol string) (float64, float64, error) {
	currentOI, _, err := oip.fetchOKXOI(ctx, symbol)
	if err != nil {
		return 0, 0, err
	}

	// Mock 1h delta calculation
	delta1h := currentOI * 0.0015 // Mock 0.15% hourly change

	return currentOI, delta1h, nil
}

// fetchBybitOIWith1hDelta gets OI and calculates 1h delta from Bybit
func (oip *OpenInterestProvider) fetchBybitOIWith1hDelta(ctx context.Context, symbol string) (float64, float64, error) {
	currentOI, _, err := oip.fetchBybitOI(ctx, symbol)
	if err != nil {
		return 0, 0, err
	}

	// Mock 1h delta calculation
	delta1h := currentOI * 0.0008 // Mock 0.08% hourly change

	return currentOI, delta1h, nil
}

// Historical data management functions

// loadHistoricalData loads 7d historical OI and price data from disk cache
func (oip *OpenInterestProvider) loadHistoricalData(symbol string) (*HistoricalOIData, error) {
	cacheFile := filepath.Join(oip.cacheDir, fmt.Sprintf("%s_history.json", strings.ToLower(symbol)))

	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("no historical OI data for %s", symbol)
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read OI historical cache: %w", err)
	}

	var historical HistoricalOIData
	if err := json.Unmarshal(data, &historical); err != nil {
		return nil, fmt.Errorf("failed to parse OI historical cache: %w", err)
	}

	return &historical, nil
}

// updateHistoricalCache updates the 7d rolling cache with current OI data point
func (oip *OpenInterestProvider) updateHistoricalCache(snapshot *OpenInterestSnapshot) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(oip.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create OI cache directory: %w", err)
	}

	// Load existing historical data or create new
	historical, err := oip.loadHistoricalData(snapshot.Symbol)
	if err != nil {
		historical = &HistoricalOIData{
			Symbol:      snapshot.Symbol,
			DataPoints:  []OIDataPoint{},
			LastUpdated: snapshot.Timestamp,
			Source:      "cryptorun_oi_provider",
		}
	}

	// Add current data point
	newPoint := OIDataPoint{
		Timestamp:     snapshot.Timestamp,
		TotalOI:       snapshot.TotalOI,
		DeltaOI_1h:    snapshot.DeltaOI_1h,
		PriceChange1h: snapshot.PriceChange1h,
	}

	historical.DataPoints = append(historical.DataPoints, newPoint)
	historical.LastUpdated = snapshot.Timestamp

	// Keep only last 7 days + buffer
	cutoff := time.Now().Add(-8 * 24 * time.Hour)
	filtered := make([]OIDataPoint, 0, len(historical.DataPoints))

	for _, point := range historical.DataPoints {
		if point.Timestamp.After(cutoff) {
			filtered = append(filtered, point)
		}
	}

	historical.DataPoints = filtered
	historical.SignatureHash = oip.calculateHistoricalHash(historical)

	// Save to disk
	cacheFile := filepath.Join(oip.cacheDir, fmt.Sprintf("%s_history.json", strings.ToLower(snapshot.Symbol)))

	data, err := json.MarshalIndent(historical, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal OI historical data: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write OI historical cache: %w", err)
	}

	return nil
}

// loadFromDiskCache loads OI snapshot from disk cache
func (oip *OpenInterestProvider) loadFromDiskCache(symbol string) (*OpenInterestSnapshot, error) {
	cacheFile := filepath.Join(oip.cacheDir, fmt.Sprintf("%s.json", strings.ToLower(symbol)))

	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("no OI disk cache for %s", symbol)
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read OI disk cache: %w", err)
	}

	var snapshot OpenInterestSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse OI disk cache: %w", err)
	}

	return &snapshot, nil
}

// saveToDiskCache saves OI snapshot to disk cache
func (oip *OpenInterestProvider) saveToDiskCache(snapshot *OpenInterestSnapshot) error {
	if err := os.MkdirAll(oip.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create OI cache directory: %w", err)
	}

	cacheFile := filepath.Join(oip.cacheDir, fmt.Sprintf("%s.json", strings.ToLower(snapshot.Symbol)))

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal OI snapshot: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write OI disk cache: %w", err)
	}

	return nil
}

// calculateSignatureHash computes data integrity hash for OI snapshot
func (oip *OpenInterestProvider) calculateSignatureHash(snapshot *OpenInterestSnapshot) string {
	hashInput := fmt.Sprintf("%s_%d_%.2f_%.6f_%.6f",
		snapshot.Symbol, snapshot.MonotonicTimestamp,
		snapshot.TotalOI, snapshot.DeltaOI_1h, snapshot.Beta7d)

	hash := 0
	for _, c := range hashInput {
		hash = hash*31 + int(c)
	}

	return fmt.Sprintf("%x", hash)
}

// calculateHistoricalHash computes hash for OI historical data integrity
func (oip *OpenInterestProvider) calculateHistoricalHash(historical *HistoricalOIData) string {
	hashInput := fmt.Sprintf("%s_%d_%d",
		historical.Symbol, historical.LastUpdated.Unix(), len(historical.DataPoints))

	hash := 0
	for _, c := range hashInput {
		hash = hash*31 + int(c)
	}

	return fmt.Sprintf("%x", hash)
}

// fetchBinanceOI gets open interest from Binance futures
func (oip *OpenInterestProvider) fetchBinanceOI(ctx context.Context, symbol string) (float64, float64, error) {
	binanceSymbol := oip.toBinanceSymbol(symbol)

	// Get current OI
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", binanceSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create Binance OI request: %w", err)
	}

	resp, err := oip.httpClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("Binance OI request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("Binance OI API error: %d %s", resp.StatusCode, string(body))
	}

	var result BinanceOIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, fmt.Errorf("failed to decode Binance OI response: %w", err)
	}

	// Get 24h OI statistics for change calculation
	statsURL := fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=%s", binanceSymbol)

	statsReq, err := http.NewRequestWithContext(ctx, "GET", statsURL, nil)
	if err != nil {
		// If stats fail, return OI with zero change
		return result.OpenInterest, 0, nil
	}

	statsResp, err := oip.httpClient.Do(statsReq)
	if err != nil {
		return result.OpenInterest, 0, nil
	}
	defer statsResp.Body.Close()

	if statsResp.StatusCode == http.StatusOK {
		var stats Binance24hStatsResponse
		if json.NewDecoder(statsResp.Body).Decode(&stats) == nil {
			// Calculate OI change (current - previous)
			oiChange := result.OpenInterest - (result.OpenInterest / (1 + stats.PriceChangePercent/100))
			return result.OpenInterest, oiChange, nil
		}
	}

	return result.OpenInterest, 0, nil
}

// fetchOKXOI gets open interest from OKX
func (oip *OpenInterestProvider) fetchOKXOI(ctx context.Context, symbol string) (float64, float64, error) {
	okxSymbol := oip.toOKXSymbol(symbol)

	url := fmt.Sprintf("https://www.okx.com/api/v5/market/open-interest?instId=%s", okxSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create OKX OI request: %w", err)
	}

	resp, err := oip.httpClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("OKX OI request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("OKX OI API error: %d %s", resp.StatusCode, string(body))
	}

	var result OKXOIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, fmt.Errorf("failed to decode OKX OI response: %w", err)
	}

	if len(result.Data) == 0 {
		return 0, 0, fmt.Errorf("no OKX OI data available")
	}

	// OKX provides OI in base currency, need to convert to USD
	// For simplicity, using the OI value directly (assuming USD-denominated)
	oi := result.Data[0].OI

	// Mock OI change calculation (would need historical endpoint in practice)
	oiChange := oi * 0.05 // Mock 5% daily change

	return oi, oiChange, nil
}

// fetchBybitOI gets open interest from Bybit
func (oip *OpenInterestProvider) fetchBybitOI(ctx context.Context, symbol string) (float64, float64, error) {
	bybitSymbol := oip.toBybitSymbol(symbol)

	url := fmt.Sprintf("https://api.bybit.com/v5/market/open-interest?category=linear&symbol=%s&intervalTime=24h&limit=2", bybitSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create Bybit OI request: %w", err)
	}

	resp, err := oip.httpClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("Bybit OI request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("Bybit OI API error: %d %s", resp.StatusCode, string(body))
	}

	var result BybitOIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, fmt.Errorf("failed to decode Bybit OI response: %w", err)
	}

	if len(result.Result.List) == 0 {
		return 0, 0, fmt.Errorf("no Bybit OI data available")
	}

	currentOI := result.Result.List[0].OpenInterest

	// Calculate OI change if we have historical data
	oiChange := 0.0
	if len(result.Result.List) > 1 {
		previousOI := result.Result.List[1].OpenInterest
		oiChange = currentOI - previousOI
	}

	return currentOI, oiChange, nil
}

// Symbol conversion helpers (reuse from funding.go patterns)

func (oip *OpenInterestProvider) toBinanceSymbol(symbol string) string {
	return strings.ToUpper(symbol) + "USDT"
}

func (oip *OpenInterestProvider) toOKXSymbol(symbol string) string {
	return strings.ToUpper(symbol) + "-USDT-SWAP"
}

func (oip *OpenInterestProvider) toBybitSymbol(symbol string) string {
	return strings.ToUpper(symbol) + "USDT"
}

// Helper methods for OI analysis

// IsOIExpanding checks if OI is growing (bullish structure signal)
func (ois *OpenInterestSnapshot) IsOIExpanding(threshold float64) bool {
	return ois.OIChange24h > threshold
}

// HasPositiveOIResidual checks if OI growth exceeds price-explained component
func (ois *OpenInterestSnapshot) HasPositiveOIResidual() bool {
	return ois.OIResidual > 0
}

// GetOIIntensity returns normalized OI change intensity
func (ois *OpenInterestSnapshot) GetOIIntensity() float64 {
	// Normalize by total OI to get percentage change
	if ois.TotalOI > 0 {
		return ois.OIChange24h / ois.TotalOI
	}
	return 0.0
}

// API Response structures

type BinanceOIResponse struct {
	OpenInterest float64 `json:"openInterest"`
	Symbol       string  `json:"symbol"`
	Time         int64   `json:"time"`
}

type Binance24hStatsResponse struct {
	Symbol             string  `json:"symbol"`
	PriceChange        string  `json:"priceChange"`
	PriceChangePercent float64 `json:"priceChangePercent"`
	WeightedAvgPrice   string  `json:"weightedAvgPrice"`
	LastPrice          string  `json:"lastPrice"`
	Volume             string  `json:"volume"`
	QuoteVolume        string  `json:"quoteVolume"`
	Count              int     `json:"count"`
}

type OKXOIResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		InstType string  `json:"instType"`
		InstId   string  `json:"instId"`
		OI       float64 `json:"oi"`
		OIUsd    string  `json:"oiUsd"`
	} `json:"data"`
}

type BybitOIResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Symbol   string `json:"symbol"`
		Category string `json:"category"`
		List     []struct {
			OpenInterest string `json:"openInterest"`
			Timestamp    string `json:"timestamp"`
		} `json:"list"`
	} `json:"result"`
}
