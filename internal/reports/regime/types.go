package regime

import (
	regimeDomain "cryptorun/internal/domain/regime"
	"time"
)

// RegimeReportData contains all data for weekly regime analysis
type RegimeReportData struct {
	GeneratedAt time.Time             `json:"generated_at"`
	Period      ReportPeriod          `json:"period"`
	FlipHistory []RegimeFlip          `json:"flip_history"`
	ExitStats   map[string]ExitStats  `json:"exit_stats"`
	DecileLifts map[string]DecileLift `json:"decile_lifts"`
	KPIAlerts   []KPIAlert            `json:"kpi_alerts"`
}

// ReportPeriod defines the analysis time window
type ReportPeriod struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  string    `json:"duration"`
}

// RegimeFlip records a regime transition event
type RegimeFlip struct {
	Timestamp      time.Time            `json:"timestamp"`
	FromRegime     string               `json:"from_regime"`
	ToRegime       string               `json:"to_regime"`
	DurationHours  float64              `json:"duration_hours"`
	DetectorInputs RegimeDetectorInputs `json:"detector_inputs"`
	WeightChanges  WeightChange         `json:"weight_changes"`
}

// RegimeDetectorInputs captures the 3-indicator snapshot at flip time
type RegimeDetectorInputs struct {
	RealizedVol7d   float64 `json:"realized_vol_7d"`
	PctAbove20MA    float64 `json:"pct_above_20ma"`
	BreadthThrust   float64 `json:"breadth_thrust"`
	StabilityScore  float64 `json:"stability_score"`
	ConfidenceLevel float64 `json:"confidence_level"`
}

// WeightChange shows before/after factor weight allocation
type WeightChange struct {
	Before regimeDomain.FactorWeights `json:"before"`
	After  regimeDomain.FactorWeights `json:"after"`
	Delta  regimeDomain.FactorWeights `json:"delta"`
}

// ExitStats tracks exit distribution by regime vs KPI targets
type ExitStats struct {
	Regime          string  `json:"regime"`
	TotalExits      int     `json:"total_exits"`
	TimeLimit       int     `json:"time_limit"` // Target: ≤40%
	HardStop        int     `json:"hard_stop"`  // Target: ≤20%
	MomentumFade    int     `json:"momentum_fade"`
	ProfitTarget    int     `json:"profit_target"` // Target: ≥25%
	VenueHealth     int     `json:"venue_health"`
	Other           int     `json:"other"`
	TimeLimitPct    float64 `json:"time_limit_pct"`
	HardStopPct     float64 `json:"hard_stop_pct"`
	ProfitTargetPct float64 `json:"profit_target_pct"`
	AvgHoldHours    float64 `json:"avg_hold_hours"`
	AvgReturnPct    float64 `json:"avg_return_pct"`
}

// DecileLift shows score→return relationship by regime
type DecileLift struct {
	Regime      string         `json:"regime"`
	Deciles     []DecileBucket `json:"deciles"`
	Correlation float64        `json:"correlation"`
	R2          float64        `json:"r2"`
	Lift        float64        `json:"lift"` // Top decile vs bottom decile
}

// DecileBucket represents one decile of the score distribution
type DecileBucket struct {
	Decile       int     `json:"decile"` // 1-10
	ScoreMin     float64 `json:"score_min"`
	ScoreMax     float64 `json:"score_max"`
	Count        int     `json:"count"`
	AvgScore     float64 `json:"avg_score"`
	AvgReturn48h float64 `json:"avg_return_48h"`
	HitRate      float64 `json:"hit_rate"`
	Sharpe       float64 `json:"sharpe"`
}

// KPIAlert flags violations of regime KPI targets
type KPIAlert struct {
	Type        string  `json:"type"` // "time_limit_breach", "hard_stop_breach", "profit_target_miss"
	Regime      string  `json:"regime"`
	CurrentPct  float64 `json:"current_pct"`
	TargetPct   float64 `json:"target_pct"`
	Severity    string  `json:"severity"` // "warning", "critical"
	Action      string  `json:"action"`   // Recommended remediation
	Description string  `json:"description"`
}

// ReportConfig configures regime report generation
type ReportConfig struct {
	Period        time.Duration `json:"period"`         // Default: 28 days
	OutputDir     string        `json:"output_dir"`     // Default: ./artifacts/reports/
	IncludeCharts bool          `json:"include_charts"` // Generate decile charts
	PITTimestamp  bool          `json:"pit_timestamp"`  // Use point-in-time data only
	KPIThresholds KPIThresholds `json:"kpi_thresholds"`
}

// KPIThresholds defines alert thresholds for regime performance
type KPIThresholds struct {
	TimeLimitMax    float64 `json:"time_limit_max"`    // 40.0%
	HardStopMax     float64 `json:"hard_stop_max"`     // 20.0%
	ProfitTargetMin float64 `json:"profit_target_min"` // 25.0%
	LiftMin         float64 `json:"lift_min"`          // 2.0x minimum lift
	CorrelationMin  float64 `json:"correlation_min"`   // 0.15 minimum correlation
}

// Default KPI thresholds matching prompt requirements
var DefaultKPIThresholds = KPIThresholds{
	TimeLimitMax:    40.0,
	HardStopMax:     20.0,
	ProfitTargetMin: 25.0,
	LiftMin:         2.0,
	CorrelationMin:  0.15,
}
