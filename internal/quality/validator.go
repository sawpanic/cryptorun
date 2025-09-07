package quality

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DataEnvelope interface defines the data structure to be validated
type DataEnvelope interface {
	GetSymbol() string
	GetVenue() string
	GetTimestamp() time.Time
	GetSourceTier() string
	GetPriceData() map[string]interface{}
	GetVolumeData() map[string]interface{}
	GetOrderBook() map[string]interface{}
	GetProvenance() ProvenanceInfo
}

// ProvenanceInfo holds data provenance information
type ProvenanceInfo struct {
	OriginalSource    string    `json:"original_source"`
	RetrievedAt       time.Time `json:"retrieved_at"`
	ConfidenceScore   float64   `json:"confidence_score"`
	CacheHit          bool      `json:"cache_hit"`
	FallbackChain     []string  `json:"fallback_chain,omitempty"`
}

// QualityConfig holds validation and anomaly detection configuration
type QualityConfig struct {
	MaxStalenessSeconds    map[string]int                   `yaml:"max_staleness_seconds"`
	MinCompletenessPercent int                              `yaml:"min_completeness_percent"`
	ExpectedDataPoints     map[string]int                   `yaml:"expected_data_points"`
	MaxPriceChangePercent  float64                          `yaml:"max_price_change_percent"`
	VolumeSpikeThreshold   float64                          `yaml:"volume_spike_threshold"`
	AnomalyDetection       AnomalyDetectionConfig           `yaml:"anomaly_detection"`
	Validation             ValidationConfig                 `yaml:"validation"`
	Scoring                ScoringConfig                    `yaml:"scoring"`
	Alerting               AlertingConfig                   `yaml:"alerting"`
}

// AnomalyDetectionConfig defines anomaly detection settings
type AnomalyDetectionConfig struct {
	Enable           bool                   `yaml:"enable"`
	WindowSize       int                    `yaml:"window_size"`       // Hours
	Sensitivity      float64                `yaml:"sensitivity"`       // Standard deviations
	MinDataPoints    int                    `yaml:"min_data_points"`
	PriceAnomalies   PriceAnomalyConfig     `yaml:"price_anomalies"`
	VolumeAnomalies  VolumeAnomalyConfig    `yaml:"volume_anomalies"`
	SpreadAnomalies  SpreadAnomalyConfig    `yaml:"spread_anomalies"`
}

// PriceAnomalyConfig defines price anomaly detection
type PriceAnomalyConfig struct {
	Enable                 bool    `yaml:"enable"`
	MaxDeviationPercent    float64 `yaml:"max_deviation_percent"`
	GapThresholdPercent    float64 `yaml:"gap_threshold_percent"`
	FlatlineDuration       int     `yaml:"flatline_duration"` // Seconds
}

// VolumeAnomalyConfig defines volume anomaly detection
type VolumeAnomalyConfig struct {
	Enable                bool    `yaml:"enable"`
	SpikeMultiplier       float64 `yaml:"spike_multiplier"`
	DroughtThreshold      float64 `yaml:"drought_threshold"`
	ZeroVolumeTolerance   int     `yaml:"zero_volume_tolerance"` // Seconds
}

// SpreadAnomalyConfig defines spread anomaly detection
type SpreadAnomalyConfig struct {
	Enable               bool    `yaml:"enable"`
	MaxSpreadBps         int     `yaml:"max_spread_bps"`
	SpreadSpikeMultiplier float64 `yaml:"spread_spike_multiplier"`
}

// ValidationConfig defines validation gate settings
type ValidationConfig struct {
	Enable              bool              `yaml:"enable"`
	FailFast            bool              `yaml:"fail_fast"`
	QuarantineThreshold int               `yaml:"quarantine_threshold"`
	RecoveryThreshold   int               `yaml:"recovery_threshold"`
	Schema              SchemaConfig      `yaml:"schema"`
	Types               TypeValidationConfig `yaml:"types"`
}

// SchemaConfig defines required fields
type SchemaConfig struct {
	RequireOHLCV     bool `yaml:"require_ohlcv"`
	RequireTimestamp bool `yaml:"require_timestamp"`
	RequireVenue     bool `yaml:"require_venue"`
	RequireSymbol    bool `yaml:"require_symbol"`
}

// TypeValidationConfig defines data type validation
type TypeValidationConfig struct {
	NumericPrecision int      `yaml:"numeric_precision"`
	TimestampFormat  string   `yaml:"timestamp_format"`
	SymbolRegex      string   `yaml:"symbol_regex"`
	VenueWhitelist   []string `yaml:"venue_whitelist"`
}

// ScoringConfig defines quality scoring weights and thresholds
type ScoringConfig struct {
	Enable     bool                   `yaml:"enable"`
	Weights    ScoringWeights         `yaml:"weights"`
	Thresholds ScoringThresholds      `yaml:"thresholds"`
}

// ScoringWeights defines component weights for quality score
type ScoringWeights struct {
	Freshness    float64 `yaml:"freshness"`
	Completeness float64 `yaml:"completeness"`
	Consistency  float64 `yaml:"consistency"`
	AnomalyFree  float64 `yaml:"anomaly_free"`
}

// ScoringThresholds defines quality level thresholds
type ScoringThresholds struct {
	Excellent  int `yaml:"excellent"`
	Good       int `yaml:"good"`
	Acceptable int `yaml:"acceptable"`
	Poor       int `yaml:"poor"`
}

// AlertingConfig defines alerting settings
type AlertingConfig struct {
	Enable                  bool     `yaml:"enable"`
	Channels                []string `yaml:"channels"`
	QualityDegradation      int      `yaml:"quality_degradation"`
	AnomalyBurst            int      `yaml:"anomaly_burst"`
	ValidationFailureRate   float64  `yaml:"validation_failure_rate"`
	SuppressDuplicates      bool     `yaml:"suppress_duplicates"`
	SuppressionWindow       string   `yaml:"suppression_window"`
	EscalationThreshold     int      `yaml:"escalation_threshold"`
}

// ValidationResult holds validation outcome
type ValidationResult struct {
	Valid         bool                   `json:"valid"`
	Errors        []string               `json:"errors,omitempty"`
	Warnings      []string               `json:"warnings,omitempty"`
	QualityScore  float64                `json:"quality_score"`
	QualityLevel  string                 `json:"quality_level"`
	Anomalies     []Anomaly              `json:"anomalies,omitempty"`
	Metrics       ValidationMetrics      `json:"metrics"`
	Timestamp     time.Time              `json:"timestamp"`
}

// Anomaly represents a detected data anomaly
type Anomaly struct {
	Type        string                 `json:"type"`        // "price", "volume", "spread"
	Severity    string                 `json:"severity"`    // "low", "medium", "high", "critical"
	Description string                 `json:"description"`
	Value       float64                `json:"value"`
	Threshold   float64                `json:"threshold"`
	Timestamp   time.Time              `json:"timestamp"`
	Context     map[string]interface{} `json:"context,omitempty"`
}

// ValidationMetrics holds validation performance metrics
type ValidationMetrics struct {
	FreshnessScore    float64 `json:"freshness_score"`
	CompletenessScore float64 `json:"completeness_score"`
	ConsistencyScore  float64 `json:"consistency_score"`
	AnomalyFreeScore  float64 `json:"anomaly_free_score"`
	ProcessingTimeMs  float64 `json:"processing_time_ms"`
	DataPointsChecked int     `json:"data_points_checked"`
}

// DataValidator handles data validation and anomaly detection
type DataValidator struct {
	config           QualityConfig
	symbolRegex      *regexp.Regexp
	historicalData   map[string][]DataEnvelope // Symbol -> historical data
	validationCounts map[string]*ValidationCounts // Symbol -> counts
	mu               sync.RWMutex
	metricsCallback  func(string, float64)
}

// ValidationCounts tracks validation statistics per symbol
type ValidationCounts struct {
	TotalValidations   int
	FailedValidations  int
	ConsecutiveFails   int
	ConsecutiveSuccess int
	Quarantined       bool
	LastValidation    time.Time
}

// NewDataValidator creates a new data validator
func NewDataValidator(config QualityConfig) (*DataValidator, error) {
	symbolRegex, err := regexp.Compile(config.Validation.Types.SymbolRegex)
	if err != nil {
		return nil, fmt.Errorf("invalid symbol regex: %w", err)
	}

	return &DataValidator{
		config:           config,
		symbolRegex:      symbolRegex,
		historicalData:   make(map[string][]DataEnvelope),
		validationCounts: make(map[string]*ValidationCounts),
	}, nil
}

// SetMetricsCallback sets the metrics reporting callback
func (v *DataValidator) SetMetricsCallback(callback func(string, float64)) {
	v.metricsCallback = callback
}

// ValidateEnvelope validates a single envelope
func (v *DataValidator) ValidateEnvelope(ctx context.Context, envelope DataEnvelope) (*ValidationResult, error) {
	startTime := time.Now()
	
	result := &ValidationResult{
		Valid:     true,
		Errors:    make([]string, 0),
		Warnings:  make([]string, 0),
		Anomalies: make([]Anomaly, 0),
		Timestamp: time.Now(),
	}

	// Update historical data
	v.updateHistoricalData(envelope)

	// Schema validation
	if v.config.Validation.Enable {
		v.validateSchema(envelope, result)
		v.validateTypes(envelope, result)
	}

	// Store envelope in historical data BEFORE anomaly detection
	v.storeHistoricalData(envelope)

	// Quality metrics calculation
	if v.config.Scoring.Enable {
		v.calculateQualityMetrics(envelope, result)
		v.calculateQualityScore(result)
	}

	// Anomaly detection (uses historical data)
	if v.config.AnomalyDetection.Enable {
		v.detectAnomalies(envelope, result)
	}

	// Update validation counts
	v.updateValidationCounts(envelope.GetSymbol(), result.Valid)

	// Processing time
	result.Metrics.ProcessingTimeMs = float64(time.Since(startTime).Nanoseconds()) / 1000000
	result.Metrics.DataPointsChecked = 1

	// Report metrics
	if v.metricsCallback != nil {
		v.reportMetrics(result)
	}

	return result, nil
}

// ValidateBatch validates multiple envelopes
func (v *DataValidator) ValidateBatch(ctx context.Context, envelopes []DataEnvelope) ([]*ValidationResult, error) {
	results := make([]*ValidationResult, len(envelopes))
	
	for i, envelope := range envelopes {
		result, err := v.ValidateEnvelope(ctx, envelope)
		if err != nil {
			return nil, fmt.Errorf("validation failed for envelope %d: %w", i, err)
		}
		results[i] = result
	}

	// Aggregate metrics for batch
	v.reportBatchMetrics(results)

	return results, nil
}

// validateSchema checks required fields are present
func (v *DataValidator) validateSchema(envelope DataEnvelope, result *ValidationResult) {
	if v.config.Validation.Schema.RequireSymbol && envelope.GetSymbol() == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "missing required field: symbol")
	}

	if v.config.Validation.Schema.RequireVenue && envelope.GetVenue() == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "missing required field: venue")
	}

	if v.config.Validation.Schema.RequireTimestamp && envelope.GetTimestamp().IsZero() {
		result.Valid = false
		result.Errors = append(result.Errors, "missing required field: timestamp")
	}

	if v.config.Validation.Schema.RequireOHLCV {
		priceData := envelope.GetPriceData()
		if priceData == nil {
			result.Valid = false
			result.Errors = append(result.Errors, "missing required field: price_data")
		} else {
			requiredFields := []string{"open", "high", "low", "close"}
			for _, field := range requiredFields {
				if _, exists := priceData[field]; !exists {
					result.Valid = false
					result.Errors = append(result.Errors, fmt.Sprintf("missing required OHLCV field: %s", field))
				}
			}
		}

		volumeData := envelope.GetVolumeData()
		if volumeData == nil {
			result.Valid = false
			result.Errors = append(result.Errors, "missing required field: volume_data")
		} else if _, exists := volumeData["volume"]; !exists {
			result.Valid = false
			result.Errors = append(result.Errors, "missing required volume field: volume")
		}
	}
}

// validateTypes checks data type constraints
func (v *DataValidator) validateTypes(envelope DataEnvelope, result *ValidationResult) {
	// Symbol validation
	if !v.symbolRegex.MatchString(envelope.GetSymbol()) {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("symbol '%s' doesn't match required pattern", envelope.GetSymbol()))
	}

	// Venue whitelist validation
	venueValid := false
	for _, venue := range v.config.Validation.Types.VenueWhitelist {
		if strings.ToLower(envelope.GetVenue()) == venue {
			venueValid = true
			break
		}
	}
	if !venueValid {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("venue '%s' not in whitelist", envelope.GetVenue()))
	}

	// Numeric precision validation
	priceData := envelope.GetPriceData()
	if priceData != nil {
		for field, value := range priceData {
			if floatVal, ok := value.(float64); ok {
				if v.exceedsPrecision(floatVal, v.config.Validation.Types.NumericPrecision) {
					result.Warnings = append(result.Warnings, 
						fmt.Sprintf("price field '%s' exceeds precision limit", field))
				}
			}
		}
	}
}

// calculateQualityMetrics computes individual quality metric scores
func (v *DataValidator) calculateQualityMetrics(envelope DataEnvelope, result *ValidationResult) {
	// Freshness score
	result.Metrics.FreshnessScore = v.calculateFreshnessScore(envelope)
	
	// Completeness score  
	result.Metrics.CompletenessScore = v.calculateCompletenessScore(envelope)
	
	// Consistency score
	result.Metrics.ConsistencyScore = v.calculateConsistencyScore(envelope)
	
	// Anomaly-free score (will be calculated after anomaly detection)
	result.Metrics.AnomalyFreeScore = 100.0 // Default, adjusted after anomaly detection
}

// calculateFreshnessScore computes freshness score based on age
func (v *DataValidator) calculateFreshnessScore(envelope DataEnvelope) float64 {
	tierKey := strings.ToLower(envelope.GetSourceTier())
	maxStalenessSec, exists := v.config.MaxStalenessSeconds[tierKey]
	if !exists {
		maxStalenessSec = 300 // Default 5 minutes
	}

	age := time.Since(envelope.GetTimestamp())
	ageSeconds := age.Seconds()

	if ageSeconds <= float64(maxStalenessSec) {
		return 100.0 * (1.0 - ageSeconds/float64(maxStalenessSec))
	}

	return 0.0 // Too stale
}

// calculateCompletenessScore computes completeness score
func (v *DataValidator) calculateCompletenessScore(envelope DataEnvelope) float64 {
	requiredFields := []string{"Symbol", "Venue", "Timestamp"}
	presentFields := 0
	totalFields := len(requiredFields)

	if envelope.GetSymbol() != "" {
		presentFields++
	}
	if envelope.GetVenue() != "" {
		presentFields++
	}
	if !envelope.GetTimestamp().IsZero() {
		presentFields++
	}

	// Check OHLCV if required
	if v.config.Validation.Schema.RequireOHLCV {
		ohlcFields := []string{"open", "high", "low", "close"}
		totalFields += len(ohlcFields) + 1 // +1 for volume

		priceData := envelope.GetPriceData()
		if priceData != nil {
			for _, field := range ohlcFields {
				if _, exists := priceData[field]; exists {
					presentFields++
				}
			}
		}

		volumeData := envelope.GetVolumeData()
		if volumeData != nil {
			if _, exists := volumeData["volume"]; exists {
				presentFields++
			}
		}
	}

	return 100.0 * float64(presentFields) / float64(totalFields)
}

// calculateConsistencyScore computes consistency score
func (v *DataValidator) calculateConsistencyScore(envelope DataEnvelope) float64 {
	score := 100.0

	// Check price consistency (high >= low, etc.)
	priceData := envelope.GetPriceData()
	if priceData != nil {
		open, hasOpen := priceData["open"].(float64)
		high, hasHigh := priceData["high"].(float64) 
		low, hasLow := priceData["low"].(float64)
		close, hasClose := priceData["close"].(float64)

		if hasHigh && hasLow && high < low {
			score -= 25 // Major inconsistency
		}

		if hasOpen && hasHigh && open > high {
			score -= 15
		}

		if hasOpen && hasLow && open < low {
			score -= 15
		}

		if hasClose && hasHigh && close > high {
			score -= 15
		}

		if hasClose && hasLow && close < low {
			score -= 15
		}
	}

	return math.Max(0, score)
}

// calculateQualityScore combines individual metric scores
func (v *DataValidator) calculateQualityScore(result *ValidationResult) {
	weights := v.config.Scoring.Weights
	metrics := &result.Metrics

	result.QualityScore = 
		weights.Freshness*metrics.FreshnessScore +
		weights.Completeness*metrics.CompletenessScore +
		weights.Consistency*metrics.ConsistencyScore +
		weights.AnomalyFree*metrics.AnomalyFreeScore

	// Determine quality level
	thresholds := v.config.Scoring.Thresholds
	switch {
	case result.QualityScore >= float64(thresholds.Excellent):
		result.QualityLevel = "excellent"
	case result.QualityScore >= float64(thresholds.Good):
		result.QualityLevel = "good"
	case result.QualityScore >= float64(thresholds.Acceptable):
		result.QualityLevel = "acceptable"
	case result.QualityScore >= float64(thresholds.Poor):
		result.QualityLevel = "poor"
	default:
		result.QualityLevel = "critical"
	}
}

// detectAnomalies runs anomaly detection algorithms
func (v *DataValidator) detectAnomalies(envelope DataEnvelope, result *ValidationResult) {
	anomalies := make([]Anomaly, 0)

	if v.config.AnomalyDetection.PriceAnomalies.Enable {
		priceAnomalies := v.detectPriceAnomalies(envelope)
		anomalies = append(anomalies, priceAnomalies...)
	}

	if v.config.AnomalyDetection.VolumeAnomalies.Enable {
		volumeAnomalies := v.detectVolumeAnomalies(envelope)
		anomalies = append(anomalies, volumeAnomalies...)
	}

	if v.config.AnomalyDetection.SpreadAnomalies.Enable {
		spreadAnomalies := v.detectSpreadAnomalies(envelope)
		anomalies = append(anomalies, spreadAnomalies...)
	}

	result.Anomalies = anomalies

	// Adjust anomaly-free score
	if len(anomalies) > 0 {
		penalty := math.Min(20.0*float64(len(anomalies)), 80.0) // Max 80% penalty
		result.Metrics.AnomalyFreeScore = math.Max(0, 100.0-penalty)
	}
}

// detectPriceAnomalies detects price-based anomalies
func (v *DataValidator) detectPriceAnomalies(envelope DataEnvelope) []Anomaly {
	anomalies := make([]Anomaly, 0)
	
	priceData := envelope.GetPriceData()
	if priceData == nil {
		return anomalies
	}

	closePrice, hasClose := priceData["close"].(float64)
	if !hasClose {
		return anomalies
	}

	// Get historical data for baseline
	v.mu.RLock()
	historical := v.historicalData[envelope.GetSymbol()]
	v.mu.RUnlock()

	if len(historical) < v.config.AnomalyDetection.MinDataPoints {
		return anomalies // Not enough data for anomaly detection
	}

	// Calculate moving average
	var priceSum float64
	validPrices := 0
	for _, hist := range historical {
		histPriceData := hist.GetPriceData()
		if histPriceData != nil {
			if price, ok := histPriceData["close"].(float64); ok {
				priceSum += price
				validPrices++
			}
		}
	}

	if validPrices == 0 {
		return anomalies
	}

	avgPrice := priceSum / float64(validPrices)
	deviation := math.Abs(closePrice-avgPrice) / avgPrice * 100

	// Check for price deviation anomaly
	if deviation > v.config.AnomalyDetection.PriceAnomalies.MaxDeviationPercent {
		anomalies = append(anomalies, Anomaly{
			Type:        "price",
			Severity:    v.determineSeverity(deviation, v.config.AnomalyDetection.PriceAnomalies.MaxDeviationPercent),
			Description: fmt.Sprintf("Price deviation %.2f%% exceeds threshold %.2f%%", deviation, v.config.AnomalyDetection.PriceAnomalies.MaxDeviationPercent),
			Value:       deviation,
			Threshold:   v.config.AnomalyDetection.PriceAnomalies.MaxDeviationPercent,
			Timestamp:   envelope.GetTimestamp(),
			Context: map[string]interface{}{
				"current_price": closePrice,
				"average_price": avgPrice,
				"symbol":       envelope.GetSymbol(),
			},
		})
	}

	return anomalies
}

// detectVolumeAnomalies detects volume-based anomalies
func (v *DataValidator) detectVolumeAnomalies(envelope DataEnvelope) []Anomaly {
	anomalies := make([]Anomaly, 0)

	volumeData := envelope.GetVolumeData()
	if volumeData == nil {
		return anomalies
	}

	volume, hasVolume := volumeData["volume"].(float64)
	if !hasVolume {
		return anomalies
	}

	// Get historical data for baseline
	v.mu.RLock()
	historical := v.historicalData[envelope.GetSymbol()]
	v.mu.RUnlock()

	if len(historical) < v.config.AnomalyDetection.MinDataPoints {
		return anomalies
	}

	// Calculate average volume
	var volumeSum float64
	validVolumes := 0
	for _, hist := range historical {
		histVolumeData := hist.GetVolumeData()
		if histVolumeData != nil {
			if vol, ok := histVolumeData["volume"].(float64); ok {
				volumeSum += vol
				validVolumes++
			}
		}
	}

	if validVolumes == 0 {
		return anomalies
	}

	avgVolume := volumeSum / float64(validVolumes)

	// Volume spike detection
	if volume > avgVolume*v.config.AnomalyDetection.VolumeAnomalies.SpikeMultiplier {
		anomalies = append(anomalies, Anomaly{
			Type:        "volume",
			Severity:    "high",
			Description: fmt.Sprintf("Volume spike: %.2f is %.1fx average volume", volume, volume/avgVolume),
			Value:       volume,
			Threshold:   avgVolume * v.config.AnomalyDetection.VolumeAnomalies.SpikeMultiplier,
			Timestamp:   envelope.GetTimestamp(),
			Context: map[string]interface{}{
				"current_volume": volume,
				"average_volume": avgVolume,
				"multiplier":     volume / avgVolume,
				"symbol":         envelope.GetSymbol(),
			},
		})
	}

	// Volume drought detection
	if volume < avgVolume*v.config.AnomalyDetection.VolumeAnomalies.DroughtThreshold {
		anomalies = append(anomalies, Anomaly{
			Type:        "volume",
			Severity:    "medium",
			Description: fmt.Sprintf("Volume drought: %.2f is %.1f%% of average volume", volume, (volume/avgVolume)*100),
			Value:       volume,
			Threshold:   avgVolume * v.config.AnomalyDetection.VolumeAnomalies.DroughtThreshold,
			Timestamp:   envelope.GetTimestamp(),
			Context: map[string]interface{}{
				"current_volume": volume,
				"average_volume": avgVolume,
				"percentage":     (volume / avgVolume) * 100,
				"symbol":         envelope.GetSymbol(),
			},
		})
	}

	return anomalies
}

// detectSpreadAnomalies detects spread-based anomalies
func (v *DataValidator) detectSpreadAnomalies(envelope DataEnvelope) []Anomaly {
	anomalies := make([]Anomaly, 0)

	orderBook := envelope.GetOrderBook()
	if orderBook == nil {
		return anomalies
	}

	bestBid, hasBid := orderBook["best_bid_price"].(float64)
	bestAsk, hasAsk := orderBook["best_ask_price"].(float64)

	if !hasBid || !hasAsk {
		return anomalies
	}

	if bestAsk <= bestBid {
		return anomalies // Invalid spread data
	}

	// Calculate spread in basis points
	midPrice := (bestBid + bestAsk) / 2
	spread := bestAsk - bestBid
	spreadBps := (spread / midPrice) * 10000

	// Check for excessive spread
	if spreadBps > float64(v.config.AnomalyDetection.SpreadAnomalies.MaxSpreadBps) {
		anomalies = append(anomalies, Anomaly{
			Type:        "spread",
			Severity:    "high",
			Description: fmt.Sprintf("Excessive spread: %.2f bps exceeds maximum %.d bps", spreadBps, v.config.AnomalyDetection.SpreadAnomalies.MaxSpreadBps),
			Value:       spreadBps,
			Threshold:   float64(v.config.AnomalyDetection.SpreadAnomalies.MaxSpreadBps),
			Timestamp:   envelope.GetTimestamp(),
			Context: map[string]interface{}{
				"best_bid":   bestBid,
				"best_ask":   bestAsk,
				"spread":     spread,
				"spread_bps": spreadBps,
				"symbol":     envelope.GetSymbol(),
			},
		})
	}

	return anomalies
}

// Helper functions

func (v *DataValidator) updateHistoricalData(envelope DataEnvelope) {
	v.mu.Lock()
	defer v.mu.Unlock()

	key := envelope.GetSymbol()
	historical := v.historicalData[key]
	
	// Add new data point
	historical = append(historical, envelope)
	
	// Keep only data within window
	cutoff := time.Now().Add(-time.Duration(v.config.AnomalyDetection.WindowSize) * time.Hour)
	filtered := make([]DataEnvelope, 0)
	for _, env := range historical {
		if env.GetTimestamp().After(cutoff) {
			filtered = append(filtered, env)
		}
	}
	
	v.historicalData[key] = filtered
}

func (v *DataValidator) updateValidationCounts(symbol string, valid bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	counts, exists := v.validationCounts[symbol]
	if !exists {
		counts = &ValidationCounts{}
		v.validationCounts[symbol] = counts
	}

	counts.TotalValidations++
	counts.LastValidation = time.Now()

	if valid {
		counts.ConsecutiveSuccess++
		counts.ConsecutiveFails = 0
		
		// Check for recovery from quarantine
		if counts.Quarantined && counts.ConsecutiveSuccess >= v.config.Validation.RecoveryThreshold {
			counts.Quarantined = false
		}
	} else {
		counts.FailedValidations++
		counts.ConsecutiveFails++
		counts.ConsecutiveSuccess = 0
		
		// Check for quarantine
		if counts.ConsecutiveFails >= v.config.Validation.QuarantineThreshold {
			counts.Quarantined = true
		}
	}
}

func (v *DataValidator) exceedsPrecision(value float64, precision int) bool {
	str := strconv.FormatFloat(value, 'f', -1, 64)
	parts := strings.Split(str, ".")
	if len(parts) > 1 {
		return len(parts[1]) > precision
	}
	return false
}

func (v *DataValidator) determineSeverity(value, threshold float64) string {
	ratio := value / threshold
	switch {
	case ratio >= 3.0:
		return "critical"
	case ratio >= 2.0:
		return "high"
	case ratio >= 1.5:
		return "medium"
	default:
		return "low"
	}
}

func (v *DataValidator) reportMetrics(result *ValidationResult) {
	if v.metricsCallback == nil {
		return
	}

	// Quality metrics
	v.metricsCallback("data_quality_score", result.QualityScore)
	v.metricsCallback("data_freshness_score", result.Metrics.FreshnessScore)
	v.metricsCallback("data_completeness_score", result.Metrics.CompletenessScore)
	v.metricsCallback("data_consistency_score", result.Metrics.ConsistencyScore)
	v.metricsCallback("data_anomaly_free_score", result.Metrics.AnomalyFreeScore)

	// Validation results
	if result.Valid {
		v.metricsCallback("data_validation_success", 1)
	} else {
		v.metricsCallback("data_validation_error", 1)
	}

	// Anomaly counts
	v.metricsCallback("data_anomalies_detected", float64(len(result.Anomalies)))

	// Processing metrics
	v.metricsCallback("data_validation_processing_time_ms", result.Metrics.ProcessingTimeMs)
}

func (v *DataValidator) reportBatchMetrics(results []*ValidationResult) {
	if v.metricsCallback == nil {
		return
	}

	totalQualityScore := 0.0
	totalValid := 0
	totalAnomalies := 0

	for _, result := range results {
		totalQualityScore += result.QualityScore
		if result.Valid {
			totalValid++
		}
		totalAnomalies += len(result.Anomalies)
	}

	batchSize := len(results)
	avgQualityScore := totalQualityScore / float64(batchSize)
	validationRate := float64(totalValid) / float64(batchSize)

	v.metricsCallback("data_batch_avg_quality_score", avgQualityScore)
	v.metricsCallback("data_batch_validation_rate", validationRate)
	v.metricsCallback("data_batch_anomaly_rate", float64(totalAnomalies)/float64(batchSize))
	v.metricsCallback("data_batch_size", float64(batchSize))
}

// IsQuarantined checks if a symbol is quarantined due to validation failures
func (v *DataValidator) IsQuarantined(symbol string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	counts, exists := v.validationCounts[symbol]
	return exists && counts.Quarantined
}

// GetValidationStats returns validation statistics for a symbol
func (v *DataValidator) GetValidationStats(symbol string) *ValidationCounts {
	v.mu.RLock()
	defer v.mu.RUnlock()

	counts, exists := v.validationCounts[symbol]
	if !exists {
		return &ValidationCounts{}
	}

	// Return a copy to avoid race conditions
	return &ValidationCounts{
		TotalValidations:   counts.TotalValidations,
		FailedValidations:  counts.FailedValidations,
		ConsecutiveFails:   counts.ConsecutiveFails,
		ConsecutiveSuccess: counts.ConsecutiveSuccess,
		Quarantined:       counts.Quarantined,
		LastValidation:    counts.LastValidation,
	}
}

// storeHistoricalData adds envelope to historical data for anomaly detection
func (v *DataValidator) storeHistoricalData(envelope DataEnvelope) {
	if !v.config.AnomalyDetection.Enable {
		return
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	symbol := envelope.GetSymbol()
	
	// Initialize slice if not exists
	if v.historicalData[symbol] == nil {
		v.historicalData[symbol] = make([]DataEnvelope, 0)
	}
	
	// Add current envelope
	v.historicalData[symbol] = append(v.historicalData[symbol], envelope)
	
	// Keep only the most recent entries (window size)
	maxEntries := v.config.AnomalyDetection.WindowSize * 6 // Assuming ~6 data points per hour for 24h window
	if len(v.historicalData[symbol]) > maxEntries {
		v.historicalData[symbol] = v.historicalData[symbol][len(v.historicalData[symbol])-maxEntries:]
	}
}