package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cryptorun/internal/qa"
)

func TestAcceptanceValidator_ValidFixtures(t *testing.T) {
	// Create temporary directory with valid QA artifacts
	testDir := t.TempDir()
	qaDir := filepath.Join(testDir, "out", "qa")
	auditDir := filepath.Join(testDir, "out", "audit")

	if err := os.MkdirAll(qaDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(auditDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid QA_REPORT.json
	qaReport := map[string]interface{}{
		"phases": []map[string]interface{}{
			{"name": "environment", "status": "pass"},
			{"name": "provider_health", "status": "pass"},
			{"name": "microstructure", "status": "pass"},
			{"name": "live_data", "status": "pass"},
			{"name": "determinism", "status": "pass"},
			{"name": "acceptance", "status": "pass"},
		},
		"provider_health": map[string]interface{}{
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
		"execution_time": "2025-01-15T10:30:00Z",
		"summary": map[string]interface{}{
			"total_phases": 6,
			"passed":       6,
			"failed":       0,
			"skipped":      0,
		},
	}

	qaReportData, _ := json.MarshalIndent(qaReport, "", "  ")
	if err := os.WriteFile(filepath.Join(qaDir, "QA_REPORT.json"), qaReportData, 0644); err != nil {
		t.Fatal(err)
	}

	// Create QA_REPORT.md
	qaReportMD := `# QA Report - CryptoRun

## Summary
All phases completed successfully.

## Provider Health
- Kraken: Healthy (95% success rate)
- OKX: Degraded (89% success rate - rate limited)

## Recommendations
- Monitor OKX rate limits
- Continue with production deployment
`
	if err := os.WriteFile(filepath.Join(qaDir, "QA_REPORT.md"), []byte(qaReportMD), 0644); err != nil {
		t.Fatal(err)
	}

	// Create provider_health.json
	providerHealth := map[string]interface{}{
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
	}
	providerHealthData, _ := json.MarshalIndent(providerHealth, "", "  ")
	if err := os.WriteFile(filepath.Join(qaDir, "provider_health.json"), providerHealthData, 0644); err != nil {
		t.Fatal(err)
	}

	// Create microstructure_sample.csv
	microstructureCSV := `pair,venue,spread_bps,depth_usd_2pc,vadr,adv
BTCUSD,kraken,5.2,150000,2.1,1000000
ETHUSD,kraken,7.8,120000,1.9,800000
BTCUSD,okx,4.9,180000,2.3,1200000
ETHUSD,okx,6.5,140000,2.0,900000
SOLUSD,kraken,12.4,80000,1.8,400000`
	if err := os.WriteFile(filepath.Join(qaDir, "microstructure_sample.csv"), []byte(microstructureCSV), 0644); err != nil {
		t.Fatal(err)
	}

	// Create live_return_diffs.json
	liveReturnDiffs := map[string]interface{}{
		"timestamp": "2025-01-15T10:30:00Z",
		"pairs": map[string]interface{}{
			"BTCUSD": map[string]interface{}{
				"expected": 45123.45,
				"actual":   45125.67,
				"diff":     0.0049,
			},
			"ETHUSD": map[string]interface{}{
				"expected": 3456.78,
				"actual":   3457.12,
				"diff":     0.0098,
			},
		},
		"max_diff": 0.0098,
		"avg_diff": 0.0074,
	}
	liveReturnDiffsData, _ := json.MarshalIndent(liveReturnDiffs, "", "  ")
	if err := os.WriteFile(filepath.Join(qaDir, "live_return_diffs.json"), liveReturnDiffsData, 0644); err != nil {
		t.Fatal(err)
	}

	// Create vadr_adv_checks.json (deterministic)
	vadrAdvChecks := []map[string]interface{}{
		{
			"pair":          "BTCUSD",
			"venue":         "kraken",
			"vadr":          2.1,
			"adv":           1000000.0,
			"spread_bps":    5.2,
			"depth_usd_2pc": 150000.0,
		},
		{
			"pair":          "ETHUSD",
			"venue":         "kraken",
			"vadr":          1.9,
			"adv":           800000.0,
			"spread_bps":    7.8,
			"depth_usd_2pc": 120000.0,
		},
		{
			"pair":          "BTCUSD",
			"venue":         "okx",
			"vadr":          2.3,
			"adv":           1200000.0,
			"spread_bps":    4.9,
			"depth_usd_2pc": 180000.0,
		},
	}
	vadrAdvChecksData, _ := json.MarshalIndent(vadrAdvChecks, "", "  ")
	if err := os.WriteFile(filepath.Join(qaDir, "vadr_adv_checks.json"), vadrAdvChecksData, 0644); err != nil {
		t.Fatal(err)
	}

	// Create progress_trace.jsonl
	progressTrace := `{"timestamp":"2025-01-15T10:28:00Z","phase":"environment","status":"started"}
{"timestamp":"2025-01-15T10:28:05Z","phase":"environment","status":"completed","duration":"5s"}
{"timestamp":"2025-01-15T10:28:05Z","phase":"provider_health","status":"started"}
{"timestamp":"2025-01-15T10:28:15Z","phase":"provider_health","status":"completed","duration":"10s"}
{"timestamp":"2025-01-15T10:28:15Z","phase":"microstructure","status":"started"}
{"timestamp":"2025-01-15T10:28:25Z","phase":"microstructure","status":"completed","duration":"10s"}
{"timestamp":"2025-01-15T10:28:25Z","phase":"live_data","status":"started"}
{"timestamp":"2025-01-15T10:29:00Z","phase":"live_data","status":"completed","duration":"35s"}
{"timestamp":"2025-01-15T10:29:00Z","phase":"determinism","status":"started"}
{"timestamp":"2025-01-15T10:29:05Z","phase":"determinism","status":"completed","duration":"5s"}
{"timestamp":"2025-01-15T10:29:05Z","phase":"acceptance","status":"started"}
{"timestamp":"2025-01-15T10:30:00Z","phase":"acceptance","status":"completed","duration":"55s"}
`
	if err := os.WriteFile(filepath.Join(auditDir, "progress_trace.jsonl"), []byte(progressTrace), 0644); err != nil {
		t.Fatal(err)
	}

	// Test ValidateArtifacts - should pass
	if err := qa.ValidateArtifacts(testDir); err != nil {
		t.Errorf("ValidateArtifacts should pass with valid fixtures: %v", err)
	}

	// Test ValidateDeterminism - should pass
	if err := qa.ValidateDeterminism(testDir); err != nil {
		t.Errorf("ValidateDeterminism should pass: %v", err)
	}

	// Test ValidateTelemetry with mock Prometheus data
	prometheusText := `# HELP cryptorun_provider_requests_total Total provider requests
# TYPE cryptorun_provider_requests_total counter
cryptorun_provider_requests_total{provider="kraken"} 1234
cryptorun_provider_requests_total{provider="okx"} 987
# HELP cryptorun_provider_latency_seconds Provider request latency
# TYPE cryptorun_provider_latency_seconds histogram
cryptorun_provider_latency_seconds_bucket{provider="kraken",le="0.1"} 100
cryptorun_provider_latency_seconds_bucket{provider="kraken",le="0.2"} 200
cryptorun_provider_latency_seconds_bucket{provider="kraken",le="+Inf"} 250
# HELP cryptorun_scan_duration_seconds Scan duration
# TYPE cryptorun_scan_duration_seconds histogram
cryptorun_scan_duration_seconds_bucket{le="1.0"} 50
cryptorun_scan_duration_seconds_bucket{le="+Inf"} 100
# HELP cryptorun_pairs_processed_total Total pairs processed
# TYPE cryptorun_pairs_processed_total counter
cryptorun_pairs_processed_total 456
# HELP cryptorun_errors_total Total errors
# TYPE cryptorun_errors_total counter
cryptorun_errors_total 7`

	if err := qa.ValidateTelemetry(prometheusText); err != nil {
		t.Errorf("ValidateTelemetry should pass with valid metrics: %v", err)
	}
}

func TestAcceptanceValidator_InvalidFixtures(t *testing.T) {
	testDir := t.TempDir()
	qaDir := filepath.Join(testDir, "out", "qa")
	auditDir := filepath.Join(testDir, "out", "audit")

	if err := os.MkdirAll(qaDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(auditDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Test missing files - should fail
	if err := qa.ValidateArtifacts(testDir); err == nil {
		t.Error("ValidateArtifacts should fail with missing files")
	}

	// Create invalid QA_REPORT.json (malformed JSON)
	invalidJSON := `{"phases": [{"name": "test", "status": "pass"}, invalid json}`
	if err := os.WriteFile(filepath.Join(qaDir, "QA_REPORT.json"), []byte(invalidJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Should still fail due to invalid JSON
	if err := qa.ValidateArtifacts(testDir); err == nil {
		t.Error("ValidateArtifacts should fail with invalid JSON")
	}

	// Create valid QA_REPORT.json but missing provider_health field
	invalidQAReport := map[string]interface{}{
		"phases": []map[string]interface{}{
			{"name": "environment", "status": "pass"},
		},
		// Missing provider_health field
	}
	invalidQAReportData, _ := json.MarshalIndent(invalidQAReport, "", "  ")
	if err := os.WriteFile(filepath.Join(qaDir, "QA_REPORT.json"), invalidQAReportData, 0644); err != nil {
		t.Fatal(err)
	}

	// Should fail due to missing provider_health
	if err := qa.ValidateArtifacts(testDir); err == nil {
		t.Error("ValidateArtifacts should fail with missing provider_health")
	}
}

func TestAcceptanceValidator_MutatedFixtures(t *testing.T) {
	// Start with valid fixtures
	testDir := t.TempDir()
	qaDir := filepath.Join(testDir, "out", "qa")
	auditDir := filepath.Join(testDir, "out", "audit")

	if err := os.MkdirAll(qaDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(auditDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create minimal valid artifacts first
	createMinimalValidArtifacts(qaDir, auditDir, t)

	// Verify initially passes
	if err := qa.ValidateArtifacts(testDir); err != nil {
		t.Fatalf("Initial validation should pass: %v", err)
	}

	// Test 1: Mutate provider_health.json to have invalid success_rate
	invalidProviderHealth := map[string]interface{}{
		"kraken": map[string]interface{}{
			"success_rate":     1.5, // Invalid > 1.0
			"latency_p50":      120.5,
			"latency_p95":      250.0,
			"budget_remaining": 1000.0,
			"degraded":         false,
		},
	}
	invalidProviderHealthData, _ := json.MarshalIndent(invalidProviderHealth, "", "  ")
	if err := os.WriteFile(filepath.Join(qaDir, "provider_health.json"), invalidProviderHealthData, 0644); err != nil {
		t.Fatal(err)
	}

	// Should fail
	if err := qa.ValidateArtifacts(testDir); err == nil {
		t.Error("ValidateArtifacts should fail with invalid success_rate")
	}

	// Test 2: Fix provider_health but break microstructure CSV
	createMinimalValidArtifacts(qaDir, auditDir, t) // Reset to valid

	// Create CSV with missing required column
	invalidCSV := `pair,venue,spread_bps
BTCUSD,kraken,5.2
ETHUSD,kraken,7.8`
	if err := os.WriteFile(filepath.Join(qaDir, "microstructure_sample.csv"), []byte(invalidCSV), 0644); err != nil {
		t.Fatal(err)
	}

	// Should fail due to missing columns
	if err := qa.ValidateArtifacts(testDir); err == nil {
		t.Error("ValidateArtifacts should fail with missing CSV columns")
	}

	// Test 3: Fix CSV but make determinism check empty
	createMinimalValidArtifacts(qaDir, auditDir, t) // Reset to valid

	// Create empty vadr_adv_checks.json
	emptyVADR := []interface{}{}
	emptyVADRData, _ := json.MarshalIndent(emptyVADR, "", "  ")
	if err := os.WriteFile(filepath.Join(qaDir, "vadr_adv_checks.json"), emptyVADRData, 0644); err != nil {
		t.Fatal(err)
	}

	// Should fail determinism check
	if err := qa.ValidateDeterminism(testDir); err == nil {
		t.Error("ValidateDeterminism should fail with empty vadr_adv_checks.json")
	}
}

func TestAcceptanceValidator_TelemetryValidation(t *testing.T) {
	// Test missing required metrics
	incompleteMetrics := `# HELP cryptorun_provider_requests_total Total provider requests
# TYPE cryptorun_provider_requests_total counter
cryptorun_provider_requests_total{provider="kraken"} 1234
# Missing other required metrics`

	if err := qa.ValidateTelemetry(incompleteMetrics); err == nil {
		t.Error("ValidateTelemetry should fail with incomplete metrics")
	}

	// Test with completely unrelated metrics
	unrelatedMetrics := `# HELP some_other_metric Some other metric
# TYPE some_other_metric counter
some_other_metric 123`

	if err := qa.ValidateTelemetry(unrelatedMetrics); err == nil {
		t.Error("ValidateTelemetry should fail with unrelated metrics")
	}

	// Test with valid complete metrics
	completeMetrics := `# HELP cryptorun_provider_requests_total Total provider requests
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

	if err := qa.ValidateTelemetry(completeMetrics); err != nil {
		t.Errorf("ValidateTelemetry should pass with complete metrics: %v", err)
	}
}

// Helper function to create minimal valid artifacts for testing
func createMinimalValidArtifacts(qaDir, auditDir string, t *testing.T) {
	// QA_REPORT.json
	qaReport := map[string]interface{}{
		"phases": []map[string]interface{}{
			{"name": "environment", "status": "pass"},
		},
		"provider_health": map[string]interface{}{
			"kraken": map[string]interface{}{
				"success_rate": 0.95,
			},
		},
	}
	qaReportData, _ := json.MarshalIndent(qaReport, "", "  ")
	if err := os.WriteFile(filepath.Join(qaDir, "QA_REPORT.json"), qaReportData, 0644); err != nil {
		t.Fatal(err)
	}

	// QA_REPORT.md
	if err := os.WriteFile(filepath.Join(qaDir, "QA_REPORT.md"), []byte("# QA Report\nTest report"), 0644); err != nil {
		t.Fatal(err)
	}

	// provider_health.json
	providerHealth := map[string]interface{}{
		"kraken": map[string]interface{}{
			"success_rate":     0.95,
			"latency_p50":      120.5,
			"latency_p95":      250.0,
			"budget_remaining": 1000.0,
			"degraded":         false,
		},
	}
	providerHealthData, _ := json.MarshalIndent(providerHealth, "", "  ")
	if err := os.WriteFile(filepath.Join(qaDir, "provider_health.json"), providerHealthData, 0644); err != nil {
		t.Fatal(err)
	}

	// microstructure_sample.csv
	csv := `pair,venue,spread_bps,depth_usd_2pc,vadr,adv
BTCUSD,kraken,5.2,150000,2.1,1000000`
	if err := os.WriteFile(filepath.Join(qaDir, "microstructure_sample.csv"), []byte(csv), 0644); err != nil {
		t.Fatal(err)
	}

	// live_return_diffs.json
	diffs := map[string]interface{}{"test": "data"}
	diffsData, _ := json.MarshalIndent(diffs, "", "  ")
	if err := os.WriteFile(filepath.Join(qaDir, "live_return_diffs.json"), diffsData, 0644); err != nil {
		t.Fatal(err)
	}

	// vadr_adv_checks.json
	vadrChecks := []map[string]interface{}{
		{
			"pair":          "BTCUSD",
			"venue":         "kraken",
			"vadr":          2.1,
			"adv":           1000000.0,
			"spread_bps":    5.2,
			"depth_usd_2pc": 150000.0,
		},
	}
	vadrChecksData, _ := json.MarshalIndent(vadrChecks, "", "  ")
	if err := os.WriteFile(filepath.Join(qaDir, "vadr_adv_checks.json"), vadrChecksData, 0644); err != nil {
		t.Fatal(err)
	}

	// progress_trace.jsonl
	if err := os.WriteFile(filepath.Join(auditDir, "progress_trace.jsonl"), []byte(`{"test":"data"}`), 0644); err != nil {
		t.Fatal(err)
	}
}
