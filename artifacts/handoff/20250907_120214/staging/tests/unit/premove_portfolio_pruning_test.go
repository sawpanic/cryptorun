package unit

import (
	"testing"

	"github.com/sawpanic/cryptorun/src/domain/premove"
)

func TestPortfolioPruner_PrunePortfolio(t *testing.T) {
	// Test portfolio constraints configuration
	constraints := premove.PortfolioConstraints{
		PairwiseCorrMax:      0.65,
		SectorCaps:           map[string]int{"L1": 2, "DeFi": 2, "Infrastructure": 2},
		BetaBudgetToBTC:      2.0,
		MaxSinglePositionPct: 5.0,
		MaxTotalExposurePct:  20.0,
	}

	pruner := premove.NewPortfolioPruner(constraints)

	t.Run("correlation_constraint_enforcement", func(t *testing.T) {
		// Create candidates with high correlation
		candidates := []premove.Candidate{
			{Symbol: "ETHUSD", Score: 85.0, Sector: "L1", Beta: 1.2, ADV: 1000000, PassedGates: 2},
			{Symbol: "MATICUSD", Score: 80.0, Sector: "L1", Beta: 1.1, ADV: 800000, PassedGates: 2},
			{Symbol: "ADAUSD", Score: 75.0, Sector: "L1", Beta: 0.9, ADV: 600000, PassedGates: 2},
		}

		// Create correlation matrix with high correlation between ETH and MATIC
		correlationMatrix := &premove.CorrelationMatrix{
			Symbols: []string{"ETHUSD", "MATICUSD", "ADAUSD"},
			Matrix: map[string]map[string]float64{
				"ETHUSD": {
					"ETHUSD":   1.0,
					"MATICUSD": 0.75, // Above 0.65 threshold
					"ADAUSD":   0.45,
				},
				"MATICUSD": {
					"ETHUSD":   0.75,
					"MATICUSD": 1.0,
					"ADAUSD":   0.50,
				},
				"ADAUSD": {
					"ETHUSD":   0.45,
					"MATICUSD": 0.50,
					"ADAUSD":   1.0,
				},
			},
			Timeframe:    "4h",
			Observations: 100,
		}

		result := pruner.PrunePortfolio(candidates, correlationMatrix)

		// Should keep highest scoring (ETHUSD) and prune MATICUSD due to correlation
		if len(result.Kept) != 2 {
			t.Errorf("Expected 2 kept candidates, got %d", len(result.Kept))
		}

		if result.Metrics.PrunedByCorrelation != 1 {
			t.Errorf("Expected 1 pruned by correlation, got %d", result.Metrics.PrunedByCorrelation)
		}

		// Verify highest score is kept
		if result.Kept[0].Symbol != "ETHUSD" {
			t.Errorf("Expected ETHUSD to be kept (highest score), got %s", result.Kept[0].Symbol)
		}

		// Check pruning reason
		prunedSymbols := make(map[string]string)
		for _, pruned := range result.Pruned {
			prunedSymbols[pruned.Symbol] = pruned.Reason
		}

		if _, exists := prunedSymbols["MATICUSD"]; !exists {
			t.Error("MATICUSD should be pruned due to correlation")
		}
	})

	t.Run("sector_cap_enforcement", func(t *testing.T) {
		// Create 3 L1 candidates (exceeds sector cap of 2)
		candidates := []premove.Candidate{
			{Symbol: "ETHUSD", Score: 85.0, Sector: "L1", Beta: 1.2, ADV: 1000000, PassedGates: 2},
			{Symbol: "MATICUSD", Score: 80.0, Sector: "L1", Beta: 1.1, ADV: 800000, PassedGates: 2},
			{Symbol: "AVAXUSD", Score: 75.0, Sector: "L1", Beta: 0.9, ADV: 600000, PassedGates: 2},
		}

		// Low correlation matrix
		correlationMatrix := &premove.CorrelationMatrix{
			Symbols: []string{"ETHUSD", "MATICUSD", "AVAXUSD"},
			Matrix: map[string]map[string]float64{
				"ETHUSD": {
					"ETHUSD":   1.0,
					"MATICUSD": 0.30,
					"AVAXUSD":  0.25,
				},
				"MATICUSD": {
					"ETHUSD":   0.30,
					"MATICUSD": 1.0,
					"AVAXUSD":  0.35,
				},
				"AVAXUSD": {
					"ETHUSD":   0.25,
					"MATICUSD": 0.35,
					"AVAXUSD":  1.0,
				},
			},
			Timeframe:    "4h",
			Observations: 100,
		}

		result := pruner.PrunePortfolio(candidates, correlationMatrix)

		// Should keep 2 (sector cap) and prune 1
		if len(result.Kept) != 2 {
			t.Errorf("Expected 2 kept candidates (sector cap), got %d", len(result.Kept))
		}

		if result.Metrics.PrunedBySector != 1 {
			t.Errorf("Expected 1 pruned by sector cap, got %d", result.Metrics.PrunedBySector)
		}

		// Should keep the highest scoring two
		expectedKept := []string{"ETHUSD", "MATICUSD"}
		keptSymbols := make([]string, len(result.Kept))
		for i, kept := range result.Kept {
			keptSymbols[i] = kept.Symbol
		}

		for _, expected := range expectedKept {
			found := false
			for _, actual := range keptSymbols {
				if actual == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected %s to be kept (top scores), but was not found", expected)
			}
		}
	})

	t.Run("beta_budget_enforcement", func(t *testing.T) {
		// Create candidates that exceed beta budget
		candidates := []premove.Candidate{
			{Symbol: "ETHUSD", Score: 85.0, Sector: "L1", Beta: 1.5, ADV: 1000000, PassedGates: 2},
			{Symbol: "LINKUSD", Score: 80.0, Sector: "Infrastructure", Beta: 1.2, ADV: 800000, PassedGates: 2},
			{Symbol: "ADAUSD", Score: 75.0, Sector: "L1", Beta: 0.8, ADV: 600000, PassedGates: 2}, // Total would be 3.5 > 2.0
		}

		// Low correlation matrix
		correlationMatrix := &premove.CorrelationMatrix{
			Symbols: []string{"ETHUSD", "LINKUSD", "ADAUSD"},
			Matrix: map[string]map[string]float64{
				"ETHUSD":  {"ETHUSD": 1.0, "LINKUSD": 0.30, "ADAUSD": 0.25},
				"LINKUSD": {"ETHUSD": 0.30, "LINKUSD": 1.0, "ADAUSD": 0.35},
				"ADAUSD":  {"ETHUSD": 0.25, "LINKUSD": 0.35, "ADAUSD": 1.0},
			},
			Timeframe:    "4h",
			Observations: 100,
		}

		result := pruner.PrunePortfolio(candidates, correlationMatrix)

		// Should prune at least one due to beta budget
		if result.Metrics.PrunedByBeta == 0 {
			t.Error("Expected at least 1 candidate pruned by beta budget")
		}

		// Final beta utilization should be <= 100%
		if result.Metrics.FinalBetaUtilization > 100.0 {
			t.Errorf("Beta utilization %.2f%% exceeds 100%%", result.Metrics.FinalBetaUtilization)
		}
	})

	t.Run("metrics_accuracy", func(t *testing.T) {
		candidates := []premove.Candidate{
			{Symbol: "ETHUSD", Score: 85.0, Sector: "L1", Beta: 1.2, ADV: 1000000, PassedGates: 2},
			{Symbol: "BTCUSD", Score: 80.0, Sector: "L1", Beta: 1.0, ADV: 800000, PassedGates: 2},
		}

		result := pruner.PrunePortfolio(candidates, nil) // No correlation matrix

		// Check metrics consistency
		totalCandidates := result.Metrics.TotalInput
		totalKept := result.Metrics.TotalKept
		totalPruned := result.Metrics.TotalPruned

		if totalCandidates != len(candidates) {
			t.Errorf("Total input %d != candidates length %d", totalCandidates, len(candidates))
		}

		if totalKept+totalPruned != totalCandidates {
			t.Errorf("Kept (%d) + Pruned (%d) != Total (%d)", totalKept, totalPruned, totalCandidates)
		}

		if len(result.Kept) != totalKept {
			t.Errorf("Kept array length %d != metrics total kept %d", len(result.Kept), totalKept)
		}

		if len(result.Pruned) != totalPruned {
			t.Errorf("Pruned array length %d != metrics total pruned %d", len(result.Pruned), totalPruned)
		}
	})

	t.Run("score_based_prioritization", func(t *testing.T) {
		// Test that higher scores are prioritized when constraints are tight
		candidates := []premove.Candidate{
			{Symbol: "LOWSCORE", Score: 65.0, Sector: "L1", Beta: 0.8, ADV: 500000, PassedGates: 2},
			{Symbol: "HIGHSCORE", Score: 90.0, Sector: "L1", Beta: 0.9, ADV: 600000, PassedGates: 2},
			{Symbol: "MIDSCORE", Score: 75.0, Sector: "L1", Beta: 0.7, ADV: 400000, PassedGates: 2},
		}

		result := pruner.PrunePortfolio(candidates, nil)

		// With sector cap of 2, should keep the 2 highest scoring
		if len(result.Kept) != 2 {
			t.Errorf("Expected 2 kept with sector cap, got %d", len(result.Kept))
		}

		// Verify highest scores are kept
		if result.Kept[0].Score < result.Kept[1].Score {
			t.Error("Kept candidates should be sorted by score (highest first)")
		}

		if result.Kept[0].Symbol != "HIGHSCORE" {
			t.Errorf("Expected HIGHSCORE to be first, got %s", result.Kept[0].Symbol)
		}

		// Verify lowest score is pruned
		prunedFound := false
		for _, pruned := range result.Pruned {
			if pruned.Symbol == "LOWSCORE" {
				prunedFound = true
				break
			}
		}
		if !prunedFound {
			t.Error("LOWSCORE should have been pruned (lowest score)")
		}
	})

	t.Run("empty_candidates", func(t *testing.T) {
		result := pruner.PrunePortfolio([]premove.Candidate{}, nil)

		if len(result.Kept) != 0 {
			t.Errorf("Expected 0 kept candidates for empty input, got %d", len(result.Kept))
		}

		if len(result.Pruned) != 0 {
			t.Errorf("Expected 0 pruned candidates for empty input, got %d", len(result.Pruned))
		}

		if result.Metrics.TotalInput != 0 {
			t.Errorf("Expected 0 total input for empty candidates, got %d", result.Metrics.TotalInput)
		}
	})
}

func TestPortfolioPruner_CorrelationConstraintEdgeCases(t *testing.T) {
	constraints := premove.PortfolioConstraints{
		PairwiseCorrMax:      0.65,
		SectorCaps:           map[string]int{"L1": 5}, // High cap to focus on correlation
		BetaBudgetToBTC:      10.0,                    // High budget to focus on correlation
		MaxSinglePositionPct: 10.0,
		MaxTotalExposurePct:  50.0,
	}

	pruner := premove.NewPortfolioPruner(constraints)

	t.Run("nil_correlation_matrix", func(t *testing.T) {
		candidates := []premove.Candidate{
			{Symbol: "ETHUSD", Score: 85.0, Sector: "L1", Beta: 1.2, ADV: 1000000, PassedGates: 2},
			{Symbol: "BTCUSD", Score: 80.0, Sector: "L1", Beta: 1.0, ADV: 800000, PassedGates: 2},
		}

		result := pruner.PrunePortfolio(candidates, nil)

		// Should keep all candidates since correlation check is skipped
		if len(result.Kept) != 2 {
			t.Errorf("Expected 2 kept candidates with nil correlation matrix, got %d", len(result.Kept))
		}

		if result.Metrics.PrunedByCorrelation != 0 {
			t.Errorf("Expected 0 pruned by correlation with nil matrix, got %d", result.Metrics.PrunedByCorrelation)
		}
	})

	t.Run("missing_correlation_data", func(t *testing.T) {
		candidates := []premove.Candidate{
			{Symbol: "ETHUSD", Score: 85.0, Sector: "L1", Beta: 1.2, ADV: 1000000, PassedGates: 2},
			{Symbol: "UNKNOWNUSD", Score: 80.0, Sector: "L1", Beta: 1.0, ADV: 800000, PassedGates: 2},
		}

		// Correlation matrix missing UNKNOWNUSD
		correlationMatrix := &premove.CorrelationMatrix{
			Symbols: []string{"ETHUSD"},
			Matrix: map[string]map[string]float64{
				"ETHUSD": {"ETHUSD": 1.0},
			},
			Timeframe:    "4h",
			Observations: 100,
		}

		result := pruner.PrunePortfolio(candidates, correlationMatrix)

		// Should keep both since correlation cannot be calculated for UNKNOWNUSD
		if len(result.Kept) != 2 {
			t.Errorf("Expected 2 kept candidates with missing correlation data, got %d", len(result.Kept))
		}
	})

	t.Run("boundary_correlation_values", func(t *testing.T) {
		candidates := []premove.Candidate{
			{Symbol: "ETHUSD", Score: 85.0, Sector: "L1", Beta: 1.2, ADV: 1000000, PassedGates: 2},
			{Symbol: "EXACTLIMIT", Score: 80.0, Sector: "L1", Beta: 1.0, ADV: 800000, PassedGates: 2},
			{Symbol: "OVERLIMIT", Score: 75.0, Sector: "L1", Beta: 0.8, ADV: 600000, PassedGates: 2},
		}

		// Test exact boundary (0.65) and slightly over
		correlationMatrix := &premove.CorrelationMatrix{
			Symbols: []string{"ETHUSD", "EXACTLIMIT", "OVERLIMIT"},
			Matrix: map[string]map[string]float64{
				"ETHUSD": {
					"ETHUSD":     1.0,
					"EXACTLIMIT": 0.65,  // Exactly at limit
					"OVERLIMIT":  0.651, // Just over limit
				},
				"EXACTLIMIT": {
					"ETHUSD":     0.65,
					"EXACTLIMIT": 1.0,
					"OVERLIMIT":  0.30,
				},
				"OVERLIMIT": {
					"ETHUSD":     0.651,
					"EXACTLIMIT": 0.30,
					"OVERLIMIT":  1.0,
				},
			},
			Timeframe:    "4h",
			Observations: 100,
		}

		result := pruner.PrunePortfolio(candidates, correlationMatrix)

		// Should prune OVERLIMIT (0.651 > 0.65) but keep EXACTLIMIT (0.65 = 0.65)
		prunedSymbols := make(map[string]bool)
		for _, pruned := range result.Pruned {
			prunedSymbols[pruned.Symbol] = true
		}

		if !prunedSymbols["OVERLIMIT"] {
			t.Error("OVERLIMIT should be pruned (correlation 0.651 > 0.65)")
		}

		keptSymbols := make(map[string]bool)
		for _, kept := range result.Kept {
			keptSymbols[kept.Symbol] = true
		}

		if !keptSymbols["EXACTLIMIT"] {
			t.Error("EXACTLIMIT should be kept (correlation 0.65 <= 0.65)")
		}
	})
}
