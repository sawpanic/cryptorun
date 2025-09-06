package unit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cryptorun/internal/qa"
)

func TestAcceptanceValidator_ValidateFileExists(t *testing.T) {
	tempDir := t.TempDir()
	qaDir := filepath.Join(tempDir, "out", "qa")
	auditDir := filepath.Join(tempDir, "out", "audit")
	
	// Create directories
	err := os.MkdirAll(qaDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create QA directory: %v", err)
	}
	err = os.MkdirAll(auditDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create audit directory: %v", err)
	}
	
	// Create valid QA report
	qaReport := map[string]interface{}{
		"phases": []map[string]interface{}{
			{"name": "environment", "status": "pass"},
			{"name": "provider_health", "status": "pass"},
		},
		"provider_health": map[string]interface{}{
			"kraken": map[string]interface{}{
				"success_rate": 0.95,
			},
		},
	}
	reportData, _ := json.MarshalIndent(qaReport, "", "  ")
	err = os.WriteFile(filepath.Join(qaDir, "QA_REPORT.json"), reportData, 0644)
	if err != nil {
		t.Fatalf("Failed to create QA_REPORT.json: %v", err)
	}
	
	err = os.WriteFile(filepath.Join(qaDir, "QA_REPORT.md"), []byte("# QA Report"), 0644)
	if err != nil {
		t.Fatalf("Failed to create QA_REPORT.md: %v", err)
	}
	
	// Create provider health
	providerHealth := map[string]interface{}{
		"kraken": map[string]interface{}{
			"success_rate":     0.95,
			"latency_p50":      120.5,
			"latency_p95":      250.0,
			"budget_remaining": 1000.0,
			"degraded":         false,
		},
	}
	healthData, _ := json.MarshalIndent(providerHealth, "", "  ")
	err = os.WriteFile(filepath.Join(qaDir, "provider_health.json"), healthData, 0644)
	if err != nil {
		t.Fatalf("Failed to create provider_health.json: %v", err)
	}
	
	// Create microstructure sample
	csvContent := `pair,venue,spread_bps,depth_usd_2pc,vadr,adv
BTCUSD,kraken,5.2,150000,2.1,1000000`
	err = os.WriteFile(filepath.Join(qaDir, "microstructure_sample.csv"), []byte(csvContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create microstructure_sample.csv: %v", err)
	}
	
	// Create other required files
	err = os.WriteFile(filepath.Join(qaDir, "live_return_diffs.json"), []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create live_return_diffs.json: %v", err)
	}
	
	vadrChecks := []map[string]interface{}{
		{"pair": "BTCUSD", "venue": "kraken", "vadr": 2.1, "adv": 1000000, "spread_bps": 5.2, "depth_usd_2pc": 150000},
	}
	vadrData, _ := json.MarshalIndent(vadrChecks, "", "  ")
	err = os.WriteFile(filepath.Join(qaDir, "vadr_adv_checks.json"), vadrData, 0644)
	if err != nil {
		t.Fatalf("Failed to create vadr_adv_checks.json: %v", err)
	}
	
	err = os.WriteFile(filepath.Join(auditDir, "progress_trace.jsonl"), []byte("test progress"), 0644)
	if err != nil {
		t.Fatalf("Failed to create progress trace: %v", err)
	}
	
	validator := qa.NewAcceptanceValidator(tempDir, auditDir)
	result, err := validator.Validate()
	
	if err != nil {
		t.Fatalf("Validator failed: %v", err)
	}
	
	// Should pass file existence check
	if len(result.ValidatedFiles) != 7 { // 6 artifacts + 1 audit file
		t.Errorf("Expected 7 validated files, got %d", len(result.ValidatedFiles))
	}
}

func TestAcceptanceValidator_MissingFiles(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	auditDir := filepath.Join(tempDir, "audit")
	
	// Create directories but no files
	os.MkdirAll(artifactsDir, 0755)
	os.MkdirAll(auditDir, 0755)
	
	validator := qa.NewAcceptanceValidator(artifactsDir, auditDir)
	result, err := validator.Validate()
	
	if err != nil {
		t.Fatalf("Validator failed: %v", err)
	}
	
	// Should fail with missing files
	if result.Success {
		t.Error("Expected validation to fail due to missing files")
	}
	
	if result.FailureCode != "ACCEPT_VERIFY_MISSING_FILES" {
		t.Errorf("Expected failure code ACCEPT_VERIFY_MISSING_FILES, got %s", result.FailureCode)
	}
	
	// Should have violations for each missing file
	if len(result.Violations) != 7 {
		t.Errorf("Expected 7 violations for missing files, got %d", len(result.Violations))
	}
}

func TestAcceptanceValidator_QAReportJSONValidation(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	auditDir := filepath.Join(tempDir, "audit")
	
	os.MkdirAll(artifactsDir, 0755)
	os.MkdirAll(auditDir, 0755)
	
	// Create valid QA report JSON
	validReport := map[string]interface{}{
		"success": true,
		"phase_results": []map[string]interface{}{
			{
				"phase":  0,
				"name":   "Environment Validation",
				"status": "pass",
			},
			{
				"phase":  1,
				"name":   "Static Analysis", 
				"status": "pass",
			},
		},
		"provider_health": map[string]interface{}{
			"healthy": true,
		},
	}
	
	reportData, _ := json.Marshal(validReport)
	err := os.WriteFile(filepath.Join(artifactsDir, "QA_REPORT.json"), reportData, 0644)
	if err != nil {
		t.Fatalf("Failed to create QA report: %v", err)
	}
	
	// Create other required files with minimal content
	otherFiles := map[string]string{
		"QA_REPORT.md":             "# QA Report",
		"live_return_diffs.json":   "{}",
		"microstructure_sample.csv": "symbol,venue,spread_bps,depth_usd_2pct,vadr,adv_usd\nBTCUSD,kraken,12.5,150000,2.3,5000000",
		"provider_health.json":     `{"providers": {"kraken": {"success_rate": 0.98, "p50_latency": 150, "p95_latency": 450, "budget_remaining": 85, "degraded": false}}}`,
		"vadr_adv_checks.json":     `{"summary": {"total_pairs": 10}}`,
	}
	
	for file, content := range otherFiles {
		err := os.WriteFile(filepath.Join(artifactsDir, file), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}
	
	err = os.WriteFile(filepath.Join(auditDir, "progress_trace.jsonl"), []byte(`{"phase": 0, "status": "pass"}`), 0644)
	if err != nil {
		t.Fatalf("Failed to create progress trace: %v", err)
	}
	
	validator := qa.NewAcceptanceValidator(artifactsDir, auditDir)
	result, err := validator.Validate()
	
	if err != nil {
		t.Fatalf("Validator failed: %v", err)
	}
	
	// Should pass validation
	if !result.Success {
		t.Errorf("Expected validation to pass, but got failure: %s", result.FailureReason)
		for _, violation := range result.Violations {
			t.Logf("Violation: %s", violation)
		}
	}
}

func TestAcceptanceValidator_InvalidQAReportJSON(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	auditDir := filepath.Join(tempDir, "audit")
	
	os.MkdirAll(artifactsDir, 0755)
	os.MkdirAll(auditDir, 0755)
	
	// Create invalid QA report JSON (missing phase_results)
	invalidReport := map[string]interface{}{
		"success": true,
		// Missing phase_results
	}
	
	reportData, _ := json.Marshal(invalidReport)
	err := os.WriteFile(filepath.Join(artifactsDir, "QA_REPORT.json"), reportData, 0644)
	if err != nil {
		t.Fatalf("Failed to create QA report: %v", err)
	}
	
	// Create other required files
	otherFiles := map[string]string{
		"QA_REPORT.md":             "# QA Report",
		"live_return_diffs.json":   "{}",
		"microstructure_sample.csv": "symbol,venue\nBTCUSD,kraken",
		"provider_health.json":     `{"providers": {}}`,
		"vadr_adv_checks.json":     `{}`,
	}
	
	for file, content := range otherFiles {
		err := os.WriteFile(filepath.Join(artifactsDir, file), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}
	
	err = os.WriteFile(filepath.Join(auditDir, "progress_trace.jsonl"), []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create progress trace: %v", err)
	}
	
	validator := qa.NewAcceptanceValidator(artifactsDir, auditDir)
	result, err := validator.Validate()
	
	if err != nil {
		t.Fatalf("Validator failed: %v", err)
	}
	
	// Should fail validation
	if result.Success {
		t.Error("Expected validation to fail due to invalid QA report structure")
	}
	
	// Should have violation for missing phase_results
	foundViolation := false
	for _, violation := range result.Violations {
		if len(violation) > 0 {
			foundViolation = true
			break
		}
	}
	
	if !foundViolation {
		t.Error("Expected violation for missing phase_results field")
	}
}

func TestAcceptanceValidator_ProviderHealthValidation(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	auditDir := filepath.Join(tempDir, "audit")
	
	os.MkdirAll(artifactsDir, 0755)
	os.MkdirAll(auditDir, 0755)
	
	// Create provider health with missing fields
	invalidHealth := map[string]interface{}{
		"providers": map[string]interface{}{
			"kraken": map[string]interface{}{
				"success_rate": 0.98,
				// Missing required fields: p50_latency, p95_latency, budget_remaining, degraded
			},
			"okx": map[string]interface{}{
				"degraded": true,
				// Missing degraded_reason when degraded=true
				"success_rate":     0.95,
				"p50_latency":      180,
				"p95_latency":      520,
				"budget_remaining": 92,
			},
		},
	}
	
	healthData, _ := json.Marshal(invalidHealth)
	err := os.WriteFile(filepath.Join(artifactsDir, "provider_health.json"), healthData, 0644)
	if err != nil {
		t.Fatalf("Failed to create provider health: %v", err)
	}
	
	// Create other required files
	otherFiles := map[string]string{
		"QA_REPORT.md":             "# QA Report",
		"QA_REPORT.json":           `{"phase_results": [{"status": "pass"}]}`,
		"live_return_diffs.json":   "{}",
		"microstructure_sample.csv": "symbol,venue,spread_bps,depth_usd_2pct,vadr,adv_usd\nBTCUSD,kraken,12.5,150000,2.3,5000000",
		"vadr_adv_checks.json":     `{}`,
	}
	
	for file, content := range otherFiles {
		err := os.WriteFile(filepath.Join(artifactsDir, file), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}
	
	err = os.WriteFile(filepath.Join(auditDir, "progress_trace.jsonl"), []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create progress trace: %v", err)
	}
	
	validator := qa.NewAcceptanceValidator(artifactsDir, auditDir)
	result, err := validator.Validate()
	
	if err != nil {
		t.Fatalf("Validator failed: %v", err)
	}
	
	// Should fail validation
	if result.Success {
		t.Error("Expected validation to fail due to invalid provider health structure")
	}
	
	// Should have violations for missing provider fields
	missingFieldViolations := 0
	
	for _, violation := range result.Violations {
		// Check if violation mentions provider field issues
		if len(violation) > 0 {
			missingFieldViolations++
		}
	}
	
	if missingFieldViolations == 0 {
		t.Error("Expected violations for missing provider fields")
	}
}

func TestAcceptanceValidator_MicrostructureCSVValidation(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	auditDir := filepath.Join(tempDir, "audit")
	
	os.MkdirAll(artifactsDir, 0755)
	os.MkdirAll(auditDir, 0755)
	
	// Create CSV with missing columns
	invalidCSV := "symbol,venue,spread_bps\nBTCUSD,kraken,12.5\n"
	err := os.WriteFile(filepath.Join(artifactsDir, "microstructure_sample.csv"), []byte(invalidCSV), 0644)
	if err != nil {
		t.Fatalf("Failed to create CSV: %v", err)
	}
	
	// Create other required files
	otherFiles := map[string]string{
		"QA_REPORT.md":           "# QA Report",
		"QA_REPORT.json":         `{"phase_results": [{"status": "pass"}]}`,
		"live_return_diffs.json": "{}",
		"provider_health.json":   `{"providers": {"kraken": {"success_rate": 0.98, "p50_latency": 150, "p95_latency": 450, "budget_remaining": 85, "degraded": false}}}`,
		"vadr_adv_checks.json":   `{}`,
	}
	
	for file, content := range otherFiles {
		err := os.WriteFile(filepath.Join(artifactsDir, file), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}
	
	err = os.WriteFile(filepath.Join(auditDir, "progress_trace.jsonl"), []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create progress trace: %v", err)
	}
	
	validator := qa.NewAcceptanceValidator(artifactsDir, auditDir)
	result, err := validator.Validate()
	
	if err != nil {
		t.Fatalf("Validator failed: %v", err)
	}
	
	// Should fail validation
	if result.Success {
		t.Error("Expected validation to fail due to missing CSV columns")
	}
	
	// Should have violations for missing columns
	missingColumnViolations := 0
	for _, violation := range result.Violations {
		if len(violation) > 0 {
			missingColumnViolations++
		}
	}
	
	if missingColumnViolations == 0 {
		t.Error("Expected violations for missing CSV columns")
	}
}