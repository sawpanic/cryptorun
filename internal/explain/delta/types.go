package delta

import (
	"time"
)

// Config represents the explain delta configuration
type Config struct {
	Universe     string // Universe specification (e.g., "topN=30")
	BaselinePath string // Path to baseline or "latest"
	OutputDir    string // Output directory for artifacts
	Progress     bool   // Show progress indicators
}

// Results represents the complete results of an explain delta analysis
type Results struct {
	Universe          string           `json:"universe"`
	Regime            string           `json:"regime"`
	BaselineTimestamp time.Time        `json:"baseline_timestamp"`
	CurrentTimestamp  time.Time        `json:"current_timestamp"`
	TotalAssets       int              `json:"total_assets"`
	FailCount         int              `json:"fail_count"`
	WarnCount         int              `json:"warn_count"`
	OKCount           int              `json:"ok_count"`
	Assets            []*AssetDelta    `json:"assets"`
	WorstOffenders    []*WorstOffender `json:"worst_offenders"`
	ToleranceConfig   *ToleranceConfig `json:"tolerance_config"`
}

// AssetDelta represents the delta analysis for a single asset
type AssetDelta struct {
	Symbol          string                     `json:"symbol"`
	Regime          string                     `json:"regime"`
	Status          string                     `json:"status"` // "OK", "WARN", "FAIL"
	BaselineFactors map[string]float64         `json:"baseline_factors"`
	CurrentFactors  map[string]float64         `json:"current_factors"`
	Deltas          map[string]float64         `json:"deltas"`
	ToleranceCheck  map[string]*ToleranceCheck `json:"tolerance_check"`
	WorstViolation  *WorstOffender             `json:"worst_violation,omitempty"`
}

// ToleranceCheck represents the tolerance validation for a factor
type ToleranceCheck struct {
	Factor    string  `json:"factor"`
	Delta     float64 `json:"delta"`
	Tolerance float64 `json:"tolerance"`
	Exceeded  bool    `json:"exceeded"`
	Severity  string  `json:"severity"` // "OK", "WARN", "FAIL"
}

// WorstOffender represents the worst factor deviation
type WorstOffender struct {
	Symbol    string  `json:"symbol"`
	Factor    string  `json:"factor"`
	Delta     float64 `json:"delta"`
	Tolerance float64 `json:"tolerance"`
	Severity  string  `json:"severity"`
	Hint      string  `json:"hint"`
}

// ToleranceConfig represents tolerance thresholds per regime per factor
type ToleranceConfig struct {
	Regimes map[string]*RegimeTolerance `json:"regimes"`
}

// RegimeTolerance represents tolerances for a specific regime
type RegimeTolerance struct {
	Name             string                      `json:"name"`
	FactorTolerances map[string]*FactorTolerance `json:"factor_tolerances"`
}

// FactorTolerance represents tolerance settings for a specific factor
type FactorTolerance struct {
	Factor    string  `json:"factor"`
	WarnAt    float64 `json:"warn_at"`   // Warning threshold (absolute)
	FailAt    float64 `json:"fail_at"`   // Failure threshold (absolute)
	Direction string  `json:"direction"` // "both", "positive", "negative"
}

// BaselineSnapshot represents a stored baseline for comparison
type BaselineSnapshot struct {
	Timestamp  time.Time                `json:"timestamp"`
	Universe   string                   `json:"universe"`
	Regime     string                   `json:"regime"`
	AssetCount int                      `json:"asset_count"`
	Factors    map[string]*AssetFactors `json:"factors"` // symbol -> factors
}

// AssetFactors represents the factor breakdown for a single asset
type AssetFactors struct {
	Symbol         string          `json:"symbol"`
	Regime         string          `json:"regime"`
	MomentumCore   float64         `json:"momentum_core"`
	TechnicalResid float64         `json:"technical_resid"`
	VolumeResid    float64         `json:"volume_resid"`
	QualityResid   float64         `json:"quality_resid"`
	SocialResid    float64         `json:"social_resid"`
	CompositeScore float64         `json:"composite_score"`
	Gates          map[string]bool `json:"gates"`
}

// ArtifactPaths represents file paths for generated artifacts
type ArtifactPaths struct {
	ResultsJSONL string `json:"results_jsonl"`
	SummaryMD    string `json:"summary_md"`
	OutputDir    string `json:"output_dir"`
}
