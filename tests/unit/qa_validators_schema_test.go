package unit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sawpanic/cryptorun/internal/qa"
)

func TestValidateArtifacts_QAReport(t *testing.T) {
	testDir := t.TempDir()
	qaDir := filepath.Join(testDir, "out", "qa")
	auditDir := filepath.Join(testDir, "out", "audit")

	// Create directories
	os.MkdirAll(qaDir, 0755)
	os.MkdirAll(auditDir, 0755)

	// Test valid QA_REPORT.json
	validQAReport := map[string]interface{}{
		"phases": []map[string]interface{}{
			{"name": "environment", "status": "pass"},
			{"name": "provider_health", "status": "pass"},
			{"name": "microstructure", "status": "pass"},
		},
		"provider_health": map[string]interface{}{
			"kraken": map[string]interface{}{
				"success_rate": 0.95,
			},
		},
	}

	reportPath := filepath.Join(qaDir, "QA_REPORT.json")
	reportData, _ := json.MarshalIndent(validQAReport, "", "  ")
	os.WriteFile(reportPath, reportData, 0644)

	// Create QA_REPORT.md
	mdPath := filepath.Join(qaDir, "QA_REPORT.md")
	os.WriteFile(mdPath, []byte("# QA Report\nTest report"), 0644)

	// Test validateQAReport function directly
	if err := qa.ValidateArtifacts(testDir); err == nil {
		t.Error("Expected validation to fail due to missing files")
	}
}

func TestValidateProviderHealth(t *testing.T) {
	testDir := t.TempDir()
	qaDir := filepath.Join(testDir, "out", "qa")
	os.MkdirAll(qaDir, 0755)

	tests := []struct {
		name        string
		data        interface{}
		shouldError bool
	}{
		{
			name: "valid provider health",
			data: map[string]interface{}{
				"kraken": map[string]interface{}{
					"success_rate":     0.95,
					"latency_p50":      120.5,
					"latency_p95":      250.0,
					"budget_remaining": 1000.0,
					"degraded":         false,
				},
				"okx": map[string]interface{}{
					"success_rate":     0.89,
					"latency_p50":      95.0,
					"latency_p95":      180.0,
					"budget_remaining": 500.0,
					"degraded":         true,
					"degraded_reason":  "rate limit exceeded",
				},
			},
			shouldError: false,
		},
		{
			name: "invalid success rate",
			data: map[string]interface{}{
				"kraken": map[string]interface{}{
					"success_rate":     1.5, // Invalid > 1.0
					"latency_p50":      120.5,
					"latency_p95":      250.0,
					"budget_remaining": 1000.0,
					"degraded":         false,
				},
			},
			shouldError: true,
		},
		{
			name: "negative latency",
			data: map[string]interface{}{
				"kraken": map[string]interface{}{
					"success_rate":     0.95,
					"latency_p50":      -10.0, // Invalid negative
					"latency_p95":      250.0,
					"budget_remaining": 1000.0,
					"degraded":         false,
				},
			},
			shouldError: true,
		},
		{
			name: "degraded without reason",
			data: map[string]interface{}{
				"kraken": map[string]interface{}{
					"success_rate":     0.95,
					"latency_p50":      120.5,
					"latency_p95":      250.0,
					"budget_remaining": 1000.0,
					"degraded":         true, // Missing degraded_reason
				},
			},
			shouldError: true,
		},
		{
			name:        "empty providers",
			data:        map[string]interface{}{},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthPath := filepath.Join(qaDir, "provider_health.json")
			healthData, _ := json.MarshalIndent(tt.data, "", "  ")
			os.WriteFile(healthPath, healthData, 0644)

			err := validateProviderHealth(qaDir)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// validateProviderHealth is a test helper that exposes the internal function
func validateProviderHealth(qaDir string) error {
	// This would normally be internal, but we need it for testing
	healthPath := filepath.Join(qaDir, "provider_health.json")

	data, err := os.ReadFile(healthPath)
	if err != nil {
		return err
	}

	var healthMap qa.ProviderHealthMap
	if err := json.Unmarshal(data, &healthMap); err != nil {
		return err
	}

	if len(healthMap) == 0 {
		return fmt.Errorf("provider_health.json contains no providers")
	}

	// Validate each provider has required fields
	for providerName, health := range healthMap {
		if health.SuccessRate < 0 || health.SuccessRate > 1 {
			return fmt.Errorf("provider %s has invalid success_rate: %f", providerName, health.SuccessRate)
		}

		if health.LatencyP50 < 0 {
			return fmt.Errorf("provider %s has negative latency_p50: %f", providerName, health.LatencyP50)
		}

		if health.LatencyP95 < 0 {
			return fmt.Errorf("provider %s has negative latency_p95: %f", providerName, health.LatencyP95)
		}

		if health.BudgetRemaining < 0 {
			return fmt.Errorf("provider %s has negative budget_remaining: %f", providerName, health.BudgetRemaining)
		}

		// If degraded is true, degraded_reason should be provided
		if health.Degraded && health.DegradedReason == "" {
			return fmt.Errorf("provider %s is degraded but missing degraded_reason", providerName)
		}
	}

	return nil
}

func TestReadCSV(t *testing.T) {
	testDir := t.TempDir()

	// Test valid CSV
	validCSVPath := filepath.Join(testDir, "valid.csv")
	validCSVContent := `pair,venue,spread_bps,depth_usd_2pc,vadr,adv
BTCUSD,kraken,5.2,150000,2.1,1000000
ETHUSD,okx,7.8,120000,1.9,800000`
	os.WriteFile(validCSVPath, []byte(validCSVContent), 0644)

	rows, err := qa.ReadCSV(validCSVPath)
	if err != nil {
		t.Fatalf("ReadCSV failed: %v", err)
	}

	if len(rows) != 3 { // Header + 2 data rows
		t.Errorf("Expected 3 rows, got %d", len(rows))
	}

	expectedHeaders := []string{"pair", "venue", "spread_bps", "depth_usd_2pc", "vadr", "adv"}
	for i, expected := range expectedHeaders {
		if rows[0][i] != expected {
			t.Errorf("Header column %d: expected %s, got %s", i, expected, rows[0][i])
		}
	}

	// Test invalid CSV
	invalidCSVPath := filepath.Join(testDir, "invalid.csv")
	invalidCSVContent := `pair,venue,spread_bps
BTCUSD,kraken,5.2,extra_field` // Mismatched columns
	os.WriteFile(invalidCSVPath, []byte(invalidCSVContent), 0644)

	_, err = qa.ReadCSV(invalidCSVPath)
	if err == nil {
		t.Error("Expected error for invalid CSV, but got none")
	}
}

func TestValidateCSVStructure(t *testing.T) {
	testDir := t.TempDir()

	tests := []struct {
		name            string
		content         string
		requiredColumns []string
		shouldError     bool
	}{
		{
			name: "valid structure",
			content: `pair,venue,spread_bps,depth_usd_2pc,vadr,adv
BTCUSD,kraken,5.2,150000,2.1,1000000`,
			requiredColumns: []string{"pair", "venue", "spread_bps", "depth_usd_2pc", "vadr", "adv"},
			shouldError:     false,
		},
		{
			name: "missing column",
			content: `pair,venue,spread_bps
BTCUSD,kraken,5.2`,
			requiredColumns: []string{"pair", "venue", "spread_bps", "depth_usd_2pc"},
			shouldError:     true,
		},
		{
			name:            "empty file",
			content:         "",
			requiredColumns: []string{"pair"},
			shouldError:     true,
		},
		{
			name:            "header only",
			content:         `pair,venue,spread_bps`,
			requiredColumns: []string{"pair", "venue", "spread_bps"},
			shouldError:     true,
		},
		{
			name: "case insensitive columns",
			content: `PAIR,Venue,SPREAD_BPS
BTCUSD,kraken,5.2`,
			requiredColumns: []string{"pair", "venue", "spread_bps"},
			shouldError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csvPath := filepath.Join(testDir, "test.csv")
			os.WriteFile(csvPath, []byte(tt.content), 0644)

			err := qa.ValidateCSVStructure(csvPath, tt.requiredColumns)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestValidateJSON(t *testing.T) {
	testDir := t.TempDir()

	// Test valid JSON
	validJSONPath := filepath.Join(testDir, "valid.json")
	validData := map[string]interface{}{
		"name":  "test",
		"value": 123,
		"items": []string{"a", "b", "c"},
	}
	validJSONData, _ := json.MarshalIndent(validData, "", "  ")
	os.WriteFile(validJSONPath, validJSONData, 0644)

	var result map[string]interface{}
	if err := qa.ReadJSON(validJSONPath, &result); err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("Expected name 'test', got %v", result["name"])
	}

	// Test invalid JSON
	invalidJSONPath := filepath.Join(testDir, "invalid.json")
	invalidJSONContent := `{"name": "test", "value": 123,}` // Trailing comma
	os.WriteFile(invalidJSONPath, []byte(invalidJSONContent), 0644)

	var invalidResult map[string]interface{}
	if err := qa.ReadJSON(invalidJSONPath, &invalidResult); err == nil {
		t.Error("Expected error for invalid JSON, but got none")
	}
}

func TestValidateTelemetry(t *testing.T) {
	// Test with Prometheus text format
	prometheusText := `# HELP cryptorun_provider_requests_total Total provider requests
# TYPE cryptorun_provider_requests_total counter
cryptorun_provider_requests_total{provider="kraken"} 1234
# HELP cryptorun_provider_latency_seconds Provider request latency
# TYPE cryptorun_provider_latency_seconds histogram
cryptorun_provider_latency_seconds_bucket{provider="kraken",le="0.1"} 100
# HELP cryptorun_scan_duration_seconds Scan duration
# TYPE cryptorun_scan_duration_seconds histogram
cryptorun_scan_duration_seconds_bucket{le="1.0"} 50
# HELP cryptorun_pairs_processed_total Total pairs processed
# TYPE cryptorun_pairs_processed_total counter
cryptorun_pairs_processed_total 456
# HELP cryptorun_errors_total Total errors
# TYPE cryptorun_errors_total counter
cryptorun_errors_total 7`

	if err := qa.ValidateTelemetry(prometheusText); err != nil {
		t.Errorf("ValidateTelemetry failed: %v", err)
	}

	// Test with missing metrics
	incompleteText := `# HELP cryptorun_provider_requests_total Total provider requests
# TYPE cryptorun_provider_requests_total counter
cryptorun_provider_requests_total{provider="kraken"} 1234`

	if err := qa.ValidateTelemetry(incompleteText); err == nil {
		t.Error("Expected error for incomplete metrics, but got none")
	}

	// Test with invalid input type
	if err := qa.ValidateTelemetry(123); err == nil {
		t.Error("Expected error for invalid input type, but got none")
	}
}

func TestFileExists(t *testing.T) {
	testDir := t.TempDir()

	// Test existing file
	existingFile := filepath.Join(testDir, "exists.txt")
	os.WriteFile(existingFile, []byte("content"), 0644)

	if err := qa.FileExists(existingFile); err != nil {
		t.Errorf("FileExists should not error for existing file: %v", err)
	}

	// Test non-existent file
	nonExistentFile := filepath.Join(testDir, "does_not_exist.txt")
	if err := qa.FileExists(nonExistentFile); err == nil {
		t.Error("FileExists should error for non-existent file")
	}

	// Test directory (should fail)
	if err := qa.FileExists(testDir); err == nil {
		t.Error("FileExists should error for directory")
	}
}

func TestValidateFileSize(t *testing.T) {
	testDir := t.TempDir()

	// Create test files of different sizes
	smallFile := filepath.Join(testDir, "small.txt")
	os.WriteFile(smallFile, []byte("a"), 0644) // 1 byte

	largeFile := filepath.Join(testDir, "large.txt")
	os.WriteFile(largeFile, make([]byte, 1000), 0644) // 1000 bytes

	// Test valid size
	if err := qa.ValidateFileSize(smallFile, 1, 10); err != nil {
		t.Errorf("ValidateFileSize should not error for valid size: %v", err)
	}

	// Test too small
	if err := qa.ValidateFileSize(smallFile, 10, 100); err == nil {
		t.Error("ValidateFileSize should error for file too small")
	}

	// Test too large
	if err := qa.ValidateFileSize(largeFile, 1, 100); err == nil {
		t.Error("ValidateFileSize should error for file too large")
	}

	// Test no max size
	if err := qa.ValidateFileSize(largeFile, 1, 0); err != nil {
		t.Errorf("ValidateFileSize should not error with no max size: %v", err)
	}
}
