package integration

import (
	"math"
	"testing"
	"time"

	"cryptorun/internal/domain/factors"
	"cryptorun/internal/domain/regime"
)

func TestRegimeOrchestrator_EndToEndIntegration(t *testing.T) {
	// Create regime detector and weight map
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	weightMap := regime.GetDefaultWeightMap()

	// Create orchestrator
	orchestrator, err := regime.NewRegimeOrchestrator(detector, weightMap)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Create sample factor rows
	factorRows := []factors.FactorRow{
		{
			Symbol:          "BTCUSDT",
			Timestamp:       time.Now(),
			MomentumCore:    85.0,
			TechnicalFactor: 70.0,
			VolumeFactor:    60.0,
			QualityFactor:   80.0,
			SocialFactor:    15.0, // Will be capped at +10
		},
		{
			Symbol:          "ETHUSDT",
			Timestamp:       time.Now(),
			MomentumCore:    72.0,
			TechnicalFactor: 68.0,
			VolumeFactor:    55.0,
			QualityFactor:   65.0,
			SocialFactor:    -5.0,
		},
		{
			Symbol:          "ADAUSDT",
			Timestamp:       time.Now(),
			MomentumCore:    45.0,
			TechnicalFactor: 50.0,
			VolumeFactor:    40.0,
			QualityFactor:   35.0,
			SocialFactor:    8.0,
		},
	}

	// Create market inputs for regime detection
	marketInputs := regime.RegimeInputs{
		RealizedVol7d: 0.25, // Low volatility
		PctAbove20MA:  0.70, // Strong bullish breadth
		BreadthThrust: 0.20, // Positive thrust
		Timestamp:     time.Now(),
	}

	// Process factors with regime adaptation
	processedRows, err := orchestrator.ProcessFactorsWithRegimeAdaptation(factorRows, marketInputs)
	if err != nil {
		t.Fatalf("failed to process factors: %v", err)
	}

	// Validate results
	if len(processedRows) != len(factorRows) {
		t.Errorf("expected %d processed rows, got %d", len(factorRows), len(processedRows))
	}

	// Verify factor processing occurred
	for i, row := range processedRows {
		// Check that orthogonalization occurred
		if row.TechnicalResidual == factorRows[i].TechnicalFactor {
			t.Errorf("technical factor should be residualized, but unchanged for symbol %s", row.Symbol)
		}

		// Check that social cap was applied
		if math.Abs(row.SocialResidual) > 10.001 {
			t.Errorf("social residual should be capped at ±10, got %f for symbol %s", row.SocialResidual, row.Symbol)
		}

		// Check that composite score was calculated
		if row.CompositeScore == 0 {
			t.Errorf("composite score should be calculated for symbol %s", row.Symbol)
		}

		// Check that ranking was applied
		if row.Rank == 0 {
			t.Errorf("rank should be assigned for symbol %s", row.Symbol)
		}
	}

	// Verify ranking is correct (highest score = rank 1)
	if len(processedRows) > 1 {
		for i := 1; i < len(processedRows); i++ {
			if processedRows[i-1].CompositeScore < processedRows[i].CompositeScore {
				t.Errorf("ranking incorrect: row %d score %f should be higher than row %d score %f",
					i-1, processedRows[i-1].CompositeScore, i, processedRows[i].CompositeScore)
			}
		}
	}
}

func TestRegimeOrchestrator_RegimeTransition(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	weightMap := regime.GetDefaultWeightMap()

	orchestrator, err := regime.NewRegimeOrchestrator(detector, weightMap)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	baseTime := time.Now()

	// Create factor rows
	factorRows := []factors.FactorRow{
		{
			Symbol:          "BTCUSDT",
			Timestamp:       baseTime,
			MomentumCore:    80.0,
			TechnicalFactor: 70.0,
			VolumeFactor:    60.0,
			QualityFactor:   50.0,
			SocialFactor:    5.0,
		},
	}

	// Process with trending bull market
	bullInputs := regime.RegimeInputs{
		RealizedVol7d: 0.25,
		PctAbove20MA:  0.70,
		BreadthThrust: 0.20,
		Timestamp:     baseTime,
	}

	bullResults, err := orchestrator.ProcessFactorsWithRegimeAdaptation(factorRows, bullInputs)
	if err != nil {
		t.Fatalf("failed to process bull market factors: %v", err)
	}

	// Wait and process with high vol market (regime transition)
	volInputs := regime.RegimeInputs{
		RealizedVol7d: 0.70, // High volatility
		PctAbove20MA:  0.40,
		BreadthThrust: -0.10,
		Timestamp:     baseTime.Add(5 * time.Hour), // After cadence period
	}

	volResults, err := orchestrator.ProcessFactorsWithRegimeAdaptation(factorRows, volInputs)
	if err != nil {
		t.Fatalf("failed to process high vol factors: %v", err)
	}

	// Compare scores - should be different due to different regime weights
	if len(bullResults) > 0 && len(volResults) > 0 {
		bullScore := bullResults[0].CompositeScore
		volScore := volResults[0].CompositeScore

		// Scores should be different due to different regime weights
		if math.Abs(bullScore-volScore) < 0.01 {
			t.Errorf("scores should differ between regimes: bull=%f, vol=%f", bullScore, volScore)
		}
	}
}

func TestRegimeOrchestrator_MomentumProtection(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	weightMap := regime.GetDefaultWeightMap()

	orchestrator, err := regime.NewRegimeOrchestrator(detector, weightMap)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Create factor rows with high correlations to test orthogonalization
	factorRows := []factors.FactorRow{
		{
			Symbol:          "TEST1",
			Timestamp:       time.Now(),
			MomentumCore:    100.0, // High momentum
			TechnicalFactor: 95.0,  // Highly correlated with momentum
			VolumeFactor:    90.0,  // Highly correlated
			QualityFactor:   85.0,  // Highly correlated
			SocialFactor:    5.0,
		},
		{
			Symbol:          "TEST2",
			Timestamp:       time.Now(),
			MomentumCore:    80.0,
			TechnicalFactor: 75.0,
			VolumeFactor:    70.0,
			QualityFactor:   65.0,
			SocialFactor:    3.0,
		},
	}

	marketInputs := regime.RegimeInputs{
		RealizedVol7d: 0.35,
		PctAbove20MA:  0.55,
		BreadthThrust: 0.10,
		Timestamp:     time.Now(),
	}

	results, err := orchestrator.ProcessFactorsWithRegimeAdaptation(factorRows, marketInputs)
	if err != nil {
		t.Fatalf("failed to process factors: %v", err)
	}

	// Check orthogonality report
	orthReport := orchestrator.GetOrthogonalityReport(results)

	// Verify momentum is protected (should have high weight in final score)
	correlationMatrix, ok := orthReport["correlation_matrix"].(map[string]map[string]float64)
	if !ok {
		t.Fatalf("correlation matrix should be map[string]map[string]float64")
	}

	// MomentumCore should maintain its original values (not residualized)
	for i, result := range results {
		if result.MomentumCore != factorRows[i].MomentumCore {
			t.Errorf("MomentumCore should be protected from orthogonalization: original=%f, processed=%f",
				factorRows[i].MomentumCore, result.MomentumCore)
		}
	}

	// Technical, Volume, Quality should be residualized (different from original)
	for i, result := range results {
		if result.TechnicalResidual == factorRows[i].TechnicalFactor {
			t.Errorf("TechnicalFactor should be residualized")
		}
		if result.VolumeResidual == factorRows[i].VolumeFactor {
			t.Errorf("VolumeFactor should be residualized")
		}
		if result.QualityResidual == factorRows[i].QualityFactor {
			t.Errorf("QualityFactor should be residualized")
		}
	}

	// Check that orthogonality quality is reported
	quality, ok := orthReport["orthogonality_quality"].(string)
	if !ok || quality == "" {
		t.Errorf("orthogonality quality should be reported as string")
	}
}

func TestRegimeOrchestrator_SocialCapEnforcement(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	weightMap := regime.GetDefaultWeightMap()

	orchestrator, err := regime.NewRegimeOrchestrator(detector, weightMap)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Create factor rows with extreme social signals
	factorRows := []factors.FactorRow{
		{
			Symbol:          "HIGH_SOCIAL",
			Timestamp:       time.Now(),
			MomentumCore:    75.0,
			TechnicalFactor: 70.0,
			VolumeFactor:    65.0,
			QualityFactor:   60.0,
			SocialFactor:    50.0, // Very high social signal
		},
		{
			Symbol:          "LOW_SOCIAL",
			Timestamp:       time.Now(),
			MomentumCore:    75.0,
			TechnicalFactor: 70.0,
			VolumeFactor:    65.0,
			QualityFactor:   60.0,
			SocialFactor:    -30.0, // Very negative social signal
		},
		{
			Symbol:          "NORMAL_SOCIAL",
			Timestamp:       time.Now(),
			MomentumCore:    75.0,
			TechnicalFactor: 70.0,
			VolumeFactor:    65.0,
			QualityFactor:   60.0,
			SocialFactor:    5.0, // Normal social signal
		},
	}

	marketInputs := regime.RegimeInputs{
		RealizedVol7d: 0.45,
		PctAbove20MA:  0.50,
		BreadthThrust: 0.05,
		Timestamp:     time.Now(),
	}

	results, err := orchestrator.ProcessFactorsWithRegimeAdaptation(factorRows, marketInputs)
	if err != nil {
		t.Fatalf("failed to process factors: %v", err)
	}

	// Verify social cap is enforced
	for _, result := range results {
		if result.SocialResidual > 10.001 {
			t.Errorf("social residual exceeds +10 cap: %f for symbol %s", result.SocialResidual, result.Symbol)
		}
		if result.SocialResidual < -10.001 {
			t.Errorf("social residual exceeds -10 cap: %f for symbol %s", result.SocialResidual, result.Symbol)
		}
	}

	// Verify extreme signals were capped
	var highSocialResult, lowSocialResult, normalSocialResult factors.FactorRow
	for _, result := range results {
		switch result.Symbol {
		case "HIGH_SOCIAL":
			highSocialResult = result
		case "LOW_SOCIAL":
			lowSocialResult = result
		case "NORMAL_SOCIAL":
			normalSocialResult = result
		}
	}

	// High social signal should be capped at +10
	if highSocialResult.SocialResidual > 10.001 {
		t.Errorf("high social signal not properly capped: %f", highSocialResult.SocialResidual)
	}

	// Low social signal should be capped at -10
	if lowSocialResult.SocialResidual < -10.001 {
		t.Errorf("low social signal not properly capped: %f", lowSocialResult.SocialResidual)
	}

	// Normal social signal should be preserved (if within cap)
	if math.Abs(normalSocialResult.SocialResidual) > 10.0 {
		t.Errorf("normal social signal should be within cap: %f", normalSocialResult.SocialResidual)
	}
}

func TestRegimeOrchestrator_StatusReporting(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	weightMap := regime.GetDefaultWeightMap()

	orchestrator, err := regime.NewRegimeOrchestrator(detector, weightMap)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Trigger regime detection
	marketInputs := regime.RegimeInputs{
		RealizedVol7d: 0.35,
		PctAbove20MA:  0.55,
		BreadthThrust: 0.08,
		Timestamp:     time.Now(),
	}

	factorRows := []factors.FactorRow{
		{
			Symbol:          "TESTUSDT",
			Timestamp:       time.Now(),
			MomentumCore:    80.0,
			TechnicalFactor: 70.0,
			VolumeFactor:    60.0,
			QualityFactor:   50.0,
			SocialFactor:    5.0,
		},
	}

	_, err = orchestrator.ProcessFactorsWithRegimeAdaptation(factorRows, marketInputs)
	if err != nil {
		t.Fatalf("failed to process factors: %v", err)
	}

	// Get status report
	status := orchestrator.GetCurrentRegimeStatus()

	// Verify status structure
	expectedTopLevel := []string{"regime", "weights", "factor_engine"}
	for _, field := range expectedTopLevel {
		if _, exists := status[field]; !exists {
			t.Errorf("status missing top-level field: %s", field)
		}
	}

	// Verify regime section
	regimeSection, ok := status["regime"].(map[string]interface{})
	if !ok {
		t.Fatalf("regime section should be map[string]interface{}")
	}

	expectedRegimeFields := []string{"current", "last_detection", "detector_status"}
	for _, field := range expectedRegimeFields {
		if _, exists := regimeSection[field]; !exists {
			t.Errorf("regime section missing field: %s", field)
		}
	}

	// Verify weights section
	weightsSection, ok := status["weights"].(map[string]interface{})
	if !ok {
		t.Fatalf("weights section should be map[string]interface{}")
	}

	expectedWeightsFields := []string{"regime_weights", "factor_weights", "momentum_protected"}
	for _, field := range expectedWeightsFields {
		if _, exists := weightsSection[field]; !exists {
			t.Errorf("weights section missing field: %s", field)
		}
	}

	// Verify factor_engine section
	factorSection, ok := status["factor_engine"].(map[string]interface{})
	if !ok {
		t.Fatalf("factor_engine section should be map[string]interface{}")
	}

	expectedFactorFields := []string{"current_regime", "social_cap", "last_updated"}
	for _, field := range expectedFactorFields {
		if _, exists := factorSection[field]; !exists {
			t.Errorf("factor_engine section missing field: %s", field)
		}
	}
}

func TestRegimeOrchestrator_WeightSensitivityAnalysis(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	weightMap := regime.GetDefaultWeightMap()

	orchestrator, err := regime.NewRegimeOrchestrator(detector, weightMap)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	analysis := orchestrator.GetWeightSensitivityAnalysis()

	// Verify analysis structure
	expectedSections := []string{"regime_weight_differences", "momentum_protection", "social_cap_info"}
	for _, section := range expectedSections {
		if _, exists := analysis[section]; !exists {
			t.Errorf("sensitivity analysis missing section: %s", section)
		}
	}

	// Verify regime differences section
	differences, ok := analysis["regime_weight_differences"].(map[string]interface{})
	if !ok {
		t.Fatalf("regime_weight_differences should be map[string]interface{}")
	}

	expectedComparisons := []string{"trending_vs_choppy", "high_vol_vs_choppy"}
	for _, comparison := range expectedComparisons {
		if _, exists := differences[comparison]; !exists {
			t.Errorf("weight differences missing comparison: %s", comparison)
		}
	}

	// Verify momentum protection for all regimes
	protection, ok := analysis["momentum_protection"].(map[string]interface{})
	if !ok {
		t.Fatalf("momentum_protection should be map[string]interface{}")
	}

	expectedRegimes := []string{"trending_bull", "choppy", "high_vol"}
	for _, regimeType := range expectedRegimes {
		if _, exists := protection[regimeType]; !exists {
			t.Errorf("momentum protection missing regime: %s", regimeType)
		}
	}

	// Verify social cap info
	socialInfo, ok := analysis["social_cap_info"].(map[string]interface{})
	if !ok {
		t.Fatalf("social_cap_info should be map[string]interface{}")
	}

	capValue, ok := socialInfo["cap_value"].(float64)
	if !ok || capValue != 10.0 {
		t.Errorf("social cap value should be 10.0, got %v", socialInfo["cap_value"])
	}

	appliedOutside, ok := socialInfo["applied_outside"].(bool)
	if !ok || !appliedOutside {
		t.Errorf("social cap should be applied outside base allocation")
	}
}
