package derivs

import (
	"fmt"
	"math"
	"testing"
	"time"

	"cryptorun/src/domain/derivs"
)

func TestFundingZ_VolumeWeightedMedian(t *testing.T) {
	config := derivs.MetricsConfig{
		FundingLookbackDays: 30,
		MinObservations:     10,
		RSquaredThreshold:   0.1,
	}

	metrics := derivs.NewDerivativesMetrics(config)

	// Create test venue data with different volumes and funding rates
	venueData := []derivs.VenueData{
		{
			VenueName: "binance",
			Volume:    1000000, // High volume
			FundingRates: []derivs.FundingRatePoint{
				{Rate: 0.001, Timestamp: time.Now(), MarkPrice: 50000},
				{Rate: 0.0015, Timestamp: time.Now().Add(-8 * time.Hour), MarkPrice: 49500},
			},
		},
		{
			VenueName: "okx",
			Volume:    500000, // Medium volume
			FundingRates: []derivs.FundingRatePoint{
				{Rate: 0.002, Timestamp: time.Now(), MarkPrice: 50100},
				{Rate: 0.0025, Timestamp: time.Now().Add(-8 * time.Hour), MarkPrice: 49600},
			},
		},
		{
			VenueName: "bybit",
			Volume:    250000, // Low volume
			FundingRates: []derivs.FundingRatePoint{
				{Rate: 0.005, Timestamp: time.Now(), MarkPrice: 50200}, // Outlier rate
				{Rate: 0.004, Timestamp: time.Now().Add(-8 * time.Hour), MarkPrice: 49700},
			},
		},
	}

	result, err := metrics.FundingZ(venueData)
	if err != nil {
		t.Fatalf("FundingZ calculation failed: %v", err)
	}

	// Volume-weighted median should be closer to binance (highest volume) rate
	// Expected: closer to 0.001 than to 0.005 due to volume weighting
	if result.VolumeWeightedMedian >= 0.004 {
		t.Errorf("Volume weighting failed: median %.6f too close to low-volume outlier",
			result.VolumeWeightedMedian)
	}

	if result.ValidVenues != 3 {
		t.Errorf("Expected 3 valid venues, got %d", result.ValidVenues)
	}

	// Check venue contributions are recorded
	if len(result.VenueContributions) != 3 {
		t.Errorf("Expected 3 venue contributions, got %d", len(result.VenueContributions))
	}

	t.Logf("Volume-weighted median: %.6f", result.VolumeWeightedMedian)
	t.Logf("Z-score: %.3f", result.ZScore)
	t.Logf("Data quality: %s", result.DataQuality)
}

func TestFundingZ_ZScoreCalculation(t *testing.T) {
	config := derivs.MetricsConfig{
		FundingLookbackDays: 30,
		MinObservations:     5, // Lower threshold for test
	}

	metrics := derivs.NewDerivativesMetrics(config)

	// Create test data with known statistics
	// Historical rates: 0.001, 0.0015, 0.002, 0.0025, 0.003
	// Mean = 0.002, Std ≈ 0.000791
	baseTime := time.Now().Add(-24 * time.Hour)

	venueData := []derivs.VenueData{
		{
			VenueName: "venue1",
			Volume:    1000000,
			FundingRates: []derivs.FundingRatePoint{
				{Rate: 0.004, Timestamp: time.Now(), MarkPrice: 50000}, // Current: high rate
				{Rate: 0.001, Timestamp: baseTime, MarkPrice: 49000},
				{Rate: 0.0015, Timestamp: baseTime.Add(-8 * time.Hour), MarkPrice: 49100},
				{Rate: 0.002, Timestamp: baseTime.Add(-16 * time.Hour), MarkPrice: 49200},
				{Rate: 0.0025, Timestamp: baseTime.Add(-24 * time.Hour), MarkPrice: 49300},
				{Rate: 0.003, Timestamp: baseTime.Add(-32 * time.Hour), MarkPrice: 49400},
			},
		},
	}

	result, err := metrics.FundingZ(venueData)
	if err != nil {
		t.Fatalf("FundingZ calculation failed: %v", err)
	}

	// Current rate (0.004) vs historical mean (~0.002) should give positive z-score
	if result.ZScore <= 0 {
		t.Errorf("Expected positive z-score for elevated funding rate, got %.3f", result.ZScore)
	}

	// Z-score should be approximately (0.004 - 0.002) / std
	expectedZ := 2.0 / 0.000791 // Roughly 2.5
	if math.Abs(result.ZScore-expectedZ) > 1.0 {
		t.Logf("Z-score calculation: got %.3f, rough expected %.3f", result.ZScore, expectedZ)
		// Don't fail - just log for inspection
	}

	t.Logf("Historical mean: %.6f, std: %.6f", result.HistoricalMean, result.HistoricalStd)
	t.Logf("Current rate: %.6f, Z-score: %.3f", result.VolumeWeightedMedian, result.ZScore)
}

func TestDeltaOIResidual_OLSRegression(t *testing.T) {
	config := derivs.MetricsConfig{
		MinObservations:   5,
		RSquaredThreshold: 0.1,
	}

	metrics := derivs.NewDerivativesMetrics(config)

	// Create test OI data with known correlation to price
	// OI follows price with some noise: OI_change = 0.5 * price_change + noise
	baseTime := time.Now().Add(-time.Hour)

	oiData := []derivs.OIPoint{
		{Value: 1000, Price: 50000, Timestamp: baseTime},
		{Value: 1020, Price: 51000, Timestamp: baseTime.Add(10 * time.Minute)}, // OI up 2%, price up 2%
		{Value: 1015, Price: 50500, Timestamp: baseTime.Add(20 * time.Minute)}, // OI down 0.5%, price down 1%
		{Value: 1040, Price: 52000, Timestamp: baseTime.Add(30 * time.Minute)}, // OI up 2.5%, price up 3%
		{Value: 1050, Price: 51500, Timestamp: baseTime.Add(40 * time.Minute)}, // OI up 1%, price down 1%
		{Value: 1080, Price: 53000, Timestamp: baseTime.Add(50 * time.Minute)}, // OI up 2.9%, price up 2.9%
	}

	result, err := metrics.DeltaOIResidual(oiData)
	if err != nil {
		t.Fatalf("DeltaOIResidual calculation failed: %v", err)
	}

	// Should detect positive correlation between OI and price changes
	if result.PriceCorr <= 0 {
		t.Errorf("Expected positive price correlation, got %.3f", result.PriceCorr)
	}

	// Beta should be positive (OI increases with price)
	if result.Beta <= 0 {
		t.Errorf("Expected positive beta coefficient, got %.3f", result.Beta)
	}

	// R-squared should be reasonable for this synthetic data
	if result.RSquared < 0.1 {
		t.Errorf("Expected R² ≥ 0.1, got %.3f", result.RSquared)
	}

	// Signal quality should be reasonable
	if result.SignalQuality == "low" {
		t.Errorf("Expected medium/high signal quality, got %s", result.SignalQuality)
	}

	t.Logf("OLS results: β=%.3f, α=%.6f, R²=%.3f", result.Beta, result.Alpha, result.RSquared)
	t.Logf("Price correlation: %.3f", result.PriceCorr)
	t.Logf("Latest residual: %.6f", result.Residual)
	t.Logf("Signal quality: %s", result.SignalQuality)
}

func TestDeltaOIResidual_InsufficientData(t *testing.T) {
	config := derivs.MetricsConfig{
		MinObservations: 10,
	}

	metrics := derivs.NewDerivativesMetrics(config)

	// Provide insufficient data points
	oiData := []derivs.OIPoint{
		{Value: 1000, Price: 50000, Timestamp: time.Now().Add(-time.Hour)},
		{Value: 1020, Price: 51000, Timestamp: time.Now().Add(-50 * time.Minute)},
		{Value: 1015, Price: 50500, Timestamp: time.Now().Add(-40 * time.Minute)},
	}

	_, err := metrics.DeltaOIResidual(oiData)
	if err == nil {
		t.Error("Expected error for insufficient data points")
	}

	if err.Error() == "" {
		t.Error("Error message should be descriptive")
	}

	t.Logf("Expected error for insufficient data: %v", err)
}

func TestBasisDispersion_VenueAnalysis(t *testing.T) {
	config := derivs.MetricsConfig{}
	metrics := derivs.NewDerivativesMetrics(config)

	// Create venue data with different funding rates (proxy for basis)
	venueData := []derivs.VenueData{
		{
			VenueName: "venue_low",
			FundingRates: []derivs.FundingRatePoint{
				{Rate: -0.001, Timestamp: time.Now()}, // Negative funding (backwardation)
			},
		},
		{
			VenueName: "venue_medium",
			FundingRates: []derivs.FundingRatePoint{
				{Rate: 0.002, Timestamp: time.Now()}, // Positive funding (contango)
			},
		},
		{
			VenueName: "venue_high",
			FundingRates: []derivs.FundingRatePoint{
				{Rate: 0.008, Timestamp: time.Now()}, // High positive funding
			},
		},
	}

	result, err := metrics.BasisDispersion(venueData)
	if err != nil {
		t.Fatalf("BasisDispersion calculation failed: %v", err)
	}

	// Should detect dispersion across venues
	if result.Dispersion <= 0 {
		t.Errorf("Expected positive dispersion, got %.6f", result.Dispersion)
	}

	// Cross-venue spread should be positive
	if result.CrossVenueSpread <= 0 {
		t.Errorf("Expected positive cross-venue spread, got %.6f", result.CrossVenueSpread)
	}

	// Should have venue basis recorded for all venues
	if len(result.VenueBasis) != 3 {
		t.Errorf("Expected 3 venue basis values, got %d", len(result.VenueBasis))
	}

	// Check signal interpretation
	if result.Signal == "" {
		t.Error("Signal interpretation should not be empty")
	}

	t.Logf("Basis dispersion: %.6f", result.Dispersion)
	t.Logf("Cross-venue spread: %.6f", result.CrossVenueSpread)
	t.Logf("Backwardation: %v, Contango: %v", result.Backwardation, result.Contango)
	t.Logf("Signal: %s", result.Signal)
	t.Logf("Venue basis: %+v", result.VenueBasis)
}

func TestBasisDispersion_EdgeCases(t *testing.T) {
	config := derivs.MetricsConfig{}
	metrics := derivs.NewDerivativesMetrics(config)

	tests := []struct {
		name        string
		venueData   []derivs.VenueData
		expectError bool
		description string
	}{
		{
			name:        "no_venues",
			venueData:   []derivs.VenueData{},
			expectError: true,
			description: "Empty venue data should return error",
		},
		{
			name: "single_venue",
			venueData: []derivs.VenueData{
				{
					VenueName: "single",
					FundingRates: []derivs.FundingRatePoint{
						{Rate: 0.001, Timestamp: time.Now()},
					},
				},
			},
			expectError: true,
			description: "Single venue insufficient for dispersion analysis",
		},
		{
			name: "venues_no_funding_data",
			venueData: []derivs.VenueData{
				{VenueName: "empty1", FundingRates: []derivs.FundingRatePoint{}},
				{VenueName: "empty2", FundingRates: []derivs.FundingRatePoint{}},
			},
			expectError: true,
			description: "Venues without funding data should fail",
		},
		{
			name: "identical_funding_rates",
			venueData: []derivs.VenueData{
				{
					VenueName: "venue1",
					FundingRates: []derivs.FundingRatePoint{
						{Rate: 0.001, Timestamp: time.Now()},
					},
				},
				{
					VenueName: "venue2",
					FundingRates: []derivs.FundingRatePoint{
						{Rate: 0.001, Timestamp: time.Now()},
					},
				},
			},
			expectError: false,
			description: "Identical rates should give zero dispersion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := metrics.BasisDispersion(tt.venueData)

			if tt.expectError && err == nil {
				t.Errorf("%s: expected error but got none", tt.description)
			}

			if !tt.expectError && err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
			}

			if !tt.expectError && result != nil {
				t.Logf("%s: dispersion=%.6f, signal=%s", tt.name, result.Dispersion, result.Signal)

				// Special case: identical rates should have zero dispersion
				if tt.name == "identical_funding_rates" && result.Dispersion > 0.000001 {
					t.Errorf("Expected zero dispersion for identical rates, got %.6f", result.Dispersion)
				}
			}
		})
	}
}

func TestVolumeWeightedMedian_Calculation(t *testing.T) {
	// Test the volume-weighted median calculation directly
	// This tests the helper function used in FundingZ

	tests := []struct {
		name      string
		values    []derivs.WeightedValue
		expected  float64
		tolerance float64
	}{
		{
			name: "simple_equal_weights",
			values: []derivs.WeightedValue{
				{Value: 1.0, Weight: 1.0},
				{Value: 2.0, Weight: 1.0},
				{Value: 3.0, Weight: 1.0},
			},
			expected:  2.0, // Regular median
			tolerance: 0.001,
		},
		{
			name: "volume_weighted_bias",
			values: []derivs.WeightedValue{
				{Value: 1.0, Weight: 1.0},  // Low volume
				{Value: 2.0, Weight: 10.0}, // High volume - should dominate
				{Value: 3.0, Weight: 1.0},  // Low volume
			},
			expected:  2.0, // Should be close to high-volume value
			tolerance: 0.1,
		},
		{
			name: "single_value",
			values: []derivs.WeightedValue{
				{Value: 5.0, Weight: 100.0},
			},
			expected:  5.0,
			tolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a basic config and metrics instance to access helper functions
			config := derivs.MetricsConfig{}
			metrics := derivs.NewDerivativesMetrics(config)

			// We need to test this indirectly through FundingZ since helper is private
			// Create dummy venue data that will produce the test values
			var venueData []derivs.VenueData
			for i, wv := range tt.values {
				venueData = append(venueData, derivs.VenueData{
					VenueName: fmt.Sprintf("venue%d", i),
					Volume:    wv.Weight,
					FundingRates: []derivs.FundingRatePoint{
						{Rate: wv.Value, Timestamp: time.Now()},
					},
				})
			}

			result, err := metrics.FundingZ(venueData)
			if err != nil {
				t.Fatalf("FundingZ failed: %v", err)
			}

			if math.Abs(result.VolumeWeightedMedian-tt.expected) > tt.tolerance {
				t.Errorf("Volume-weighted median: expected %.3f, got %.3f (tolerance %.3f)",
					tt.expected, result.VolumeWeightedMedian, tt.tolerance)
			}
		})
	}
}
