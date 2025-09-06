package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cryptorun/internal/algo/dip"
	"cryptorun/internal/atomicio"
	"cryptorun/internal/domain"
)

// DipScanConfig contains pipeline configuration
type DipScanConfig struct {
	Trend          dip.TrendConfig          `yaml:"trend"`
	Fib            dip.FibConfig            `yaml:"fib"`
	RSI            dip.RSIConfig            `yaml:"rsi"`
	Volume         dip.VolumeConfig         `yaml:"volume"`
	Microstructure dip.MicrostructureConfig `yaml:"microstructure"`
	Scoring        ScoringConfig            `yaml:"scoring"`
	Decay          dip.TimeDecayConfig      `yaml:"decay"`
	Guards         dip.GuardsConfig         `yaml:"guards"`
}

// ScoringConfig contains composite scoring parameters
type ScoringConfig struct {
	CoreWeight    float64 `yaml:"core_w"`
	VolumeWeight  float64 `yaml:"vol_w"`
	QualityWeight float64 `yaml:"qual_w"`
	BrandCap      int     `yaml:"brand_cap_pts"`
	Threshold     float64 `yaml:"threshold"`
}

// DipCandidate represents a qualified dip opportunity
type DipCandidate struct {
	Symbol         string              `json:"symbol"`
	Exchange       string              `json:"exchange"`
	Timestamp      time.Time           `json:"timestamp"`
	TrendResult    *dip.TrendResult    `json:"trend_result"`
	DipPoint       *dip.DipPoint       `json:"dip_point"`
	QualityScore   *dip.QualityMetrics `json:"quality_score"`
	CompositeScore float64             `json:"composite_score"`
	GuardResult    *dip.GuardResult    `json:"guard_result"`
	Entry          *EntrySignal        `json:"entry,omitempty"`
	Attribution    DipAttribution      `json:"attribution"`
}

// EntrySignal represents validated entry conditions
type EntrySignal struct {
	Price      float64   `json:"price"`
	Timestamp  time.Time `json:"timestamp"`
	Confidence float64   `json:"confidence"`
	StopLoss   float64   `json:"stop_loss"`
	TakeProfit []float64 `json:"take_profit"`
	RiskReward float64   `json:"risk_reward"`
}

// DipAttribution contains explainability data for dip analysis
type DipAttribution struct {
	TrendSource    string               `json:"trend_source"`
	VolumeSource   string               `json:"volume_source"`
	MicroSource    string               `json:"micro_source"`
	SocialSource   string               `json:"social_source,omitempty"`
	ProcessingTime time.Duration        `json:"processing_time"`
	DataTimestamps map[string]time.Time `json:"data_timestamps"`
	QualityChecks  []string             `json:"quality_checks"`
}

// DipDataProvider provides market data for dip analysis
type DipDataProvider interface {
	GetMarketData(ctx context.Context, symbol string, timeframe string, periods int) ([]dip.MarketData, error)
	GetMicrostructureData(ctx context.Context, symbol string) (*domain.MicroGateInputs, error)
	GetSocialData(ctx context.Context, symbol string) (*dip.SocialData, error)
}

// DipPipeline orchestrates dip detection and qualification
type DipPipeline struct {
	config          DipScanConfig
	dataProvider    DipDataProvider
	dipCore         *dip.DipCore
	qualityAnalyzer *dip.QualityAnalyzer
	guards          *dip.DipGuards
	outputDir       string
}

// NewDipPipeline creates a new dip scanning pipeline
func NewDipPipeline(config DipScanConfig, dataProvider DipDataProvider, outputDir string) *DipPipeline {
	dipCore := dip.NewDipCore(config.Trend, config.Fib, config.RSI)
	qualityAnalyzer := dip.NewQualityAnalyzer(config.Volume, config.Microstructure, config.Scoring.BrandCap)
	guards := dip.NewDipGuards(config.Guards)

	return &DipPipeline{
		config:          config,
		dataProvider:    dataProvider,
		dipCore:         dipCore,
		qualityAnalyzer: qualityAnalyzer,
		guards:          guards,
		outputDir:       outputDir,
	}
}

// ScanForDips performs comprehensive dip analysis for given symbols
func (dp *DipPipeline) ScanForDips(ctx context.Context, symbols []string) ([]*DipCandidate, error) {
	startTime := time.Now()
	var candidates []*DipCandidate

	for _, symbol := range symbols {
		candidate, err := dp.analyzeSymbol(ctx, symbol, startTime)
		if err != nil {
			// Log error but continue with other symbols
			continue
		}

		if candidate != nil {
			candidates = append(candidates, candidate)
		}
	}

	// Write explainability output
	if err := dp.writeExplainabilityOutput(candidates, startTime); err != nil {
		return candidates, fmt.Errorf("failed to write explainability output: %w", err)
	}

	return candidates, nil
}

// analyzeSymbol performs complete dip analysis for a single symbol
func (dp *DipPipeline) analyzeSymbol(ctx context.Context, symbol string, pipelineStart time.Time) (*DipCandidate, error) {
	symbolStart := time.Now()

	// Gather market data for all timeframes
	data1h, err := dp.dataProvider.GetMarketData(ctx, symbol, "1h", 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get 1h data: %w", err)
	}

	data12h, err := dp.dataProvider.GetMarketData(ctx, symbol, "12h", 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get 12h data: %w", err)
	}

	data24h, err := dp.dataProvider.GetMarketData(ctx, symbol, "24h", 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get 24h data: %w", err)
	}

	data4h, err := dp.dataProvider.GetMarketData(ctx, symbol, "4h", 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get 4h data: %w", err)
	}

	if len(data1h) == 0 {
		return nil, fmt.Errorf("no market data available")
	}

	currentPrice := data1h[len(data1h)-1].Close

	// Step 1: Trend qualification (must pass)
	trendResult, err := dp.dipCore.QualifyTrend(ctx, data12h, data24h, data4h, currentPrice)
	if err != nil {
		return nil, fmt.Errorf("trend qualification failed: %w", err)
	}

	if !trendResult.Qualified {
		return nil, nil // Not in uptrend - skip
	}

	// Step 2: Dip identification
	dipPoint, err := dp.dipCore.IdentifyDip(ctx, data1h, trendResult)
	if err != nil {
		return nil, fmt.Errorf("dip identification failed: %w", err)
	}

	if dipPoint == nil {
		return nil, nil // No dip found - skip
	}

	// Step 3: Quality analysis
	microInputs, err := dp.dataProvider.GetMicrostructureData(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("microstructure data failed: %w", err)
	}

	socialData, _ := dp.dataProvider.GetSocialData(ctx, symbol) // Optional

	qualityMetrics, err := dp.qualityAnalyzer.AnalyzeQuality(ctx, symbol, dipPoint, microInputs, data1h, socialData)
	if err != nil {
		return nil, fmt.Errorf("quality analysis failed: %w", err)
	}

	// Step 4: Composite scoring
	compositeScore := dp.calculateCompositeScore(trendResult, dipPoint, qualityMetrics)

	// Check score threshold
	if compositeScore < dp.config.Scoring.Threshold {
		return nil, nil // Below threshold - skip
	}

	// Step 5: Guard validation
	guardResult, err := dp.guards.ValidateEntry(ctx, dipPoint, data1h, time.Now())
	if err != nil {
		return nil, fmt.Errorf("guard validation failed: %w", err)
	}

	if !guardResult.Passed {
		return nil, nil // Guards failed - skip
	}

	// Create candidate with full attribution
	candidate := &DipCandidate{
		Symbol:         symbol,
		Exchange:       "kraken", // Default exchange
		Timestamp:      time.Now(),
		TrendResult:    trendResult,
		DipPoint:       dipPoint,
		QualityScore:   qualityMetrics,
		CompositeScore: compositeScore,
		GuardResult:    guardResult,
		Attribution:    dp.buildAttribution(symbol, symbolStart, pipelineStart, microInputs, socialData),
	}

	// Generate entry signal if all conditions met
	if compositeScore >= dp.config.Scoring.Threshold && guardResult.Passed {
		entry := dp.generateEntrySignal(dipPoint, data1h, compositeScore)
		candidate.Entry = entry
	}

	return candidate, nil
}

// calculateCompositeScore computes weighted score from all components
func (dp *DipPipeline) calculateCompositeScore(trend *dip.TrendResult, dipPoint *dip.DipPoint, quality *dip.QualityMetrics) float64 {
	// Core dip score based on technical strength
	coreScore := dp.calculateCoreScore(trend, dipPoint)

	// Volume score from quality metrics
	volumeScore := 0.0
	if quality.Volume.Qualified {
		volumeScore = (quality.Volume.VolumeRatio / dp.config.Volume.ADVMultMin) * 25
		if volumeScore > 25 {
			volumeScore = 25
		}
	}

	// Quality score (already calculated)
	qualityScore := quality.Score

	// Brand/social score (capped in quality metrics)
	brandScore := quality.Brand.CappedScore

	// Weighted composite
	composite := (coreScore * dp.config.Scoring.CoreWeight) +
		(volumeScore * dp.config.Scoring.VolumeWeight) +
		(qualityScore * dp.config.Scoring.QualityWeight) +
		brandScore

	// Normalize to 0-100 scale
	if composite > 100 {
		composite = 100
	}

	return composite
}

// calculateCoreScore computes core technical score
func (dp *DipPipeline) calculateCoreScore(trend *dip.TrendResult, dipPoint *dip.DipPoint) float64 {
	score := 0.0

	// Trend strength component (up to 30 points)
	if trend.PriceAboveMA {
		score += 10
	}
	if trend.MA12hSlope > 0 {
		score += 5
	}
	if trend.MA24hSlope > 0 {
		score += 5
	}
	if trend.ADX4h >= dp.config.Trend.ADX4hMin {
		score += 5
	}
	if trend.Hurst > dp.config.Trend.HurstMin {
		score += 5
	}

	// Dip quality component (up to 20 points)
	if dipPoint.HasDivergence {
		score += 10
	}
	if dipPoint.HasEngulfing {
		score += 5
	}
	if dipPoint.FibLevel >= dp.config.Fib.Min && dipPoint.FibLevel <= dp.config.Fib.Max {
		score += 5
	}

	return score
}

// generateEntrySignal creates entry conditions for validated dip
func (dp *DipPipeline) generateEntrySignal(dipPoint *dip.DipPoint, data1h []dip.MarketData, confidence float64) *EntrySignal {
	if len(data1h) == 0 {
		return nil
	}

	currentPrice := data1h[len(data1h)-1].Close

	// Stop loss below dip low with buffer
	stopLoss := dipPoint.Price * 0.98

	// Take profit levels based on Fibonacci extensions
	takeProfits := []float64{
		currentPrice * 1.02, // 2%
		currentPrice * 1.05, // 5%
		currentPrice * 1.08, // 8%
	}

	// Risk-reward ratio
	risk := currentPrice - stopLoss
	reward := takeProfits[0] - currentPrice
	riskReward := 0.0
	if risk > 0 {
		riskReward = reward / risk
	}

	return &EntrySignal{
		Price:      currentPrice,
		Timestamp:  time.Now(),
		Confidence: confidence / 100.0, // Normalize to 0-1
		StopLoss:   stopLoss,
		TakeProfit: takeProfits,
		RiskReward: riskReward,
	}
}

// buildAttribution creates complete attribution record
func (dp *DipPipeline) buildAttribution(symbol string, symbolStart, pipelineStart time.Time,
	microInputs *domain.MicroGateInputs, socialData *dip.SocialData) DipAttribution {

	dataTimestamps := make(map[string]time.Time)
	dataTimestamps["pipeline_start"] = pipelineStart
	dataTimestamps["symbol_start"] = symbolStart
	dataTimestamps["completed"] = time.Now()

	qualityChecks := []string{
		"trend_qualification",
		"dip_identification",
		"liquidity_validation",
		"volume_confirmation",
		"guard_validation",
	}

	attribution := DipAttribution{
		TrendSource:    "kraken_native_ohlc",
		VolumeSource:   "kraken_native_volume",
		MicroSource:    "kraken_l1_orderbook",
		ProcessingTime: time.Since(symbolStart),
		DataTimestamps: dataTimestamps,
		QualityChecks:  qualityChecks,
	}

	if socialData != nil {
		attribution.SocialSource = "coingecko_social_metrics"
	}

	return attribution
}

// writeExplainabilityOutput writes detailed analysis to JSON
func (dp *DipPipeline) writeExplainabilityOutput(candidates []*DipCandidate, startTime time.Time) error {
	if dp.outputDir == "" {
		return nil
	}

	// Ensure output directory exists
	scanDir := filepath.Join(dp.outputDir, "scan")
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		return fmt.Errorf("failed to create scan directory: %w", err)
	}

	// Create explainability report
	report := ExplainabilityReport{
		Timestamp:       time.Now(),
		ProcessingTime:  time.Since(startTime),
		TotalCandidates: len(candidates),
		Candidates:      candidates,
		Config:          dp.config,
		Summary:         dp.buildSummary(candidates),
	}

	// Write to JSON file
	outputPath := filepath.Join(scanDir, "dip_explain.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal explainability report: %w", err)
	}

	return atomicio.WriteFile(outputPath, data, 0644)
}

// ExplainabilityReport contains complete analysis results
type ExplainabilityReport struct {
	Timestamp       time.Time       `json:"timestamp"`
	ProcessingTime  time.Duration   `json:"processing_time"`
	TotalCandidates int             `json:"total_candidates"`
	Candidates      []*DipCandidate `json:"candidates"`
	Config          DipScanConfig   `json:"config"`
	Summary         ReportSummary   `json:"summary"`
}

// ReportSummary contains high-level analysis summary
type ReportSummary struct {
	QualifiedTrends   int     `json:"qualified_trends"`
	DetectedDips      int     `json:"detected_dips"`
	PassedGuards      int     `json:"passed_guards"`
	AvgCompositeScore float64 `json:"avg_composite_score"`
	TopSymbol         string  `json:"top_symbol"`
	TopScore          float64 `json:"top_score"`
}

// buildSummary creates analysis summary
func (dp *DipPipeline) buildSummary(candidates []*DipCandidate) ReportSummary {
	if len(candidates) == 0 {
		return ReportSummary{}
	}

	totalScore := 0.0
	topScore := 0.0
	topSymbol := ""

	for _, candidate := range candidates {
		totalScore += candidate.CompositeScore
		if candidate.CompositeScore > topScore {
			topScore = candidate.CompositeScore
			topSymbol = candidate.Symbol
		}
	}

	return ReportSummary{
		QualifiedTrends:   len(candidates), // All candidates have qualified trends
		DetectedDips:      len(candidates), // All candidates have detected dips
		PassedGuards:      len(candidates), // All candidates passed guards
		AvgCompositeScore: totalScore / float64(len(candidates)),
		TopSymbol:         topSymbol,
		TopScore:          topScore,
	}
}
