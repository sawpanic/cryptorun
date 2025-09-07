package validate

import (
	"fmt"
	"time"
)

// StalenessChecker validates data freshness based on timestamp skew
type StalenessChecker struct {
	config StalenessConfig
}

// StalenessConfig holds configuration for staleness validation
type StalenessConfig struct {
	MaxSkewHot    time.Duration `json:"max_skew_hot"`    // Max allowed skew for hot tier (e.g., 5s)
	MaxSkewWarm   time.Duration `json:"max_skew_warm"`   // Max allowed skew for warm tier (e.g., 60s)
	MaxSkewCold   time.Duration `json:"max_skew_cold"`   // Max allowed skew for cold tier (e.g., 1h)
	ClockTolerance time.Duration `json:"clock_tolerance"` // Tolerance for clock differences (e.g., 100ms)
	EnableFutureCheck bool       `json:"enable_future_check"` // Check for future timestamps
	MaxFuture     time.Duration `json:"max_future"`      // Max allowed future timestamp
}

// StalenessResult holds the result of staleness validation
type StalenessResult struct {
	Valid         bool          `json:"valid"`
	ActualSkew    time.Duration `json:"actual_skew"`
	AllowedSkew   time.Duration `json:"allowed_skew"`
	Timestamp     time.Time     `json:"timestamp"`
	CheckTime     time.Time     `json:"check_time"`
	Tier          string        `json:"tier"`
	SkewType      SkewType      `json:"skew_type"`
	Message       string        `json:"message"`
}

// SkewType represents the type of timestamp skew detected
type SkewType string

const (
	SkewNone   SkewType = "none"
	SkewPast   SkewType = "past"    // Data is too old
	SkewFuture SkewType = "future"  // Data is from the future
)

// NewStalenessChecker creates a new staleness checker with configuration
func NewStalenessChecker(config StalenessConfig) *StalenessChecker {
	return &StalenessChecker{
		config: config,
	}
}

// DefaultStalenessConfig returns default staleness checking configuration
func DefaultStalenessConfig() StalenessConfig {
	return StalenessConfig{
		MaxSkewHot:        5 * time.Second,
		MaxSkewWarm:       60 * time.Second,
		MaxSkewCold:       3600 * time.Second, // 1 hour
		ClockTolerance:    100 * time.Millisecond,
		EnableFutureCheck: true,
		MaxFuture:         10 * time.Second, // Allow up to 10s future timestamps
	}
}

// CheckStaleness validates data freshness for a given tier
func (sc *StalenessChecker) CheckStaleness(data map[string]interface{}, tier string) *StalenessResult {
	return sc.CheckStalenessAtTime(data, tier, time.Now())
}

// CheckStalenessAtTime validates data freshness against a specific check time
func (sc *StalenessChecker) CheckStalenessAtTime(data map[string]interface{}, tier string, checkTime time.Time) *StalenessResult {
	result := &StalenessResult{
		Valid:     true,
		CheckTime: checkTime,
		Tier:      tier,
		SkewType:  SkewNone,
	}
	
	// Extract timestamp from data
	timestamp, err := sc.extractTimestamp(data)
	if err != nil {
		result.Valid = false
		result.Message = fmt.Sprintf("failed to extract timestamp: %v", err)
		return result
	}
	
	result.Timestamp = timestamp
	
	// Determine allowed skew for tier
	allowedSkew := sc.getMaxSkewForTier(tier)
	result.AllowedSkew = allowedSkew
	
	// Calculate actual skew
	skew := checkTime.Sub(timestamp)
	result.ActualSkew = skew
	
	// Check for future timestamps
	if sc.config.EnableFutureCheck && skew < -sc.config.ClockTolerance {
		futureSkew := timestamp.Sub(checkTime)
		result.SkewType = SkewFuture
		
		if futureSkew > sc.config.MaxFuture {
			result.Valid = false
			result.Message = fmt.Sprintf("timestamp is too far in future: %v > %v", 
				futureSkew, sc.config.MaxFuture)
			return result
		}
		
		// Future timestamp within tolerance
		result.Message = fmt.Sprintf("timestamp is in future but within tolerance: %v", futureSkew)
		return result
	}
	
	// Check for stale data (past timestamps)
	if skew > allowedSkew {
		result.Valid = false
		result.SkewType = SkewPast
		result.Message = fmt.Sprintf("data is too stale for %s tier: %v > %v", 
			tier, skew, allowedSkew)
		return result
	}
	
	// Data is fresh
	if skew > 0 {
		result.Message = fmt.Sprintf("data is fresh: %v old (limit: %v)", skew, allowedSkew)
	} else {
		result.Message = "data timestamp is current"
	}
	
	return result
}

// extractTimestamp extracts timestamp from data in various formats
func (sc *StalenessChecker) extractTimestamp(data map[string]interface{}) (time.Time, error) {
	// Try different timestamp field names
	timestampFields := []string{"timestamp", "ts", "time", "created_at", "updated_at"}
	
	for _, field := range timestampFields {
		if value, exists := data[field]; exists {
			return sc.parseTimestamp(value)
		}
	}
	
	return time.Time{}, fmt.Errorf("no timestamp field found in data")
}

// parseTimestamp parses timestamp from various formats
func (sc *StalenessChecker) parseTimestamp(value interface{}) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
		
	case string:
		// Try parsing as RFC3339
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t, nil
		}
		
		// Try parsing as RFC3339Nano
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return t, nil
		}
		
		// Try parsing as Unix timestamp string
		if t, err := time.Parse("1642611234", v); err == nil {
			return t, nil
		}
		
		return time.Time{}, fmt.Errorf("unable to parse timestamp string: %s", v)
		
	case int64:
		// Unix timestamp in seconds
		if v > 1000000000 && v < 4102444800 { // Reasonable range: 2001-2100
			return time.Unix(v, 0), nil
		}
		
		// Unix timestamp in milliseconds
		if v > 1000000000000 && v < 4102444800000 {
			return time.Unix(v/1000, (v%1000)*1000000), nil
		}
		
		// Unix timestamp in microseconds
		if v > 1000000000000000 && v < 4102444800000000 {
			return time.Unix(v/1000000, (v%1000000)*1000), nil
		}
		
		// Unix timestamp in nanoseconds
		if v > 1000000000000000000 {
			return time.Unix(0, v), nil
		}
		
		return time.Time{}, fmt.Errorf("unix timestamp out of reasonable range: %d", v)
		
	case int:
		return sc.parseTimestamp(int64(v))
		
	case float64:
		// Unix timestamp as float (seconds with fractional part)
		if v > 1000000000 && v < 4102444800 {
			sec := int64(v)
			nsec := int64((v - float64(sec)) * 1000000000)
			return time.Unix(sec, nsec), nil
		}
		
		return time.Time{}, fmt.Errorf("float timestamp out of reasonable range: %f", v)
		
	default:
		return time.Time{}, fmt.Errorf("unsupported timestamp type: %T", value)
	}
}

// getMaxSkewForTier returns the maximum allowed skew for a data tier
func (sc *StalenessChecker) getMaxSkewForTier(tier string) time.Duration {
	switch tier {
	case "hot":
		return sc.config.MaxSkewHot
	case "warm":
		return sc.config.MaxSkewWarm
	case "cold":
		return sc.config.MaxSkewCold
	default:
		// Unknown tier, use warm tier as default
		return sc.config.MaxSkewWarm
	}
}

// ValidateIngestionLatency checks if ingestion latency is within acceptable bounds
func (sc *StalenessChecker) ValidateIngestionLatency(dataTimestamp, ingestionTime time.Time, tier string) *StalenessResult {
	result := &StalenessResult{
		Valid:       true,
		CheckTime:   ingestionTime,
		Timestamp:   dataTimestamp,
		Tier:        tier,
		SkewType:    SkewNone,
		AllowedSkew: sc.getMaxSkewForTier(tier),
	}
	
	latency := ingestionTime.Sub(dataTimestamp)
	result.ActualSkew = latency
	
	// Negative latency indicates clock skew or data from future
	if latency < 0 {
		result.SkewType = SkewFuture
		if -latency > sc.config.ClockTolerance {
			result.Valid = false
			result.Message = fmt.Sprintf("data ingested before creation: %v", -latency)
			return result
		}
		result.Message = "small negative latency within clock tolerance"
		return result
	}
	
	// Check if latency exceeds allowed threshold
	if latency > result.AllowedSkew {
		result.Valid = false
		result.SkewType = SkewPast
		result.Message = fmt.Sprintf("ingestion latency too high: %v > %v", latency, result.AllowedSkew)
		return result
	}
	
	result.Message = fmt.Sprintf("ingestion latency acceptable: %v", latency)
	return result
}

// CreateStalenessValidator returns a validation function for use with replication
func (sc *StalenessChecker) CreateStalenessValidator(tier string) func(map[string]interface{}) error {
	return func(data map[string]interface{}) error {
		result := sc.CheckStaleness(data, tier)
		if !result.Valid {
			return fmt.Errorf("staleness check failed: %s", result.Message)
		}
		return nil
	}
}

// GetStalenessMetrics returns metrics about staleness checking
func (sc *StalenessChecker) GetStalenessMetrics(results []*StalenessResult) map[string]interface{} {
	metrics := map[string]interface{}{
		"total_checks":    len(results),
		"valid_checks":    0,
		"invalid_checks":  0,
		"future_skews":    0,
		"past_skews":      0,
		"average_skew_ms": 0.0,
		"max_skew_ms":     0.0,
		"min_skew_ms":     0.0,
	}
	
	if len(results) == 0 {
		return metrics
	}
	
	var totalSkew time.Duration
	var maxSkew time.Duration
	var minSkew time.Duration = 24 * time.Hour // Initialize to a large value
	
	for _, result := range results {
		if result.Valid {
			metrics["valid_checks"] = metrics["valid_checks"].(int) + 1
		} else {
			metrics["invalid_checks"] = metrics["invalid_checks"].(int) + 1
		}
		
		switch result.SkewType {
		case SkewFuture:
			metrics["future_skews"] = metrics["future_skews"].(int) + 1
		case SkewPast:
			metrics["past_skews"] = metrics["past_skews"].(int) + 1
		}
		
		skew := result.ActualSkew
		if skew < 0 {
			skew = -skew // Use absolute value for statistics
		}
		
		totalSkew += skew
		
		if skew > maxSkew {
			maxSkew = skew
		}
		
		if skew < minSkew {
			minSkew = skew
		}
	}
	
	// Calculate averages
	avgSkew := totalSkew / time.Duration(len(results))
	metrics["average_skew_ms"] = float64(avgSkew.Nanoseconds()) / 1000000.0
	metrics["max_skew_ms"] = float64(maxSkew.Nanoseconds()) / 1000000.0
	metrics["min_skew_ms"] = float64(minSkew.Nanoseconds()) / 1000000.0
	
	return metrics
}

// BatchValidateStaleness validates staleness for multiple data points
func (sc *StalenessChecker) BatchValidateStaleness(dataPoints []map[string]interface{}, tier string) []*StalenessResult {
	results := make([]*StalenessResult, len(dataPoints))
	checkTime := time.Now()
	
	for i, data := range dataPoints {
		results[i] = sc.CheckStalenessAtTime(data, tier, checkTime)
	}
	
	return results
}

// GetHealthStatus returns overall health status based on recent staleness checks
func (sc *StalenessChecker) GetHealthStatus(results []*StalenessResult, maxFailureRate float64) (bool, string) {
	if len(results) == 0 {
		return true, "no data to check"
	}
	
	failures := 0
	for _, result := range results {
		if !result.Valid {
			failures++
		}
	}
	
	failureRate := float64(failures) / float64(len(results))
	healthy := failureRate <= maxFailureRate
	
	status := fmt.Sprintf("staleness check failure rate: %.2f%% (%d/%d)", 
		failureRate*100, failures, len(results))
	
	return healthy, status
}