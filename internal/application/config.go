package application

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type APIsConfig struct {
	PrimaryExchange string `yaml:"primary_exchange"`
	Budgets         struct {
		MonthlyLimitUSD      int `yaml:"monthly_limit_usd"`
		SwitchAtRemainingUSD int `yaml:"switch_at_remaining_usd"`
	} `yaml:"budgets"`
}

func LoadAPIsConfig(path string) (*APIsConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c APIsConfig
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

type CacheConfig struct {
	Redis struct {
		Addr              string
		DB                int
		TLS               bool
		DefaultTTLSeconds int `yaml:"default_ttl_seconds"`
	} `yaml:"redis"`
}

func LoadCacheConfig(path string) (*CacheConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c CacheConfig
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *CacheConfig) DefaultTTL() time.Duration {
	return time.Duration(c.Redis.DefaultTTLSeconds) * time.Second
}

// WeightsConfig defines the unified factor weights configuration
type WeightsConfig struct {
	DefaultRegime string `yaml:"default_regime"`
	Validation    struct {
		WeightSumTolerance float64 `yaml:"weight_sum_tolerance"`
		MinMomentumWeight  float64 `yaml:"min_momentum_weight"`
		MaxSocialWeight    float64 `yaml:"max_social_weight"`
		SocialHardCap      float64 `yaml:"social_hard_cap"`
	} `yaml:"validation"`
	Regimes map[string]RegimeWeights `yaml:"regimes"`
	Orthogonalization OrthogonalizationConfig `yaml:"orthogonalization"`
	FactorDefinitions map[string]FactorDefinition `yaml:"factor_definitions"`
	QARequirements    QARequirements `yaml:"qa_requirements"`
	TestFixtures      TestFixtures `yaml:"test_fixtures"`
}

// RegimeWeights defines factor weights for a specific market regime
type RegimeWeights struct {
	Description       string  `yaml:"description"`
	MomentumCore      float64 `yaml:"momentum_core"`
	TechnicalResidual float64 `yaml:"technical_residual"`
	VolumeResidual    float64 `yaml:"volume_residual"`
	QualityResidual   float64 `yaml:"quality_residual"`
	SocialResidual    float64 `yaml:"social_residual"`
}

// OrthogonalizationConfig defines the Gram-Schmidt orthogonalization sequence
type OrthogonalizationConfig struct {
	ProtectedFactors []string `yaml:"protected_factors"`
	Sequence         []string `yaml:"sequence"`
}

// FactorDefinition defines metadata for a factor
type FactorDefinition struct {
	Description        string   `yaml:"description"`
	Protected          bool     `yaml:"protected"`
	Range              string   `yaml:"range"`
	ResidualizedAgainst []string `yaml:"residualized_against"`
	PostProcessing     string   `yaml:"post_processing"`
}

// QARequirements defines quality assurance thresholds
type QARequirements struct {
	CorrelationThreshold float64 `yaml:"correlation_threshold"`
	WeightSumExact      float64 `yaml:"weight_sum_exact"`
	MomentumMinimum     float64 `yaml:"momentum_minimum"`
	SocialMaximum       float64 `yaml:"social_maximum"`
}

// TestFixtures defines test configuration for validation
type TestFixtures struct {
	SampleSize               int                        `yaml:"sample_size"`
	AcceptableCorrelations map[string][2]float64 `yaml:"acceptable_correlations"`
}

// Validate checks if the weights configuration is valid
func (w *WeightsConfig) Validate() error {
	for regimeName, regime := range w.Regimes {
		total := regime.MomentumCore + regime.TechnicalResidual + regime.VolumeResidual + regime.QualityResidual + regime.SocialResidual
		if abs(total-1.0) > w.Validation.WeightSumTolerance {
			return fmt.Errorf("regime %s weights sum to %.6f, expected 1.0 ±%.3f", regimeName, total, w.Validation.WeightSumTolerance)
		}
		
		if regime.MomentumCore < w.Validation.MinMomentumWeight {
			return fmt.Errorf("regime %s momentum weight %.3f below minimum %.3f", regimeName, regime.MomentumCore, w.Validation.MinMomentumWeight)
		}
		
		if regime.SocialResidual > w.Validation.MaxSocialWeight {
			return fmt.Errorf("regime %s social weight %.3f above maximum %.3f", regimeName, regime.SocialResidual, w.Validation.MaxSocialWeight)
		}
	}
	return nil
}

// LoadWeightsConfig loads and validates the weights configuration
func LoadWeightsConfig(path string) (*WeightsConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read weights config: %w", err)
	}
	
	var config WeightsConfig
	if err := yaml.Unmarshal(b, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal weights config: %w", err)
	}
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("weights config validation failed: %w", err)
	}
	
	return &config, nil
}

// GuardsConfig defines the safety guard configuration
type GuardsConfig struct {
	RegimeAware   bool                      `yaml:"regime_aware"`
	ActiveProfile string                    `yaml:"active_profile"`
	Profiles      map[string]GuardProfile   `yaml:"profiles"`
}

// GuardProfile defines guard thresholds for different market conditions
type GuardProfile struct {
	Name        string                         `yaml:"name"`
	Description string                         `yaml:"description"`
	Regimes     map[string]RegimeGuardSettings `yaml:"regimes"`
}

// RegimeGuardSettings defines guard settings for a specific regime
type RegimeGuardSettings struct {
	Fatigue   FatigueGuardConfig   `yaml:"fatigue"`
	Freshness FreshnessGuardConfig `yaml:"freshness"`
	LateFill  LateFillGuardConfig  `yaml:"late_fill"`
}

// FatigueGuardConfig prevents chasing overextended moves
type FatigueGuardConfig struct {
	Threshold24h float64 `yaml:"threshold_24h"`
	RSI4h        int     `yaml:"rsi_4h"`
}

// FreshnessGuardConfig ensures signal recency
type FreshnessGuardConfig struct {
	MaxBarsAge int     `yaml:"max_bars_age"`
	ATRFactor  float64 `yaml:"atr_factor"`
}

// LateFillGuardConfig prevents late execution
type LateFillGuardConfig struct {
	MaxDelaySeconds int `yaml:"max_delay_seconds"`
	P99LatencyReq   int `yaml:"p99_latency_req"`
	ATRProximity    float64 `yaml:"atr_proximity"`
}

// GetActiveGuardSettings returns the guard settings for the active profile and regime
func (g *GuardsConfig) GetActiveGuardSettings(regime string) (RegimeGuardSettings, error) {
	profile, exists := g.Profiles[g.ActiveProfile]
	if !exists {
		return RegimeGuardSettings{}, fmt.Errorf("active profile %s not found", g.ActiveProfile)
	}
	
	settings, exists := profile.Regimes[regime]
	if !exists {
		return RegimeGuardSettings{}, fmt.Errorf("regime %s not found in profile %s", regime, g.ActiveProfile)
	}
	
	return settings, nil
}

// LoadGuardsConfig loads and validates the guards configuration
func LoadGuardsConfig(path string) (*GuardsConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read guards config: %w", err)
	}
	
	var config GuardsConfig
	if err := yaml.Unmarshal(b, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal guards config: %w", err)
	}
	
	return &config, nil
}

// LimitsConfig defines system-wide limits and thresholds
type LimitsConfig struct {
	Scanning ScanningLimits `yaml:"scanning"`
	Entry    EntryLimits    `yaml:"entry"`
	Exit     ExitLimits     `yaml:"exit"`
	Risk     RiskLimits     `yaml:"risk"`
	System   SystemLimits   `yaml:"system"`
}

// ScanningLimits defines limits for scanning operations
type ScanningLimits struct {
	MaxPairs              int           `yaml:"max_pairs"`
	MaxConcurrentScans    int           `yaml:"max_concurrent_scans"`
	ScanTimeoutSeconds    int           `yaml:"scan_timeout_seconds"`
	MinScoreThreshold     float64       `yaml:"min_score_threshold"`
	DataFreshnessSeconds  int           `yaml:"data_freshness_seconds"`
	CacheHitRateMinimum   float64       `yaml:"cache_hit_rate_minimum"`
}

// EntryLimits defines limits for trade entry
type EntryLimits struct {
	MinScore                float64 `yaml:"min_score"`
	MinVADR                 float64 `yaml:"min_vadr"`
	MaxSpreadBps            float64 `yaml:"max_spread_bps"`
	MinDepthUSD             float64 `yaml:"min_depth_usd"`
	MinFundingDivergenceSigma float64 `yaml:"min_funding_divergence_sigma"`
	MinVolumeMultiple       float64 `yaml:"min_volume_multiple"`
	MinADX                  float64 `yaml:"min_adx"`
	MinHurst                float64 `yaml:"min_hurst"`
}

// ExitLimits defines limits for trade exit
type ExitLimits struct {
	MaxPositionHours        int     `yaml:"max_position_hours"`
	StopLossPercent         float64 `yaml:"stop_loss_percent"`
	TrailingStopPercent     float64 `yaml:"trailing_stop_percent"`
	ProfitTargetPercent     float64 `yaml:"profit_target_percent"`
	MaxDrawdownPercent      float64 `yaml:"max_drawdown_percent"`
	VenueHealthThreshold    float64 `yaml:"venue_health_threshold"`
}

// RiskLimits defines risk management limits
type RiskLimits struct {
	MaxPositionSizeUSD      float64 `yaml:"max_position_size_usd"`
	MaxDailyLossPercent     float64 `yaml:"max_daily_loss_percent"`
	MaxWeeklyLossPercent    float64 `yaml:"max_weekly_loss_percent"`
	MaxMonthlyLossPercent   float64 `yaml:"max_monthly_loss_percent"`
	MaxCorrelatedPositions  int     `yaml:"max_correlated_positions"`
	MaxLeverageRatio        float64 `yaml:"max_leverage_ratio"`
}

// SystemLimits defines system performance limits
type SystemLimits struct {
	MaxMemoryMB             int           `yaml:"max_memory_mb"`
	MaxCPUPercent           int           `yaml:"max_cpu_percent"`
	MaxGoroutines           int           `yaml:"max_goroutines"`
	P99LatencyMaxMs         int           `yaml:"p99_latency_max_ms"`
	MaxRequestsPerSecond    int           `yaml:"max_requests_per_second"`
	CircuitBreakerThreshold int           `yaml:"circuit_breaker_threshold"`
	HealthCheckIntervalSec  int           `yaml:"health_check_interval_sec"`
}

// LoadLimitsConfig loads the limits configuration
func LoadLimitsConfig(path string) (*LimitsConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read limits config: %w", err)
	}
	
	var config LimitsConfig
	if err := yaml.Unmarshal(b, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal limits config: %w", err)
	}
	
	return &config, nil
}

// FeatureFlagsConfig defines feature toggles and experimental functionality
type FeatureFlagsConfig struct {
	Experimental ExperimentalFeatures `yaml:"experimental"`
	Core         CoreFeatures         `yaml:"core"`
	API          APIFeatures          `yaml:"api"`
	Monitoring   MonitoringFeatures   `yaml:"monitoring"`
	Safety       SafetyFeatures       `yaml:"safety"`
}

// ExperimentalFeatures defines experimental functionality toggles
type ExperimentalFeatures struct {
	EnableQuantumFactors    bool `yaml:"enable_quantum_factors"`
	EnableAIOrthogonalization bool `yaml:"enable_ai_orthogonalization"`
	EnablePredictiveGuards  bool `yaml:"enable_predictive_guards"`
	EnableMLRegimeDetection bool `yaml:"enable_ml_regime_detection"`
	EnableAdvancedCaching   bool `yaml:"enable_advanced_caching"`
}

// CoreFeatures defines core functionality toggles
type CoreFeatures struct {
	EnableRegimeAwareWeights bool `yaml:"enable_regime_aware_weights"`
	EnableAdaptiveGuards     bool `yaml:"enable_adaptive_guards"`
	EnableSocialCapOverride  bool `yaml:"enable_social_cap_override"`
	EnableFactorValidation   bool `yaml:"enable_factor_validation"`
	EnableCorrelationChecks  bool `yaml:"enable_correlation_checks"`
}

// APIFeatures defines API functionality toggles
type APIFeatures struct {
	EnableDebugEndpoints     bool `yaml:"enable_debug_endpoints"`
	EnableMetricsExport      bool `yaml:"enable_metrics_export"`
	EnableHealthChecks       bool `yaml:"enable_health_checks"`
	EnableAdminEndpoints     bool `yaml:"enable_admin_endpoints"`
	EnableRealTimeStreaming  bool `yaml:"enable_real_time_streaming"`
}

// MonitoringFeatures defines monitoring functionality toggles
type MonitoringFeatures struct {
	EnablePerformanceMonitoring bool `yaml:"enable_performance_monitoring"`
	EnableDetailedTracing       bool `yaml:"enable_detailed_tracing"`
	EnableAuditLogging          bool `yaml:"enable_audit_logging"`
	EnableAlerts                bool `yaml:"enable_alerts"`
	EnableMetricsCollection     bool `yaml:"enable_metrics_collection"`
}

// SafetyFeatures defines safety functionality toggles
type SafetyFeatures struct {
	EnableDryRun            bool `yaml:"enable_dry_run"`
	EnableGuardValidation   bool `yaml:"enable_guard_validation"`
	EnableSafetyChecks      bool `yaml:"enable_safety_checks"`
	EnableEmergencyShutdown bool `yaml:"enable_emergency_shutdown"`
	EnableFailsafes         bool `yaml:"enable_failsafes"`
}

// IsEnabled checks if a feature is enabled based on the feature path
func (f *FeatureFlagsConfig) IsEnabled(featurePath string) bool {
	switch featurePath {
	// Experimental features
	case "experimental.quantum_factors":
		return f.Experimental.EnableQuantumFactors
	case "experimental.ai_orthogonalization":
		return f.Experimental.EnableAIOrthogonalization
	case "experimental.predictive_guards":
		return f.Experimental.EnablePredictiveGuards
	case "experimental.ml_regime_detection":
		return f.Experimental.EnableMLRegimeDetection
	case "experimental.advanced_caching":
		return f.Experimental.EnableAdvancedCaching
	
	// Core features
	case "core.regime_aware_weights":
		return f.Core.EnableRegimeAwareWeights
	case "core.adaptive_guards":
		return f.Core.EnableAdaptiveGuards
	case "core.social_cap_override":
		return f.Core.EnableSocialCapOverride
	case "core.factor_validation":
		return f.Core.EnableFactorValidation
	case "core.correlation_checks":
		return f.Core.EnableCorrelationChecks
	
	// API features
	case "api.debug_endpoints":
		return f.API.EnableDebugEndpoints
	case "api.metrics_export":
		return f.API.EnableMetricsExport
	case "api.health_checks":
		return f.API.EnableHealthChecks
	case "api.admin_endpoints":
		return f.API.EnableAdminEndpoints
	case "api.real_time_streaming":
		return f.API.EnableRealTimeStreaming
	
	// Monitoring features
	case "monitoring.performance_monitoring":
		return f.Monitoring.EnablePerformanceMonitoring
	case "monitoring.detailed_tracing":
		return f.Monitoring.EnableDetailedTracing
	case "monitoring.audit_logging":
		return f.Monitoring.EnableAuditLogging
	case "monitoring.alerts":
		return f.Monitoring.EnableAlerts
	case "monitoring.metrics_collection":
		return f.Monitoring.EnableMetricsCollection
	
	// Safety features
	case "safety.dry_run":
		return f.Safety.EnableDryRun
	case "safety.guard_validation":
		return f.Safety.EnableGuardValidation
	case "safety.safety_checks":
		return f.Safety.EnableSafetyChecks
	case "safety.emergency_shutdown":
		return f.Safety.EnableEmergencyShutdown
	case "safety.failsafes":
		return f.Safety.EnableFailsafes
	
	default:
		return false
	}
}

// LoadFeatureFlagsConfig loads the feature flags configuration
func LoadFeatureFlagsConfig(path string) (*FeatureFlagsConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read feature flags config: %w", err)
	}
	
	var config FeatureFlagsConfig
	if err := yaml.Unmarshal(b, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feature flags config: %w", err)
	}
	
	return &config, nil
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
