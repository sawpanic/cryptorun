package scan

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/sawpanic/cryptorun/internal/application"
	"github.com/sawpanic/cryptorun/internal/domain/factors"
	"github.com/sawpanic/cryptorun/internal/config/regime"
	domainregime "github.com/sawpanic/cryptorun/internal/domain/regime"
	"github.com/sawpanic/cryptorun/internal/domain/scoring"
	"github.com/sawpanic/cryptorun/internal/infrastructure/datafacade"
)

// OfflineScanner performs offline cryptocurrency momentum scanning
type OfflineScanner struct {
	dataFacade     *datafacade.DataFacade
	factorBuilder  *factors.FactorBuilder
	regimeDetector *regime.RegimeDetector
	compositeScorer *scoring.CompositeScorer
	config         ScanConfig
}

// ScanConfig configures the offline scanning behavior
type ScanConfig struct {
	// Symbols to scan
	Symbols []string
	
	// Output configuration
	OutputFormat   OutputFormat
	OutputFile     string
	IncludeHeaders bool
	
	// Filtering
	MinScore       float64
	MaxResults     int
	SortBy         SortCriteria
	
	// Attribution detail level
	AttributionLevel AttributionLevel
	
	// Execution options
	DryRun         bool
	Parallel       bool
	MaxConcurrency int
	Timeout        time.Duration
}

// OutputFormat specifies the output format for scan results
type OutputFormat string

const (
	OutputJSON OutputFormat = "json"
	OutputCSV  OutputFormat = "csv"
	OutputTSV  OutputFormat = "tsv"
)

// SortCriteria specifies how to sort scan results
type SortCriteria string

const (
	SortByScore      SortCriteria = "score"
	SortByMomentum   SortCriteria = "momentum"
	SortBySymbol     SortCriteria = "symbol"
	SortByVolume     SortCriteria = "volume"
	SortByTimestamp  SortCriteria = "timestamp"
)

// AttributionLevel controls the detail level of attribution data
type AttributionLevel string

const (
	AttributionMinimal AttributionLevel = "minimal"
	AttributionBasic   AttributionLevel = "basic"
	AttributionFull    AttributionLevel = "full"
	AttributionDebug   AttributionLevel = "debug"
)

// ScanResult represents a single scan result with full attribution
type ScanResult struct {
	// Basic identification
	Symbol    string    `json:"symbol" csv:"symbol"`
	Timestamp time.Time `json:"timestamp" csv:"timestamp"`
	
	// Core scoring
	FinalScore     float64 `json:"final_score" csv:"final_score"`
	Regime         string  `json:"regime" csv:"regime"`
	RegimeConfidence float64 `json:"regime_confidence" csv:"regime_confidence"`
	
	// Factor components (pre-weighting)
	MomentumCore      float64 `json:"momentum_core" csv:"momentum_core"`
	TechnicalResidual float64 `json:"technical_residual" csv:"technical_residual"`
	VolumeResidual    float64 `json:"volume_residual" csv:"volume_residual"`
	QualityResidual   float64 `json:"quality_residual" csv:"quality_residual"`
	SocialCapped      float64 `json:"social_capped" csv:"social_capped"`
	
	// Weighted contributions
	WeightedMomentum  float64 `json:"weighted_momentum" csv:"weighted_momentum"`
	WeightedTechnical float64 `json:"weighted_technical" csv:"weighted_technical"`
	WeightedVolume    float64 `json:"weighted_volume" csv:"weighted_volume"`
	WeightedQuality   float64 `json:"weighted_quality" csv:"weighted_quality"`
	WeightedSocial    float64 `json:"weighted_social" csv:"weighted_social"`
	
	// Weights used
	Weights WeightAttribution `json:"weights" csv:"weights"`
	
	// Quality metrics
	OrthogonalityScore   float64 `json:"orthogonality_score" csv:"orthogonality_score"`
	MomentumPreserved    float64 `json:"momentum_preserved" csv:"momentum_preserved"`
	MaxCorrelation       float64 `json:"max_correlation" csv:"max_correlation"`
	
	// Attribution (included based on AttributionLevel)
	Attribution *Attribution `json:"attribution,omitempty" csv:"-"`
}

// WeightAttribution shows the weights used for each factor
type WeightAttribution struct {
	MomentumCore float64 `json:"momentum_core" csv:"momentum_core"`
	Technical    float64 `json:"technical" csv:"technical"`
	Volume       float64 `json:"volume" csv:"volume"`
	Quality      float64 `json:"quality" csv:"quality"`
	Social       float64 `json:"social" csv:"social"`
}

// Attribution provides detailed attribution and debugging information
type Attribution struct {
	// Data sources
	DataSources   []string `json:"data_sources,omitempty"`
	CacheHits     []string `json:"cache_hits,omitempty"`
	CacheMisses   []string `json:"cache_misses,omitempty"`
	
	// Processing details
	ProcessingTime      time.Duration            `json:"processing_time_ms,omitempty"`
	RegimeDetectionTime time.Duration            `json:"regime_detection_time_ms,omitempty"`
	ScoringTime         time.Duration            `json:"scoring_time_ms,omitempty"`
	
	// Factor breakdown (full attribution only)
	MomentumComponents  map[string]float64 `json:"momentum_components,omitempty"`
	TechnicalSources    map[string]float64 `json:"technical_sources,omitempty"`
	VolumeSources       map[string]float64 `json:"volume_sources,omitempty"`
	QualitySources      map[string]float64 `json:"quality_sources,omitempty"`
	SocialSources       map[string]float64 `json:"social_sources,omitempty"`
	
	// Debug information (debug level only)
	RawFactors         *factors.RawFactorRow           `json:"raw_factors,omitempty"`
	OrthogonalizedFactors *factors.OrthogonalizedFactorRow `json:"orthogonalized_factors,omitempty"`
	RegimeIndicators   []regime.RegimeIndicator        `json:"regime_indicators,omitempty"`
}

// ScanSummary provides overall scan statistics
type ScanSummary struct {
	TotalSymbols     int           `json:"total_symbols"`
	SuccessfulScans  int           `json:"successful_scans"`
	FailedScans      int           `json:"failed_scans"`
	TotalTime        time.Duration `json:"total_time_ms"`
	AverageTime      time.Duration `json:"average_time_ms"`
	CacheHitRate     float64       `json:"cache_hit_rate"`
	RegimeDetected   string        `json:"regime_detected"`
	TopScore         float64       `json:"top_score"`
	BottomScore      float64       `json:"bottom_score"`
	Timestamp        time.Time     `json:"timestamp"`
}

// NewOfflineScanner creates a new offline scanner
func NewOfflineScanner(dataFacade *datafacade.DataFacade, config ScanConfig) *OfflineScanner {
	// Create core components with test configuration
	weightsConfig := application.WeightsConfig{
		DefaultRegime: "normal",
		Validation: struct {
			WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
			MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
			MaxSocialWeight    float64 `yaml:"max_social_weight"`
			SocialHardCap      float64 `yaml:"social_hard_cap"`
		}{
			WeightSumTolerance: 0.05,
			MinMomentumWeight:  0.3,
			MaxSocialWeight:    10.0,
			SocialHardCap:      10.0,
		},
		Regimes: map[string]application.RegimeWeights{
			"calm": {
				Description:       "Low volatility trending market",
				MomentumCore:      0.5,
				TechnicalResidual: 0.2,
				VolumeResidual:    0.2,
				QualityResidual:   0.1,
				SocialResidual:    6.0,
			},
			"normal": {
				Description:       "Mixed volatility market",
				MomentumCore:      0.4,
				TechnicalResidual: 0.3,
				VolumeResidual:    0.2,
				QualityResidual:   0.1,
				SocialResidual:    8.0,
			},
			"volatile": {
				Description:       "High volatility choppy market",
				MomentumCore:      0.6,
				TechnicalResidual: 0.15,
				VolumeResidual:    0.15,
				QualityResidual:   0.1,
				SocialResidual: 4.0,
			},
		},
	}
	
	// Convert application config to regime config format
	regimeWeightsConfig := regime.WeightsConfig{
		Regimes: make(map[string]regime.RegimeWeights),
		Social: regime.SocialConfig{
			MaxContribution:           weightsConfig.Validation.MaxSocialWeight,
			AppliedAfterNormalization: true,
		},
		Validation: regime.ValidationConfig{
			WeightSumTolerance: weightsConfig.Validation.WeightSumTolerance,
			MinWeight:          0.05, // Default minimum weight
			MaxWeight:          0.60, // Default maximum weight  
			SocialHardCap:      weightsConfig.Validation.SocialHardCap,
		},
		QARequirements: regime.QARequirements{
			CorrelationThreshold: 0.3, // Default correlation threshold
		},
	}
	
	// Convert regime weights
	for regimeName, appWeights := range weightsConfig.Regimes {
		regimeWeightsConfig.Regimes[regimeName] = regime.RegimeWeights{
			MomentumCore:      appWeights.MomentumCore,
			TechnicalResid:    appWeights.TechnicalResidual, 
			SupplyDemandBlock: appWeights.VolumeResidual + appWeights.QualityResidual, // Combine volume + quality
			CatalystBlock:     appWeights.SocialResidual / 10.0, // Convert to 0-1 scale
		}
	}
	
	factorBuilder := factors.NewFactorBuilder(regimeWeightsConfig)
	regimeDetector := domainregime.NewRegimeDetector(regimeWeightsConfig)
	compositeScorer := scoring.NewCompositeScorer(regimeWeightsConfig, regimeDetector)
	
	return &OfflineScanner{
		dataFacade:     dataFacade,
		factorBuilder:  factorBuilder,
		regimeDetector: regimeDetector,
		compositeScorer: compositeScorer,
		config:        config,
	}
}

// Scan performs the offline momentum scan
func (scanner *OfflineScanner) Scan(ctx context.Context) (*ScanOutput, error) {
	scanStart := time.Now()
	
	// Get symbols to scan
	symbols := scanner.config.Symbols
	if len(symbols) == 0 {
		symbols = scanner.dataFacade.GetSupportedSymbols()
	}
	
	// Validate symbols
	supportedSymbols := scanner.dataFacade.GetSupportedSymbols()
	validSymbols := []string{}
	for _, symbol := range symbols {
		if contains(supportedSymbols, symbol) {
			validSymbols = append(validSymbols, symbol)
		}
	}
	
	if len(validSymbols) == 0 {
		return nil, fmt.Errorf("no valid symbols to scan")
	}
	
	// Get regime data (market-wide)
	regimeData, err := scanner.dataFacade.GetRegimeData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get regime data: %w", err)
	}
	
	// Scan symbols
	results := []ScanResult{}
	errors := []error{}
	
	if scanner.config.Parallel && scanner.config.MaxConcurrency > 1 {
		results, errors = scanner.scanParallel(ctx, validSymbols, regimeData)
	} else {
		results, errors = scanner.scanSequential(ctx, validSymbols, regimeData)
	}
	
	// Filter and sort results
	filteredResults := scanner.filterResults(results)
	sortedResults := scanner.sortResults(filteredResults)
	
	// Limit results if specified
	if scanner.config.MaxResults > 0 && len(sortedResults) > scanner.config.MaxResults {
		sortedResults = sortedResults[:scanner.config.MaxResults]
	}
	
	// Calculate summary statistics
	summary := scanner.calculateSummary(validSymbols, sortedResults, errors, time.Since(scanStart))
	
	return &ScanOutput{
		Results: sortedResults,
		Summary: summary,
		Errors:  errors,
	}, nil
}

// scanSequential processes symbols one by one
func (scanner *OfflineScanner) scanSequential(ctx context.Context, symbols []string, regimeData *regime.MarketData) ([]ScanResult, []error) {
	results := []ScanResult{}
	errors := []error{}
	
	for _, symbol := range symbols {
		result, err := scanner.scanSymbol(ctx, symbol, regimeData)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to scan %s: %w", symbol, err))
			continue
		}
		results = append(results, *result)
	}
	
	return results, errors
}

// scanParallel processes symbols concurrently (simplified implementation)
func (scanner *OfflineScanner) scanParallel(ctx context.Context, symbols []string, regimeData *regime.MarketData) ([]ScanResult, []error) {
	// For this implementation, fall back to sequential
	// In production, would use worker pools and channels
	return scanner.scanSequential(ctx, symbols, regimeData)
}

// scanSymbol scans a single symbol and returns the result
func (scanner *OfflineScanner) scanSymbol(ctx context.Context, symbol string, regimeData *regime.MarketData) (*ScanResult, error) {
	scanStart := time.Now()
	
	// Get microstructure data
	microData, err := scanner.dataFacade.GetMicrostructureData(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get microstructure data: %w", err)
	}
	
	// Build raw factors
	rawFactors, err := scanner.factorBuilder.BuildFactorRow(symbol, microData, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to build factors: %w", err)
	}
	
	// Calculate composite score
	score, err := scanner.compositeScorer.CalculateCompositeScore(*rawFactors, *regimeData)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate composite score: %w", err)
	}
	
	// Build scan result with attribution
	result := ScanResult{
		Symbol:            symbol,
		Timestamp:         time.Now(),
		FinalScore:        score.FinalScore,
		Regime:            string(score.Regime),
		RegimeConfidence:  score.ScoringMetadata.RegimeConfidence,
		MomentumCore:      score.MomentumCore,
		TechnicalResidual: score.TechnicalResidual,
		VolumeResidual:    score.VolumeResidual,
		QualityResidual:   score.QualityResidual,
		SocialCapped:      score.SocialCapped,
		WeightedMomentum:  score.WeightedMomentum,
		WeightedTechnical: score.WeightedTechnical,
		WeightedVolume:    score.WeightedVolume,
		WeightedQuality:   score.WeightedQuality,
		WeightedSocial:    score.WeightedSocial,
		Weights: WeightAttribution{
			MomentumCore: score.Weights.MomentumCore,
			Technical:    score.Weights.Technical,
			Volume:       score.Weights.Volume,
			Quality:      score.Weights.Quality,
			Social:       score.Weights.Social,
		},
		OrthogonalityScore: score.FactorBreakdown.OrthogonalizationQuality.OrthogonalityScore,
		MomentumPreserved:  score.FactorBreakdown.OrthogonalizationQuality.MomentumPreserved,
		MaxCorrelation:     score.FactorBreakdown.OrthogonalizationQuality.MaxCorrelation,
	}
	
	// Add attribution based on level
	if scanner.config.AttributionLevel != AttributionMinimal {
		result.Attribution = scanner.buildAttribution(score, rawFactors, time.Since(scanStart))
	}
	
	return &result, nil
}

// buildAttribution creates attribution information based on configuration level
func (scanner *OfflineScanner) buildAttribution(score *scoring.CompositeScore, rawFactors *factors.RawFactorRow, processingTime time.Duration) *Attribution {
	attribution := &Attribution{
		DataSources:    score.ScoringMetadata.DataSources,
		ProcessingTime: processingTime,
	}
	
	if scanner.config.AttributionLevel == AttributionBasic || scanner.config.AttributionLevel == AttributionFull {
		// Add timing breakdowns
		attribution.ScoringTime = processingTime // Simplified - would track individual timings
	}
	
	if scanner.config.AttributionLevel == AttributionFull {
		// Add factor breakdowns
		attribution.MomentumComponents = score.FactorBreakdown.MomentumComponents
		attribution.TechnicalSources = score.FactorBreakdown.TechnicalSources
		attribution.VolumeSources = score.FactorBreakdown.VolumeSources
		attribution.QualitySources = score.FactorBreakdown.QualitySources
		attribution.SocialSources = score.FactorBreakdown.SocialSources
	}
	
	if scanner.config.AttributionLevel == AttributionDebug {
		// Add debug information
		attribution.RawFactors = rawFactors
		// attribution.OrthogonalizedFactors would be available from score
		// attribution.RegimeIndicators would be available from regime detection
	}
	
	return attribution
}

// filterResults applies filtering criteria to scan results
func (scanner *OfflineScanner) filterResults(results []ScanResult) []ScanResult {
	filtered := []ScanResult{}
	
	for _, result := range results {
		if result.FinalScore >= scanner.config.MinScore {
			filtered = append(filtered, result)
		}
	}
	
	return filtered
}

// sortResults sorts scan results according to configured criteria
func (scanner *OfflineScanner) sortResults(results []ScanResult) []ScanResult {
	sorted := make([]ScanResult, len(results))
	copy(sorted, results)
	
	sort.Slice(sorted, func(i, j int) bool {
		switch scanner.config.SortBy {
		case SortByScore:
			return sorted[i].FinalScore > sorted[j].FinalScore // Descending
		case SortByMomentum:
			return sorted[i].MomentumCore > sorted[j].MomentumCore
		case SortBySymbol:
			return sorted[i].Symbol < sorted[j].Symbol
		case SortByVolume:
			return sorted[i].VolumeResidual > sorted[j].VolumeResidual
		case SortByTimestamp:
			return sorted[i].Timestamp.After(sorted[j].Timestamp)
		default:
			return sorted[i].FinalScore > sorted[j].FinalScore
		}
	})
	
	return sorted
}

// calculateSummary generates scan summary statistics
func (scanner *OfflineScanner) calculateSummary(symbols []string, results []ScanResult, errors []error, totalTime time.Duration) ScanSummary {
	summary := ScanSummary{
		TotalSymbols:    len(symbols),
		SuccessfulScans: len(results),
		FailedScans:     len(errors),
		TotalTime:       totalTime,
		Timestamp:       time.Now(),
	}
	
	if len(results) > 0 {
		summary.AverageTime = totalTime / time.Duration(len(results))
		summary.TopScore = results[0].FinalScore
		summary.BottomScore = results[len(results)-1].FinalScore
		summary.RegimeDetected = results[0].Regime
	}
	
	// Get cache hit rate from data facade
	cacheStats := scanner.dataFacade.GetCacheStats()
	summary.CacheHitRate = cacheStats.HitRate
	
	return summary
}

// ScanOutput contains the complete scan results
type ScanOutput struct {
	Results []ScanResult `json:"results"`
	Summary ScanSummary  `json:"summary"`
	Errors  []error      `json:"errors,omitempty"`
}

// WriteOutput writes scan results to the specified output format
func (scanner *OfflineScanner) WriteOutput(output *ScanOutput, filename string) error {
	if scanner.config.DryRun {
		fmt.Printf("DRY RUN: Would write %d results to %s\n", len(output.Results), filename)
		return nil
	}
	
	switch scanner.config.OutputFormat {
	case OutputJSON:
		return scanner.writeJSON(output, filename)
	case OutputCSV:
		return scanner.writeCSV(output.Results, filename)
	case OutputTSV:
		return scanner.writeTSV(output.Results, filename)
	default:
		return fmt.Errorf("unsupported output format: %s", scanner.config.OutputFormat)
	}
}

// writeJSON writes results as JSON
func (scanner *OfflineScanner) writeJSON(output *ScanOutput, filename string) error {
	file, err := scanner.createOutputFile(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// writeCSV writes results as CSV
func (scanner *OfflineScanner) writeCSV(results []ScanResult, filename string) error {
	return scanner.writeDelimited(results, filename, ',')
}

// writeTSV writes results as TSV
func (scanner *OfflineScanner) writeTSV(results []ScanResult, filename string) error {
	return scanner.writeDelimited(results, filename, '\t')
}

// writeDelimited writes results in delimited format (CSV/TSV)
func (scanner *OfflineScanner) writeDelimited(results []ScanResult, filename string, delimiter rune) error {
	file, err := scanner.createOutputFile(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	writer.Comma = delimiter
	defer writer.Flush()
	
	// Write headers if requested
	if scanner.config.IncludeHeaders {
		headers := []string{
			"symbol", "timestamp", "final_score", "regime", "regime_confidence",
			"momentum_core", "technical_residual", "volume_residual", "quality_residual", "social_capped",
			"weighted_momentum", "weighted_technical", "weighted_volume", "weighted_quality", "weighted_social",
			"weight_momentum", "weight_technical", "weight_volume", "weight_quality", "weight_social",
			"orthogonality_score", "momentum_preserved", "max_correlation",
		}
		if err := writer.Write(headers); err != nil {
			return err
		}
	}
	
	// Write data rows
	for _, result := range results {
		row := []string{
			result.Symbol,
			result.Timestamp.Format(time.RFC3339),
			fmt.Sprintf("%.2f", result.FinalScore),
			result.Regime,
			fmt.Sprintf("%.1f", result.RegimeConfidence),
			fmt.Sprintf("%.2f", result.MomentumCore),
			fmt.Sprintf("%.2f", result.TechnicalResidual),
			fmt.Sprintf("%.2f", result.VolumeResidual),
			fmt.Sprintf("%.2f", result.QualityResidual),
			fmt.Sprintf("%.2f", result.SocialCapped),
			fmt.Sprintf("%.2f", result.WeightedMomentum),
			fmt.Sprintf("%.2f", result.WeightedTechnical),
			fmt.Sprintf("%.2f", result.WeightedVolume),
			fmt.Sprintf("%.2f", result.WeightedQuality),
			fmt.Sprintf("%.2f", result.WeightedSocial),
			fmt.Sprintf("%.3f", result.Weights.MomentumCore),
			fmt.Sprintf("%.3f", result.Weights.Technical),
			fmt.Sprintf("%.3f", result.Weights.Volume),
			fmt.Sprintf("%.3f", result.Weights.Quality),
			fmt.Sprintf("%.3f", result.Weights.Social),
			fmt.Sprintf("%.1f", result.OrthogonalityScore),
			fmt.Sprintf("%.1f", result.MomentumPreserved),
			fmt.Sprintf("%.3f", result.MaxCorrelation),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	
	return nil
}

// createOutputFile creates the output file, handling stdout special case
func (scanner *OfflineScanner) createOutputFile(filename string) (*os.File, error) {
	if filename == "" || filename == "-" || filename == "stdout" {
		return os.Stdout, nil
	}
	
	return os.Create(filename)
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// DefaultScanConfig returns a reasonable default scan configuration
func DefaultScanConfig() ScanConfig {
	return ScanConfig{
		Symbols:          []string{}, // Empty = scan all supported
		OutputFormat:     OutputJSON,
		OutputFile:       "",         // Empty = stdout
		IncludeHeaders:   true,
		MinScore:         0.0,        // No minimum filter
		MaxResults:       50,         // Top 50 results
		SortBy:           SortByScore,
		AttributionLevel: AttributionBasic,
		DryRun:           false,
		Parallel:         false,
		MaxConcurrency:   4,
		Timeout:          30 * time.Second,
	}
}