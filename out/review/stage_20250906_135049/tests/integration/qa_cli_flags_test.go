package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestQACommandFlags tests the QA command with various flags
func TestQACommandFlags(t *testing.T) {
	binary := buildCLIBinary(t)
	defer os.Remove(binary)

	tests := []struct {
		name         string
		args         []string
		expectOutput []string
		expectFiles  []string
		expectError  bool
		timeout      time.Duration
	}{
		{
			name: "qa help command",
			args: []string{"qa", "--help"},
			expectOutput: []string{
				"Run first-class QA suite with provider guards",
				"--verify",
				"--fail-on-stubs",
				"--progress",
				"--resume",
				"--ttl",
			},
			timeout: 5 * time.Second,
		},
		{
			name: "qa with progress json",
			args: []string{"qa", "--progress", "json", "--max-sample", "1"},
			expectOutput: []string{
				`"event":"qa_start"`,
				`"phase":0`,
				`"status":"pass"`,
				`"event":"qa_complete"`,
			},
			expectFiles: []string{
				"out/audit/progress_trace.jsonl",
				"out/qa",
			},
			timeout: 30 * time.Second,
		},
		{
			name: "qa with progress plain",
			args: []string{"qa", "--progress", "plain", "--max-sample", "1"},
			expectOutput: []string{
				"🚀 Starting QA suite",
				"[0]",
				"✅ QA PASSED",
				"📁 Artifacts:",
			},
			expectFiles: []string{
				"out/qa",
			},
			timeout: 30 * time.Second,
		},
		{
			name: "qa with fail-on-stubs enabled",
			args: []string{"qa", "--fail-on-stubs", "--max-sample", "1"},
			expectOutput: []string{
				"Starting QA suite",
				"Phase 0:",
			},
			expectFiles: []string{
				"out/qa",
			},
			timeout: 30 * time.Second,
		},
		{
			name: "qa with verify enabled",
			args: []string{"qa", "--verify", "--max-sample", "1"},
			expectOutput: []string{
				"Starting QA suite",
				"Phase 7:",
				"Acceptance Verification",
			},
			expectFiles: []string{
				"out/qa",
			},
			timeout: 45 * time.Second,
		},
		{
			name: "qa with custom venues",
			args: []string{"qa", "--venues", "kraken", "--max-sample", "1"},
			expectOutput: []string{
				"Starting QA suite",
			},
			expectFiles: []string{
				"out/qa",
			},
			timeout: 30 * time.Second,
		},
		{
			name: "qa with resume flag",
			args: []string{"qa", "--resume", "--max-sample", "1"},
			expectOutput: []string{
				"Starting QA suite",
			},
			expectFiles: []string{
				"out/qa",
			},
			timeout: 30 * time.Second,
		},
		{
			name: "qa with custom ttl",
			args: []string{"qa", "--ttl", "60", "--max-sample", "1"},
			expectOutput: []string{
				"Starting QA suite",
			},
			timeout: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up previous outputs
			cleanupQAOutputs(t)

			// Set timeout
			timeout := tt.timeout
			if timeout == 0 {
				timeout = 30 * time.Second
			}

			// Run command with timeout
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
				// Check if it's a context timeout
				if ctx.Err() == context.DeadlineExceeded {
					t.Errorf("Command timed out after %v", timeout)
					return
				}
				t.Errorf("Command failed: %v\nStderr: %s\nStdout: %s", err, stderr.String(), stdout.String())
				return
			}

			output := stdout.String() + stderr.String()

			// Check expected output
			for _, expected := range tt.expectOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but got:\n%s", expected, output)
				}
			}

			// Check expected files
			for _, expectedFile := range tt.expectFiles {
				if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
					t.Errorf("Expected file/directory %s to exist, but it doesn't", expectedFile)
				}
			}

			// Validate JSON output if progress is json
			if contains(tt.args, "json") {
				validateQAJSONOutput(t, output)
			}
		})
	}
}

// TestQAFlagCombinations tests various flag combinations
func TestQAFlagCombinations(t *testing.T) {
	binary := buildCLIBinary(t)
	defer os.Remove(binary)

	tests := []struct {
		name         string
		args         []string
		expectOutput []string
		timeout      time.Duration
	}{
		{
			name: "qa with verify and fail-on-stubs",
			args: []string{"qa", "--verify", "--fail-on-stubs", "--progress", "plain", "--max-sample", "1"},
			expectOutput: []string{
				"Starting QA suite",
				"Phase 7:",
				"Acceptance Verification",
			},
			timeout: 45 * time.Second,
		},
		{
			name: "qa with json progress and custom settings",
			args: []string{"qa", "--progress", "json", "--venues", "kraken", "--ttl", "120", "--max-sample", "1"},
			expectOutput: []string{
				`"event":"qa_start"`,
				`"event":"qa_complete"`,
			},
			timeout: 30 * time.Second,
		},
		{
			name: "qa with all major flags",
			args: []string{"qa", "--verify", "--fail-on-stubs", "--progress", "plain", "--venues", "kraken,okx", "--max-sample", "2"},
			expectOutput: []string{
				"Starting QA suite",
				"Phase 7:",
			},
			timeout: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupQAOutputs(t)

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			cmd := exec.CommandContext(ctx, binary, tt.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					t.Errorf("Command timed out after %v", tt.timeout)
					return
				}
				t.Errorf("Command failed: %v\nOutput: %s", err, stdout.String()+stderr.String())
				return
			}

			output := stdout.String() + stderr.String()

			for _, expected := range tt.expectOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but got:\n%s", expected, output)
				}
			}
		})
	}
}

// TestQAProgressModes tests different progress output modes for QA
func TestQAProgressModes(t *testing.T) {
	binary := buildCLIBinary(t)
	defer os.Remove(binary)

	modes := []struct {
		name     string
		mode     string
		validate func(t *testing.T, output string)
	}{
		{
			name: "auto mode",
			mode: "auto",
			validate: func(t *testing.T, output string) {
				// Auto mode should adapt, in test it should be structured
				if !strings.Contains(output, "Starting QA suite") {
					t.Errorf("Auto mode should show QA output")
				}
			},
		},
		{
			name: "plain mode",
			mode: "plain",
			validate: func(t *testing.T, output string) {
				expectedMarkers := []string{"🚀", "[0]", "✅", "━━━"}
				for _, marker := range expectedMarkers {
					if !strings.Contains(output, marker) {
						t.Errorf("Plain mode missing expected marker: %s", marker)
					}
				}
			},
		},
		{
			name: "json mode",
			mode: "json",
			validate: validateQAJSONOutput,
		},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			cleanupQAOutputs(t)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			args := []string{"qa", "--progress", mode.mode, "--max-sample", "1"}
			cmd := exec.CommandContext(ctx, binary, args...)
			output, err := cmd.CombinedOutput()

			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					t.Errorf("Command timed out")
					return
				}
				t.Errorf("Command failed: %v\nOutput: %s", err, output)
				return
			}

			mode.validate(t, string(output))
		})
	}
}

// Helper functions

func cleanupQAOutputs(t *testing.T) {
	outputs := []string{
		"out/qa",
		"out/audit/progress_trace.jsonl",
	}
	
	for _, output := range outputs {
		os.RemoveAll(output)
	}
}

func validateQAJSONOutput(t *testing.T, output string) {
	lines := strings.Split(output, "\n")
	jsonLines := []string{}
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") {
			jsonLines = append(jsonLines, line)
		}
	}
	
	if len(jsonLines) == 0 {
		t.Errorf("No JSON lines found in QA output")
		return
	}
	
	// Look for qa_start event
	foundStart := false
	foundComplete := false
	
	for _, jsonLine := range jsonLines {
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(jsonLine), &event); err != nil {
			continue // Skip malformed JSON
		}
		
		if event["event"] == "qa_start" {
			foundStart = true
			if _, ok := event["total_phases"]; !ok {
				t.Errorf("qa_start event missing total_phases field")
			}
		}
		
		if event["event"] == "qa_complete" {
			foundComplete = true
			if _, ok := event["success"]; !ok {
				t.Errorf("qa_complete event missing success field")
			}
		}
		
		if event["event"] == "qa_phase" {
			requiredFields := []string{"phase", "name", "status", "duration"}
			for _, field := range requiredFields {
				if _, ok := event[field]; !ok {
					t.Errorf("qa_phase event missing required field: %s", field)
				}
			}
		}
	}
	
	if !foundStart {
		t.Errorf("Missing qa_start event in JSON output")
	}
	if !foundComplete {
		t.Errorf("Missing qa_complete event in JSON output")
	}
}