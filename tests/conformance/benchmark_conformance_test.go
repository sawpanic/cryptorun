package conformance_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// BenchmarkDiagnostic represents benchmark diagnostic output
type BenchmarkDiagnostic struct {
	Methodology          string `json:"methodology"`
	SampleSizeValidation *struct {
		RequiredMinimum        int            `json:"required_minimum"`
		WindowSampleSizes      map[string]int `json:"window_sample_sizes"`
		RecommendationsEnabled bool           `json:"recommendations_enabled"`
		InsufficientWindows    []string       `json:"insufficient_windows"`
	} `json:"sample_size_validation,omitempty"`

	WindowAnalysis map[string]struct {
		AlignmentScore float64 `json:"alignment_score"`
		TotalGainers   int     `json:"total_gainers"`
		TotalMatches   int     `json:"total_matches"`

		Hits []struct {
			Symbol            string   `json:"symbol"`
			GainPercentage    float64  `json:"gain_percentage"`
			RawGainPercentage *float64 `json:"raw_gain_percentage,omitempty"`
			SpecCompliantPnL  *float64 `json:"spec_compliant_pnl,omitempty"`
			SeriesSource      *string  `json:"series_source,omitempty"`
		} `json:"hits"`

		Misses []struct {
			Symbol            string   `json:"symbol"`
			GainPercentage    float64  `json:"gain_percentage"`
			RawGainPercentage *float64 `json:"raw_gain_percentage,omitempty"`
			SpecCompliantPnL  *float64 `json:"spec_compliant_pnl,omitempty"`
			SeriesSource      *string  `json:"series_source,omitempty"`
			PrimaryReason     string   `json:"primary_reason"`
			ConfigTweak       string   `json:"config_tweak"`
		} `json:"misses"`
	} `json:"window_analysis"`

	ActionableInsights *struct {
		TopImprovements []struct {
			Change             string `json:"change"`
			Impact             string `json:"impact"`
			Risk               string `json:"risk"`
			RawGainBased       *bool  `json:"raw_gain_based,omitempty"`
			SpecCompliantBased *bool  `json:"spec_compliant_based,omitempty"`
		} `json:"top_improvements"`
	} `json:"actionable_insights,omitempty"`
}

// TestBenchmarkSampleSizeConformance verifies n≥20 requirement for recommendations
func TestBenchmarkSampleSizeConformance(t *testing.T) {
	diagnosticFiles := []string{
		"out/bench/diagnostics/bench_diag.json",
		"out/bench/calibration/before_alignment.json",
		"out/bench/calibration/after_alignment.json",
	}

	for _, filePath := range diagnosticFiles {
		t.Run(strings.ReplaceAll(filePath, "/", "_"), func(t *testing.T) {
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Skipf("Diagnostic file %s not found", filePath)
			}

			var diag BenchmarkDiagnostic
			if err := json.Unmarshal(data, &diag); err != nil {
				t.Fatalf("Failed to parse diagnostic %s: %v", filePath, err)
			}

			// Check sample size validation if present
			if diag.SampleSizeValidation != nil {
				if diag.SampleSizeValidation.RequiredMinimum != 20 {
					t.Errorf("CONFORMANCE VIOLATION: %s required minimum = %d, must be 20",
						filePath, diag.SampleSizeValidation.RequiredMinimum)
				}

				// Verify recommendations are disabled for insufficient samples
				for window, size := range diag.SampleSizeValidation.WindowSampleSizes {
					if size < 20 {
						if diag.SampleSizeValidation.RecommendationsEnabled {
							t.Errorf("CONFORMANCE VIOLATION: %s enables recommendations for %s window with n=%d < 20",
								filePath, window, size)
						}

						// Verify window is marked as insufficient
						found := false
						for _, insufficient := range diag.SampleSizeValidation.InsufficientWindows {
							if insufficient == window {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("CONFORMANCE VIOLATION: %s missing %s in insufficient_windows (n=%d)",
								filePath, window, size)
						}
					}
				}
			}

			// Verify sample sizes in window analysis
			for window, analysis := range diag.WindowAnalysis {
				if analysis.TotalGainers < 20 && analysis.TotalGainers > 0 {
					t.Logf("WARNING: %s window %s has sample size %d < 20 (recommendations should be disabled)",
						filePath, window, analysis.TotalGainers)
				}
			}
		})
	}
}

// TestBenchmarkSpecCompliantPnLConformance verifies spec-compliant P&L usage
func TestBenchmarkSpecCompliantPnLConformance(t *testing.T) {
	diagnosticFiles := []string{
		"out/bench/diagnostics/bench_diag.json",
		"out/bench/calibration/before_alignment.json",
		"out/bench/calibration/after_alignment.json",
	}

	for _, filePath := range diagnosticFiles {
		t.Run(strings.ReplaceAll(filePath, "/", "_"), func(t *testing.T) {
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Skipf("Diagnostic file %s not found", filePath)
			}

			var diag BenchmarkDiagnostic
			if err := json.Unmarshal(data, &diag); err != nil {
				t.Fatalf("Failed to parse diagnostic %s: %v", filePath, err)
			}

			// Verify spec-compliant P&L is used for decisions
			for window, analysis := range diag.WindowAnalysis {
				// Check hits use spec-compliant P&L when available
				for _, hit := range analysis.Hits {
					if hit.SpecCompliantPnL != nil && hit.RawGainPercentage != nil {
						// Spec-compliant should be <= raw due to entry/exit timing
						if *hit.SpecCompliantPnL > *hit.RawGainPercentage {
							t.Errorf("CONFORMANCE VIOLATION: %s %s hit %s has spec-compliant P&L %.2f > raw %.2f",
								filePath, window, hit.Symbol, *hit.SpecCompliantPnL, *hit.RawGainPercentage)
						}
					}
				}

				// Check misses use spec-compliant P&L when available
				for _, miss := range analysis.Misses {
					if miss.SpecCompliantPnL != nil && miss.RawGainPercentage != nil {
						// Spec-compliant should be <= raw due to entry/exit timing
						if *miss.SpecCompliantPnL > *miss.RawGainPercentage {
							t.Errorf("CONFORMANCE VIOLATION: %s %s miss %s has spec-compliant P&L %.2f > raw %.2f",
								filePath, window, miss.Symbol, *miss.SpecCompliantPnL, *miss.RawGainPercentage)
						}
					}

					// Check config recommendations don't mention raw gains
					if strings.Contains(miss.ConfigTweak, "raw") || strings.Contains(miss.ConfigTweak, "24h gain") {
						if miss.SpecCompliantPnL == nil {
							t.Errorf("CONFORMANCE VIOLATION: %s %s miss %s config tweak mentions raw gains without spec-compliant alternative",
								filePath, window, miss.Symbol)
						}
					}
				}
			}

			// Verify actionable insights use spec-compliant P&L
			if diag.ActionableInsights != nil {
				for _, improvement := range diag.ActionableInsights.TopImprovements {
					// Check if impact mentions raw gains without spec-compliant basis
					if strings.Contains(improvement.Impact, "%") && strings.Contains(improvement.Impact, "gain") {
						if improvement.RawGainBased != nil && *improvement.RawGainBased {
							if improvement.SpecCompliantBased == nil || !*improvement.SpecCompliantBased {
								t.Errorf("CONFORMANCE VIOLATION: %s improvement '%s' uses raw gains without spec-compliant basis",
									filePath, improvement.Change)
							}
						}
					}
				}
			}
		})
	}
}

// TestBenchmarkMethodologyConformance verifies proper diagnostic methodology
func TestBenchmarkMethodologyConformance(t *testing.T) {
	diagnosticFiles := []string{
		"out/bench/diagnostics/bench_diag.json",
	}

	for _, filePath := range diagnosticFiles {
		t.Run(strings.ReplaceAll(filePath, "/", "_"), func(t *testing.T) {
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Skipf("Diagnostic file %s not found", filePath)
			}

			var diag BenchmarkDiagnostic
			if err := json.Unmarshal(data, &diag); err != nil {
				t.Fatalf("Failed to parse diagnostic %s: %v", filePath, err)
			}

			// Verify methodology mentions spec-compliant approach
			expectedTerms := []string{"spec-compliant", "entry", "exit", "simulation"}
			foundTerms := 0

			for _, term := range expectedTerms {
				if strings.Contains(strings.ToLower(diag.Methodology), strings.ToLower(term)) {
					foundTerms++
				}
			}

			if foundTerms < 2 {
				t.Errorf("CONFORMANCE VIOLATION: %s methodology must mention spec-compliant P&L simulation approach",
					filePath)
			}

			// Verify exchange-native series attribution
			for window, analysis := range diag.WindowAnalysis {
				for _, hit := range analysis.Hits {
					if hit.SeriesSource != nil {
						if !strings.Contains(*hit.SeriesSource, "exchange_native") &&
							!strings.Contains(*hit.SeriesSource, "binance") &&
							!strings.Contains(*hit.SeriesSource, "kraken") &&
							!strings.Contains(*hit.SeriesSource, "coinbase") {
							if !strings.Contains(*hit.SeriesSource, "aggregator_fallback") {
								t.Errorf("CONFORMANCE VIOLATION: %s %s hit %s series source '%s' not properly labeled",
									filePath, window, hit.Symbol, *hit.SeriesSource)
							}
						}
					}
				}

				for _, miss := range analysis.Misses {
					if miss.SeriesSource != nil {
						if !strings.Contains(*miss.SeriesSource, "exchange_native") &&
							!strings.Contains(*miss.SeriesSource, "binance") &&
							!strings.Contains(*miss.SeriesSource, "kraken") &&
							!strings.Contains(*miss.SeriesSource, "coinbase") {
							if !strings.Contains(*miss.SeriesSource, "aggregator_fallback") {
								t.Errorf("CONFORMANCE VIOLATION: %s %s miss %s series source '%s' not properly labeled",
									filePath, window, miss.Symbol, *miss.SeriesSource)
							}
						}
					}
				}
			}
		})
	}
}
