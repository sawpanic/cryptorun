package analyst

import "time"

// WinnerCandidate represents a top-performing asset from market data
type WinnerCandidate struct {
	Symbol        string    `json:"symbol"`
	Timeframe     string    `json:"timeframe"`     // "1h", "24h", "7d"
	PerformancePC float64   `json:"performance_pc"` // Percentage gain
	Volume        float64   `json:"volume"`
	Price         float64   `json:"price"`
	Rank          int       `json:"rank"`          // 1-based ranking within timeframe
	Source        string    `json:"source"`        // "kraken_ticker" or "fixture"
	Timestamp     time.Time `json:"timestamp"`
}

// CandidateMiss represents a missed opportunity with reason analysis
type CandidateMiss struct {
	Symbol      string    `json:"symbol"`
	Timeframe   string    `json:"timeframe"`
	Performance float64   `json:"performance_pc"`
	ReasonCode  string    `json:"reason_code"`    // From gate traces in candidates JSONL
	Evidence    string    `json:"evidence"`       // Supporting evidence from gate data
	CandidateScore float64 `json:"candidate_score,omitempty"` // If found in candidates
	Selected    bool      `json:"selected"`       // Whether candidate was selected
	Timestamp   time.Time `json:"timestamp"`
}

// CoverageMetrics represents coverage analysis across timeframes
type CoverageMetrics struct {
	Timeframe       string  `json:"timeframe"`
	TotalWinners    int     `json:"total_winners"`
	CandidatesFound int     `json:"candidates_found"`
	Selected        int     `json:"selected"`
	RecallAt20      float64 `json:"recall_at_20"`      // Winners found in top 20 candidates
	GoodFilterRate  float64 `json:"good_filter_rate"`  // Selected winners / total selected
	BadMissRate     float64 `json:"bad_miss_rate"`     // High-performing misses / total winners
	StaleDataRate   float64 `json:"stale_data_rate"`   // DATA_STALE misses / total misses
	Timestamp       time.Time `json:"timestamp"`
}

// CoverageReport aggregates all analysis results
type CoverageReport struct {
	Generated   time.Time         `json:"generated"`
	Timeframes  []string          `json:"timeframes"`
	Winners     []WinnerCandidate `json:"winners_summary"`
	Metrics     []CoverageMetrics `json:"metrics"`
	TopReasons  []ReasonSummary   `json:"top_reasons"`      // Top 3 reason codes with counts
	PolicyCheck PolicyResult      `json:"policy_check"`
	Universe    UniverseInfo      `json:"universe"`
}

// ReasonSummary represents reason code statistics
type ReasonSummary struct {
	ReasonCode string `json:"reason_code"`
	Count      int    `json:"count"`
	Percentage float64 `json:"percentage"`
	Examples   []string `json:"examples,omitempty"` // Sample symbols
}

// PolicyResult represents quality policy evaluation
type PolicyResult struct {
	Overall      string             `json:"overall"`       // "PASS" or "FAIL"
	Violations   []PolicyViolation  `json:"violations"`
	Thresholds   map[string]float64 `json:"thresholds"`
	ActualValues map[string]float64 `json:"actual_values"`
}

// PolicyViolation represents a policy threshold breach
type PolicyViolation struct {
	Timeframe string  `json:"timeframe"`
	Metric    string  `json:"metric"`
	Threshold float64 `json:"threshold"`
	Actual    float64 `json:"actual"`
	Severity  string  `json:"severity"`
}

// UniverseInfo represents universe analysis context
type UniverseInfo struct {
	Source        string `json:"source"`         // "config/universe.json"
	TotalPairs    int    `json:"total_pairs"`
	CandidateLimit int   `json:"candidate_limit"`
	Exchange      string `json:"exchange"`
}

// QualityPolicies represents configurable quality thresholds
type QualityPolicies struct {
	BadMissRateThresholds map[string]float64 `json:"bad_miss_rate"` // By timeframe
}

// AnalystResult represents the complete analyst output
type AnalystResult struct {
	Winners   []WinnerCandidate `json:"winners"`
	Misses    []CandidateMiss   `json:"misses"`
	Coverage  CoverageReport    `json:"coverage"`
	ExitCode  int               `json:"exit_code"`
}

// Reason codes for missed opportunities
const (
	ReasonSpreadWide   = "SPREAD_WIDE"
	ReasonFreshnessFail = "FRESHNESS_FAIL"
	ReasonDataStale    = "DATA_STALE"
	ReasonDepthLow     = "DEPTH_LOW"
	ReasonVADRFail     = "VADR_FAIL"
	ReasonADVLow       = "ADV_LOW"
	ReasonFatigueBlock = "FATIGUE_BLOCK"
	ReasonLateFill     = "LATE_FILL"
	ReasonNotCandidate = "NOT_CANDIDATE"
	ReasonLowScore     = "LOW_SCORE"
	ReasonNotSelected  = "NOT_SELECTED"
)