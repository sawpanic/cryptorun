package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// NoTodoScanner implements the No-TODO QA gate
type NoTodoScanner struct {
	patterns     *regexp.Regexp
	allowPatterns []string
	excludePatterns []string
}

// ScanResult holds the results of scanning a file
type ScanResult struct {
	File    string
	Line    int
	Content string
	Pattern string
}

// NewScanner creates a new No-TODO scanner
func NewScanner() *NoTodoScanner {
	// Case-insensitive patterns for TODO-like markers
	patterns := regexp.MustCompile(`(?i)\b(TODO|FIXME|XXX|STUB|PENDING)\b`)
	
	defaultExcludes := []string{
		"vendor/", "third_party/", ".git/", "node_modules/",
		"*.pb.go", "*.gen.go", "generated/", "artifacts/", "out/",
		"scripts/qa/scanner.go", // Don't scan this file
	}
	
	return &NoTodoScanner{
		patterns:        patterns,
		excludePatterns: defaultExcludes,
	}
}

// LoadAllowList loads exclusion patterns from the allow file
func (s *NoTodoScanner) LoadAllowList(allowFile string) error {
	file, err := os.Open(allowFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Allow file is optional
		}
		return fmt.Errorf("failed to open allow file: %w", err)
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		s.allowPatterns = append(s.allowPatterns, line)
	}
	
	return scanner.Err()
}

// shouldExclude checks if a file should be excluded from scanning
func (s *NoTodoScanner) shouldExclude(filePath string) bool {
	// Check built-in exclusions
	for _, pattern := range s.excludePatterns {
		if strings.Contains(filePath, strings.TrimSuffix(pattern, "/")) {
			return true
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(filePath)); matched {
			return true
		}
	}
	
	// Check allow list
	for _, pattern := range s.allowPatterns {
		if strings.Contains(filePath, pattern) {
			return true
		}
	}
	
	return false
}

// isTextFile checks if a file is likely a text file
func (s *NoTodoScanner) isTextFile(filePath string) bool {
	textExtensions := map[string]bool{
		".go": true, ".js": true, ".ts": true, ".py": true, ".java": true,
		".c": true, ".cpp": true, ".h": true, ".hpp": true, ".rs": true,
		".md": true, ".txt": true, ".yml": true, ".yaml": true, ".json": true,
		".xml": true, ".html": true, ".css": true, ".sql": true, ".sh": true,
	}
	
	ext := strings.ToLower(filepath.Ext(filePath))
	return textExtensions[ext]
}

// ScanFile scans a single file for TODO-like patterns
func (s *NoTodoScanner) ScanFile(filePath string) ([]ScanResult, error) {
	if s.shouldExclude(filePath) {
		return nil, nil
	}
	
	if !s.isTextFile(filePath) {
		return nil, nil
	}
	
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()
	
	var results []ScanResult
	scanner := bufio.NewScanner(file)
	lineNum := 1
	
	for scanner.Scan() {
		line := scanner.Text()
		matches := s.patterns.FindAllString(line, -1)
		
		for _, match := range matches {
			results = append(results, ScanResult{
				File:    filePath,
				Line:    lineNum,
				Content: strings.TrimSpace(line),
				Pattern: match,
			})
		}
		lineNum++
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan file %s: %w", filePath, err)
	}
	
	return results, nil
}

// ScanDirectory recursively scans a directory
func (s *NoTodoScanner) ScanDirectory(rootDir string) ([]ScanResult, error) {
	var allResults []ScanResult
	
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if info.IsDir() {
			return nil
		}
		
		results, err := s.ScanFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			return nil // Continue scanning other files
		}
		
		allResults = append(allResults, results...)
		return nil
	})
	
	return allResults, err
}

func main() {
	scanner := NewScanner()
	
	// Load allow list
	allowFile := filepath.Join("scripts", "qa", "no_todo.allow")
	if err := scanner.LoadAllowList(allowFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading allow list: %v\n", err)
		os.Exit(1)
	}
	
	// Scan current directory
	fmt.Println("🔍 Running No-TODO QA Gate Scanner (Go version)...")
	results, err := scanner.ScanDirectory(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
		os.Exit(1)
	}
	
	// Generate report
	reportFile := "no_todo_report.txt"
	file, err := os.Create(reportFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating report file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()
	
	fmt.Fprintf(file, "No-TODO QA Gate Report (Go Scanner) - %s\n", "2025-09-07")
	fmt.Fprintf(file, "===============================================\n\n")
	
	if len(results) == 0 {
		fmt.Printf("✅ QA Gate PASSED: No TODO/FIXME/STUB markers found\n")
		fmt.Fprintf(file, "✅ All clear: No TODO/FIXME/STUB markers found\n")
		os.Exit(0)
	}
	
	// Group results by file
	fileResults := make(map[string][]ScanResult)
	for _, result := range results {
		fileResults[result.File] = append(fileResults[result.File], result)
	}
	
	for filePath, fileRes := range fileResults {
		fmt.Fprintf(file, "❌ %s:\n", filePath)
		for _, res := range fileRes {
			fmt.Fprintf(file, "  Line %d: %s\n", res.Line, res.Content)
		}
		fmt.Fprintf(file, "\n")
	}
	
	fmt.Fprintf(file, "Summary:\n")
	fmt.Fprintf(file, "- Files with issues: %d\n", len(fileResults))
	fmt.Fprintf(file, "- Total markers found: %d\n", len(results))
	
	// Output to console
	fmt.Printf("❌ QA Gate FAILED: Found TODO/FIXME/STUB markers in %d file(s)\n", len(fileResults))
	fmt.Printf("📋 Full report: %s\n", reportFile)
	
	os.Exit(1)
}