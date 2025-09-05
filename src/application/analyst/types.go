package analyst

import (
	"time"

	"cryptorun/application/pipeline"
)

// Winner represents a cryptocurrency that performed well over a specific timeframe
type Winner struct {
	Symbol      string    `json:"symbol"`
	TimeFrame   string    `json:"timeframe"`
	Performance float64   `json:"performance_pct"`
	Volume24h   float64   `json:"volume_24h_usd"`
	Timestamp   time.Time `json:"timestamp"`
	Source      string    `json:"source"` // "kraken" or "fixture"
}

// WinnerSet contains winners across multiple timeframes
type WinnerSet struct {
	Winners1h  []Winner `json:"winners_1h"`
	Winners24h []Winner `json:"winners_24h"`
	Winners7d  []Winner `json:"winners_7d"`
	FetchTime  time.Time `json:"fetch_time"`
	Source     string    `json:"source"`
}

// Miss represents a winner that our scanner missed
type Miss struct {
	Symbol       string                 `json:"symbol"`
	TimeFrame    string                 `json:"timeframe"`
	Performance  float64                `json:"performance_pct"`
	ReasonCode   string                 `json:"reason_code"`
	Evidence     map[string]interface{} `json:"evidence"`
	WasCandidate bool                   `json:"was_candidate"`
	Timestamp    time.Time              `json:"timestamp"`
}

// Coverage contains comprehensive coverage metrics
type Coverage struct {
	TimeFrame        string  `json:"timeframe"`
	TotalWinners     int     `json:"total_winners"`
	CandidatesFound  int     `json:"candidates_found"`
	Hits             int     `json:"hits"`
	Misses           int     `json:"misses"`
	RecallAt20       float64 `json:"recall_at_20"`
	GoodFilterRate   float64 `json:"good_filter_rate"`
	BadMissRate      float64 `json:"bad_miss_rate"`
	StaleDataRate    float64 `json:"stale_data_rate"`
	ThresholdBreach  bool    `json:"threshold_breach"`
	PolicyThreshold  float64 `json:"policy_threshold"`
}

// AnalystReport contains complete coverage analysis
type AnalystReport struct {
	RunTime     time.Time  `json:"run_time"`
	Coverage1h  Coverage   `json:"coverage_1h"`
	Coverage24h Coverage   `json:"coverage_24h"`
	Coverage7d  Coverage   `json:"coverage_7d"`
	TopMisses   []Miss     `json:"top_misses"`
	Summary     string     `json:"summary"`
	PolicyPass  bool       `json:"policy_pass"`
}

// QualityPolicy defines coverage thresholds
type QualityPolicy struct {
	BadMissRateThresholds map[string]float64 `json:"bad_miss_rate_thresholds"`
	Description           string             `json:"description"`
}

// DefaultQualityPolicy returns default quality policy
func DefaultQualityPolicy() QualityPolicy {
	return QualityPolicy{
		BadMissRateThresholds: map[string]float64{
			"1h":  0.35, // 35% max bad miss rate for 1h window
			"24h": 0.40, // 40% max bad miss rate for 24h window  
			"7d":  0.40, // 40% max bad miss rate for 7d window
		},
		Description: "Maximum acceptable bad miss rates per timeframe",
	}
}

// ReasonCode constants for miss analysis
const (
	ReasonSpreadWide     = "SPREAD_WIDE"
	ReasonDepthLow       = "DEPTH_LOW"
	ReasonVADRLow        = "VADR_LOW"
	ReasonFreshnessStale = "FRESHNESS_STALE"
	ReasonFatigue        = "FATIGUE"
	ReasonLateFill       = "LATE_FILL"
	ReasonDataStale      = "DATA_STALE"
	ReasonNotCandidate   = "NOT_CANDIDATE"
	ReasonScoreLow       = "SCORE_LOW"
	ReasonUnknown        = "UNKNOWN"
)

// CandidateResult represents a candidate from the scanning pipeline
// This mirrors the structure from scan.go for analysis purposes
type CandidateResult struct {
	Symbol   string                    `json:"symbol"`
	Score    pipeline.CompositeScore   `json:"score"`
	Factors  pipeline.FactorSet        `json:"factors"`
	Decision string                    `json:"decision"`
	Gates    CandidateGates            `json:"gates"`
	Meta     CandidateMeta             `json:"meta"`
}

type CandidateGates struct {
	Freshness      map[string]interface{} `json:"freshness"`
	LateFill       map[string]interface{} `json:"late_fill"`
	Fatigue        map[string]interface{} `json:"fatigue"`
	Microstructure map[string]interface{} `json:"microstructure"`
}

type CandidateMeta struct {
	Regime    string    `json:"regime"`
	Timestamp time.Time `json:"timestamp"`
}