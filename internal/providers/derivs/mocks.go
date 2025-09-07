package derivs

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

// MockDerivProvider implements DerivProvider for testing
type MockDerivProvider struct {
	venue           string
	healthy         bool
	latencyMS       float64
	errorRate       float64
	supportedPairs  []string
	mockData        map[string]*DerivMetrics
	errors          []string
	callCount       int
	shouldFail      bool
	failureReason   string
}

// NewMockDerivProvider creates a new mock derivatives provider
func NewMockDerivProvider(venue string) *MockDerivProvider {
	return &MockDerivProvider{
		venue:          venue,
		healthy:        true,
		latencyMS:      25.0,
		errorRate:      0.0,
		supportedPairs: []string{"BTCUSDT", "ETHUSDT", "ADAUSDT", "SOLUSDT"},
		mockData:       make(map[string]*DerivMetrics),
		errors:         []string{},
		callCount:      0,
	}
}

// GetLatest retrieves latest derivatives metrics for a symbol
func (m *MockDerivProvider) GetLatest(ctx context.Context, symbol string) (*DerivMetrics, error) {
	m.callCount++
	
	if m.shouldFail {
		return nil, fmt.Errorf("mock failure: %s", m.failureReason)
	}
	
	if !m.isUSDSymbol(symbol) {
		return nil, fmt.Errorf("non-USD symbol rejected: %s - USD pairs only", symbol)
	}
	
	// Generate or return cached mock data
	if cached, exists := m.mockData[symbol]; exists {
		return cached, nil
	}
	
	metrics := m.generateMockMetrics(symbol)
	m.mockData[symbol] = metrics
	
	return metrics, nil
}

// GetFundingWindow retrieves funding rate history within time range
func (m *MockDerivProvider) GetFundingWindow(ctx context.Context, symbol string, tr TimeRange) ([]DerivMetrics, error) {
	m.callCount++
	
	if m.shouldFail {
		return nil, fmt.Errorf("mock failure: %s", m.failureReason)
	}
	
	if !m.isUSDSymbol(symbol) {
		return nil, fmt.Errorf("non-USD symbol rejected: %s - USD pairs only", symbol)
	}
	
	// Generate mock historical data
	var history []DerivMetrics
	periods := int(tr.To.Sub(tr.From).Hours() / 8) // 8-hour funding periods
	if periods > 100 {
		periods = 100 // Limit to prevent excessive data
	}
	
	for i := 0; i < periods; i++ {
		timestamp := tr.To.Add(-time.Duration(i*8) * time.Hour)
		metrics := m.generateMockMetricsForTime(symbol, timestamp)
		history = append(history, *metrics)
	}
	
	return history, nil
}

// GetMultipleLatest retrieves latest metrics for multiple symbols
func (m *MockDerivProvider) GetMultipleLatest(ctx context.Context, symbols []string) (map[string]*DerivMetrics, error) {
	m.callCount++
	
	if m.shouldFail {
		return nil, fmt.Errorf("mock failure: %s", m.failureReason)
	}
	
	results := make(map[string]*DerivMetrics)
	
	for _, symbol := range symbols {
		if m.isUSDSymbol(symbol) {
			metrics, _ := m.GetLatest(ctx, symbol)
			if metrics != nil {
				results[symbol] = metrics
			}
		}
	}
	
	return results, nil
}

// CalculateFundingZScore calculates z-score for funding rates using historical data
func (m *MockDerivProvider) CalculateFundingZScore(ctx context.Context, symbol string, lookbackPeriods int) (float64, error) {
	m.callCount++
	
	if m.shouldFail {
		return 0, fmt.Errorf("mock failure: %s", m.failureReason)
	}
	
	// Generate mock z-score based on symbol characteristics
	baseZScore := rand.NormFloat64() * 1.5 // Random z-score with std dev 1.5
	
	// Add some symbol-specific behavior
	if strings.Contains(symbol, "BTC") {
		baseZScore *= 0.8 // BTC tends to have more stable funding
	} else if strings.Contains(symbol, "ETH") {
		baseZScore *= 0.9 // ETH slightly less volatile
	} else {
		baseZScore *= 1.2 // Altcoins more volatile funding
	}
	
	return math.Max(-5.0, math.Min(5.0, baseZScore)), nil
}

// GetOpenInterestHistory retrieves OI history for trend analysis
func (m *MockDerivProvider) GetOpenInterestHistory(ctx context.Context, symbol string, tr TimeRange) ([]DerivMetrics, error) {
	m.callCount++
	
	if m.shouldFail {
		return nil, fmt.Errorf("mock failure: %s", m.failureReason)
	}
	
	// For mock, return single latest point
	latest, err := m.GetLatest(ctx, symbol)
	if err != nil {
		return nil, err
	}
	
	return []DerivMetrics{*latest}, nil
}

// Health returns provider health and connectivity status
func (m *MockDerivProvider) Health(ctx context.Context) (*ProviderHealth, error) {
	m.callCount++
	
	health := &ProviderHealth{
		Healthy:          m.healthy,
		Venue:            m.venue,
		LastUpdate:       time.Now(),
		LatencyMS:        m.latencyMS,
		ErrorRate:        m.errorRate,
		SupportedSymbols: len(m.supportedPairs),
		DataFreshness:    make(map[string]time.Duration),
		Errors:           m.errors,
	}
	
	// Mock data freshness
	for _, symbol := range m.supportedPairs {
		health.DataFreshness[symbol] = time.Duration(rand.Intn(30)) * time.Second
	}
	
	return health, nil
}

// GetSupportedSymbols returns list of supported derivative symbols (USD pairs only)
func (m *MockDerivProvider) GetSupportedSymbols(ctx context.Context) ([]string, error) {
	m.callCount++
	
	if m.shouldFail {
		return nil, fmt.Errorf("mock failure: %s", m.failureReason)
	}
	
	return m.supportedPairs, nil
}

// Mock configuration methods

// SetHealthy sets the health status of the mock provider
func (m *MockDerivProvider) SetHealthy(healthy bool) {
	m.healthy = healthy
}

// SetLatency sets the mock latency in milliseconds
func (m *MockDerivProvider) SetLatency(latencyMS float64) {
	m.latencyMS = latencyMS
}

// SetErrorRate sets the mock error rate (0.0-1.0)
func (m *MockDerivProvider) SetErrorRate(errorRate float64) {
	m.errorRate = errorRate
}

// AddError adds an error to the mock provider
func (m *MockDerivProvider) AddError(error string) {
	m.errors = append(m.errors, error)
}

// SetShouldFail configures the mock to fail with a specific reason
func (m *MockDerivProvider) SetShouldFail(shouldFail bool, reason string) {
	m.shouldFail = shouldFail
	m.failureReason = reason
}

// GetCallCount returns the number of calls made to the mock provider
func (m *MockDerivProvider) GetCallCount() int {
	return m.callCount
}

// ResetCallCount resets the call counter
func (m *MockDerivProvider) ResetCallCount() {
	m.callCount = 0
}

// Helper methods

func (m *MockDerivProvider) isUSDSymbol(symbol string) bool {
	upperSymbol := strings.ToUpper(symbol)
	return strings.HasSuffix(upperSymbol, "USDT") ||
		   strings.HasSuffix(upperSymbol, "USDC") ||
		   strings.HasSuffix(upperSymbol, "USD")
}

func (m *MockDerivProvider) generateMockMetrics(symbol string) *DerivMetrics {
	return m.generateMockMetricsForTime(symbol, time.Now())
}

func (m *MockDerivProvider) generateMockMetricsForTime(symbol string, timestamp time.Time) *DerivMetrics {
	// Generate realistic mock data based on symbol
	basePrice := m.getBasePriceForSymbol(symbol)
	
	// Add some realistic variation
	priceVariation := 1.0 + (rand.Float64()-0.5)*0.02 // ±1% variation
	markPrice := basePrice * priceVariation
	indexPrice := basePrice * (1.0 + (rand.Float64()-0.5)*0.001) // Smaller index variation
	
	// Generate funding rate (-0.1% to +0.1% typical range)
	fundingRate := (rand.Float64() - 0.5) * 0.002
	
	// Generate OI based on symbol popularity
	baseOI := m.getBaseOIForSymbol(symbol)
	oi := baseOI * (0.8 + rand.Float64()*0.4) // ±20% variation
	
	return &DerivMetrics{
		Timestamp:        timestamp,
		Symbol:           symbol,
		Venue:            m.venue,
		DataSource:       fmt.Sprintf("%s_mock", m.venue),
		ConfidenceScore:  0.95, // High confidence for mocks
		PITShift:         1,
		
		// Funding data
		Funding:          fundingRate,
		FundingZScore:    rand.NormFloat64() * 1.5, // Random z-score
		NextFundingTime:  timestamp.Add(8 * time.Hour),
		
		// OI data
		OpenInterest:     oi,
		OpenInterestUSD:  oi * markPrice,
		OIResidual:       oi * 0.1, // 10% residual
		
		// Price data
		MarkPrice:        markPrice,
		IndexPrice:       indexPrice,
		LastPrice:        markPrice * (1.0 + (rand.Float64()-0.5)*0.001),
		
		// Basis
		Basis:            (markPrice - indexPrice) / indexPrice,
		BasisPercent:     ((markPrice - indexPrice) / indexPrice) * 100,
		
		// Volume
		Volume24h:        rand.Float64() * 1000000,
		VolumeUSD24h:     rand.Float64() * 50000000,
		VolumeRatio:      0.8 + rand.Float64()*0.4, // 0.8-1.2x
	}
}

func (m *MockDerivProvider) getBasePriceForSymbol(symbol string) float64 {
	upperSymbol := strings.ToUpper(symbol)
	
	switch {
	case strings.HasPrefix(upperSymbol, "BTC"):
		return 45000.0
	case strings.HasPrefix(upperSymbol, "ETH"):
		return 3000.0
	case strings.HasPrefix(upperSymbol, "ADA"):
		return 0.45
	case strings.HasPrefix(upperSymbol, "SOL"):
		return 85.0
	default:
		return 1.0 // Default for unknown symbols
	}
}

func (m *MockDerivProvider) getBaseOIForSymbol(symbol string) float64 {
	upperSymbol := strings.ToUpper(symbol)
	
	switch {
	case strings.HasPrefix(upperSymbol, "BTC"):
		return 50000.0 // 50k contracts
	case strings.HasPrefix(upperSymbol, "ETH"):
		return 100000.0 // 100k contracts
	case strings.HasPrefix(upperSymbol, "ADA"):
		return 200000.0 // 200k contracts
	case strings.HasPrefix(upperSymbol, "SOL"):
		return 75000.0 // 75k contracts
	default:
		return 10000.0 // Default
	}
}

// MockErrorScenarios provides pre-configured error scenarios for testing
func MockErrorScenarios() map[string]*MockDerivProvider {
	scenarios := make(map[string]*MockDerivProvider)
	
	// Network timeout
	timeoutProvider := NewMockDerivProvider("timeout_test")
	timeoutProvider.SetShouldFail(true, "network timeout")
	scenarios["timeout"] = timeoutProvider
	
	// Rate limited
	rateLimitProvider := NewMockDerivProvider("ratelimit_test")
	rateLimitProvider.SetShouldFail(true, "rate limited")
	scenarios["rate_limit"] = rateLimitProvider
	
	// API error
	apiErrorProvider := NewMockDerivProvider("apierror_test")
	apiErrorProvider.SetShouldFail(true, "API returned error 500")
	scenarios["api_error"] = apiErrorProvider
	
	// Unhealthy provider
	unhealthyProvider := NewMockDerivProvider("unhealthy_test")
	unhealthyProvider.SetHealthy(false)
	unhealthyProvider.SetErrorRate(0.8)
	unhealthyProvider.AddError("Connection unstable")
	scenarios["unhealthy"] = unhealthyProvider
	
	// High latency provider
	slowProvider := NewMockDerivProvider("slow_test")
	slowProvider.SetLatency(5000.0) // 5 second latency
	scenarios["high_latency"] = slowProvider
	
	return scenarios
}

// RateLimiter mock interface
type RateLimiter interface {
	Wait(ctx context.Context) error
}

// MockRateLimiter implements RateLimiter for testing
type MockRateLimiter struct {
	waitTime     time.Duration
	shouldBlock  bool
	callCount    int
}

// NewTokenBucketLimiter creates a new mock rate limiter
func NewTokenBucketLimiter(rps float64) RateLimiter {
	return &MockRateLimiter{
		waitTime: time.Duration(1.0/rps) * time.Second,
	}
}

func (m *MockRateLimiter) Wait(ctx context.Context) error {
	m.callCount++
	
	if m.shouldBlock {
		select {
		case <-time.After(m.waitTime):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	return nil
}

func (m *MockRateLimiter) SetShouldBlock(shouldBlock bool) {
	m.shouldBlock = shouldBlock
}

func (m *MockRateLimiter) GetCallCount() int {
	return m.callCount
}