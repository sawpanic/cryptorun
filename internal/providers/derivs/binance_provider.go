package derivs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BinanceDerivProvider implements DerivProvider for Binance derivatives
type BinanceDerivProvider struct {
	config         DerivProviderConfig
	httpClient     *http.Client
	rateLimiter    RateLimiter
	metrics        MetricsCallback
	mu             sync.RWMutex
	lastUpdate     time.Time
	symbolCache    map[string]bool
	cacheExpiry    time.Time
}

// NewBinanceDerivProvider creates a new Binance derivatives provider
func NewBinanceDerivProvider(config DerivProviderConfig) *BinanceDerivProvider {
	// Set defaults
	if config.BaseURL == "" {
		config.BaseURL = "https://fapi.binance.com" // Futures API
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 10 * time.Second
	}
	if config.RateLimitRPS == 0 {
		config.RateLimitRPS = 10.0 // Binance futures: ~1200/min = 20/sec, be conservative
	}
	if config.PITShiftPeriods == 0 {
		config.PITShiftPeriods = 1 // Default PIT shift
	}
	if config.UserAgent == "" {
		config.UserAgent = "CryptoRun/3.2.1 (Exchange-Native Derivatives)"
	}

	return &BinanceDerivProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
		rateLimiter: NewTokenBucketLimiter(config.RateLimitRPS),
		symbolCache: make(map[string]bool),
	}
}

// SetMetricsCallback sets the metrics collection callback
func (b *BinanceDerivProvider) SetMetricsCallback(callback MetricsCallback) {
	b.metrics = callback
}

// GetLatest retrieves latest derivatives metrics for a symbol
func (b *BinanceDerivProvider) GetLatest(ctx context.Context, symbol string) (*DerivMetrics, error) {
	if !b.isUSDSymbol(symbol) {
		return nil, fmt.Errorf("non-USD symbol rejected: %s - USD pairs only", symbol)
	}

	start := time.Now()
	defer func() {
		if b.metrics != nil {
			b.metrics("binance_deriv_request_duration_ms", 
				float64(time.Since(start).Milliseconds()),
				map[string]string{"venue": "binance", "symbol": symbol, "endpoint": "latest"})
		}
	}()

	// Get funding rate, OI, and price data in parallel
	var (
		fundingData *BinanceFundingInfo
		oiData      *BinanceOIData
		priceData   *BinancePriceData
		err         error
	)

	// Use goroutines for parallel requests (within rate limits)
	errChan := make(chan error, 3)
	
	go func() {
		fundingData, err = b.getFundingInfo(ctx, symbol)
		errChan <- err
	}()
	
	go func() {
		oiData, err = b.getOpenInterest(ctx, symbol)
		errChan <- err
	}()
	
	go func() {
		priceData, err = b.getPriceData(ctx, symbol)
		errChan <- err
	}()

	// Collect results
	var errors []error
	for i := 0; i < 3; i++ {
		if err := <-errChan; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("failed to get derivatives data: %v", errors)
	}

	// Calculate derived metrics
	metrics := &DerivMetrics{
		Timestamp:       time.Now().Add(-time.Duration(b.config.PITShiftPeriods) * time.Second), // PIT shift
		Symbol:          symbol,
		Venue:           "binance",
		DataSource:      "binance_futures_api",
		ConfidenceScore: 0.9, // High confidence for exchange-native data
		PITShift:        b.config.PITShiftPeriods,
	}

	// Funding data
	if fundingData != nil {
		metrics.Funding = fundingData.FundingRate
		metrics.NextFundingTime = time.Unix(fundingData.FundingTime/1000, 0)
		
		// Calculate funding z-score (requires historical data)
		if zScore, err := b.calculateFundingZScore(ctx, symbol, 30); err == nil {
			metrics.FundingZScore = zScore
		}
	}

	// Open Interest data
	if oiData != nil {
		metrics.OpenInterest = oiData.OpenInterest
		metrics.OpenInterestUSD = oiData.OpenInterestUSD
		
		// Calculate OI residual (simplified - would need trend analysis in full implementation)
		metrics.OIResidual = b.calculateOIResidual(oiData.OpenInterest)
	}

	// Price data
	if priceData != nil {
		metrics.MarkPrice = priceData.MarkPrice
		metrics.IndexPrice = priceData.IndexPrice
		metrics.LastPrice = priceData.LastPrice
		metrics.Volume24h = priceData.Volume
		metrics.VolumeUSD24h = priceData.QuoteVolume

		// Calculate basis (futures premium)
		if priceData.IndexPrice > 0 {
			metrics.Basis = (priceData.MarkPrice - priceData.IndexPrice) / priceData.IndexPrice
			metrics.BasisPercent = metrics.Basis * 100
		}
	}

	b.updateMetrics(metrics)
	return metrics, nil
}

// GetFundingWindow retrieves funding rate history within time range
func (b *BinanceDerivProvider) GetFundingWindow(ctx context.Context, symbol string, tr TimeRange) ([]DerivMetrics, error) {
	if !b.isUSDSymbol(symbol) {
		return nil, fmt.Errorf("non-USD symbol rejected: %s - USD pairs only", symbol)
	}

	start := time.Now()
	defer func() {
		if b.metrics != nil {
			b.metrics("binance_deriv_request_duration_ms", 
				float64(time.Since(start).Milliseconds()),
				map[string]string{"venue": "binance", "symbol": symbol, "endpoint": "funding_window"})
		}
	}()

	// Get funding rate history from Binance
	fundingHistory, err := b.getFundingRateHistory(ctx, symbol, tr)
	if err != nil {
		return nil, fmt.Errorf("failed to get funding history: %w", err)
	}

	var metrics []DerivMetrics
	for _, funding := range fundingHistory {
		metric := DerivMetrics{
			Timestamp:       time.Unix(funding.FundingTime/1000, 0).Add(-time.Duration(b.config.PITShiftPeriods) * time.Second),
			Symbol:          symbol,
			Venue:           "binance",
			Funding:         funding.FundingRate,
			DataSource:      "binance_futures_api",
			ConfidenceScore: 0.9,
			PITShift:        b.config.PITShiftPeriods,
		}
		
		metrics = append(metrics, metric)
	}

	// Apply PIT ordering (reverse chronological)
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Timestamp.After(metrics[j].Timestamp)
	})

	return metrics, nil
}

// GetMultipleLatest retrieves latest metrics for multiple symbols
func (b *BinanceDerivProvider) GetMultipleLatest(ctx context.Context, symbols []string) (map[string]*DerivMetrics, error) {
	results := make(map[string]*DerivMetrics)
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	// Limit concurrency to respect rate limits
	semaphore := make(chan struct{}, 5)
	
	for _, symbol := range symbols {
		if !b.isUSDSymbol(symbol) {
			continue // Skip non-USD symbols
		}
		
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release
			
			if metric, err := b.GetLatest(ctx, sym); err == nil {
				mu.Lock()
				results[sym] = metric
				mu.Unlock()
			}
		}(symbol)
	}
	
	wg.Wait()
	return results, nil
}

// CalculateFundingZScore calculates z-score for funding rates using historical data
func (b *BinanceDerivProvider) CalculateFundingZScore(ctx context.Context, symbol string, lookbackPeriods int) (float64, error) {
	return b.calculateFundingZScore(ctx, symbol, lookbackPeriods)
}

// GetOpenInterestHistory retrieves OI history for trend analysis
func (b *BinanceDerivProvider) GetOpenInterestHistory(ctx context.Context, symbol string, tr TimeRange) ([]DerivMetrics, error) {
	if !b.isUSDSymbol(symbol) {
		return nil, fmt.Errorf("non-USD symbol rejected: %s - USD pairs only", symbol)
	}

	// For this implementation, we'll return current OI as single point
	// Full implementation would use /fapi/v1/openInterestHist endpoint
	latest, err := b.GetLatest(ctx, symbol)
	if err != nil {
		return nil, err
	}

	return []DerivMetrics{*latest}, nil
}

// Health returns provider health and connectivity status
func (b *BinanceDerivProvider) Health(ctx context.Context) (*ProviderHealth, error) {
	start := time.Now()
	
	// Test connectivity with server time request
	_, err := b.makeRequest(ctx, "/fapi/v1/time")
	latency := time.Since(start).Seconds() * 1000 // Convert to milliseconds

	health := &ProviderHealth{
		Venue:      "binance",
		LastUpdate: time.Now(),
		LatencyMS:  latency,
	}

	if err != nil {
		health.Healthy = false
		health.Errors = []string{fmt.Sprintf("Connectivity test failed: %v", err)}
		health.ErrorRate = 1.0
	} else {
		health.Healthy = true
		health.ErrorRate = 0.0
	}

	// Get supported symbols count
	if symbols, err := b.GetSupportedSymbols(ctx); err == nil {
		health.SupportedSymbols = len(symbols)
	}

	return health, nil
}

// GetSupportedSymbols returns list of supported derivative symbols (USD pairs only)
func (b *BinanceDerivProvider) GetSupportedSymbols(ctx context.Context) ([]string, error) {
	// Check cache first
	b.mu.RLock()
	if time.Now().Before(b.cacheExpiry) && len(b.symbolCache) > 0 {
		symbols := make([]string, 0, len(b.symbolCache))
		for symbol := range b.symbolCache {
			symbols = append(symbols, symbol)
		}
		b.mu.RUnlock()
		return symbols, nil
	}
	b.mu.RUnlock()

	// Fetch from API
	response, err := b.makeRequest(ctx, "/fapi/v1/exchangeInfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange info: %w", err)
	}

	var exchangeInfo BinanceExchangeInfo
	if err := json.Unmarshal(response, &exchangeInfo); err != nil {
		return nil, fmt.Errorf("failed to parse exchange info: %w", err)
	}

	// Filter for USD symbols only
	var usdSymbols []string
	symbolMap := make(map[string]bool)
	
	for _, symbol := range exchangeInfo.Symbols {
		if symbol.Status == "TRADING" && b.isUSDSymbol(symbol.Symbol) {
			usdSymbols = append(usdSymbols, symbol.Symbol)
			symbolMap[symbol.Symbol] = true
		}
	}

	// Update cache
	b.mu.Lock()
	b.symbolCache = symbolMap
	b.cacheExpiry = time.Now().Add(1 * time.Hour) // Cache for 1 hour
	b.mu.Unlock()

	return usdSymbols, nil
}

// Helper methods

func (b *BinanceDerivProvider) makeRequest(ctx context.Context, endpoint string) ([]byte, error) {
	if err := b.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	url := b.config.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", b.config.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

func (b *BinanceDerivProvider) getFundingInfo(ctx context.Context, symbol string) (*BinanceFundingInfo, error) {
	endpoint := fmt.Sprintf("/fapi/v1/premiumIndex?symbol=%s", strings.ToUpper(symbol))
	response, err := b.makeRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var fundingInfo BinanceFundingInfo
	if err := json.Unmarshal(response, &fundingInfo); err != nil {
		return nil, fmt.Errorf("failed to parse funding info: %w", err)
	}

	return &fundingInfo, nil
}

func (b *BinanceDerivProvider) getOpenInterest(ctx context.Context, symbol string) (*BinanceOIData, error) {
	endpoint := fmt.Sprintf("/fapi/v1/openInterest?symbol=%s", strings.ToUpper(symbol))
	response, err := b.makeRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var rawOI struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}
	
	if err := json.Unmarshal(response, &rawOI); err != nil {
		return nil, fmt.Errorf("failed to parse OI data: %w", err)
	}

	oi, err := strconv.ParseFloat(rawOI.OpenInterest, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OI value: %w", err)
	}

	// Get mark price for USD conversion
	priceData, err := b.getPriceData(ctx, symbol)
	if err != nil {
		return nil, err
	}

	return &BinanceOIData{
		OpenInterest:    oi,
		OpenInterestUSD: oi * priceData.MarkPrice,
		Symbol:          rawOI.Symbol,
		Time:            rawOI.Time,
	}, nil
}

func (b *BinanceDerivProvider) getPriceData(ctx context.Context, symbol string) (*BinancePriceData, error) {
	endpoint := fmt.Sprintf("/fapi/v1/ticker/24hr?symbol=%s", strings.ToUpper(symbol))
	response, err := b.makeRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var ticker struct {
		Symbol             string `json:"symbol"`
		LastPrice          string `json:"lastPrice"`
		MarkPrice          string `json:"markPrice"`
		IndexPrice         string `json:"indexPrice"`
		Volume             string `json:"volume"`
		QuoteVolume        string `json:"quoteVolume"`
	}

	if err := json.Unmarshal(response, &ticker); err != nil {
		return nil, fmt.Errorf("failed to parse price data: %w", err)
	}

	lastPrice, _ := strconv.ParseFloat(ticker.LastPrice, 64)
	markPrice, _ := strconv.ParseFloat(ticker.MarkPrice, 64)
	indexPrice, _ := strconv.ParseFloat(ticker.IndexPrice, 64)
	volume, _ := strconv.ParseFloat(ticker.Volume, 64)
	quoteVolume, _ := strconv.ParseFloat(ticker.QuoteVolume, 64)

	return &BinancePriceData{
		Symbol:      ticker.Symbol,
		LastPrice:   lastPrice,
		MarkPrice:   markPrice,
		IndexPrice:  indexPrice,
		Volume:      volume,
		QuoteVolume: quoteVolume,
	}, nil
}

func (b *BinanceDerivProvider) getFundingRateHistory(ctx context.Context, symbol string, tr TimeRange) ([]BinanceFundingInfo, error) {
	endpoint := fmt.Sprintf("/fapi/v1/fundingRate?symbol=%s&startTime=%d&endTime=%d&limit=1000",
		strings.ToUpper(symbol),
		tr.From.UnixNano()/1000000, // Convert to milliseconds
		tr.To.UnixNano()/1000000,
	)

	response, err := b.makeRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var fundingHistory []BinanceFundingInfo
	if err := json.Unmarshal(response, &fundingHistory); err != nil {
		return nil, fmt.Errorf("failed to parse funding history: %w", err)
	}

	return fundingHistory, nil
}

func (b *BinanceDerivProvider) calculateFundingZScore(ctx context.Context, symbol string, lookbackPeriods int) (float64, error) {
	// Get historical funding data (30 periods back)
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(lookbackPeriods*8) * time.Hour) // 8h funding periods
	
	tr := TimeRange{From: startTime, To: endTime}
	history, err := b.GetFundingWindow(ctx, symbol, tr)
	if err != nil {
		return 0, fmt.Errorf("failed to get funding history for z-score: %w", err)
	}

	if len(history) < 10 {
		return 0, fmt.Errorf("insufficient historical data for z-score calculation")
	}

	// Extract funding rates
	var rates []float64
	for _, h := range history[1:] { // Exclude current period
		rates = append(rates, h.Funding)
	}

	// Calculate mean and standard deviation
	var sum float64
	for _, rate := range rates {
		sum += rate
	}
	mean := sum / float64(len(rates))

	var varianceSum float64
	for _, rate := range rates {
		diff := rate - mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(rates))
	stdDev := math.Sqrt(variance)

	if stdDev == 0 {
		return 0, nil // No variance, z-score is 0
	}

	// Calculate z-score for current funding rate
	currentRate := history[0].Funding
	zScore := (currentRate - mean) / stdDev

	return zScore, nil
}

func (b *BinanceDerivProvider) calculateOIResidual(currentOI float64) float64 {
	// Simplified OI residual calculation
	// Full implementation would use trend analysis and seasonal adjustment
	return currentOI * 0.1 // Placeholder: 10% of current OI as residual
}

func (b *BinanceDerivProvider) isUSDSymbol(symbol string) bool {
	upperSymbol := strings.ToUpper(symbol)
	return strings.HasSuffix(upperSymbol, "USDT") ||
		   strings.HasSuffix(upperSymbol, "USDC") ||
		   strings.HasSuffix(upperSymbol, "USD")
}

func (b *BinanceDerivProvider) updateMetrics(metrics *DerivMetrics) {
	b.mu.Lock()
	b.lastUpdate = time.Now()
	b.mu.Unlock()

	if b.metrics != nil {
		tags := map[string]string{
			"venue":  "binance",
			"symbol": metrics.Symbol,
		}
		
		b.metrics("binance_deriv_funding_rate", metrics.Funding, tags)
		b.metrics("binance_deriv_funding_z_score", metrics.FundingZScore, tags)
		b.metrics("binance_deriv_open_interest_usd", metrics.OpenInterestUSD, tags)
		b.metrics("binance_deriv_basis_bps", metrics.Basis*10000, tags)
		b.metrics("binance_deriv_confidence_score", metrics.ConfidenceScore, tags)
	}
}

// Data structures for Binance API responses

type BinanceFundingInfo struct {
	Symbol           string  `json:"symbol"`
	MarkPrice        string  `json:"markPrice"`
	IndexPrice       string  `json:"indexPrice"`
	EstimatedSettle  string  `json:"estimatedSettlePrice"`
	LastFundingRate  string  `json:"lastFundingRate"`
	InterestRate     string  `json:"interestRate"`
	NextFundingTime  int64   `json:"nextFundingTime"`
	Time             int64   `json:"time"`
	
	// Parsed values
	FundingRate      float64 `json:"-"`
	FundingTime      int64   `json:"-"`
}

// UnmarshalJSON implements custom unmarshaling for BinanceFundingInfo
func (b *BinanceFundingInfo) UnmarshalJSON(data []byte) error {
	type Alias BinanceFundingInfo
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(b),
	}
	
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	
	// Parse string values to floats
	if rate, err := strconv.ParseFloat(b.LastFundingRate, 64); err == nil {
		b.FundingRate = rate
	}
	
	b.FundingTime = b.NextFundingTime
	
	return nil
}

type BinanceOIData struct {
	OpenInterest    float64 `json:"open_interest"`
	OpenInterestUSD float64 `json:"open_interest_usd"`
	Symbol          string  `json:"symbol"`
	Time            int64   `json:"time"`
}

type BinancePriceData struct {
	Symbol      string  `json:"symbol"`
	LastPrice   float64 `json:"last_price"`
	MarkPrice   float64 `json:"mark_price"`
	IndexPrice  float64 `json:"index_price"`
	Volume      float64 `json:"volume"`
	QuoteVolume float64 `json:"quote_volume"`
}

type BinanceExchangeInfo struct {
	Symbols []struct {
		Symbol string `json:"symbol"`
		Status string `json:"status"`
	} `json:"symbols"`
}