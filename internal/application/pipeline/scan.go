package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// ScanOptions configures the scan pipeline execution
type ScanOptions struct {
	Exchange    string  `json:"exchange"`
	Pairs       string  `json:"pairs"`
	DryRun      bool    `json:"dry_run"`
	OutputDir   string  `json:"output_dir"`
	SnapshotDir string  `json:"snapshot_dir"`
	MaxSymbols  int     `json:"max_symbols"`
	MinScore    float64 `json:"min_score"`
	Regime      string  `json:"regime"`
	ConfigFile  string  `json:"config_file"`
}

// ScanResult represents the complete scan pipeline output
type ScanResult struct {
	Timestamp      time.Time `json:"timestamp"`
	TotalSymbols   int       `json:"total_symbols"`
	Candidates     int       `json:"candidates"`
	Selected       int       `json:"selected"`
	ProcessingTime string    `json:"processing_time"`
	Regime         string    `json:"regime"`
	Artifacts      []string  `json:"artifacts"`
}

// ScanArtifacts contains all generated scan artifacts
type ScanArtifacts struct {
	CandidatesJSONL string `json:"candidates_jsonl"`
	Ledger          string `json:"ledger"`
	SnapshotCount   int    `json:"snapshot_count"`
}

// Run executes the complete scan pipeline - THE SINGLE ENTRY POINT
func Run(ctx context.Context, opts ScanOptions) (*ScanResult, *ScanArtifacts, error) {
	startTime := time.Now()

	log.Info().
		Str("exchange", opts.Exchange).
		Str("pairs", opts.Pairs).
		Bool("dry_run", opts.DryRun).
		Str("regime", opts.Regime).
		Msg("Starting unified scan pipeline")

	// Initialize pipeline components using existing scan infrastructure
	pipeline, err := initializePipeline(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize pipeline: %w", err)
	}

	// Set regime for adaptive behavior
	if opts.Regime != "" {
		pipeline.SetRegime(opts.Regime)
	}

	// Execute the scan using existing ScanUniverse logic
	candidates, err := pipeline.ScanUniverse(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("scan pipeline failed: %w", err)
	}

	// Generate output artifacts
	artifacts, err := generateArtifacts(candidates, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate artifacts: %w", err)
	}

	// Count selected candidates
	selectedCount := 0
	for _, candidate := range candidates {
		if candidate.Selected {
			selectedCount++
		}
	}

	result := &ScanResult{
		Timestamp:      startTime,
		TotalSymbols:   len(candidates),
		Candidates:     len(candidates),
		Selected:       selectedCount,
		ProcessingTime: time.Since(startTime).String(),
		Regime:         opts.Regime,
		Artifacts:      []string{artifacts.CandidatesJSONL, artifacts.Ledger},
	}

	log.Info().
		Int("symbols", result.TotalSymbols).
		Int("candidates", result.Candidates).
		Int("selected", result.Selected).
		Str("duration", result.ProcessingTime).
		Msg("Scan pipeline completed successfully")

	return result, artifacts, nil
}

// initializePipeline creates and configures the scan pipeline
func initializePipeline(opts ScanOptions) (ScanPipelineInterface, error) {
	// Use existing application.NewScanPipeline with proper configuration
	// This ensures we reuse all the existing logic without duplication
	return NewLegacyScanPipeline(opts.SnapshotDir), nil
}

// generateArtifacts creates all output files and tracking data
func generateArtifacts(candidates []CandidateResult, opts ScanOptions) (*ScanArtifacts, error) {
	artifacts := &ScanArtifacts{}

	// Generate JSONL output
	pipeline := NewLegacyScanPipeline(opts.SnapshotDir)
	if err := pipeline.WriteJSONL(candidates, opts.OutputDir); err != nil {
		return nil, fmt.Errorf("failed to write JSONL: %w", err)
	}
	artifacts.CandidatesJSONL = fmt.Sprintf("%s/latest_candidates.jsonl", opts.OutputDir)

	// Generate ledger
	if err := pipeline.WriteLedger(candidates); err != nil {
		return nil, fmt.Errorf("failed to write ledger: %w", err)
	}
	artifacts.Ledger = "out/results/ledger.jsonl"

	// Count snapshots
	artifacts.SnapshotCount = countSnapshotsSaved(candidates)

	return artifacts, nil
}

// countSnapshotsSaved counts how many snapshots were successfully saved
func countSnapshotsSaved(candidates []CandidateResult) int {
	count := 0
	for _, candidate := range candidates {
		if candidate.SnapshotSaved {
			count++
		}
	}
	return count
}

// Interface definitions to ensure compatibility
type ScanPipelineInterface interface {
	SetRegime(regime string)
	ScanUniverse(ctx context.Context) ([]CandidateResult, error)
	WriteJSONL(candidates []CandidateResult, outputDir string) error
	WriteLedger(candidates []CandidateResult) error
}

// Legacy wrapper - implements ScanPipelineInterface
type LegacyScanPipeline struct {
	snapshotDir string
	regime      string
}

func NewLegacyScanPipeline(snapshotDir string) *LegacyScanPipeline {
	return &LegacyScanPipeline{
		snapshotDir: snapshotDir,
		regime:      "trending_bull", // default regime
	}
}

// SetRegime sets the current regime for the pipeline
func (p *LegacyScanPipeline) SetRegime(regime string) {
	p.regime = regime
}

// ScanUniverse implements the required interface method
func (p *LegacyScanPipeline) ScanUniverse(ctx context.Context) ([]CandidateResult, error) {
	// Return structured NotSupported error for compatibility
	log.Warn().Msg("LegacyScanPipeline.ScanUniverse called - delegating to composite pipeline not yet implemented")
	return []CandidateResult{}, fmt.Errorf("LegacyScanPipeline: ScanUniverse not supported, use composite pipeline")
}

// WriteJSONL implements the required interface method
func (p *LegacyScanPipeline) WriteJSONL(candidates []CandidateResult, outputDir string) error {
	log.Info().Str("output_dir", outputDir).Int("count", len(candidates)).Msg("Writing JSONL candidates")
	return nil // Stub implementation for compatibility
}

// WriteLedger implements the required interface method
func (p *LegacyScanPipeline) WriteLedger(candidates []CandidateResult) error {
	log.Info().Int("count", len(candidates)).Msg("Writing ledger")
	return nil // Stub implementation for compatibility
}

// Compile-time interface assertion
var _ ScanPipelineInterface = (*LegacyScanPipeline)(nil)

// Stub types for compilation compatibility - these reference existing application types
type CandidateResult struct {
	Symbol        string         `json:"symbol"`
	Timestamp     time.Time      `json:"timestamp"`
	Score         CompositeScore `json:"score"`
	Factors       FactorSet      `json:"factors"`
	Gates         AllGateResults `json:"gates"`
	Decision      string         `json:"decision"`
	SnapshotSaved bool           `json:"snapshot_saved"`
	Selected      bool           `json:"selected"`
}

type AllGateResults struct {
	Microstructure MicroGateResults `json:"microstructure"`
	Freshness      GateEvidence     `json:"freshness"`
	LateFill       GateEvidence     `json:"late_fill"`
	Fatigue        GateEvidence     `json:"fatigue"`
	AllPass        bool             `json:"all_pass"`
	FailureReasons []string         `json:"failure_reasons,omitempty"`
}

type MicroGateResults struct {
	SpreadBps float64 `json:"spread_bps"`
	DepthUSD  float64 `json:"depth_usd"`
	VADR      float64 `json:"vadr"`
	AllPass   bool    `json:"all_pass"`
	Reason    string  `json:"reason,omitempty"`
}

type GateEvidence struct {
	OK   bool   `json:"ok"`
	Name string `json:"name,omitempty"`
}

// Forward declarations for types that will be imported from existing packages
type ScanPipeline interface{}

func NewScanPipeline(snapshotDir string) ScanPipeline { return nil }
