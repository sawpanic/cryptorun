package unit

import (
	"os"
	"path/filepath"
	"testing"

	"cryptorun/internal/qa"
)

func TestNoStubGate_DetectsStubs(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	
	// Create a fake Go file with a TODO
	testGoFile := filepath.Join(tempDir, "test.go")
	testContent := `package main

import "fmt"

func main() {
	// TODO: implement this function properly
	fmt.Println("Hello World")
}

func notImplemented() {
	panic("not implemented")
}`

	err := os.WriteFile(testGoFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Change to temp directory for testing
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)
	
	// Create audit directory
	auditDir := filepath.Join(tempDir, "audit")
	gate := qa.NewNoStubGate(auditDir)
	
	// Run scan
	report, err := gate.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	// Should detect both patterns
	if report.TotalHits != 2 {
		t.Errorf("Expected 2 hits, got %d", report.TotalHits)
	}
	
	if len(report.Hits) != 2 {
		t.Errorf("Expected 2 hit entries, got %d", len(report.Hits))
	}
	
	// Should have scanned 1 file
	if report.Scanned != 1 {
		t.Errorf("Expected 1 file scanned, got %d", report.Scanned)
	}
	
	// Check hit details
	foundTODO := false
	foundPanic := false
	
	for _, hit := range report.Hits {
		if hit.File == "test.go" && hit.Line == 6 {
			foundTODO = true
		}
		if hit.File == "test.go" && hit.Line == 10 {
			foundPanic = true
		}
	}
	
	if !foundTODO {
		t.Error("TODO pattern not detected")
	}
	
	if !foundPanic {
		t.Error("panic('not implemented') pattern not detected")
	}
}

func TestNoStubGate_ExcludesTestFiles(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a test file (should be excluded)
	testFile := filepath.Join(tempDir, "main_test.go")
	testContent := `package main

import "testing"

func TestSomething(t *testing.T) {
	// TODO: write actual test
	t.Skip("not implemented yet")
}`

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Create a regular Go file with TODO
	regularFile := filepath.Join(tempDir, "main.go")
	regularContent := `package main

func main() {
	// TODO: implement
}`

	err = os.WriteFile(regularFile, []byte(regularContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}
	
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)
	
	auditDir := filepath.Join(tempDir, "audit")
	gate := qa.NewNoStubGate(auditDir)
	
	report, err := gate.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	// Should only detect TODO in regular file, not test file
	if report.TotalHits != 1 {
		t.Errorf("Expected 1 hit, got %d", report.TotalHits)
	}
	
	if len(report.Hits) != 1 {
		t.Errorf("Expected 1 hit entry, got %d", len(report.Hits))
	}
	
	// Should have scanned only regular file
	if report.Scanned != 1 {
		t.Errorf("Expected 1 file scanned, got %d", report.Scanned)
	}
	
	// Should have excluded test file
	if report.Excluded != 1 {
		t.Errorf("Expected 1 file excluded, got %d", report.Excluded)
	}
	
	// Check that hit is from regular file, not test file
	hit := report.Hits[0]
	if hit.File != "main.go" {
		t.Errorf("Expected hit in main.go, got %s", hit.File)
	}
}

func TestNoStubGate_RunGate_FailsOnStubs(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create file with stub
	stubFile := filepath.Join(tempDir, "stub.go")
	stubContent := `package main

func stub() {
	// FIXME: this is a stub implementation
	return
}`

	err := os.WriteFile(stubFile, []byte(stubContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create stub file: %v", err)
	}
	
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)
	
	auditDir := filepath.Join(tempDir, "audit")
	gate := qa.NewNoStubGate(auditDir)
	
	// RunGate should fail
	err = gate.RunGate()
	if err == nil {
		t.Error("Expected RunGate to fail on stub, but it passed")
	}
	
	// Error should contain SCAFFOLDS_FOUND
	if !containsString(err.Error(), "SCAFFOLDS_FOUND") {
		t.Errorf("Error should contain 'SCAFFOLDS_FOUND', got: %v", err)
	}
	
	// Should create report file
	reportFile := filepath.Join(auditDir, "nostub_hits.json")
	if _, err := os.Stat(reportFile); os.IsNotExist(err) {
		t.Error("Expected report file to be created")
	}
}

func TestNoStubGate_RunGate_PassesOnCleanCode(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create clean file
	cleanFile := filepath.Join(tempDir, "clean.go")
	cleanContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, clean world!")
}

func factorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * factorial(n-1)
}`

	err := os.WriteFile(cleanFile, []byte(cleanContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create clean file: %v", err)
	}
	
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)
	
	auditDir := filepath.Join(tempDir, "audit")
	gate := qa.NewNoStubGate(auditDir)
	
	// RunGate should pass
	err = gate.RunGate()
	if err != nil {
		t.Errorf("Expected RunGate to pass on clean code, but got error: %v", err)
	}
}

func TestNoStubGate_PatternMatching(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create file with various stub patterns
	patternFile := filepath.Join(tempDir, "patterns.go")
	patternContent := `package main

func test1() {
	panic("not implemented")
}

func test2() {
	panic('not implemented')
}

func test3() {
	// TODO: implement this
}

func test4() {
	// FIXME: broken logic
}

func test5() {
	// This is a STUB function
}

func test6() {
	return nil // TODO
}

func test7() {
	// TODO: refactor this code
}

// NotImplemented marks unfinished functions
func test8() {
	// dummy implementation for now
}`

	err := os.WriteFile(patternFile, []byte(patternContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create pattern file: %v", err)
	}
	
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)
	
	auditDir := filepath.Join(tempDir, "audit")
	gate := qa.NewNoStubGate(auditDir)
	
	report, err := gate.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	// Should detect multiple patterns
	if report.TotalHits < 7 {
		t.Errorf("Expected at least 7 hits for various patterns, got %d", report.TotalHits)
	}
	
	// Verify specific patterns were found
	patterns := make(map[int]bool) // line number -> found
	for _, hit := range report.Hits {
		patterns[hit.Line] = true
	}
	
	expectedLines := []int{4, 8, 12, 16, 20, 24, 28, 32} // Lines with patterns
	for _, line := range expectedLines {
		if !patterns[line] {
			t.Errorf("Expected pattern on line %d to be detected", line)
		}
	}
}

// Helper function for string containment check
func containsString(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}