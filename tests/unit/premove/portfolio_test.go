package premove

import (
	"testing"

	"cryptorun/src/application/premove"
	"cryptorun/src/domain/premove/portfolio"
)

// Tests for actual implemented components

func TestPortfolioPruner_BasicFunctionality(t *testing.T) {
	t.Run("create_pruner_with_defaults", func(t *testing.T) {
		pruner := portfolio.NewPruner()

		if pruner.PairwiseCorrMax != 0.65 {
			t.Errorf("Expected default pairwise correlation 0.65, got %.2f", pruner.PairwiseCorrMax)
		}

		if pruner.BetaBudgetToBTC != 2.0 {
			t.Errorf("Expected default beta budget 2.0, got %.2f", pruner.BetaBudgetToBTC)
		}

		if pruner.MaxSinglePositionPct != 5.0 {
			t.Errorf("Expected default single position limit 5.0%%, got %.1f%%", pruner.MaxSinglePositionPct)
		}

		if pruner.MaxTotalExposurePct != 20.0 {
			t.Errorf("Expected default total exposure limit 20.0%%, got %.1f%%", pruner.MaxTotalExposurePct)
		}
	})

	t.Run("prune_empty_candidates", func(t *testing.T) {
		pruner := portfolio.NewPruner()
		candidates := []portfolio.PruneCandidate{}

		result := pruner.Prune(candidates, nil)

		if result.Summary.TotalCandidates != 0 {
			t.Errorf("Expected 0 total candidates, got %d", result.Summary.TotalCandidates)
		}

		if len(result.Accepted) != 0 {
			t.Errorf("Expected 0 accepted candidates, got %d", len(result.Accepted))
		}
	})

	t.Run("prune_single_candidate", func(t *testing.T) {
		pruner := portfolio.NewPruner()
		candidates := []portfolio.PruneCandidate{
			{
				Symbol:      "BTC-USD",
				Score:       80.0,
				PassedGates: 3,
				Sector:      "Layer1",
				Beta:        1.0,
				ADV:         1000000,
			},
		}

		result := pruner.Prune(candidates, nil)

		if result.Summary.TotalCandidates != 1 {
			t.Errorf("Expected 1 total candidate, got %d", result.Summary.TotalCandidates)
		}

		if len(result.Accepted) != 1 {
			t.Errorf("Expected 1 accepted candidate, got %d", len(result.Accepted))
		}

		if result.Summary.AcceptedCount != 1 {
			t.Errorf("Expected 1 accepted in summary, got %d", result.Summary.AcceptedCount)
		}
	})

	t.Run("prune_beta_budget_exceeded", func(t *testing.T) {
		pruner := portfolio.NewPruner()
		candidates := []portfolio.PruneCandidate{
			{Symbol: "BTC-USD", Score: 85.0, Beta: 1.5, Sector: "Layer1"},
			{Symbol: "ETH-USD", Score: 80.0, Beta: 1.0, Sector: "Layer1"},
		}

		result := pruner.Prune(candidates, nil)

		// First candidate (highest score) should be accepted
		if len(result.Accepted) != 1 || result.Accepted[0].Symbol != "BTC-USD" {
			t.Errorf("Expected BTC-USD to be accepted first")
		}

		// Second candidate should be rejected due to beta budget
		if len(result.Rejected) != 1 || result.Rejected[0].Symbol != "ETH-USD" {
			t.Errorf("Expected ETH-USD to be rejected due to beta budget")
		}

		if result.RejectionReasons["ETH-USD"] == "" {
			t.Error("Expected rejection reason for ETH-USD")
		}
	})

	t.Run("prune_sector_caps", func(t *testing.T) {
		sectorCaps := map[string]int{"Layer1": 1}
		pruner := portfolio.NewPrunerWithConstraints(0.65, sectorCaps, 3.0, 5.0, 20.0)

		candidates := []portfolio.PruneCandidate{
			{Symbol: "BTC-USD", Score: 85.0, Beta: 0.8, Sector: "Layer1"},
			{Symbol: "ETH-USD", Score: 80.0, Beta: 0.9, Sector: "Layer1"},
		}

		result := pruner.Prune(candidates, nil)

		if len(result.Accepted) != 1 {
			t.Errorf("Expected 1 accepted candidate, got %d", len(result.Accepted))
		}

		if result.Accepted[0].Symbol != "BTC-USD" {
			t.Errorf("Expected BTC-USD (highest score) to be accepted")
		}

		if len(result.Rejected) != 1 {
			t.Errorf("Expected 1 rejected candidate, got %d", len(result.Rejected))
		}
	})
}

func TestPortfolioManager_Integration(t *testing.T) {
	t.Run("create_portfolio_manager", func(t *testing.T) {
		pm := premove.NewPortfolioManager()

		if pm == nil {
			t.Error("Expected portfolio manager to be created")
		}

		status := pm.GetPortfolioStatus()
		if status == nil {
			t.Error("Expected portfolio status to be available")
		}
	})

	t.Run("prune_post_gates", func(t *testing.T) {
		pm := premove.NewPortfolioManager()

		candidates := []portfolio.PruneCandidate{
			{Symbol: "BTC-USD", Score: 85.0, Beta: 1.0, Sector: "Layer1"},
			{Symbol: "ETH-USD", Score: 80.0, Beta: 1.2, Sector: "Layer1"},
		}

		result, err := pm.PrunePostGates(candidates)
		if err != nil {
			t.Errorf("Expected pruning to succeed, got error: %v", err)
		}

		if result == nil {
			t.Error("Expected prune result to be returned")
		}
	})

	t.Run("validate_portfolio", func(t *testing.T) {
		pm := premove.NewPortfolioManager()

		positions := []portfolio.PruneCandidate{
			{Symbol: "BTC-USD", Beta: 0.5, Sector: "Layer1"},
			{Symbol: "ETH-USD", Beta: 0.8, Sector: "Layer1"},
		}

		err := pm.ValidatePortfolio(positions)
		if err != nil {
			t.Errorf("Expected portfolio validation to pass, got error: %v", err)
		}
	})
}

func TestCorrelationProvider(t *testing.T) {
	t.Run("simple_correlation_provider", func(t *testing.T) {
		provider := premove.NewSimpleCorrelationProvider()

		// Test self-correlation
		corr, exists := provider.GetCorrelation("BTC-USD", "BTC-USD")
		if !exists || corr != 1.0 {
			t.Errorf("Expected self-correlation of 1.0, got %.2f (exists: %v)", corr, exists)
		}

		// Test predefined correlation
		corr, exists = provider.GetCorrelation("BTC-USD", "ETH-USD")
		if !exists || corr != 0.7 {
			t.Errorf("Expected BTC-ETH correlation of 0.7, got %.2f (exists: %v)", corr, exists)
		}

		// Test reverse lookup
		corr2, exists2 := provider.GetCorrelation("ETH-USD", "BTC-USD")
		if !exists2 || corr2 != 0.7 {
			t.Errorf("Expected reverse correlation to work, got %.2f (exists: %v)", corr2, exists2)
		}

		// Test non-existent pair
		_, exists = provider.GetCorrelation("BTC-USD", "UNKNOWN")
		if exists {
			t.Error("Expected unknown correlation pair to return false")
		}
	})
}

// Existing advanced tests below (kept for future implementation)

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
			BetaAdjustment:      "risk_parity",
			SectorRebalancing:   true,
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
