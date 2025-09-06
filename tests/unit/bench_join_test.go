package unit

import (
	"strings"
	"testing"

	"cryptorun/internal/bench"
)

// TestSymbolOverlap tests symbol overlap calculation
func TestSymbolOverlap(t *testing.T) {
	tests := []struct {
		name        string
		gainers     []string
		scan        []string
		expectedMin float64 // Minimum expected score
		expectedMax float64 // Maximum expected score
	}{
		{
			name:        "perfect overlap",
			gainers:     []string{"BTC", "ETH", "ADA"},
			scan:        []string{"BTC", "ETH", "ADA"},
			expectedMin: 1.0,
			expectedMax: 1.0,
		},
		{
			name:        "no overlap",
			gainers:     []string{"BTC", "ETH", "ADA"},
			scan:        []string{"SOL", "DOT", "LINK"},
			expectedMin: 0.0,
			expectedMax: 0.0,
		},
		{
			name:        "partial overlap",
			gainers:     []string{"BTC", "ETH", "ADA", "SOL"},
			scan:        []string{"BTC", "ETH", "DOT", "LINK"},
			expectedMin: 0.3, // 2 common / 6 total = 0.33
			expectedMax: 0.4,
		},
		{
			name:        "empty scan results",
			gainers:     []string{"BTC", "ETH", "ADA"},
			scan:        []string{},
			expectedMin: 0.0,
			expectedMax: 0.0,
		},
		{
			name:        "case insensitive",
			gainers:     []string{"btc", "eth", "ada"},
			scan:        []string{"BTC", "ETH", "ADA"},
			expectedMin: 1.0,
			expectedMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock top gainers
			gainers := make([]bench.TopGainerResult, len(tt.gainers))
			for i, symbol := range tt.gainers {
				gainers[i] = bench.TopGainerResult{
					Symbol: symbol,
				}
			}

			weights := bench.DefaultScoreWeights()
			score := bench.CalculateCompositeScore("test", gainers, tt.scan, weights)

			if score.SymbolOverlap < tt.expectedMin || score.SymbolOverlap > tt.expectedMax {
				t.Errorf("Symbol overlap %.3f not in expected range [%.3f, %.3f]",
					score.SymbolOverlap, tt.expectedMin, tt.expectedMax)
			}

			// Validate details
			if len(tt.gainers) > 0 && len(tt.scan) > 0 {
				if len(score.Details.CommonSymbols) == 0 && score.SymbolOverlap > 0 {
					t.Errorf("Expected common symbols when overlap > 0")
				}
			}
		})
	}
}

// TestRankCorrelation tests rank correlation calculation
func TestRankCorrelation(t *testing.T) {
	tests := []struct {
		name        string
		gainers     []string
		scan        []string
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "perfect correlation",
			gainers:     []string{"BTC", "ETH", "ADA"},
			scan:        []string{"BTC", "ETH", "ADA"},
			expectedMin: 0.8, // Should be high correlation
			expectedMax: 1.0,
		},
		{
			name:        "reverse correlation",
			gainers:     []string{"BTC", "ETH", "ADA"},
			scan:        []string{"ADA", "ETH", "BTC"},
			expectedMin: 0.0, // Should be lower correlation
			expectedMax: 0.8,
		},
		{
			name:        "no common symbols",
			gainers:     []string{"BTC", "ETH", "ADA"},
			scan:        []string{"SOL", "DOT", "LINK"},
			expectedMin: 0.0,
			expectedMax: 0.0,
		},
		{
			name:        "partial common with good ranks",
			gainers:     []string{"BTC", "ETH", "ADA", "SOL"},
			scan:        []string{"BTC", "DOT", "ETH", "LINK"},
			expectedMin: 0.3, // Some correlation from common symbols
			expectedMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gainers := make([]bench.TopGainerResult, len(tt.gainers))
			for i, symbol := range tt.gainers {
				gainers[i] = bench.TopGainerResult{
					Symbol: symbol,
				}
			}

			weights := bench.DefaultScoreWeights()
			score := bench.CalculateCompositeScore("test", gainers, tt.scan, weights)

			if score.RankCorrelation < tt.expectedMin || score.RankCorrelation > tt.expectedMax {
				t.Errorf("Rank correlation %.3f not in expected range [%.3f, %.3f]",
					score.RankCorrelation, tt.expectedMin, tt.expectedMax)
			}

			// Validate rank differences are populated for common symbols
			for symbol := range score.Details.RankDifferences {
				found := false
				for _, s := range tt.gainers {
					if s == symbol {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Symbol %s in rank differences but not in gainers", symbol)
				}
			}
		})
	}
}

// TestCompositeScoreWeights tests that composite scores respect weights
func TestCompositeScoreWeights(t *testing.T) {
	gainers := []bench.TopGainerResult{
		{Symbol: "BTC"},
		{Symbol: "ETH"},
		{Symbol: "ADA"},
	}
	scan := []string{"BTC", "ETH", "SOL"}

	tests := []struct {
		name    string
		weights bench.ScoreWeights
	}{
		{
			name: "default weights",
			weights: bench.ScoreWeights{
				SymbolOverlap:   0.6,
				RankCorrelation: 0.3,
				PercentageAlign: 0.1,
			},
		},
		{
			name: "overlap focused",
			weights: bench.ScoreWeights{
				SymbolOverlap:   0.9,
				RankCorrelation: 0.1,
				PercentageAlign: 0.0,
			},
		},
		{
			name: "correlation focused",
			weights: bench.ScoreWeights{
				SymbolOverlap:   0.1,
				RankCorrelation: 0.9,
				PercentageAlign: 0.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := bench.CalculateCompositeScore("test", gainers, scan, tt.weights)

			// Verify weights sum approximately to 1.0
			weightSum := tt.weights.SymbolOverlap + tt.weights.RankCorrelation + tt.weights.PercentageAlign
			if weightSum < 0.99 || weightSum > 1.01 {
				t.Errorf("Weights should sum to ~1.0, got %.3f", weightSum)
			}

			// Verify overall score is within valid range
			if score.OverallScore < 0.0 || score.OverallScore > 1.0 {
				t.Errorf("Overall score %.3f should be in [0.0, 1.0]", score.OverallScore)
			}

			// Verify component scores are valid
			if score.SymbolOverlap < 0.0 || score.SymbolOverlap > 1.0 {
				t.Errorf("Symbol overlap %.3f should be in [0.0, 1.0]", score.SymbolOverlap)
			}

			if score.RankCorrelation < 0.0 || score.RankCorrelation > 1.0 {
				t.Errorf("Rank correlation %.3f should be in [0.0, 1.0]", score.RankCorrelation)
			}

			// Verify details are populated
			if score.Details.TopGainersCount != len(gainers) {
				t.Errorf("Expected top gainers count %d, got %d", len(gainers), score.Details.TopGainersCount)
			}

			if score.Details.ScanResultsCount != len(scan) {
				t.Errorf("Expected scan results count %d, got %d", len(scan), score.Details.ScanResultsCount)
			}
		})
	}
}

// TestWindowComparison tests cross-window analysis
func TestWindowComparison(t *testing.T) {
	// Create mock window results
	results := map[string]bench.CompositeScore{
		"1h": {
			OverallScore: 0.8,
			Details: bench.ScoreDetails{
				CommonSymbols: []string{"BTC", "ETH"},
			},
		},
		"24h": {
			OverallScore: 0.6,
			Details: bench.ScoreDetails{
				CommonSymbols: []string{"BTC"},
			},
		},
		"7d": {
			OverallScore: 0.4,
			Details: bench.ScoreDetails{
				CommonSymbols: []string{},
			},
		},
	}

	comparison := bench.CompareWindows(results)

	// Test basic properties
	if len(comparison.Windows) != 3 {
		t.Errorf("Expected 3 windows, got %d", len(comparison.Windows))
	}

	if comparison.BestWindow != "1h" {
		t.Errorf("Expected best window to be '1h', got '%s'", comparison.BestWindow)
	}

	if comparison.WorstWindow != "7d" {
		t.Errorf("Expected worst window to be '7d', got '%s'", comparison.WorstWindow)
	}

	// Test average calculation
	expectedAvg := (0.8 + 0.6 + 0.4) / 3.0
	if abs(comparison.AverageScore-expectedAvg) > 0.001 {
		t.Errorf("Expected average %.3f, got %.3f", expectedAvg, comparison.AverageScore)
	}

	// Test insights generation
	if len(comparison.Insights) == 0 {
		t.Errorf("Expected insights to be generated")
	}

	// Test that insights contain meaningful content
	insightsText := ""
	for _, insight := range comparison.Insights {
		insightsText += insight + " "
	}

	if !containsAny(insightsText, []string{"alignment", "window", "performance"}) {
		t.Errorf("Insights should contain relevant keywords, got: %v", comparison.Insights)
	}
}

// TestScoreValidation tests edge cases and validation
func TestScoreValidation(t *testing.T) {
	tests := []struct {
		name        string
		gainers     []bench.TopGainerResult
		scan        []string
		shouldError bool
	}{
		{
			name:    "empty gainers",
			gainers: []bench.TopGainerResult{},
			scan:    []string{"BTC", "ETH"},
		},
		{
			name: "empty scan",
			gainers: []bench.TopGainerResult{
				{Symbol: "BTC"},
				{Symbol: "ETH"},
			},
			scan: []string{},
		},
		{
			name:    "both empty",
			gainers: []bench.TopGainerResult{},
			scan:    []string{},
		},
		{
			name: "duplicate symbols in gainers",
			gainers: []bench.TopGainerResult{
				{Symbol: "BTC"},
				{Symbol: "BTC"}, // Duplicate
				{Symbol: "ETH"},
			},
			scan: []string{"BTC", "ETH"},
		},
		{
			name: "duplicate symbols in scan",
			gainers: []bench.TopGainerResult{
				{Symbol: "BTC"},
				{Symbol: "ETH"},
			},
			scan: []string{"BTC", "BTC", "ETH"}, // Duplicate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weights := bench.DefaultScoreWeights()

			// Should not panic or error
			score := bench.CalculateCompositeScore("test", tt.gainers, tt.scan, weights)

			// Basic validation
			if score.OverallScore < 0.0 || score.OverallScore > 1.0 {
				t.Errorf("Overall score %.3f should be in [0.0, 1.0]", score.OverallScore)
			}

			if score.Details.TopGainersCount != len(tt.gainers) {
				t.Errorf("Top gainers count mismatch: expected %d, got %d",
					len(tt.gainers), score.Details.TopGainersCount)
			}

			if score.Details.ScanResultsCount != len(tt.scan) {
				t.Errorf("Scan results count mismatch: expected %d, got %d",
					len(tt.scan), score.Details.ScanResultsCount)
			}
		})
	}
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(text), strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}
