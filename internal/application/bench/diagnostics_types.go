package bench

import (
	"time"
)

// TopGainerData represents a top gainer from external source
type TopGainerData struct {
	Symbol     string    `json:"symbol"`
	Rank       int       `json:"rank"`
	Raw24hGain float64   `json:"raw_24h_gain"`
	SignalTime time.Time `json:"signal_time"`
}

// BenchmarkConfig represents the loaded benchmark configuration
type BenchmarkConfig struct {
	Diagnostics DiagnosticsConfig `yaml:"diagnostics"`
}

// DiagnosticsConfig represents the diagnostics section of config
type DiagnosticsConfig struct {
	SampleSize SampleSizeConfig `yaml:"sample_size"`
	Series     SeriesConfig     `yaml:"series"`
	Simulation SimulationConfig `yaml:"simulation"`
	Gates      struct {
		MinScore     float64 `yaml:"min_score"`
		MaxSpreadBps float64 `yaml:"max_spread_bps"`
		MinDepthUSD  float64 `yaml:"min_depth_usd"`
		MinVADR      float64 `yaml:"min_vadr"`
	} `yaml:"gates"`
	Guards struct {
		Fatigue struct {
			BaselineThreshold  float64 `yaml:"baseline_threshold"`
			TrendingMultiplier float64 `yaml:"trending_multiplier"`
			RSIThreshold       float64 `yaml:"rsi_threshold"`
		} `yaml:"fatigue"`
		Freshness struct {
			MaxBarsAge         int     `yaml:"max_bars_age"`
			TrendingMaxBarsAge int     `yaml:"trending_max_bars_age"`
			ATRFactor          float64 `yaml:"atr_factor"`
		} `yaml:"freshness"`
		LateFill struct {
			MaxDelaySeconds         int `yaml:"max_delay_seconds"`
			TrendingMaxDelaySeconds int `yaml:"trending_max_delay_seconds"`
		} `yaml:"late_fill"`
	} `yaml:"guards"`
	Output struct {
		IncludeRaw24h     bool   `yaml:"include_raw_24h"`
		PrimaryMetric     string `yaml:"primary_metric"`
		SuppressRawAdvice bool   `yaml:"suppress_raw_advice"`
		ShowBothColumns   bool   `yaml:"show_both_columns"`
	} `yaml:"output"`
	Recommendations struct {
		BaseOnSpecPnLOnly  bool    `yaml:"base_on_spec_pnl_only"`
		MinConfidence      float64 `yaml:"min_confidence"`
		MaxRecommendations int     `yaml:"max_recommendations"`
	} `yaml:"recommendations"`
}

// SampleSizeConfig represents sample size requirements
type SampleSizeConfig struct {
	MinPerWindow               int    `yaml:"min_per_window"`
	RequiredForRecommendations bool   `yaml:"required_for_recommendations"`
	InsufficientWindowHandling string `yaml:"insufficient_window_handling"`
}

// SeriesConfig represents price series configuration
type SeriesConfig struct {
	ExchangeNativeFirst bool     `yaml:"exchange_native_first"`
	PreferredExchanges  []string `yaml:"preferred_exchanges"`
	FallbackAggregators []string `yaml:"fallback_aggregators"`
	RequireLabeling     bool     `yaml:"require_labeling"`
}

// SimulationConfig represents P&L simulation configuration
type SimulationConfig struct {
	EntryLogic    string   `yaml:"entry_logic"`
	ExitHierarchy []string `yaml:"exit_hierarchy"`
	RegimeAware   bool     `yaml:"regime_aware"`
}

// SpecCompliantDiagnosticOutput represents the dual-column output format
type SpecCompliantDiagnosticOutput struct {
	Timestamp        time.Time                    `json:"timestamp"`
	Methodology      string                       `json:"methodology"`
	RegimeSnapshot   string                       `json:"regime_snapshot"`
	SampleValidation SampleValidationResult       `json:"sample_validation"`
	WindowAnalysis   map[string]WindowDiagnostics `json:"window_analysis"`
	Insights         ActionableInsightsResult     `json:"actionable_insights"`
}

// SampleValidationResult enforces n≥20 requirement
type SampleValidationResult struct {
	RequiredMinimum        int            `json:"required_minimum"`
	WindowSampleSizes      map[string]int `json:"window_sample_sizes"`
	RecommendationsEnabled bool           `json:"recommendations_enabled"`
	InsufficientWindows    []string       `json:"insufficient_windows"`
}

// WindowDiagnostics with dual-column P&L display
type WindowDiagnostics struct {
	AlignmentScore float64             `json:"alignment_score"`
	TotalGainers   int                 `json:"total_gainers"`
	TotalMatches   int                 `json:"total_matches"`
	Hits           []SpecCompliantHit  `json:"hits"`
	Misses         []SpecCompliantMiss `json:"misses"`
}

// SpecCompliantHit shows both raw and spec P&L
type SpecCompliantHit struct {
	Symbol            string   `json:"symbol"`
	GainPercentage    float64  `json:"gain_percentage"`               // Raw 24h (context only)
	RawGainPercentage *float64 `json:"raw_gain_percentage,omitempty"` // Explicit raw
	SpecCompliantPnL  *float64 `json:"spec_compliant_pnl,omitempty"`  // THE decision metric
	SeriesSource      *string  `json:"series_source,omitempty"`       // exchange_native_binance or aggregator_fallback_coingecko
}

// SpecCompliantMiss shows why missed with spec P&L basis
type SpecCompliantMiss struct {
	Symbol            string   `json:"symbol"`
	GainPercentage    float64  `json:"gain_percentage"`               // Raw 24h (context only)
	RawGainPercentage *float64 `json:"raw_gain_percentage,omitempty"` // Explicit raw
	SpecCompliantPnL  *float64 `json:"spec_compliant_pnl,omitempty"`  // THE decision metric
	SeriesSource      *string  `json:"series_source,omitempty"`       // Source attribution
	PrimaryReason     string   `json:"primary_reason"`                // Gate/guard failure
	ConfigTweak       string   `json:"config_tweak"`                  // ONLY if spec P&L supports
}

// ActionableInsightsResult ensures spec P&L basis for recommendations
type ActionableInsightsResult struct {
	TopImprovements []SpecCompliantImprovement `json:"top_improvements"`
}

// SpecCompliantImprovement ensures recommendations use only spec P&L
type SpecCompliantImprovement struct {
	Change             string `json:"change"`
	Impact             string `json:"impact"`
	Risk               string `json:"risk"`
	RawGainBased       *bool  `json:"raw_gain_based,omitempty"`       // Should be false/null
	SpecCompliantBased *bool  `json:"spec_compliant_based,omitempty"` // Should be true
}

// Helper functions

// loadBenchmarkConfig loads the benchmark configuration
func loadBenchmarkConfig() (*BenchmarkConfig, error) {
	// Mock configuration for demo - in production would load from config/bench.yaml
	config := &BenchmarkConfig{
		Diagnostics: DiagnosticsConfig{
			SampleSize: SampleSizeConfig{
				MinPerWindow:               20,
				RequiredForRecommendations: true,
				InsufficientWindowHandling: "disable_recommendations",
			},
			Series: SeriesConfig{
				ExchangeNativeFirst: true,
				PreferredExchanges:  []string{"binance", "kraken", "coinbase", "okx"},
				FallbackAggregators: []string{"coingecko", "dexscreener"},
				RequireLabeling:     true,
			},
			Simulation: SimulationConfig{
				EntryLogic:    "first_bar_after_signal_with_gates_guards",
				ExitHierarchy: []string{"hard_stop", "venue_health", "time_limit_48h", "accel_reversal", "momentum_fade", "profit_target"},
				RegimeAware:   true,
			},
		},
	}

	// Set default gate/guard values
	config.Diagnostics.Gates.MinScore = 2.0
	config.Diagnostics.Gates.MaxSpreadBps = 50.0
	config.Diagnostics.Gates.MinDepthUSD = 100000.0
	config.Diagnostics.Gates.MinVADR = 1.75

	config.Diagnostics.Guards.Fatigue.BaselineThreshold = 12.0
	config.Diagnostics.Guards.Fatigue.TrendingMultiplier = 1.5
	config.Diagnostics.Guards.Fatigue.RSIThreshold = 70.0

	config.Diagnostics.Guards.Freshness.MaxBarsAge = 2
	config.Diagnostics.Guards.Freshness.TrendingMaxBarsAge = 3
	config.Diagnostics.Guards.Freshness.ATRFactor = 1.2

	config.Diagnostics.Guards.LateFill.MaxDelaySeconds = 30
	config.Diagnostics.Guards.LateFill.TrendingMaxDelaySeconds = 45

	config.Diagnostics.Output.IncludeRaw24h = true
	config.Diagnostics.Output.PrimaryMetric = "spec_pnl_pct"
	config.Diagnostics.Output.SuppressRawAdvice = true
	config.Diagnostics.Output.ShowBothColumns = true

	config.Diagnostics.Recommendations.BaseOnSpecPnLOnly = true
	config.Diagnostics.Recommendations.MinConfidence = 0.75
	config.Diagnostics.Recommendations.MaxRecommendations = 5

	return config, nil
}

// contains checks if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// findRank finds the rank of item in slice (1-indexed)
func findRank(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i + 1
		}
	}
	return -1
}
