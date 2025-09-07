package explain

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type MockAtomicWriter struct {
	BaseDir string
}

func NewMockAtomicWriter(baseDir string) *MockAtomicWriter {
	if baseDir == "" {
		baseDir = "artifacts/explain"
	}
	return &MockAtomicWriter{BaseDir: baseDir}
}

func (w *MockAtomicWriter) WriteExplainReport(report interface{}, prefix string) error {
	if err := w.ensureDir(); err != nil {
		return err
	}

	timestamp := time.Now().UTC().Format("20060102-150405")

	jsonFile := filepath.Join(w.BaseDir, timestamp+"-"+prefix+"-explain.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonFile, data, 0644); err != nil {
		return err
	}

	csvFile := filepath.Join(w.BaseDir, timestamp+"-"+prefix+"-explain.csv")
	file, err := os.Create(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"symbol", "decision", "score", "rank", "momentum", "technical",
		"volume", "quality", "social", "entry_gate", "spread_bps",
		"depth_usd", "vadr", "heat_score", "regime", "exchange",
		"top_reason", "cache_hit_rate"}
	writer.Write(header)

	return nil
}

func (w *MockAtomicWriter) ensureDir() error {
	return os.MkdirAll(w.BaseDir, 0755)
}

func TestAtomicWriter(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewMockAtomicWriter(tempDir)

	report := &ExplainReport{
		Meta: ReportMeta{
			Timestamp:   time.Now().UTC(),
			Version:     "test-v1.0",
			AssetsCount: 2,
		},
		Universe: []AssetExplain{
			{
				Symbol:   "BTC-USD",
				Decision: "included",
				Score:    85.5,
				Rank:     1,
			},
			{
				Symbol:   "ETH-USD",
				Decision: "excluded",
				Score:    65.2,
				Rank:     2,
			},
		},
	}

	if err := writer.WriteExplainReport(report, "test"); err != nil {
		t.Fatalf("write explain report failed: %v", err)
	}

	jsonFiles, _ := filepath.Glob(filepath.Join(tempDir, "*-test-explain.json"))
	if len(jsonFiles) != 1 {
		t.Errorf("expected 1 JSON file, got %d", len(jsonFiles))
	}

	csvFiles, _ := filepath.Glob(filepath.Join(tempDir, "*-test-explain.csv"))
	if len(csvFiles) != 1 {
		t.Errorf("expected 1 CSV file, got %d", len(csvFiles))
	}

	data, err := os.ReadFile(jsonFiles[0])
	if err != nil {
		t.Fatalf("read JSON file failed: %v", err)
	}

	var parsed ExplainReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal JSON failed: %v", err)
	}

	if parsed.Meta.AssetsCount != 2 {
		t.Errorf("expected assets count 2, got %d", parsed.Meta.AssetsCount)
	}

	if len(parsed.Universe) != 2 {
		t.Errorf("expected 2 universe entries, got %d", len(parsed.Universe))
	}
}
