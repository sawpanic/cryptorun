package gc

import (
	"fmt"
	"sort"
	"time"

	"cryptorun/internal/artifacts/manifest"
)

// Plan represents a garbage collection plan
type Plan struct {
	// Metadata
	CreatedAt    time.Time `json:"created_at"`
	DryRun       bool      `json:"dry_run"`
	TotalEntries int       `json:"total_entries"`

	// Actions by family
	FamilyPlans map[string]FamilyPlan `json:"family_plans"`

	// Summary
	TotalToDelete int   `json:"total_to_delete"`
	TotalToKeep   int   `json:"total_to_keep"`
	BytesToDelete int64 `json:"bytes_to_delete"`
	FilesToDelete int   `json:"files_to_delete"`

	// Safety information
	PinnedKept   int `json:"pinned_kept"`
	LastPassKept int `json:"last_pass_kept"`
	LastRunKept  int `json:"last_run_kept"`
}

// FamilyPlan represents the GC plan for a specific artifact family
type FamilyPlan struct {
	Family        string              `json:"family"`
	TotalEntries  int                 `json:"total_entries"`
	KeepCount     int                 `json:"keep_count"`
	ToDelete      []string            `json:"to_delete"`      // Entry IDs to delete
	ToKeep        []string            `json:"to_keep"`        // Entry IDs to keep
	ReasonToKeep  map[string][]string `json:"reason_to_keep"` // Why each entry is kept
	BytesToDelete int64               `json:"bytes_to_delete"`
	FilesToDelete int                 `json:"files_to_delete"`
}

// RetentionConfig defines retention rules for GC planning
type RetentionConfig struct {
	Keep            int      `yaml:"keep"`        // Number of recent entries to keep
	Pin             []string `yaml:"pin"`         // Pinned entry IDs (never delete)
	AlwaysKeepRules []string `yaml:"always_keep"` // Rules: last_pass, last_run, pinned
}

// Planner creates garbage collection plans
type Planner struct {
	config map[string]RetentionConfig
}

// NewPlanner creates a new GC planner with the given retention configuration
func NewPlanner(config map[string]RetentionConfig) *Planner {
	return &Planner{
		config: config,
	}
}

// CreatePlan generates a garbage collection plan for the given manifest
func (p *Planner) CreatePlan(manifest *manifest.Manifest, dryRun bool) (*Plan, error) {
	plan := &Plan{
		CreatedAt:    time.Now(),
		DryRun:       dryRun,
		TotalEntries: len(manifest.Entries),
		FamilyPlans:  make(map[string]FamilyPlan),
	}

	// Process each family separately
	for family := range manifest.Families {
		familyPlan, err := p.planFamily(manifest, family)
		if err != nil {
			return nil, fmt.Errorf("failed to plan family %s: %w", family, err)
		}

		plan.FamilyPlans[family] = familyPlan
		plan.TotalToDelete += len(familyPlan.ToDelete)
		plan.TotalToKeep += len(familyPlan.ToKeep)
		plan.BytesToDelete += familyPlan.BytesToDelete
		plan.FilesToDelete += familyPlan.FilesToDelete
	}

	// Calculate safety counts
	plan.PinnedKept = p.countPinnedKept(manifest, plan)
	plan.LastPassKept = p.countLastPassKept(manifest, plan)
	plan.LastRunKept = p.countLastRunKept(manifest, plan)

	return plan, nil
}

// planFamily creates a GC plan for a specific family
func (p *Planner) planFamily(manifest *manifest.Manifest, family string) (FamilyPlan, error) {
	entries := manifest.GetByFamily(family)
	if len(entries) == 0 {
		return FamilyPlan{Family: family}, nil
	}

	// Get retention config for this family
	retentionConfig, exists := p.config[family]
	if !exists {
		// Use default config if family not specified
		retentionConfig = RetentionConfig{
			Keep:            5,
			AlwaysKeepRules: []string{"last_pass", "last_run", "pinned"},
		}
	}

	familyPlan := FamilyPlan{
		Family:       family,
		TotalEntries: len(entries),
		ToDelete:     make([]string, 0),
		ToKeep:       make([]string, 0),
		ReasonToKeep: make(map[string][]string),
	}

	// Sort entries by timestamp (newest first)
	sortedEntries := entries
	if len(sortedEntries) == 0 {
		return familyPlan, nil
	}
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].Timestamp.After(sortedEntries[j].Timestamp)
	})

	// Apply retention logic
	for i, entry := range sortedEntries {
		reasons := p.shouldKeepEntry(entry, i, retentionConfig)

		if len(reasons) > 0 {
			// Keep this entry
			familyPlan.ToKeep = append(familyPlan.ToKeep, entry.ID)
			familyPlan.ReasonToKeep[entry.ID] = reasons
			familyPlan.KeepCount++
		} else {
			// Mark for deletion
			familyPlan.ToDelete = append(familyPlan.ToDelete, entry.ID)
			familyPlan.BytesToDelete += entry.TotalBytes
			familyPlan.FilesToDelete += len(entry.Paths)
		}
	}

	return familyPlan, nil
}

// shouldKeepEntry determines if an entry should be kept and why
func (p *Planner) shouldKeepEntry(entry *manifest.ArtifactEntry, index int, config RetentionConfig) []string {
	var reasons []string

	// Check always-keep rules
	for _, rule := range config.AlwaysKeepRules {
		switch rule {
		case "pinned":
			if entry.IsPinned {
				reasons = append(reasons, "pinned")
			}
		case "last_pass":
			if entry.IsLastPass {
				reasons = append(reasons, "last_pass")
			}
		case "last_run":
			if entry.IsLastRun {
				reasons = append(reasons, "last_run")
			}
		}
	}

	// Check explicit pin list
	for _, pinnedID := range config.Pin {
		if entry.ID == pinnedID {
			reasons = append(reasons, "explicitly_pinned")
			break
		}
	}

	// Check retention count (index-based, 0 = newest)
	if index < config.Keep {
		reasons = append(reasons, fmt.Sprintf("within_keep_count_%d", config.Keep))
	}

	return reasons
}

// countPinnedKept counts how many pinned entries are being kept
func (p *Planner) countPinnedKept(manifest *manifest.Manifest, plan *Plan) int {
	count := 0
	for _, familyPlan := range plan.FamilyPlans {
		for entryID, reasons := range familyPlan.ReasonToKeep {
			for _, reason := range reasons {
				if reason == "pinned" || reason == "explicitly_pinned" {
					count++
					break
				}
			}
			_ = entryID // Suppress unused variable warning
		}
	}
	return count
}

// countLastPassKept counts how many last-pass entries are being kept
func (p *Planner) countLastPassKept(manifest *manifest.Manifest, plan *Plan) int {
	count := 0
	for _, familyPlan := range plan.FamilyPlans {
		for _, reasons := range familyPlan.ReasonToKeep {
			for _, reason := range reasons {
				if reason == "last_pass" {
					count++
					break
				}
			}
		}
	}
	return count
}

// countLastRunKept counts how many last-run entries are being kept
func (p *Planner) countLastRunKept(manifest *manifest.Manifest, plan *Plan) int {
	count := 0
	for _, familyPlan := range plan.FamilyPlans {
		for _, reasons := range familyPlan.ReasonToKeep {
			for _, reason := range reasons {
				if reason == "last_run" {
					count++
					break
				}
			}
		}
	}
	return count
}

// ValidatePlan performs safety checks on a GC plan
func (p *Planner) ValidatePlan(plan *Plan, manifest *manifest.Manifest) error {
	// Ensure we're not deleting all entries from any family
	for family, familyPlan := range plan.FamilyPlans {
		if familyPlan.TotalEntries > 0 && len(familyPlan.ToKeep) == 0 {
			return fmt.Errorf("plan would delete all entries from family %s", family)
		}

		// Ensure we keep at least the most recent entry
		familyEntries := manifest.GetByFamily(family)
		if len(familyEntries) > 0 {
			mostRecent := familyEntries[0] // Already sorted newest first
			found := false
			for _, keepID := range familyPlan.ToKeep {
				if keepID == mostRecent.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("plan would delete most recent entry from family %s", family)
			}
		}
	}

	// Ensure no pinned entries are marked for deletion
	for _, entry := range manifest.Entries {
		if entry.IsPinned {
			if familyPlan, exists := plan.FamilyPlans[entry.Family]; exists {
				for _, deleteID := range familyPlan.ToDelete {
					if deleteID == entry.ID {
						return fmt.Errorf("plan would delete pinned entry %s", entry.ID)
					}
				}
			}
		}
	}

	return nil
}

// GetPlanSummary returns a human-readable summary of the plan
func (p *Planner) GetPlanSummary(plan *Plan) string {
	summary := fmt.Sprintf("GC Plan Summary (DryRun: %v)\n", plan.DryRun)
	summary += fmt.Sprintf("Total Entries: %d\n", plan.TotalEntries)
	summary += fmt.Sprintf("To Delete: %d entries (%s, %d files)\n",
		plan.TotalToDelete, formatBytes(plan.BytesToDelete), plan.FilesToDelete)
	summary += fmt.Sprintf("To Keep: %d entries\n", plan.TotalToKeep)
	summary += fmt.Sprintf("Safety: %d pinned, %d last-pass, %d last-run kept\n",
		plan.PinnedKept, plan.LastPassKept, plan.LastRunKept)
	summary += "\nBy Family:\n"

	for family, familyPlan := range plan.FamilyPlans {
		summary += fmt.Sprintf("  %s: keep %d/%d, delete %s\n",
			family, len(familyPlan.ToKeep), familyPlan.TotalEntries,
			formatBytes(familyPlan.BytesToDelete))
	}

	return summary
}

// formatBytes formats byte counts in human-readable form
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
}
