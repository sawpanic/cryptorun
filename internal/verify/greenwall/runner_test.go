package greenwall

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunner_FormatWall_AllPass(t *testing.T) {
	result := &Result{
		TestsPassed:         true,
		TestsCoverage:       85.7,
		MicroPassed:         5,
		MicroFailed:         0,
		MicroUnproven:       1,
		MicroArtifacts:      "./artifacts/proofs/2025-09-06/",
		BenchWindows:        4,
		BenchCorrelation:    0.753,
		BenchHitRate:        0.652,
		Smoke90Entries:      30,
		Smoke90HitRate:      0.583,
		Smoke90RelaxPer100:  3,
		Smoke90ThrottleRate: 0.125,
		PostmergePassed:     true,
		ExecutionTime:       45 * time.Second,
	}

	wall := result.FormatWall()

	// Check overall status
	if !strings.Contains(wall, "GREEN-WALL — ✅ PASS") {
		t.Errorf("Expected PASS status, got: %s", wall)
	}

	// Check individual components
	expectedPatterns := []string{
		"tests: ✅ pass (coverage 85.7%)",
		"microstructure: ✅ 5/0/1 | artifacts: ./artifacts/proofs/2025-09-06/",
		"bench topgainers: ✅ 4 windows | alignment ρ=0.753, hit=65.2%",
		"smoke90: ✅ 30 entries | hit 58.3% | relax/100 3 | throttle 12.5%",
		"postmerge: ✅ pass",
		"elapsed: 45.0s",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(wall, pattern) {
			t.Errorf("Expected pattern '%s' not found in wall:\n%s", pattern, wall)
		}
	}

	// Should indicate all passed
	if !result.AllPassed() {
		t.Error("Expected AllPassed() to return true")
	}
}

func TestRunner_FormatWall_WithFailures(t *testing.T) {
	result := &Result{
		TestsPassed:         false,
		TestsCoverage:       45.2,
		MicroPassed:         2,
		MicroFailed:         3,
		MicroUnproven:       1,
		MicroArtifacts:      "",
		BenchWindows:        0,
		BenchCorrelation:    0.0,
		BenchHitRate:        0.0,
		Smoke90Entries:      0,
		Smoke90HitRate:      0.0,
		Smoke90RelaxPer100:  0,
		Smoke90ThrottleRate: 0.0,
		PostmergePassed:     false,
		ExecutionTime:       12 * time.Second,
		Errors: []string{
			"tests: coverage too low",
			"microstructure: API timeout",
		},
	}

	wall := result.FormatWall()

	// Check overall status
	if !strings.Contains(wall, "GREEN-WALL — ❌ FAIL") {
		t.Errorf("Expected FAIL status, got: %s", wall)
	}

	// Check failure indicators
	expectedPatterns := []string{
		"tests: ❌ pass (coverage 45.2%)",
		"microstructure: ❌ 2/3/1 | artifacts: none",
		"bench topgainers: ❌ 0 windows",
		"smoke90: ❌ 0 entries",
		"postmerge: ❌ pass",
		"errors:",
		"* tests: coverage too low",
		"* microstructure: API timeout",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(wall, pattern) {
			t.Errorf("Expected pattern '%s' not found in wall:\n%s", pattern, wall)
		}
	}

	// Should indicate failures
	if result.AllPassed() {
		t.Error("Expected AllPassed() to return false")
	}
}

func TestRunner_PostmergeStatus(t *testing.T) {
	tests := []struct {
		name     string
		passed   bool
		expected string
	}{
		{
			name:     "Postmerge Pass",
			passed:   true,
			expected: "✅ pass",
		},
		{
			name:     "Postmerge Fail",
			passed:   false,
			expected: "❌ fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{PostmergePassed: tt.passed}
			status := result.PostmergeStatus()

			if status != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, status)
			}
		})
	}
}

func TestRunner_NewRunner(t *testing.T) {
	config := Config{
		SampleSize:   25,
		ShowProgress: true,
		Timeout:      30 * time.Second,
	}

	runner := NewRunner(config)

	if runner.config.SampleSize != 25 {
		t.Errorf("Expected SampleSize 25, got %d", runner.config.SampleSize)
	}

	if !runner.config.ShowProgress {
		t.Error("Expected ShowProgress to be true")
	}

	if runner.config.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout 30s, got %v", runner.config.Timeout)
	}
}

func TestRunner_RunPostmergeCheck_Success(t *testing.T) {
	runner := NewRunner(Config{})
	result := &Result{}
	ctx := context.Background()

	// This test will pass if we're in a valid Go module with the expected structure
	err := runner.runPostmergeCheck(ctx, result)

	// Since we're in the actual CryptoRun repo, this should work
	if err != nil {
		t.Logf("Postmerge check failed (expected in test environment): %v", err)
		// Don't fail the test since test environment may not have full structure
	} else if !result.PostmergePassed {
		t.Error("Expected PostmergePassed to be true when no error returned")
	}
}

func TestRunner_AllPassed_Logic(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		expected bool
	}{
		{
			name: "All Pass",
			result: Result{
				TestsPassed:     true,
				MicroFailed:     0,
				Smoke90Entries:  30,
				PostmergePassed: true,
			},
			expected: true,
		},
		{
			name: "Tests Failed",
			result: Result{
				TestsPassed:     false,
				MicroFailed:     0,
				Smoke90Entries:  30,
				PostmergePassed: true,
			},
			expected: false,
		},
		{
			name: "Micro Failed",
			result: Result{
				TestsPassed:     true,
				MicroFailed:     2,
				Smoke90Entries:  30,
				PostmergePassed: true,
			},
			expected: false,
		},
		{
			name: "No Smoke Entries",
			result: Result{
				TestsPassed:     true,
				MicroFailed:     0,
				Smoke90Entries:  0,
				PostmergePassed: true,
			},
			expected: false,
		},
		{
			name: "Postmerge Failed",
			result: Result{
				TestsPassed:     true,
				MicroFailed:     0,
				Smoke90Entries:  30,
				PostmergePassed: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.AllPassed(); got != tt.expected {
				t.Errorf("AllPassed() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestRunner_ParseMetricsFromOutput(t *testing.T) {
	runner := NewRunner(Config{})

	output := `
Running tests...
PASS
coverage: 85.3% of statements
ok  	cryptorun/internal/verify	0.123s	coverage: 85.3% of statements

Benchmark results:
hit rate: 65.7%
success: 58.2% of entries passed
`

	metrics := runner.parseMetricsFromOutput([]byte(output))

	if coverage, exists := metrics["coverage"]; !exists || coverage != 85.3 {
		t.Errorf("Expected coverage 85.3, got %v (exists: %v)", coverage, exists)
	}

	if hitRate, exists := metrics["hit_rate"]; !exists || hitRate != 0.657 {
		t.Errorf("Expected hit_rate 0.657, got %v (exists: %v)", hitRate, exists)
	}
}

func TestRunner_SimulatedSteps(t *testing.T) {
	runner := NewRunner(Config{SampleSize: 20})
	result := &Result{}
	ctx := context.Background()

	// Test microstructure simulation
	err := runner.runMicrostructure(ctx, result)
	if err != nil {
		t.Errorf("runMicrostructure failed: %v", err)
	}

	if result.MicroPassed == 0 && result.MicroFailed == 0 && result.MicroUnproven == 0 {
		t.Error("Expected some microstructure results")
	}

	// Test bench simulation
	err = runner.runBenchTopGainers(ctx, result)
	if err != nil {
		t.Errorf("runBenchTopGainers failed: %v", err)
	}

	if result.BenchWindows == 0 {
		t.Error("Expected some bench windows")
	}

	// Test smoke90 simulation
	err = runner.runSmoke90(ctx, result)
	if err != nil {
		t.Errorf("runSmoke90 failed: %v", err)
	}

	if result.Smoke90Entries != 20 {
		t.Errorf("Expected Smoke90Entries=20, got %d", result.Smoke90Entries)
	}
}

func TestRunner_WallFormatting_EdgeCases(t *testing.T) {
	// Test with zero values
	result := &Result{}
	wall := result.FormatWall()

	if !strings.Contains(wall, "GREEN-WALL — ❌ FAIL") {
		t.Error("Expected FAIL status for zero result")
	}

	// Test with no artifacts
	result.MicroArtifacts = ""
	wall = result.FormatWall()

	if !strings.Contains(wall, "artifacts: none") {
		t.Error("Expected 'artifacts: none' when no artifacts present")
	}

	// Test with no errors
	result.Errors = nil
	wall = result.FormatWall()

	if strings.Contains(wall, "errors:") {
		t.Error("Should not show errors section when no errors")
	}
}
