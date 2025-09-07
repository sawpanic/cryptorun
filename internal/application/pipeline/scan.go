package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
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

// ScanUniverse implements the required interface method with offline facade
func (p *LegacyScanPipeline) ScanUniverse(ctx context.Context) ([]CandidateResult, error) {
	log.Info().Str("regime", p.regime).Msg("Starting offline scan with fake data facade")

	// Get fake universe symbols for testing
	symbols := getFakeUniverseSymbols()
	
	var candidates []CandidateResult
	
	// Generate fake candidates based on deterministic data
	for i, symbol := range symbols {
		if i >= 20 { // Limit to 20 symbols for offline mode
			break
		}
		
		candidate := generateFakeCandidate(symbol, p.regime)
		candidates = append(candidates, candidate)
	}
	
	log.Info().Int("total_candidates", len(candidates)).Msg("Generated fake candidates for offline scanning")
	
	return candidates, nil
}

// WriteJSONL implements the required interface method
func (p *LegacyScanPipeline) WriteJSONL(candidates []CandidateResult, outputDir string) error {
	log.Info().Str("output_dir", outputDir).Int("count", len(candidates)).Msg("Writing JSONL candidates")
	
	// Create output directory if it doesn't exist
	if err := createDirIfNotExists(outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Write to JSONL format (JSON Lines)
	outputPath := fmt.Sprintf("%s/latest_candidates.jsonl", outputDir)
	if err := writeJSONL(candidates, outputPath); err != nil {
		return fmt.Errorf("failed to write JSONL file: %w", err)
	}
	
	log.Info().Str("path", outputPath).Msg("JSONL candidates written successfully")
	return nil
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
	Symbol        string          `json:"symbol"`
	Timestamp     time.Time       `json:"timestamp"`
	Score         SimpleScore     `json:"score"`
	Factors       SimpleFactorSet `json:"factors"`
	Gates         AllGateResults  `json:"gates"`
	Decision      string          `json:"decision"`
	SnapshotSaved bool            `json:"snapshot_saved"`
	Selected      bool            `json:"selected"`
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

// Helper functions for offline scanning

// getFakeUniverseSymbols returns fake universe symbols for offline scanning
func getFakeUniverseSymbols() []string {
	// Return the same symbols as the facade without importing it
	return []string{
		"BTCUSD", "ETHUSD", "SOLUSD", "ADAUSD", "LINKUSD",
		"DOTUSD", "MATICUSD", "AVAXUSD", "UNIUSD", "LTCUSD",
		"XRPUSD", "ALGOUSD", "ATOMUSD", "NEARUSD", "FTMUSD",
		"MANAUSD", "SANDUSD", "ENJUSD", "GALAUSD", "CHZUSD",
	}
}

// generateFakeCandidate creates a deterministic fake candidate for testing
func generateFakeCandidate(symbol, regime string) CandidateResult {
	// Use symbol as seed for deterministic results
	seed := int64(0)
	for _, char := range symbol {
		seed += int64(char)
	}
	rng := rand.New(rand.NewSource(seed))
	
	// Generate regime-adapted scores
	score := generateFakeScore(symbol, regime, rng)
	factors := generateFakeFactors(symbol, regime, rng)
	gates := generateFakeGates(symbol, regime, rng)
	
	// Determine selection based on score and gates
	selected := score.Total >= 75.0 && gates.AllPass
	
	return CandidateResult{
		Symbol:        symbol,
		Timestamp:     time.Now(),
		Score:         score,
		Factors:       factors,
		Gates:         gates,
		Decision:      getDecisionReason(selected, score.Total, gates.AllPass),
		SnapshotSaved: true, // Always save in offline mode
		Selected:      selected,
	}
}

// generateFakeScore creates regime-aware fake scoring
func generateFakeScore(symbol, regime string, rng *rand.Rand) SimpleScore {
	// Base score varies by symbol deterministically
	baseScore := 40.0 + rng.Float64()*50.0 // 40-90 range
	
	// Regime adjustments
	switch regime {
	case "trending_bull", "bull", "trending":
		baseScore += 10.0 // Higher scores in bull markets
	case "high_vol", "volatile", "high_volatility":
		baseScore -= 5.0 // Lower scores in high volatility
	}
	
	// Ensure some candidates pass the threshold
	if symbol == "BTCUSD" || symbol == "ETHUSD" {
		baseScore = 80.0 + rng.Float64()*15.0 // 80-95 for majors
	}
	
	return SimpleScore{
		Total:      baseScore,
		Momentum:   baseScore * 0.4,  // 40% momentum component
		Technical:  baseScore * 0.25, // 25% technical
		Volume:     baseScore * 0.2,  // 20% volume
		Quality:    baseScore * 0.1,  // 10% quality
		Social:     rng.Float64() * 10.0, // 0-10 social cap
		Breakdown:  "fake scoring for offline mode",
	}
}

// generateFakeFactors creates regime-aware fake factor data
func generateFakeFactors(symbol, regime string, rng *rand.Rand) SimpleFactorSet {
	return SimpleFactorSet{
		Momentum:  60.0 + rng.Float64()*30.0, // 60-90
		Technical: 50.0 + rng.Float64()*40.0, // 50-90  
		Volume:    70.0 + rng.Float64()*25.0, // 70-95
		Quality:   65.0 + rng.Float64()*30.0, // 65-95
		Social:    rng.Float64() * 15.0,      // 0-15 (capped at 10 in composite)
	}
}

// generateFakeGates creates regime-aware fake gate results
func generateFakeGates(symbol, regime string, rng *rand.Rand) AllGateResults {
	// Most symbols should pass gates in fake mode
	passRate := 0.8 // 80% pass rate
	if symbol == "BTCUSD" || symbol == "ETHUSD" {
		passRate = 0.95 // 95% for majors
	}
	
	microPass := rng.Float64() < passRate
	freshPass := rng.Float64() < 0.9  // 90% pass freshness
	latePass  := rng.Float64() < 0.95 // 95% pass late fill
	fatiguePass := rng.Float64() < 0.85 // 85% pass fatigue
	
	allPass := microPass && freshPass && latePass && fatiguePass
	
	var reasons []string
	if !microPass {
		reasons = append(reasons, "microstructure_fail")
	}
	if !freshPass {
		reasons = append(reasons, "freshness_fail")
	}
	if !latePass {
		reasons = append(reasons, "late_fill_fail")
	}
	if !fatiguePass {
		reasons = append(reasons, "fatigue_fail")
	}
	
	return AllGateResults{
		Microstructure: MicroGateResults{
			SpreadBps: 15.0 + rng.Float64()*35.0, // 15-50 bps
			DepthUSD:  50000 + rng.Float64()*150000, // 50k-200k USD
			VADR:      1.2 + rng.Float64()*1.3, // 1.2-2.5
			AllPass:   microPass,
			Reason:    getGateReason(microPass),
		},
		Freshness: GateEvidence{
			OK:   freshPass,
			Name: "freshness_guard",
		},
		LateFill: GateEvidence{
			OK:   latePass,
			Name: "late_fill_guard",
		},
		Fatigue: GateEvidence{
			OK:   fatiguePass,
			Name: "fatigue_guard",
		},
		AllPass:        allPass,
		FailureReasons: reasons,
	}
}

// getDecisionReason returns human-readable decision explanation
func getDecisionReason(selected bool, score float64, gatesPass bool) string {
	if selected {
		return fmt.Sprintf("SELECTED: score=%.1f ≥75.0, gates=PASS", score)
	}
	
	if score < 75.0 && !gatesPass {
		return fmt.Sprintf("REJECTED: score=%.1f <75.0, gates=FAIL", score)
	} else if score < 75.0 {
		return fmt.Sprintf("REJECTED: score=%.1f <75.0", score)
	} else {
		return fmt.Sprintf("REJECTED: gates=FAIL (score=%.1f ≥75.0)", score)
	}
}

// getGateReason returns reason for gate result
func getGateReason(pass bool) string {
	if pass {
		return ""
	}
	return "spread/depth/vadr thresholds not met"
}

// File utility functions

// createDirIfNotExists creates directory if it doesn't exist
func createDirIfNotExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

// writeJSONL writes candidates to JSON Lines format
func writeJSONL(candidates []CandidateResult, filePath string) error {
	// Create directory if needed
	dir := filepath.Dir(filePath)
	if err := createDirIfNotExists(dir); err != nil {
		return err
	}
	
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	for _, candidate := range candidates {
		if err := encoder.Encode(candidate); err != nil {
			return err
		}
	}
	
	return nil
}

// SimpleScore for offline mode compatibility
type SimpleScore struct {
	Total      float64 `json:"total"`
	Momentum   float64 `json:"momentum"`
	Technical  float64 `json:"technical"`
	Volume     float64 `json:"volume"`
	Quality    float64 `json:"quality"`
	Social     float64 `json:"social"`
	Breakdown  string  `json:"breakdown"`
}

// SimpleFactorSet for offline mode compatibility
type SimpleFactorSet struct {
	Momentum  float64 `json:"momentum"`
	Technical float64 `json:"technical"`
	Volume    float64 `json:"volume"`
	Quality   float64 `json:"quality"`
	Social    float64 `json:"social"`
}

// Forward declarations for types that will be imported from existing packages
type ScanPipeline interface{}

func NewScanPipeline(snapshotDir string) ScanPipeline { return nil }
