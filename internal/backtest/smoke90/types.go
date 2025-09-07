package smoke90

import (
	"time"
)

// BacktestResults represents the complete results of a smoke90 backtest
type BacktestResults struct {
	Config           *Config         `json:"config"`
	StartTime        time.Time       `json:"start_time"`
	EndTime          time.Time       `json:"end_time"`
	TotalWindows     int             `json:"total_windows"`
	ProcessedWindows int             `json:"processed_windows"`
	SkippedWindows   int             `json:"skipped_windows"`
	Windows          []*WindowResult `json:"windows"`
	Metrics          *MetricsSummary `json:"metrics"`
}

// WindowResult represents results for a single time window
type WindowResult struct {
	Timestamp      time.Time             `json:"timestamp"`
	Candidates     []*CandidateResult    `json:"candidates"`
	PassedCount    int                   `json:"passed_count"`
	FailedCount    int                   `json:"failed_count"`
	GuardPassRate  float64               `json:"guard_pass_rate"`
	GuardStats     map[string]*GuardStat `json:"guard_stats"`
	ThrottleEvents []*ThrottleEvent      `json:"throttle_events"`
	RelaxEvents    []*RelaxEvent         `json:"relax_events"`
	SkipReasons    []string              `json:"skip_reasons"`
}

// CandidateResult represents results for a single candidate
type CandidateResult struct {
	Symbol      string                  `json:"symbol"`
	Score       float64                 `json:"score"`
	Timestamp   time.Time               `json:"timestamp"`
	Passed      bool                    `json:"passed"`
	FailReason  string                  `json:"fail_reason,omitempty"`
	GuardResult map[string]*GuardResult `json:"guard_result"`
	MicroResult *MicroResult            `json:"micro_result,omitempty"`
	PnL         float64                 `json:"pnl"`
	PnLError    string                  `json:"pnl_error,omitempty"`
}

// GuardResult represents the result of applying a single guard
type GuardResult struct {
	Type   string `json:"type"` // "hard" or "soft"
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}

// MicroResult represents microstructure validation result
type MicroResult struct {
	Passed bool     `json:"passed"`
	Reason string   `json:"reason"`
	Venues []string `json:"venues"`
}

// GuardStat represents statistics for a guard across all candidates
type GuardStat struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Total    int     `json:"total"`
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	PassRate float64 `json:"pass_rate"`
}

// ThrottleEvent represents a provider throttling event
type ThrottleEvent struct {
	Provider  string    `json:"provider"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
	Symbol    string    `json:"symbol"`
}

// RelaxEvent represents a P99 latency relaxation event
type RelaxEvent struct {
	Type      string    `json:"type"`
	Symbol    string    `json:"symbol"`
	P99Ms     float64   `json:"p99_ms"`
	GraceMs   float64   `json:"grace_ms"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
}

// Candidate represents a candidate asset for backtesting
type Candidate struct {
	Symbol               string  `json:"symbol"`
	Score                float64 `json:"score"` // Unified score
	VADR                 float64 `json:"vadr"`  // Volume-Adjusted Daily Range
	HasFundingDivergence bool    `json:"has_funding_divergence"`
	Volume24h            float64 `json:"volume_24h"`
	PriceChange1h        float64 `json:"price_change_1h"`
	PriceChange24h       float64 `json:"price_change_24h"`
	PriceChange7d        float64 `json:"price_change_7d,omitempty"`
}

// MetricsSummary represents aggregated metrics across all windows
type MetricsSummary struct {
	TotalCandidates   int                   `json:"total_candidates"`
	PassedCandidates  int                   `json:"passed_candidates"`
	FailedCandidates  int                   `json:"failed_candidates"`
	OverallPassRate   float64               `json:"overall_pass_rate"`
	GuardStats        map[string]*GuardStat `json:"guard_stats"`
	TopGainersHitRate *HitRateStats         `json:"top_gainers_hit_rate"`
	ThrottleStats     *ThrottleStats        `json:"throttle_stats"`
	RelaxStats        *RelaxStats           `json:"relax_stats"`
	SkipStats         *SkipStats            `json:"skip_stats"`
	ErrorCount        int                   `json:"error_count"`
	Errors            []string              `json:"errors,omitempty"`
}

// HitRateStats represents hit rate statistics vs TopGainers
type HitRateStats struct {
	OneHour        *HitRate `json:"1h"`
	TwentyFourHour *HitRate `json:"24h"`
	SevenDay       *HitRate `json:"7d"`
}

// HitRate represents hit/miss statistics for a specific timeframe
type HitRate struct {
	Hits    int     `json:"hits"`
	Misses  int     `json:"misses"`
	Total   int     `json:"total"`
	HitRate float64 `json:"hit_rate"`
}

// ThrottleStats represents provider throttling statistics
type ThrottleStats struct {
	TotalEvents    int            `json:"total_events"`
	EventsPer100   float64        `json:"events_per_100_signals"`
	ProviderCounts map[string]int `json:"provider_counts"`
	MostThrottled  string         `json:"most_throttled_provider"`
}

// RelaxStats represents P99 relaxation statistics
type RelaxStats struct {
	TotalEvents  int     `json:"total_events"`
	EventsPer100 float64 `json:"events_per_100_signals"`
	AvgP99Ms     float64 `json:"avg_p99_ms"`
	AvgGraceMs   float64 `json:"avg_grace_ms"`
}

// SkipStats represents window skip statistics
type SkipStats struct {
	TotalSkips  int            `json:"total_skips"`
	SkipReasons map[string]int `json:"skip_reasons"`
	MostCommon  string         `json:"most_common_reason"`
}

// PerformanceMetrics represents performance metrics for the backtest
type PerformanceMetrics struct {
	TotalRuntime  time.Duration `json:"total_runtime"`
	WindowsPerSec float64       `json:"windows_per_sec"`
	AvgWindowTime time.Duration `json:"avg_window_time"`
	CacheHitRate  float64       `json:"cache_hit_rate"`
}

// ValidationResult represents validation results for test assertions
type ValidationResult struct {
	SchemaValid  bool     `json:"schema_valid"`
	DataComplete bool     `json:"data_complete"`
	MetricsValid bool     `json:"metrics_valid"`
	Errors       []string `json:"errors,omitempty"`
}

// ArtifactPaths represents file paths for generated artifacts
type ArtifactPaths struct {
	ResultsJSONL string `json:"results_jsonl"`
	ReportMD     string `json:"report_md"`
	OutputDir    string `json:"output_dir"`
}
