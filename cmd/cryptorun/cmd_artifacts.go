package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"cryptorun/internal/artifacts/compact"
	"cryptorun/internal/artifacts/gc"
	"cryptorun/internal/artifacts/manifest"
)

var artifactsCmd = &cobra.Command{
	Use:   "artifacts",
	Short: "Manage artifact retention, compaction, and garbage collection",
	Long: `Artifact management commands for CryptoRun verification outputs.
	
Manages retention policies, compaction, and garbage collection for:
- GREEN-WALL verification artifacts (proofs, benches, smoke90, greenwall)
- Factor explanation deltas and analysis outputs
- Performance benchmarks and test results`,
}

var artifactsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List artifacts with filtering options",
	Long: `List all tracked artifacts with optional family filtering.
	
Shows: ID, family, timestamp, size, pass/fail status, and pinned flag.`,
	RunE: runArtifactsList,
}

var artifactsGCCmd = &cobra.Command{
	Use:   "gc",
	Short: "Run garbage collection on artifacts",
	Long: `Run garbage collection to remove old artifacts according to retention policies.
	
By default runs in dry-run mode. Use --apply to actually delete files.
Always preserves:
- Pinned artifacts
- Most recent PASS for each family  
- Most recent run for each family`,
	RunE: runArtifactsGC,
}

var artifactsCompactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Compact JSONL and Markdown files",
	Long: `Compact artifacts to reduce disk usage while preserving data integrity.
	
JSONL compaction uses dictionary compression for repeated field values.
Markdown compaction removes empty sections and canonicalizes headers.`,
	RunE: runArtifactsCompact,
}

var artifactsPinCmd = &cobra.Command{
	Use:   "pin",
	Short: "Pin or unpin artifacts to prevent deletion",
	Long: `Pin artifacts to protect them from garbage collection.
	
Pinned artifacts are never deleted regardless of retention policies.`,
	RunE: runArtifactsPin,
}

var artifactsScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan filesystem and rebuild artifact manifest",
	Long: `Scan the artifacts directory to discover and index all artifact files.
	
Builds or updates the artifact manifest with current filesystem state.`,
	RunE: runArtifactsScan,
}

// Command flags
var (
	artifactFamily   string
	artifactsDryRun  bool
	artifactsApply   bool
	artifactsForce   bool
	artifactsID      string
	artifactsPinOn   bool
	artifactsPinOff  bool
	artifactsVerbose bool
	artifactsJSON    bool
)

func init() {
	// Add subcommands
	artifactsCmd.AddCommand(artifactsListCmd)
	artifactsCmd.AddCommand(artifactsGCCmd)
	artifactsCmd.AddCommand(artifactsCompactCmd)
	artifactsCmd.AddCommand(artifactsPinCmd)
	artifactsCmd.AddCommand(artifactsScanCmd)

	// List command flags
	artifactsListCmd.Flags().StringVar(&artifactFamily, "family", "", "Filter by artifact family (proofs, bench, smoke90, explain, greenwall)")
	artifactsListCmd.Flags().BoolVar(&artifactsJSON, "json", false, "Output as JSON")
	artifactsListCmd.Flags().BoolVar(&artifactsVerbose, "verbose", false, "Show detailed information")

	// GC command flags
	artifactsGCCmd.Flags().BoolVar(&artifactsDryRun, "dry-run", true, "Show what would be deleted without actually deleting")
	artifactsGCCmd.Flags().BoolVar(&artifactsApply, "apply", false, "Actually perform deletions (overrides --dry-run)")
	artifactsGCCmd.Flags().BoolVar(&artifactsForce, "force", false, "Skip confirmation prompts")

	// Compact command flags
	artifactsCompactCmd.Flags().StringVar(&artifactFamily, "family", "", "Compact specific family only")
	artifactsCompactCmd.Flags().BoolVar(&artifactsApply, "apply", false, "Actually perform compaction (dry-run by default)")
	artifactsCompactCmd.Flags().BoolVar(&artifactsVerbose, "verbose", false, "Show detailed compaction progress")

	// Pin command flags
	artifactsPinCmd.Flags().StringVar(&artifactsID, "id", "", "Artifact ID to pin/unpin (required)")
	artifactsPinCmd.Flags().BoolVar(&artifactsPinOn, "on", false, "Pin the artifact")
	artifactsPinCmd.Flags().BoolVar(&artifactsPinOff, "off", false, "Unpin the artifact")

	// Make id flag required for pin command
	artifactsPinCmd.MarkFlagRequired("id")

	// Scan command flags
	artifactsScanCmd.Flags().BoolVar(&artifactsVerbose, "verbose", false, "Show detailed scan progress")
	artifactsScanCmd.Flags().BoolVar(&artifactsForce, "force", false, "Force full rescan even if manifest is recent")

	rootCmd.AddCommand(artifactsCmd)
}

// runArtifactsList lists artifacts with optional filtering
func runArtifactsList(cmd *cobra.Command, args []string) error {
	// Load configuration
	config, err := loadArtifactsConfig()
	if err != nil {
		return fmt.Errorf("failed to load artifacts config: %w", err)
	}

	// Load or scan manifest
	manifestIO := manifest.NewIO(config.Manifest.File)
	scanner := createScanner(config)

	artifactManifest, err := manifestIO.LoadOrScan(scanner)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Filter by family if specified
	var entries []*manifest.ArtifactEntry
	if artifactFamily != "" {
		entries = artifactManifest.GetByFamily(artifactFamily)
		if len(entries) == 0 {
			fmt.Printf("No artifacts found for family: %s\n", artifactFamily)
			return nil
		}
	} else {
		// Get all entries
		entries = make([]*manifest.ArtifactEntry, len(artifactManifest.Entries))
		for i := range artifactManifest.Entries {
			entries[i] = &artifactManifest.Entries[i]
		}

		// Sort by timestamp (newest first)
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Timestamp.After(entries[j].Timestamp)
		})
	}

	// Output format
	if artifactsJSON {
		return outputArtifactsJSON(entries)
	}

	return outputArtifactsTable(entries)
}

// runArtifactsGC runs garbage collection on artifacts
func runArtifactsGC(cmd *cobra.Command, args []string) error {
	// Load configuration
	config, err := loadArtifactsConfig()
	if err != nil {
		return fmt.Errorf("failed to load artifacts config: %w", err)
	}

	// Load manifest
	manifestIO := manifest.NewIO(config.Manifest.File)
	artifactManifest, err := manifestIO.Load()
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Determine dry-run mode
	isDryRun := artifactsDryRun && !artifactsApply

	// Create retention config
	retentionConfig := make(map[string]gc.RetentionConfig)
	for family, retention := range config.Retention {
		retentionConfig[family] = gc.RetentionConfig{
			Keep:            retention.Keep,
			Pin:             retention.Pin,
			AlwaysKeepRules: config.GC.AlwaysKeep,
		}
	}

	// Create planner and plan GC
	planner := gc.NewPlanner(retentionConfig)
	plan, err := planner.CreatePlan(artifactManifest, isDryRun)
	if err != nil {
		return fmt.Errorf("failed to create GC plan: %w", err)
	}

	// Validate plan
	if err := planner.ValidatePlan(plan, artifactManifest); err != nil {
		return fmt.Errorf("GC plan validation failed: %w", err)
	}

	// Show plan summary
	fmt.Print(planner.GetPlanSummary(plan))

	if !isDryRun {
		// Confirm before proceeding
		if !artifactsForce && !confirmProceed("Proceed with deletion?") {
			fmt.Println("GC cancelled by user")
			return nil
		}

		// Execute the plan
		executor := gc.NewExecutor(config.GC.TrashDir, config.GC.BackupBeforeDelete)
		result, err := executor.Apply(plan, artifactManifest)
		if err != nil {
			return fmt.Errorf("GC execution failed: %w", err)
		}

		// Show results
		fmt.Printf("\n✅ GC completed successfully\n")
		fmt.Printf("Files deleted: %d\n", result.FilesDeleted)
		fmt.Printf("Bytes freed: %s\n", formatBytes(result.BytesDeleted))
		fmt.Printf("Duration: %v\n", result.Duration)

		if len(result.Errors) > 0 {
			fmt.Printf("⚠️  %d errors occurred:\n", len(result.Errors))
			for _, errMsg := range result.Errors {
				fmt.Printf("  - %s\n", errMsg)
			}
		}
	}

	return nil
}

// runArtifactsCompact compacts JSONL and Markdown files
func runArtifactsCompact(cmd *cobra.Command, args []string) error {
	// Load configuration
	config, err := loadArtifactsConfig()
	if err != nil {
		return fmt.Errorf("failed to load artifacts config: %w", err)
	}

	// Load manifest
	manifestIO := manifest.NewIO(config.Manifest.File)
	artifactManifest, err := manifestIO.Load()
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	isDryRun := !artifactsApply

	// Get entries to compact
	var entries []*manifest.ArtifactEntry
	if artifactFamily != "" {
		entries = artifactManifest.GetByFamily(artifactFamily)
	} else {
		entries = make([]*manifest.ArtifactEntry, len(artifactManifest.Entries))
		for i := range artifactManifest.Entries {
			entries[i] = &artifactManifest.Entries[i]
		}
	}

	if len(entries) == 0 {
		fmt.Printf("No artifacts found to compact\n")
		return nil
	}

	// Create compactors
	jsonlCompactor := compact.NewJSONLCompactor(config.Compaction.JSONL)
	mdCompactor := compact.NewMarkdownCompactor(config.Compaction.Markdown)

	compactResults := make([]*compact.CompactResult, 0)

	fmt.Printf("Compacting %d artifact entries (dry-run: %v)\n\n", len(entries), isDryRun)

	// Process each entry
	for _, entry := range entries {
		if artifactsVerbose {
			fmt.Printf("Processing entry %s (%s)...\n", entry.ID[:8], entry.Family)
		}

		for _, path := range entry.Paths {
			var result *compact.CompactResult
			var err error

			if strings.HasSuffix(path, ".jsonl") && config.Compaction.JSONL.Enabled {
				if !isDryRun {
					result, err = jsonlCompactor.CompactFile(path)
				} else {
					// Simulate compaction
					stat, _ := os.Stat(path)
					result = &compact.CompactResult{
						OriginalPath:     path,
						CompactedPath:    strings.TrimSuffix(path, ".jsonl") + ".compact.jsonl",
						OriginalSize:     stat.Size(),
						CompactedSize:    stat.Size() * 7 / 10, // Estimate 30% reduction
						CompressionRatio: 0.7,
						LinesProcessed:   100, // Estimate
					}
				}
			} else if strings.HasSuffix(path, ".md") && config.Compaction.Markdown.Enabled {
				if !isDryRun {
					result, err = mdCompactor.CompactFile(path)
				} else {
					// Simulate compaction
					stat, _ := os.Stat(path)
					result = &compact.CompactResult{
						OriginalPath:     path,
						CompactedPath:    strings.TrimSuffix(path, ".md") + ".compact.md",
						OriginalSize:     stat.Size(),
						CompactedSize:    stat.Size() * 8 / 10, // Estimate 20% reduction
						CompressionRatio: 0.8,
						LinesProcessed:   50, // Estimate
					}
				}
			}

			if result != nil {
				compactResults = append(compactResults, result)
				if err != nil {
					fmt.Printf("⚠️  Failed to compact %s: %v\n", path, err)
				} else if artifactsVerbose {
					fmt.Printf("  %s: %s -> %s (%.1f%% reduction)\n",
						filepath.Base(path),
						formatBytes(result.OriginalSize),
						formatBytes(result.CompactedSize),
						(1-result.CompressionRatio)*100)
				}
			}
		}
	}

	// Show summary
	fmt.Printf("\n📊 Compaction Summary\n")
	fmt.Printf("Files processed: %d\n", len(compactResults))

	var totalOriginal, totalCompacted int64
	for _, result := range compactResults {
		totalOriginal += result.OriginalSize
		totalCompacted += result.CompactedSize
	}

	if totalOriginal > 0 {
		reduction := 1.0 - float64(totalCompacted)/float64(totalOriginal)
		fmt.Printf("Total space: %s -> %s (%.1f%% reduction)\n",
			formatBytes(totalOriginal), formatBytes(totalCompacted), reduction*100)
	}

	return nil
}

// runArtifactsPin pins or unpins artifacts
func runArtifactsPin(cmd *cobra.Command, args []string) error {
	// Validate flags
	if !artifactsPinOn && !artifactsPinOff {
		return fmt.Errorf("must specify either --on or --off")
	}
	if artifactsPinOn && artifactsPinOff {
		return fmt.Errorf("cannot specify both --on and --off")
	}

	// Load manifest
	config, err := loadArtifactsConfig()
	if err != nil {
		return fmt.Errorf("failed to load artifacts config: %w", err)
	}

	manifestIO := manifest.NewIO(config.Manifest.File)
	artifactManifest, err := manifestIO.Load()
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Find the entry
	entry := artifactManifest.GetByID(artifactsID)
	if entry == nil {
		return fmt.Errorf("artifact not found: %s", artifactsID)
	}

	// Update pin status
	newPinStatus := artifactsPinOn
	err = artifactManifest.SetPinned(artifactsID, newPinStatus)
	if err != nil {
		return fmt.Errorf("failed to update pin status: %w", err)
	}

	// Save manifest
	if err := manifestIO.Save(artifactManifest); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	// Show result
	action := "unpinned"
	if newPinStatus {
		action = "pinned"
	}

	fmt.Printf("✅ Artifact %s %s\n", artifactsID[:8], action)
	fmt.Printf("Family: %s\n", entry.Family)
	fmt.Printf("Timestamp: %s\n", entry.Timestamp.Format("2006-01-02 15:04:05"))

	return nil
}

// runArtifactsScan scans filesystem and rebuilds manifest
func runArtifactsScan(cmd *cobra.Command, args []string) error {
	// Load configuration
	config, err := loadArtifactsConfig()
	if err != nil {
		return fmt.Errorf("failed to load artifacts config: %w", err)
	}

	// Create scanner
	scanner := createScanner(config)

	// Check if we need to force scan
	if !artifactsForce {
		manifestIO := manifest.NewIO(config.Manifest.File)
		if existing, err := manifestIO.Load(); err == nil {
			if time.Since(existing.GeneratedAt) < 6*time.Hour {
				fmt.Printf("Manifest is recent (generated %v ago). Use --force to rescan.\n",
					time.Since(existing.GeneratedAt).Round(time.Minute))
				return nil
			}
		}
	}

	fmt.Printf("Scanning artifacts directory...\n")
	if artifactsVerbose {
		fmt.Printf("Root paths: %v\n", scanner.Config.RootPaths)
		fmt.Printf("Workers: %d\n", scanner.Config.WorkerCount)
	}

	// Perform scan
	manifestIO := manifest.NewIO(config.Manifest.File)
	result, err := manifestIO.ScanAndSave(scanner)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Show results
	fmt.Printf("✅ Scan completed\n")
	fmt.Printf("Files scanned: %d\n", result.FilesScanned)
	fmt.Printf("Artifacts found: %d\n", len(result.Manifest.Entries))
	fmt.Printf("Families: %d\n", len(result.Manifest.Families))
	fmt.Printf("Duration: %v\n", result.ScanDuration)

	if result.ErrorCount > 0 {
		fmt.Printf("⚠️  %d errors occurred during scan\n", result.ErrorCount)
	}

	if artifactsVerbose {
		fmt.Printf("\nFamily breakdown:\n")
		for family, count := range result.Manifest.Families {
			fmt.Printf("  %s: %d\n", family, count)
		}
	}

	return nil
}

// Helper functions

func loadArtifactsConfig() (*ArtifactsConfig, error) {
	configPath := filepath.Join("config", "artifacts.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config ArtifactsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func createScanner(config *ArtifactsConfig) *manifest.Scanner {
	scanConfig := &manifest.ScanConfig{
		RootPaths:          []string{"./artifacts", "./out"},
		FamilyPatterns:     config.Patterns,
		WorkerCount:        config.Indexing.ParallelWorkers,
		ChecksumBufferSize: config.Indexing.ChecksumBufferSizeKB * 1024,
		MaxFilesPerScan:    config.Indexing.MaxFilesPerScan,
		FollowSymlinks:     false,
	}

	return manifest.NewScanner(scanConfig)
}

func outputArtifactsJSON(entries []*manifest.ArtifactEntry) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

func outputArtifactsTable(entries []*manifest.ArtifactEntry) error {
	if len(entries) == 0 {
		fmt.Println("No artifacts found")
		return nil
	}

	// Table header
	fmt.Printf("%-12s %-10s %-19s %-8s %-6s %-6s %s\n",
		"ID", "FAMILY", "TIMESTAMP", "SIZE", "STATUS", "PINNED", "FILES")
	fmt.Println(strings.Repeat("-", 80))

	// Table rows
	for _, entry := range entries {
		pinned := ""
		if entry.IsPinned {
			pinned = "📌"
		}

		status := entry.PassFail
		if entry.IsLastRun {
			status += "*"
		}
		if entry.IsLastPass && entry.PassFail == "pass" {
			status += "+"
		}

		fmt.Printf("%-12s %-10s %-19s %-8s %-6s %-6s %d\n",
			entry.ID[:8],
			entry.Family,
			entry.Timestamp.Format("2006-01-02 15:04:05"),
			formatBytes(entry.TotalBytes),
			status,
			pinned,
			len(entry.Paths))
	}

	fmt.Printf("\nLegend: * = last run, + = last pass, 📌 = pinned\n")
	return nil
}

func confirmProceed(message string) bool {
	fmt.Printf("%s [y/N]: ", message)
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1fG", float64(bytes)/(1024*1024*1024))
}

// Configuration structures

type ArtifactsConfig struct {
	Retention  map[string]RetentionPolicy `yaml:"retention"`
	Compaction CompactionConfig           `yaml:"compaction"`
	GC         GCConfig                   `yaml:"gc"`
	Manifest   ManifestConfig             `yaml:"manifest"`
	Patterns   map[string][]string        `yaml:"patterns"`
	Indexing   IndexingConfig             `yaml:"indexing"`
}

type RetentionPolicy struct {
	Keep int      `yaml:"keep"`
	Pin  []string `yaml:"pin"`
}

type CompactionConfig struct {
	JSONL    compact.JSONLConfig    `yaml:"jsonl"`
	Markdown compact.MarkdownConfig `yaml:"markdown"`
}

type GCConfig struct {
	DryRunDefault       bool     `yaml:"dry_run_default"`
	RequireConfirmation bool     `yaml:"require_confirmation"`
	BackupBeforeDelete  bool     `yaml:"backup_before_delete"`
	AlwaysKeep          []string `yaml:"always_keep"`
	TrashDir            string   `yaml:"trash_dir"`
	TrashRetentionDays  int      `yaml:"trash_retention_days"`
}

type ManifestConfig struct {
	File              string `yaml:"file"`
	BackupFile        string `yaml:"backup_file"`
	ScanIntervalHours int    `yaml:"scan_interval_hours"`
	ChecksumAlgorithm string `yaml:"checksum_algorithm"`
}

type IndexingConfig struct {
	ExtractFields        []string `yaml:"extract_fields"`
	ParallelWorkers      int      `yaml:"parallel_workers"`
	ChecksumBufferSizeKB int      `yaml:"checksum_buffer_size_kb"`
	MaxFilesPerScan      int      `yaml:"max_files_per_scan"`
}
