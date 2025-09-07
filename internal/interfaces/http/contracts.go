package http

import (
	"time"
)

// CandidatesResponse represents the top composite candidates with gate status
type CandidatesResponse struct {
	Timestamp  time.Time         `json:"timestamp"`
	Regime     string            `json:"regime"`
	TotalCount int               `json:"total_count"`
	Requested  int               `json:"requested"`
	Candidates []CandidateRecord `json:"candidates"`
	Summary    CandidatesSummary `json:"summary"`
}

// CandidateRecord represents a single trading candidate
type CandidateRecord struct {
	Symbol         string             `json:"symbol"`
	Exchange       string             `json:"exchange"`
	Score          float64            `json:"score"`
	Rank           int                `json:"rank"`
	GateStatus     GateStatus         `json:"gate_status"`
	Microstructure MicrostructureData `json:"microstructure"`
	Attribution    Attribution        `json:"attribution"`
	LastUpdated    time.Time          `json:"last_updated"`
}

// GateStatus represents the status of various entry gates
type GateStatus struct {
	ScoreGate      bool     `json:"score_gate"`                // Score >= 75
	VADRGate       bool     `json:"vadr_gate"`                 // VADR >= 1.8
	FundingGate    bool     `json:"funding_gate"`              // Funding divergence >= 2σ
	SpreadGate     bool     `json:"spread_gate"`               // Spread < 50bps
	DepthGate      bool     `json:"depth_gate"`                // Depth >= $100k within ±2%
	FatigueGate    bool     `json:"fatigue_gate"`              // Not fatigued
	FreshnessGate  bool     `json:"freshness_gate"`            // Fresh signal
	OverallPassed  bool     `json:"overall_passed"`            // All gates passed
	FailureReasons []string `json:"failure_reasons,omitempty"` // Reasons if not passed
}

// MicrostructureData represents market microstructure information
type MicrostructureData struct {
	SpreadBps float64 `json:"spread_bps"`
	DepthUSD  float64 `json:"depth_usd"`
	VADR      float64 `json:"vadr"`
	Volume24h float64 `json:"volume_24h"`
	LastPrice float64 `json:"last_price"`
	BidPrice  float64 `json:"bid_price"`
	AskPrice  float64 `json:"ask_price"`
}

// Attribution represents scoring attribution breakdown
type Attribution struct {
	MomentumScore  float64 `json:"momentum_score"`
	TechnicalScore float64 `json:"technical_score"`
	VolumeScore    float64 `json:"volume_score"`
	QualityScore   float64 `json:"quality_score"`
	SocialBonus    float64 `json:"social_bonus"`   // Capped at +10
	WeightProfile  string  `json:"weight_profile"` // calm/normal/volatile
}

// CandidatesSummary provides aggregate statistics
type CandidatesSummary struct {
	PassedAllGates     int           `json:"passed_all_gates"`
	AvgScore           float64       `json:"avg_score"`
	MedianScore        float64       `json:"median_score"`
	TopDecileThreshold float64       `json:"top_decile_threshold"`
	GatePassRates      GatePassRates `json:"gate_pass_rates"`
}

// GatePassRates shows how many candidates passed each gate
type GatePassRates struct {
	Score     float64 `json:"score"`     // % passing score gate
	VADR      float64 `json:"vadr"`      // % passing VADR gate
	Funding   float64 `json:"funding"`   // % passing funding gate
	Spread    float64 `json:"spread"`    // % passing spread gate
	Depth     float64 `json:"depth"`     // % passing depth gate
	Fatigue   float64 `json:"fatigue"`   // % passing fatigue gate
	Freshness float64 `json:"freshness"` // % passing freshness gate
}

// ExplainResponse represents explainability information for a symbol
type ExplainResponse struct {
	Symbol      string                 `json:"symbol"`
	Exchange    string                 `json:"exchange"`
	Timestamp   time.Time              `json:"timestamp"`
	DataSource  string                 `json:"data_source"` // "artifacts" or "live"
	Score       ScoreExplanation       `json:"score"`
	Gates       GateExplanation        `json:"gates"`
	Factors     FactorExplanation      `json:"factors"`
	Regime      RegimeExplanation      `json:"regime"`
	Attribution AttributionExplanation `json:"attribution"`
}

// ScoreExplanation breaks down the composite score calculation
type ScoreExplanation struct {
	FinalScore       float64            `json:"final_score"`
	PreOrthogonal    map[string]float64 `json:"pre_orthogonal"`  // Raw factor scores
	PostOrthogonal   map[string]float64 `json:"post_orthogonal"` // After Gram-Schmidt
	WeightedScores   map[string]float64 `json:"weighted_scores"` // After regime weights
	SocialBonus      float64            `json:"social_bonus"`    // Added separately
	CalculationSteps []CalculationStep  `json:"calculation_steps"`
}

// CalculationStep represents each step in score calculation
type CalculationStep struct {
	Step        string  `json:"step"`
	Description string  `json:"description"`
	Input       float64 `json:"input"`
	Output      float64 `json:"output"`
	Applied     string  `json:"applied"` // What was applied (weight, orthogonalization, etc.)
}

// GateExplanation provides detailed gate evaluation results
type GateExplanation struct {
	Overall        bool       `json:"overall"`
	ScoreGate      GateDetail `json:"score_gate"`
	VADRGate       GateDetail `json:"vadr_gate"`
	FundingGate    GateDetail `json:"funding_gate"`
	SpreadGate     GateDetail `json:"spread_gate"`
	DepthGate      GateDetail `json:"depth_gate"`
	FatigueGate    GateDetail `json:"fatigue_gate"`
	FreshnessGate  GateDetail `json:"freshness_gate"`
	EvaluationTime time.Time  `json:"evaluation_time"`
}

// GateDetail provides specific information about a gate evaluation
type GateDetail struct {
	Passed      bool      `json:"passed"`
	Threshold   float64   `json:"threshold"`
	ActualValue float64   `json:"actual_value"`
	Margin      float64   `json:"margin"` // How far above/below threshold
	Description string    `json:"description"`
	LastChecked time.Time `json:"last_checked"`
}

// FactorExplanation breaks down individual factor calculations
type FactorExplanation struct {
	MomentumCore MomentumFactorDetail `json:"momentum_core"`
	Technical    FactorDetail         `json:"technical"`
	Volume       FactorDetail         `json:"volume"`
	Quality      FactorDetail         `json:"quality"`
	Social       SocialFactorDetail   `json:"social"`
}

// FactorDetail provides detailed information about a factor
type FactorDetail struct {
	RawScore        float64            `json:"raw_score"`
	OrthogonalScore float64            `json:"orthogonal_score"`
	Weight          float64            `json:"weight"`
	WeightedScore   float64            `json:"weighted_score"`
	Components      map[string]float64 `json:"components"`
	DataAge         time.Duration      `json:"data_age"`
	Confidence      float64            `json:"confidence"`
}

// MomentumFactorDetail provides detailed momentum information (protected from orthogonalization)
type MomentumFactorDetail struct {
	RawScore      float64            `json:"raw_score"`
	Weight        float64            `json:"weight"`
	WeightedScore float64            `json:"weighted_score"`
	Timeframes    map[string]float64 `json:"timeframes"` // 1h, 4h, 12h, 24h
	Protected     bool               `json:"protected"`  // Always true for momentum
	DataAge       time.Duration      `json:"data_age"`
	Confidence    float64            `json:"confidence"`
}

// SocialFactorDetail provides social factor information with capping
type SocialFactorDetail struct {
	RawScore    float64            `json:"raw_score"`
	CappedScore float64            `json:"capped_score"` // Limited to +10
	Bonus       float64            `json:"bonus"`        // Same as capped_score
	Sources     map[string]float64 `json:"sources"`
	WasCapped   bool               `json:"was_capped"`
	DataAge     time.Duration      `json:"data_age"`
}

// RegimeExplanation provides regime detection information
type RegimeExplanation struct {
	CurrentRegime string             `json:"current_regime"`
	RegimeWeights map[string]float64 `json:"regime_weights"`
	Indicators    map[string]float64 `json:"indicators"`
	Confidence    float64            `json:"confidence"`
	LastSwitch    time.Time          `json:"last_switch"`
	SwitchReason  string             `json:"switch_reason"`
}

// AttributionExplanation provides comprehensive scoring attribution
type AttributionExplanation struct {
	TotalContributions map[string]float64 `json:"total_contributions"`
	StepByStep         []AttributionStep  `json:"step_by_step"`
	DataSources        map[string]string  `json:"data_sources"`
	CacheStatus        map[string]bool    `json:"cache_status"`
	PerformanceMetrics PerformanceMetrics `json:"performance_metrics"`
}

// AttributionStep shows how each step contributed to final score
type AttributionStep struct {
	Step         string        `json:"step"`
	RunningTotal float64       `json:"running_total"`
	Contribution float64       `json:"contribution"`
	Duration     time.Duration `json:"duration"`
}

// PerformanceMetrics shows calculation performance
type PerformanceMetrics struct {
	TotalDuration time.Duration `json:"total_duration"`
	CacheHits     int           `json:"cache_hits"`
	CacheMisses   int           `json:"cache_misses"`
	APICallsMade  int           `json:"api_calls_made"`
	DataFreshness time.Duration `json:"data_freshness"`
}

// RegimeResponse represents the current regime information
type RegimeResponse struct {
	Timestamp        time.Time          `json:"timestamp"`
	CurrentRegime    string             `json:"current_regime"`
	RegimeNumeric    float64            `json:"regime_numeric"` // 0=choppy, 1=bull, 2=highvol
	Health           RegimeHealthData   `json:"health"`
	Weights          map[string]float64 `json:"weights"`
	SwitchesToday    int                `json:"switches_today"`
	AvgDurationHours float64            `json:"avg_duration_hours"`
	NextEvaluation   time.Time          `json:"next_evaluation"`   // Next 4h evaluation
	History          []RegimeSwitch     `json:"history,omitempty"` // Recent switches
}

// RegimeHealthData represents regime detection health indicators
type RegimeHealthData struct {
	Volatility7d   float64 `json:"volatility_7d"`   // Realized volatility
	AboveMA_Pct    float64 `json:"above_ma_pct"`    // % above 20MA
	BreadthThrust  float64 `json:"breadth_thrust"`  // Breadth thrust indicator
	StabilityScore float64 `json:"stability_score"` // Overall stability
}

// RegimeSwitch represents a historical regime change
type RegimeSwitch struct {
	Timestamp  time.Time     `json:"timestamp"`
	FromRegime string        `json:"from_regime"`
	ToRegime   string        `json:"to_regime"`
	Trigger    string        `json:"trigger"`    // What caused the switch
	Confidence float64       `json:"confidence"` // Switch confidence
	Duration   time.Duration `json:"duration"`   // Time in previous regime
}

// ErrorResponse represents API error responses
type ErrorResponse struct {
	Error     string    `json:"error"`
	Code      string    `json:"code,omitempty"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// PaginationInfo represents pagination metadata for endpoints that support it
type PaginationInfo struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalCount int  `json:"total_count"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// ServerInfo represents server operational information
type ServerInfo struct {
	Version     string        `json:"version"`
	BuildStamp  string        `json:"build_stamp"`
	Uptime      time.Duration `json:"uptime"`
	StartTime   time.Time     `json:"start_time"`
	Environment string        `json:"environment"`
}
