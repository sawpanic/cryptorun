package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/qa"
)

// TestQACommand_EndToEnd tests the full QA command execution
func TestQACommand_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "qa")
	auditDir := filepath.Join(tempDir, "audit")

	config := qa.Config{
		Progress:     "json",
		Resume:       false,
		TTL:          300 * time.Second,
		Venues:       []string{"kraken", "okx", "coinbase"},
		MaxSample:    10,
		ArtifactsDir: artifactsDir,
		AuditDir:     auditDir,
		ProviderTTL:  300 * time.Second,
	}

	runner := qa.NewRunner(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := runner.Run(ctx)

	if err != nil {
		t.Fatalf("QA integration test failed: %v", err)
	}

	if result == nil {
		t.Fatal("QA result is nil")
	}

	// Verify all phases completed
	if len(result.PhaseResults) != 7 {
		t.Errorf("Expected 7 phase results, got %d", len(result.PhaseResults))
	}

	// Verify artifacts were generated
	expectedArtifacts := []string{
		"QA_REPORT.md",
		"QA_REPORT.json",
		"provider_health.json",
		"microstructure_sample.csv",
		"vadr_adv_checks.json",
	}

	for _, artifact := range expectedArtifacts {
		artifactPath := filepath.Join(artifactsDir, artifact)
		if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
			t.Errorf("Expected artifact %s not found", artifact)
		} else {
			// Check file is not empty
			info, err := os.Stat(artifactPath)
			if err == nil && info.Size() == 0 {
				t.Errorf("Artifact %s is empty", artifact)
			}
		}
	}

	// Verify audit trail was created
	auditFile := filepath.Join(auditDir, "progress_trace.jsonl")
	if _, err := os.Stat(auditFile); os.IsNotExist(err) {
		t.Error("Progress trace file not found")
	}

	// Log results for manual inspection
	t.Logf("QA completed in %v", result.TotalDuration)
	t.Logf("Passed phases: %d/%d", result.PassedPhases, result.TotalPhases)
	t.Logf("Healthy providers: %d", result.HealthyProviders)

	if !result.Success {
		t.Logf("QA failed: %s", result.FailureReason)
		if result.Hint != "" {
			t.Logf("Hint: %s", result.Hint)
		}
	}
}

// TestQACommand_StressTest runs QA with higher concurrency and samples
func TestQACommand_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	if os.Getenv("QA_STRESS_TEST") == "" {
		t.Skip("Set QA_STRESS_TEST=1 to run stress tests")
	}

	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "qa")
	auditDir := filepath.Join(tempDir, "audit")

	config := qa.Config{
		Progress:     "json",
		Resume:       false,
		TTL:          60 * time.Second, // Shorter TTL for stress test
		Venues:       []string{"kraken", "okx", "coinbase"},
		MaxSample:    50, // Higher sample count
		ArtifactsDir: artifactsDir,
		AuditDir:     auditDir,
		ProviderTTL:  60 * time.Second,
	}

	runner := qa.NewRunner(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	startTime := time.Now()
	result, err := runner.Run(ctx)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("QA stress test failed: %v", err)
	}

	// Performance assertions
	if duration > 8*time.Minute {
		t.Errorf("QA took too long: %v (expected < 8min)", duration)
	}

	// Should handle stress without degradation
	if result.HealthyProviders == 0 {
		t.Error("All providers became unhealthy during stress test")
	}

	t.Logf("Stress test completed in %v", duration)
	t.Logf("Provider health: %d healthy", result.HealthyProviders)
}

// TestQACommand_ProviderDegradation tests QA behavior when providers fail
func TestQACommand_ProviderDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping degradation test in short mode")
	}

	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "qa")
	auditDir := filepath.Join(tempDir, "audit")

	// Use invalid venues to trigger degradation
	config := qa.Config{
		Progress:     "plain",
		Resume:       false,
		TTL:          300 * time.Second,
		Venues:       []string{"invalid_venue"}, // This should cause degradation
		MaxSample:    5,
		ArtifactsDir: artifactsDir,
		AuditDir:     auditDir,
		ProviderTTL:  300 * time.Second,
	}

	runner := qa.NewRunner(config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := runner.Run(ctx)

	// QA should complete even with provider degradation
	if err != nil {
		t.Fatalf("QA should handle degraded providers gracefully: %v", err)
	}

	// Some phases might fail due to provider issues, but QA should provide hints
	if !result.Success && result.Hint == "" {
		t.Error("Failed QA should provide hints for resolution")
	}

	t.Logf("Degradation test: %s", result.FailureReason)
	if result.Hint != "" {
		t.Logf("Hint provided: %s", result.Hint)
	}
}

// TestQACommand_Resume tests the resume functionality with real artifacts
func TestQACommand_Resume(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resume test in short mode")
	}

	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "qa")
	auditDir := filepath.Join(tempDir, "audit")

	config := qa.Config{
		Progress:     "json",
		Resume:       false,
		TTL:          300 * time.Second,
		Venues:       []string{"kraken"},
		MaxSample:    5,
		ArtifactsDir: artifactsDir,
		AuditDir:     auditDir,
		ProviderTTL:  300 * time.Second,
	}

	// First run - complete execution
	runner1 := qa.NewRunner(config)
	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Minute)
	result1, err := runner1.Run(ctx1)
	cancel1()

	if err != nil {
		t.Fatalf("Initial QA run failed: %v", err)
	}

	if !result1.Success {
		t.Fatalf("Initial QA should succeed: %s", result1.FailureReason)
	}

	// Second run - with resume enabled
	config.Resume = true
	runner2 := qa.NewRunner(config)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Minute)
	result2, err := runner2.Run(ctx2)
	cancel2()

	if err != nil {
		t.Fatalf("Resume QA run failed: %v", err)
	}

	// Resume should be faster since phases are already completed
	if result2.TotalDuration > result1.TotalDuration {
		t.Logf("Resume took %v vs initial %v - may indicate phases were re-executed",
			result2.TotalDuration, result1.TotalDuration)
	}

	t.Logf("Initial run: %v", result1.TotalDuration)
	t.Logf("Resume run: %v", result2.TotalDuration)
}
