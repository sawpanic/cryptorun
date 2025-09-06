package artifacts

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "artifacts-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldDir := artifactsDir
	testDir := filepath.Join(tmpDir, "test-ledger")
	artifactsDir = testDir
	defer func() { artifactsDir = oldDir }()

	testData := map[string]interface{}{
		"timestamp": "2024-09-06T14:30:22Z",
		"component": "scanner",
		"version":   "v3.2.1",
		"data": map[string]interface{}{
			"pair":  "BTC-USD",
			"score": 82.5,
		},
	}

	if err := WriteJSON("test-results", testData); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read test dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	filename := files[0].Name()
	if !strings.HasSuffix(filename, "-test-results.json") {
		t.Errorf("Unexpected filename: %s", filename)
	}

	filePath := filepath.Join(testDir, filename)
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result["component"] != "scanner" {
		t.Errorf("Expected component 'scanner', got %v", result["component"])
	}
}

func TestWriteCSV(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "artifacts-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldDir := artifactsDir
	testDir := filepath.Join(tmpDir, "test-ledger")
	artifactsDir = testDir
	defer func() { artifactsDir = oldDir }()

	rows := [][]string{
		{"timestamp", "pair", "score", "volume"},
		{"2024-09-06T14:30:22Z", "BTC-USD", "82.5", "1234567"},
		{"2024-09-06T14:30:22Z", "ETH-USD", "76.2", "987654"},
	}

	if err := WriteCSV("top-pairs", rows); err != nil {
		t.Fatalf("WriteCSV failed: %v", err)
	}

	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read test dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	filename := files[0].Name()
	if !strings.HasSuffix(filename, "-top-pairs.csv") {
		t.Errorf("Unexpected filename: %s", filename)
	}

	filePath := filepath.Join(testDir, filename)
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}
	if records[0][1] != "pair" {
		t.Errorf("Expected header 'pair', got %s", records[0][1])
	}
	if records[1][1] != "BTC-USD" {
		t.Errorf("Expected 'BTC-USD', got %s", records[1][1])
	}
}