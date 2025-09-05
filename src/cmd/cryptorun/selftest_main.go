package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// handleResilientSelfTest runs comprehensive precision and resilience tests
func (ui *MenuUI) handleResilientSelfTest(ctx context.Context) error {
	fmt.Println("🧪 Resilience Self-Test Suite")
	fmt.Println("Testing precision semantics, error handling, and network resilience")
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println()

	startTime := time.Now()

	// Test results
	var results TestSuiteResults
	
	// Run precision tests
	fmt.Println("🔍 1. Precision Semantics Tests")
	fmt.Println("   Testing HALF-UP rounding, inclusive thresholds, and borderline cases...")
	precisionPass, precisionDetails := ui.runPrecisionTests(ctx)
	results.PrecisionPass = precisionPass
	results.PrecisionDetails = precisionDetails

	// Run resilience tests
	fmt.Println("\n🛡️  2. Network Resilience Tests")
	fmt.Println("   Testing timeout handling, malformed JSON, and empty responses...")
	resiliencePass, resilienceDetails := ui.runResilienceTests(ctx)
	results.ResiliencePass = resiliencePass
	results.ResilienceDetails = resilienceDetails

	// Run integration tests
	fmt.Println("\n⚙️  3. Circuit Breaker Tests")
	fmt.Println("   Testing graceful degradation and recovery patterns...")
	circuitPass, circuitDetails := ui.runCircuitBreakerTests(ctx)
	results.CircuitBreakerPass = circuitPass
	results.CircuitBreakerDetails = circuitDetails

	duration := time.Since(startTime)

	// Display results summary
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("📊 SELF-TEST RESULTS")
	fmt.Println(strings.Repeat("=", 72))
	
	ui.printTestResult("Precision Semantics", results.PrecisionPass, results.PrecisionDetails)
	ui.printTestResult("Network Resilience", results.ResiliencePass, results.ResilienceDetails)  
	ui.printTestResult("Circuit Breaker", results.CircuitBreakerPass, results.CircuitBreakerDetails)
	
	fmt.Printf("\n⏱️  Total execution time: %v\n", duration.Round(time.Millisecond))
	
	// Overall status
	overallPass := results.PrecisionPass && results.ResiliencePass && results.CircuitBreakerPass
	if overallPass {
		fmt.Println("\n✅ ALL TESTS PASSED - System is resilient and ready for production")
	} else {
		fmt.Println("\n❌ SOME TESTS FAILED - Review failures before production deployment")
		
		// List specific failures
		failures := []string{}
		if !results.PrecisionPass {
			failures = append(failures, "Precision Semantics")
		}
		if !results.ResiliencePass {
			failures = append(failures, "Network Resilience")
		}
		if !results.CircuitBreakerPass {
			failures = append(failures, "Circuit Breaker")
		}
		
		fmt.Printf("   Failed areas: %s\n", strings.Join(failures, ", "))
	}
	
	fmt.Println()
	return nil
}

// TestSuiteResults holds results from all test categories
type TestSuiteResults struct {
	PrecisionPass        bool
	PrecisionDetails     string
	ResiliencePass       bool
	ResilienceDetails    string
	CircuitBreakerPass   bool
	CircuitBreakerDetails string
}

// runPrecisionTests executes precision-focused unit tests
func (ui *MenuUI) runPrecisionTests(ctx context.Context) (bool, string) {
	// Run specific precision tests
	testCmd := []string{
		"go", "test", 
		"./tests/unit", 
		"-run", "TestRoundBps|TestComputeSpreadBps|TestSpreadGate_Inclusive|TestDepthGate_Inclusive|TestDepth2pcUSD|TestVADRGate_NaN|TestGuardFinite|TestGuardPositive",
		"-v",
	}
	
	output, success := ui.runGoTest(ctx, testCmd, "precision tests")
	
	if success {
		return true, "✅ All precision semantics verified (HALF-UP rounding, inclusive thresholds)"
	} else {
		return false, fmt.Sprintf("❌ Precision test failures detected:\n%s", output)
	}
}

// runResilienceTests executes network resilience integration tests
func (ui *MenuUI) runResilienceTests(ctx context.Context) (bool, string) {
	// Run resilience integration tests
	testCmd := []string{
		"go", "test",
		"./tests/integration",
		"-run", "TestTimeoutResilience|TestBadJSONResilience|TestEmptyBookResilience|TestWinnersFetcherResilience",
		"-v",
	}
	
	output, success := ui.runGoTest(ctx, testCmd, "resilience tests")
	
	if success {
		return true, "✅ Network resilience confirmed (timeout/JSON/empty book handling)"
	} else {
		return false, fmt.Sprintf("❌ Resilience test failures:\n%s", output)
	}
}

// runCircuitBreakerTests executes circuit breaker and degradation tests
func (ui *MenuUI) runCircuitBreakerTests(ctx context.Context) (bool, string) {
	// Run circuit breaker tests
	testCmd := []string{
		"go", "test",
		"./tests/integration", 
		"-run", "TestCircuitBreakerBehavior|TestGracefulDegradation",
		"-v",
	}
	
	output, success := ui.runGoTest(ctx, testCmd, "circuit breaker tests")
	
	if success {
		return true, "✅ Circuit breaker behavior verified (graceful degradation & recovery)"
	} else {
		return false, fmt.Sprintf("❌ Circuit breaker test issues:\n%s", output)
	}
}

// runGoTest executes a go test command and returns output and success status
func (ui *MenuUI) runGoTest(ctx context.Context, testCmd []string, testType string) (string, bool) {
	// Set working directory to project root
	wd, _ := os.Getwd()
	
	// Create command
	cmd := exec.CommandContext(ctx, testCmd[0], testCmd[1:]...)
	cmd.Dir = wd
	
	// Set environment
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	
	// Execute command
	fmt.Printf("   Running %s...", testType)
	
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	
	if err != nil {
		fmt.Printf(" ❌ FAILED\n")
		return fmt.Sprintf("Command failed: %v\nOutput:\n%s", err, outputStr), false
	} else {
		fmt.Printf(" ✅ PASSED\n")
		return outputStr, true
	}
}

// printTestResult displays a formatted test result
func (ui *MenuUI) printTestResult(testName string, passed bool, details string) {
	status := "❌ FAIL"
	if passed {
		status = "✅ PASS"
	}
	
	fmt.Printf("%-20s %s\n", testName+":", status)
	
	// Show details for failed tests or brief success message
	if !passed {
		// Show first few lines of failure details
		lines := strings.Split(details, "\n")
		maxLines := 3
		if len(lines) > maxLines {
			for i := 0; i < maxLines; i++ {
				if strings.TrimSpace(lines[i]) != "" {
					fmt.Printf("   %s\n", lines[i])
				}
			}
			fmt.Printf("   ... (truncated, see full logs above)\n")
		} else {
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					fmt.Printf("   %s\n", line)
				}
			}
		}
	} else {
		fmt.Printf("   %s\n", details)
	}
}

// RunStandaloneSelfTest runs the self-test suite from CLI without menu
func RunStandaloneSelfTest() error {
	fmt.Println("🧪 CryptoRun Resilience Self-Test Suite")
	fmt.Println("======================================")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	// Create a minimal UI for running tests
	ui := &MenuUI{}
	
	err := ui.handleResilientSelfTest(ctx)
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "Self-test execution failed: %v\n", err)
		return err
	}
	
	return nil
}

// GetProjectRoot attempts to find the project root directory
func GetProjectRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	
	// Look for go.mod file walking up the directory tree
	for dir := wd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
	}
	
	// Fallback to current directory
	return wd, nil
}

// getGoExecutable returns the path to the go executable
func getGoExecutable() string {
	if runtime.GOOS == "windows" {
		return "go.exe"
	}
	return "go"
}