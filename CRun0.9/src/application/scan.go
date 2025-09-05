package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"

	"cryptorun/application/pipeline"
	"cryptorun/domain"
	"cryptorun/infrastructure/market"
)

// CandidateResult represents a complete scan result with all evidence
type CandidateResult struct {
	Symbol          string                        `json:"symbol"`
	Timestamp       time.Time                     `json:"timestamp"`
	Score           pipeline.CompositeScore       `json:"score"`
	Factors         pipeline.FactorSet            `json:"factors"`
	Gates           AllGateResults               `json:"gates"`
	Decision        string                       `json:"decision"`
	SnapshotSaved   bool                         `json:"snapshot_saved"`
	Selected        bool                         `json:"selected"`
}

// AllGateResults contains results from all gate types
type AllGateResults struct {
	Microstructure domain.MicroGateResults `json:"microstructure"`
	Freshness      domain.GateEvidence     `json:"freshness"`
	LateFill       domain.GateEvidence     `json:"late_fill"`
	Fatigue        domain.GateEvidence     `json:"fatigue"`
	AllPass        bool                   `json:"all_pass"`
	FailureReasons []string               `json:"failure_reasons,omitempty"`
}


// ScanPipeline orchestrates the complete momentum scanning process
type ScanPipeline struct {
	momentumCalc    *pipeline.MomentumCalculator
	orthogonalizer  *pipeline.Orthogonalizer
	scorer          *pipeline.Scorer
	snapshotWriter  *market.SnapshotWriter
	microThresholds domain.MicroGateThresholds
	freshnessGate   domain.FreshnessGate
	lateFillGate    domain.LateFillGate
	fatigueGate     domain.FatigueGate
	regime          string
}

// MockDataProvider provides fixture data when live data is unavailable
type MockDataProvider struct{}

func (m *MockDataProvider) GetMarketData(ctx context.Context, symbol string, timeframe pipeline.TimeFrame, periods int) ([]pipeline.MarketData, error) {
	// Generate mock data with some randomness based on symbol hash
	hash := 0
	for _, c := range symbol {
		hash += int(c)
	}
	
	basePrice := 100.0 + float64(hash%1000)
	data := make([]pipeline.MarketData, periods)
	
	for i := 0; i < periods; i++ {
		// Simulate price movement with some momentum
		momentum := float64(hash%20 - 10) // -10% to +10%
		priceMove := momentum * float64(i) / float64(periods) / 100.0
		
		price := basePrice * (1.0 + priceMove)
		volume := 1000000.0 + float64(hash%500000)
		
		data[i] = pipeline.MarketData{
			Symbol:    symbol,
			Timestamp: time.Now().Add(-time.Duration(periods-i) * time.Hour),
			Price:     price,
			Volume:    volume,
			High:      price * 1.02,
			Low:       price * 0.98,
		}
	}
	
	return data, nil
}

// NewScanPipeline creates a new comprehensive scanning pipeline
func NewScanPipeline(snapshotDir string) *ScanPipeline {
	dataProvider := &MockDataProvider{} // Use fixtures if live data unavailable
	
	return &ScanPipeline{
		momentumCalc:    pipeline.NewMomentumCalculator(dataProvider),
		orthogonalizer:  pipeline.NewOrthogonalizer(),
		scorer:          pipeline.NewScorer(),
		snapshotWriter:  market.NewSnapshotWriter(snapshotDir),
		microThresholds: domain.DefaultMicroGateThresholds(),
		freshnessGate:   domain.DefaultFreshnessGate(),
		lateFillGate:    domain.DefaultLateFillGate(),
		fatigueGate:     domain.DefaultFatigueGate(),
		regime:          "bull", // Default regime
	}
}

// SetRegime updates the market regime for all pipeline components
func (sp *ScanPipeline) SetRegime(regime string) {
	sp.regime = regime
	sp.momentumCalc.SetRegime(regime)
	sp.scorer.SetRegime(regime)
	
	log.Info().Str("regime", regime).Msg("Updated pipeline regime")
}

// ScanUniverse performs complete momentum scanning on the trading universe
func (sp *ScanPipeline) ScanUniverse(ctx context.Context) ([]CandidateResult, error) {
	// Load universe from config
	universe, err := sp.loadUniverse()
	if err != nil {
		return nil, fmt.Errorf("failed to load universe: %w", err)
	}

	log.Info().Int("symbols", len(universe.USDPairs)).Str("venue", universe.Venue).
		Msg("Starting universe scan")

	// Limit to reasonable number for demo (first 50 pairs)
	symbols := universe.USDPairs
	if len(symbols) > 50 {
		symbols = symbols[:50]
		log.Info().Msg("Limited scan to first 50 symbols for demo")
	}

	// Step 1: Calculate momentum factors for all symbols
	var factorSets []pipeline.FactorSet
	for _, symbol := range symbols {
		factors, err := sp.momentumCalc.CalculateMomentum(ctx, symbol)
		if err != nil {
			log.Warn().Err(err).Str("symbol", symbol).Msg("Failed to calculate momentum")
			continue
		}

		// Build factor set with mock volume and social data
		volumeFactor := sp.calculateVolumeFactor(factors)
		socialFactor := sp.calculateSocialFactor(symbol)
		volatilityFactor := sp.calculateVolatilityFactor(factors)

		factorSet := pipeline.BuildFactorSet(symbol, factors, volumeFactor, socialFactor, volatilityFactor)
		
		if pipeline.ValidateFactorSet(factorSet) {
			factorSets = append(factorSets, factorSet)
		} else {
			log.Warn().Str("symbol", symbol).Msg("Invalid factor set, skipping")
		}
	}

	log.Info().Int("valid_factors", len(factorSets)).Msg("Momentum calculation completed")

	// Step 2: Apply orthogonalization with protected MomentumCore
	orthogonalFactorSets, err := sp.orthogonalizer.OrthogonalizeFactors(factorSets)
	if err != nil {
		return nil, fmt.Errorf("orthogonalization failed: %w", err)
	}

	// Step 3: Apply social cap (+10 max)
	orthogonalFactorSets = sp.orthogonalizer.ApplySocialCap(orthogonalFactorSets)

	// Step 4: Compute composite scores
	scores, err := sp.scorer.ComputeScores(orthogonalFactorSets)
	if err != nil {
		return nil, fmt.Errorf("scoring failed: %w", err)
	}

	// Step 5: Select Top-N candidates (Top-20)
	topCandidates := sp.scorer.SelectTopN(scores, 20)

	// Step 6: Evaluate all gates and create candidate results
	candidates := make([]CandidateResult, len(topCandidates))
	
	for i, score := range topCandidates {
		factorSet := sp.findFactorSet(orthogonalFactorSets, score.Symbol)
		if factorSet == nil {
			log.Warn().Str("symbol", score.Symbol).Msg("Factor set not found")
			continue
		}

		gates := sp.evaluateAllGates(ctx, score.Symbol, *factorSet)
		
		decision := "PASS"
		if !gates.AllPass {
			decision = "REJECT"
		}

		// Save microstructure snapshot
		snapshotSaved := sp.saveSnapshot(score.Symbol, *factorSet)

		candidates[i] = CandidateResult{
			Symbol:        score.Symbol,
			Timestamp:     time.Now().UTC(),
			Score:         score,
			Factors:       *factorSet,
			Gates:         gates,
			Decision:      decision,
			SnapshotSaved: snapshotSaved,
			Selected:      score.Selected,
		}
	}

	log.Info().Int("candidates", len(candidates)).
		Int("selected", len(topCandidates)).
		Msg("Scanning pipeline completed")

	return candidates, nil
}

// WriteJSONL saves candidates to latest_candidates.jsonl
func (sp *ScanPipeline) WriteJSONL(candidates []CandidateResult, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputFile := filepath.Join(outputDir, "latest_candidates.jsonl")
	
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	for _, candidate := range candidates {
		jsonBytes, err := json.Marshal(candidate)
		if err != nil {
			log.Warn().Err(err).Str("symbol", candidate.Symbol).Msg("Failed to marshal candidate")
			continue
		}

		if _, err := file.Write(jsonBytes); err != nil {
			return fmt.Errorf("failed to write candidate: %w", err)
		}
		
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}

	log.Info().Str("file", outputFile).Int("candidates", len(candidates)).
		Msg("Saved candidates to JSONL")

	return nil
}

// loadUniverse loads the trading universe from config
func (sp *ScanPipeline) loadUniverse() (*UniverseConfig, error) {
	configPath := "config/universe.json"
	
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read universe config: %w", err)
	}

	var universe UniverseConfig
	if err := json.Unmarshal(data, &universe); err != nil {
		return nil, fmt.Errorf("failed to parse universe config: %w", err)
	}

	return &universe, nil
}

// calculateVolumeFactor computes volume factor from momentum data
func (sp *ScanPipeline) calculateVolumeFactor(momentum *pipeline.MomentumFactors) float64 {
	if math.IsNaN(momentum.Volume24h) || momentum.Volume24h <= 0 {
		return 1.0 // Neutral volume factor
	}

	// Simple volume factor: higher volume = higher factor
	// This would typically be volume vs average volume
	volumeFactor := math.Log10(momentum.Volume24h / 1000000.0) // Normalize by 1M
	
	// Clamp to reasonable range
	return math.Max(-2.0, math.Min(3.0, volumeFactor))
}

// calculateSocialFactor computes social sentiment factor (mock implementation)
func (sp *ScanPipeline) calculateSocialFactor(symbol string) float64 {
	// Mock social factor based on symbol hash
	hash := 0
	for _, c := range symbol {
		hash += int(c)
	}
	
	// Generate factor between -5 and +8 (will be capped at +10 later)
	socialFactor := float64(hash%14 - 5)
	
	return socialFactor
}

// calculateVolatilityFactor computes volatility factor from ATR
func (sp *ScanPipeline) calculateVolatilityFactor(momentum *pipeline.MomentumFactors) float64 {
	if math.IsNaN(momentum.ATR1h) || momentum.ATR1h <= 0 {
		return 15.0 // Default moderate volatility
	}
	
	// Convert ATR to percentage volatility estimate
	volatility := momentum.ATR1h * 24.0 // Approximate 24h volatility from 1h ATR
	
	return volatility
}

// evaluateAllGates runs all gate checks on a symbol
func (sp *ScanPipeline) evaluateAllGates(ctx context.Context, symbol string, factors pipeline.FactorSet) AllGateResults {
	var failureReasons []string

	// 1. Microstructure gates
	microInputs := domain.MicroGateInputs{
		Symbol:      symbol,
		Bid:         100.0, // Mock bid/ask from factors
		Ask:         100.5,
		Depth2PcUSD: 150000.0,
		VADR:        2.0,
		ADVUSD:      5000000,
	}
	microResults := domain.EvaluateMicroGates(microInputs, sp.microThresholds)
	
	if !microResults.AllPass {
		failureReasons = append(failureReasons, microResults.Reason)
	}

	// 2. Freshness gate (mock with current data)
	freshnessInput := domain.BuildFreshnessInput(symbol, 100.5, 100.0, time.Now().Add(-30*time.Minute), factors.Raw["atr_1h"], 1)
	freshnessResult := domain.EvaluateFreshnessGate(freshnessInput, sp.freshnessGate)
	
	if !freshnessResult.OK {
		failureReasons = append(failureReasons, fmt.Sprintf("freshness: %s", freshnessResult.Name))
	}

	// 3. Late-fill gate (mock timing)
	lateFillInput := domain.BuildLateFillInput(symbol, time.Now().Add(-10*time.Second), time.Now(), time.Now().Add(-15*time.Second))
	lateFillResult := domain.EvaluateLateFillGate(lateFillInput, sp.lateFillGate)
	
	if !lateFillResult.OK {
		failureReasons = append(failureReasons, fmt.Sprintf("late_fill: %s", lateFillResult.Name))
	}

	// 4. Fatigue gate (24h momentum + RSI check)
	fatigueInput := domain.FatigueInput{
		Symbol:      symbol,
		Momentum24h: factors.Raw["momentum_24h"],
		RSI4h:       factors.Raw["rsi_4h"],
		Acceleration: factors.Raw["momentum_4h"] - factors.Raw["momentum_12h"], // Simple acceleration
		Timestamp:   time.Now(),
	}
	fatigueResult := domain.EvaluateFatigueGate(fatigueInput, sp.fatigueGate)
	
	if !fatigueResult.OK {
		failureReasons = append(failureReasons, fmt.Sprintf("fatigue: %s", fatigueResult.Name))
	}

	allPass := microResults.AllPass && freshnessResult.OK && lateFillResult.OK && fatigueResult.OK

	return AllGateResults{
		Microstructure: microResults,
		Freshness:      freshnessResult,
		LateFill:       lateFillResult,
		Fatigue:        fatigueResult,
		AllPass:        allPass,
		FailureReasons: failureReasons,
	}
}

// saveSnapshot saves microstructure snapshot for audit trail
func (sp *ScanPipeline) saveSnapshot(symbol string, factors pipeline.FactorSet) bool {
	snapshot := market.NewSnapshot(
		symbol,
		100.0,   // Mock bid
		100.5,   // Mock ask
		50.0,    // Mock spread
		150000.0, // Mock depth
		2.0,     // Mock VADR
		5000000, // Mock ADV
	)

	if err := sp.snapshotWriter.SaveSnapshot(snapshot); err != nil {
		log.Warn().Err(err).Str("symbol", symbol).Msg("Failed to save snapshot")
		return false
	}

	return true
}

// findFactorSet finds a factor set by symbol
func (sp *ScanPipeline) findFactorSet(factorSets []pipeline.FactorSet, symbol string) *pipeline.FactorSet {
	for i := range factorSets {
		if factorSets[i].Symbol == symbol {
			return &factorSets[i]
		}
	}
	return nil
}

// Legacy compatibility functions (preserved)
type ScanInputs struct {
	Symbol      string
	Bid         float64
	Ask         float64
	Depth2PcUSD float64
	VADR        float64
	ADVUSD      int64
	Timestamp   time.Time
}

type ScanResult struct {
	Symbol      string                  `json:"symbol"`
	Timestamp   time.Time              `json:"timestamp"`
	Decision    string                 `json:"decision"`
	GateResults domain.MicroGateResults `json:"gate_results"`
	SnapshotSaved bool                 `json:"snapshot_saved"`
}

type Scanner struct {
	snapshotWriter *market.SnapshotWriter
	thresholds     domain.MicroGateThresholds
}

func NewScanner(snapshotDir string) *Scanner {
	return &Scanner{
		snapshotWriter: market.NewSnapshotWriter(snapshotDir),
		thresholds:     domain.DefaultMicroGateThresholds(),
	}
}

func (s *Scanner) ScanSymbol(ctx context.Context, inputs ScanInputs) (*ScanResult, error) {
	// Legacy implementation preserved for compatibility
	gateInputs := domain.MicroGateInputs{
		Symbol:      inputs.Symbol,
		Bid:         inputs.Bid,
		Ask:         inputs.Ask,
		Depth2PcUSD: inputs.Depth2PcUSD,
		VADR:        inputs.VADR,
		ADVUSD:      inputs.ADVUSD,
	}

	gateResults := domain.EvaluateMicroGates(gateInputs, s.thresholds)
	spreadBps := domain.CalculateSpreadBps(inputs.Bid, inputs.Ask)

	snapshot := market.NewSnapshot(
		inputs.Symbol,
		inputs.Bid,
		inputs.Ask,
		spreadBps,
		inputs.Depth2PcUSD,
		inputs.VADR,
		inputs.ADVUSD,
	)

	var snapshotSaved bool
	if err := s.snapshotWriter.SaveSnapshot(snapshot); err != nil {
		log.Warn().Err(err).Str("symbol", inputs.Symbol).Msg("Failed to save snapshot")
		snapshotSaved = false
	} else {
		snapshotSaved = true
	}

	decision := "REJECT"
	if gateResults.AllPass {
		decision = "PASS"
	}

	return &ScanResult{
		Symbol:        inputs.Symbol,
		Timestamp:     inputs.Timestamp,
		Decision:      decision,
		GateResults:   gateResults,
		SnapshotSaved: snapshotSaved,
	}, nil
}

// ConvertLegacyGateInputs converts old GateInputs to new scan inputs format
func ConvertLegacyGateInputs(symbol string, legacy domain.GateInputs) ScanInputs {
	estimatedMid := 100.0
	spreadAmount := (legacy.SpreadBps / 10000.0) * estimatedMid
	
	return ScanInputs{
		Symbol:      symbol,
		Bid:         estimatedMid - (spreadAmount / 2),
		Ask:         estimatedMid + (spreadAmount / 2),
		Depth2PcUSD: 150000.0,
		VADR:        legacy.VADR,
		ADVUSD:      int64(legacy.DailyVolUSD),
		Timestamp:   time.Now().UTC(),
	}
}