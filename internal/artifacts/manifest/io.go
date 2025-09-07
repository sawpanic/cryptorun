package manifest

import (
	"encoding/json"
	"fmt"
	iolib "io"
	"os"
	"path/filepath"
	"time"
)

// IO handles reading and writing manifest files
type IO struct {
	manifestPath string
	backupPath   string
}

// NewIO creates a new manifest I/O handler
func NewIO(manifestPath string) *IO {
	return &IO{
		manifestPath: manifestPath,
		backupPath:   manifestPath + ".backup",
	}
}

// Load reads a manifest from disk
func (io *IO) Load() (*Manifest, error) {
	if _, err := os.Stat(io.manifestPath); os.IsNotExist(err) {
		return NewManifest(), nil // Return empty manifest if file doesn't exist
	}

	file, err := os.Open(io.manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest file: %w", err)
	}
	defer file.Close()

	var manifest Manifest
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	// Rebuild indices
	manifest.BuildIndices()

	return &manifest, nil
}

// Save writes a manifest to disk with atomic operation
func (io *IO) Save(manifest *Manifest) error {
	// Ensure directory exists
	dir := filepath.Dir(io.manifestPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}

	// Backup existing manifest if it exists
	if err := io.createBackup(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Write to temporary file first (atomic operation)
	tempPath := io.manifestPath + ".tmp"

	// Update generation timestamp
	manifest.GeneratedAt = time.Now()

	// Create temporary file
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempPath) // Clean up temp file on error
	}()

	// Write manifest as pretty JSON
	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return fmt.Errorf("failed to encode manifest: %w", err)
	}

	// Sync to disk
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close before rename
	tempFile.Close()

	// Atomically replace the manifest file
	if err := os.Rename(tempPath, io.manifestPath); err != nil {
		return fmt.Errorf("failed to replace manifest file: %w", err)
	}

	return nil
}

// createBackup creates a backup of the current manifest
func (io *IO) createBackup() error {
	if _, err := os.Stat(io.manifestPath); os.IsNotExist(err) {
		return nil // No existing file to backup
	}

	sourceFile, err := os.Open(io.manifestPath)
	if err != nil {
		return fmt.Errorf("failed to open source manifest: %w", err)
	}
	defer sourceFile.Close()

	backupFile, err := os.Create(io.backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer backupFile.Close()

	// Copy content
	if _, err := iolib.Copy(backupFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy to backup: %w", err)
	}

	return backupFile.Sync()
}

// RestoreFromBackup restores manifest from backup file
func (io *IO) RestoreFromBackup() error {
	if _, err := os.Stat(io.backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist: %s", io.backupPath)
	}

	// Copy backup to main manifest
	backupFile, err := os.Open(io.backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer backupFile.Close()

	manifestFile, err := os.Create(io.manifestPath)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer manifestFile.Close()

	if _, err := iolib.Copy(manifestFile, backupFile); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	return manifestFile.Sync()
}

// ExportToFile exports manifest to a different file (for reports, etc.)
func (io *IO) ExportToFile(manifest *Manifest, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return fmt.Errorf("failed to encode manifest for export: %w", err)
	}

	return file.Sync()
}

// Validate performs basic validation of a manifest file
func (io *IO) Validate(manifest *Manifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}

	if manifest.Version == "" {
		return fmt.Errorf("manifest version is empty")
	}

	if manifest.GeneratedAt.IsZero() {
		return fmt.Errorf("manifest generation timestamp is zero")
	}

	// Check for duplicate IDs
	idsSeen := make(map[string]bool)
	for _, entry := range manifest.Entries {
		if entry.ID == "" {
			return fmt.Errorf("entry has empty ID")
		}

		if idsSeen[entry.ID] {
			return fmt.Errorf("duplicate entry ID: %s", entry.ID)
		}
		idsSeen[entry.ID] = true

		// Validate entry fields
		if entry.Family == "" {
			return fmt.Errorf("entry %s has empty family", entry.ID)
		}

		if len(entry.Paths) == 0 {
			return fmt.Errorf("entry %s has no paths", entry.ID)
		}

		if entry.TotalBytes < 0 {
			return fmt.Errorf("entry %s has negative total bytes", entry.ID)
		}
	}

	return nil
}

// GetManifestInfo returns basic information about the manifest file
func (io *IO) GetManifestInfo() (*ManifestFileInfo, error) {
	stat, err := os.Stat(io.manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ManifestFileInfo{
				Exists: false,
				Path:   io.manifestPath,
			}, nil
		}
		return nil, fmt.Errorf("failed to stat manifest file: %w", err)
	}

	info := &ManifestFileInfo{
		Exists:  true,
		Path:    io.manifestPath,
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
	}

	// Check backup
	if backupStat, err := os.Stat(io.backupPath); err == nil {
		info.HasBackup = true
		info.BackupSize = backupStat.Size()
		info.BackupModTime = backupStat.ModTime()
	}

	return info, nil
}

// ManifestFileInfo provides information about manifest files on disk
type ManifestFileInfo struct {
	Exists        bool      `json:"exists"`
	Path          string    `json:"path"`
	Size          int64     `json:"size"`
	ModTime       time.Time `json:"mod_time"`
	HasBackup     bool      `json:"has_backup"`
	BackupSize    int64     `json:"backup_size,omitempty"`
	BackupModTime time.Time `json:"backup_mod_time,omitempty"`
}

// ScanAndSave performs a scan and saves the resulting manifest
func (io *IO) ScanAndSave(scanner *Scanner) (*ScanResult, error) {
	// Perform the scan
	result, err := scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	// Validate manifest before saving
	if err := io.Validate(result.Manifest); err != nil {
		return nil, fmt.Errorf("manifest validation failed: %w", err)
	}

	// Save the manifest
	if err := io.Save(result.Manifest); err != nil {
		return nil, fmt.Errorf("failed to save manifest: %w", err)
	}

	return result, nil
}

// LoadOrScan loads existing manifest or performs a new scan if none exists
func (io *IO) LoadOrScan(scanner *Scanner) (*Manifest, error) {
	// Try to load existing manifest
	manifest, err := io.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// If manifest is empty or very old, perform a new scan
	if len(manifest.Entries) == 0 || time.Since(manifest.GeneratedAt) > 24*time.Hour {
		result, err := io.ScanAndSave(scanner)
		if err != nil {
			return nil, fmt.Errorf("failed to scan and save: %w", err)
		}
		return result.Manifest, nil
	}

	return manifest, nil
}
