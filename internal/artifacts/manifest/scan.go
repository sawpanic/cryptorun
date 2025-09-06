package manifest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Scanner scans the filesystem to build an artifact manifest
type Scanner struct {
	config      *ScanConfig
	patterns    map[string][]*regexp.Regexp
	workerCount int
}

// ScanConfig configures the scanning behavior
type ScanConfig struct {
	RootPaths          []string            `yaml:"root_paths"`
	FamilyPatterns     map[string][]string `yaml:"family_patterns"`
	WorkerCount        int                 `yaml:"worker_count"`
	ChecksumBufferSize int                 `yaml:"checksum_buffer_size"`
	MaxFilesPerScan    int                 `yaml:"max_files_per_scan"`
	FollowSymlinks     bool                `yaml:"follow_symlinks"`
}

// ScanResult represents the result of a filesystem scan
type ScanResult struct {
	Manifest     *Manifest     `json:"manifest"`
	ScanDuration time.Duration `json:"scan_duration"`
	FilesScanned int           `json:"files_scanned"`
	BytesScanned int64         `json:"bytes_scanned"`
	ErrorCount   int           `json:"error_count"`
	Errors       []string      `json:"errors,omitempty"`
}

// FileInfo represents information about a file during scanning
type FileInfo struct {
	Path     string
	Size     int64
	ModTime  time.Time
	IsDir    bool
	Family   string
	RunID    string
	PassFail string
}

// NewScanner creates a new artifact scanner with the given configuration
func NewScanner(config *ScanConfig) *Scanner {
	scanner := &Scanner{
		config:      config,
		patterns:    make(map[string][]*regexp.Regexp),
		workerCount: config.WorkerCount,
	}

	// Compile patterns for each family
	for family, patterns := range config.FamilyPatterns {
		for _, pattern := range patterns {
			// Convert glob pattern to regex
			regexPattern := globToRegex(pattern)
			if compiled, err := regexp.Compile(regexPattern); err == nil {
				scanner.patterns[family] = append(scanner.patterns[family], compiled)
			}
		}
	}

	if scanner.workerCount <= 0 {
		scanner.workerCount = 4 // Default to 4 workers
	}

	return scanner
}

// Scan performs a complete scan of the configured paths
func (s *Scanner) Scan() (*ScanResult, error) {
	startTime := time.Now()

	result := &ScanResult{
		Manifest: NewManifest(),
		Errors:   make([]string, 0),
	}

	// Configure scanner info
	result.Manifest.Scanner = ScannerInfo{
		Version:      "1.0",
		ConfigHash:   s.computeConfigHash(),
		ScanPaths:    s.config.RootPaths,
		Patterns:     s.config.FamilyPatterns,
		WorkerCount:  s.workerCount,
		ScanDuration: 0, // Will be set at the end
	}

	// Find all files to scan
	fileInfos, err := s.findFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to find files: %w", err)
	}

	// Process files in parallel
	entries, errors := s.processFiles(fileInfos)

	// Add entries to manifest
	for _, entry := range entries {
		result.Manifest.AddEntry(entry)
	}

	// Update manifest metadata
	result.Manifest.MarkLastRuns()
	result.Manifest.UpdateSummary()
	result.Manifest.GeneratedAt = time.Now()

	// Set result statistics
	result.ScanDuration = time.Since(startTime)
	result.Manifest.Scanner.ScanDuration = result.ScanDuration
	result.FilesScanned = len(fileInfos)
	result.ErrorCount = len(errors)
	result.Errors = errors

	for _, entry := range result.Manifest.Entries {
		result.BytesScanned += entry.TotalBytes
	}

	return result, nil
}

// findFiles discovers all files matching the configured patterns
func (s *Scanner) findFiles() ([]FileInfo, error) {
	var allFiles []FileInfo

	for _, rootPath := range s.config.RootPaths {
		err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Continue walking, ignore errors
			}

			if info.IsDir() {
				return nil // Skip directories
			}

			// Check if file matches any family pattern
			family := s.matchFamily(path)
			if family == "" {
				return nil // Not an artifact file
			}

			// Extract metadata from path/filename
			runID := s.extractRunID(path)
			passFail := s.extractPassFail(path)

			fileInfo := FileInfo{
				Path:     path,
				Size:     info.Size(),
				ModTime:  info.ModTime(),
				IsDir:    false,
				Family:   family,
				RunID:    runID,
				PassFail: passFail,
			}

			allFiles = append(allFiles, fileInfo)

			// Check file limit
			if s.config.MaxFilesPerScan > 0 && len(allFiles) >= s.config.MaxFilesPerScan {
				return fmt.Errorf("max files limit reached: %d", s.config.MaxFilesPerScan)
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk path %s: %w", rootPath, err)
		}
	}

	return allFiles, nil
}

// processFiles processes file infos in parallel to create artifact entries
func (s *Scanner) processFiles(fileInfos []FileInfo) ([]ArtifactEntry, []string) {
	// Group files by potential artifact (same runID and family)
	artifactFiles := s.groupFilesByArtifact(fileInfos)

	// Set up parallel processing
	type processResult struct {
		entry ArtifactEntry
		err   error
	}

	jobs := make(chan []FileInfo, len(artifactFiles))
	results := make(chan processResult, len(artifactFiles))

	// Start workers
	for i := 0; i < s.workerCount; i++ {
		go func() {
			for files := range jobs {
				entry, err := s.createArtifactEntry(files)
				results <- processResult{entry: entry, err: err}
			}
		}()
	}

	// Send jobs
	for _, files := range artifactFiles {
		jobs <- files
	}
	close(jobs)

	// Collect results
	var entries []ArtifactEntry
	var errors []string

	for i := 0; i < len(artifactFiles); i++ {
		result := <-results
		if result.err != nil {
			errors = append(errors, result.err.Error())
		} else {
			entries = append(entries, result.entry)
		}
	}

	return entries, errors
}

// groupFilesByArtifact groups files that belong to the same artifact
func (s *Scanner) groupFilesByArtifact(fileInfos []FileInfo) [][]FileInfo {
	artifactMap := make(map[string][]FileInfo)

	for _, fileInfo := range fileInfos {
		// Create key from family and runID (or path if no runID)
		key := fileInfo.Family
		if fileInfo.RunID != "" {
			key += ":" + fileInfo.RunID
		} else {
			// Use directory path as grouping key
			key += ":" + filepath.Dir(fileInfo.Path)
		}

		artifactMap[key] = append(artifactMap[key], fileInfo)
	}

	// Convert map to slice
	var result [][]FileInfo
	for _, files := range artifactMap {
		result = append(result, files)
	}

	return result
}

// createArtifactEntry creates an artifact entry from a group of files
func (s *Scanner) createArtifactEntry(files []FileInfo) (ArtifactEntry, error) {
	if len(files) == 0 {
		return ArtifactEntry{}, fmt.Errorf("no files provided")
	}

	// Use first file as template
	first := files[0]

	entry := ArtifactEntry{
		Family:    first.Family,
		RunID:     first.RunID,
		PassFail:  first.PassFail,
		ScannedAt: time.Now(),
		Paths:     make([]string, 0, len(files)),
		Tags:      make([]string, 0),
	}

	// Process all files
	var totalBytes int64
	var oldestTime, newestTime time.Time
	var checksumData []byte

	for _, file := range files {
		entry.Paths = append(entry.Paths, file.Path)
		totalBytes += file.Size

		// Track timestamps
		if oldestTime.IsZero() || file.ModTime.Before(oldestTime) {
			oldestTime = file.ModTime
		}
		if newestTime.IsZero() || file.ModTime.After(newestTime) {
			newestTime = file.ModTime
		}

		// Add to checksum data
		checksumData = append(checksumData, []byte(file.Path+fmt.Sprintf("%d", file.Size))...)
	}

	// Set timestamps
	entry.CreatedAt = oldestTime
	entry.ModifiedAt = newestTime
	entry.Timestamp = s.extractTimestamp(entry.Paths[0], newestTime)
	entry.TotalBytes = totalBytes

	// Compute checksum
	hash := sha256.Sum256(checksumData)
	entry.ChecksumSHA256 = fmt.Sprintf("%x", hash)

	// Set default pass/fail if not determined
	if entry.PassFail == "" {
		entry.PassFail = "unknown"
	}

	return entry, nil
}

// matchFamily determines which family a file path belongs to
func (s *Scanner) matchFamily(path string) string {
	for family, patterns := range s.patterns {
		for _, pattern := range patterns {
			if pattern.MatchString(path) {
				return family
			}
		}
	}
	return ""
}

// extractRunID attempts to extract a run identifier from the file path
func (s *Scanner) extractRunID(path string) string {
	// Look for common run ID patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`run[_-](\d{8}[_-]\d{6})`), // run_20250906_143022
		regexp.MustCompile(`(\d{8}[_-]\d{6})`),        // 20250906_143022
		regexp.MustCompile(`run[_-]([a-f0-9]{8,})`),   // run_a1b2c3d4
		regexp.MustCompile(`([a-f0-9]{8}[a-f0-9]*)`),  // a1b2c3d4efgh5678
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(path); len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// extractPassFail attempts to determine pass/fail status from path or filename
func (s *Scanner) extractPassFail(path string) string {
	filename := strings.ToLower(filepath.Base(path))
	dirname := strings.ToLower(filepath.Dir(path))

	// Check for pass/fail indicators
	if strings.Contains(filename, "pass") || strings.Contains(dirname, "pass") {
		return "pass"
	}
	if strings.Contains(filename, "fail") || strings.Contains(dirname, "fail") {
		return "fail"
	}
	if strings.Contains(filename, "error") || strings.Contains(dirname, "error") {
		return "fail"
	}
	if strings.Contains(filename, "success") || strings.Contains(dirname, "success") {
		return "pass"
	}

	return "unknown"
}

// extractTimestamp attempts to extract timestamp from path, falls back to file mod time
func (s *Scanner) extractTimestamp(path string, fallback time.Time) time.Time {
	// Look for timestamp patterns in path
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(\d{8})[_-](\d{6})`),                       // 20250906_143022
		regexp.MustCompile(`(\d{4})[_-](\d{2})[_-](\d{2})[_-](\d{6})`), // 2025_09_06_143022
		regexp.MustCompile(`(\d{4})(\d{2})(\d{2})(\d{6})`),             // 20250906143022
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(path); len(matches) > 1 {
			// Try to parse the timestamp
			var timeStr string
			if len(matches) == 3 {
				timeStr = matches[1] + matches[2] // YYYYMMDD + HHMMSS
			} else if len(matches) == 5 {
				timeStr = matches[1] + matches[2] + matches[3] + matches[4] // YYYY MM DD HHMMSS
			}

			if parsed, err := time.Parse("20060102150405", timeStr); err == nil {
				return parsed
			}
		}
	}

	return fallback
}

// computeConfigHash creates a hash of the scanner configuration
func (s *Scanner) computeConfigHash() string {
	var data strings.Builder

	// Include paths and patterns in hash
	for _, path := range s.config.RootPaths {
		data.WriteString(path)
	}

	for family, patterns := range s.config.FamilyPatterns {
		data.WriteString(family)
		for _, pattern := range patterns {
			data.WriteString(pattern)
		}
	}

	data.WriteString(fmt.Sprintf("%d", s.config.WorkerCount))

	hash := sha256.Sum256([]byte(data.String()))
	return fmt.Sprintf("%x", hash)[:12]
}

// globToRegex converts a glob pattern to a regular expression
func globToRegex(glob string) string {
	// Escape special regex characters except * and ?
	escaped := regexp.QuoteMeta(glob)

	// Replace escaped glob patterns with regex equivalents
	escaped = strings.ReplaceAll(escaped, `\*\*`, `.*`)    // ** -> .*
	escaped = strings.ReplaceAll(escaped, `\*`, `[^/\\]*`) // * -> [^/\]*
	escaped = strings.ReplaceAll(escaped, `\?`, `.`)       // ? -> .

	// Anchor the pattern
	return `^` + escaped + `$`
}
