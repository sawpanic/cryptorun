package manifest

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// ArtifactEntry represents a single artifact in the manifest
type ArtifactEntry struct {
	// Identity
	ID     string `json:"id"`     // Unique identifier (hash-based)
	RunID  string `json:"run_id"` // Run identifier from filename/content
	Family string `json:"family"` // Artifact family (proofs, bench, smoke90, etc.)

	// Timestamps
	Timestamp  time.Time `json:"timestamp"`   // Artifact creation time
	CreatedAt  time.Time `json:"created_at"`  // File creation time
	ModifiedAt time.Time `json:"modified_at"` // File last modified time
	ScannedAt  time.Time `json:"scanned_at"`  // When this entry was indexed

	// Status
	PassFail   string `json:"pass_fail"`    // "pass", "fail", or "unknown"
	IsLastPass bool   `json:"is_last_pass"` // Most recent PASS for this family
	IsLastRun  bool   `json:"is_last_run"`  // Most recent run for this family
	IsPinned   bool   `json:"is_pinned"`    // Protected from GC

	// File information
	Paths          []string `json:"paths"`           // All file paths for this artifact
	TotalBytes     int64    `json:"total_bytes"`     // Sum of all file sizes
	ChecksumSHA256 string   `json:"checksum_sha256"` // Content checksum

	// Metadata
	Tags        []string `json:"tags,omitempty"`        // Optional tags
	Description string   `json:"description,omitempty"` // Optional description
	Version     string   `json:"version,omitempty"`     // Version/build info
}

// Manifest represents the complete artifact index
type Manifest struct {
	// Metadata
	Version     string      `json:"version"`      // Manifest format version
	GeneratedAt time.Time   `json:"generated_at"` // When manifest was created
	Scanner     ScannerInfo `json:"scanner"`      // Scanner configuration used

	// Content
	Entries  []ArtifactEntry `json:"entries"`  // All artifact entries
	Families map[string]int  `json:"families"` // Count by family
	Summary  ManifestSummary `json:"summary"`  // Quick stats

	// Index for fast lookups (not serialized)
	ByID     map[string]*ArtifactEntry   `json:"-"`
	ByFamily map[string][]*ArtifactEntry `json:"-"`
	ByRunID  map[string]*ArtifactEntry   `json:"-"`
}

// ScannerInfo captures the configuration used to generate the manifest
type ScannerInfo struct {
	Version      string              `json:"version"`       // Scanner version
	ConfigHash   string              `json:"config_hash"`   // Hash of configuration
	ScanPaths    []string            `json:"scan_paths"`    // Paths scanned
	Patterns     map[string][]string `json:"patterns"`      // Family patterns used
	WorkerCount  int                 `json:"worker_count"`  // Parallel workers used
	ScanDuration time.Duration       `json:"scan_duration"` // Time taken to scan
}

// ManifestSummary provides quick statistics
type ManifestSummary struct {
	TotalEntries int            `json:"total_entries"`
	TotalBytes   int64          `json:"total_bytes"`
	TotalFiles   int            `json:"total_files"`
	FamilyCounts map[string]int `json:"family_counts"`
	PinnedCount  int            `json:"pinned_count"`
	PassCount    int            `json:"pass_count"`
	FailCount    int            `json:"fail_count"`
	OldestEntry  time.Time      `json:"oldest_entry"`
	NewestEntry  time.Time      `json:"newest_entry"`
}

// NewManifest creates a new empty manifest
func NewManifest() *Manifest {
	return &Manifest{
		Version:     "1.0",
		GeneratedAt: time.Now(),
		Entries:     make([]ArtifactEntry, 0),
		Families:    make(map[string]int),
		ByID:        make(map[string]*ArtifactEntry),
		ByFamily:    make(map[string][]*ArtifactEntry),
		ByRunID:     make(map[string]*ArtifactEntry),
	}
}

// AddEntry adds an artifact entry to the manifest
func (m *Manifest) AddEntry(entry ArtifactEntry) {
	// Generate ID if not provided
	if entry.ID == "" {
		entry.ID = m.generateID(entry)
	}

	// Add to main collection
	m.Entries = append(m.Entries, entry)

	// Update indices
	m.ByID[entry.ID] = &m.Entries[len(m.Entries)-1]

	if _, exists := m.ByFamily[entry.Family]; !exists {
		m.ByFamily[entry.Family] = make([]*ArtifactEntry, 0)
	}
	m.ByFamily[entry.Family] = append(m.ByFamily[entry.Family], &m.Entries[len(m.Entries)-1])

	if entry.RunID != "" {
		m.ByRunID[entry.RunID] = &m.Entries[len(m.Entries)-1]
	}

	// Update counters
	m.Families[entry.Family]++
}

// generateID creates a unique identifier for an artifact entry
func (m *Manifest) generateID(entry ArtifactEntry) string {
	// Use family, timestamp, and first path to generate stable ID
	data := fmt.Sprintf("%s:%d", entry.Family, entry.Timestamp.Unix())
	if len(entry.Paths) > 0 {
		data += ":" + entry.Paths[0]
	}

	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)[:12] // Use first 12 chars
}

// BuildIndices rebuilds the lookup indices from entries
func (m *Manifest) BuildIndices() {
	m.ByID = make(map[string]*ArtifactEntry)
	m.ByFamily = make(map[string][]*ArtifactEntry)
	m.ByRunID = make(map[string]*ArtifactEntry)

	for i := range m.Entries {
		entry := &m.Entries[i]

		// By ID
		m.ByID[entry.ID] = entry

		// By Family
		if _, exists := m.ByFamily[entry.Family]; !exists {
			m.ByFamily[entry.Family] = make([]*ArtifactEntry, 0)
		}
		m.ByFamily[entry.Family] = append(m.ByFamily[entry.Family], entry)

		// By RunID
		if entry.RunID != "" {
			m.ByRunID[entry.RunID] = entry
		}
	}
}

// UpdateSummary recalculates the manifest summary statistics
func (m *Manifest) UpdateSummary() {
	summary := ManifestSummary{
		FamilyCounts: make(map[string]int),
	}

	for _, entry := range m.Entries {
		summary.TotalEntries++
		summary.TotalBytes += entry.TotalBytes
		summary.TotalFiles += len(entry.Paths)
		summary.FamilyCounts[entry.Family]++

		if entry.IsPinned {
			summary.PinnedCount++
		}

		switch entry.PassFail {
		case "pass":
			summary.PassCount++
		case "fail":
			summary.FailCount++
		}

		// Track oldest/newest
		if summary.OldestEntry.IsZero() || entry.Timestamp.Before(summary.OldestEntry) {
			summary.OldestEntry = entry.Timestamp
		}
		if summary.NewestEntry.IsZero() || entry.Timestamp.After(summary.NewestEntry) {
			summary.NewestEntry = entry.Timestamp
		}
	}

	m.Summary = summary
}

// GetByFamily returns all entries for a given family, sorted by timestamp (newest first)
func (m *Manifest) GetByFamily(family string) []*ArtifactEntry {
	entries, exists := m.ByFamily[family]
	if !exists {
		return nil
	}

	// Return copy to prevent modification
	result := make([]*ArtifactEntry, len(entries))
	copy(result, entries)

	// Sort by timestamp descending (newest first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Timestamp.Before(result[j].Timestamp) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// GetByID returns an entry by its ID
func (m *Manifest) GetByID(id string) *ArtifactEntry {
	return m.ByID[id]
}

// SetPinned sets the pinned status for an entry
func (m *Manifest) SetPinned(id string, pinned bool) error {
	entry := m.GetByID(id)
	if entry == nil {
		return fmt.Errorf("entry not found: %s", id)
	}

	entry.IsPinned = pinned
	return nil
}

// MarkLastRuns identifies and marks the most recent run and pass for each family
func (m *Manifest) MarkLastRuns() {
	// Reset all flags first
	for i := range m.Entries {
		m.Entries[i].IsLastRun = false
		m.Entries[i].IsLastPass = false
	}

	// Find last run and last pass for each family
	for family, entries := range m.ByFamily {
		if len(entries) == 0 {
			continue
		}

		// Sort by timestamp descending
		sortedEntries := m.GetByFamily(family)

		// Mark most recent run
		if len(sortedEntries) > 0 {
			sortedEntries[0].IsLastRun = true
		}

		// Mark most recent pass
		for _, entry := range sortedEntries {
			if entry.PassFail == "pass" {
				entry.IsLastPass = true
				break
			}
		}
	}
}
