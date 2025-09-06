package qa

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cryptorun/internal/atomicio"
)

// Hit represents a stub detection hit
type Hit struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Excerpt string `json:"excerpt"`
	Pattern string `json:"pattern"`
}

// NostubScanResult contains the full scan results
type NostubScanResult struct {
	Timestamp    time.Time `json:"timestamp"`
	Root         string    `json:"root"`
	Hits         []Hit     `json:"hits"`
	Total        int       `json:"total"`
	FilesScanned int       `json:"files_scanned"`
}

var stubPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)panic\s*\(\s*"not implemented"`),
	regexp.MustCompile(`(?i)TODO`),
	regexp.MustCompile(`(?i)FIXME`),
	regexp.MustCompile(`(?i)STUB`),
	regexp.MustCompile(`(?i)NotImplemented`),
	regexp.MustCompile(`(?i)dummy implementation`),
	regexp.MustCompile(`(?i)return nil\s*//\s*TODO`),
}

var patternNames = []string{
	"panic_not_implemented",
	"TODO",
	"FIXME",
	"STUB",
	"NotImplemented",
	"dummy_implementation",
	"return_nil_todo",
}

// NostubScan scans a directory tree for stub patterns in Go source files
func NostubScan(root string, excludes []string) ([]Hit, error) {
	var hits []Hit
	var filesScanned int

	// Default excludes - always exclude test files, vendor, out, etc.
	defaultExcludes := []string{
		"*_test.go",
		"vendor/",
		"out/",
		"_codereview/",
		"testdata/",
		".git/",
		"node_modules/",
	}

	allExcludes := append(defaultExcludes, excludes...)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Check exclusions
		relPath, _ := filepath.Rel(root, path)
		for _, exclude := range allExcludes {
			if matched, _ := filepath.Match(exclude, filepath.Base(path)); matched {
				return nil
			}
			if strings.Contains(relPath, strings.TrimSuffix(exclude, "/")) {
				return nil
			}
		}

		filesScanned++
		fileHits, err := scanFile(path)
		if err != nil {
			return fmt.Errorf("failed to scan %s: %w", path, err)
		}

		hits = append(hits, fileHits...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk failed: %w", err)
	}

	// Write results to audit output
	result := NostubScanResult{
		Timestamp:    time.Now().UTC(),
		Root:         root,
		Hits:         hits,
		Total:        len(hits),
		FilesScanned: filesScanned,
	}

	if err := writeNostubResults(result); err != nil {
		return hits, fmt.Errorf("failed to write results: %w", err)
	}

	return hits, nil
}

// scanFile scans a single file for stub patterns
func scanFile(filepath string) ([]Hit, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var hits []Hit
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for i, pattern := range stubPatterns {
			if pattern.MatchString(line) {
				hits = append(hits, Hit{
					File:    filepath,
					Line:    lineNum,
					Excerpt: strings.TrimSpace(line),
					Pattern: patternNames[i],
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan error: %w", err)
	}

	return hits, nil
}

// writeNostubResults writes the scan results to out/audit/nostub_hits.json
func writeNostubResults(result NostubScanResult) error {
	// Ensure audit directory exists
	auditDir := "out/audit"
	if err := os.MkdirAll(auditDir, 0755); err != nil {
		return fmt.Errorf("failed to create audit directory: %w", err)
	}

	// Marshal results
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	// Write atomically
	outputPath := filepath.Join(auditDir, "nostub_hits.json")
	if err := atomicio.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write results: %w", err)
	}

	return nil
}

// ValidateNoStubs performs the no-stub gate check and returns an error if stubs are found
func ValidateNoStubs(root string, excludes []string) error {
	hits, err := NostubScan(root, excludes)
	if err != nil {
		return fmt.Errorf("nostub scan failed: %w", err)
	}

	if len(hits) > 0 {
		return fmt.Errorf("found %d stub patterns in code - acceptance gate failed", len(hits))
	}

	return nil
}

// NoStubGate provides the interface expected by tests
type NoStubGate struct {
	auditDir string
}

// ScanReport contains the scan results for testing
type ScanReport struct {
	TotalHits int
	Hits      []Hit
	Scanned   int
	Excluded  int
}

// NewNoStubGate creates a new no-stub gate scanner
func NewNoStubGate(auditDir string) *NoStubGate {
	return &NoStubGate{auditDir: auditDir}
}

// Scan performs the stub detection scan
func (g *NoStubGate) Scan() (*ScanReport, error) {
	hits, err := NostubScan(".", []string{})
	if err != nil {
		return nil, err
	}

	// Count scanned and excluded files by walking the directory
	scannedCount := 0
	excludedCount := 0
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			// Skip excluded files
			relPath, _ := filepath.Rel(".", path)
			excluded := false
			for _, exclude := range []string{"*_test.go", "vendor/", "out/", "_codereview/", "testdata/", ".git/", "node_modules/"} {
				if matched, _ := filepath.Match(exclude, filepath.Base(path)); matched {
					excluded = true
					break
				}
				if strings.Contains(relPath, strings.TrimSuffix(exclude, "/")) {
					excluded = true
					break
				}
			}
			if excluded {
				excludedCount++
			} else {
				scannedCount++
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &ScanReport{
		TotalHits: len(hits),
		Hits:      hits,
		Scanned:   scannedCount,
		Excluded:  excludedCount,
	}, nil
}

// RunGate runs the gate check and returns error if stubs found
func (g *NoStubGate) RunGate() error {
	return ValidateNoStubs(".", []string{})
}
