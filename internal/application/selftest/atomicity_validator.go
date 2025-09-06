package selftest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AtomicityValidator validates temp-then-rename pattern usage
type AtomicityValidator struct{}

// NewAtomicityValidator creates a new atomicity validator
func NewAtomicityValidator() *AtomicityValidator {
	return &AtomicityValidator{}
}

// Name returns the validator name
func (av *AtomicityValidator) Name() string {
	return "Atomicity Validation"
}

// Validate checks temp-then-rename pattern compliance
func (av *AtomicityValidator) Validate() TestResult {
	start := time.Now()
	result := TestResult{
		Name:      av.Name(),
		Timestamp: start,
		Details:   []string{},
	}

	// Test temp-then-rename pattern
	testDir := "out/selftest/atomicity_test"
	if err := os.MkdirAll(testDir, 0755); err != nil {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Failed to create test directory: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	// Clean up test directory at end
	defer func() {
		os.RemoveAll(testDir)
	}()

	// Check 1: Verify temp-then-rename pattern works
	targetPath := filepath.Join(testDir, "test_output.json")
	tempPath := targetPath + ".tmp"

	// Write to temp file
	testContent := `{"test": "atomicity", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`
	if err := os.WriteFile(tempPath, []byte(testContent), 0644); err != nil {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Failed to write temp file: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	result.Details = append(result.Details, "Successfully created temp file")

	// Verify temp file exists and target doesn't
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		result.Status = "FAIL"
		result.Message = "Temp file was not created"
		result.Duration = time.Since(start)
		return result
	}

	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		result.Status = "FAIL"
		result.Message = "Target file exists before rename"
		result.Duration = time.Since(start)
		return result
	}
	result.Details = append(result.Details, "Temp file exists, target file doesn't exist (correct)")

	// Rename temp to target (atomic operation)
	if err := os.Rename(tempPath, targetPath); err != nil {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Failed to rename temp to target: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	result.Details = append(result.Details, "Successfully renamed temp to target")

	// Verify target exists and temp doesn't
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		result.Status = "FAIL"
		result.Message = "Target file doesn't exist after rename"
		result.Duration = time.Since(start)
		return result
	}

	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		result.Status = "FAIL"
		result.Message = "Temp file still exists after rename"
		result.Duration = time.Since(start)
		return result
	}
	result.Details = append(result.Details, "Target file exists, temp file doesn't exist (correct)")

	// Verify content integrity
	readContent, err := os.ReadFile(targetPath)
	if err != nil {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Failed to read target file: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	if string(readContent) != testContent {
		result.Status = "FAIL"
		result.Message = "Content integrity check failed"
		result.Duration = time.Since(start)
		return result
	}
	result.Details = append(result.Details, "Content integrity verified")

	// Check 2: Scan codebase for direct writes to output files
	violations, err := av.scanForDirectWrites()
	if err != nil {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Failed to scan codebase: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	if len(violations) > 0 {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Found %d atomicity violations", len(violations))
		result.Details = append(result.Details, "Atomicity violations found:")
		for _, violation := range violations {
			result.Details = append(result.Details, fmt.Sprintf("  - %s", violation))
		}
	} else {
		result.Details = append(result.Details, "No atomicity violations found in codebase")
	}

	if result.Status == "" {
		result.Status = "PASS"
		result.Message = "Atomicity validation passed"
	}

	result.Duration = time.Since(start)
	return result
}

// scanForDirectWrites scans codebase for potential atomicity violations
func (av *AtomicityValidator) scanForDirectWrites() ([]string, error) {
	violations := []string{}

	// Walk through source directories
	sourceDirs := []string{"src", "internal"}

	for _, sourceDir := range sourceDirs {
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			continue // Skip if directory doesn't exist
		}

		err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Only check .go files
			if !strings.HasSuffix(path, ".go") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				// Look for suspicious patterns that might violate atomicity
				if av.containsAtomicityViolation(line) {
					violations = append(violations, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line)))
				}
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return violations, nil
}

// containsAtomicityViolation checks if a line contains potential atomicity violations
func (av *AtomicityValidator) containsAtomicityViolation(line string) bool {
	line = strings.TrimSpace(line)

	// Skip comments
	if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "*") {
		return false
	}

	// Look for direct writes to output files without temp pattern
	suspiciousPatterns := []string{
		"os.WriteFile(\"out/",
		"ioutil.WriteFile(\"out/",
		"os.Create(\"out/",
		"os.OpenFile(\"out/",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(line, pattern) {
			// Check if it's using temp pattern
			if !strings.Contains(line, ".tmp") && !strings.Contains(line, "temp") {
				return true
			}
		}
	}

	return false
}
