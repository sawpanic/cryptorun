package atomicio

import (
	"os"
	"path/filepath"
	"testing"

	"cryptorun/internal/atomicio"
)

func TestWriteFile(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := []byte("test content for atomic write")

	// Test atomic write
	err := atomicio.WriteFile(testFile, testContent, 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify file exists and content is correct
	readContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(readContent) != string(testContent) {
		t.Fatalf("Content mismatch: expected %q, got %q", string(testContent), string(readContent))
	}

	// Verify temp file was cleaned up
	tempFile := testFile + ".tmp"
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Fatalf("Temp file was not cleaned up: %s", tempFile)
	}
}

func TestWriteFileError(t *testing.T) {
	// Test with invalid path (should fail gracefully)
	invalidPath := "/invalid/path/that/does/not/exist/file.txt"
	err := atomicio.WriteFile(invalidPath, []byte("test"), 0644)
	if err == nil {
		t.Fatal("Expected error for invalid path, but got nil")
	}
}
