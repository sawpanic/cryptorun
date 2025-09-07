package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	requiredUXHeading = "## UX MUST — Live Progress & Explainability"
	exitSuccess       = 0
	exitFailure       = 1
)

// ExcludedPaths defines directories to skip during markdown scanning
var ExcludedPaths = []string{
	".git",
	"vendor",
	"_codereview",
	"out",
}

// DocUXChecker validates UX documentation requirements
type DocUXChecker struct {
	missingUXFiles  []string
	brandViolations []BrandViolation
	totalFiles      int
}

// BrandViolation represents a branding consistency violation
type BrandViolation struct {
	FilePath  string
	LineNum   int
	Content   string
	Violation string
}

// NewDocUXChecker creates a new documentation checker
func NewDocUXChecker() *DocUXChecker {
	return &DocUXChecker{
		missingUXFiles:  []string{},
		brandViolations: []BrandViolation{},
		totalFiles:      0,
	}
}

// CheckRepository validates all markdown files in the repository
func (checker *DocUXChecker) CheckRepository() error {
	return filepath.Walk(".", checker.walkFunc)
}

// walkFunc processes each file encountered during directory traversal
func (checker *DocUXChecker) walkFunc(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	// Skip directories and non-markdown files
	if info.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") {
		return nil
	}

	// Check if path should be excluded
	if checker.shouldExcludePath(path) {
		return nil
	}

	checker.totalFiles++

	// Check UX MUST block requirement
	if err := checker.checkUXMustBlock(path); err != nil {
		return fmt.Errorf("failed to check UX MUST block in %s: %w", path, err)
	}

	// Check branding consistency
	if err := checker.checkBrandConsistency(path); err != nil {
		return fmt.Errorf("failed to check branding in %s: %w", path, err)
	}

	return nil
}

// shouldExcludePath determines if a path should be skipped
func (checker *DocUXChecker) shouldExcludePath(path string) bool {
	cleanPath := filepath.Clean(path)
	pathParts := strings.Split(cleanPath, string(filepath.Separator))

	for _, part := range pathParts {
		for _, excluded := range ExcludedPaths {
			if part == excluded {
				return true
			}
		}
	}

	return false
}

// checkUXMustBlock verifies the required UX MUST heading exists
func (checker *DocUXChecker) checkUXMustBlock(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	hasUXMustBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == requiredUXHeading {
			hasUXMustBlock = true
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if !hasUXMustBlock {
		checker.missingUXFiles = append(checker.missingUXFiles, filePath)
	}

	return nil
}

// checkBrandConsistency validates brand name usage
func (checker *DocUXChecker) checkBrandConsistency(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Allow historic mentions only inside _codereview/**
	isCodereviewPath := strings.Contains(filePath, "_codereview")

	scanner := bufio.NewScanner(file)
	lineNum := 0

	// Brand violation patterns
	cryptoEdgePattern := regexp.MustCompile(`(?i)\bcrypto\s*edge\b`)
	cryptoEdgeCamelPattern := regexp.MustCompile(`\bCryptoEdge\b`)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip _codereview paths for historic mentions
		if isCodereviewPath {
			continue
		}

		// Check for brand violations, but skip documentation about the violations themselves
		isDocumentationAboutBrandRules := strings.Contains(strings.ToLower(line), "forbidden") ||
			strings.Contains(strings.ToLower(line), "brand") ||
			strings.Contains(strings.ToLower(line), "consistency") ||
			strings.Contains(strings.ToLower(line), "except in") ||
			strings.Contains(strings.ToLower(line), "allowed only")

		if !isDocumentationAboutBrandRules && (cryptoEdgePattern.MatchString(line) || cryptoEdgeCamelPattern.MatchString(line)) {
			violation := BrandViolation{
				FilePath:  filePath,
				LineNum:   lineNum,
				Content:   strings.TrimSpace(line),
				Violation: "Found 'CryptoEdge' or 'Crypto Edge' outside _codereview/**",
			}
			checker.brandViolations = append(checker.brandViolations, violation)
		}
	}

	return scanner.Err()
}

// PrintResults outputs the validation results
func (checker *DocUXChecker) PrintResults() {
	fmt.Printf("📋 CryptoRun Documentation UX Guard\n")
	fmt.Printf("Scanned %d markdown files\n\n", checker.totalFiles)

	// Report UX MUST block violations
	if len(checker.missingUXFiles) > 0 {
		fmt.Printf("❌ UX MUST Block Violations (%d files):\n", len(checker.missingUXFiles))
		fmt.Printf("Missing required heading: %s\n\n", requiredUXHeading)

		for _, filePath := range checker.missingUXFiles {
			fmt.Printf("  - %s\n", filePath)
		}
		fmt.Printf("\n")
	} else {
		fmt.Printf("✅ UX MUST Block: All files compliant\n\n")
	}

	// Report brand violations
	if len(checker.brandViolations) > 0 {
		fmt.Printf("❌ Brand Consistency Violations (%d issues):\n", len(checker.brandViolations))
		fmt.Printf("Only 'CryptoRun' is permitted. 'CryptoEdge'/'Crypto Edge' allowed only in _codereview/**\n\n")

		for _, violation := range checker.brandViolations {
			fmt.Printf("  - %s:%d\n", violation.FilePath, violation.LineNum)
			fmt.Printf("    %s\n", violation.Violation)
			fmt.Printf("    Content: %s\n\n", violation.Content)
		}
	} else {
		fmt.Printf("✅ Brand Consistency: All mentions compliant\n\n")
	}
}

// HasViolations returns true if any violations were found
func (checker *DocUXChecker) HasViolations() bool {
	return len(checker.missingUXFiles) > 0 || len(checker.brandViolations) > 0
}

// PrintSummary outputs a concise summary for CI/automation
func (checker *DocUXChecker) PrintSummary() {
	if !checker.HasViolations() {
		fmt.Printf("✅ DOCS_UX_GUARD: PASS - %d files validated\n", checker.totalFiles)
		return
	}

	fmt.Printf("❌ DOCS_UX_GUARD: FAIL\n")
	if len(checker.missingUXFiles) > 0 {
		fmt.Printf("   UX_MUST_MISSING: %d files\n", len(checker.missingUXFiles))
	}
	if len(checker.brandViolations) > 0 {
		fmt.Printf("   BRAND_VIOLATIONS: %d issues\n", len(checker.brandViolations))
	}
}

func main() {
	checker := NewDocUXChecker()

	// Check repository
	if err := checker.CheckRepository(); err != nil {
		fmt.Fprintf(os.Stderr, "Error checking repository: %v\n", err)
		os.Exit(exitFailure)
	}

	// Print detailed results
	checker.PrintResults()

	// Print summary for automation
	checker.PrintSummary()

	// Exit with appropriate code
	if checker.HasViolations() {
		os.Exit(exitFailure)
	}

	os.Exit(exitSuccess)
}
