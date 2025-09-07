package gc

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sawpanic/cryptorun/internal/artifacts/manifest"
)

// Executor applies garbage collection plans safely
type Executor struct {
	trashDir        string
	backupEnabled   bool
	checksumEnabled bool
}

// NewExecutor creates a new GC executor
func NewExecutor(trashDir string, backupEnabled bool) *Executor {
	return &Executor{
		trashDir:        trashDir,
		backupEnabled:   backupEnabled,
		checksumEnabled: true,
	}
}

// ApplyResult contains the results of applying a GC plan
type ApplyResult struct {
	Plan      *Plan         `json:"plan"`
	Success   bool          `json:"success"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`

	// Execution details
	FilesDeleted      int   `json:"files_deleted"`
	BytesDeleted      int64 `json:"bytes_deleted"`
	FilesMovedToTrash int   `json:"files_moved_to_trash"`

	// Errors and warnings
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`

	// Per-family results
	FamilyResults map[string]FamilyResult `json:"family_results"`
}

// FamilyResult contains results for a specific family
type FamilyResult struct {
	Family       string   `json:"family"`
	Planned      int      `json:"planned_deletions"`
	Executed     int      `json:"executed_deletions"`
	Failed       int      `json:"failed_deletions"`
	BytesDeleted int64    `json:"bytes_deleted"`
	FilesDeleted int      `json:"files_deleted"`
	Errors       []string `json:"errors,omitempty"`
}

// Apply executes a GC plan with safety checks and atomic operations
func (e *Executor) Apply(plan *Plan, manifest *manifest.Manifest) (*ApplyResult, error) {
	if plan.DryRun {
		return e.simulateApply(plan, manifest)
	}

	result := &ApplyResult{
		Plan:          plan,
		StartTime:     time.Now(),
		FamilyResults: make(map[string]FamilyResult),
	}

	// Prepare trash directory
	if err := e.prepareTrashDir(); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to prepare trash directory: %v", err))
		return result, err
	}

	// Apply plan family by family
	for family, familyPlan := range plan.FamilyPlans {
		familyResult := e.applyFamilyPlan(familyPlan, manifest)
		result.FamilyResults[family] = familyResult

		// Aggregate results
		result.FilesDeleted += familyResult.FilesDeleted
		result.BytesDeleted += familyResult.BytesDeleted
		result.Errors = append(result.Errors, familyResult.Errors...)

		// Track failures
		if familyResult.Failed > 0 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Family %s: %d deletions failed", family, familyResult.Failed))
		}
	}

	// Finalize results
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Success = len(result.Errors) == 0

	// Write GC report
	if err := e.writeGCReport(result); err != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("failed to write GC report: %v", err))
	}

	return result, nil
}

// simulateApply simulates plan execution for dry runs
func (e *Executor) simulateApply(plan *Plan, manifest *manifest.Manifest) (*ApplyResult, error) {
	result := &ApplyResult{
		Plan:          plan,
		Success:       true,
		StartTime:     time.Now(),
		FamilyResults: make(map[string]FamilyResult),
	}

	// Simulate family processing
	for family, familyPlan := range plan.FamilyPlans {
		familyResult := FamilyResult{
			Family:       family,
			Planned:      len(familyPlan.ToDelete),
			Executed:     len(familyPlan.ToDelete), // Would be executed
			Failed:       0,
			BytesDeleted: familyPlan.BytesToDelete,
			FilesDeleted: familyPlan.FilesToDelete,
		}

		result.FamilyResults[family] = familyResult
		result.FilesDeleted += familyResult.FilesDeleted
		result.BytesDeleted += familyResult.BytesDeleted
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// applyFamilyPlan applies deletions for a specific family
func (e *Executor) applyFamilyPlan(familyPlan FamilyPlan, manifest *manifest.Manifest) FamilyResult {
	result := FamilyResult{
		Family:  familyPlan.Family,
		Planned: len(familyPlan.ToDelete),
		Errors:  make([]string, 0),
	}

	// Process each entry marked for deletion
	for _, entryID := range familyPlan.ToDelete {
		entry := manifest.GetByID(entryID)
		if entry == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("entry not found: %s", entryID))
			result.Failed++
			continue
		}

		// Delete entry files
		if err := e.deleteEntry(entry); err != nil {
			result.Errors = append(result.Errors,
				fmt.Sprintf("failed to delete entry %s: %v", entryID, err))
			result.Failed++
			continue
		}

		// Track successful deletion
		result.Executed++
		result.BytesDeleted += entry.TotalBytes
		result.FilesDeleted += len(entry.Paths)
	}

	return result
}

// deleteEntry safely deletes all files for an entry
func (e *Executor) deleteEntry(entry *manifest.ArtifactEntry) error {
	// First, verify all files exist and get checksums if enabled
	fileChecksums := make(map[string]string)

	for _, path := range entry.Paths {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue // File already gone, not an error
			}
			return fmt.Errorf("failed to stat file %s: %w", path, err)
		}

		// Compute checksum for verification
		if e.checksumEnabled {
			checksum, err := e.computeFileChecksum(path)
			if err != nil {
				return fmt.Errorf("failed to compute checksum for %s: %w", path, err)
			}
			fileChecksums[path] = checksum
		}
	}

	// Move files to trash (atomic operation per file)
	trashPaths := make([]string, 0, len(entry.Paths))

	for _, path := range entry.Paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue // Skip files that don't exist
		}

		// Generate trash path
		trashPath, err := e.generateTrashPath(path, entry.ID)
		if err != nil {
			// Try to clean up already moved files
			e.cleanupTrashFiles(trashPaths)
			return fmt.Errorf("failed to generate trash path for %s: %w", path, err)
		}

		// Move to trash
		if err := e.moveToTrash(path, trashPath); err != nil {
			// Try to clean up already moved files
			e.cleanupTrashFiles(trashPaths)
			return fmt.Errorf("failed to move %s to trash: %w", path, err)
		}

		trashPaths = append(trashPaths, trashPath)
	}

	// Verify checksums if enabled
	if e.checksumEnabled {
		if err := e.verifyTrashChecksums(trashPaths, fileChecksums); err != nil {
			// Try to restore files from trash
			e.restoreFromTrash(trashPaths, entry.Paths)
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Final deletion from trash (optional - files can stay in trash)
	// This allows for recovery if needed

	return nil
}

// moveToTrash atomically moves a file to the trash directory
func (e *Executor) moveToTrash(sourcePath, trashPath string) error {
	// Ensure trash directory exists
	trashDir := filepath.Dir(trashPath)
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return fmt.Errorf("failed to create trash directory: %w", err)
	}

	// Use atomic move
	return os.Rename(sourcePath, trashPath)
}

// generateTrashPath creates a unique path in the trash directory
func (e *Executor) generateTrashPath(originalPath, entryID string) (string, error) {
	// Create a safe filename
	timestamp := time.Now().Format("20060102_150405")

	// Create subdirectory structure in trash
	relPath, err := filepath.Rel(".", originalPath)
	if err != nil {
		relPath = strings.ReplaceAll(originalPath, string(os.PathSeparator), "_")
	}

	// Replace path separators with underscores for safety
	safeRelPath := strings.ReplaceAll(relPath, string(os.PathSeparator), "_")

	trashPath := filepath.Join(e.trashDir, entryID, timestamp, safeRelPath)

	return trashPath, nil
}

// computeFileChecksum computes SHA256 checksum of a file
func (e *Executor) computeFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// verifyTrashChecksums verifies that files in trash match original checksums
func (e *Executor) verifyTrashChecksums(trashPaths []string, originalChecksums map[string]string) error {
	for i, trashPath := range trashPaths {
		// Find corresponding original path
		var originalPath string
		for origPath := range originalChecksums {
			if strings.Contains(trashPath, filepath.Base(origPath)) {
				originalPath = origPath
				break
			}
		}

		if originalPath == "" {
			continue // Skip verification if can't match
		}

		// Compute trash file checksum
		trashChecksum, err := e.computeFileChecksum(trashPath)
		if err != nil {
			return fmt.Errorf("failed to compute trash checksum for %s: %w", trashPath, err)
		}

		// Compare checksums
		if trashChecksum != originalChecksums[originalPath] {
			return fmt.Errorf("checksum mismatch for %s: expected %s, got %s",
				originalPath, originalChecksums[originalPath], trashChecksum)
		}

		_ = i // Suppress unused variable
	}

	return nil
}

// cleanupTrashFiles removes files from trash (cleanup after error)
func (e *Executor) cleanupTrashFiles(trashPaths []string) {
	for _, trashPath := range trashPaths {
		os.Remove(trashPath) // Best effort cleanup
	}
}

// restoreFromTrash attempts to restore files from trash to original locations
func (e *Executor) restoreFromTrash(trashPaths, originalPaths []string) {
	for i, trashPath := range trashPaths {
		if i < len(originalPaths) {
			os.Rename(trashPath, originalPaths[i]) // Best effort restore
		}
	}
}

// prepareTrashDir ensures the trash directory exists and is accessible
func (e *Executor) prepareTrashDir() error {
	if err := os.MkdirAll(e.trashDir, 0755); err != nil {
		return fmt.Errorf("failed to create trash directory: %w", err)
	}

	// Test write access
	testFile := filepath.Join(e.trashDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("trash directory not writable: %w", err)
	}
	os.Remove(testFile)

	return nil
}

// writeGCReport writes a detailed report of the GC operation
func (e *Executor) writeGCReport(result *ApplyResult) error {
	reportPath := filepath.Join(e.trashDir, fmt.Sprintf("gc_report_%s.md",
		result.StartTime.Format("20060102_150405")))

	file, err := os.Create(reportPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write report content
	fmt.Fprintf(file, "# Garbage Collection Report\n\n")
	fmt.Fprintf(file, "**Generated:** %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "**Duration:** %v\n", result.Duration)
	fmt.Fprintf(file, "**Success:** %t\n\n", result.Success)

	fmt.Fprintf(file, "## Summary\n\n")
	fmt.Fprintf(file, "- Files Deleted: %d\n", result.FilesDeleted)
	fmt.Fprintf(file, "- Bytes Deleted: %s\n", formatBytes(result.BytesDeleted))
	fmt.Fprintf(file, "- Errors: %d\n", len(result.Errors))
	fmt.Fprintf(file, "- Warnings: %d\n\n", len(result.Warnings))

	if len(result.Errors) > 0 {
		fmt.Fprintf(file, "## Errors\n\n")
		for _, err := range result.Errors {
			fmt.Fprintf(file, "- %s\n", err)
		}
		fmt.Fprintf(file, "\n")
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintf(file, "## Warnings\n\n")
		for _, warning := range result.Warnings {
			fmt.Fprintf(file, "- %s\n", warning)
		}
		fmt.Fprintf(file, "\n")
	}

	fmt.Fprintf(file, "## Family Results\n\n")
	for family, familyResult := range result.FamilyResults {
		fmt.Fprintf(file, "### %s\n\n", family)
		fmt.Fprintf(file, "- Planned: %d\n", familyResult.Planned)
		fmt.Fprintf(file, "- Executed: %d\n", familyResult.Executed)
		fmt.Fprintf(file, "- Failed: %d\n", familyResult.Failed)
		fmt.Fprintf(file, "- Bytes Deleted: %s\n", formatBytes(familyResult.BytesDeleted))
		fmt.Fprintf(file, "- Files Deleted: %d\n\n", familyResult.FilesDeleted)
	}

	return nil
}
