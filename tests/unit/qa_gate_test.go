package unit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQAGateScanner(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	
	// Test cases
	tests := []struct {
		name        string
		files       map[string]string
		shouldFail  bool
		description string
	}{
		{
			name: "clean_code",
			files: map[string]string{
				"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello World")
}`,
				"utils.go": `package main

func add(a, b int) int {
	return a + b
}`,
			},
			shouldFail:  false,
			description: "Code without any TODO markers should pass",
		},
		{
			name: "has_todo",
			files: map[string]string{
				"main.go": `package main

import "fmt"

func main() {
	// TODO: implement proper error handling
	fmt.Println("Hello World")
}`,
			},
			shouldFail:  true,
			description: "Code with TODO marker should fail",
		},
		{
			name: "has_fixme",
			files: map[string]string{
				"main.go": `package main

import "fmt"

func main() {
	// FIXME: this is broken
	fmt.Println("Hello World")
}`,
			},
			shouldFail:  true,
			description: "Code with FIXME marker should fail",
		},
		{
			name: "has_stub",
			files: map[string]string{
				"main.go": `package main

import "fmt"

func main() {
	// STUB: placeholder implementation
	fmt.Println("Hello World")
}`,
			},
			shouldFail:  true,
			description: "Code with STUB marker should fail",
		},
		{
			name: "case_insensitive",
			files: map[string]string{
				"main.go": `package main

import "fmt"

func main() {
	// todo: lowercase should also be caught
	fmt.Println("Hello World")
}`,
			},
			shouldFail:  true,
			description: "Lowercase TODO should also be caught",
		},
		{
			name: "excluded_file",
			files: map[string]string{
				"main.go": `package main

func main() {
	// This is fine
}`,
				"vendor/lib.go": `package vendor

// TODO: this should be ignored in vendor/
func lib() {}`,
			},
			shouldFail:  false,
			description: "TODO in vendor/ should be ignored",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			testDir := filepath.Join(tempDir, tt.name)
			require.NoError(t, os.MkdirAll(testDir, 0755))
			
			// Create test files
			for filename, content := range tt.files {
				filePath := filepath.Join(testDir, filename)
				require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0755))
				require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))
			}
			
			// Run the Go scanner
			originalDir, err := os.Getwd()
			require.NoError(t, err)
			
			defer func() {
				os.Chdir(originalDir)
			}()
			
			require.NoError(t, os.Chdir(testDir))
			
			// Copy the scanner to test directory
			scannerContent := `package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type NoTodoScanner struct {
	patterns *regexp.Regexp
	excludePatterns []string
}

func NewScanner() *NoTodoScanner {
	patterns := regexp.MustCompile("(?i)\\b(TODO|FIXME|XXX|STUB|PENDING)\\b")
	defaultExcludes := []string{"vendor/", ".git/"}
	
	return &NoTodoScanner{
		patterns:        patterns,
		excludePatterns: defaultExcludes,
	}
}

func (s *NoTodoScanner) shouldExclude(filePath string) bool {
	for _, pattern := range s.excludePatterns {
		if strings.Contains(filePath, strings.TrimSuffix(pattern, "/")) {
			return true
		}
	}
	return false
}

func (s *NoTodoScanner) isTextFile(filePath string) bool {
	return strings.HasSuffix(filePath, ".go")
}

func (s *NoTodoScanner) scanFile(filePath string) ([]string, error) {
	if s.shouldExclude(filePath) {
		return nil, nil
	}
	
	if !s.isTextFile(filePath) {
		return nil, nil
	}
	
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var results []string
	scanner := bufio.NewScanner(file)
	lineNum := 1
	
	for scanner.Scan() {
		line := scanner.Text()
		if s.patterns.MatchString(line) {
			results = append(results, fmt.Sprintf("%s:%d:%s", filePath, lineNum, strings.TrimSpace(line)))
		}
		lineNum++
	}
	
	return results, scanner.Err()
}

func main() {
	scanner := NewScanner()
	hasIssues := false
	
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		
		results, err := scanner.scanFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning %s: %v\n", path, err)
			return nil
		}
		
		if len(results) > 0 {
			hasIssues = true
			for _, result := range results {
				fmt.Println(result)
			}
		}
		
		return nil
	})
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
		os.Exit(1)
	}
	
	if hasIssues {
		os.Exit(1)
	}
}`
			
			scannerPath := filepath.Join(testDir, "scanner.go")
			require.NoError(t, os.WriteFile(scannerPath, []byte(scannerContent), 0644))
			
			// Run the scanner
			cmd := exec.Command("go", "run", "scanner.go")
			output, err := cmd.CombinedOutput()
			
			if tt.shouldFail {
				assert.Error(t, err, "Scanner should fail for %s: %s", tt.description, string(output))
			} else {
				assert.NoError(t, err, "Scanner should pass for %s: %s", tt.description, string(output))
			}
		})
	}
}

func TestQAGateIntegration(t *testing.T) {
	// Test that the actual scanner script exists and is executable
	_, err := os.Stat("scripts/qa/no_todo.sh")
	if err != nil {
		t.Skip("Skipping integration test: scripts/qa/no_todo.sh not found")
	}
	
	// Test that the Go scanner compiles
	cmd := exec.Command("go", "build", "-o", "/tmp/qa_scanner", "scripts/qa/scanner.go")
	err = cmd.Run()
	assert.NoError(t, err, "QA scanner should compile without errors")
	
	// Clean up
	os.Remove("/tmp/qa_scanner")
}