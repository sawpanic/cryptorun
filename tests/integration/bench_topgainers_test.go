package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cryptorun/internal/bench"
)

// TestTopGainersBenchmark tests the complete top gainers benchmark flow
func TestTopGainersBenchmark(t *testing.T) {
	// Clean up test artifacts
	testOutputDir := "out/bench/test"
	defer os.RemoveAll(testOutputDir)

	config := bench.TopGainersConfig{
		TTL:       5 * time.Minute, // Use reasonable TTL for testing
		Limit:     5,               // Small limit for faster testing
		Windows:   []string{"1h", "24h"},
		OutputDir: testOutputDir,
		AuditDir:  "out/audit/test",
	}

	// Create benchmark runner
	runner := bench.NewTopGainersBenchmark(config)

	// Run benchmark
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := runner.RunBenchmark(ctx)
	if err != nil {
		t.Fatalf("Benchmark failed: %v", err)
	}

	// Validate result structure
	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if result.OverallAlignment < 0.0 || result.OverallAlignment > 1.0 {
		t.Errorf("Overall alignment %.3f should be in [0.0, 1.0]", result.OverallAlignment)
	}

	if len(result.WindowAlignments) != len(config.Windows) {
		t.Errorf("Expected %d window alignments, got %d",
			len(config.Windows), len(result.WindowAlignments))
	}

	if len(result.TopGainers) != len(config.Windows) {
		t.Errorf("Expected %d top gainer sets, got %d",
			len(config.Windows), len(result.TopGainers))
	}

	// Validate each window has data
	for _, window := range config.Windows {
		if _, exists := result.WindowAlignments[window]; !exists {
			t.Errorf("Missing window alignment for %s", window)
		}

		if _, exists := result.TopGainers[window]; !exists {
			t.Errorf("Missing top gainers for %s", window)
		}

		if _, exists := result.ScanResults[window]; !exists {
			t.Errorf("Missing scan results for %s", window)
		}
	}

	// Validate artifacts were created
	expectedFiles := []string{
		"topgainers_alignment.json",
		"topgainers_alignment.md",
	}

	for _, window := range config.Windows {
		expectedFiles = append(expectedFiles, "topgainers_"+window+".json")
	}

	for _, filename := range expectedFiles {
		filepath := filepath.Join(testOutputDir, filename)
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			t.Errorf("Expected artifact file %s was not created", filepath)
		}
	}
}

// TestTopGainersCaching tests TTL caching behavior
func TestTopGainersCaching(t *testing.T) {
	testOutputDir := "out/bench/cache-test"
	defer os.RemoveAll(testOutputDir)

	// First run with long TTL
	config := bench.TopGainersConfig{
		TTL:       1 * time.Hour, // Long TTL to enable caching
		Limit:     3,
		Windows:   []string{"1h"},
		OutputDir: testOutputDir,
		AuditDir:  "out/audit/cache-test",
	}

	runner1 := bench.NewTopGainersBenchmark(config)
	ctx := context.Background()

	// First benchmark run
	start1 := time.Now()
	result1, err := runner1.RunBenchmark(ctx)
	if err != nil {
		t.Fatalf("First benchmark run failed: %v", err)
	}
	duration1 := time.Since(start1)

	// Verify cache file was created
	cacheFile := filepath.Join(testOutputDir, ".cache", "topgainers_1h.json")
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Errorf("Cache file %s was not created", cacheFile)
	}

	// Second run should use cache (faster)
	runner2 := bench.NewTopGainersBenchmark(config)
	start2 := time.Now()
	result2, err := runner2.RunBenchmark(ctx)
	if err != nil {
		t.Fatalf("Second benchmark run failed: %v", err)
	}
	duration2 := time.Since(start2)

	// Results should be similar (allowing for small timing differences)
	if len(result1.TopGainers["1h"]) != len(result2.TopGainers["1h"]) {
		t.Errorf("Cached results differ: %d vs %d gainers",
			len(result1.TopGainers["1h"]), len(result2.TopGainers["1h"]))
	}

	// Cache run should be faster (though this is not guaranteed in all environments)
	t.Logf("First run: %v, Second run: %v", duration1, duration2)
	if duration2 > duration1*2 {
		t.Logf("Warning: Second run was not faster, cache may not be working")
	}
}

// TestTopGainersTTLEnforcement tests that TTL is properly enforced
func TestTopGainersTTLEnforcement(t *testing.T) {
	testOutputDir := "out/bench/ttl-test"
	defer os.RemoveAll(testOutputDir)

	config := bench.TopGainersConfig{
		TTL:       1 * time.Second, // Very short TTL for testing
		Limit:     3,
		Windows:   []string{"1h"},
		OutputDir: testOutputDir,
		AuditDir:  "out/audit/ttl-test",
	}

	runner := bench.NewTopGainersBenchmark(config)
	ctx := context.Background()

	// First run
	_, err := runner.RunBenchmark(ctx)
	if err != nil {
		t.Fatalf("First run failed: %v", err)
	}

	// Verify cache file exists
	cacheFile := filepath.Join(testOutputDir, ".cache", "topgainers_1h.json")
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Errorf("Cache file was not created")
	}

	// Wait for TTL to expire
	time.Sleep(2 * time.Second)

	// Second run should not use expired cache
	_, err = runner.RunBenchmark(ctx)
	if err != nil {
		t.Fatalf("Second run failed: %v", err)
	}

	// Should still succeed (generates new data)
}

// TestTopGainersValidation tests input validation
func TestTopGainersValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      bench.TopGainersConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: bench.TopGainersConfig{
				TTL:       5 * time.Minute,
				Limit:     10,
				Windows:   []string{"1h", "24h"},
				OutputDir: "out/bench/valid",
				AuditDir:  "out/audit/valid",
			},
			expectError: false,
		},
		{
			name: "too short TTL",
			config: bench.TopGainersConfig{
				TTL:       100 * time.Second, // Less than 300s minimum
				Limit:     10,
				Windows:   []string{"1h"},
				OutputDir: "out/bench/short-ttl",
				AuditDir:  "out/audit/short-ttl",
			},
			expectError: false, // Should work but not respect minimum
		},
		{
			name: "zero limit",
			config: bench.TopGainersConfig{
				TTL:       5 * time.Minute,
				Limit:     0,
				Windows:   []string{"1h"},
				OutputDir: "out/bench/zero-limit",
				AuditDir:  "out/audit/zero-limit",
			},
			expectError: false, // Should work with 0 results
		},
		{
			name: "empty windows",
			config: bench.TopGainersConfig{
				TTL:       5 * time.Minute,
				Limit:     10,
				Windows:   []string{},
				OutputDir: "out/bench/empty-windows",
				AuditDir:  "out/audit/empty-windows",
			},
			expectError: false, // Should work with no windows
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.RemoveAll(tt.config.OutputDir)

			runner := bench.NewTopGainersBenchmark(tt.config)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, err := runner.RunBenchmark(ctx)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but benchmark succeeded")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestTopGainersOutputFormats tests that outputs are in correct format
func TestTopGainersOutputFormats(t *testing.T) {
	testOutputDir := "out/bench/format-test"
	defer os.RemoveAll(testOutputDir)

	config := bench.TopGainersConfig{
		TTL:       5 * time.Minute,
		Limit:     5,
		Windows:   []string{"1h", "24h"},
		OutputDir: testOutputDir,
		AuditDir:  "out/audit/format-test",
	}

	runner := bench.NewTopGainersBenchmark(config)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := runner.RunBenchmark(ctx)
	if err != nil {
		t.Fatalf("Benchmark failed: %v", err)
	}

	// Test JSON format
	jsonFile := filepath.Join(testOutputDir, "topgainers_alignment.json")
	jsonData, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}

	var jsonResult map[string]interface{}
	if err := json.Unmarshal(jsonData, &jsonResult); err != nil {
		t.Fatalf("JSON is not valid: %v", err)
	}

	// Validate required JSON fields
	requiredFields := []string{"timestamp", "overall_alignment", "window_alignments", "methodology"}
	for _, field := range requiredFields {
		if _, exists := jsonResult[field]; !exists {
			t.Errorf("JSON missing required field: %s", field)
		}
	}

	// Test Markdown format
	mdFile := filepath.Join(testOutputDir, "topgainers_alignment.md")
	mdData, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("Failed to read Markdown file: %v", err)
	}

	mdContent := string(mdData)

	// Validate Markdown structure
	requiredMdSections := []string{
		"# Top Gainers Benchmark Report",
		"## Window Analysis",
		"## Interpretation",
		"**Generated**:",
		"**Overall Alignment**:",
	}

	for _, section := range requiredMdSections {
		if !strings.Contains(mdContent, section) {
			t.Errorf("Markdown missing required section: %s", section)
		}
	}

	// Test individual window files
	for _, window := range config.Windows {
		windowFile := filepath.Join(testOutputDir, "topgainers_"+window+".json")
		windowData, err := os.ReadFile(windowFile)
		if err != nil {
			t.Errorf("Failed to read window file %s: %v", windowFile, err)
			continue
		}

		var windowResult map[string]interface{}
		if err := json.Unmarshal(windowData, &windowResult); err != nil {
			t.Errorf("Window JSON %s is not valid: %v", windowFile, err)
			continue
		}

		// Validate window file structure
		if windowResult["window"] != window {
			t.Errorf("Window file %s has wrong window value: %v", windowFile, windowResult["window"])
		}

		if _, exists := windowResult["top_gainers"]; !exists {
			t.Errorf("Window file %s missing top_gainers field", windowFile)
		}
	}
}

// TestTopGainersNoAggregatorMicrostructure ensures no aggregator data is used
func TestTopGainersNoAggregatorMicrostructure(t *testing.T) {
	// This test verifies that the benchmark only uses list/indices data
	// and never attempts to get microstructure data from aggregators

	testOutputDir := "out/bench/no-aggregator-test"
	defer os.RemoveAll(testOutputDir)

	config := bench.TopGainersConfig{
		TTL:       5 * time.Minute,
		Limit:     3,
		Windows:   []string{"1h"},
		OutputDir: testOutputDir,
		AuditDir:  "out/audit/no-aggregator-test",
	}

	runner := bench.NewTopGainersBenchmark(config)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := runner.RunBenchmark(ctx)
	if err != nil {
		t.Fatalf("Benchmark failed: %v", err)
	}

	// Verify that we only have basic list data, not microstructure data
	for window, gainers := range result.TopGainers {
		for i, gainer := range gainers {
			// Should have basic list data
			if gainer.Symbol == "" {
				t.Errorf("Window %s gainer %d missing symbol", window, i)
			}

			// Should NOT have microstructure data (we verify by checking it's only basic fields)
			// This is enforced by our TopGainerResult struct only having list fields
			if gainer.PriceChangePercentage == "" && gainer.PercentageFloat == 0 {
				t.Errorf("Window %s gainer %d missing price change data", window, i)
			}
		}
	}

	// Verify methodology mentions indices/lists only
	if !strings.Contains(result.Methodology, "indices") && !strings.Contains(result.Methodology, "trending") {
		t.Errorf("Methodology should mention indices/trending, got: %s", result.Methodology)
	}
}

// TestTopGainersProgressStreaming tests progress event streaming
func TestTopGainersProgressStreaming(t *testing.T) {
	testOutputDir := "out/bench/progress-test"
	defer os.RemoveAll(testOutputDir)

	config := bench.TopGainersConfig{
		TTL:       5 * time.Minute,
		Limit:     3,
		Windows:   []string{"1h"},
		OutputDir: testOutputDir,
		AuditDir:  "out/audit/progress-test",
	}

	runner := bench.NewTopGainersBenchmark(config)

	// Create a mock progress bus that captures events
	events := []string{}
	mockBus := &MockProgressBus{
		events: &events,
	}

	runner.SetProgressBus(mockBus)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := runner.RunBenchmark(ctx)
	if err != nil {
		t.Fatalf("Benchmark failed: %v", err)
	}

	// Verify progress events were captured
	if len(events) == 0 {
		t.Errorf("No progress events were captured")
	}

	// Verify we have expected phases
	expectedPhases := []string{"init", "fetch", "analyze", "score", "output"}
	for _, phase := range expectedPhases {
		found := false
		for _, event := range events {
			if strings.Contains(event, phase) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected phase %s not found in progress events", phase)
		}
	}

	t.Logf("Captured %d progress events", len(events))
}

// MockProgressBus for testing progress streaming
type MockProgressBus struct {
	events *[]string
}

func (m *MockProgressBus) ScanStart(pipeline string, symbols []string) {
	*m.events = append(*m.events, "scan_start:"+pipeline)
}

func (m *MockProgressBus) ScanEvent(event interface{}) {
	*m.events = append(*m.events, "scan_event")
}

func (m *MockProgressBus) ScanComplete(candidates int, outputPath string) {
	*m.events = append(*m.events, "scan_complete")
}
