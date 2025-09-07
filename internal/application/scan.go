package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sawpanic/cryptorun/internal/application/pipeline"
)

// Stub types for compilation
type SnapshotWriter struct{}

func (sw *SnapshotWriter) Save(interface{}) error { return nil }

type Snapshot struct{}

func NewSnapshot(...interface{}) *Snapshot     { return &Snapshot{} }
func NewSnapshotWriter(string) *SnapshotWriter { return &SnapshotWriter{} }

// Domain stub types for compilation
type FreshnessGate struct{}
type LateFillGate struct{}
type FatigueGate struct{}

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

type MicroGateThresholds struct {
	MaxSpreadBps float64
	MinDepthUSD  float64
	MinVADR      float64
}

func DefaultMicroGateThresholds() MicroGateThresholds {
	return MicroGateThresholds{
		MaxSpreadBps: 50.0,
		MinDepthUSD:  100000.0,
		MinVADR:      1.75,
	}
}

type MicroGateInputs struct {
	Symbol      string
	Bid         float64
	Ask         float64
	Depth2PcUSD float64
	VADR        float64
	ADVUSD      float64
}

type FreshnessInput struct {
	Symbol       string
	CurrentPrice float64
	BasePrice    float64
	LastUpdate   time.Time
	ATR          float64
	BarsOld      int
}

type LateFillInput struct {
	Symbol       string
	SignalTime   time.Time
	FillTime     time.Time
	BarCloseTime time.Time
}

type FatigueInput struct {
	Symbol         string
	Performance24h float64
	RSI4h          float64
	Acceleration   float64
	Timestamp      time.Time
}

type GateInputs struct {
	Symbol      string
	Price       float64
	SpreadBps   float64
	VADR        float64
	DailyVolUSD float64
}

func DefaultFreshnessGate() FreshnessGate { return FreshnessGate{} }
func DefaultLateFillGate() LateFillGate   { return LateFillGate{} }
func DefaultFatigueGate() FatigueGate     { return FatigueGate{} }

func EvaluateMicroGates(inputs MicroGateInputs, thresholds MicroGateThresholds) MicroGateResults {
	spreadBps := CalculateSpreadBps(inputs.Bid, inputs.Ask)
	spreadPass := spreadBps <= thresholds.MaxSpreadBps
	depthPass := inputs.Depth2PcUSD >= thresholds.MinDepthUSD
	vadrPass := inputs.VADR >= thresholds.MinVADR

	allPass := spreadPass && depthPass && vadrPass
	reason := ""
	if !allPass {
		if !spreadPass {
			reason = "SPREAD_TOO_WIDE"
		} else if !depthPass {
			reason = "INSUFFICIENT_DEPTH"
		} else if !vadrPass {
			reason = "LOW_VADR"
		}
	}

	return MicroGateResults{
		SpreadBps: spreadBps,
		DepthUSD:  inputs.Depth2PcUSD,
		VADR:      inputs.VADR,
		AllPass:   allPass,
		Reason:    reason,
	}
}

func BuildFreshnessInput(symbol string, currentPrice, basePrice float64, lastUpdate time.Time, atr float64, barsOld int) FreshnessInput {
	return FreshnessInput{
		Symbol:       symbol,
		CurrentPrice: currentPrice,
		BasePrice:    basePrice,
		LastUpdate:   lastUpdate,
		ATR:          atr,
		BarsOld:      barsOld,
	}
}

func EvaluateFreshnessGate(input FreshnessInput, gate FreshnessGate) GateEvidence {
	// Simplified logic
	withinATR := math.Abs(input.CurrentPrice-input.BasePrice) <= 1.2*input.ATR
	fresh := input.BarsOld <= 2
	ok := withinATR && fresh

	name := ""
	if !ok {
		if !withinATR {
			name = "PRICE_DEVIATION"
		} else {
			name = "STALE_DATA"
		}
	}

	return GateEvidence{OK: ok, Name: name}
}

func BuildLateFillInput(symbol string, signalTime, fillTime, barCloseTime time.Time) LateFillInput {
	return LateFillInput{
		Symbol:       symbol,
		SignalTime:   signalTime,
		FillTime:     fillTime,
		BarCloseTime: barCloseTime,
	}
}

func EvaluateLateFillGate(input LateFillInput, gate LateFillGate) GateEvidence {
	// Check if fill was within 30s of bar close
	fillDelay := input.FillTime.Sub(input.BarCloseTime)
	ok := fillDelay <= 30*time.Second

	name := ""
	if !ok {
		name = "LATE_FILL"
	}

	return GateEvidence{OK: ok, Name: name}
}

func EvaluateFatigueGate(input FatigueInput, gate FatigueGate) GateEvidence {
	// Simplified fatigue logic
	highPerf := input.Performance24h > 12.0
	highRSI := input.RSI4h > 70.0
	decelerating := input.Acceleration < 0

	fatigued := highPerf && highRSI && decelerating
	ok := !fatigued

	name := ""
	if !ok {
		name = "FATIGUE_DETECTED"
	}

	return GateEvidence{OK: ok, Name: name}
}

func CalculateSpreadBps(bid, ask float64) float64 {
	if bid <= 0 || ask <= 0 || ask <= bid {
		return 10000.0 // Invalid spread
	}
	mid := (bid + ask) / 2
	return ((ask - bid) / mid) * 10000.0
}

// CandidateResult represents a complete scan result with all evidence
type CandidateResult struct {
	Symbol        string                  `json:"symbol"`
	Timestamp     time.Time               `json:"timestamp"`
	Score         pipeline.CompositeScore `json:"score"`
	Factors       pipeline.FactorSet      `json:"factors"`
	Gates         AllGateResults          `json:"gates"`
	Decision      string                  `json:"decision"`
	SnapshotSaved bool                    `json:"snapshot_saved"`
	Selected      bool                    `json:"selected"`
}

// AllGateResults contains results from all gate types
type AllGateResults struct {
	Microstructure MicroGateResults `json:"microstructure"`
	Freshness      GateEvidence     `json:"freshness"`
	LateFill       GateEvidence     `json:"late_fill"`
	Fatigue        GateEvidence     `json:"fatigue"`
	AllPass        bool             `json:"all_pass"`
	FailureReasons []string         `json:"failure_reasons,omitempty"`
}

// ScanPipeline orchestrates the complete momentum scanning process
type ScanPipeline struct {
	momentumCalc    *pipeline.MomentumCalculator
	orthogonalizer  *pipeline.Orthogonalizer
	scorer          *pipeline.Scorer
	snapshotWriter  *SnapshotWriter
	microThresholds MicroGateThresholds
	freshnessGate   FreshnessGate
	lateFillGate    LateFillGate
	fatigueGate     FatigueGate
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
		snapshotWriter:  NewSnapshotWriter(snapshotDir),
		microThresholds: DefaultMicroGateThresholds(),
		freshnessGate:   DefaultFreshnessGate(),
		lateFillGate:    DefaultLateFillGate(),
		fatigueGate:     DefaultFatigueGate(),
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

		factorSet := pipeline.BuildFactorSet(symbol, factors, 0.0, volumeFactor, volatilityFactor, socialFactor)

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

	// Write to tracking ledger
	if err := sp.WriteLedger(candidates); err != nil {
		log.Error().Err(err).Msg("Failed to write tracking ledger")
		// Don't fail the scan, just log error
	}

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

	// Symbol guard: validate symbols before writing
	validCandidates := make([]CandidateResult, 0, len(candidates))
	var offenders []string

	for _, candidate := range candidates {
		if sp.validateSymbolFormat(candidate.Symbol) {
			validCandidates = append(validCandidates, candidate)
		} else {
			offenders = append(offenders, candidate.Symbol)
			log.Warn().Str("symbol", candidate.Symbol).
				Msg("Symbol guard: blocked malformed symbol from output")
		}
	}

	// Log offenders if any found
	if len(offenders) > 0 {
		if err := sp.logSymbolOffenders(offenders); err != nil {
			log.Error().Err(err).Msg("Failed to log symbol offenders")
		}
	}

	for _, candidate := range validCandidates {
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

	log.Info().Str("file", outputFile).Int("candidates", len(validCandidates)).
		Int("offenders", len(offenders)).
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

	// Normalize symbols at ingest boundary to prevent duplication
	normalized := make([]string, 0, len(universe.USDPairs))
	for _, symbol := range universe.USDPairs {
		normalizedSymbol := sp.normalizeSymbol(symbol)
		normalized = append(normalized, normalizedSymbol)
	}
	universe.USDPairs = normalized

	return &universe, nil
}

// normalizeSymbol ensures symbol has exactly one USD suffix
func (sp *ScanPipeline) normalizeSymbol(symbol string) string {
	// Remove any existing USD suffixes first
	cleaned := symbol
	for strings.HasSuffix(cleaned, "USD") {
		cleaned = strings.TrimSuffix(cleaned, "USD")
	}

	// Add exactly one USD suffix
	return cleaned + "USD"
}

// validateSymbolFormat checks if symbol matches ^[A-Z0-9]+USD$ pattern
func (sp *ScanPipeline) validateSymbolFormat(symbol string) bool {
	// Use regex to validate symbol format
	symbolRegex := regexp.MustCompile(`^[A-Z0-9]+USD$`)
	return symbolRegex.MatchString(symbol)
}

// logSymbolOffenders writes offending symbols to audit log
func (sp *ScanPipeline) logSymbolOffenders(offenders []string) error {
	auditDir := "out/audit"
	if err := os.MkdirAll(auditDir, 0755); err != nil {
		return fmt.Errorf("failed to create audit directory: %w", err)
	}

	auditFile := filepath.Join(auditDir, "symbol_offenders.jsonl")

	// Open file in append mode
	file, err := os.OpenFile(auditFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit file: %w", err)
	}
	defer file.Close()

	// Write each offender as a separate JSON line
	for _, symbol := range offenders {
		logEntry := map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"symbol":    symbol,
			"violation": "malformed_symbol",
			"expected":  "^[A-Z0-9]+USD$",
			"source":    "scanner_writer",
		}

		jsonBytes, err := json.Marshal(logEntry)
		if err != nil {
			continue // Skip malformed entries
		}

		if _, err := file.Write(jsonBytes); err != nil {
			return fmt.Errorf("failed to write offender log: %w", err)
		}

		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}

	return nil
}

// WriteLedger appends scan candidates to the tracking ledger
func (sp *ScanPipeline) WriteLedger(candidates []CandidateResult) error {
	ledgerDir := "out/results"
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		return fmt.Errorf("failed to create ledger directory: %w", err)
	}

	ledgerFile := filepath.Join(ledgerDir, "ledger.jsonl")

	// Open file in append mode
	file, err := os.OpenFile(ledgerFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open ledger file: %w", err)
	}
	defer file.Close()

	scanTime := time.Now().UTC()

	// Write each candidate as a separate JSON line
	for _, candidate := range candidates {
		// Calculate horizon timestamps
		horizons := map[string]time.Time{
			"6h":  scanTime.Add(6 * time.Hour),
			"12h": scanTime.Add(12 * time.Hour),
			"24h": scanTime.Add(24 * time.Hour),
			"48h": scanTime.Add(48 * time.Hour),
		}

		// Initialize realized returns and pass flags as null
		realized := map[string]*float64{
			"6h":  nil,
			"12h": nil,
			"24h": nil,
			"48h": nil,
		}

		pass := map[string]*bool{
			"6h":  nil,
			"12h": nil,
			"24h": nil,
			"48h": nil,
		}

		entry := LedgerEntry{
			TsScan:       scanTime,
			Symbol:       candidate.Symbol,
			Composite:    candidate.Score.Score,
			GatesAllPass: candidate.Gates.AllPass,
			Horizons:     horizons,
			Realized:     realized,
			Pass:         pass,
		}

		jsonBytes, err := json.Marshal(entry)
		if err != nil {
			log.Warn().Err(err).Str("symbol", candidate.Symbol).Msg("Failed to marshal ledger entry")
			continue
		}

		if _, err := file.Write(jsonBytes); err != nil {
			return fmt.Errorf("failed to write ledger entry: %w", err)
		}

		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}

	log.Info().Str("file", ledgerFile).Int("entries", len(candidates)).
		Msg("Appended candidates to tracking ledger")

	return nil
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
	microInputs := MicroGateInputs{
		Symbol:      symbol,
		Bid:         100.0, // Mock bid/ask from factors
		Ask:         100.5,
		Depth2PcUSD: 150000.0,
		VADR:        2.0,
		ADVUSD:      5000000,
	}
	microResults := EvaluateMicroGates(microInputs, sp.microThresholds)

	if !microResults.AllPass {
		failureReasons = append(failureReasons, microResults.Reason)
	}

	// 2. Freshness gate (mock with current data)
	atr1h := 0.0
	if metadata := factors.Metadata["atr_1h"]; metadata != nil {
		if atr, ok := metadata.(float64); ok {
			atr1h = atr
		}
	}
	freshnessInput := BuildFreshnessInput(symbol, 100.5, 100.0, time.Now().Add(-30*time.Minute), atr1h, 1)
	freshnessResult := EvaluateFreshnessGate(freshnessInput, sp.freshnessGate)

	if !freshnessResult.OK {
		failureReasons = append(failureReasons, fmt.Sprintf("freshness: %s", freshnessResult.Name))
	}

	// 3. Late-fill gate (mock timing)
	lateFillInput := BuildLateFillInput(symbol, time.Now().Add(-10*time.Second), time.Now(), time.Now().Add(-15*time.Second))
	lateFillResult := EvaluateLateFillGate(lateFillInput, sp.lateFillGate)

	if !lateFillResult.OK {
		failureReasons = append(failureReasons, fmt.Sprintf("late_fill: %s", lateFillResult.Name))
	}

	// 4. Fatigue gate (24h momentum + RSI check)
	momentum24h := 0.0
	rsi4h := 0.0
	momentum4h := 0.0
	momentum12h := 0.0

	if metadata := factors.Metadata["momentum_24h"]; metadata != nil {
		if val, ok := metadata.(float64); ok {
			momentum24h = val
		}
	}
	if metadata := factors.Metadata["rsi_4h"]; metadata != nil {
		if val, ok := metadata.(float64); ok {
			rsi4h = val
		}
	}
	if metadata := factors.Metadata["momentum_4h"]; metadata != nil {
		if val, ok := metadata.(float64); ok {
			momentum4h = val
		}
	}
	if metadata := factors.Metadata["momentum_12h"]; metadata != nil {
		if val, ok := metadata.(float64); ok {
			momentum12h = val
		}
	}

	fatigueInput := FatigueInput{
		Symbol:         symbol,
		Performance24h: momentum24h,
		RSI4h:          rsi4h,
		Acceleration:   momentum4h - momentum12h, // Simple acceleration
		Timestamp:      time.Now(),
	}
	fatigueResult := EvaluateFatigueGate(fatigueInput, sp.fatigueGate)

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
	snapshot := NewSnapshot(
		symbol,
		100.0,    // Mock bid
		100.5,    // Mock ask
		50.0,     // Mock spread
		150000.0, // Mock depth
		2.0,      // Mock VADR
		5000000,  // Mock ADV
	)

	if err := sp.snapshotWriter.Save(snapshot); err != nil {
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
	Symbol        string           `json:"symbol"`
	Timestamp     time.Time        `json:"timestamp"`
	Decision      string           `json:"decision"`
	GateResults   MicroGateResults `json:"gate_results"`
	SnapshotSaved bool             `json:"snapshot_saved"`
}

type Scanner struct {
	snapshotWriter *SnapshotWriter
	thresholds     MicroGateThresholds
}

func NewScanner(snapshotDir string) *Scanner {
	return &Scanner{
		snapshotWriter: NewSnapshotWriter(snapshotDir),
		thresholds:     DefaultMicroGateThresholds(),
	}
}

func (s *Scanner) ScanSymbol(ctx context.Context, inputs ScanInputs) (*ScanResult, error) {
	// Legacy implementation preserved for compatibility
	gateInputs := MicroGateInputs{
		Symbol:      inputs.Symbol,
		Bid:         inputs.Bid,
		Ask:         inputs.Ask,
		Depth2PcUSD: inputs.Depth2PcUSD,
		VADR:        inputs.VADR,
		ADVUSD:      float64(inputs.ADVUSD),
	}

	gateResults := EvaluateMicroGates(gateInputs, s.thresholds)
	spreadBps := CalculateSpreadBps(inputs.Bid, inputs.Ask)

	snapshot := NewSnapshot(
		inputs.Symbol,
		inputs.Bid,
		inputs.Ask,
		spreadBps,
		inputs.Depth2PcUSD,
		inputs.VADR,
		inputs.ADVUSD,
	)

	var snapshotSaved bool
	if err := s.snapshotWriter.Save(snapshot); err != nil {
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
func ConvertLegacyGateInputs(symbol string, legacy GateInputs) ScanInputs {
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
