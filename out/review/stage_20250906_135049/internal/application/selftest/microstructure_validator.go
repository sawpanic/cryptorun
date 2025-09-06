package selftest

import (
	"fmt"
	"math"
	"time"
)

// MicrostructureValidator validates microstructure requirements
type MicrostructureValidator struct{}

// NewMicrostructureValidator creates a new microstructure validator
func NewMicrostructureValidator() *MicrostructureValidator {
	return &MicrostructureValidator{}
}

// Name returns the validator name
func (mv *MicrostructureValidator) Name() string {
	return "Microstructure Validation"
}

// MicrostructureTestCase represents a test case for microstructure validation
type MicrostructureTestCase struct {
	Name           string
	Symbol         string
	Exchange       string
	BidPrice       float64
	AskPrice       float64
	BidSize        float64
	AskSize        float64
	LastPrice      float64
	Volume24h      float64
	VolumeWindow   float64 // Volume in measurement window
	WindowDuration time.Duration
	ExpectedResult MicrostructureResult
}

// MicrostructureResult holds expected validation results
type MicrostructureResult struct {
	SpreadPass   bool    // Spread < 50bps
	DepthPass    bool    // Depth ≥ $100k within ±2%
	VADRPass     bool    // VADR ≥ 1.75×
	OverallPass  bool    // All checks pass
	SpreadBps    float64 // Calculated spread in bps
	DepthUSD     float64 // Calculated depth in USD
	VADRValue    float64 // Calculated VADR
}

// Validate tests microstructure requirements
func (mv *MicrostructureValidator) Validate() TestResult {
	start := time.Now()
	result := TestResult{
		Name:      mv.Name(),
		Timestamp: start,
		Details:   []string{},
	}
	
	// Create test cases
	testCases := mv.createTestCases()
	result.Details = append(result.Details, fmt.Sprintf("Created %d microstructure test cases", len(testCases)))
	
	passedTests := 0
	totalTests := len(testCases)
	
	// Test each case
	for _, testCase := range testCases {
		actualResult := mv.calculateMicrostructure(testCase)
		passed := mv.compareResults(testCase.ExpectedResult, actualResult)
		
		if passed {
			passedTests++
			result.Details = append(result.Details, fmt.Sprintf("✅ %s: PASS", testCase.Name))
		} else {
			result.Details = append(result.Details, fmt.Sprintf("❌ %s: FAIL", testCase.Name))
		}
		
		// Add detailed results
		result.Details = append(result.Details, fmt.Sprintf("   Spread: %.1f bps (pass=%t)", actualResult.SpreadBps, actualResult.SpreadPass))
		result.Details = append(result.Details, fmt.Sprintf("   Depth: $%.0f (pass=%t)", actualResult.DepthUSD, actualResult.DepthPass))
		result.Details = append(result.Details, fmt.Sprintf("   VADR: %.2f× (pass=%t)", actualResult.VADRValue, actualResult.VADRPass))
	}
	
	// Test aggregator detection
	aggregatorTests := mv.testAggregatorDetection()
	result.Details = append(result.Details, "")
	result.Details = append(result.Details, "Aggregator Detection Tests:")
	
	for testName, passed := range aggregatorTests {
		if passed {
			result.Details = append(result.Details, fmt.Sprintf("✅ %s: PASS", testName))
		} else {
			result.Details = append(result.Details, fmt.Sprintf("❌ %s: FAIL", testName))
			passedTests-- // Adjust for failed aggregator tests
		}
		totalTests++
	}
	
	// Overall result
	if passedTests == totalTests {
		result.Status = "PASS"
		result.Message = fmt.Sprintf("All %d microstructure validation tests passed", totalTests)
	} else {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Microstructure validation failed: %d/%d tests passed", passedTests, totalTests)
	}
	
	result.Duration = time.Since(start)
	return result
}

// calculateMicrostructure performs actual microstructure calculations
func (mv *MicrostructureValidator) calculateMicrostructure(testCase MicrostructureTestCase) MicrostructureResult {
	result := MicrostructureResult{}
	
	// Calculate spread in basis points
	if testCase.BidPrice > 0 && testCase.AskPrice > 0 {
		spread := testCase.AskPrice - testCase.BidPrice
		midPrice := (testCase.BidPrice + testCase.AskPrice) / 2.0
		result.SpreadBps = (spread / midPrice) * 10000.0
		result.SpreadPass = result.SpreadBps < 50.0 // < 50bps
	}
	
	// Calculate depth in USD (±2% of mid price)
	if testCase.BidPrice > 0 && testCase.AskSize > 0 {
		_ = (testCase.BidPrice + testCase.AskPrice) / 2.0 // midPrice
		
		// For simplicity, assume bid/ask sizes represent depth within ±2%
		// In practice, this would sum order book entries within the range
		bidDepth := testCase.BidSize * testCase.BidPrice
		askDepth := testCase.AskSize * testCase.AskPrice
		result.DepthUSD = math.Min(bidDepth, askDepth)
		result.DepthPass = result.DepthUSD >= 100000.0 // ≥ $100k
	}
	
	// Calculate VADR (Volume-Adjusted Daily Range)
	if testCase.Volume24h > 0 && testCase.VolumeWindow > 0 && testCase.WindowDuration > 0 {
		// VADR = (Volume in window / Volume per time unit) / Average ratio
		hoursInDay := 24.0
		windowHours := testCase.WindowDuration.Hours()
		
		avgVolumePerHour := testCase.Volume24h / hoursInDay
		expectedVolumeInWindow := avgVolumePerHour * windowHours
		
		if expectedVolumeInWindow > 0 {
			result.VADRValue = testCase.VolumeWindow / expectedVolumeInWindow
			result.VADRPass = result.VADRValue >= 1.75 // ≥ 1.75×
		}
	}
	
	// Overall pass
	result.OverallPass = result.SpreadPass && result.DepthPass && result.VADRPass
	
	return result
}

// compareResults compares expected vs actual microstructure results
func (mv *MicrostructureValidator) compareResults(expected, actual MicrostructureResult) bool {
	tolerance := 0.01 // 1% tolerance for floating point comparisons
	
	spreadMatch := expected.SpreadPass == actual.SpreadPass
	depthMatch := expected.DepthPass == actual.DepthPass
	vadrMatch := expected.VADRPass == actual.VADRPass
	
	// Also check values are within tolerance
	if expected.SpreadBps > 0 {
		spreadMatch = spreadMatch && math.Abs(expected.SpreadBps-actual.SpreadBps) < expected.SpreadBps*tolerance
	}
	
	if expected.DepthUSD > 0 {
		depthMatch = depthMatch && math.Abs(expected.DepthUSD-actual.DepthUSD) < expected.DepthUSD*tolerance
	}
	
	if expected.VADRValue > 0 {
		vadrMatch = vadrMatch && math.Abs(expected.VADRValue-actual.VADRValue) < expected.VADRValue*tolerance
	}
	
	return spreadMatch && depthMatch && vadrMatch
}

// testAggregatorDetection tests detection of banned aggregator usage
func (mv *MicrostructureValidator) testAggregatorDetection() map[string]bool {
	results := make(map[string]bool)
	
	// Test 1: Detect banned aggregator patterns
	bannedAggregators := []string{
		"dexscreener",
		"coingecko",
		"coinmarketcap",
		"1inch",
		"0x",
		"jupiter",
		"uniswap",
	}
	
	allowedExchanges := []string{
		"kraken",
		"binance",
		"coinbase",
		"okx",
	}
	
	// Test banned aggregator detection
	for _, aggregator := range bannedAggregators {
		detected := mv.isAggregatorBanned(aggregator)
		results[fmt.Sprintf("Detect banned aggregator: %s", aggregator)] = detected
	}
	
	// Test allowed exchange detection
	for _, exchange := range allowedExchanges {
		notDetected := !mv.isAggregatorBanned(exchange)
		results[fmt.Sprintf("Allow native exchange: %s", exchange)] = notDetected
	}
	
	return results
}

// isAggregatorBanned checks if a source is a banned aggregator
func (mv *MicrostructureValidator) isAggregatorBanned(source string) bool {
	bannedSources := map[string]bool{
		"dexscreener":   true,
		"coingecko":     true,
		"coinmarketcap": true,
		"1inch":         true,
		"0x":            true,
		"jupiter":       true,
		"uniswap":       true,
		"pancakeswap":   true,
		"sushiswap":     true,
	}
	
	return bannedSources[source]
}

// createTestCases creates comprehensive test cases for microstructure validation
func (mv *MicrostructureValidator) createTestCases() []MicrostructureTestCase {
	return []MicrostructureTestCase{
		{
			Name:           "BTC/USD - Excellent Liquidity",
			Symbol:         "BTC/USD",
			Exchange:       "kraken",
			BidPrice:       50000.0,
			AskPrice:       50002.0, // 4bps spread
			BidSize:        5.0,     // $250k depth
			AskSize:        5.0,     // $250k depth
			LastPrice:      50001.0,
			Volume24h:      1000000.0,
			VolumeWindow:   100000.0,
			WindowDuration: 1 * time.Hour,
			ExpectedResult: MicrostructureResult{
				SpreadPass:   true,    // 4bps < 50bps
				DepthPass:    true,    // $250k > $100k
				VADRPass:     true,    // 2.4× > 1.75×
				OverallPass:  true,
				SpreadBps:    4.0,
				DepthUSD:     250000.0,
				VADRValue:    2.4,
			},
		},
		{
			Name:           "ETH/USD - Marginal Liquidity",
			Symbol:         "ETH/USD",
			Exchange:       "binance",
			BidPrice:       3000.0,
			AskPrice:       3001.5, // 50bps spread (at limit)
			BidSize:        35.0,   // $105k depth
			AskSize:        33.0,   // $99k depth (below threshold)
			LastPrice:      3000.75,
			Volume24h:      500000.0,
			VolumeWindow:   22000.0,
			WindowDuration: 1 * time.Hour,
			ExpectedResult: MicrostructureResult{
				SpreadPass:   true,    // 50bps = 50bps (at limit)
				DepthPass:    false,   // $99k < $100k
				VADRPass:     true,    // 1.85× > 1.75×
				OverallPass:  false,
				SpreadBps:    50.0,
				DepthUSD:     99000.0,
				VADRValue:    1.85,
			},
		},
		{
			Name:           "SOL/USD - Poor Liquidity",
			Symbol:         "SOL/USD",
			Exchange:       "coinbase",
			BidPrice:       100.0,
			AskPrice:       100.8,  // 80bps spread (too wide)
			BidSize:        500.0,  // $50k depth (too low)
			AskSize:        500.0,  // $50k depth (too low)
			LastPrice:      100.4,
			Volume24h:      100000.0,
			VolumeWindow:   3000.0,
			WindowDuration: 1 * time.Hour,
			ExpectedResult: MicrostructureResult{
				SpreadPass:   false,   // 80bps > 50bps
				DepthPass:    false,   // $50k < $100k
				VADRPass:     false,   // 0.72× < 1.75×
				OverallPass:  false,
				SpreadBps:    80.0,
				DepthUSD:     50000.0,
				VADRValue:    0.72,
			},
		},
		{
			Name:           "ADA/USD - High Volume Low Price",
			Symbol:         "ADA/USD",
			Exchange:       "okx",
			BidPrice:       0.45,
			AskPrice:       0.4502, // 4.4bps spread
			BidSize:        250000.0, // $112.5k depth
			AskSize:        250000.0, // $112.5k depth
			LastPrice:      0.4501,
			Volume24h:      2000000.0,
			VolumeWindow:   200000.0,
			WindowDuration: 1 * time.Hour,
			ExpectedResult: MicrostructureResult{
				SpreadPass:   true,     // 4.4bps < 50bps
				DepthPass:    true,     // $112.5k > $100k
				VADRPass:     true,     // 2.4× > 1.75×
				OverallPass:  true,
				SpreadBps:    4.4,
				DepthUSD:     112500.0,
				VADRValue:    2.4,
			},
		},
		{
			Name:           "DOGE/USD - Edge Case Values",
			Symbol:         "DOGE/USD",
			Exchange:       "kraken",
			BidPrice:       0.08,
			AskPrice:       0.08004, // 50bps exactly
			BidSize:        1250000.0, // $100k exactly
			AskSize:        1250000.0, // $100k exactly
			LastPrice:      0.08002,
			Volume24h:      800000.0,
			VolumeWindow:   58333.0, // Exactly 1.75× expected
			WindowDuration: 1 * time.Hour,
			ExpectedResult: MicrostructureResult{
				SpreadPass:   true,     // 50bps = 50bps (at limit)
				DepthPass:    true,     // $100k = $100k (at limit)
				VADRPass:     true,     // 1.75× = 1.75× (at limit)
				OverallPass:  true,
				SpreadBps:    50.0,
				DepthUSD:     100000.0,
				VADRValue:    1.75,
			},
		},
	}
}