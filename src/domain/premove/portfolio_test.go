package premove

import (
	"testing"
)

func TestPortfolioPruner_BasicFunctionality(t *testing.T) {
	constraints := PortfolioConstraints{
		PairwiseCorrMax:      0.65,
		SectorCaps:           map[string]int{"L1": 2, "DeFi": 2},
		BetaBudgetToBTC:      2.0,
		MaxSinglePositionPct: 5.0,
		MaxTotalExposurePct:  20.0,
	}

	pruner := NewPortfolioPruner(constraints)

	t.Run("basic_pruning", func(t *testing.T) {
		candidates := []Candidate{
			{Symbol: "ETHUSD", Score: 85.0, Sector: "L1", Beta: 1.2, ADV: 1000000, PassedGates: 2},
			{Symbol: "BTCUSD", Score: 80.0, Sector: "L1", Beta: 1.0, ADV: 800000, PassedGates: 2},
			{Symbol: "UNIUSD", Score: 75.0, Sector: "DeFi", Beta: 0.8, ADV: 600000, PassedGates: 2},
		}

		result := pruner.PrunePortfolio(candidates, nil)

		if result == nil {
			t.Fatal("PrunePortfolio returned nil result")
		}

		// Should keep all candidates since no correlation matrix provided
		if len(result.Kept) == 0 {
			t.Error("Expected some candidates to be kept")
		}

		// Check metrics consistency
		if result.Metrics.TotalInput != len(candidates) {
			t.Errorf("Expected TotalInput %d, got %d", len(candidates), result.Metrics.TotalInput)
		}

		totalProcessed := result.Metrics.TotalKept + result.Metrics.TotalPruned
		if totalProcessed != result.Metrics.TotalInput {
			t.Errorf("Kept (%d) + Pruned (%d) != Total (%d)",
				result.Metrics.TotalKept, result.Metrics.TotalPruned, result.Metrics.TotalInput)
		}
	})

	t.Run("sector_caps", func(t *testing.T) {
		// Create 3 L1 tokens (exceeds cap of 2)
		candidates := []Candidate{
			{Symbol: "ETHUSD", Score: 85.0, Sector: "L1", Beta: 0.5, ADV: 1000000, PassedGates: 2},
			{Symbol: "MATICUSD", Score: 80.0, Sector: "L1", Beta: 0.5, ADV: 800000, PassedGates: 2},
			{Symbol: "AVAXUSD", Score: 75.0, Sector: "L1", Beta: 0.5, ADV: 600000, PassedGates: 2},
		}

		result := pruner.PrunePortfolio(candidates, nil)

		// Should keep exactly 2 due to L1 sector cap
		if len(result.Kept) != 2 {
			t.Errorf("Expected 2 kept (sector cap), got %d", len(result.Kept))
		}

		if result.Metrics.PrunedBySector != 1 {
			t.Errorf("Expected 1 pruned by sector, got %d", result.Metrics.PrunedBySector)
		}

		// Should keep highest scoring
		if result.Kept[0].Score < result.Kept[1].Score {
			t.Error("Kept candidates should be sorted by score")
		}
	})

	t.Run("beta_budget", func(t *testing.T) {
		// Create candidates that would exceed beta budget of 2.0
		candidates := []Candidate{
			{Symbol: "ETHUSD", Score: 85.0, Sector: "L1", Beta: 1.5, ADV: 1000000, PassedGates: 2},
			{Symbol: "LINKUSD", Score: 80.0, Sector: "Infrastructure", Beta: 1.2, ADV: 800000, PassedGates: 2},
		}

		result := pruner.PrunePortfolio(candidates, nil)

		// Should prune at least one due to beta budget (1.5 + 1.2 = 2.7 > 2.0)
		if result.Metrics.PrunedByBeta == 0 {
			t.Error("Expected at least one candidate pruned by beta budget")
		}

		if result.Metrics.FinalBetaUtilization > 100.0 {
			t.Errorf("Beta utilization %.2f%% should not exceed 100%%", result.Metrics.FinalBetaUtilization)
		}
	})

	t.Run("empty_input", func(t *testing.T) {
		result := pruner.PrunePortfolio([]Candidate{}, nil)

		if result.Metrics.TotalInput != 0 {
			t.Errorf("Expected 0 total input, got %d", result.Metrics.TotalInput)
		}

		if len(result.Kept) != 0 || len(result.Pruned) != 0 {
			t.Error("Expected empty kept and pruned arrays for empty input")
		}
	})
}

func TestPortfolioPruner_CorrelationConstraints(t *testing.T) {
	constraints := PortfolioConstraints{
		PairwiseCorrMax:      0.65,
		SectorCaps:           map[string]int{"L1": 5}, // High to focus on correlation
		BetaBudgetToBTC:      10.0,                    // High to focus on correlation
		MaxSinglePositionPct: 10.0,
		MaxTotalExposurePct:  50.0,
	}

	pruner := NewPortfolioPruner(constraints)

	t.Run("high_correlation_pruning", func(t *testing.T) {
		candidates := []Candidate{
			{Symbol: "ETHUSD", Score: 85.0, Sector: "L1", Beta: 0.5, ADV: 1000000, PassedGates: 2},
			{Symbol: "MATICUSD", Score: 80.0, Sector: "L1", Beta: 0.5, ADV: 800000, PassedGates: 2},
		}

		// High correlation matrix
		correlationMatrix := &CorrelationMatrix{
			Symbols: []string{"ETHUSD", "MATICUSD"},
			Matrix: map[string]map[string]float64{
				"ETHUSD": {
					"ETHUSD":   1.0,
					"MATICUSD": 0.75, // Above 0.65 threshold
				},
				"MATICUSD": {
					"ETHUSD":   0.75,
					"MATICUSD": 1.0,
				},
			},
		}

		result := pruner.PrunePortfolio(candidates, correlationMatrix)

		// Should keep only one due to high correlation
		if len(result.Kept) != 1 {
			t.Errorf("Expected 1 kept (correlation constraint), got %d", len(result.Kept))
		}

		if result.Metrics.PrunedByCorrelation != 1 {
			t.Errorf("Expected 1 pruned by correlation, got %d", result.Metrics.PrunedByCorrelation)
		}

		// Should keep the higher scoring one
		if result.Kept[0].Symbol != "ETHUSD" {
			t.Errorf("Expected ETHUSD (higher score) to be kept, got %s", result.Kept[0].Symbol)
		}
	})
}
