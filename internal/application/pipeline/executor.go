package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	httpmetrics "cryptorun/internal/interfaces/http"
	logprogress "cryptorun/internal/log"
)

// PipelineExecutor manages the execution of CryptoRun scanning pipelines
type PipelineExecutor struct {
	metrics    *httpmetrics.MetricsRegistry
	stepLogger *logprogress.StepLogger
}

// PipelineConfig contains configuration for pipeline execution
type PipelineConfig struct {
	MaxSymbols     int           `json:"max_symbols"`
	TimeoutPerStep time.Duration `json:"timeout_per_step"`
	EnableMetrics  bool          `json:"enable_metrics"`
	EnableProgress bool          `json:"enable_progress"`
	ProgressStyle  string        `json:"progress_style"`
}

// PipelineResult contains the results of pipeline execution
type PipelineResult struct {
	Success        bool                     `json:"success"`
	TotalDuration  time.Duration            `json:"total_duration"`
	StepDurations  map[string]time.Duration `json:"step_durations"`
	ProcessedCount int                      `json:"processed_count"`
	Candidates     []ScanCandidate          `json:"candidates"`
	Errors         []PipelineError          `json:"errors"`
}

// ScanCandidate represents a candidate from the scanning pipeline
type ScanCandidate struct {
	Symbol           string             `json:"symbol"`
	Score            float64            `json:"score"`
	Factors          map[string]float64 `json:"factors"`
	GuardsPassed     []string           `json:"guards_passed"`
	GatesPassed      []string           `json:"gates_passed"`
	ProcessingTimeMs int64              `json:"processing_time_ms"`
}

// PipelineError represents an error that occurred during pipeline execution
type PipelineError struct {
	Step    string `json:"step"`
	Symbol  string `json:"symbol,omitempty"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// NewPipelineExecutor creates a new pipeline executor
func NewPipelineExecutor(config PipelineConfig) *PipelineExecutor {
	var metrics *httpmetrics.MetricsRegistry
	if config.EnableMetrics {
		metrics = httpmetrics.DefaultMetrics
		if metrics == nil {
			httpmetrics.InitializeMetrics()
			metrics = httpmetrics.DefaultMetrics
		}
	}

	// Define pipeline steps
	steps := []string{
		"Universe", "Data Fetch", "Guards", "Factors",
		"Orthogonalize", "Score", "Gates", "Output",
	}

	var stepLogger *logprogress.StepLogger
	if config.EnableProgress {
		stepLogger = logprogress.NewStepLogger("CryptoRun Pipeline", steps)
	}

	return &PipelineExecutor{
		metrics:    metrics,
		stepLogger: stepLogger,
	}
}

// ExecutePipeline runs the complete CryptoRun scanning pipeline
func (pe *PipelineExecutor) ExecutePipeline(ctx context.Context, config PipelineConfig) (*PipelineResult, error) {
	startTime := time.Now()

	if pe.metrics != nil {
		pe.metrics.IncrementActiveScans()
		defer pe.metrics.DecrementActiveScans()
	}

	result := &PipelineResult{
		StepDurations: make(map[string]time.Duration),
		Candidates:    []ScanCandidate{},
		Errors:        []PipelineError{},
	}

	// Execute each pipeline step with timing and progress
	steps := []struct {
		name string
		fn   func(ctx context.Context, config PipelineConfig, result *PipelineResult) error
	}{
		{"Universe", pe.executeUniverseStep},
		{"Data Fetch", pe.executeDataFetchStep},
		{"Guards", pe.executeGuardsStep},
		{"Factors", pe.executeFactorsStep},
		{"Orthogonalize", pe.executeOrthogonalizeStep},
		{"Score", pe.executeScoreStep},
		{"Gates", pe.executeGatesStep},
		{"Output", pe.executeOutputStep},
	}

	for _, step := range steps {
		stepStartTime := time.Now()

		if pe.stepLogger != nil {
			pe.stepLogger.StartStep(step.name)
		}

		// Start step timer for metrics
		var stepTimer *httpmetrics.StepTimer
		if pe.metrics != nil {
			stepTimer = pe.metrics.StartStepTimer(string(httpmetrics.PipelineStep(step.name)))
		}

		// Execute step
		err := step.fn(ctx, config, result)
		stepDuration := time.Since(stepStartTime)
		result.StepDurations[step.name] = stepDuration

		// Complete step timing
		if stepTimer != nil {
			if err != nil {
				stepTimer.Stop(string(httpmetrics.ResultError))
				pe.metrics.RecordPipelineError(step.name, "execution_error")
			} else {
				stepTimer.Stop(string(httpmetrics.ResultSuccess))
			}
		}

		if pe.stepLogger != nil {
			pe.stepLogger.CompleteStep()
		}

		// Handle step errors
		if err != nil {
			result.Success = false
			result.Errors = append(result.Errors, PipelineError{
				Step:    step.name,
				Message: err.Error(),
				Code:    "STEP_EXECUTION_ERROR",
			})

			if pe.stepLogger != nil {
				pe.stepLogger.Fail(err.Error())
			}

			log.Error().
				Str("step", step.name).
				Err(err).
				Dur("step_duration", stepDuration).
				Msg("Pipeline step failed")

			return result, fmt.Errorf("pipeline failed at step %s: %w", step.name, err)
		}

		log.Info().
			Str("step", step.name).
			Dur("duration", stepDuration).
			Msg("Pipeline step completed successfully")
	}

	result.Success = true
	result.TotalDuration = time.Since(startTime)

	if pe.stepLogger != nil {
		pe.stepLogger.Finish()
	}

	log.Info().
		Dur("total_duration", result.TotalDuration).
		Int("candidates", len(result.Candidates)).
		Int("processed_symbols", result.ProcessedCount).
		Msg("Pipeline execution completed successfully")

	return result, nil
}

// executeUniverseStep builds the universe of symbols to scan
func (pe *PipelineExecutor) executeUniverseStep(ctx context.Context, config PipelineConfig, result *PipelineResult) error {
	// Mock universe building - in production this would query actual universe config
	symbols := []string{"BTC", "ETH", "ADA", "SOL", "DOT", "MATIC", "AVAX", "LINK", "UNI", "ATOM"}

	if len(symbols) > config.MaxSymbols {
		symbols = symbols[:config.MaxSymbols]
	}

	result.ProcessedCount = len(symbols)

	log.Info().
		Int("universe_size", len(symbols)).
		Int("max_symbols", config.MaxSymbols).
		Msg("Universe step: symbol universe built")

	return nil
}

// executeDataFetchStep fetches market data for universe symbols
func (pe *PipelineExecutor) executeDataFetchStep(ctx context.Context, config PipelineConfig, result *PipelineResult) error {
	symbols := result.ProcessedCount // Use processed count as symbol count

	// Mock data fetching with progress indication
	progressConfig := logprogress.DefaultProgressConfig()
	progressConfig.SpinnerStyle = logprogress.SpinnerDots

	progress := logprogress.NewProgressIndicator("Fetching market data", symbols, progressConfig)
	defer progress.Finish()

	for i := 0; i < symbols; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Simulate data fetching
			time.Sleep(10 * time.Millisecond)
			progress.Increment()

			// Record cache hits/misses for metrics
			if pe.metrics != nil {
				if i%3 == 0 {
					pe.metrics.RecordCacheMiss("market_data")
				} else {
					pe.metrics.RecordCacheHit("market_data")
				}
			}
		}
	}

	log.Info().
		Int("symbols_fetched", symbols).
		Msg("Data fetch step: market data retrieved")

	return nil
}

// executeGuardsStep applies safety guards to filter candidates
func (pe *PipelineExecutor) executeGuardsStep(ctx context.Context, config PipelineConfig, result *PipelineResult) error {
	symbols := result.ProcessedCount

	// Mock guard filtering
	passedGuards := int(float64(symbols) * 0.7) // 70% pass guards
	result.ProcessedCount = passedGuards

	log.Info().
		Int("input_symbols", symbols).
		Int("passed_guards", passedGuards).
		Float64("pass_rate", 0.7).
		Msg("Guards step: safety guards applied")

	return nil
}

// executeFactorsStep calculates momentum and other factors
func (pe *PipelineExecutor) executeFactorsStep(ctx context.Context, config PipelineConfig, result *PipelineResult) error {
	symbols := result.ProcessedCount

	// Mock factor calculation with progress
	progressConfig := logprogress.DefaultProgressConfig()
	progressConfig.SpinnerStyle = logprogress.SpinnerBounce

	progress := logprogress.NewProgressIndicator("Calculating factors", symbols, progressConfig)
	defer progress.Finish()

	for i := 0; i < symbols; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Simulate factor calculation
			time.Sleep(20 * time.Millisecond)
			progress.UpdateWithMessage(i+1, fmt.Sprintf("Symbol %d factors", i+1))
		}
	}

	log.Info().
		Int("symbols_processed", symbols).
		Msg("Factors step: momentum factors calculated")

	return nil
}

// executeOrthogonalizeStep performs Gram-Schmidt orthogonalization
func (pe *PipelineExecutor) executeOrthogonalizeStep(ctx context.Context, config PipelineConfig, result *PipelineResult) error {
	symbols := result.ProcessedCount

	// Mock orthogonalization
	log.Info().
		Int("symbols", symbols).
		Str("method", "Gram-Schmidt").
		Msg("Orthogonalize step: factor orthogonalization applied")

	return nil
}

// executeScoreStep calculates final scores
func (pe *PipelineExecutor) executeScoreStep(ctx context.Context, config PipelineConfig, result *PipelineResult) error {
	symbols := result.ProcessedCount

	// Generate mock candidates
	for i := 0; i < symbols; i++ {
		candidate := ScanCandidate{
			Symbol: fmt.Sprintf("SYM%d", i),
			Score:  float64(100-i) / 10.0, // Decreasing scores
			Factors: map[string]float64{
				"momentum_1h":  float64(50-i) / 10.0,
				"momentum_4h":  float64(60-i) / 10.0,
				"momentum_24h": float64(40-i) / 10.0,
				"volume":       float64(30-i) / 10.0,
			},
			GuardsPassed:     []string{"fatigue", "freshness", "late_fill"},
			GatesPassed:      []string{"volume", "spread", "depth"},
			ProcessingTimeMs: int64(15 + i*2),
		}
		result.Candidates = append(result.Candidates, candidate)
	}

	log.Info().
		Int("candidates_scored", len(result.Candidates)).
		Msg("Score step: final scores calculated")

	return nil
}

// executeGatesStep applies entry gates for final filtering
func (pe *PipelineExecutor) executeGatesStep(ctx context.Context, config PipelineConfig, result *PipelineResult) error {
	initialCount := len(result.Candidates)

	// Mock gate filtering - keep top 80%
	keepCount := int(float64(initialCount) * 0.8)
	if keepCount < len(result.Candidates) {
		result.Candidates = result.Candidates[:keepCount]
	}

	log.Info().
		Int("input_candidates", initialCount).
		Int("passed_gates", len(result.Candidates)).
		Float64("pass_rate", float64(len(result.Candidates))/float64(initialCount)).
		Msg("Gates step: entry gates applied")

	return nil
}

// executeOutputStep prepares final output
func (pe *PipelineExecutor) executeOutputStep(ctx context.Context, config PipelineConfig, result *PipelineResult) error {
	candidates := len(result.Candidates)

	log.Info().
		Int("final_candidates", candidates).
		Msg("Output step: results prepared for output")

	return nil
}

// GetStepNames returns the standard pipeline step names
func GetStepNames() []string {
	return []string{
		"Universe", "Data Fetch", "Guards", "Factors",
		"Orthogonalize", "Score", "Gates", "Output",
	}
}
