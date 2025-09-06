package http

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_model/go"
	"github.com/rs/zerolog/log"
)

// MetricsRegistry holds all Prometheus metrics for CryptoRun
type MetricsRegistry struct {
	// Step duration metrics
	StepDuration *prometheus.HistogramVec

	// Cache performance metrics
	CacheHitRatio prometheus.Gauge
	CacheHits     *prometheus.CounterVec
	CacheMisses   *prometheus.CounterVec

	// WebSocket latency metrics
	WSLatency *prometheus.HistogramVec

	// Pipeline performance metrics
	PipelineSteps  *prometheus.CounterVec
	PipelineErrors *prometheus.CounterVec

	// System metrics
	ActiveScans prometheus.Gauge
	TotalScans  prometheus.Counter

	// Regime metrics
	RegimeSwitches *prometheus.CounterVec
	RegimeDuration *prometheus.HistogramVec
	ActiveRegime   prometheus.Gauge
	RegimeHealth   *prometheus.GaugeVec
}

// NewMetricsRegistry creates a new metrics registry with all CryptoRun metrics
func NewMetricsRegistry() *MetricsRegistry {
	registry := &MetricsRegistry{
		StepDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cryptorun_step_duration_seconds",
				Help:    "Duration of each pipeline step in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
			},
			[]string{"step", "result"},
		),

		CacheHitRatio: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "cryptorun_cache_hit_ratio",
				Help: "Current cache hit ratio (0.0 to 1.0)",
			},
		),

		CacheHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cryptorun_cache_hits_total",
				Help: "Total number of cache hits by cache type",
			},
			[]string{"cache_type"},
		),

		CacheMisses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cryptorun_cache_misses_total",
				Help: "Total number of cache misses by cache type",
			},
			[]string{"cache_type"},
		),

		WSLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cryptorun_ws_latency_ms",
				Help:    "WebSocket round-trip latency in milliseconds",
				Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
			},
			[]string{"exchange", "endpoint"},
		),

		PipelineSteps: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cryptorun_pipeline_steps_total",
				Help: "Total number of pipeline steps executed",
			},
			[]string{"step", "status"},
		),

		PipelineErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cryptorun_pipeline_errors_total",
				Help: "Total number of pipeline errors by step",
			},
			[]string{"step", "error_type"},
		),

		ActiveScans: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "cryptorun_active_scans",
				Help: "Number of currently active scans",
			},
		),

		TotalScans: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "cryptorun_scans_total",
				Help: "Total number of scans initiated",
			},
		),

		RegimeSwitches: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cryptorun_regime_switches_total",
				Help: "Total number of regime switches by from/to regime",
			},
			[]string{"from_regime", "to_regime"},
		),

		RegimeDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cryptorun_regime_duration_hours",
				Help:    "Duration of regime periods in hours",
				Buckets: []float64{0.5, 1, 2, 4, 8, 12, 24, 48, 72, 168},
			},
			[]string{"regime"},
		),

		ActiveRegime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "cryptorun_active_regime",
				Help: "Current active regime (0=choppy, 1=bull, 2=highvol)",
			},
		),

		RegimeHealth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cryptorun_regime_health",
				Help: "Regime detection health metrics",
			},
			[]string{"regime", "indicator"},
		),
	}

	// Register all metrics with Prometheus
	prometheus.MustRegister(
		registry.StepDuration,
		registry.CacheHitRatio,
		registry.CacheHits,
		registry.CacheMisses,
		registry.WSLatency,
		registry.PipelineSteps,
		registry.PipelineErrors,
		registry.ActiveScans,
		registry.TotalScans,
		registry.RegimeSwitches,
		registry.RegimeDuration,
		registry.ActiveRegime,
		registry.RegimeHealth,
	)

	return registry
}

// StepTimer tracks execution time for pipeline steps
type StepTimer struct {
	metrics *MetricsRegistry
	step    string
	start   time.Time
}

// StartStepTimer begins timing a pipeline step
func (m *MetricsRegistry) StartStepTimer(step string) *StepTimer {
	return &StepTimer{
		metrics: m,
		step:    step,
		start:   time.Now(),
	}
}

// Stop completes the step timing and records the metric
func (st *StepTimer) Stop(result string) {
	duration := time.Since(st.start)
	st.metrics.StepDuration.WithLabelValues(st.step, result).Observe(duration.Seconds())
	st.metrics.PipelineSteps.WithLabelValues(st.step, result).Inc()

	log.Debug().
		Str("step", st.step).
		Str("result", result).
		Dur("duration", duration).
		Msg("Pipeline step completed")
}

// RecordCacheHit records a cache hit for the specified cache type
func (m *MetricsRegistry) RecordCacheHit(cacheType string) {
	m.CacheHits.WithLabelValues(cacheType).Inc()
	m.updateCacheHitRatio()
}

// RecordCacheMiss records a cache miss for the specified cache type
func (m *MetricsRegistry) RecordCacheMiss(cacheType string) {
	m.CacheMisses.WithLabelValues(cacheType).Inc()
	m.updateCacheHitRatio()
}

// RecordWSLatency records WebSocket latency
func (m *MetricsRegistry) RecordWSLatency(exchange, endpoint string, latencyMs float64) {
	m.WSLatency.WithLabelValues(exchange, endpoint).Observe(latencyMs)
}

// RecordPipelineError records a pipeline error
func (m *MetricsRegistry) RecordPipelineError(step, errorType string) {
	m.PipelineErrors.WithLabelValues(step, errorType).Inc()
	log.Warn().
		Str("step", step).
		Str("error_type", errorType).
		Msg("Pipeline error recorded")
}

// IncrementActiveScans increments the active scans counter
func (m *MetricsRegistry) IncrementActiveScans() {
	m.ActiveScans.Inc()
	m.TotalScans.Inc()
}

// DecrementActiveScans decrements the active scans counter
func (m *MetricsRegistry) DecrementActiveScans() {
	m.ActiveScans.Dec()
}

// updateCacheHitRatio calculates and updates the cache hit ratio
func (m *MetricsRegistry) updateCacheHitRatio() {
	// Get current metrics values
	hitMetrics := &io_prometheus_client.Metric{}
	missMetrics := &io_prometheus_client.Metric{}

	// Sum all cache hits and misses across cache types
	totalHits := 0.0
	totalMisses := 0.0

	// In production, we would iterate through all cache type labels
	// For now, use a simplified calculation
	cacheTypes := []string{"market_data", "momentum", "regime", "universe"}

	for _, cacheType := range cacheTypes {
		if hitCounter, err := m.CacheHits.GetMetricWithLabelValues(cacheType); err == nil {
			if err := hitCounter.Write(hitMetrics); err == nil {
				totalHits += hitMetrics.GetCounter().GetValue()
			}
		}

		if missCounter, err := m.CacheMisses.GetMetricWithLabelValues(cacheType); err == nil {
			if err := missCounter.Write(missMetrics); err == nil {
				totalMisses += missMetrics.GetCounter().GetValue()
			}
		}
	}

	// Calculate hit ratio
	total := totalHits + totalMisses
	if total > 0 {
		ratio := totalHits / total
		m.CacheHitRatio.Set(ratio)
	}
}

// MetricsHandler returns an HTTP handler for Prometheus metrics
func (m *MetricsRegistry) MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RegimeStatusHandler returns regime information as JSON
func (m *MetricsRegistry) RegimeStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get current regime value
		currentRegime := getCurrentRegimeFromGauge(m.ActiveRegime)

		// Mock regime health data - in production this would come from the regime detector
		response := map[string]interface{}{
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
			"current_regime": currentRegime,
			"regime_numeric": regimeToGaugeValue(currentRegime),
			"health": map[string]interface{}{
				"volatility_7d":   0.45,
				"above_ma_pct":    0.68,
				"breadth_thrust":  0.23,
				"stability_score": 0.85,
			},
			"weights":            getRegimeWeightsFromGauge(currentRegime),
			"switches_today":     2,
			"avg_duration_hours": 18.5,
		}

		// Write JSON response
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
  "timestamp": "%s",
  "current_regime": "%s", 
  "regime_numeric": %.1f,
  "health": {
    "volatility_7d": %.2f,
    "above_ma_pct": %.2f,
    "breadth_thrust": %.2f,
    "stability_score": %.2f
  },
  "weights": {
    "momentum": %.1f,
    "technical": %.1f,
    "volume": %.1f,
    "quality": %.1f,
    "catalyst": %.1f
  },
  "switches_today": %d,
  "avg_duration_hours": %.1f
}`,
			response["timestamp"],
			response["current_regime"],
			response["regime_numeric"],
			0.45, 0.68, 0.23, 0.85, // health metrics
			50.0, 20.0, 15.0, 10.0, 5.0, // weights
			2, 18.5) // switches and duration
	}
}

// getCurrentRegimeFromGauge converts gauge value back to regime string
func getCurrentRegimeFromGauge(gauge prometheus.Gauge) string {
	// In production, this would use the gauge's current value
	// For now, return a default regime
	return "trending_bull"
}

// getRegimeWeightsFromGauge returns weights for a regime
func getRegimeWeightsFromGauge(regime string) map[string]float64 {
	switch strings.ToLower(regime) {
	case "trending_bull", "bull", "trending":
		return map[string]float64{
			"momentum": 50.0, "technical": 20.0, "volume": 15.0, "quality": 10.0, "catalyst": 5.0,
		}
	case "choppy", "chop", "ranging":
		return map[string]float64{
			"momentum": 35.0, "technical": 30.0, "volume": 15.0, "quality": 15.0, "catalyst": 5.0,
		}
	case "high_vol", "volatile", "high_volatility", "highvol":
		return map[string]float64{
			"momentum": 30.0, "technical": 25.0, "volume": 20.0, "quality": 20.0, "catalyst": 5.0,
		}
	default:
		return map[string]float64{
			"momentum": 35.0, "technical": 30.0, "volume": 15.0, "quality": 15.0, "catalyst": 5.0,
		}
	}
}

// PipelineStep represents the different steps in the CryptoRun pipeline
type PipelineStep string

const (
	StepUniverse      PipelineStep = "universe"
	StepDataFetch     PipelineStep = "data_fetch"
	StepGuards        PipelineStep = "guards"
	StepFactors       PipelineStep = "factors"
	StepOrthogonalize PipelineStep = "orthogonalize"
	StepScore         PipelineStep = "score"
	StepGates         PipelineStep = "gates"
	StepOutput        PipelineStep = "output"
)

// PipelineResult represents the result of a pipeline step
type PipelineResult string

const (
	ResultSuccess PipelineResult = "success"
	ResultError   PipelineResult = "error"
	ResultSkipped PipelineResult = "skipped"
	ResultTimeout PipelineResult = "timeout"
)

// Global metrics registry instance
var DefaultMetrics *MetricsRegistry

// InitializeMetrics initializes the global metrics registry
func InitializeMetrics() {
	DefaultMetrics = NewMetricsRegistry()
	log.Info().Msg("Prometheus metrics registry initialized")
}

// GetP99Latency returns the P99 WebSocket latency for monitoring
func (m *MetricsRegistry) GetP99Latency(exchange, endpoint string) float64 {
	// In production, this would query the histogram for P99 value
	// For now, return a mock value that would be exposed in /metrics
	return 125.0 // Mock P99 latency in milliseconds
}

// RecordRegimeSwitch records a regime transition
func (m *MetricsRegistry) RecordRegimeSwitch(fromRegime, toRegime string) {
	m.RegimeSwitches.WithLabelValues(fromRegime, toRegime).Inc()

	// Update active regime gauge
	regimeValue := regimeToGaugeValue(toRegime)
	m.ActiveRegime.Set(regimeValue)

	log.Info().
		Str("from_regime", fromRegime).
		Str("to_regime", toRegime).
		Float64("gauge_value", regimeValue).
		Msg("Regime switch recorded")
}

// RecordRegimeDuration records how long a regime lasted
func (m *MetricsRegistry) RecordRegimeDuration(regime string, durationHours float64) {
	m.RegimeDuration.WithLabelValues(regime).Observe(durationHours)

	log.Debug().
		Str("regime", regime).
		Float64("duration_hours", durationHours).
		Msg("Regime duration recorded")
}

// UpdateRegimeHealth records regime detection health indicators
func (m *MetricsRegistry) UpdateRegimeHealth(regime string, indicators map[string]float64) {
	for indicator, value := range indicators {
		m.RegimeHealth.WithLabelValues(regime, indicator).Set(value)
	}

	log.Debug().
		Str("regime", regime).
		Interface("indicators", indicators).
		Msg("Regime health indicators updated")
}

// SetActiveRegime updates the current active regime
func (m *MetricsRegistry) SetActiveRegime(regime string) {
	regimeValue := regimeToGaugeValue(regime)
	m.ActiveRegime.Set(regimeValue)
}

// regimeToGaugeValue converts regime string to numeric value for gauge
func regimeToGaugeValue(regime string) float64 {
	switch strings.ToLower(regime) {
	case "choppy", "chop", "ranging":
		return 0.0
	case "trending_bull", "bull", "trending":
		return 1.0
	case "high_vol", "volatile", "high_volatility", "highvol":
		return 2.0
	default:
		return -1.0 // Unknown/manual override
	}
}
