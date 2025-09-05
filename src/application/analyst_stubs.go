package application

import "context"

// CoverageReport represents analyst coverage analysis results
type CoverageReport struct {
	Metrics               map[string]*CoverageMetrics `json:"metrics"`
	Misses               []CoverageMiss               `json:"misses"`
	HasPolicyViolations  bool                         `json:"has_policy_violations"`
}

// CoverageMetrics holds coverage statistics for a timeframe
type CoverageMetrics struct {
	RecallAt20 float64 `json:"recall_at_20"`
}

// CoverageMiss represents a missed opportunity
type CoverageMiss struct {
	Symbol     string `json:"symbol"`
	ReasonCode string `json:"reason_code"`
}

// AnalystConfig holds analyst configuration
type AnalystConfig struct {
	UseFixtures bool     `json:"use_fixtures"`
	Timeframes  []string `json:"timeframes"`
	OutputDir   string   `json:"output_dir"`
}

// ScanSymbol scans a single symbol (stub implementation)
func (sp *ScanPipeline) ScanSymbol(ctx context.Context, symbol string) (*CandidateResult, error) {
	// Stub implementation for dry run
	return &CandidateResult{
		Symbol:   symbol,
		Selected: false,
	}, nil
}

// RunCoverageAnalysis runs coverage analysis (stub implementation)
func RunCoverageAnalysis(ctx context.Context, config AnalystConfig) (*CoverageReport, error) {
	// Stub implementation returning minimal coverage
	return &CoverageReport{
		Metrics: map[string]*CoverageMetrics{
			"1h":  {RecallAt20: 0.65},
			"24h": {RecallAt20: 0.78},
			"7d":  {RecallAt20: 0.82},
		},
		Misses: []CoverageMiss{
			{Symbol: "BTCUSD", ReasonCode: "FRESHNESS_FAIL"},
			{Symbol: "ETHUSD", ReasonCode: "MICROSTRUCTURE_FAIL"},
		},
		HasPolicyViolations: false,
	}, nil
}