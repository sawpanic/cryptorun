package integration

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestCLIBasicCommands tests basic CLI functionality
func TestCLIBasicCommands(t *testing.T) {
	// Skip if we can't find the binary
	binary := findCLIBinary(t)
	if binary == "" {
		t.Skip("CLI binary not found - run 'go build -o cryptorun.exe ./cmd/cryptorun' first")
	}

	tests := []struct {
		name         string
		args         []string
		expectInHelp []string
		expectError  bool
	}{
		{
			name: "root help",
			args: []string{"--help"},
			expectInHelp: []string{
				"6-48 hour cryptocurrency momentum scanner",
				"scan",
				"pairs",
				"monitor",
				"qa",
			},
		},
		{
			name: "scan help",
			args: []string{"scan", "--help"},
			expectInHelp: []string{
				"Run momentum or dip scanning",
				"momentum",
				"dip",
			},
		},
		{
			name: "scan momentum help",
			args: []string{"scan", "momentum", "--help"},
			expectInHelp: []string{
				"Multi-timeframe momentum scanning",
				"--venues",
				"--max-sample",
				"--progress",
				"--regime",
				"--top-n",
			},
		},
		{
			name: "scan dip help",
			args: []string{"scan", "dip", "--help"},
			expectInHelp: []string{
				"Quality-dip scanner",
				"--venues",
				"--max-sample",
				"--progress",
			},
		},
		{
			name: "qa help",
			args: []string{"qa", "--help"},
			expectInHelp: []string{
				"Comprehensive QA runner",
				"--verify",
				"--fail-on-stubs",
				"--progress",
				"--venues",
				"--max-sample",
			},
		},
		{
			name: "pairs help",
			args: []string{"pairs", "--help"},
			expectInHelp: []string{
				"Commands for discovering, syncing, and managing trading pairs",
				"sync",
			},
		},
		{
			name: "monitor help",
			args: []string{"monitor", "--help"},
			expectInHelp: []string{
				"Starts HTTP server",
				"--port",
				"--host",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binary, tt.args...)
			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but command succeeded")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Command failed unexpectedly: %v\nOutput: %s", err, outputStr)
				return
			}

			// Check that expected help content is present
			for _, expected := range tt.expectInHelp {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected help output to contain %q, but it was missing from:\n%s", expected, outputStr)
				}
			}
		})
	}
}

// TestCLIFlagValidation tests flag validation and error handling
func TestCLIFlagValidation(t *testing.T) {
	binary := findCLIBinary(t)
	if binary == "" {
		t.Skip("CLI binary not found")
	}

	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorText   []string
	}{
		{
			name:        "invalid scan subcommand",
			args:        []string{"scan", "invalid"},
			expectError: false, // Shows help instead of error
			errorText:   []string{},
		},
		{
			name:        "invalid flag",
			args:        []string{"scan", "momentum", "--invalid-flag"},
			expectError: true,
			errorText:   []string{"unknown flag"},
		},
		{
			name:        "pairs sync with unsupported venue",
			args:        []string{"pairs", "sync", "--venue", "binance"},
			expectError: true,
			errorText:   []string{"unsupported venue"},
		},
		{
			name:        "pairs sync with unsupported quote",
			args:        []string{"pairs", "sync", "--quote", "EUR"},
			expectError: true,
			errorText:   []string{"unsupported quote currency"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binary, tt.args...)
			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected command to fail, but it succeeded")
					return
				}

				// Check that error message contains expected text
				for _, expectedError := range tt.errorText {
					if !strings.Contains(outputStr, expectedError) {
						t.Errorf("Expected error output to contain %q, but got:\n%s", expectedError, outputStr)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Command failed unexpectedly: %v\nOutput: %s", err, outputStr)
				}
			}
		})
	}
}

// TestCLIVersion tests version information
func TestCLIVersion(t *testing.T) {
	binary := findCLIBinary(t)
	if binary == "" {
		t.Skip("CLI binary not found")
	}

	cmd := exec.Command(binary, "--version")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("Version command failed: %v\nOutput: %s", err, output)
		return
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "v3.2.1") {
		t.Errorf("Expected version output to contain 'v3.2.1', but got:\n%s", outputStr)
	}
}

// Helper function to find CLI binary
func findCLIBinary(t *testing.T) string {
	// List of possible binary names to check (prefer newer builds)
	candidates := []string{
		"../../cryptorun_fresh.exe",
		"../../cryptorun_test.exe",
		"cryptorun_fresh.exe",
		"cryptorun_test.exe",
		"./cryptorun_fresh.exe",
		"../../cryptorun.exe",
		"cryptorun.exe",
		"./cryptorun.exe",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}
