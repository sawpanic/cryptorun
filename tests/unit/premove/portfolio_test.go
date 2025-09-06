package premove

import (
	"testing"

	"cryptorun/src/application/premove"
)

func TestPortfolioPruner_ConstraintEnforcement(t *testing.T) {
	t.Run("correlation_matrix_validation", func(t *testing.T) {
		// This test expects a correlation validator that doesn't exist yet
		validator := premove.NewCorrelationValidator()
		
		// Invalid correlation matrix (negative correlation > 1.0)
		invalidMatrix := map[string]map[string]float64{
			"BTCUSD": {"BTCUSD": 1.0, "ETHUSD": 1.5}, // Invalid: > 1.0
			"ETHUSD": {"BTCUSD": 1.5, "ETHUSD": 1.0},
		}
		
		err := validator.ValidateMatrix(invalidMatrix)
		if err == nil {
			t.Error("Expected validation error for correlation > 1.0")
		}
	})

	t.Run("advanced_pruning_strategies", func(t *testing.T) {
		// This test expects an advanced pruner that doesn't exist yet
		pruner := premove.NewAdvancedPortfolioPruner(premove.AdvancedPruningConfig{
			CorrelationStrategy: "eigenvalue_decomposition",
			BetaAdjustment:     "risk_parity",
			SectorRebalancing:  true,
		})
		
		candidates := []premove.PruningCandidate{
			{Symbol: "BTCUSD", Score: 85.0, Beta: 1.0, Sector: "L1"},
			{Symbol: "ETHUSD", Score: 80.0, Beta: 1.2, Sector: "L1"},
			{Symbol: "SOLUSD", Score: 75.0, Beta: 1.5, Sector: "L1"},
		}
		
		result, err := pruner.PruneWithStrategy(candidates)
		if err != nil {
			t.Errorf("Expected pruning to succeed, got: %v", err)
		}
		
		// Should use eigenvalue decomposition for better correlation handling
		if len(result.Accepted) == 0 {
			t.Error("Advanced pruner should accept some candidates")
		}
	})

	t.Run("dynamic_correlation_windows", func(t *testing.T) {
		// This test expects correlation calculator with dynamic windows
		calculator := premove.NewDynamicCorrelationCalculator()
		
		// Mock price data for different timeframes
		priceData := map[string][]float64{
			"BTCUSD": {45000, 45100, 45200, 45150, 45300},
			"ETHUSD": {3000, 3010, 3020, 3015, 3030},
		}
		
		// Calculate correlation for different windows
		shortCorr, err := calculator.CalculateCorrelation(priceData, "1h", 24)
		if err != nil {
			t.Errorf("Short correlation calculation failed: %v", err)
		}
		
		longCorr, err := calculator.CalculateCorrelation(priceData, "4h", 168)
		if err != nil {
			t.Errorf("Long correlation calculation failed: %v", err)
		}
		
		// Different windows should potentially give different correlations
		if shortCorr == longCorr {
			t.Log("Note: Short and long correlation are equal - may be expected")
		}
	})

	t.Run("sector_rotation_detection", func(t *testing.T) {
		// This test expects sector rotation detector
		detector := premove.NewSectorRotationDetector()
		
		historicalFlows := []premove.SectorFlow{
			{Sector: "L1", NetFlow: -1000000, Timestamp: "2025-09-06T10:00:00Z"},
			{Sector: "DeFi", NetFlow: 500000, Timestamp: "2025-09-06T10:00:00Z"},
			{Sector: "Gaming", NetFlow: 300000, Timestamp: "2025-09-06T10:00:00Z"},
		}
		
		rotation := detector.DetectRotation(historicalFlows)
		if rotation.FromSector == "" || rotation.ToSector == "" {
			t.Error("Expected sector rotation to be detected")
		}
		
		if rotation.Confidence < 0.7 {
			t.Errorf("Expected high confidence rotation, got %.2f", rotation.Confidence)
		}
	})
}

func TestPortfolioPruner_RiskMetrics(t *testing.T) {
	t.Run("value_at_risk_calculation", func(t *testing.T) {
		// This test expects VaR calculator
		calculator := premove.NewVaRCalculator(premove.VaRConfig{
			ConfidenceLevel: 0.95,
			HoldingPeriod:   1, // 1 day
			Method:          "monte_carlo",
		})
		
		portfolio := []premove.Position{
			{Symbol: "BTCUSD", Weight: 0.6, Volatility: 0.04},
			{Symbol: "ETHUSD", Weight: 0.4, Volatility: 0.05},
		}
		
		var_, err := calculator.CalculateVaR(portfolio)
		if err != nil {
			t.Errorf("VaR calculation failed: %v", err)
		}
		
		if var_ <= 0 {
			t.Errorf("Expected positive VaR, got %.4f", var_)
		}
	})

	t.Run("expected_shortfall", func(t *testing.T) {
		// This test expects Expected Shortfall calculator
		calculator := premove.NewExpectedShortfallCalculator()
		
		portfolio := premove.Portfolio{
			Positions: []premove.Position{
				{Symbol: "BTCUSD", Weight: 0.5, ExpectedReturn: 0.001},
				{Symbol: "ETHUSD", Weight: 0.5, ExpectedReturn: 0.0015},
			},
			CorrelationMatrix: map[string]map[string]float64{
				"BTCUSD": {"BTCUSD": 1.0, "ETHUSD": 0.7},
				"ETHUSD": {"BTCUSD": 0.7, "ETHUSD": 1.0},
			},
		}
		
		es, err := calculator.CalculateExpectedShortfall(portfolio, 0.95)
		if err != nil {
			t.Errorf("Expected Shortfall calculation failed: %v", err)
		}
		
		if es >= 0 {
			t.Errorf("Expected negative Expected Shortfall, got %.4f", es)
		}
	})

	t.Run("maximum_drawdown_estimation", func(t *testing.T) {
		// This test expects drawdown estimator
		estimator := premove.NewDrawdownEstimator()
		
		// Historical returns for drawdown estimation
		returns := []float64{0.02, -0.01, 0.03, -0.05, 0.01, -0.03, 0.04}
		
		maxDrawdown, err := estimator.EstimateMaxDrawdown(returns, 0.95)
		if err != nil {
			t.Errorf("Max drawdown estimation failed: %v", err)
		}
		
		if maxDrawdown >= 0 {
			t.Errorf("Expected negative max drawdown, got %.4f", maxDrawdown)
		}
	})
}

func TestPortfolioPruner_PerformanceAttribution(t *testing.T) {
	t.Run("factor_attribution", func(t *testing.T) {
		// This test expects factor attribution analyzer
		analyzer := premove.NewFactorAttributionAnalyzer()
		
		portfolio := premove.Portfolio{
			Positions: []premove.Position{
				{Symbol: "BTCUSD", Weight: 0.4, FactorLoadings: map[string]float64{"momentum": 0.8, "volatility": -0.3}},
				{Symbol: "ETHUSD", Weight: 0.6, FactorLoadings: map[string]float64{"momentum": 0.7, "volatility": -0.2}},
			},
		}
		
		factorReturns := map[string]float64{
			"momentum":   0.02,
			"volatility": -0.01,
		}
		
		attribution, err := analyzer.AttributeReturns(portfolio, factorReturns)
		if err != nil {
			t.Errorf("Factor attribution failed: %v", err)
		}
		
		if len(attribution.FactorContributions) == 0 {
			t.Error("Expected factor contributions to be calculated")
		}
		
		totalAttribution := 0.0
		for _, contrib := range attribution.FactorContributions {
			totalAttribution += contrib
		}
		
		if totalAttribution == 0 {
			t.Error("Expected non-zero total factor attribution")
		}
	})

	t.Run("risk_decomposition", func(t *testing.T) {
		// This test expects risk decomposition analyzer
		analyzer := premove.NewRiskDecompositionAnalyzer()
		
		portfolio := premove.Portfolio{
			Positions: []premove.Position{
				{Symbol: "BTCUSD", Weight: 0.3, Volatility: 0.04},
				{Symbol: "ETHUSD", Weight: 0.3, Volatility: 0.05},
				{Symbol: "SOLUSD", Weight: 0.4, Volatility: 0.07},
			},
			CorrelationMatrix: map[string]map[string]float64{
				"BTCUSD": {"BTCUSD": 1.0, "ETHUSD": 0.7, "SOLUSD": 0.6},
				"ETHUSD": {"BTCUSD": 0.7, "ETHUSD": 1.0, "SOLUSD": 0.8},
				"SOLUSD": {"BTCUSD": 0.6, "ETHUSD": 0.8, "SOLUSD": 1.0},
			},
		}
		
		decomposition, err := analyzer.DecomposeRisk(portfolio)
		if err != nil {
			t.Errorf("Risk decomposition failed: %v", err)
		}
		
		if decomposition.TotalRisk <= 0 {
			t.Errorf("Expected positive total risk, got %.4f", decomposition.TotalRisk)
		}
		
		if len(decomposition.ComponentRisks) != 3 {
			t.Errorf("Expected 3 component risks, got %d", len(decomposition.ComponentRisks))
		}
	})
}