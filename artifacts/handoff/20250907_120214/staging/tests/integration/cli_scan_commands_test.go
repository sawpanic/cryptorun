package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestScanMomentumCLI tests the CLI momentum scan command
func TestScanMomentumCLI(t *testing.T) {
	// Build the CLI binary for testing
	binary := buildCLIBinary(t)
	defer os.Remove(binary)

	tests := []struct {
		name         string
		args         []string
		expectOutput []string
		expectFiles  []string
		expectError  bool
	}{
		{
			name: "momentum scan with minimal args",
			args: []string{"scan", "momentum", "--max-sample", "2"},
			expectOutput: []string{
				"Starting momentum scanning pipeline",
				"Momentum scan completed",
				"Results written to: out/scan/momentum_explain.json",
			},
			expectFiles: []string{
				"out/scan/momentum_explain.json",
				"out/audit/progress_trace.jsonl",
			},
		},
		{
			name: "momentum scan with plain progress",
			args: []string{"scan", "momentum", "--max-sample", "1", "--progress", "plain"},
			expectOutput: []string{
				"🔍 Starting momentum scan",
				"📋 Initializing pipeline",
				"🧮 Analyzing momentum signals",
				"✅ Scan completed",
			},
			expectFiles: []string{
				"out/scan/momentum_explain.json",
				"out/audit/progress_trace.jsonl",
			},
		},
		{
			name: "momentum scan with json progress",
			args: []string{"scan", "momentum", "--max-sample", "1", "--progress", "json"},
			expectOutput: []string{
				`"event":"scan_start"`,
				`"phase":"init"`,
				`"phase":"fetch"`,
				`"phase":"analyze"`,
				`"event":"scan_complete"`,
			},
			expectFiles: []string{
				"out/scan/momentum_explain.json",
				"out/audit/progress_trace.jsonl",
			},
		},
		{
			name: "momentum scan with custom venues and regime",
			args: []string{"scan", "momentum", "--max-sample", "1", "--venues", "kraken", "--regime", "choppy"},
			expectOutput: []string{
				"Starting momentum scanning pipeline",
				"Momentum scan completed",
			},
			expectFiles: []string{
				"out/scan/momentum_explain.json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up previous test outputs
			cleanupTestOutputs(t)

			// Run the CLI command
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, binary, tt.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected command to fail, but it succeeded")
				}
				return
			}

			if err != nil {
				t.Errorf("Command failed: %v\nStderr: %s\nStdout: %s", err, stderr.String(), stdout.String())
				return
			}

			output := stdout.String() + stderr.String()

			// Check expected output strings
			for _, expected := range tt.expectOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but got:\n%s", expected, output)
				}
			}

			// Check expected files were created
			for _, expectedFile := range tt.expectFiles {
				if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
					t.Errorf("Expected file %s to be created, but it doesn't exist", expectedFile)
				}
			}

			// Validate JSON output format for JSON progress mode
			if contains(tt.args, "--progress") && contains(tt.args, "json") {
				validateJSONProgressOutput(t, output)
			}

			// Validate explainability file structure
			if fileExists("out/scan/momentum_explain.json") {
				validateMomentumExplainFile(t, "out/scan/momentum_explain.json")
			}

			// Validate progress trace file
			if fileExists("out/audit/progress_trace.jsonl") {
				validateProgressTraceFile(t, "out/audit/progress_trace.jsonl")
			}
		})
	}
}

// TestScanDipCLI tests the CLI dip scan command
func TestScanDipCLI(t *testing.T) {
	binary := buildCLIBinary(t)
	defer os.Remove(binary)

	tests := []struct {
		name         string
		args         []string
		expectOutput []string
		expectError  bool
	}{
		{
			name: "dip scan with minimal args",
			args: []string{"scan", "dip", "--max-sample", "2"},
			expectOutput: []string{
				"Starting quality-dip scanning pipeline",
				"Dip scan completed",
				"implementation pending - use momentum for now",
			},
		},
		{
			name: "dip scan with custom settings",
			args: []string{"scan", "dip", "--max-sample", "1", "--top-n", "5"},
			expectOutput: []string{
				"Starting quality-dip scanning pipeline",
				"Dip scan completed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up previous test outputs
			cleanupTestOutputs(t)

			// Run the CLI command
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, binary, tt.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected command to fail, but it succeeded")
				}
				return
			}

			if err != nil {
				t.Errorf("Command failed: %v\nStderr: %s\nStdout: %s", err, stderr.String(), stdout.String())
				return
			}

			output := stdout.String() + stderr.String()

			// Check expected output strings
			for _, expected := range tt.expectOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but got:\n%s", expected, output)
				}
			}
		})
	}
}

// TestScanCommandHelp tests the help output for scan commands
func TestScanCommandHelp(t *testing.T) {
	binary := buildCLIBinary(t)
	defer os.Remove(binary)

	tests := []struct {
		name   string
		args   []string
		expect []string
	}{
		{
			name: "scan help",
			args: []string{"scan", "--help"},
			expect: []string{
				"Run scanning pipelines",
				"momentum",
				"dip",
			},
		},
		{
			name: "momentum help",
			args: []string{"scan", "momentum", "--help"},
			expect: []string{
				"Multi-timeframe momentum scanning",
				"--venues",
				"--max-sample",
				"--progress",
				"--regime",
			},
		},
		{
			name: "dip help",
			args: []string{"scan", "dip", "--help"},
			expect: []string{
				"Quality-dip scanner",
				"--venues",
				"--max-sample",
				"--progress",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binary, tt.args...)
			output, err := cmd.CombinedOutput()

			if err != nil {
				t.Errorf("Help command failed: %v\nOutput: %s", err, output)
				return
			}

			outputStr := string(output)
			for _, expected := range tt.expect {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected help output to contain %q, but got:\n%s", expected, outputStr)
				}
			}
		})
	}
}

// TestProgressModeIntegration tests different progress modes
func TestProgressModeIntegration(t *testing.T) {
	binary := buildCLIBinary(t)
	defer os.Remove(binary)

	progressModes := []struct {
		mode     string
		validate func(t *testing.T, output string)
	}{
		{
			mode: "auto",
			validate: func(t *testing.T, output string) {
				// Auto mode should adapt based on terminal detection
				// In test environment, it should default to JSON
				if !strings.Contains(output, "Starting momentum scanning pipeline") {
					t.Errorf("Auto mode should show structured output in test environment")
				}
			},
		},
		{
			mode: "plain",
			validate: func(t *testing.T, output string) {
				expectedMarkers := []string{"🔍", "📋", "🧮", "✅"}
				for _, marker := range expectedMarkers {
					if !strings.Contains(output, marker) {
						t.Errorf("Plain mode output missing expected marker: %s", marker)
					}
				}
			},
		},
		{
			mode:     "json",
			validate: validateJSONProgressOutput,
		},
	}

	for _, pm := range progressModes {
		t.Run("progress_mode_"+pm.mode, func(t *testing.T) {
			cleanupTestOutputs(t)

			args := []string{"scan", "momentum", "--max-sample", "1", "--progress", pm.mode}
			cmd := exec.Command(binary, args...)
			output, err := cmd.CombinedOutput()

			if err != nil {
				t.Errorf("Command failed: %v\nOutput: %s", err, output)
				return
			}

			pm.validate(t, string(output))

			// Verify progress trace file is always written
			if !fileExists("out/audit/progress_trace.jsonl") {
				t.Errorf("Progress trace file should be created regardless of progress mode")
			}
		})
	}
}

// Helper functions

func buildCLIBinary(t *testing.T) string {
	binaryName := filepath.Join(os.TempDir(), "cryptorun_test_"+strings.ReplaceAll(t.Name(), "/", "_"))

	cmd := exec.Command("go", "build", "-o", binaryName, "./cmd/cryptorun")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build CLI binary: %v", err)
	}

	return binaryName
}

func cleanupTestOutputs(t *testing.T) {
	outputs := []string{
		"out/scan/momentum_explain.json",
		"out/scan/dip_explain.json",
		"out/audit/progress_trace.jsonl",
	}

	for _, output := range outputs {
		os.Remove(output)
	}

	// Clean up output directories if they exist
	os.RemoveAll("out/scan")
	os.RemoveAll("out/audit")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func validateJSONProgressOutput(t *testing.T, output string) {
	lines := strings.Split(output, "\n")
	jsonLines := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") {
			jsonLines = append(jsonLines, line)
		}
	}

	if len(jsonLines) == 0 {
		t.Errorf("No JSON lines found in output")
		return
	}

	// Validate first line is scan_start
	var firstEvent map[string]interface{}
	if err := json.Unmarshal([]byte(jsonLines[0]), &firstEvent); err != nil {
		t.Errorf("Failed to parse first JSON line: %v", err)
		return
	}

	if event, ok := firstEvent["event"]; !ok || event != "scan_start" {
		t.Errorf("Expected first event to be 'scan_start', got: %v", event)
	}

	// Validate last line is scan_complete
	var lastEvent map[string]interface{}
	lastLine := jsonLines[len(jsonLines)-1]
	if err := json.Unmarshal([]byte(lastLine), &lastEvent); err != nil {
		t.Errorf("Failed to parse last JSON line: %v", err)
		return
	}

	if event, ok := lastEvent["event"]; !ok || event != "scan_complete" {
		t.Errorf("Expected last event to be 'scan_complete', got: %v", event)
	}
}

func validateMomentumExplainFile(t *testing.T, filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read explain file: %v", err)
		return
	}

	var explain map[string]interface{}
	if err := json.Unmarshal(data, &explain); err != nil {
		t.Errorf("Failed to parse explain JSON: %v", err)
		return
	}

	// Validate required sections
	requiredSections := []string{"scan_metadata", "configuration", "candidates", "summary"}
	for _, section := range requiredSections {
		if _, ok := explain[section]; !ok {
			t.Errorf("Explain file missing required section: %s", section)
		}
	}

	// Validate metadata
	if metadata, ok := explain["scan_metadata"].(map[string]interface{}); ok {
		if _, ok := metadata["methodology"]; !ok {
			t.Errorf("Scan metadata missing methodology field")
		}
		if _, ok := metadata["timestamp"]; !ok {
			t.Errorf("Scan metadata missing timestamp field")
		}
	}
}

func validateProgressTraceFile(t *testing.T, filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read progress trace file: %v", err)
		return
	}

	lines := strings.Split(string(data), "\n")
	validLines := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Errorf("Invalid JSON in progress trace: %v", err)
			return
		}

		// Validate required fields
		requiredFields := []string{"timestamp", "phase", "status"}
		for _, field := range requiredFields {
			if _, ok := event[field]; !ok {
				t.Errorf("Progress trace event missing required field: %s", field)
			}
		}

		validLines++
	}

	if validLines == 0 {
		t.Errorf("Progress trace file contains no valid events")
	}
}
