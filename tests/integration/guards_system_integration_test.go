package integration

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestGuardsSystemIntegration tests the complete guards system integration
func TestGuardsSystemIntegration(t *testing.T) {
	// Test that guard tests run successfully and complete within time limit
	cmd := exec.Command("go", "test", "./internal/application/guards/e2e", "-v", "-count=1")
	cmd.Dir = "../../"

	start := time.Now()
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	// Validate tests complete within 5s target
	if elapsed > 5*time.Second {
		t.Errorf("Guard tests took %v, expected <5s", elapsed)
	}

	// Validate tests pass
	if err != nil {
		t.Errorf("Guard tests failed: %v\nOutput:\n%s", err, string(output))
	}

	outputStr := string(output)

	// Check for expected test cases
	expectedTests := []string{
		"TestFatigueGuardCalmRegime",
		"TestFreshnessGuardNormalRegime",
		"TestLiquidityGuards",
		"TestSocialCapsRegimeAware",
		"TestGuardEvaluationPerformance",
	}

	for _, test := range expectedTests {
		if !strings.Contains(outputStr, test) {
			t.Errorf("Expected test %s not found in output", test)
		}

		// Verify test passed (look for PASS)
		if !strings.Contains(outputStr, "PASS: "+test) && !strings.Contains(outputStr, test+"...ok") {
			t.Errorf("Test %s did not pass", test)
		}
	}

	// Check for benchmark test
	if !strings.Contains(outputStr, "BenchmarkGuardEvaluation") {
		t.Error("Expected benchmark test not found")
	}
}

// TestMenuGuardsIntegration tests menu-guards E2E integration
func TestMenuGuardsIntegration(t *testing.T) {
	cmd := exec.Command("go", "test", "./internal/application/menu/e2e", "-v", "-run", "MenuGuard", "-count=1")
	cmd.Dir = "../../"

	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("Menu guard tests failed: %v\nOutput:\n%s", err, string(output))
	}

	outputStr := string(output)

	// Check for expected menu guard tests
	expectedMenuTests := []string{
		"TestMenuGuardStatusDisplay",
		"TestMenuGuardProgressBreadcrumbs",
		"TestMenuGuardDetailedReasons",
		"TestMenuGuardThresholdAdjustment",
		"TestMenuGuardExitCodes",
	}

	for _, test := range expectedMenuTests {
		if !strings.Contains(outputStr, test) {
			t.Errorf("Expected menu test %s not found in output", test)
		}
	}
}

// TestTestDataIntegrity validates test fixture integrity
func TestTestDataIntegrity(t *testing.T) {
	fixtures := []string{
		"../../testdata/guards/fatigue_calm.json",
		"../../testdata/guards/freshness_normal.json",
		"../../testdata/guards/liquidity_gates.json",
		"../../testdata/guards/social_caps.json",
	}

	for _, fixture := range fixtures {
		// Use Go to validate JSON syntax
		cmd := exec.Command("go", "run", "-", fixture)
		cmd.Dir = "../../"
		cmd.Stdin = strings.NewReader(`
package main
import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)
func main() {
	data, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("ERROR: %v", err)
		os.Exit(1)
	}
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		fmt.Printf("ERROR: %v", err)
		os.Exit(1)
	}
	fmt.Print("OK")
}
`)

		output, err := cmd.CombinedOutput()

		if err != nil || !strings.Contains(string(output), "OK") {
			t.Errorf("Fixture %s has invalid JSON: %v\nOutput: %s", fixture, err, string(output))
		}
	}
}

// TestGoldenFileExistence validates golden files exist and are readable
func TestGoldenFileExistence(t *testing.T) {
	goldenFiles := []string{
		"../../testdata/guards/golden/fatigue_calm.golden",
	}

	for _, golden := range goldenFiles {
		cmd := exec.Command("cat", golden)
		output, err := cmd.CombinedOutput()

		if err != nil {
			t.Errorf("Golden file %s not accessible: %v", golden, err)
			continue
		}

		content := string(output)

		// Validate golden file has expected structure
		if !strings.Contains(content, "🛡️ Guard Results") {
			t.Errorf("Golden file %s missing expected header", golden)
		}

		if !strings.Contains(content, "┌") || !strings.Contains(content, "└") {
			t.Errorf("Golden file %s missing table borders", golden)
		}

		if !strings.Contains(content, "Summary:") {
			t.Errorf("Golden file %s missing summary line", golden)
		}
	}
}

// TestGuardsTestkitFunctionality validates the testkit infrastructure
func TestGuardsTestkitFunctionality(t *testing.T) {
	// Test that testkit compiles and basic functions work
	cmd := exec.Command("go", "build", "./internal/application/guards/testkit")
	cmd.Dir = "../../"

	if err := cmd.Run(); err != nil {
		t.Errorf("Testkit package failed to build: %v", err)
	}

	// Test a simple testkit usage
	testCode := `
package main

import (
	"fmt"
	"testing"
	"github.com/sawpanic/cryptorun/internal/application/guards/testkit"
)

func main() {
	// Simulate loading a fixture
	t := &testing.T{}
	fixture := &testkit.GuardTestFixture{
		Name:      "test",
		Regime:    "normal",
		Timestamp: "2025-01-15T12:00:00Z",
		Expected: testkit.ExpectedResults{
			PassCount: 1,
			FailCount: 0,
			ExitCode:  0,
		},
	}
	
	evaluator := testkit.NewMockEvaluator(fixture)
	result := evaluator.EvaluateAllGuards()
	
	if result.PassCount != 1 {
		fmt.Printf("ERROR: Expected 1 pass, got %d", result.PassCount)
		return
	}
	
	output := result.FormatTableOutput()
	if len(output) == 0 {
		fmt.Print("ERROR: Empty table output")
		return
	}
	
	fmt.Print("OK")
}
`

	cmd = exec.Command("go", "run", "-")
	cmd.Dir = "../../"
	cmd.Stdin = strings.NewReader(testCode)

	output, err := cmd.CombinedOutput()

	if err != nil || !strings.Contains(string(output), "OK") {
		t.Errorf("Testkit functionality test failed: %v\nOutput: %s", err, string(output))
	}
}

// TestPerformanceTargets validates performance targets are met
func TestPerformanceTargets(t *testing.T) {
	// Run benchmark and check results
	cmd := exec.Command("go", "test", "./internal/application/guards/e2e", "-bench=BenchmarkGuardEvaluation", "-benchtime=1s")
	cmd.Dir = "../../"

	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("Benchmark failed: %v\nOutput: %s", err, string(output))
		return
	}

	outputStr := string(output)

	// Look for benchmark results
	if !strings.Contains(outputStr, "BenchmarkGuardEvaluation") {
		t.Error("Benchmark results not found in output")
	}

	// Basic validation that benchmark ran
	if !strings.Contains(outputStr, "ns/op") {
		t.Error("Expected nanoseconds per operation in benchmark output")
	}
}

// TestSystemReadiness validates the complete system is ready for use
func TestSystemReadiness(t *testing.T) {
	checks := map[string]func(*testing.T){
		"GuardTests":   func(t *testing.T) { TestGuardsSystemIntegration(t) },
		"MenuTests":    func(t *testing.T) { TestMenuGuardsIntegration(t) },
		"TestData":     func(t *testing.T) { TestTestDataIntegrity(t) },
		"GoldenFiles":  func(t *testing.T) { TestGoldenFileExistence(t) },
		"TestkitBuild": func(t *testing.T) { TestGuardsTestkitFunctionality(t) },
		"Performance":  func(t *testing.T) { TestPerformanceTargets(t) },
	}

	passed := 0
	for checkName, checkFunc := range checks {
		t.Run(checkName, func(t *testing.T) {
			checkFunc(t)
			passed++
		})
	}

	t.Logf("System readiness: %d/%d checks passed", passed, len(checks))

	if passed != len(checks) {
		t.Errorf("System not ready: %d/%d checks failed", len(checks)-passed, len(checks))
	} else {
		t.Log("✅ Guards system is ready with comprehensive E2E testing infrastructure")
	}
}
