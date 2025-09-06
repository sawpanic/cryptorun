package branding

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// BrandGuardConfig defines configuration for brand consistency checking
type BrandGuardConfig struct {
	AllowedBrands   []string
	ForbiddenBrands []string
	ExcludedPaths   []string
	CodereviewPath  string
}

// DefaultBrandGuardConfig returns the standard configuration for CryptoRun
func DefaultBrandGuardConfig() BrandGuardConfig {
	return BrandGuardConfig{
		AllowedBrands:   []string{"CryptoRun"},
		ForbiddenBrands: []string{"CryptoEdge", "Crypto Edge"},
		ExcludedPaths:   []string{".git", "vendor", "out", "node_modules"},
		CodereviewPath:  "_codereview",
	}
}

// BrandViolation represents a branding consistency violation
type BrandViolation struct {
	FilePath      string
	LineNumber    int
	Line          string
	ViolatingTerm string
}

// BrandGuard validates brand consistency across documentation
type BrandGuard struct {
	config     BrandGuardConfig
	violations []BrandViolation
}

// NewBrandGuard creates a new brand guard with default configuration
func NewBrandGuard() *BrandGuard {
	return &BrandGuard{
		config:     DefaultBrandGuardConfig(),
		violations: []BrandViolation{},
	}
}

// ScanDirectory recursively scans a directory for brand violations
func (bg *BrandGuard) ScanDirectory(rootPath string) error {
	return filepath.Walk(rootPath, bg.walkFunc)
}

// walkFunc processes each file during directory traversal
func (bg *BrandGuard) walkFunc(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	// Skip directories and non-text files
	if info.IsDir() {
		return nil
	}

	// Only check markdown and text files
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".txt" && ext != ".rst" {
		return nil
	}

	// Check if path should be excluded
	if bg.shouldExcludePath(path) {
		return nil
	}

	return bg.scanFile(path)
}

// shouldExcludePath determines if a path should be skipped
func (bg *BrandGuard) shouldExcludePath(path string) bool {
	cleanPath := filepath.Clean(path)
	pathParts := strings.Split(cleanPath, string(filepath.Separator))

	for _, part := range pathParts {
		for _, excluded := range bg.config.ExcludedPaths {
			if part == excluded {
				return true
			}
		}
	}

	return false
}

// scanFile scans a single file for brand violations
func (bg *BrandGuard) scanFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check if file is in codereview path (allows historic mentions)
	isCodereviewPath := strings.Contains(filePath, bg.config.CodereviewPath)

	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		// Skip codereview paths for historic brand mentions
		if isCodereviewPath {
			continue
		}

		// Check for forbidden brand mentions
		for _, forbidden := range bg.config.ForbiddenBrands {
			if bg.containsBrandMention(line, forbidden) {
				violation := BrandViolation{
					FilePath:      filePath,
					LineNumber:    lineNumber,
					Line:          strings.TrimSpace(line),
					ViolatingTerm: forbidden,
				}
				bg.violations = append(bg.violations, violation)
			}
		}
	}

	return scanner.Err()
}

// containsBrandMention checks if a line contains a specific brand mention
func (bg *BrandGuard) containsBrandMention(line, brand string) bool {
	// Handle "Crypto Edge" (with space)
	if strings.Contains(brand, " ") {
		pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(brand) + `\b`)
		return pattern.MatchString(line)
	}

	// Handle "CryptoEdge" (camelCase)
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(brand) + `\b`)
	return pattern.MatchString(line)
}

// GetViolations returns all found violations
func (bg *BrandGuard) GetViolations() []BrandViolation {
	return bg.violations
}

// HasViolations returns true if any violations were found
func (bg *BrandGuard) HasViolations() bool {
	return len(bg.violations) > 0
}

// Reset clears all accumulated violations
func (bg *BrandGuard) Reset() {
	bg.violations = []BrandViolation{}
}

// TestBrandConsistency validates brand consistency across the repository
func TestBrandConsistency(t *testing.T) {
	guard := NewBrandGuard()

	// Scan from project root
	err := guard.ScanDirectory("../..")
	if err != nil {
		t.Fatalf("Failed to scan directory: %v", err)
	}

	violations := guard.GetViolations()
	if len(violations) > 0 {
		t.Errorf("Found %d brand consistency violations:", len(violations))
		for _, violation := range violations {
			t.Errorf("  %s:%d - Found '%s' in: %s",
				violation.FilePath,
				violation.LineNumber,
				violation.ViolatingTerm,
				violation.Line)
		}

		t.Error("\nBrand consistency rules:")
		t.Error("  - Only 'CryptoRun' is permitted in active documentation")
		t.Error("  - 'CryptoEdge' and 'Crypto Edge' are forbidden outside _codereview/**")
		t.Error("  - Historic mentions in _codereview/** are allowed but should not be linked by active docs")
	}
}

// TestBrandGuardFunctionality tests the brand guard implementation
func TestBrandGuardFunctionality(t *testing.T) {
	guard := NewBrandGuard()

	// Test cases for brand detection
	testCases := []struct {
		name            string
		line            string
		expectViolation bool
		violatingTerm   string
	}{
		{
			name:            "CryptoRun should be allowed",
			line:            "This is about CryptoRun system",
			expectViolation: false,
		},
		{
			name:            "CryptoEdge should be detected",
			line:            "The old CryptoEdge system was replaced",
			expectViolation: true,
			violatingTerm:   "CryptoEdge",
		},
		{
			name:            "Crypto Edge with space should be detected",
			line:            "Crypto Edge was the previous name",
			expectViolation: true,
			violatingTerm:   "Crypto Edge",
		},
		{
			name:            "Case insensitive detection for spaced version",
			line:            "crypto edge should not appear",
			expectViolation: true,
			violatingTerm:   "Crypto Edge",
		},
		{
			name:            "Partial matches should not trigger",
			line:            "CryptoCurrency edge cases",
			expectViolation: false,
		},
		{
			name:            "Word boundaries should be respected",
			line:            "MyCryptoEdgeTool is different",
			expectViolation: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			guard.Reset()

			// Simulate scanning a line
			foundViolation := false
			for _, forbidden := range guard.config.ForbiddenBrands {
				if guard.containsBrandMention(tc.line, forbidden) {
					foundViolation = true
					if tc.expectViolation && forbidden != tc.violatingTerm {
						continue // Check if we found the expected violating term
					}
					break
				}
			}

			if tc.expectViolation && !foundViolation {
				t.Errorf("Expected violation for line: %s", tc.line)
			}

			if !tc.expectViolation && foundViolation {
				t.Errorf("Unexpected violation for line: %s", tc.line)
			}
		})
	}
}

// TestCodereviewPathExclusion tests that _codereview paths are properly excluded
func TestCodereviewPathExclusion(t *testing.T) {
	guard := NewBrandGuard()

	testPaths := []struct {
		path          string
		shouldExclude bool
	}{
		{
			path:          "_codereview/historic/old_docs.md",
			shouldExclude: false, // Checked in scanFile, not shouldExcludePath
		},
		{
			path:          "docs/active/readme.md",
			shouldExclude: false,
		},
		{
			path:          ".git/config",
			shouldExclude: true,
		},
		{
			path:          "vendor/package/readme.md",
			shouldExclude: true,
		},
		{
			path:          "out/results/summary.md",
			shouldExclude: true,
		},
	}

	for _, tc := range testPaths {
		t.Run(tc.path, func(t *testing.T) {
			result := guard.shouldExcludePath(tc.path)
			if result != tc.shouldExclude {
				t.Errorf("Path %s: expected exclude=%t, got exclude=%t",
					tc.path, tc.shouldExclude, result)
			}
		})
	}
}

// BenchmarkBrandGuardScan benchmarks the brand guard scanning performance
func BenchmarkBrandGuardScan(b *testing.B) {
	guard := NewBrandGuard()

	// Create test content
	testLine := "This document describes CryptoRun functionality and mentions some Crypto Edge legacy issues."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, forbidden := range guard.config.ForbiddenBrands {
			guard.containsBrandMention(testLine, forbidden)
		}
	}
}
