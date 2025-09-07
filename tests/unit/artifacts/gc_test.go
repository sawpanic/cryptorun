package artifacts

import (
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/artifacts/gc"
	"github.com/sawpanic/cryptorun/internal/artifacts/manifest"
)

func TestPlan_RetentionMath(t *testing.T) {
	// Create test manifest with multiple entries
	m := manifest.NewManifest()

	now := time.Now()
	entries := []manifest.ArtifactEntry{
		// Test family - 5 entries, keep 3
		{Family: "test", RunID: "run1", Timestamp: now.Add(-4 * time.Hour), PassFail: "pass", TotalBytes: 1000, Paths: []string{"/test1"}},
		{Family: "test", RunID: "run2", Timestamp: now.Add(-3 * time.Hour), PassFail: "fail", TotalBytes: 2000, Paths: []string{"/test2"}},
		{Family: "test", RunID: "run3", Timestamp: now.Add(-2 * time.Hour), PassFail: "pass", TotalBytes: 1500, Paths: []string{"/test3"}},
		{Family: "test", RunID: "run4", Timestamp: now.Add(-1 * time.Hour), PassFail: "fail", TotalBytes: 3000, Paths: []string{"/test4"}},
		{Family: "test", RunID: "run5", Timestamp: now, PassFail: "pass", TotalBytes: 2500, Paths: []string{"/test5"}},
	}

	for _, entry := range entries {
		m.AddEntry(entry)
	}

	m.MarkLastRuns()

	// Configure retention: keep 3 entries
	retentionConfig := map[string]gc.RetentionConfig{
		"test": {
			Keep:            3,
			AlwaysKeepRules: []string{"last_pass", "last_run", "pinned"},
		},
	}

	planner := gc.NewPlanner(retentionConfig)
	plan, err := planner.CreatePlan(m, true)
	if err != nil {
		t.Fatalf("Failed to create plan: %v", err)
	}

	testPlan := plan.FamilyPlans["test"]

	// Should keep 3 most recent + any last_pass/last_run not in top 3
	// run5 (newest, last_run, last_pass), run4, run3 should be kept by count
	// run1 was an earlier pass but should be deleted since run5 is the last_pass
	// run2 should be deleted

	if len(testPlan.ToKeep) < 3 {
		t.Errorf("Expected at least 3 entries to keep, got %d", len(testPlan.ToKeep))
	}

	if len(testPlan.ToDelete) != 5-len(testPlan.ToKeep) {
		t.Errorf("ToKeep + ToDelete should equal 5, got keep=%d delete=%d",
			len(testPlan.ToKeep), len(testPlan.ToDelete))
	}

	// Validate that most recent entries are kept
	keepReasons := testPlan.ReasonToKeep

	// run5 should be kept (newest, last_run, last_pass)
	run5Entry := m.ByRunID["run5"]
	if run5Entry == nil {
		t.Fatal("run5 entry not found")
	}

	found := false
	for _, keepID := range testPlan.ToKeep {
		if keepID == run5Entry.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected run5 (newest) to be kept")
	}

	// Check reasons for keeping run5
	if reasons, exists := keepReasons[run5Entry.ID]; exists {
		hasKeepCountReason := false
		hasLastRunReason := false
		hasLastPassReason := false

		for _, reason := range reasons {
			if reason == "within_keep_count_3" {
				hasKeepCountReason = true
			}
			if reason == "last_run" {
				hasLastRunReason = true
			}
			if reason == "last_pass" {
				hasLastPassReason = true
			}
		}

		if !hasKeepCountReason {
			t.Error("Expected run5 to be kept due to keep count")
		}
		if !hasLastRunReason {
			t.Error("Expected run5 to be kept as last run")
		}
		if !hasLastPassReason {
			t.Error("Expected run5 to be kept as last pass")
		}
	} else {
		t.Error("No reasons found for keeping run5")
	}
}

func TestPlan_KeepsPinnedAndLastPass(t *testing.T) {
	m := manifest.NewManifest()

	now := time.Now()
	entries := []manifest.ArtifactEntry{
		{Family: "test", RunID: "run1", Timestamp: now.Add(-3 * time.Hour), PassFail: "pass", IsPinned: true, TotalBytes: 1000, Paths: []string{"/test1"}},
		{Family: "test", RunID: "run2", Timestamp: now.Add(-2 * time.Hour), PassFail: "fail", TotalBytes: 2000, Paths: []string{"/test2"}},
		{Family: "test", RunID: "run3", Timestamp: now.Add(-1 * time.Hour), PassFail: "pass", TotalBytes: 1500, Paths: []string{"/test3"}},
		{Family: "test", RunID: "run4", Timestamp: now, PassFail: "fail", TotalBytes: 3000, Paths: []string{"/test4"}},
	}

	for _, entry := range entries {
		m.AddEntry(entry)
	}

	m.MarkLastRuns()

	// Configure retention: keep only 1 entry (but pinned and last_pass should still be kept)
	retentionConfig := map[string]gc.RetentionConfig{
		"test": {
			Keep:            1,
			AlwaysKeepRules: []string{"last_pass", "last_run", "pinned"},
		},
	}

	planner := gc.NewPlanner(retentionConfig)
	plan, err := planner.CreatePlan(m, true)
	if err != nil {
		t.Fatalf("Failed to create plan: %v", err)
	}

	testPlan := plan.FamilyPlans["test"]

	// Should keep:
	// - run1 (pinned)
	// - run3 (last pass)
	// - run4 (last run, within keep count)
	// Should delete: run2

	if len(testPlan.ToKeep) < 3 {
		t.Errorf("Expected at least 3 entries to keep (pinned + last_pass + last_run), got %d", len(testPlan.ToKeep))
	}

	// Check that pinned entry is kept
	run1Entry := m.ByRunID["run1"]
	found := false
	for _, keepID := range testPlan.ToKeep {
		if keepID == run1Entry.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected pinned entry to be kept")
	}

	// Check that last pass is kept
	run3Entry := m.ByRunID["run3"]
	found = false
	for _, keepID := range testPlan.ToKeep {
		if keepID == run3Entry.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected last pass entry to be kept")
	}

	// Check that last run is kept
	run4Entry := m.ByRunID["run4"]
	found = false
	for _, keepID := range testPlan.ToKeep {
		if keepID == run4Entry.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected last run entry to be kept")
	}
}

func TestPlan_ValidatePlan(t *testing.T) {
	m := manifest.NewManifest()

	now := time.Now()
	entries := []manifest.ArtifactEntry{
		{Family: "test", RunID: "run1", Timestamp: now.Add(-1 * time.Hour), PassFail: "pass", IsPinned: true, TotalBytes: 1000, Paths: []string{"/test1"}},
		{Family: "test", RunID: "run2", Timestamp: now, PassFail: "fail", TotalBytes: 2000, Paths: []string{"/test2"}},
	}

	for _, entry := range entries {
		m.AddEntry(entry)
	}

	m.MarkLastRuns()

	retentionConfig := map[string]gc.RetentionConfig{
		"test": {
			Keep:            1,
			AlwaysKeepRules: []string{"last_pass", "last_run", "pinned"},
		},
	}

	planner := gc.NewPlanner(retentionConfig)

	// Create valid plan
	plan, err := planner.CreatePlan(m, true)
	if err != nil {
		t.Fatalf("Failed to create plan: %v", err)
	}

	// Should pass validation
	err = planner.ValidatePlan(plan, m)
	if err != nil {
		t.Errorf("Valid plan failed validation: %v", err)
	}

	// Test invalid plan - manually create plan that would delete pinned entry
	invalidPlan := &gc.Plan{
		FamilyPlans: map[string]gc.FamilyPlan{
			"test": {
				ToDelete: []string{m.ByRunID["run1"].ID}, // Try to delete pinned entry
				ToKeep:   []string{m.ByRunID["run2"].ID},
			},
		},
	}

	err = planner.ValidatePlan(invalidPlan, m)
	if err == nil {
		t.Error("Expected validation to fail for plan that deletes pinned entry")
	}
}

func TestPlan_GetPlanSummary(t *testing.T) {
	m := manifest.NewManifest()

	now := time.Now()
	entries := []manifest.ArtifactEntry{
		{Family: "test", RunID: "run1", Timestamp: now.Add(-2 * time.Hour), PassFail: "pass", TotalBytes: 1000, Paths: []string{"/test1"}},
		{Family: "test", RunID: "run2", Timestamp: now.Add(-1 * time.Hour), PassFail: "fail", TotalBytes: 2000, Paths: []string{"/test2", "/test2.md"}},
		{Family: "test", RunID: "run3", Timestamp: now, PassFail: "pass", TotalBytes: 1500, Paths: []string{"/test3"}},
	}

	for _, entry := range entries {
		m.AddEntry(entry)
	}

	m.MarkLastRuns()

	retentionConfig := map[string]gc.RetentionConfig{
		"test": {
			Keep:            2,
			AlwaysKeepRules: []string{"last_pass", "last_run"},
		},
	}

	planner := gc.NewPlanner(retentionConfig)
	plan, err := planner.CreatePlan(m, true)
	if err != nil {
		t.Fatalf("Failed to create plan: %v", err)
	}

	summary := planner.GetPlanSummary(plan)

	// Check that summary contains expected information
	if summary == "" {
		t.Error("Expected non-empty summary")
	}

	// Should mention dry run
	if !contains(summary, "DryRun: true") {
		t.Error("Summary should mention dry run status")
	}

	// Should mention total entries
	if !contains(summary, "Total Entries: 3") {
		t.Error("Summary should mention total entries")
	}

	// Should mention family breakdown
	if !contains(summary, "test:") {
		t.Error("Summary should mention family breakdown")
	}
}

func TestPlan_EmptyFamily(t *testing.T) {
	m := manifest.NewManifest()

	// Add entries for one family only
	now := time.Now()
	entry := manifest.ArtifactEntry{
		Family: "test", RunID: "run1", Timestamp: now, PassFail: "pass",
		TotalBytes: 1000, Paths: []string{"/test1"},
	}
	m.AddEntry(entry)

	// Configure retention for different family
	retentionConfig := map[string]gc.RetentionConfig{
		"other": {
			Keep:            5,
			AlwaysKeepRules: []string{"last_pass", "last_run"},
		},
	}

	planner := gc.NewPlanner(retentionConfig)
	plan, err := planner.CreatePlan(m, true)
	if err != nil {
		t.Fatalf("Failed to create plan: %v", err)
	}

	// Should have plan for test family (using defaults) but not for other family
	if _, exists := plan.FamilyPlans["test"]; !exists {
		t.Error("Expected plan for test family")
	}

	if _, exists := plan.FamilyPlans["other"]; exists {
		t.Error("Should not have plan for empty family")
	}
}

// Helper function
func contains(text, substring string) bool {
	return len(text) >= len(substring) && (text[:len(substring)] == substring ||
		(len(text) > len(substring) && contains(text[1:], substring)))
}
