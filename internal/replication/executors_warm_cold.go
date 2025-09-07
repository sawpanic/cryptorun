package replication

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// WarmColdExecutor handles file-based replication for warm and cold tiers
type WarmColdExecutor struct {
	config      WarmColdExecutorConfig
	transfers   map[string]*TransferState
	metrics     *WarmColdMetrics
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// WarmColdExecutorConfig holds configuration for warm/cold tier replication
type WarmColdExecutorConfig struct {
	BasePath          string        `json:"base_path"`
	ChunkSize         int64         `json:"chunk_size"`         // For large file transfers
	MaxConcurrent     int           `json:"max_concurrent"`     // Concurrent transfers
	RetryDelay        time.Duration `json:"retry_delay"`
	IntegrityCheck    bool          `json:"integrity_check"`    // Verify checksums
	ResumePartials    bool          `json:"resume_partials"`    // Resume interrupted transfers
	CompressionLevel  int           `json:"compression_level"`  // 0-9, 0=none
	TempSuffix        string        `json:"temp_suffix"`        // Suffix for temp files
}

// TransferState tracks the state of a file transfer
type TransferState struct {
	ID            string    `json:"id"`
	SourcePath    string    `json:"source_path"`
	DestPath      string    `json:"dest_path"`
	TotalSize     int64     `json:"total_size"`
	TransferredSize int64   `json:"transferred_size"`
	Checksum      string    `json:"checksum"`
	StartTime     time.Time `json:"start_time"`
	Status        TransferStatus `json:"status"`
	Error         string    `json:"error,omitempty"`
	RetryCount    int       `json:"retry_count"`
}

// TransferStatus represents the status of a file transfer
type TransferStatus string

const (
	TransferPending    TransferStatus = "pending"
	TransferInProgress TransferStatus = "in_progress"
	TransferCompleted  TransferStatus = "completed"
	TransferFailed     TransferStatus = "failed"
	TransferPaused     TransferStatus = "paused"
)

// WarmColdMetrics tracks warm/cold tier replication metrics
type WarmColdMetrics struct {
	FilesTransferred     int64
	BytesTransferred     int64
	TransferErrors       int64
	IntegrityFailures    int64
	AverageTransferRate  float64 // MB/s
	CurrentTransfers     int64
	QueuedTransfers      int64
	mu                   sync.RWMutex
}

// FileInfo contains metadata about a file to be replicated
type FileInfo struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	Checksum     string    `json:"checksum"`
	Permissions  os.FileMode `json:"permissions"`
	IsDirectory  bool      `json:"is_directory"`
}

// NewWarmColdExecutor creates a new warm/cold tier executor
func NewWarmColdExecutor(config WarmColdExecutorConfig) *WarmColdExecutor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &WarmColdExecutor{
		config:    config,
		transfers: make(map[string]*TransferState),
		metrics:   &WarmColdMetrics{},
		ctx:       ctx,
		cancel:    cancel,
	}
}

// ExecuteStep executes a warm or cold tier replication step
func (w *WarmColdExecutor) ExecuteStep(ctx context.Context, step Step) error {
	if step.Tier != TierWarm && step.Tier != TierCold {
		return fmt.Errorf("warm/cold executor can only handle warm and cold tier steps")
	}
	
	log.Printf("Executing %s tier replication step %s: %s -> %s", 
		step.Tier, step.ID, step.From, step.To)
	
	// Build file list for the time window
	files, err := w.discoverFiles(step.Tier, step.From, step.Window)
	if err != nil {
		return fmt.Errorf("failed to discover files: %w", err)
	}
	
	if len(files) == 0 {
		log.Printf("No files found for replication in step %s", step.ID)
		return nil
	}
	
	log.Printf("Found %d files to replicate for step %s", len(files), step.ID)
	
	// Execute file transfers
	return w.replicateFiles(ctx, step, files)
}

// discoverFiles finds files that need to be replicated for the given time window
func (w *WarmColdExecutor) discoverFiles(tier Tier, region Region, window TimeRange) ([]FileInfo, error) {
	basePath := w.getRegionBasePath(tier, region)
	var files []FileInfo
	
	// Walk directory tree to find relevant files
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error walking path %s: %v", path, err)
			return nil // Continue walking, don't fail entire operation
		}
		
		// Skip directories for now (could be enhanced to replicate directory structure)
		if info.IsDir() {
			return nil
		}
		
		// Check if file is relevant to the time window
		if w.isFileRelevantToWindow(path, info, window) {
			checksum, err := w.calculateChecksum(path)
			if err != nil {
				log.Printf("Warning: failed to calculate checksum for %s: %v", path, err)
				checksum = "" // Continue without checksum
			}
			
			files = append(files, FileInfo{
				Path:        path,
				Size:        info.Size(),
				ModTime:     info.ModTime(),
				Checksum:    checksum,
				Permissions: info.Mode(),
				IsDirectory: info.IsDir(),
			})
		}
		
		return nil
	})
	
	return files, err
}

// isFileRelevantToWindow determines if a file is relevant to the replication window
func (w *WarmColdExecutor) isFileRelevantToWindow(path string, info os.FileInfo, window TimeRange) bool {
	// Strategy 1: Use file modification time
	if !info.ModTime().Before(window.From) && info.ModTime().Before(window.To) {
		return true
	}
	
	// Strategy 2: Parse time from filename for time-partitioned files
	if w.isTimePartitionedFile(path) {
		if fileTime, err := w.parseTimeFromFilename(path); err == nil {
			return !fileTime.Before(window.From) && fileTime.Before(window.To)
		}
	}
	
	// Strategy 3: For recent files, always include (catch-up replication)
	if time.Since(info.ModTime()) < 24*time.Hour {
		return true
	}
	
	return false
}

// isTimePartitionedFile checks if a file follows time partitioning naming convention
func (w *WarmColdExecutor) isTimePartitionedFile(path string) bool {
	filename := filepath.Base(path)
	
	// Look for patterns like: data_2025-09-07_14.parquet, trades_20250907.csv
	timePatterns := []string{
		"2025-", "2024-", "2026-", // ISO date patterns
		"202509", "202508",        // Compact date patterns
	}
	
	for _, pattern := range timePatterns {
		if strings.Contains(filename, pattern) {
			return true
		}
	}
	
	return false
}

// parseTimeFromFilename attempts to extract timestamp from filename
func (w *WarmColdExecutor) parseTimeFromFilename(path string) (time.Time, error) {
	filename := filepath.Base(path)
	
	// Common timestamp formats in filenames
	formats := []string{
		"2006-01-02_15",         // data_2025-09-07_14.parquet
		"2006-01-02",            // data_2025-09-07.parquet
		"20060102_15",           // data_20250907_14.parquet
		"20060102",              // data_20250907.parquet
		"2006_01_02_15_04_05",   // data_2025_09_07_14_30_00.parquet
	}
	
	// Extract potential timestamp strings from filename
	for _, format := range formats {
		// Try to find timestamp pattern in filename
		if strings.Contains(filename, "2025") || strings.Contains(filename, "2024") || strings.Contains(filename, "2026") {
			// Simple extraction - in production would use regex
			for i := 0; i < len(filename)-len(format); i++ {
				substr := filename[i : i+len(format)]
				if t, err := time.Parse(format, substr); err == nil {
					return t, nil
				}
			}
		}
	}
	
	return time.Time{}, fmt.Errorf("no timestamp found in filename")
}

// replicateFiles performs the actual file replication
func (w *WarmColdExecutor) replicateFiles(ctx context.Context, step Step, files []FileInfo) error {
	// Create destination directory
	destBasePath := w.getRegionBasePath(step.Tier, step.To)
	if err := os.MkdirAll(destBasePath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	
	// Set up semaphore for concurrent transfers
	semaphore := make(chan struct{}, w.config.MaxConcurrent)
	var wg sync.WaitGroup
	var firstError error
	var errorMu sync.Mutex
	
	// Process files in batches
	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case semaphore <- struct{}{}: // Acquire semaphore
		}
		
		wg.Add(1)
		go func(f FileInfo) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore
			
			if err := w.replicateFile(ctx, step, f); err != nil {
				log.Printf("Failed to replicate file %s: %v", f.Path, err)
				
				errorMu.Lock()
				if firstError == nil {
					firstError = err
				}
				errorMu.Unlock()
				
				w.updateMetrics(func(m *WarmColdMetrics) {
					m.TransferErrors++
				})
			} else {
				w.updateMetrics(func(m *WarmColdMetrics) {
					m.FilesTransferred++
					m.BytesTransferred += f.Size
				})
			}
		}(file)
	}
	
	// Wait for all transfers to complete
	wg.Wait()
	
	return firstError
}

// replicateFile replicates a single file
func (w *WarmColdExecutor) replicateFile(ctx context.Context, step Step, file FileInfo) error {
	// Calculate destination path
	sourcePath := file.Path
	sourceBase := w.getRegionBasePath(step.Tier, step.From)
	destBase := w.getRegionBasePath(step.Tier, step.To)
	
	relativePath, err := filepath.Rel(sourceBase, sourcePath)
	if err != nil {
		return fmt.Errorf("failed to calculate relative path: %w", err)
	}
	
	destPath := filepath.Join(destBase, relativePath)
	
	// Create transfer state
	transferID := fmt.Sprintf("%s-%s", step.ID, filepath.Base(sourcePath))
	transfer := &TransferState{
		ID:          transferID,
		SourcePath:  sourcePath,
		DestPath:    destPath,
		TotalSize:   file.Size,
		StartTime:   time.Now(),
		Status:      TransferPending,
	}
	
	w.mu.Lock()
	w.transfers[transferID] = transfer
	w.mu.Unlock()
	
	// Update metrics
	w.updateMetrics(func(m *WarmColdMetrics) {
		m.QueuedTransfers++
	})
	
	defer func() {
		w.updateMetrics(func(m *WarmColdMetrics) {
			m.QueuedTransfers--
		})
	}()
	
	// Check if destination already exists and is up-to-date
	if w.isFileUpToDate(sourcePath, destPath, file.Checksum) {
		log.Printf("File %s is already up-to-date, skipping", relativePath)
		transfer.Status = TransferCompleted
		return nil
	}
	
	// Execute the transfer with retries
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		transfer.Status = TransferInProgress
		transfer.RetryCount = attempt
		
		w.updateMetrics(func(m *WarmColdMetrics) {
			m.CurrentTransfers++
		})
		
		err := w.performFileTransfer(ctx, transfer, file)
		
		w.updateMetrics(func(m *WarmColdMetrics) {
			m.CurrentTransfers--
		})
		
		if err == nil {
			transfer.Status = TransferCompleted
			return nil
		}
		
		transfer.Error = err.Error()
		transfer.Status = TransferFailed
		
		if attempt < maxRetries-1 {
			log.Printf("Transfer attempt %d failed for %s, retrying: %v", attempt+1, relativePath, err)
			
			// Exponential backoff
			delay := w.config.RetryDelay * time.Duration(1<<uint(attempt))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	
	return fmt.Errorf("failed to transfer file after %d attempts: %s", maxRetries, transfer.Error)
}

// performFileTransfer performs the actual file transfer
func (w *WarmColdExecutor) performFileTransfer(ctx context.Context, transfer *TransferState, file FileInfo) error {
	// Create destination directory
	destDir := filepath.Dir(transfer.DestPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	
	// Open source file
	sourceFile, err := os.Open(transfer.SourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()
	
	// Create temporary destination file
	tempDestPath := transfer.DestPath + w.config.TempSuffix
	destFile, err := os.Create(tempDestPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()
	
	// Copy file with progress tracking
	var transferred int64
	buffer := make([]byte, w.config.ChunkSize)
	
	for {
		select {
		case <-ctx.Done():
			os.Remove(tempDestPath) // Clean up temp file
			return ctx.Err()
		default:
		}
		
		n, err := sourceFile.Read(buffer)
		if n > 0 {
			if _, writeErr := destFile.Write(buffer[:n]); writeErr != nil {
				os.Remove(tempDestPath)
				return fmt.Errorf("failed to write to destination: %w", writeErr)
			}
			
			transferred += int64(n)
			transfer.TransferredSize = transferred
			
			// Calculate transfer rate for metrics
			elapsed := time.Since(transfer.StartTime).Seconds()
			if elapsed > 0 {
				rate := float64(transferred) / elapsed / (1024 * 1024) // MB/s
				w.updateMetrics(func(m *WarmColdMetrics) {
					m.AverageTransferRate = rate
				})
			}
		}
		
		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(tempDestPath)
			return fmt.Errorf("failed to read from source: %w", err)
		}
	}
	
	// Verify file size
	if transferred != file.Size {
		os.Remove(tempDestPath)
		return fmt.Errorf("transferred size mismatch: expected %d, got %d", file.Size, transferred)
	}
	
	// Verify integrity if enabled
	if w.config.IntegrityCheck && file.Checksum != "" {
		destChecksum, err := w.calculateChecksum(tempDestPath)
		if err != nil {
			os.Remove(tempDestPath)
			return fmt.Errorf("failed to calculate destination checksum: %w", err)
		}
		
		if destChecksum != file.Checksum {
			os.Remove(tempDestPath)
			w.updateMetrics(func(m *WarmColdMetrics) {
				m.IntegrityFailures++
			})
			return fmt.Errorf("integrity check failed: expected %s, got %s", file.Checksum, destChecksum)
		}
	}
	
	// Set file permissions
	if err := os.Chmod(tempDestPath, file.Permissions); err != nil {
		log.Printf("Warning: failed to set file permissions: %v", err)
	}
	
	// Atomic move to final destination
	if err := os.Rename(tempDestPath, transfer.DestPath); err != nil {
		os.Remove(tempDestPath)
		return fmt.Errorf("failed to move file to final destination: %w", err)
	}
	
	log.Printf("Successfully replicated file %s (%d bytes)", 
		filepath.Base(transfer.SourcePath), transferred)
	
	return nil
}

// isFileUpToDate checks if the destination file is already up-to-date
func (w *WarmColdExecutor) isFileUpToDate(sourcePath, destPath, expectedChecksum string) bool {
	destInfo, err := os.Stat(destPath)
	if err != nil {
		return false // File doesn't exist
	}
	
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false // Source file doesn't exist
	}
	
	// Quick check: different sizes
	if destInfo.Size() != sourceInfo.Size() {
		return false
	}
	
	// Quick check: destination is older
	if destInfo.ModTime().Before(sourceInfo.ModTime()) {
		return false
	}
	
	// Thorough check: compare checksums if available
	if expectedChecksum != "" && w.config.IntegrityCheck {
		destChecksum, err := w.calculateChecksum(destPath)
		if err != nil {
			return false
		}
		return destChecksum == expectedChecksum
	}
	
	// Assume up-to-date if size and mod time match
	return true
}

// calculateChecksum calculates SHA256 checksum of a file
func (w *WarmColdExecutor) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
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

// getRegionBasePath returns the base path for a region and tier
func (w *WarmColdExecutor) getRegionBasePath(tier Tier, region Region) string {
	return filepath.Join(w.config.BasePath, string(tier), string(region))
}

// updateMetrics safely updates executor metrics
func (w *WarmColdExecutor) updateMetrics(fn func(*WarmColdMetrics)) {
	w.metrics.mu.Lock()
	defer w.metrics.mu.Unlock()
	fn(w.metrics)
}

// GetMetrics returns a copy of current metrics
func (w *WarmColdExecutor) GetMetrics() WarmColdMetrics {
	w.metrics.mu.RLock()
	defer w.metrics.mu.RUnlock()
	return *w.metrics
}

// GetTransferState returns the current state of all transfers
func (w *WarmColdExecutor) GetTransferState() map[string]*TransferState {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	result := make(map[string]*TransferState)
	for id, state := range w.transfers {
		// Create a copy to prevent external modification
		stateCopy := *state
		result[id] = &stateCopy
	}
	
	return result
}

// Stop gracefully shuts down the warm/cold executor
func (w *WarmColdExecutor) Stop() error {
	log.Println("Stopping warm/cold tier executor...")
	
	w.cancel()
	
	// Wait for active transfers to complete (with timeout)
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for transfers to complete")
		case <-ticker.C:
			metrics := w.GetMetrics()
			if metrics.CurrentTransfers == 0 {
				log.Println("Warm/cold tier executor stopped successfully")
				return nil
			}
			log.Printf("Waiting for %d active transfers to complete", metrics.CurrentTransfers)
		}
	}
}