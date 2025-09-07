package conformance

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

// DiagnosticsOutput represents the structure we expect in bench diagnostic outputs
type DiagnosticsOutput struct {
	Misses               []DiagnosticMiss `json:"misses"`
	Hits                 []DiagnosticHit  `json:"hits"`
	SampleSizeValidation *SampleSizeInfo  `json:"sample_size_validation,omitempty"`
	Methodology          string           `json:"methodology"`
}

type DiagnosticMiss struct {
	Symbol            string  `json:"symbol"`
	GainPercentage    float64 `json:"gain_percentage"`     // Raw 24h - context only
	RawGainPercentage float64 `json:"raw_gain_percentage"` // Explicit raw column
	SpecCompliantPnL  float64 `json:"spec_compliant_pnl"`  // THE decision metric
	SeriesSource      string  `json:"series_source"`
	PrimaryReason     string  `json:"primary_reason"`
	ConfigTweak       string  `json:"config_tweak"`
}

type DiagnosticHit struct {
	Symbol           string  `json:"symbol"`
	GainPercentage   float64 `json:"gain_percentage"`
	SpecCompliantPnL float64 `json:"spec_compliant_pnl"`
	SeriesSource     string  `json:"series_source"`
	Reason           string  `json:"reason"`
}

type SampleSizeInfo struct {
	RequiredMinimum        int            `json:"required_minimum"`
	WindowSampleSizes      map[string]int `json:"window_sample_sizes"`
	RecommendationsEnabled bool           `json:"recommendations_enabled"`
	InsufficientWindows    []string       `json:"insufficient_windows"`
}

// TestDiagnosticsNeverUsesRaw24h verifies advice NEVER based on raw_24h_change
func TestDiagnosticsNeverUsesRaw24h(t *testing.T) {
	diagnosticsPath := filepath.Join("..", "..", "out", "bench", "diagnostics", "bench_diag.json")

	if !fileExists(diagnosticsPath) {
		t.Skip("CONFORMANCE SKIP: bench_diag.json not found - no diagnostics to validate")
		return
	}

	data, err := ioutil.ReadFile(diagnosticsPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read bench_diag.json: %v", err)
	}

	var diagnostics DiagnosticsOutput
	if err := json.Unmarshal(data, &diagnostics); err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot parse bench_diag.json: %v", err)
	}

	// Verify methodology mentions spec-compliant approach
	if !strings.Contains(strings.ToLower(diagnostics.Methodology), "spec") {
		t.Error("CONFORMANCE VIOLATION: Diagnostics methodology does not mention spec-compliant approach")
	}

	// Check all misses for proper spec-compliant P&L basis
	for i, miss := range diagnostics.Misses {
		// Must have both raw and spec P&L for transparency
		if miss.RawGainPercentage == 0 {
			t.Errorf("CONFORMANCE VIOLATION: Miss %d lacks raw_gain_percentage for transparency", i)
		}

		// Config recommendations MUST be based on spec P&L, not raw
		if miss.SpecCompliantPnL <= 1.0 && miss.ConfigTweak != "" &&
			!strings.Contains(strings.ToLower(miss.ConfigTweak), "none") {
			t.Errorf("CONFORMANCE VIOLATION: Miss %d recommends config change despite spec P&L %.2f%% ≤ 1%%",
				i, miss.SpecCompliantPnL)
		}

		// High raw gain but negative spec P&L should have "correctly_filtered" reason
		if miss.RawGainPercentage > 20.0 && miss.SpecCompliantPnL < 0 {
			if !strings.Contains(strings.ToLower(miss.PrimaryReason), "correctly") {
				t.Errorf("CONFORMANCE VIOLATION: Miss %d has high raw gain (%.1f%%) but negative spec P&L (%.2f%%) without 'correctly filtered' reason",
					i, miss.RawGainPercentage, miss.SpecCompliantPnL)
			}
		}

		// Series source must be labeled
		if miss.SeriesSource == "" {
			t.Errorf("CONFORMANCE VIOLATION: Miss %d lacks series_source attribution", i)
		}
	}

	// Verify hits also have dual-column format
	for i, hit := range diagnostics.Hits {
		if hit.SpecCompliantPnL == 0 {
			t.Errorf("CONFORMANCE VIOLATION: Hit %d lacks spec_compliant_pnl column", i)
		}
		if hit.SeriesSource == "" {
			t.Errorf("CONFORMANCE VIOLATION: Hit %d lacks series_source attribution", i)
		}
	}
}

// TestSampleSizeEnforcement ensures n≥20 rule is enforced
func TestSampleSizeEnforcement(t *testing.T) {
	diagnosticsPath := filepath.Join("..", "..", "out", "bench", "diagnostics", "bench_diag.json")

	if !fileExists(diagnosticsPath) {
		t.Skip("CONFORMANCE SKIP: bench_diag.json not found")
		return
	}

	data, err := ioutil.ReadFile(diagnosticsPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read bench_diag.json: %v", err)
	}

	var diagnostics DiagnosticsOutput
	if err := json.Unmarshal(data, &diagnostics); err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot parse bench_diag.json: %v", err)
	}

	// Must have sample size validation section
	if diagnostics.SampleSizeValidation == nil {
		t.Error("CONFORMANCE VIOLATION: Missing sample_size_validation section in diagnostics")
		return
	}

	validation := diagnostics.SampleSizeValidation

	// Required minimum must be 20
	if validation.RequiredMinimum != 20 {
		t.Errorf("CONFORMANCE VIOLATION: Required minimum sample size is %d, must be 20",
			validation.RequiredMinimum)
	}

	// Check window-specific enforcement
	hasInsufficientSample := false
	for window, size := range validation.WindowSampleSizes {
		if size < 20 {
			hasInsufficientSample = true

			// Window with n<20 must be in insufficient list
			found := false
			for _, insufficientWindow := range validation.InsufficientWindows {
				if insufficientWindow == window {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("CONFORMANCE VIOLATION: Window '%s' has sample size %d<20 but not in insufficient_windows list",
					window, size)
			}
		}
	}

	// If any window has n<20, recommendations must be disabled
	if hasInsufficientSample && validation.RecommendationsEnabled {
		t.Error("CONFORMANCE VIOLATION: Recommendations enabled despite insufficient sample size in some windows")
	}

	// If all windows have n≥20, recommendations should be enabled
	if !hasInsufficientSample && !validation.RecommendationsEnabled {
		t.Error("CONFORMANCE VIOLATION: Recommendations disabled despite sufficient sample sizes")
	}
}

// TestRecommendationLogicCompliance validates recommendation generation logic
func TestRecommendationLogicCompliance(t *testing.T) {
	// Check diagnostic logic implementation for spec compliance
	diagPath := filepath.Join("..", "..", "internal", "application", "bench", "diagnostics.go")

	if !fileExists(diagPath) {
		t.Skip("CONFORMANCE SKIP: diagnostics.go not found")
		return
	}

	content, err := readFileContent(diagPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read diagnostics.go: %v", err)
	}

	// Must reference spec_compliant_pnl in recommendation logic
	requiredPatterns := []string{
		"spec_compliant_pnl", "specCompliantPnl", "SpecCompliantPnL",
		"spec.*pnl", "spec.*p&l",
	}

	foundSpecReference := false
	for _, pattern := range requiredPatterns {
		if containsPattern(content, pattern) {
			foundSpecReference = true
			break
		}
	}

	if !foundSpecReference {
		t.Error("CONFORMANCE VIOLATION: diagnostics.go does not reference spec-compliant P&L in logic")
	}

	// Must not base recommendations on raw 24h changes
	forbiddenPatterns := []string{
		"raw.*gain.*recommend", "raw.*percentage.*recommend",
		"24h.*change.*recommend", "gain_percentage.*recommend",
	}

	for _, forbidden := range forbiddenPatterns {
		if containsPattern(content, forbidden) {
			t.Errorf("CONFORMANCE VIOLATION: diagnostics.go uses forbidden pattern '%s' for recommendations", forbidden)
		}
	}

	// Must have sample size check in recommendation function
	if !strings.Contains(content, "sample") || !strings.Contains(content, "20") {
		t.Error("CONFORMANCE VIOLATION: diagnostics.go missing sample size validation")
	}
}

// TestSeriesSourceLabeling ensures all data sources are properly attributed
func TestSeriesSourceLabeling(t *testing.T) {
	diagnosticsPath := filepath.Join("..", "..", "out", "bench", "diagnostics", "bench_diag.json")

	if !fileExists(diagnosticsPath) {
		t.Skip("CONFORMANCE SKIP: bench_diag.json not found")
		return
	}

	data, err := ioutil.ReadFile(diagnosticsPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read bench_diag.json: %v", err)
	}

	var diagnostics DiagnosticsOutput
	if err := json.Unmarshal(data, &diagnostics); err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot parse bench_diag.json: %v", err)
	}

	// Valid series source patterns
	validSources := []string{
		"exchange_native_binance", "exchange_native_kraken", "exchange_native_coinbase", "exchange_native_okx",
		"aggregator_fallback_coingecko", "aggregator_fallback_dexscreener",
	}

	// Check all misses have valid series sources
	for i, miss := range diagnostics.Misses {
		if miss.SeriesSource == "" {
			t.Errorf("CONFORMANCE VIOLATION: Miss %d missing series_source", i)
			continue
		}

		validSource := false
		for _, valid := range validSources {
			if strings.Contains(strings.ToLower(miss.SeriesSource), strings.ToLower(valid)) {
				validSource = true
				break
			}
		}

		if !validSource {
			t.Errorf("CONFORMANCE VIOLATION: Miss %d has invalid series_source '%s'", i, miss.SeriesSource)
		}

		// If using aggregator fallback, must be clearly labeled
		if strings.Contains(strings.ToLower(miss.SeriesSource), "aggregator") &&
			!strings.Contains(strings.ToLower(miss.SeriesSource), "fallback") {
			t.Errorf("CONFORMANCE VIOLATION: Miss %d aggregator source not labeled as fallback: '%s'",
				i, miss.SeriesSource)
		}
	}

	// Check all hits have valid series sources
	for i, hit := range diagnostics.Hits {
		if hit.SeriesSource == "" {
			t.Errorf("CONFORMANCE VIOLATION: Hit %d missing series_source", i)
		}
	}
}

// TestDiagnosticsOutputFormat validates the dual-column display requirement
func TestDiagnosticsOutputFormat(t *testing.T) {
	diagnosticsPath := filepath.Join("..", "..", "out", "bench", "diagnostics", "bench_diag.json")

	if !fileExists(diagnosticsPath) {
		t.Skip("CONFORMANCE SKIP: bench_diag.json not found")
		return
	}

	data, err := ioutil.ReadFile(diagnosticsPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read bench_diag.json: %v", err)
	}

	var diagnostics DiagnosticsOutput
	if err := json.Unmarshal(data, &diagnostics); err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot parse bench_diag.json: %v", err)
	}

	// Every miss must have BOTH raw and spec-compliant columns
	for i, miss := range diagnostics.Misses {
		if miss.GainPercentage == 0 && miss.RawGainPercentage == 0 {
			t.Errorf("CONFORMANCE VIOLATION: Miss %d missing raw gain percentage column", i)
		}

		if miss.SpecCompliantPnL == 0 {
			t.Errorf("CONFORMANCE VIOLATION: Miss %d missing spec_compliant_pnl column", i)
		}

		// Raw and explicit raw should match (dual representation for clarity)
		if miss.GainPercentage != miss.RawGainPercentage && miss.RawGainPercentage != 0 {
			t.Errorf("CONFORMANCE VIOLATION: Miss %d gain_percentage %.2f != raw_gain_percentage %.2f",
				i, miss.GainPercentage, miss.RawGainPercentage)
		}
	}

	// Every hit should also have spec P&L for consistency
	for i, hit := range diagnostics.Hits {
		if hit.SpecCompliantPnL == 0 {
			t.Errorf("CONFORMANCE VIOLATION: Hit %d missing spec_compliant_pnl column", i)
		}
	}
}
