package unit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cryptorun/internal/qa"
)

func TestQARunner_BasicFlow(t *testing.T) {
	// Create temporary directories for testing
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	auditDir := filepath.Join(tempDir, "audit")

	config := qa.Config{
		Progress:      "plain",
		Resume:        false,
		TTL:           300 * time.Second,
		Venues:        []string{"kraken"},
		MaxSample:     5,
		ArtifactsDir:  artifactsDir,
		AuditDir:      auditDir,
		ProviderTTL:   300 * time.Second,
	}

	runner := qa.NewRunner(config)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := runner.Run(ctx)
	
	if err != nil {
		t.Fatalf("QA runner failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("QA result is nil")
	}
	
	// Should have 7 phases (0-6)
	if result.TotalPhases != 7 {
		t.Errorf("Expected 7 phases, got %d", result.TotalPhases)
	}
	
	// All phases should pass in basic test
	if result.PassedPhases != 7 {
		t.Errorf("Expected all 7 phases to pass, got %d", result.PassedPhases)
	}
	
	if !result.Success {
		t.Errorf("QA should succeed, but got failure: %s", result.FailureReason)
	}
	
	// Check artifacts directory was created
	if _, err := os.Stat(artifactsDir); os.IsNotExist(err) {
		t.Error("Artifacts directory was not created")
	}
	
	// Check that some artifacts were generated
	expectedFiles := []string{
		"QA_REPORT.md",
		"QA_REPORT.json",
		"provider_health.json",
		"microstructure_sample.csv",
		"vadr_adv_checks.json",
	}
	
	for _, file := range expectedFiles {
		artifactPath := filepath.Join(artifactsDir, file)
		if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
			t.Errorf("Expected artifact file %s was not created", file)
		}
	}
}

func TestQAPhases_IndividualExecution(t *testing.T) {
	phases := qa.GetQAPhases()
	
	if len(phases) != 7 {
		t.Errorf("Expected 7 QA phases, got %d", len(phases))
	}
	
	expectedPhases := []string{
		"Environment Validation",
		"Static Analysis", 
		"Live Index Diffs",
		"Microstructure Validation",
		"Determinism Validation",
		"Explainability Validation",
		"UX Validation",
	}
	
	for i, phase := range phases {
		if phase.Name() != expectedPhases[i] {
			t.Errorf("Phase %d: expected %s, got %s", i, expectedPhases[i], phase.Name())
		}
		
		// Test phase execution
		result := &qa.PhaseResult{
			Phase:   i,
			Name:    phase.Name(),
			Metrics: make(map[string]interface{}),
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), phase.Timeout())
		err := phase.Execute(ctx, result)
		cancel()
		
		if err != nil {
			t.Errorf("Phase %d (%s) failed: %v", i, phase.Name(), err)
		}
		
		if result.Status != "pass" && result.Status != "" {
			t.Errorf("Phase %d (%s) should pass or be unset, got %s", i, phase.Name(), result.Status)
		}
	}
}

func TestQARunner_Resume(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	auditDir := filepath.Join(tempDir, "audit")

	// First, run QA to generate progress file
	config1 := qa.Config{
		Progress:      "json",
		Resume:        false,
		TTL:           300 * time.Second,
		Venues:        []string{"kraken"},
		MaxSample:     5,
		ArtifactsDir:  artifactsDir,
		AuditDir:      auditDir,
		ProviderTTL:   300 * time.Second,
	}

	runner1 := qa.NewRunner(config1)
	
	ctx1, cancel1 := context.WithTimeout(context.Background(), 10*time.Second)
	_, err := runner1.Run(ctx1)
	cancel1()
	
	if err != nil {
		t.Fatalf("Initial QA run failed: %v", err)
	}
	
	// Now test resume functionality
	config2 := qa.Config{
		Progress:      "json",
		Resume:        true,  // Enable resume
		TTL:           300 * time.Second,
		Venues:        []string{"kraken"},
		MaxSample:     5,
		ArtifactsDir:  artifactsDir,
		AuditDir:      auditDir,
		ProviderTTL:   300 * time.Second,
	}

	runner2 := qa.NewRunner(config2)
	
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	result, err := runner2.Run(ctx2)
	cancel2()
	
	if err != nil {
		t.Fatalf("Resume QA run failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Resume QA should succeed, but got failure: %s", result.FailureReason)
	}
}

func TestQARunner_ConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      qa.Config
		expectError bool
	}{
		{
			name: "Valid config",
			config: qa.Config{
				Progress:      "plain",
				TTL:           300 * time.Second,
				Venues:        []string{"kraken"},
				MaxSample:     10,
				ArtifactsDir:  "/tmp/artifacts",
				AuditDir:      "/tmp/audit",
				ProviderTTL:   300 * time.Second,
			},
			expectError: false,
		},
		{
			name: "Empty venues",
			config: qa.Config{
				Progress:      "plain", 
				TTL:           300 * time.Second,
				Venues:        []string{},
				MaxSample:     10,
				ArtifactsDir:  "/tmp/artifacts",
				AuditDir:      "/tmp/audit",
				ProviderTTL:   300 * time.Second,
			},
			expectError: false, // Should default to standard venues
		},
		{
			name: "Zero max sample",
			config: qa.Config{
				Progress:      "plain",
				TTL:           300 * time.Second,
				Venues:        []string{"kraken"},
				MaxSample:     0,
				ArtifactsDir:  "/tmp/artifacts",
				AuditDir:      "/tmp/audit",
				ProviderTTL:   300 * time.Second,
			},
			expectError: false, // Should handle gracefully
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := qa.NewRunner(tt.config)
			if runner == nil {
				t.Error("QA runner should not be nil")
			}
		})
	}
}

func TestQAProgressTracker(t *testing.T) {
	tempDir := t.TempDir()
	
	tracker := qa.NewProgressTracker(tempDir)
	
	// Initially, should return -1 (no progress)
	lastPhase, err := tracker.GetLastCompletedPhase()
	if err != nil {
		t.Fatalf("GetLastCompletedPhase failed: %v", err)
	}
	if lastPhase != -1 {
		t.Errorf("Expected -1 for no progress, got %d", lastPhase)
	}
	
	// Record some phase progress
	result1 := qa.PhaseResult{
		Phase:    0,
		Name:     "Test Phase 0",
		Status:   "pass",
		Duration: 100 * time.Millisecond,
	}
	
	err = tracker.RecordPhase(0, result1)
	if err != nil {
		t.Fatalf("RecordPhase failed: %v", err)
	}
	
	result2 := qa.PhaseResult{
		Phase:    1,
		Name:     "Test Phase 1",
		Status:   "pass",
		Duration: 200 * time.Millisecond,
	}
	
	err = tracker.RecordPhase(1, result2)
	if err != nil {
		t.Fatalf("RecordPhase failed: %v", err)
	}
	
	// Should now return 1 as last completed phase
	lastPhase, err = tracker.GetLastCompletedPhase()
	if err != nil {
		t.Fatalf("GetLastCompletedPhase failed: %v", err)
	}
	if lastPhase != 1 {
		t.Errorf("Expected 1 as last completed phase, got %d", lastPhase)
	}
}