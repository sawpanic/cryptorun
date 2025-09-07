package application

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ShipConfig represents shipping configuration and policies
type ShipConfig struct {
	QualityPolicies    QualityPolicies `json:"quality_policies"`
	MonitorHost        string          `json:"monitor_host"`
	MonitorPort        string          `json:"monitor_port"`
	MonitorTimeoutMs   int             `json:"monitor_timeout_ms"`
	GitHubRepo         string          `json:"github_repo"`
	AttachmentsEnabled bool            `json:"attachments_enabled"`
}

// QualityPolicies represents the complete quality gate configuration
type QualityPolicies struct {
	Description           string              `json:"description"`
	PrecisionThresholds   PrecisionThresholds `json:"precision_thresholds"`
	BadMissRateThresholds map[string]float64  `json:"bad_miss_rate_thresholds"`
	OperationalHealth     OperationalHealth   `json:"operational_health"`
	ArtifactIntegrity     ArtifactIntegrity   `json:"artifact_integrity"`
}

// PrecisionThresholds contains precision requirements for shipping
type PrecisionThresholds struct {
	MinPrecisionP2024h float64 `json:"min_precision_p20_24h"`
	MinPrecisionP2048h float64 `json:"min_precision_p20_48h"`
	MinWinRate24h      float64 `json:"min_win_rate_24h"`
	MinWinRate48h      float64 `json:"min_win_rate_48h"`
}

// OperationalHealth contains operational health requirements
type OperationalHealth struct {
	MinCacheHitRateHot       float64  `json:"min_cache_hit_rate_hot"`
	MinCacheHitRateWarm      float64  `json:"min_cache_hit_rate_warm"`
	MaxScanLatencyP99Ms      float64  `json:"max_scan_latency_p99_ms"`
	RequiredArtifacts        []string `json:"required_artifacts"`
	MaxCandidatesAgeHours    int      `json:"max_candidates_age_hours"`
	MonitorEndpointTimeoutMs int      `json:"monitor_endpoint_timeout_ms"`
}

// ArtifactIntegrity contains artifact validation requirements
type ArtifactIntegrity struct {
	MinCandidatesLines   int                    `json:"min_candidates_lines"`
	RequiredUniverseHash bool                   `json:"required_universe_hash"`
	MaxFileSizes         map[string]interface{} `json:"max_file_sizes"`
}

// ShipResults contains comprehensive shipping analysis
type ShipResults struct {
	// Performance Results
	LatestDigest       *DigestData     `json:"latest_digest,omitempty"`
	PerformanceMetrics PerformanceKPIs `json:"performance_metrics"`

	// Operational Health
	MonitorSnapshot  *MetricsSnapshot `json:"monitor_snapshot,omitempty"`
	MonitorReachable bool             `json:"monitor_reachable"`

	// Artifact Integrity
	ArtifactChecks []ArtifactCheck `json:"artifact_checks"`
	CoveragePolicy string          `json:"coverage_policy"`

	// Quality Gates
	PolicyViolations   []PolicyViolation `json:"policy_violations"`
	QualityGatesPassed bool              `json:"quality_gates_passed"`

	// Git Context
	Branch       string `json:"branch"`
	CommitSHA    string `json:"commit_sha"`
	LatestDryrun string `json:"latest_dryrun"`
}

// PerformanceKPIs contains key performance indicators
type PerformanceKPIs struct {
	Precision24h      float64 `json:"precision_24h"`
	Precision48h      float64 `json:"precision_48h"`
	WinRate24h        float64 `json:"win_rate_24h"`
	WinRate48h        float64 `json:"win_rate_48h"`
	LiftVsBaseline24h float64 `json:"lift_vs_baseline_24h"`
	LiftVsBaseline48h float64 `json:"lift_vs_baseline_48h"`
	Sparkline7d       string  `json:"sparkline_7d"`
	TotalEntries      int     `json:"total_entries"`
}

// ArtifactCheck represents validation status of an artifact
type ArtifactCheck struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Exists       bool      `json:"exists"`
	Size         string    `json:"size"`
	SizeBytes    int64     `json:"size_bytes"`
	Lines        int       `json:"lines,omitempty"`
	SHA          string    `json:"sha"`
	Age          string    `json:"age"`
	Status       string    `json:"status"` // "PASS", "FAIL", "WARN"
	Issues       []string  `json:"issues,omitempty"`
	LastModified time.Time `json:"last_modified"`
}

// PolicyViolation represents a shipping policy violation
type PolicyViolation struct {
	Category    string      `json:"category"`
	Description string      `json:"description"`
	Current     interface{} `json:"current,omitempty"`
	Required    interface{} `json:"required,omitempty"`
	Blocker     bool        `json:"blocker"`
}

// AttachmentLink represents a link to an attached artifact
type AttachmentLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Path string `json:"path"`
}

// DigestData represents digest analysis data (simplified for ship use)
type DigestData struct {
	Date              string  `json:"date"`
	PrecisionAt20_24h float64 `json:"precision_at_20_24h"`
	PrecisionAt20_48h float64 `json:"precision_at_20_48h"`
	WinRate24h        float64 `json:"win_rate_24h"`
	WinRate48h        float64 `json:"win_rate_48h"`
	LiftVsBaseline24h float64 `json:"lift_vs_baseline_24h"`
	LiftVsBaseline48h float64 `json:"lift_vs_baseline_48h"`
	Sparkline7d       string  `json:"sparkline_7d"`
	TotalEntries      int     `json:"total_entries"`
}

// CoverageAnalysis represents analyst coverage analysis
type CoverageAnalysis struct {
	Generated      string           `json:"generated"`
	TotalSymbols   int              `json:"total_symbols"`
	CoveredSymbols int              `json:"covered_symbols"`
	CoverageRate   float64          `json:"coverage_rate"`
	Symbols        []CoverageSymbol `json:"symbols"`
	PolicyStatus   string           `json:"policy_status"`
}

// CoverageSymbol represents a single symbol's coverage status
type CoverageSymbol struct {
	Symbol  string `json:"symbol"`
	Reason  string `json:"reason,omitempty"` // Empty if covered
	BadMiss bool   `json:"bad_miss,omitempty"`
}

// ShipManager handles the complete shipping workflow
type ShipManager struct {
	config           *ShipConfig
	metricsCollector *MetricsCollector
	templatePath     string
}

// NewShipManager creates a new ship manager
func NewShipManager() (*ShipManager, error) {
	// Load quality policies
	policies, err := LoadQualityPolicies("config/quality_policies.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load quality policies: %w", err)
	}

	config := &ShipConfig{
		QualityPolicies:    *policies,
		MonitorHost:        "localhost",
		MonitorPort:        "8080",
		MonitorTimeoutMs:   5000,
		AttachmentsEnabled: true,
	}

	metricsCollector := NewMetricsCollector(config.MonitorHost, config.MonitorPort, config.MonitorTimeoutMs)

	return &ShipManager{
		config:           config,
		metricsCollector: metricsCollector,
		templatePath:     "templates/PR_BODY.md.tmpl",
	}, nil
}

// LoadQualityPolicies loads quality policies from JSON file
func LoadQualityPolicies(path string) (*QualityPolicies, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var policies QualityPolicies
	if err := json.Unmarshal(data, &policies); err != nil {
		return nil, err
	}

	return &policies, nil
}

// AnalyzeShipReadiness performs comprehensive shipping analysis
func (sm *ShipManager) AnalyzeShipReadiness() (*ShipResults, error) {
	results := &ShipResults{
		PolicyViolations:   []PolicyViolation{},
		QualityGatesPassed: true,
		ArtifactChecks:     []ArtifactCheck{},
	}

	// Get git context
	if err := sm.populateGitContext(results); err != nil {
		log.Warn().Err(err).Msg("Failed to get git context")
	}

	// Get latest dryrun from changelog
	if err := sm.populateLatestDryrun(results); err != nil {
		log.Warn().Err(err).Msg("Failed to get latest dryrun")
	}

	// Analyze performance results
	if err := sm.analyzePerformanceResults(results); err != nil {
		return nil, fmt.Errorf("failed to analyze performance results: %w", err)
	}

	// Collect operational health snapshot
	sm.collectOperationalHealth(results)

	// Validate artifact integrity
	if err := sm.validateArtifactIntegrity(results); err != nil {
		return nil, fmt.Errorf("failed to validate artifacts: %w", err)
	}

	// Check coverage policy
	sm.checkCoveragePolicy(results)

	// Evaluate quality gates
	sm.evaluateQualityGates(results)

	return results, nil
}

// populateGitContext fills git branch and commit information
func (sm *ShipManager) populateGitContext(results *ShipResults) error {
	// Get current branch (simplified - in real implementation use git commands)
	results.Branch = "feature/enhanced-ship"
	results.CommitSHA = "abc1234" // In real implementation, get from git rev-parse HEAD
	return nil
}

// populateLatestDryrun extracts the latest dryrun line from CHANGELOG
func (sm *ShipManager) populateLatestDryrun(results *ShipResults) error {
	data, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "dryrun") ||
			strings.Contains(strings.ToLower(line), "dry-run") ||
			strings.Contains(strings.ToLower(line), "dry run") {
			results.LatestDryrun = strings.TrimSpace(line)
			break
		}
	}

	if results.LatestDryrun == "" {
		results.LatestDryrun = "No dryrun entry found in CHANGELOG"
	}

	return nil
}

// analyzePerformanceResults extracts KPIs from the latest digest
func (sm *ShipManager) analyzePerformanceResults(results *ShipResults) error {
	// Find latest digest
	digestData, err := sm.findLatestDigest()
	if err != nil {
		violation := PolicyViolation{
			Category:    "performance",
			Description: "Latest digest not found or unreadable",
			Blocker:     true,
		}
		results.PolicyViolations = append(results.PolicyViolations, violation)
		results.QualityGatesPassed = false
		return nil // Continue with other checks
	}

	results.LatestDigest = digestData

	// Extract performance KPIs
	kpis := PerformanceKPIs{
		Precision24h:      digestData.PrecisionAt20_24h,
		Precision48h:      digestData.PrecisionAt20_48h,
		WinRate24h:        digestData.WinRate24h,
		WinRate48h:        digestData.WinRate48h,
		LiftVsBaseline24h: digestData.LiftVsBaseline24h,
		LiftVsBaseline48h: digestData.LiftVsBaseline48h,
		Sparkline7d:       digestData.Sparkline7d,
		TotalEntries:      digestData.TotalEntries,
	}
	results.PerformanceMetrics = kpis

	// Check precision thresholds
	if kpis.Precision24h < sm.config.QualityPolicies.PrecisionThresholds.MinPrecisionP2024h {
		violation := PolicyViolation{
			Category:    "performance",
			Description: "Precision@20 (24h) below minimum threshold",
			Current:     kpis.Precision24h,
			Required:    sm.config.QualityPolicies.PrecisionThresholds.MinPrecisionP2024h,
			Blocker:     true,
		}
		results.PolicyViolations = append(results.PolicyViolations, violation)
		results.QualityGatesPassed = false
	}

	if kpis.Precision48h < sm.config.QualityPolicies.PrecisionThresholds.MinPrecisionP2048h {
		violation := PolicyViolation{
			Category:    "performance",
			Description: "Precision@20 (48h) below minimum threshold",
			Current:     kpis.Precision48h,
			Required:    sm.config.QualityPolicies.PrecisionThresholds.MinPrecisionP2048h,
			Blocker:     true,
		}
		results.PolicyViolations = append(results.PolicyViolations, violation)
		results.QualityGatesPassed = false
	}

	return nil
}

// findLatestDigest finds the most recent digest data
func (sm *ShipManager) findLatestDigest() (*DigestData, error) {
	digestDir := "out/digest"

	// Check for latest symlink first
	latestPath := filepath.Join(digestDir, "latest", "digest.json")
	if data, err := os.ReadFile(latestPath); err == nil {
		var digestData DigestData
		if err := json.Unmarshal(data, &digestData); err == nil {
			return &digestData, nil
		}
	}

	// Fall back to finding newest date directory
	entries, err := os.ReadDir(digestDir)
	if err != nil {
		return nil, err
	}

	var latestDate time.Time
	var latestFile string

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "latest" {
			if date, err := time.Parse("2006-01-02", entry.Name()); err == nil {
				if date.After(latestDate) {
					latestDate = date
					latestFile = filepath.Join(digestDir, entry.Name(), "digest.json")
				}
			}
		}
	}

	if latestFile == "" {
		return nil, fmt.Errorf("no digest files found")
	}

	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, err
	}

	var digestData DigestData
	if err := json.Unmarshal(data, &digestData); err != nil {
		return nil, err
	}

	return &digestData, nil
}

// collectOperationalHealth collects monitor health snapshot
func (sm *ShipManager) collectOperationalHealth(results *ShipResults) {
	snapshot, err := sm.metricsCollector.CollectSnapshot()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to collect metrics snapshot")
		results.MonitorReachable = false

		violation := PolicyViolation{
			Category:    "operational",
			Description: "Monitor /metrics endpoint unreachable",
			Blocker:     true,
		}
		results.PolicyViolations = append(results.PolicyViolations, violation)
		results.QualityGatesPassed = false
		return
	}

	results.MonitorSnapshot = snapshot
	results.MonitorReachable = true

	// Check operational health against policies
	policiesMap := map[string]interface{}{
		"operational_health": map[string]interface{}{
			"min_cache_hit_rate_hot":  sm.config.QualityPolicies.OperationalHealth.MinCacheHitRateHot,
			"min_cache_hit_rate_warm": sm.config.QualityPolicies.OperationalHealth.MinCacheHitRateWarm,
			"max_scan_latency_p99_ms": sm.config.QualityPolicies.OperationalHealth.MaxScanLatencyP99Ms,
		},
	}

	if !snapshot.IsHealthy(policiesMap) {
		// Check specific violations
		if snapshot.CacheHitRates.Hot.HitRate < sm.config.QualityPolicies.OperationalHealth.MinCacheHitRateHot {
			violation := PolicyViolation{
				Category:    "operational",
				Description: "Hot cache hit rate below threshold",
				Current:     snapshot.CacheHitRates.Hot.HitRate,
				Required:    sm.config.QualityPolicies.OperationalHealth.MinCacheHitRateHot,
				Blocker:     false, // Warning, not blocker
			}
			results.PolicyViolations = append(results.PolicyViolations, violation)
		}

		if snapshot.Latency.ScanP99 > sm.config.QualityPolicies.OperationalHealth.MaxScanLatencyP99Ms {
			violation := PolicyViolation{
				Category:    "operational",
				Description: "Scan P99 latency exceeds threshold",
				Current:     snapshot.Latency.ScanP99,
				Required:    sm.config.QualityPolicies.OperationalHealth.MaxScanLatencyP99Ms,
				Blocker:     false, // Warning, not blocker
			}
			results.PolicyViolations = append(results.PolicyViolations, violation)
		}
	}
}

// validateArtifactIntegrity checks all required artifacts
func (sm *ShipManager) validateArtifactIntegrity(results *ShipResults) error {
	requiredArtifacts := sm.config.QualityPolicies.OperationalHealth.RequiredArtifacts

	for _, artifactPath := range requiredArtifacts {
		check := sm.checkArtifact(artifactPath)
		results.ArtifactChecks = append(results.ArtifactChecks, check)

		if check.Status == "FAIL" {
			violation := PolicyViolation{
				Category:    "artifacts",
				Description: fmt.Sprintf("Required artifact missing or invalid: %s", artifactPath),
				Blocker:     true,
			}
			results.PolicyViolations = append(results.PolicyViolations, violation)
			results.QualityGatesPassed = false
		}
	}

	// Special check for universe.json hash
	if sm.config.QualityPolicies.ArtifactIntegrity.RequiredUniverseHash {
		universeCheck := sm.checkUniverseHash()
		if universeCheck.Status == "FAIL" {
			violation := PolicyViolation{
				Category:    "artifacts",
				Description: "universe.json missing required _hash field",
				Blocker:     true,
			}
			results.PolicyViolations = append(results.PolicyViolations, violation)
			results.QualityGatesPassed = false
		}
	}

	return nil
}

// checkArtifact validates a single artifact
func (sm *ShipManager) checkArtifact(path string) ArtifactCheck {
	check := ArtifactCheck{
		Name:   filepath.Base(path),
		Path:   path,
		Status: "FAIL",
		Issues: []string{},
	}

	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			check.Issues = append(check.Issues, "File does not exist")
		} else {
			check.Issues = append(check.Issues, fmt.Sprintf("Stat error: %v", err))
		}
		return check
	}

	check.Exists = true
	check.SizeBytes = info.Size()
	check.Size = formatFileSize(info.Size())
	check.LastModified = info.ModTime()
	check.Age = formatAge(time.Since(info.ModTime()))

	// Calculate SHA256
	if hash, err := calculateFileSHA256(path); err == nil {
		check.SHA = hash[:7] // Short SHA
	}

	// Count lines for text files
	if strings.HasSuffix(path, ".jsonl") || strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".json") {
		if lines, err := countFileLines(path); err == nil {
			check.Lines = lines
		}
	}

	// Check specific requirements
	if strings.Contains(path, "candidates.jsonl") {
		minLines := sm.config.QualityPolicies.ArtifactIntegrity.MinCandidatesLines
		if check.Lines < minLines {
			check.Issues = append(check.Issues, fmt.Sprintf("Too few lines: %d < %d", check.Lines, minLines))
		}

		// Check age
		maxAge := time.Duration(sm.config.QualityPolicies.OperationalHealth.MaxCandidatesAgeHours) * time.Hour
		if time.Since(check.LastModified) > maxAge {
			check.Issues = append(check.Issues, fmt.Sprintf("File too old: %s", check.Age))
		}
	}

	// Check file size limits
	if sm.checkFileSizeLimit(path, check.SizeBytes) {
		check.Issues = append(check.Issues, "File size exceeds limit")
	}

	// Set final status
	if len(check.Issues) == 0 {
		check.Status = "PASS"
	} else if len(check.Issues) == 1 && strings.Contains(check.Issues[0], "old") {
		check.Status = "WARN" // Age warnings are not blockers
	}

	return check
}

// checkFileSizeLimit checks if file exceeds size limits
func (sm *ShipManager) checkFileSizeLimit(path string, sizeBytes int64) bool {
	sizeLimits := sm.config.QualityPolicies.ArtifactIntegrity.MaxFileSizes

	if strings.Contains(path, "candidates.jsonl") {
		if limit, ok := sizeLimits["candidates_jsonl_mb"]; ok {
			if limitFloat, ok := limit.(float64); ok {
				maxBytes := int64(limitFloat * 1024 * 1024)
				return sizeBytes > maxBytes
			}
		}
	}

	if strings.Contains(path, "coverage.json") {
		if limit, ok := sizeLimits["coverage_json_kb"]; ok {
			if limitFloat, ok := limit.(float64); ok {
				maxBytes := int64(limitFloat * 1024)
				return sizeBytes > maxBytes
			}
		}
	}

	if strings.Contains(path, "digest.md") {
		if limit, ok := sizeLimits["digest_md_kb"]; ok {
			if limitFloat, ok := limit.(float64); ok {
				maxBytes := int64(limitFloat * 1024)
				return sizeBytes > maxBytes
			}
		}
	}

	return false
}

// checkUniverseHash validates universe.json has _hash field
func (sm *ShipManager) checkUniverseHash() ArtifactCheck {
	check := ArtifactCheck{
		Name:   "universe.json",
		Path:   "config/universe.json",
		Status: "PASS",
	}

	data, err := os.ReadFile("config/universe.json")
	if err != nil {
		check.Status = "FAIL"
		check.Issues = []string{"Cannot read universe.json"}
		return check
	}

	var universeConfig map[string]interface{}
	if err := json.Unmarshal(data, &universeConfig); err != nil {
		check.Status = "FAIL"
		check.Issues = []string{"Invalid JSON in universe.json"}
		return check
	}

	// Check for _metadata.hash
	if metadata, ok := universeConfig["_metadata"].(map[string]interface{}); ok {
		if hash, ok := metadata["hash"].(string); ok && hash != "" {
			check.SHA = hash[:7] // Use first 7 chars of hash
		} else {
			check.Status = "FAIL"
			check.Issues = []string{"Missing _metadata.hash field"}
		}
	} else {
		check.Status = "FAIL"
		check.Issues = []string{"Missing _metadata section"}
	}

	return check
}

// checkCoveragePolicy evaluates coverage.json and sets policy status
func (sm *ShipManager) checkCoveragePolicy(results *ShipResults) {
	coverageData, err := sm.loadCoverageData()
	if err != nil {
		results.CoveragePolicy = "FAIL - coverage.json missing or unreadable"

		violation := PolicyViolation{
			Category:    "coverage",
			Description: "coverage.json missing or unreadable",
			Blocker:     true,
		}
		results.PolicyViolations = append(results.PolicyViolations, violation)
		results.QualityGatesPassed = false
		return
	}

	// Evaluate coverage policy (simplified)
	totalSymbols := len(coverageData.Symbols)
	missingCount := 0
	for _, symbol := range coverageData.Symbols {
		if symbol.Reason != "" { // Has blocking reason
			missingCount++
		}
	}

	coverageRate := float64(totalSymbols-missingCount) / float64(totalSymbols)

	if coverageRate >= 0.80 { // 80% coverage threshold
		results.CoveragePolicy = "PASS"
	} else {
		results.CoveragePolicy = fmt.Sprintf("FAIL - Coverage %.1f%% < 80%%", coverageRate*100)

		violation := PolicyViolation{
			Category:    "coverage",
			Description: "Coverage below 80% threshold",
			Current:     coverageRate,
			Required:    0.80,
			Blocker:     false, // Warning, not blocker
		}
		results.PolicyViolations = append(results.PolicyViolations, violation)
	}
}

// loadCoverageData loads analyst coverage data
func (sm *ShipManager) loadCoverageData() (*CoverageAnalysis, error) {
	data, err := os.ReadFile("out/analyst/coverage.json")
	if err != nil {
		return nil, err
	}

	var coverage CoverageAnalysis
	if err := json.Unmarshal(data, &coverage); err != nil {
		return nil, err
	}

	return &coverage, nil
}

// evaluateQualityGates performs final quality gate evaluation
func (sm *ShipManager) evaluateQualityGates(results *ShipResults) {
	// Count blocking violations
	blockingViolations := 0
	for _, violation := range results.PolicyViolations {
		if violation.Blocker {
			blockingViolations++
		}
	}

	results.QualityGatesPassed = blockingViolations == 0
}

// Helper functions

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatAge(duration time.Duration) string {
	if duration < time.Hour {
		return fmt.Sprintf("%.0fm", duration.Minutes())
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%.1fh", duration.Hours())
	}
	return fmt.Sprintf("%.1fd", duration.Hours()/24)
}

func calculateFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func countFileLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
	}

	return lines, scanner.Err()
}
