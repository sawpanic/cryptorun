package validate

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// AnomalyConfig defines configuration for anomaly detection
type AnomalyConfig struct {
	MADThreshold    float64 `json:"mad_threshold"`     // Median Absolute Deviation threshold (e.g., 3.0)
	SpikeThreshold  float64 `json:"spike_threshold"`   // Spike detection threshold for volume/price
	WindowSize      int     `json:"window_size"`       // Rolling window size for comparison
	MinDataPoints   int     `json:"min_data_points"`   // Minimum data points required for detection
	PriceFields     []string `json:"price_fields"`     // Fields to check for price anomalies
	VolumeFields    []string `json:"volume_fields"`    // Fields to check for volume anomalies
	EnableQuarantine bool    `json:"enable_quarantine"` // Whether to quarantine anomalous data
}

// AnomalyType represents the type of anomaly detected
type AnomalyType string

const (
	AnomalyTypePrice      AnomalyType = "price"
	AnomalyTypeVolume     AnomalyType = "volume"
	AnomalyTypeSpike      AnomalyType = "spike"
	AnomalyTypeOutlier    AnomalyType = "outlier"
	AnomalyTypeCorruption AnomalyType = "corruption"
)

// AnomalyResult contains the result of anomaly detection
type AnomalyResult struct {
	IsAnomaly       bool                   `json:"is_anomaly"`
	AnomalyType     AnomalyType           `json:"anomaly_type,omitempty"`
	Field           string                `json:"field,omitempty"`
	Value           interface{}           `json:"value,omitempty"`
	ExpectedRange   *Range                `json:"expected_range,omitempty"`
	MADScore        float64               `json:"mad_score,omitempty"`
	SeverityLevel   string                `json:"severity_level,omitempty"`
	ShouldQuarantine bool                 `json:"should_quarantine"`
	Reason          string                `json:"reason,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	DetectedAt      time.Time             `json:"detected_at"`
}

// Range represents a numeric range for expected values
type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// AnomalyChecker performs anomaly detection on data records
type AnomalyChecker struct {
	config      AnomalyConfig
	priceWindow []float64 // Rolling window for price values
	volumeWindow []float64 // Rolling window for volume values
	metrics     *AnomalyMetrics
}

// AnomalyMetrics tracks anomaly detection metrics
type AnomalyMetrics struct {
	TotalChecks     int64
	AnomaliesFound  int64
	QuarantineCount int64
	FalsePositives  int64
	LastCheckTime   time.Time
}

// NewAnomalyChecker creates a new anomaly checker with the given configuration
func NewAnomalyChecker(config AnomalyConfig) *AnomalyChecker {
	if config.MADThreshold == 0 {
		config.MADThreshold = 3.0 // Default MAD threshold
	}
	if config.WindowSize == 0 {
		config.WindowSize = 100 // Default window size
	}
	if config.MinDataPoints == 0 {
		config.MinDataPoints = 20 // Minimum points for reliable MAD calculation
	}
	if config.SpikeThreshold == 0 {
		config.SpikeThreshold = 5.0 // Default spike threshold
	}

	return &AnomalyChecker{
		config:       config,
		priceWindow:  make([]float64, 0, config.WindowSize),
		volumeWindow: make([]float64, 0, config.WindowSize),
		metrics:      &AnomalyMetrics{},
	}
}

// CheckAnomaly performs comprehensive anomaly detection on a data record
func (ac *AnomalyChecker) CheckAnomaly(data map[string]interface{}, tier string) *AnomalyResult {
	ac.metrics.TotalChecks++
	ac.metrics.LastCheckTime = time.Now()

	result := &AnomalyResult{
		DetectedAt: time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	// Check price anomalies
	for _, field := range ac.config.PriceFields {
		if value, exists := data[field]; exists {
			if floatVal, ok := convertToFloat64(value); ok {
				if anomaly := ac.checkPriceAnomaly(field, floatVal, tier); anomaly != nil {
					*result = *anomaly
					result.DetectedAt = time.Now()
					ac.metrics.AnomaliesFound++
					if result.ShouldQuarantine {
						ac.metrics.QuarantineCount++
					}
					return result
				}
			}
		}
	}

	// Check volume anomalies
	for _, field := range ac.config.VolumeFields {
		if value, exists := data[field]; exists {
			if floatVal, ok := convertToFloat64(value); ok {
				if anomaly := ac.checkVolumeAnomaly(field, floatVal, tier); anomaly != nil {
					*result = *anomaly
					result.DetectedAt = time.Now()
					ac.metrics.AnomaliesFound++
					if result.ShouldQuarantine {
						ac.metrics.QuarantineCount++
					}
					return result
				}
			}
		}
	}

	// Check for data corruption
	if anomaly := ac.checkDataCorruption(data); anomaly != nil {
		*result = *anomaly
		result.DetectedAt = time.Now()
		ac.metrics.AnomaliesFound++
		return result
	}

	return result // No anomaly detected
}

// checkPriceAnomaly detects price anomalies using MAD-based z-score
func (ac *AnomalyChecker) checkPriceAnomaly(field string, value float64, tier string) *AnomalyResult {
	// Add to rolling window
	ac.priceWindow = append(ac.priceWindow, value)
	if len(ac.priceWindow) > ac.config.WindowSize {
		ac.priceWindow = ac.priceWindow[1:]
	}

	// Need minimum data points for reliable detection
	if len(ac.priceWindow) < ac.config.MinDataPoints {
		return nil
	}

	// Calculate MAD-based z-score
	madScore := ac.calculateMADScore(ac.priceWindow, value)
	
	// Check if value is anomalous
	if math.Abs(madScore) > ac.config.MADThreshold {
		severity := ac.getSeverityLevel(math.Abs(madScore))
		shouldQuarantine := ac.config.EnableQuarantine && severity == "critical"

		// Calculate expected range
		median := ac.calculateMedian(ac.priceWindow)
		mad := ac.calculateMAD(ac.priceWindow)
		expectedRange := &Range{
			Min: median - (ac.config.MADThreshold * mad),
			Max: median + (ac.config.MADThreshold * mad),
		}

		return &AnomalyResult{
			IsAnomaly:       true,
			AnomalyType:     AnomalyTypePrice,
			Field:           field,
			Value:           value,
			ExpectedRange:   expectedRange,
			MADScore:        madScore,
			SeverityLevel:   severity,
			ShouldQuarantine: shouldQuarantine,
			Reason:          fmt.Sprintf("Price value %.4f exceeds MAD threshold (%.2f) with score %.4f", value, ac.config.MADThreshold, madScore),
			Metadata: map[string]interface{}{
				"tier":        tier,
				"window_size": len(ac.priceWindow),
				"median":      median,
				"mad":         mad,
			},
		}
	}

	return nil
}

// checkVolumeAnomaly detects volume anomalies and spikes
func (ac *AnomalyChecker) checkVolumeAnomaly(field string, value float64, tier string) *AnomalyResult {
	// Add to rolling window
	ac.volumeWindow = append(ac.volumeWindow, value)
	if len(ac.volumeWindow) > ac.config.WindowSize {
		ac.volumeWindow = ac.volumeWindow[1:]
	}

	// Need minimum data points
	if len(ac.volumeWindow) < ac.config.MinDataPoints {
		return nil
	}

	// Check for volume spikes first
	median := ac.calculateMedian(ac.volumeWindow)
	if value > median*ac.config.SpikeThreshold {
		return &AnomalyResult{
			IsAnomaly:       true,
			AnomalyType:     AnomalyTypeSpike,
			Field:           field,
			Value:           value,
			SeverityLevel:   "warning",
			ShouldQuarantine: false, // Volume spikes are often legitimate
			Reason:          fmt.Sprintf("Volume spike detected: %.4f is %.2fx median (%.4f)", value, value/median, median),
			Metadata: map[string]interface{}{
				"tier":           tier,
				"spike_ratio":    value / median,
				"median_volume":  median,
			},
		}
	}

	// Calculate MAD-based z-score for outliers
	madScore := ac.calculateMADScore(ac.volumeWindow, value)
	
	if math.Abs(madScore) > ac.config.MADThreshold {
		severity := ac.getSeverityLevel(math.Abs(madScore))
		shouldQuarantine := ac.config.EnableQuarantine && severity == "critical"

		mad := ac.calculateMAD(ac.volumeWindow)
		expectedRange := &Range{
			Min: math.Max(0, median-(ac.config.MADThreshold*mad)), // Volume can't be negative
			Max: median + (ac.config.MADThreshold * mad),
		}

		return &AnomalyResult{
			IsAnomaly:       true,
			AnomalyType:     AnomalyTypeVolume,
			Field:           field,
			Value:           value,
			ExpectedRange:   expectedRange,
			MADScore:        madScore,
			SeverityLevel:   severity,
			ShouldQuarantine: shouldQuarantine,
			Reason:          fmt.Sprintf("Volume outlier detected: %.4f exceeds MAD threshold with score %.4f", value, madScore),
			Metadata: map[string]interface{}{
				"tier":        tier,
				"window_size": len(ac.volumeWindow),
				"median":      median,
				"mad":         mad,
			},
		}
	}

	return nil
}

// checkDataCorruption detects obviously corrupted data
func (ac *AnomalyChecker) checkDataCorruption(data map[string]interface{}) *AnomalyResult {
	for field, value := range data {
		// Check for NaN or infinite values
		if floatVal, ok := convertToFloat64(value); ok {
			if math.IsNaN(floatVal) || math.IsInf(floatVal, 0) {
				return &AnomalyResult{
					IsAnomaly:       true,
					AnomalyType:     AnomalyTypeCorruption,
					Field:           field,
					Value:           value,
					SeverityLevel:   "critical",
					ShouldQuarantine: true,
					Reason:          fmt.Sprintf("Data corruption detected: %s contains %v", field, value),
				}
			}
		}

		// Check for negative prices
		if (field == "price" || field == "close" || field == "open" || field == "high" || field == "low") {
			if floatVal, ok := convertToFloat64(value); ok && floatVal <= 0 {
				return &AnomalyResult{
					IsAnomaly:       true,
					AnomalyType:     AnomalyTypeCorruption,
					Field:           field,
					Value:           value,
					SeverityLevel:   "critical",
					ShouldQuarantine: true,
					Reason:          fmt.Sprintf("Invalid price detected: %s = %v (prices must be positive)", field, value),
				}
			}
		}

		// Check for negative volumes
		if (field == "volume" || field == "base_volume" || field == "quote_volume") {
			if floatVal, ok := convertToFloat64(value); ok && floatVal < 0 {
				return &AnomalyResult{
					IsAnomaly:       true,
					AnomalyType:     AnomalyTypeCorruption,
					Field:           field,
					Value:           value,
					SeverityLevel:   "critical",
					ShouldQuarantine: true,
					Reason:          fmt.Sprintf("Invalid volume detected: %s = %v (volumes cannot be negative)", field, value),
				}
			}
		}
	}

	return nil
}

// calculateMADScore calculates the MAD-based z-score for a value
func (ac *AnomalyChecker) calculateMADScore(window []float64, value float64) float64 {
	if len(window) == 0 {
		return 0
	}

	median := ac.calculateMedian(window)
	mad := ac.calculateMAD(window)
	
	if mad == 0 {
		return 0 // Avoid division by zero
	}

	return (value - median) / mad
}

// calculateMedian calculates the median of a slice of float64 values
func (ac *AnomalyChecker) calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Create a copy to avoid modifying the original
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// calculateMAD calculates the Median Absolute Deviation
func (ac *AnomalyChecker) calculateMAD(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	median := ac.calculateMedian(values)
	deviations := make([]float64, len(values))
	
	for i, value := range values {
		deviations[i] = math.Abs(value - median)
	}

	return ac.calculateMedian(deviations)
}

// getSeverityLevel determines the severity level based on MAD score
func (ac *AnomalyChecker) getSeverityLevel(absMADScore float64) string {
	if absMADScore > 5.0 {
		return "critical"
	} else if absMADScore > 4.0 {
		return "high"
	} else if absMADScore > 3.0 {
		return "medium"
	}
	return "low"
}

// GetMetrics returns the current anomaly detection metrics
func (ac *AnomalyChecker) GetMetrics() *AnomalyMetrics {
	return ac.metrics
}

// Reset clears the rolling windows and resets metrics
func (ac *AnomalyChecker) Reset() {
	ac.priceWindow = ac.priceWindow[:0]
	ac.volumeWindow = ac.volumeWindow[:0]
	ac.metrics = &AnomalyMetrics{}
}

// convertToFloat64 safely converts various numeric types to float64
func convertToFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	default:
		return 0, false
	}
}

// AnomalyCheckFn creates a validation function for anomaly detection
func AnomalyCheckFn(config AnomalyConfig, tier string) ValidateFn {
	checker := NewAnomalyChecker(config)
	
	return func(data map[string]interface{}) error {
		result := checker.CheckAnomaly(data, tier)
		if result.IsAnomaly && result.ShouldQuarantine {
			return fmt.Errorf("anomaly detected and quarantined: %s", result.Reason)
		}
		return nil
	}
}