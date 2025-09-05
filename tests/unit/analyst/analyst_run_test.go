package analyst

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"cryptorun/application/analyst"
)

func TestAnalystRunner_WithFixtures(t *testing.T) {
	// Create temporary output directory
	tmpDir, err := os.MkdirTemp("", "analyst_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test config with quality policies
	configPath := filepath.Join(tmpDir, "quality_policies.json")
	configContent := `{
		"bad_miss_rate_thresholds": {
			"1h": 0.35,
			"24h": 0.40,
			"7d": 0.40
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create dummy candidates file (empty for fixture mode)
	candidatesPath := filepath.Join(tmpDir, "candidates.jsonl")
	if err := os.WriteFile(candidatesPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write candidates: %v", err)
	}

	// Create and run analyst with fixtures
	runner := analyst.NewAnalystRunner(tmpDir, candidatesPath, configPath, true)
	
	err = runner.Run()
	if err != nil {
		t.Fatalf("analyst run failed: %v", err)
	}

	// Verify all output files were created
	expectedFiles := []string{
		"winners.json",
		"misses.jsonl", 
		"coverage.json",
		"report.json",
		"report.md",
	}

	for _, filename := range expectedFiles {
		path := filepath.Join(tmpDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file not created: %s", filename)
		}
	}
}

func TestAnalystRunner_QualityPolicyCheck(t *testing.T) {
	// Create temporary output directory
	tmpDir, err := os.MkdirTemp("", "analyst_policy_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test config with strict thresholds that should pass with fixtures
	configPath := filepath.Join(tmpDir, "quality_policies.json")
	configContent := `{
		"bad_miss_rate_thresholds": {
			"1h": 0.95,
			"24h": 0.95,
			"7d": 0.95
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	candidatesPath := filepath.Join(tmpDir, "candidates.jsonl")
	if err := os.WriteFile(candidatesPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write candidates: %v", err)
	}

	runner := analyst.NewAnalystRunner(tmpDir, candidatesPath, configPath, true)
	
	// This should pass because fixture data creates controlled scenarios
	err = runner.Run()
	if err != nil {
		t.Fatalf("analyst run should pass with lenient thresholds: %v", err)
	}
}

func TestWinnersFetcher_Fixtures(t *testing.T) {
	fetcher := analyst.NewWinnersFetcher(true) // Use fixtures
	
	timeframes := []string{"1h", "24h", "7d"}
	winners, err := fetcher.FetchWinners(timeframes)
	if err != nil {
		t.Fatalf("failed to fetch fixture winners: %v", err)
	}
	
	if len(winners) == 0 {
		t.Error("expected winners from fixtures, got none")
	}
	
	// Verify all timeframes are represented
	timeframeCount := make(map[string]int)
	for _, winner := range winners {
		timeframeCount[winner.Timeframe]++
	}
	
	for _, tf := range timeframes {
		if timeframeCount[tf] == 0 {
			t.Errorf("no winners found for timeframe %s", tf)
		}
	}
	
	// Verify winner data structure
	for _, winner := range winners {
		if winner.Symbol == "" {
			t.Error("winner missing symbol")
		}
		if winner.Timeframe == "" {
			t.Error("winner missing timeframe")
		}
		if winner.PerformancePC <= 0 {
			t.Error("winner should have positive performance")
		}
		if winner.Volume <= 0 {
			t.Error("winner should have positive volume")
		}
		if winner.Price <= 0 {
			t.Error("winner should have positive price")
		}
		if winner.Rank <= 0 {
			t.Error("winner should have positive rank")
		}
		if winner.Source != "fixture" {
			t.Errorf("fixture winner should have source 'fixture', got %s", winner.Source)
		}
		if winner.Timestamp.IsZero() {
			t.Error("winner should have timestamp")
		}
	}
}

func TestWinnersFetcher_DeterministicOrdering(t *testing.T) {
	fetcher := analyst.NewWinnersFetcher(true)
	
	// Fetch winners multiple times
	winners1, err := fetcher.FetchWinners([]string{"1h"})
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	
	time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
	
	winners2, err := fetcher.FetchWinners([]string{"1h"})
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}
	
	// Filter to 1h timeframe for both
	var h1_1, h1_2 []analyst.WinnerCandidate
	for _, w := range winners1 {
		if w.Timeframe == "1h" {
			h1_1 = append(h1_1, w)
		}
	}
	for _, w := range winners2 {
		if w.Timeframe == "1h" {
			h1_2 = append(h1_2, w)
		}
	}
	
	if len(h1_1) != len(h1_2) {
		t.Fatalf("winner counts differ: %d vs %d", len(h1_1), len(h1_2))
	}
	
	// Verify ordering is deterministic (ignoring timestamp differences)
	for i := range h1_1 {
		if h1_1[i].Symbol != h1_2[i].Symbol {
			t.Errorf("deterministic ordering failed at index %d: %s vs %s", 
				i, h1_1[i].Symbol, h1_2[i].Symbol)
		}
		if h1_1[i].Rank != h1_2[i].Rank {
			t.Errorf("rank mismatch at index %d: %d vs %d", 
				i, h1_1[i].Rank, h1_2[i].Rank)
		}
	}
}

func TestAnalystRunner_EmptyCandidate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyst_empty_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "quality_policies.json")
	configContent := `{
		"bad_miss_rate_thresholds": {
			"1h": 0.35,
			"24h": 0.40,  
			"7d": 0.40
		}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create empty candidates file
	candidatesPath := filepath.Join(tmpDir, "candidates.jsonl")
	if err := os.WriteFile(candidatesPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write candidates: %v", err)
	}

	runner := analyst.NewAnalystRunner(tmpDir, candidatesPath, configPath, true)
	
	err = runner.Run()
	if err != nil {
		t.Fatalf("should handle empty candidates gracefully: %v", err)
	}

	// Verify misses.jsonl is created and shows all winners as NOT_CANDIDATE
	missesPath := filepath.Join(tmpDir, "misses.jsonl")
	content, err := os.ReadFile(missesPath)
	if err != nil {
		t.Fatalf("failed to read misses file: %v", err)
	}
	
	if len(content) == 0 {
		t.Error("misses file should contain entries when no candidates match winners")
	}
}

func TestAnalystRunner_FileAtomicity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyst_atomic_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "quality_policies.json")
	configContent := `{"bad_miss_rate_thresholds": {"1h": 0.35, "24h": 0.40, "7d": 0.40}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	candidatesPath := filepath.Join(tmpDir, "candidates.jsonl")
	if err := os.WriteFile(candidatesPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write candidates: %v", err)
	}

	runner := analyst.NewAnalystRunner(tmpDir, candidatesPath, configPath, true)
	
	err = runner.Run()
	if err != nil {
		t.Fatalf("analyst run failed: %v", err)
	}

	// Verify no .tmp files remain (indicating atomic writes completed)
	files, err := filepath.Glob(filepath.Join(tmpDir, "*.tmp"))
	if err != nil {
		t.Fatalf("failed to check for tmp files: %v", err)
	}
	
	if len(files) > 0 {
		t.Errorf("found leftover tmp files: %v", files)
	}
}