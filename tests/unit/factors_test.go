package unit

import (
	"math"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/application"
	"github.com/sawpanic/cryptorun/internal/domain/factors"
	"github.com/sawpanic/cryptorun/internal/domain/indicators"
)

func TestBuildRawFactorRow(t *testing.T) {
	config := application.WeightsConfig{
		Validation: struct {
			WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
			MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
			MaxSocialWeight    float64 `yaml:"max_social_weight"`
			SocialHardCap      float64 `yaml:"social_hard_cap"`
		}{
			SocialHardCap: 10.0,
		},
	}

	builder := factors.NewFactorBuilder(config)

	// Create test data
	priceHistory := make([]float64, 48) // 48 hours of data
	for i := range priceHistory {
		priceHistory[i] = 50000.0 + float64(i)*100.0 // Trending upward
	}

	volumeHistory := make([]float64, 7) // 7 days of volume
	for i := range volumeHistory {
		volumeHistory[i] = 1000000.0 // 1M volume per day
	}

	ohlcHistory := make([]indicators.PriceBar, 24)
	for i := range ohlcHistory {
		price := priceHistory[i+24] // Use last 24 hours
		ohlcHistory[i] = indicators.PriceBar{
			High:  price * 1.02,
			Low:   price * 0.98,
			Close: price,
		}
	}

	technicalData := indicators.TechnicalIndicators{
		RSI: indicators.RSIResult{
			Value:   60.0,
			IsValid: true,
		},
		ATR: indicators.ATRResult{
			Value:   1000.0, // $1000 ATR
			IsValid: true,
		},
		ADX: indicators.ADXResult{
			ADX:     30.0, // Strong trend
			IsValid: true,
		},
		Hurst: indicators.HurstResult{
			Exponent: 0.7, // Persistent
			IsValid:  true,
		},
	}

	factorData := factors.FactorData{
		Symbol:          "BTC-USD",
		CurrentPrice:    52400.0, // Last price in series
		PriceHistory:    priceHistory,
		VolumeHistory:   volumeHistory,
		OHLCHistory:     ohlcHistory,
		TechnicalData:   technicalData,
		FundingRate:     0.001,
		OpenInterest:    1000000000,
		SocialScore:     75.0,
		QualityScore:    80.0,
		MarketCap:       1000000000000, // $1T market cap
		Volume24h:       2000000.0,     // 2M volume (2x average)
		Timestamp:       time.Now(),
	}

	// Test successful factor building
	row, err := builder.BuildRawFactorRow(factorData)
	if err != nil {
		t.Fatalf("BuildRawFactorRow failed: %v", err)
	}

	// Validate basic properties
	if row.Symbol != factorData.Symbol {
		t.Errorf("Expected symbol %s, got %s", factorData.Symbol, row.Symbol)
	}

	// Check momentum core calculation
	if row.MomentumCore == 0.0 {
		t.Error("MomentumCore should not be zero with trending data")
	}

	// Check technical factor
	if row.TechnicalFactor < 0 || row.TechnicalFactor > 100 {
		t.Errorf("TechnicalFactor should be 0-100, got %.2f", row.TechnicalFactor)
	}

	// Check volume factor (should be high due to 2x average volume)
	if row.VolumeFactor < 50 {
		t.Errorf("VolumeFactor should be high with 2x volume, got %.2f", row.VolumeFactor)
	}

	// Check quality factor
	if row.QualityFactor < 0 || row.QualityFactor > 100 {
		t.Errorf("QualityFactor should be 0-100, got %.2f", row.QualityFactor)
	}

	// Check social factor
	if row.SocialFactor < 0 || row.SocialFactor > 100 {
		t.Errorf("SocialFactor should be 0-100, got %.2f", row.SocialFactor)
	}

	// Validate factor details
	if row.FactorDetails.MomentumBreakdown.Composite != row.MomentumCore {
		t.Error("MomentumCore should match momentum breakdown composite")
	}

	// Test validation
	if err := factors.ValidateFactorRow(row); err != nil {
		t.Errorf("Valid factor row failed validation: %v", err)
	}
}

func TestBuildRawFactorRowErrors(t *testing.T) {
	config := application.WeightsConfig{}
	builder := factors.NewFactorBuilder(config)

	// Test with invalid price
	invalidData := factors.FactorData{
		Symbol:       "BTC-USD",
		CurrentPrice: 0.0, // Invalid
		Timestamp:    time.Now(),
	}

	_, err := builder.BuildRawFactorRow(invalidData)
	if err == nil {
		t.Error("Expected error for invalid current price")
	}

	// Test with negative price
	invalidData.CurrentPrice = -100.0
	_, err = builder.BuildRawFactorRow(invalidData)
	if err == nil {
		t.Error("Expected error for negative current price")
	}
}

func TestMomentumCalculation(t *testing.T) {
	config := application.WeightsConfig{}
	builder := factors.NewFactorBuilder(config)

	// Test with perfect 1% hourly growth
	priceHistory := make([]float64, 25) // 25 hours
	basePrice := 100.0
	for i := range priceHistory {
		priceHistory[i] = basePrice * math.Pow(1.01, float64(i))
	}

	currentPrice := priceHistory[24] // Last price

	factorData := factors.FactorData{
		Symbol:       "TEST-USD",
		CurrentPrice: currentPrice,
		PriceHistory: priceHistory,
		Timestamp:    time.Now(),
		TechnicalData: indicators.TechnicalIndicators{
			RSI: indicators.RSIResult{IsValid: false}, // Invalid to isolate momentum
		},
	}

	row, err := builder.BuildRawFactorRow(factorData)
	if err != nil {
		t.Fatalf("BuildRawFactorRow failed: %v", err)
	}

	// With 1% hourly growth, momentum should be positive
	if row.MomentumCore <= 0 {
		t.Errorf("Expected positive momentum with growing prices, got %.2f", row.MomentumCore)
	}

	// Check momentum breakdown
	momentum1h := row.FactorDetails.MomentumBreakdown.Momentum1h
	if momentum1h < 0.8 || momentum1h > 1.2 { // Should be ~1%
		t.Errorf("1h momentum should be ~1%%, got %.2f%%", momentum1h)
	}

	momentum24h := row.FactorDetails.MomentumBreakdown.Momentum24h
	if momentum24h < 20 || momentum24h > 30 { // Should be ~24%
		t.Errorf("24h momentum should be ~24%%, got %.2f%%", momentum24h)
	}
}

func TestVolumeFactorCalculation(t *testing.T) {
	config := application.WeightsConfig{}
	builder := factors.NewFactorBuilder(config)

	tests := []struct {
		name             string
		volumeHistory    []float64
		currentVolume    float64
		expectedCategory string // "low", "medium", "high"
	}{
		{
			name:             "volume surge",
			volumeHistory:    []float64{1000, 1100, 900, 1000, 1050, 950, 1000},
			currentVolume:    3000, // 3x average
			expectedCategory: "high",
		},
		{
			name:             "normal volume",
			volumeHistory:    []float64{1000, 1100, 900, 1000, 1050, 950, 1000},
			currentVolume:    1000, // Same as average
			expectedCategory: "low",
		},
		{
			name:             "moderate increase",
			volumeHistory:    []float64{1000, 1100, 900, 1000, 1050, 950, 1000},
			currentVolume:    2000, // 2x average
			expectedCategory: "medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factorData := factors.FactorData{
				Symbol:        "TEST-USD",
				CurrentPrice:  100.0,
				VolumeHistory: tt.volumeHistory,
				Volume24h:     tt.currentVolume,
				Timestamp:     time.Now(),
			}

			row, err := builder.BuildRawFactorRow(factorData)
			if err != nil {
				t.Fatalf("BuildRawFactorRow failed: %v", err)
			}

			volumeFactor := row.VolumeFactor
			volumeInputs := row.FactorDetails.VolumeInputs

			// Check volume ratio calculation
			expectedRatio := tt.currentVolume / 1000.0 // Average is 1000
			if math.Abs(volumeInputs.VolumeRatio-expectedRatio) > 0.01 {
				t.Errorf("Expected volume ratio %.2f, got %.2f", expectedRatio, volumeInputs.VolumeRatio)
			}

			// Check surge detection
			expectedSurge := tt.currentVolume >= 1750 // 1.75x threshold
			if volumeInputs.VolumeSurge != expectedSurge {
				t.Errorf("Expected volume surge %v, got %v", expectedSurge, volumeInputs.VolumeSurge)
			}

			// Check score categories
			switch tt.expectedCategory {
			case "high":
				if volumeFactor < 80 {
					t.Errorf("Expected high volume score (>80), got %.1f", volumeFactor)
				}
			case "medium":
				if volumeFactor < 40 || volumeFactor > 80 {
					t.Errorf("Expected medium volume score (40-80), got %.1f", volumeFactor)
				}
			case "low":
				if volumeFactor > 40 {
					t.Errorf("Expected low volume score (<40), got %.1f", volumeFactor)
				}
			}
		})
	}
}

func TestOrthogonalizeBatch(t *testing.T) {
	// Create config
	config := application.WeightsConfig{
		Validation: struct {
			WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
			MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
			MaxSocialWeight    float64 `yaml:"max_social_weight"`
			SocialHardCap      float64 `yaml:"social_hard_cap"`
		}{
			SocialHardCap: 10.0,
		},
		QARequirements: application.QARequirements{
			CorrelationThreshold: 0.6,
		},
	}

	orthogonalizer := factors.NewGramSchmidtOrthogonalizer(config)

	// Create test data with known correlations
	numRows := 100
	rawRows := make([]factors.RawFactorRow, numRows)

	for i := 0; i < numRows; i++ {
		// Create factors with some correlation to test orthogonalization
		momentum := float64(i) * 0.1              // Linear trend
		technical := momentum*0.5 + float64(i)*0.05  // Correlated with momentum
		volume := float64(i) * 0.2 + 50.0          // Different trend
		quality := 75.0 - float64(i)*0.02          // Declining trend
		social := momentum * 0.3 + 60.0             // Correlated with momentum

		rawRows[i] = factors.RawFactorRow{
			Symbol:          fmt.Sprintf("COIN%d", i),
			MomentumCore:    momentum,
			TechnicalFactor: technical,
			VolumeFactor:    volume,
			QualityFactor:   quality,
			SocialFactor:    social,
			Timestamp:       time.Now(),
		}
	}

	// Test orthogonalization
	orthogonalizedRows, err := orthogonalizer.OrthogonalizeBatch(rawRows)
	if err != nil {
		t.Fatalf("OrthogonalizeBatch failed: %v", err)
	}

	if len(orthogonalizedRows) != numRows {
		t.Errorf("Expected %d orthogonalized rows, got %d", numRows, len(orthogonalizedRows))
	}

	// Test first row in detail
	firstRow := orthogonalizedRows[0]

	// Check that MomentumCore is preserved
	if math.Abs(firstRow.MomentumCore-rawRows[0].MomentumCore) > 1e-10 {
		t.Errorf("MomentumCore not preserved: %.6f != %.6f", 
			firstRow.MomentumCore, rawRows[0].MomentumCore)
	}

	// Check orthogonalization info
	if firstRow.OrthogonalizationInfo.QualityMetrics.MaxCorrelation > config.QARequirements.CorrelationThreshold {
		t.Errorf("Max correlation %.3f exceeds threshold %.3f", 
			firstRow.OrthogonalizationInfo.QualityMetrics.MaxCorrelation, 
			config.QARequirements.CorrelationThreshold)
	}

	// Check momentum preservation
	if firstRow.OrthogonalizationInfo.QualityMetrics.MomentumPreserved < 99.0 {
		t.Errorf("Momentum preservation too low: %.1f%%", 
			firstRow.OrthogonalizationInfo.QualityMetrics.MomentumPreserved)
	}

	// Check social capping
	if math.Abs(firstRow.SocialCapped) > config.Validation.SocialHardCap+0.001 {
		t.Errorf("Social cap not enforced: |%.3f| > %.1f", 
			firstRow.SocialCapped, config.Validation.SocialHardCap)
	}

	// Validate orthogonalization
	for i, row := range orthogonalizedRows {
		if err := factors.ValidateOrthogonalization(row, config); err != nil {
			t.Errorf("Row %d failed orthogonalization validation: %v", i, err)
		}
	}
}

func TestOrthogonalizationWithExtremeValues(t *testing.T) {
	config := application.WeightsConfig{
		Validation: struct {
			WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
			MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
			MaxSocialWeight    float64 `yaml:"max_social_weight"`
			SocialHardCap      float64 `yaml:"social_hard_cap"`
		}{
			SocialHardCap: 10.0,
		},
		QARequirements: application.QARequirements{
			CorrelationThreshold: 0.6,
		},
	}

	orthogonalizer := factors.NewGramSchmidtOrthogonalizer(config)

	// Create test data with extreme social values to test capping
	rawRows := []factors.RawFactorRow{
		{
			Symbol:          "TEST1",
			MomentumCore:    10.0,
			TechnicalFactor: 50.0,
			VolumeFactor:    60.0,
			QualityFactor:   70.0,
			SocialFactor:    150.0, // Above cap
			Timestamp:       time.Now(),
		},
		{
			Symbol:          "TEST2",
			MomentumCore:    -5.0,
			TechnicalFactor: 30.0,
			VolumeFactor:    40.0,
			QualityFactor:   50.0,
			SocialFactor:    -50.0, // Below cap
			Timestamp:       time.Now(),
		},
	}

	orthogonalizedRows, err := orthogonalizer.OrthogonalizeBatch(rawRows)
	if err != nil {
		t.Fatalf("OrthogonalizeBatch failed: %v", err)
	}

	// Check social capping
	for i, row := range orthogonalizedRows {
		if math.Abs(row.SocialCapped) > config.Validation.SocialHardCap+0.001 {
			t.Errorf("Row %d social cap not enforced: %.3f", i, row.SocialCapped)
		}

		// Social residual before capping might be higher, but capped should be limited
		if row.SocialCapped > config.Validation.SocialHardCap {
			t.Errorf("Row %d social capped value too high: %.3f > %.1f", 
				i, row.SocialCapped, config.Validation.SocialHardCap)
		}

		if row.SocialCapped < -config.Validation.SocialHardCap {
			t.Errorf("Row %d social capped value too low: %.3f < %.1f", 
				i, row.SocialCapped, -config.Validation.SocialHardCap)
		}
	}
}

func TestFactorValidation(t *testing.T) {
	validRow := factors.RawFactorRow{
		Symbol:          "BTC-USD",
		MomentumCore:    10.5,
		TechnicalFactor: 65.0,
		VolumeFactor:    80.0,
		QualityFactor:   75.0,
		SocialFactor:    45.0,
		Timestamp:       time.Now(),
	}

	// Valid row should pass
	if err := factors.ValidateFactorRow(validRow); err != nil {
		t.Errorf("Valid row failed validation: %v", err)
	}

	// Test NaN values
	invalidRow := validRow
	invalidRow.MomentumCore = math.NaN()
	if err := factors.ValidateFactorRow(invalidRow); err == nil {
		t.Error("Expected error for NaN MomentumCore")
	}

	// Test infinite values
	invalidRow = validRow
	invalidRow.TechnicalFactor = math.Inf(1)
	if err := factors.ValidateFactorRow(invalidRow); err == nil {
		t.Error("Expected error for infinite TechnicalFactor")
	}

	// Test extreme momentum
	invalidRow = validRow
	invalidRow.MomentumCore = 2000.0 // >1000% momentum
	if err := factors.ValidateFactorRow(invalidRow); err == nil {
		t.Error("Expected error for extreme momentum")
	}

	// Test out-of-range technical factor
	invalidRow = validRow
	invalidRow.TechnicalFactor = 150.0
	if err := factors.ValidateFactorRow(invalidRow); err == nil {
		t.Error("Expected error for out-of-range technical factor")
	}

	// Test negative volume factor
	invalidRow = validRow
	invalidRow.VolumeFactor = -10.0
	if err := factors.ValidateFactorRow(invalidRow); err == nil {
		t.Error("Expected error for negative volume factor")
	}
}

func TestBuildFactorRowBatch(t *testing.T) {
	config := application.WeightsConfig{
		Validation: struct {
			WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
			MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
			MaxSocialWeight    float64 `yaml:"max_social_weight"`
			SocialHardCap      float64 `yaml:"social_hard_cap"`
		}{
			SocialHardCap: 10.0,
		},
	}

	builder := factors.NewFactorBuilder(config)

	// Create batch of factor data
	dataList := make([]factors.FactorData, 3)
	for i := range dataList {
		dataList[i] = factors.FactorData{
			Symbol:       fmt.Sprintf("COIN%d", i),
			CurrentPrice: float64(100 + i*10),
			PriceHistory: []float64{90, 95, 100, 105, float64(100 + i*10)},
			VolumeHistory: []float64{1000, 1100, 1200},
			Volume24h:     2000,
			SocialScore:   float64(50 + i*10),
			MarketCap:     1000000000,
			Timestamp:     time.Now(),
		}
	}

	results, errors := builder.BuildFactorRowBatch(dataList)

	if len(results) != len(dataList) {
		t.Errorf("Expected %d results, got %d", len(dataList), len(results))
	}

	if len(errors) != len(dataList) {
		t.Errorf("Expected %d error entries, got %d", len(dataList), len(errors))
	}

	// Check that all succeeded
	for i, err := range errors {
		if err != nil {
			t.Errorf("Batch item %d failed: %v", i, err)
		}
	}

	// Check that symbols match
	for i, result := range results {
		if result.Symbol != dataList[i].Symbol {
			t.Errorf("Result %d symbol mismatch: expected %s, got %s", 
				i, dataList[i].Symbol, result.Symbol)
		}
	}
}