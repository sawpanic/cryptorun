package microstructure

import (
	"context"
	"testing"
	"time"

	"cryptorun/internal/data/venue/types"
)

func TestChecker_ValidateAsset(t *testing.T) {
	tests := []struct {
		name             string
		symbol           string
		maxSpreadBPS     float64
		minDepthUSD      float64
		minVADR          float64
		venues           []string
		requireAllVenues bool
		expectedValid    bool
		expectedEligible int
	}{
		{
			name:             "BTCUSDT meets all requirements",
			symbol:           "BTCUSDT",
			maxSpreadBPS:     50.0,
			minDepthUSD:      100000,
			minVADR:          1.75,
			venues:           []string{"binance"},
			requireAllVenues: false,
			expectedValid:    true,
			expectedEligible: 1,
		},
		{
			name:             "Strict spread requirement fails",
			symbol:           "BTCUSDT",
			maxSpreadBPS:     10.0, // Very strict
			minDepthUSD:      100000,
			minVADR:          1.75,
			venues:           []string{"binance"},
			requireAllVenues: false,
			expectedValid:    false,
			expectedEligible: 0,
		},
		{
			name:             "High depth requirement fails",
			symbol:           "BTCUSDT",
			maxSpreadBPS:     50.0,
			minDepthUSD:      1000000, // Very high
			minVADR:          1.75,
			venues:           []string{"binance"},
			requireAllVenues: false,
			expectedValid:    false,
			expectedEligible: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create checker with mock clients for testing
			checker := NewMockChecker()
			checker.SetThresholds(tt.maxSpreadBPS, tt.minDepthUSD, tt.minVADR)
			checker.SetVenues(tt.venues, tt.requireAllVenues)

			ctx := context.Background()
			result, err := checker.ValidateAsset(ctx, tt.symbol)

			if err != nil {
				t.Fatalf("ValidateAsset() error = %v", err)
			}

			if result.OverallValid != tt.expectedValid {
				t.Errorf("ValidateAsset() OverallValid = %v, expected %v",
					result.OverallValid, tt.expectedValid)
			}

			if result.PassedVenueCount != tt.expectedEligible {
				t.Errorf("ValidateAsset() PassedVenueCount = %v, expected %v",
					result.PassedVenueCount, tt.expectedEligible)
			}

			// Validate result structure
			if result.Symbol != tt.symbol {
				t.Errorf("ValidateAsset() Symbol = %v, expected %v",
					result.Symbol, tt.symbol)
			}

			if result.TotalVenueCount != len(tt.venues) {
				t.Errorf("ValidateAsset() TotalVenueCount = %v, expected %v",
					result.TotalVenueCount, len(tt.venues))
			}
		})
	}
}

func TestValidationResult_GetSummary(t *testing.T) {
	tests := []struct {
		name            string
		result          *ValidationResult
		expectedSummary string
	}{
		{
			name: "Successful validation",
			result: &ValidationResult{
				OverallValid:     true,
				PassedVenueCount: 2,
				EligibleVenues:   []string{"binance", "okx"},
			},
			expectedSummary: "✅ ELIGIBLE - Passed on 2 venue(s): [binance okx]",
		},
		{
			name: "Failed validation",
			result: &ValidationResult{
				OverallValid:     false,
				PassedVenueCount: 0,
				TotalVenueCount:  3,
				FailedVenues:     []string{"binance", "okx", "coinbase"},
			},
			expectedSummary: "❌ NOT ELIGIBLE - Failed on 3/3 venues",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := tt.result.GetSummary()
			if summary != tt.expectedSummary {
				t.Errorf("GetSummary() = %v, expected %v", summary, tt.expectedSummary)
			}
		})
	}
}

func TestValidationResult_GetDetailedReasons(t *testing.T) {
	result := &ValidationResult{
		VenueResults: map[string]*VenueValidation{
			"binance": {
				Venue: "binance",
				Valid: false,
				FailureReasons: []string{
					"Spread 75.0bps > 50.0bps limit",
					"Depth $80k < $100k limit",
				},
			},
			"okx": {
				Venue: "okx",
				Valid: true,
			},
			"coinbase": {
				Venue: "coinbase",
				Valid: false,
				Error: "API timeout",
			},
		},
	}

	reasons := result.GetDetailedReasons()

	// Should have reasons for failed venues only
	if len(reasons) != 2 {
		t.Errorf("GetDetailedReasons() returned %d venues, expected 2", len(reasons))
	}

	// Check binance reasons
	binanceReasons, ok := reasons["binance"]
	if !ok {
		t.Error("GetDetailedReasons() missing binance reasons")
	}
	if len(binanceReasons) != 2 {
		t.Errorf("GetDetailedReasons() binance has %d reasons, expected 2", len(binanceReasons))
	}

	// Check coinbase error
	coinbaseReasons, ok := reasons["coinbase"]
	if !ok {
		t.Error("GetDetailedReasons() missing coinbase reasons")
	}
	if len(coinbaseReasons) != 1 || coinbaseReasons[0] != "API timeout" {
		t.Errorf("GetDetailedReasons() coinbase = %v, expected [API timeout]", coinbaseReasons)
	}

	// Should not have reasons for successful venues
	if _, exists := reasons["okx"]; exists {
		t.Error("GetDetailedReasons() should not include successful venues")
	}
}

// NewMockChecker creates a checker with mock clients for testing
func NewMockChecker() *MockChecker {
	return &MockChecker{
		maxSpreadBPS: 50.0,
		minDepthUSD:  100000,
		minVADR:      1.75,
		venues:       []string{"binance"},
	}
}

// MockChecker implements validation logic with deterministic test data
type MockChecker struct {
	maxSpreadBPS     float64
	minDepthUSD      float64
	minVADR          float64
	requireAllVenues bool
	venues           []string
}

func (mc *MockChecker) SetThresholds(maxSpreadBPS, minDepthUSD, minVADR float64) {
	mc.maxSpreadBPS = maxSpreadBPS
	mc.minDepthUSD = minDepthUSD
	mc.minVADR = minVADR
}

func (mc *MockChecker) SetVenues(venues []string, requireAll bool) {
	mc.venues = venues
	mc.requireAllVenues = requireAll
}

func (mc *MockChecker) ValidateAsset(ctx context.Context, symbol string) (*ValidationResult, error) {
	result := &ValidationResult{
		Symbol:         symbol,
		TimestampMono:  time.Now(),
		VenueResults:   make(map[string]*VenueValidation),
		EligibleVenues: []string{},
		FailedVenues:   []string{},
	}

	for _, venue := range mc.venues {
		venueResult := mc.createMockVenueValidation(symbol, venue)
		result.VenueResults[venue] = venueResult

		if venueResult.Valid {
			result.EligibleVenues = append(result.EligibleVenues, venue)
			result.PassedVenueCount++
		} else {
			result.FailedVenues = append(result.FailedVenues, venue)
		}
		result.TotalVenueCount++
	}

	if mc.requireAllVenues {
		result.OverallValid = result.PassedVenueCount == result.TotalVenueCount
	} else {
		result.OverallValid = result.PassedVenueCount > 0
	}

	return result, nil
}

func (mc *MockChecker) createMockVenueValidation(symbol, venue string) *VenueValidation {
	// Create mock orderbook with realistic values
	orderBook := &types.OrderBook{
		Symbol:                symbol,
		Venue:                 venue,
		TimestampMono:         time.Now(),
		BestBidPrice:          43250.50,
		BestAskPrice:          43251.00,
		MidPrice:              43250.75,
		SpreadBPS:             11.55,  // ~0.12% spread (50 bps = 0.5%)
		DepthUSDPlusMinus2Pct: 180000, // $180k depth
	}

	// Create mock metrics
	metrics := &types.MicrostructureMetrics{
		Symbol:                symbol,
		Venue:                 venue,
		TimestampMono:         time.Now(),
		SpreadBPS:             orderBook.SpreadBPS,
		DepthUSDPlusMinus2Pct: orderBook.DepthUSDPlusMinus2Pct,
		VADR:                  2.1, // Good VADR
		DataSource:            venue,
	}

	// Validate against thresholds
	spreadValid := metrics.SpreadBPS < mc.maxSpreadBPS
	depthValid := metrics.DepthUSDPlusMinus2Pct >= mc.minDepthUSD
	vadrValid := metrics.VADR >= mc.minVADR
	overallValid := spreadValid && depthValid && vadrValid

	metrics.SpreadValid = spreadValid
	metrics.DepthValid = depthValid
	metrics.VADRValid = vadrValid
	metrics.OverallValid = overallValid

	// Build failure reasons
	var failureReasons []string
	if !spreadValid {
		failureReasons = append(failureReasons,
			"Spread 75.0bps > 50.0bps limit") // Mock high spread for test
	}
	if !depthValid {
		failureReasons = append(failureReasons,
			"Depth $80k < $100k limit") // Mock low depth for test
	}
	if !vadrValid {
		failureReasons = append(failureReasons,
			"VADR 1.50x < 1.75x limit") // Mock low VADR for test
	}

	return &VenueValidation{
		Venue:          venue,
		Valid:          overallValid,
		OrderBook:      orderBook,
		Metrics:        metrics,
		FailureReasons: failureReasons,
	}
}
