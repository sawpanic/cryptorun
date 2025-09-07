package unit

import (
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/application"
	"github.com/sawpanic/cryptorun/internal/domain/regime"
)

func TestRegimeDetection(t *testing.T) {
	config := createTestWeightsConfig()
	detector := regime.NewRegimeDetector(config)

	// Test calm regime (low vol, strong trend)
	calmData := regime.MarketData{
		Symbol:        "BTC-USD",
		RealizedVol7d: 0.10,  // Low volatility
		MA20:          50000.0,
		CurrentPrice:  52500.0,  // 5% above MA = trending
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: 0.8,
			VolumeRatio:         0.7,
			NewHighsNewLows:     0.6,
		},
		Timestamp: time.Now(),
	}

	detection, err := detector.DetectRegime(calmData)
	if err != nil {
		t.Fatalf("Regime detection failed: %v", err)
	}

	if detection.CurrentRegime != regime.RegimeCalm {
		t.Errorf("Expected calm regime, got %s", detection.CurrentRegime)
	}

	if detection.Confidence < 50.0 {
		t.Errorf("Expected confidence > 50%%, got %.1f%%", detection.Confidence)
	}

	if len(detection.Indicators) != 3 {
		t.Errorf("Expected 3 indicators, got %d", len(detection.Indicators))
	}

	// Verify 4-hour cache works
	detection2, err := detector.DetectRegime(calmData)
	if err != nil {
		t.Fatalf("Second regime detection failed: %v", err)
	}

	if detection2.DetectionTime != detection.DetectionTime {
		t.Errorf("Expected cached result, got fresh detection")
	}
}

func TestRegimeDetectionVolatile(t *testing.T) {
	config := createTestWeightsConfig()
	detector := regime.NewRegimeDetector(config)

	// Test volatile regime (high vol, choppy)
	volatileData := regime.MarketData{
		Symbol:        "BTC-USD",
		RealizedVol7d: 0.45,  // High volatility
		MA20:          50000.0,
		CurrentPrice:  50100.0,  // Close to MA = choppy
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: 0.2,  // Weak breadth
			VolumeRatio:         0.1,
			NewHighsNewLows:     0.0,
		},
		Timestamp: time.Now(),
	}

	detection, err := detector.DetectRegime(volatileData)
	if err != nil {
		t.Fatalf("Volatile regime detection failed: %v", err)
	}

	if detection.CurrentRegime != regime.RegimeVolatile {
		t.Errorf("Expected volatile regime, got %s", detection.CurrentRegime)
	}
}

func TestRegimeDetectionNormal(t *testing.T) {
	config := createTestWeightsConfig()
	detector := regime.NewRegimeDetector(config)

	// Test normal regime (moderate vol, moderate trend)
	normalData := regime.MarketData{
		Symbol:        "BTC-USD",
		RealizedVol7d: 0.25,  // Moderate volatility
		MA20:          50000.0,
		CurrentPrice:  51500.0,  // 3% above MA = moderate trend
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: 0.5,  // Moderate breadth
			VolumeRatio:         0.4,
			NewHighsNewLows:     0.3,
		},
		Timestamp: time.Now(),
	}

	detection, err := detector.DetectRegime(normalData)
	if err != nil {
		t.Fatalf("Normal regime detection failed: %v", err)
	}

	if detection.CurrentRegime != regime.RegimeNormal {
		t.Errorf("Expected normal regime, got %s", detection.CurrentRegime)
	}
}

func TestRegimeWeightValidation(t *testing.T) {
	config := createTestWeightsConfig()

	// Test valid weights
	validWeights := application.RegimeWeights{
		MomentumCore: 0.4,
		Technical:    0.3,
		Volume:      0.2,
		Quality:     0.1,
		Social:      8.0,  // Within hard cap of 10
	}

	err := regime.ValidateRegimeWeights(validWeights, config)
	if err != nil {
		t.Errorf("Valid weights should pass validation: %v", err)
	}

	// Test invalid weight sum
	invalidSumWeights := application.RegimeWeights{
		MomentumCore: 0.6,  // Sum = 1.2 > tolerance
		Technical:    0.3,
		Volume:      0.2,
		Quality:     0.1,
		Social:      8.0,
	}

	err = regime.ValidateRegimeWeights(invalidSumWeights, config)
	if err == nil {
		t.Error("Invalid weight sum should fail validation")
	}

	// Test insufficient momentum weight
	lowMomentumWeights := application.RegimeWeights{
		MomentumCore: 0.1,  // Below minimum of 0.3
		Technical:    0.4,
		Volume:      0.3,
		Quality:     0.2,
		Social:      8.0,
	}

	err = regime.ValidateRegimeWeights(lowMomentumWeights, config)
	if err == nil {
		t.Error("Low momentum weight should fail validation")
	}

	// Test excessive social weight
	highSocialWeights := application.RegimeWeights{
		MomentumCore: 0.4,
		Technical:    0.3,
		Volume:      0.2,
		Quality:     0.1,
		Social:      15.0,  // Above hard cap of 10
	}

	err = regime.ValidateRegimeWeights(highSocialWeights, config)
	if err == nil {
		t.Error("High social weight should fail validation")
	}
}

func TestGetWeightsForRegime(t *testing.T) {
	config := createTestWeightsConfig()
	detector := regime.NewRegimeDetector(config)

	// Test getting weights for each regime
	regimes := []regime.RegimeType{
		regime.RegimeCalm,
		regime.RegimeNormal,
		regime.RegimeVolatile,
	}

	for _, regimeType := range regimes {
		weights, err := detector.GetWeightsForRegime(regimeType)
		if err != nil {
			t.Errorf("Failed to get weights for regime %s: %v", regimeType, err)
			continue
		}

		// Validate the returned weights
		err = regime.ValidateRegimeWeights(weights, config)
		if err != nil {
			t.Errorf("Weights for regime %s are invalid: %v", regimeType, err)
		}
	}

	// Test fallback to default regime
	_, err := detector.GetWeightsForRegime("nonexistent")
	if err == nil {
		t.Error("Should return error for nonexistent regime")
	}
}

func TestRegimeIndicatorWeights(t *testing.T) {
	config := createTestWeightsConfig()
	detector := regime.NewRegimeDetector(config)

	data := regime.MarketData{
		Symbol:        "BTC-USD",
		RealizedVol7d: 0.20,
		MA20:          50000.0,
		CurrentPrice:  51000.0,
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: 0.6,
			VolumeRatio:         0.5,
			NewHighsNewLows:     0.4,
		},
		Timestamp: time.Now(),
	}

	detection, err := detector.DetectRegime(data)
	if err != nil {
		t.Fatalf("Regime detection failed: %v", err)
	}

	// Verify indicator weights sum to 1.0
	totalWeight := 0.0
	for _, indicator := range detection.Indicators {
		totalWeight += indicator.Weight
		
		// Each indicator should have a positive weight
		if indicator.Weight <= 0 {
			t.Errorf("Indicator %s has non-positive weight: %f", indicator.Name, indicator.Weight)
		}
	}

	if totalWeight < 0.99 || totalWeight > 1.01 {
		t.Errorf("Indicator weights should sum to ~1.0, got %f", totalWeight)
	}
}

func TestRegimeChangeTracking(t *testing.T) {
	config := createTestWeightsConfig()
	detector := regime.NewRegimeDetector(config)

	// First detection - calm regime
	calmData := regime.MarketData{
		Symbol:        "BTC-USD",
		RealizedVol7d: 0.10,
		MA20:          50000.0,
		CurrentPrice:  52500.0,
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: 0.8,
			VolumeRatio:         0.7,
			NewHighsNewLows:     0.6,
		},
		Timestamp: time.Now(),
	}

	detection1, _ := detector.DetectRegime(calmData)
	
	// Force cache expiry for second detection
	time.Sleep(1 * time.Millisecond)
	detector = regime.NewRegimeDetector(config)  // Fresh detector

	// Second detection - volatile regime  
	volatileData := regime.MarketData{
		Symbol:        "BTC-USD",
		RealizedVol7d: 0.50,
		MA20:          50000.0,
		CurrentPrice:  50000.0,
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: 0.1,
			VolumeRatio:         0.1,
			NewHighsNewLows:     0.0,
		},
		Timestamp: time.Now(),
	}

	detection2, _ := detector.DetectRegime(volatileData)

	if detection1.CurrentRegime == detection2.CurrentRegime {
		t.Error("Expected regime change between detections")
	}

	// The second detection should not have a regime change timestamp 
	// since it's from a fresh detector
	if detection2.RegimeChangedAt != nil {
		t.Error("Fresh detector should not show regime change")
	}
}

func TestFormatRegimeReport(t *testing.T) {
	config := createTestWeightsConfig()
	detector := regime.NewRegimeDetector(config)

	data := regime.MarketData{
		Symbol:        "BTC-USD",
		RealizedVol7d: 0.20,
		MA20:          50000.0,
		CurrentPrice:  51000.0,
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: 0.6,
			VolumeRatio:         0.5,
			NewHighsNewLows:     0.4,
		},
		Timestamp: time.Now(),
	}

	detection, err := detector.DetectRegime(data)
	if err != nil {
		t.Fatalf("Regime detection failed: %v", err)
	}

	report := regime.FormatRegimeReport(detection)
	
	if len(report) == 0 {
		t.Error("Report should not be empty")
	}

	// Check that report contains key information
	expectedContents := []string{
		string(detection.CurrentRegime),
		"confidence",
		"Indicator Breakdown",
	}

	for _, content := range expectedContents {
		if !contains(report, content) {
			t.Errorf("Report should contain '%s'", content)
		}
	}

	// Test nil detection
	nilReport := regime.FormatRegimeReport(nil)
	if nilReport != "No regime detection available" {
		t.Errorf("Nil detection should return standard message")
	}
}

// Helper functions
func createTestWeightsConfig() application.WeightsConfig {
	return application.WeightsConfig{
		DefaultRegime: "normal",
		Validation: struct {
			WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
			MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
			MaxSocialWeight    float64 `yaml:"max_social_weight"`
			SocialHardCap      float64 `yaml:"social_hard_cap"`
		}{
			WeightSumTolerance: 0.05,
			MinMomentumWeight:  0.3,
			MaxSocialWeight:    10.0,
			SocialHardCap:      10.0,
		},
		Regimes: map[string]application.RegimeWeights{
			"calm": {
				MomentumCore: 0.5,
				Technical:    0.2,
				Volume:      0.2,
				Quality:     0.1,
				Social:      6.0,
			},
			"normal": {
				MomentumCore: 0.4,
				Technical:    0.3,
				Volume:      0.2,
				Quality:     0.1,
				Social:      8.0,
			},
			"volatile": {
				MomentumCore: 0.6,
				Technical:    0.15,
				Volume:      0.15,
				Quality:     0.1,
				Social:      4.0,
			},
		},
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		contains(s[1:], substr))))
}