package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sawpanic/cryptorun/internal/algo/momentum"
	"github.com/sawpanic/cryptorun/internal/scan/progress"
)

// MomentumPipeline orchestrates momentum scanning with explainability
type MomentumPipeline struct {
	momentumCore   *momentum.MomentumCore
	orthogonalizer *momentum.GramSchmidtOrthogonalizer
	entryExitGates *momentum.EntryExitGates
	config         MomentumPipelineConfig
	outputDir      string
	progressBus    *progress.ScanProgressBus
}

// MomentumPipelineConfig defines pipeline configuration
type MomentumPipelineConfig struct {
	Momentum  momentum.MomentumConfig  `yaml:"momentum"`
	EntryExit momentum.EntryExitConfig `yaml:"entry_exit"`
	Pipeline  PipelineConfig           `yaml:"pipeline"`
}

// PipelineConfig defines pipeline-specific settings
type PipelineConfig struct {
	ExplainabilityOutput bool     `yaml:"explainability_output"`
	OutputPath           string   `yaml:"output_path"`
	ProtectedFactors     []string `yaml:"protected_factors"`
	MaxSymbols           int      `yaml:"max_symbols"`
}

// MomentumCandidate represents a momentum scanning candidate
type MomentumCandidate struct {
	Symbol          string                   `json:"symbol"`
	Timestamp       time.Time                `json:"timestamp"`
	MomentumResult  *momentum.MomentumResult `json:"momentum_result"`
	EntrySignal     *momentum.EntrySignal    `json:"entry_signal,omitempty"`
	OrthogonalScore float64                  `json:"orthogonal_score"`
	Qualified       bool                     `json:"qualified"`
	Reason          string                   `json:"reason,omitempty"`
	Attribution     Attribution              `json:"attribution"`
}

// Attribution contains explainability data
type Attribution struct {
	DataSources    []string      `json:"data_sources"`
	ProcessingTime time.Duration `json:"processing_time"`
	Methodology    string        `json:"methodology"`
	Confidence     float64       `json:"confidence"`
	GuardsPassed   []string      `json:"guards_passed"`
	GuardsFailed   []string      `json:"guards_failed"`
}

// DataProvider provides market data for momentum analysis
type DataProvider interface {
	GetMarketData(ctx context.Context, symbol string, timeframes []string) (map[string][]momentum.MarketData, error)
	GetVolumeData(ctx context.Context, symbol string, periods int) ([]float64, error)
	GetRegimeData(ctx context.Context) (string, error)
}

// NewMomentumPipeline creates a new momentum scanning pipeline
func NewMomentumPipeline(config MomentumPipelineConfig, dataProvider DataProvider, outputDir string) *MomentumPipeline {
	momentumCore := momentum.NewMomentumCore(config.Momentum)
	orthogonalizer := momentum.NewGramSchmidtOrthogonalizer(config.Pipeline.ProtectedFactors)
	entryExitGates := momentum.NewEntryExitGates(config.EntryExit)

	return &MomentumPipeline{
		momentumCore:   momentumCore,
		orthogonalizer: orthogonalizer,
		entryExitGates: entryExitGates,
		config:         config,
		outputDir:      outputDir,
	}
}

// SetProgressBus sets the progress streaming bus
func (mp *MomentumPipeline) SetProgressBus(progressBus *progress.ScanProgressBus) {
	mp.progressBus = progressBus
}

// ScanMomentum performs momentum scanning across symbols
func (mp *MomentumPipeline) ScanMomentum(ctx context.Context, symbols []string, dataProvider DataProvider) ([]*MomentumCandidate, error) {
	startTime := time.Now()
	var candidates []*MomentumCandidate

	// Emit scan start event
	if mp.progressBus != nil {
		mp.progressBus.ScanStart("momentum", symbols)
	}

	// Emit initialization event
	mp.emitProgressEvent("init", "", "start", 0, len(symbols), 0, "Initializing momentum pipeline", nil, "")

	// Limit symbols if configured
	if mp.config.Pipeline.MaxSymbols > 0 && len(symbols) > mp.config.Pipeline.MaxSymbols {
		symbols = symbols[:mp.config.Pipeline.MaxSymbols]
	}

	// Get current regime
	mp.emitProgressEvent("fetch", "", "start", 5, len(symbols), 0, "Fetching regime data", nil, "")
	regime, err := dataProvider.GetRegimeData(ctx)
	if err != nil {
		mp.emitProgressEvent("fetch", "", "error", 5, len(symbols), 0, "Failed to get regime data", nil, err.Error())
		regime = "unknown"
	} else {
		mp.emitProgressEvent("fetch", "", "success", 10, len(symbols), 0, "Regime data fetched", map[string]interface{}{"regime": regime}, "")
	}

	// Process each symbol
	mp.emitProgressEvent("analyze", "", "start", 10, len(symbols), 0, "Starting symbol analysis", nil, "")
	for i, symbol := range symbols {
		mp.emitProgressEvent("analyze", symbol, "progress", 10+int(float64(i)/float64(len(symbols))*60), len(symbols), i+1, "Processing symbol", nil, "")

		candidate, err := mp.processSingleSymbol(ctx, symbol, regime, dataProvider, startTime)
		if err != nil {
			mp.emitProgressEvent("analyze", symbol, "error", 10+int(float64(i)/float64(len(symbols))*60), len(symbols), i+1, "Symbol processing failed", nil, err.Error())
			continue
		}

		if candidate != nil {
			candidates = append(candidates, candidate)
			mp.emitProgressEvent("analyze", symbol, "success", 10+int(float64(i)/float64(len(symbols))*60), len(symbols), i+1, "Symbol analyzed", map[string]interface{}{
				"momentum_score": candidate.MomentumResult.CoreScore,
				"qualified":      candidate.Qualified,
			}, "")
		} else {
			mp.emitProgressEvent("analyze", symbol, "success", 10+int(float64(i)/float64(len(symbols))*60), len(symbols), i+1, "Symbol analyzed (no candidate)", nil, "")
		}
	}

	// Apply orthogonalization if we have multiple candidates
	mp.emitProgressEvent("orthogonalize", "", "start", 70, 1, 0, "Starting orthogonalization", map[string]interface{}{"candidate_count": len(candidates)}, "")
	if len(candidates) > 1 {
		err = mp.applyOrthogonalization(candidates)
		if err != nil {
			mp.emitProgressEvent("orthogonalize", "", "error", 70, 1, 0, "Orthogonalization failed", nil, err.Error())
			return nil, fmt.Errorf("orthogonalization failed: %w", err)
		}
		mp.emitProgressEvent("orthogonalize", "", "success", 80, 1, 1, "Orthogonalization completed", nil, "")
	} else {
		mp.emitProgressEvent("orthogonalize", "", "success", 80, 1, 1, "Orthogonalization skipped (insufficient candidates)", nil, "")
	}

	// Filter candidates based on qualification
	mp.emitProgressEvent("filter", "", "start", 80, 1, 0, "Filtering qualified candidates", nil, "")
	qualifiedCandidates := mp.filterQualifiedCandidates(candidates)
	mp.emitProgressEvent("filter", "", "success", 90, 1, 1, "Candidate filtering completed", map[string]interface{}{
		"qualified_count": len(qualifiedCandidates),
		"total_count":     len(candidates),
	}, "")

	// Generate explainability output
	if mp.config.Pipeline.ExplainabilityOutput {
		mp.emitProgressEvent("complete", "", "start", 90, 1, 0, "Generating explainability output", nil, "")
		err = mp.generateExplainabilityOutput(qualifiedCandidates, startTime)
		if err != nil {
			mp.emitProgressEvent("complete", "", "error", 90, 1, 0, "Explainability output failed", nil, err.Error())
			return nil, fmt.Errorf("explainability output failed: %w", err)
		}
		mp.emitProgressEvent("complete", "", "success", 100, 1, 1, "Explainability output completed", nil, "")
	}

	// Emit scan completion
	if mp.progressBus != nil {
		mp.progressBus.ScanComplete(len(qualifiedCandidates), filepath.Join(mp.outputDir, "scan", "momentum_explain.json"))
	}

	return qualifiedCandidates, nil
}

// processSingleSymbol processes momentum analysis for a single symbol
func (mp *MomentumPipeline) processSingleSymbol(ctx context.Context, symbol string, regime string, dataProvider DataProvider, startTime time.Time) (*MomentumCandidate, error) {
	symbolStartTime := time.Now()

	// Get market data for all required timeframes
	timeframes := []string{"1h", "4h", "12h", "24h"}
	marketData, err := dataProvider.GetMarketData(ctx, symbol, timeframes)
	if err != nil {
		return nil, fmt.Errorf("failed to get market data for %s: %w", symbol, err)
	}

	// Get volume data
	volumeData, err := dataProvider.GetVolumeData(ctx, symbol, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to get volume data for %s: %w", symbol, err)
	}

	// Calculate momentum
	momentumResult, err := mp.momentumCore.Calculate(ctx, symbol, marketData, regime)
	if err != nil {
		return nil, fmt.Errorf("momentum calculation failed for %s: %w", symbol, err)
	}

	// Evaluate entry conditions
	entrySignal := mp.entryExitGates.EvaluateEntry(momentumResult, marketData, volumeData)

	// Create candidate
	candidate := &MomentumCandidate{
		Symbol:          symbol,
		Timestamp:       time.Now(),
		MomentumResult:  momentumResult,
		EntrySignal:     entrySignal,
		OrthogonalScore: momentumResult.CoreScore, // Will be updated by orthogonalization
		Qualified:       entrySignal.Qualified,
		Attribution: Attribution{
			DataSources:    []string{"market_data", "volume_data", "regime_data"},
			ProcessingTime: time.Since(symbolStartTime),
			Methodology:    "multi_timeframe_momentum_v3.2.1",
			Confidence:     mp.calculateConfidence(momentumResult, entrySignal),
		},
	}

	// Set qualification reason
	if candidate.Qualified {
		candidate.Reason = "momentum and entry conditions satisfied"
		candidate.Attribution.GuardsPassed = mp.getPassedGuards(momentumResult, entrySignal)
	} else {
		candidate.Reason = entrySignal.Reason
		candidate.Attribution.GuardsFailed = mp.getFailedGuards(momentumResult, entrySignal)
	}

	return candidate, nil
}

// applyOrthogonalization applies Gram-Schmidt orthogonalization with MomentumCore protection
func (mp *MomentumPipeline) applyOrthogonalization(candidates []*MomentumCandidate) error {
	if len(candidates) < 2 {
		return nil
	}

	// Build factor matrix
	factorMatrix := momentum.FactorMatrix{
		Symbols: make([]string, len(candidates)),
		Factors: []string{"MomentumCore", "TechnicalResidual", "VolumeResidual", "QualityResidual"},
		Data:    make([][]float64, len(candidates)),
	}

	// Populate matrix with candidate scores
	for i, candidate := range candidates {
		factorMatrix.Symbols[i] = candidate.Symbol
		factorMatrix.Data[i] = []float64{
			candidate.MomentumResult.CoreScore,
			candidate.MomentumResult.CoreScore * 0.8,                // Simplified technical residual
			candidate.EntrySignal.GateResults.VolumeGate.Value * 10, // Volume component
			50.0, // Simplified quality component
		}
	}

	// Apply orthogonalization
	orthogonalResult, err := mp.orthogonalizer.Orthogonalize(factorMatrix)
	if err != nil {
		return err
	}

	// Update candidates with orthogonalized scores
	for i, candidate := range candidates {
		if i < len(orthogonalResult.OrthogonalMatrix.Data) {
			// MomentumCore is protected, so use it as the orthogonal score
			candidate.OrthogonalScore = orthogonalResult.OrthogonalMatrix.Data[i][0]

			// Update attribution with orthogonalization info
			candidate.Attribution.DataSources = append(candidate.Attribution.DataSources, "gram_schmidt_orthogonalization")
			candidate.Attribution.Methodology += "_with_orthogonalization"
		}
	}

	return nil
}

// filterQualifiedCandidates filters candidates based on qualification status
func (mp *MomentumPipeline) filterQualifiedCandidates(candidates []*MomentumCandidate) []*MomentumCandidate {
	var qualified []*MomentumCandidate

	for _, candidate := range candidates {
		if candidate.Qualified {
			qualified = append(qualified, candidate)
		}
	}

	return qualified
}

// generateExplainabilityOutput generates detailed explainability artifacts
func (mp *MomentumPipeline) generateExplainabilityOutput(candidates []*MomentumCandidate, startTime time.Time) error {
	// Create output directory if it doesn't exist
	outputPath := filepath.Join(mp.outputDir, "scan")
	err := os.MkdirAll(outputPath, 0755)
	if err != nil {
		return err
	}

	// Create explainability report
	explainReport := map[string]interface{}{
		"scan_metadata": map[string]interface{}{
			"timestamp":        time.Now().Format(time.RFC3339),
			"processing_time":  time.Since(startTime).String(),
			"total_candidates": len(candidates),
			"methodology":      "CryptoRun MomentumCore v3.2.1 with Gram-Schmidt orthogonalization",
		},
		"configuration": map[string]interface{}{
			"momentum_config":   mp.config.Momentum,
			"entry_exit_config": mp.config.EntryExit,
			"pipeline_config":   mp.config.Pipeline,
		},
		"candidates": candidates,
		"summary": map[string]interface{}{
			"qualified_count": len(candidates),
			"avg_score":       mp.calculateAverageScore(candidates),
			"top_performer":   mp.getTopPerformer(candidates),
		},
	}

	// Write explainability JSON
	outputFile := filepath.Join(outputPath, "momentum_explain.json")
	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(explainReport)
}

// calculateConfidence calculates confidence score for a candidate
func (mp *MomentumPipeline) calculateConfidence(momentumResult *momentum.MomentumResult, entrySignal *momentum.EntrySignal) float64 {
	confidence := 0.0

	// Base confidence from momentum score (0-50%)
	if momentumResult.CoreScore > 0 {
		confidence += min(momentumResult.CoreScore/10.0*50.0, 50.0)
	}

	// Guards passing adds confidence (0-30%)
	guardsPassedCount := 0
	if momentumResult.GuardResults.Fatigue.Pass {
		guardsPassedCount++
	}
	if momentumResult.GuardResults.Freshness.Pass {
		guardsPassedCount++
	}
	if momentumResult.GuardResults.LateFill.Pass {
		guardsPassedCount++
	}
	confidence += float64(guardsPassedCount) / 3.0 * 30.0

	// Entry gates passing adds confidence (0-20%)
	entryGatesPassedCount := 0
	if entrySignal.GateResults.ScoreGate.Pass {
		entryGatesPassedCount++
	}
	if entrySignal.GateResults.VolumeGate.Pass {
		entryGatesPassedCount++
	}
	if entrySignal.GateResults.ADXGate.Pass {
		entryGatesPassedCount++
	}
	if entrySignal.GateResults.HurstGate.Pass {
		entryGatesPassedCount++
	}
	confidence += float64(entryGatesPassedCount) / 4.0 * 20.0

	return min(confidence, 100.0)
}

// getPassedGuards returns list of passed guards
func (mp *MomentumPipeline) getPassedGuards(momentumResult *momentum.MomentumResult, entrySignal *momentum.EntrySignal) []string {
	var passed []string

	if momentumResult.GuardResults.Fatigue.Pass {
		passed = append(passed, "fatigue_guard")
	}
	if momentumResult.GuardResults.Freshness.Pass {
		passed = append(passed, "freshness_guard")
	}
	if momentumResult.GuardResults.LateFill.Pass {
		passed = append(passed, "late_fill_guard")
	}
	if entrySignal.GateResults.ScoreGate.Pass {
		passed = append(passed, "score_gate")
	}
	if entrySignal.GateResults.VolumeGate.Pass {
		passed = append(passed, "volume_gate")
	}
	if entrySignal.GateResults.ADXGate.Pass {
		passed = append(passed, "adx_gate")
	}
	if entrySignal.GateResults.HurstGate.Pass {
		passed = append(passed, "hurst_gate")
	}

	return passed
}

// getFailedGuards returns list of failed guards
func (mp *MomentumPipeline) getFailedGuards(momentumResult *momentum.MomentumResult, entrySignal *momentum.EntrySignal) []string {
	var failed []string

	if !momentumResult.GuardResults.Fatigue.Pass {
		failed = append(failed, "fatigue_guard")
	}
	if !momentumResult.GuardResults.Freshness.Pass {
		failed = append(failed, "freshness_guard")
	}
	if !momentumResult.GuardResults.LateFill.Pass {
		failed = append(failed, "late_fill_guard")
	}
	if !entrySignal.GateResults.ScoreGate.Pass {
		failed = append(failed, "score_gate")
	}
	if !entrySignal.GateResults.VolumeGate.Pass {
		failed = append(failed, "volume_gate")
	}
	if !entrySignal.GateResults.ADXGate.Pass {
		failed = append(failed, "adx_gate")
	}
	if !entrySignal.GateResults.HurstGate.Pass {
		failed = append(failed, "hurst_gate")
	}

	return failed
}

// calculateAverageScore calculates average score across candidates
func (mp *MomentumPipeline) calculateAverageScore(candidates []*MomentumCandidate) float64 {
	if len(candidates) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, candidate := range candidates {
		sum += candidate.OrthogonalScore
	}
	return sum / float64(len(candidates))
}

// getTopPerformer returns the top performing candidate
func (mp *MomentumPipeline) getTopPerformer(candidates []*MomentumCandidate) string {
	if len(candidates) == 0 {
		return "none"
	}

	topScore := candidates[0].OrthogonalScore
	topSymbol := candidates[0].Symbol

	for _, candidate := range candidates[1:] {
		if candidate.OrthogonalScore > topScore {
			topScore = candidate.OrthogonalScore
			topSymbol = candidate.Symbol
		}
	}

	return topSymbol
}

// min returns minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// emitProgressEvent emits a progress event if progress bus is available
func (mp *MomentumPipeline) emitProgressEvent(phase, symbol, status string, progressPct, total, current int, message string, metrics map[string]interface{}, errorMsg string) {
	if mp.progressBus == nil {
		return
	}

	event := progress.ScanEvent{
		Phase:    phase,
		Symbol:   symbol,
		Status:   status,
		Progress: progressPct,
		Total:    total,
		Current:  current,
		Message:  message,
		Metrics:  metrics,
		Error:    errorMsg,
	}

	mp.progressBus.ScanEvent(event)
}
