package artifacts

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"cryptorun/internal/artifacts/manifest"
)

func TestManifest_NewManifest(t *testing.T) {
	m := manifest.NewManifest()

	if m.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", m.Version)
	}

	if len(m.Entries) != 0 {
		t.Errorf("Expected empty entries, got %d", len(m.Entries))
	}

	if m.ByID == nil || m.ByFamily == nil || m.ByRunID == nil {
		t.Error("Expected indices to be initialized")
	}
}

func TestManifest_AddEntry(t *testing.T) {
	m := manifest.NewManifest()

	entry := manifest.ArtifactEntry{
		Family:     "test",
		RunID:      "run123",
		Timestamp:  time.Now(),
		Paths:      []string{"/test/path1.jsonl"},
		TotalBytes: 1024,
		PassFail:   "pass",
	}

	m.AddEntry(entry)

	// Check entry was added
	if len(m.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(m.Entries))
	}

	// Check ID was generated
	if m.Entries[0].ID == "" {
		t.Error("Expected ID to be generated")
	}

	// Check indices were updated
	if m.ByID[m.Entries[0].ID] == nil {
		t.Error("Entry not found in ByID index")
	}

	if len(m.ByFamily["test"]) != 1 {
		t.Error("Entry not found in ByFamily index")
	}

	if m.ByRunID["run123"] == nil {
		t.Error("Entry not found in ByRunID index")
	}

	// Check family counter
	if m.Families["test"] != 1 {
		t.Errorf("Expected family count 1, got %d", m.Families["test"])
	}
}

func TestManifest_GetByFamily(t *testing.T) {
	m := manifest.NewManifest()

	// Add entries with different timestamps
	now := time.Now()
	entries := []manifest.ArtifactEntry{
		{
			Family: "test", RunID: "run1", Timestamp: now.Add(-2 * time.Hour),
			Paths: []string{"/test/path1.jsonl"}, PassFail: "pass",
		},
		{
			Family: "test", RunID: "run2", Timestamp: now.Add(-1 * time.Hour),
			Paths: []string{"/test/path2.jsonl"}, PassFail: "fail",
		},
		{
			Family: "test", RunID: "run3", Timestamp: now,
			Paths: []string{"/test/path3.jsonl"}, PassFail: "pass",
		},
		{
			Family: "other", RunID: "run4", Timestamp: now,
			Paths: []string{"/test/path4.jsonl"}, PassFail: "pass",
		},
	}

	for _, entry := range entries {
		m.AddEntry(entry)
	}

	// Get test family entries (should be sorted newest first)
	testEntries := m.GetByFamily("test")
	if len(testEntries) != 3 {
		t.Fatalf("Expected 3 entries for test family, got %d", len(testEntries))
	}

	// Check sorting (newest first)
	if testEntries[0].RunID != "run3" {
		t.Errorf("Expected newest entry first, got %s", testEntries[0].RunID)
	}

	if testEntries[1].RunID != "run2" {
		t.Errorf("Expected middle entry second, got %s", testEntries[1].RunID)
	}

	if testEntries[2].RunID != "run1" {
		t.Errorf("Expected oldest entry last, got %s", testEntries[2].RunID)
	}

	// Test non-existent family
	nonExistent := m.GetByFamily("nonexistent")
	if nonExistent != nil {
		t.Error("Expected nil for non-existent family")
	}
}

func TestManifest_MarkLastRuns(t *testing.T) {
	m := manifest.NewManifest()

	now := time.Now()
	entries := []manifest.ArtifactEntry{
		{
			Family: "test", RunID: "run1", Timestamp: now.Add(-3 * time.Hour),
			Paths: []string{"/test/path1.jsonl"}, PassFail: "pass",
		},
		{
			Family: "test", RunID: "run2", Timestamp: now.Add(-2 * time.Hour),
			Paths: []string{"/test/path2.jsonl"}, PassFail: "fail",
		},
		{
			Family: "test", RunID: "run3", Timestamp: now.Add(-1 * time.Hour),
			Paths: []string{"/test/path3.jsonl"}, PassFail: "pass",
		},
		{
			Family: "test", RunID: "run4", Timestamp: now,
			Paths: []string{"/test/path4.jsonl"}, PassFail: "fail",
		},
	}

	for _, entry := range entries {
		m.AddEntry(entry)
	}

	m.MarkLastRuns()

	// Check last run (most recent)
	run4Entry := m.ByRunID["run4"]
	if run4Entry == nil || !run4Entry.IsLastRun {
		t.Error("Expected run4 to be marked as last run")
	}

	// Check last pass (most recent pass)
	run3Entry := m.ByRunID["run3"]
	if run3Entry == nil || !run3Entry.IsLastPass {
		t.Error("Expected run3 to be marked as last pass")
	}

	// Ensure other entries are not marked
	run1Entry := m.ByRunID["run1"]
	if run1Entry == nil || run1Entry.IsLastRun || run1Entry.IsLastPass {
		t.Error("Expected run1 to not be marked as last run or pass")
	}
}

func TestManifest_SetPinned(t *testing.T) {
	m := manifest.NewManifest()

	entry := manifest.ArtifactEntry{
		Family: "test", RunID: "run1", Timestamp: time.Now(),
		Paths: []string{"/test/path1.jsonl"}, PassFail: "pass",
	}

	m.AddEntry(entry)
	entryID := m.Entries[0].ID

	// Pin the entry
	err := m.SetPinned(entryID, true)
	if err != nil {
		t.Fatalf("Failed to pin entry: %v", err)
	}

	if !m.Entries[0].IsPinned {
		t.Error("Expected entry to be pinned")
	}

	// Unpin the entry
	err = m.SetPinned(entryID, false)
	if err != nil {
		t.Fatalf("Failed to unpin entry: %v", err)
	}

	if m.Entries[0].IsPinned {
		t.Error("Expected entry to be unpinned")
	}

	// Test non-existent entry
	err = m.SetPinned("nonexistent", true)
	if err == nil {
		t.Error("Expected error for non-existent entry")
	}
}

func TestManifest_UpdateSummary(t *testing.T) {
	m := manifest.NewManifest()

	now := time.Now()
	entries := []manifest.ArtifactEntry{
		{
			Family: "test", RunID: "run1", Timestamp: now.Add(-1 * time.Hour),
			Paths:      []string{"/test/path1.jsonl", "/test/path1.md"},
			TotalBytes: 1024, PassFail: "pass", IsPinned: true,
		},
		{
			Family: "test", RunID: "run2", Timestamp: now,
			Paths:      []string{"/test/path2.jsonl"},
			TotalBytes: 2048, PassFail: "fail",
		},
		{
			Family: "other", RunID: "run3", Timestamp: now.Add(-2 * time.Hour),
			Paths:      []string{"/test/path3.jsonl"},
			TotalBytes: 512, PassFail: "pass",
		},
	}

	for _, entry := range entries {
		m.AddEntry(entry)
	}

	m.UpdateSummary()

	// Check totals
	if m.Summary.TotalEntries != 3 {
		t.Errorf("Expected 3 total entries, got %d", m.Summary.TotalEntries)
	}

	if m.Summary.TotalBytes != 3584 {
		t.Errorf("Expected 3584 total bytes, got %d", m.Summary.TotalBytes)
	}

	if m.Summary.TotalFiles != 4 {
		t.Errorf("Expected 4 total files, got %d", m.Summary.TotalFiles)
	}

	// Check family counts
	if m.Summary.FamilyCounts["test"] != 2 {
		t.Errorf("Expected 2 test entries, got %d", m.Summary.FamilyCounts["test"])
	}

	if m.Summary.FamilyCounts["other"] != 1 {
		t.Errorf("Expected 1 other entry, got %d", m.Summary.FamilyCounts["other"])
	}

	// Check status counts
	if m.Summary.PassCount != 2 {
		t.Errorf("Expected 2 pass entries, got %d", m.Summary.PassCount)
	}

	if m.Summary.FailCount != 1 {
		t.Errorf("Expected 1 fail entry, got %d", m.Summary.FailCount)
	}

	if m.Summary.PinnedCount != 1 {
		t.Errorf("Expected 1 pinned entry, got %d", m.Summary.PinnedCount)
	}

	// Check timestamp bounds
	expectedOldest := now.Add(-2 * time.Hour).Truncate(time.Second)
	actualOldest := m.Summary.OldestEntry.Truncate(time.Second)
	if !actualOldest.Equal(expectedOldest) {
		t.Errorf("Expected oldest %v, got %v", expectedOldest, actualOldest)
	}

	expectedNewest := now.Truncate(time.Second)
	actualNewest := m.Summary.NewestEntry.Truncate(time.Second)
	if !actualNewest.Equal(expectedNewest) {
		t.Errorf("Expected newest %v, got %v", expectedNewest, actualNewest)
	}
}

func TestScanner_Integration(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir := t.TempDir()

	// Create test directories and files
	testDirs := []string{
		filepath.Join(tempDir, "artifacts", "proofs"),
		filepath.Join(tempDir, "artifacts", "bench"),
		filepath.Join(tempDir, "artifacts", "smoke90"),
	}

	for _, dir := range testDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
	}

	// Create test files
	testFiles := map[string]string{
		filepath.Join(tempDir, "artifacts", "proofs", "run_20250906_143022.jsonl"):  `{"test": "data"}`,
		filepath.Join(tempDir, "artifacts", "proofs", "run_20250906_143022.md"):     "# Test Report\nPASS",
		filepath.Join(tempDir, "artifacts", "bench", "bench_20250906_144500.jsonl"): `{"benchmark": "result"}`,
		filepath.Join(tempDir, "artifacts", "smoke90", "smoke_fail_20250906.jsonl"): `{"smoke": "fail"}`,
	}

	for path, content := range testFiles {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Create scanner configuration
	config := &manifest.ScanConfig{
		RootPaths: []string{tempDir},
		FamilyPatterns: map[string][]string{
			"proofs":  {"**/proofs/*.jsonl", "**/proofs/*.md"},
			"bench":   {"**/bench/*.jsonl"},
			"smoke90": {"**/smoke90/*.jsonl"},
		},
		WorkerCount:        2,
		ChecksumBufferSize: 1024,
		MaxFilesPerScan:    100,
		FollowSymlinks:     false,
	}

	// Create and run scanner
	scanner := manifest.NewScanner(config)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Validate scan results
	if len(result.Manifest.Entries) != 3 { // 3 artifact groups (proofs counted as 1)
		t.Errorf("Expected 3 entries, got %d", len(result.Manifest.Entries))
	}

	if result.FilesScanned != 4 {
		t.Errorf("Expected 4 files scanned, got %d", result.FilesScanned)
	}

	if result.ErrorCount != 0 {
		t.Errorf("Expected 0 errors, got %d", result.ErrorCount)
	}

	// Check families were detected correctly
	familyCounts := result.Manifest.Families
	if familyCounts["proofs"] != 1 {
		t.Errorf("Expected 1 proofs entry, got %d", familyCounts["proofs"])
	}

	if familyCounts["bench"] != 1 {
		t.Errorf("Expected 1 bench entry, got %d", familyCounts["bench"])
	}

	if familyCounts["smoke90"] != 1 {
		t.Errorf("Expected 1 smoke90 entry, got %d", familyCounts["smoke90"])
	}

	// Test pass/fail detection
	found := false
	for _, entry := range result.Manifest.Entries {
		if entry.Family == "smoke90" {
			if entry.PassFail != "fail" {
				t.Errorf("Expected smoke90 entry to be marked as fail, got %s", entry.PassFail)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("smoke90 entry not found")
	}
}
