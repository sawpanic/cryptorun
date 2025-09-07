package qa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

type Config struct {
	Progress     string
	Resume       bool
	TTL          time.Duration
	Venues       []string
	MaxSample    int
	ArtifactsDir string
	AuditDir     string
	ProviderTTL  time.Duration
	Verify       bool
	FailOnStubs  bool
}

type PhaseResult struct {
	Phase     int                    `json:"phase"`
	Name      string                 `json:"name"`
	Status    string                 `json:"status"` // "pass", "fail", "skip"
	Duration  time.Duration          `json:"duration"`
	Error     string                 `json:"error,omitempty"`
	Artifacts []string               `json:"artifacts,omitempty"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
}

type RunResult struct {
	Success          bool          `json:"success"`
	FailureReason    string        `json:"failure_reason,omitempty"`
	Hint             string        `json:"hint,omitempty"`
	PhaseResults     []PhaseResult `json:"phase_results"`
	ArtifactsPath    string        `json:"artifacts_path"`
	PassedPhases     int           `json:"passed_phases"`
	TotalPhases      int           `json:"total_phases"`
	HealthyProviders int           `json:"healthy_providers"`
	TotalDuration    time.Duration `json:"total_duration"`
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time"`
}

type Runner struct {
	config   Config
	phases   []Phase
	printer  Printer
	progress ProgressTracker
}

func NewRunner(config Config) *Runner {
	runner := &Runner{
		config: config,
		phases: GetQAPhases(),
	}

	// Initialize printer based on progress mode
	switch config.Progress {
	case "json":
		runner.printer = NewJSONPrinter()
	case "plain":
		runner.printer = NewPlainPrinter()
	default: // "auto"
		runner.printer = NewAutoPrinter()
	}

	runner.progress = NewProgressTracker(config.AuditDir)

	return runner
}

func (r *Runner) Run(ctx context.Context) (*RunResult, error) {
	startTime := time.Now()

	// Ensure directories exist
	if err := r.ensureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	totalPhases := len(r.phases)
	if r.config.Verify {
		totalPhases++ // Add acceptance phase
	}

	r.printer.Start(totalPhases)

	result := &RunResult{
		Success:      true,
		StartTime:    startTime,
		TotalPhases:  totalPhases,
		PhaseResults: make([]PhaseResult, 0, totalPhases),
	}

	// Phase -1: No-Stub Gate (if enabled)
	if r.config.FailOnStubs {
		if err := r.executeNoStubGate(result); err != nil {
			return result, nil // Result already has failure details
		}
	}

	// Check for resume point
	resumePhase := 0
	if r.config.Resume {
		if lastPhase, err := r.progress.GetLastCompletedPhase(); err == nil {
			resumePhase = lastPhase + 1
			log.Info().Int("resume_phase", resumePhase).Msg("Resuming QA from checkpoint")
		}
	}

	// Execute phases 0-6
	for i, phase := range r.phases {
		if i < resumePhase {
			log.Debug().Int("phase", i).Str("name", phase.Name()).Msg("Skipping completed phase")
			continue
		}

		phaseResult := r.executePhase(ctx, i, phase)
		result.PhaseResults = append(result.PhaseResults, phaseResult)

		r.printer.Phase(phaseResult)

		if phaseResult.Status == "fail" {
			result.Success = false
			result.FailureReason = phaseResult.Error
			result.Hint = phase.GetHint(phaseResult.Error)
			break
		}

		if phaseResult.Status == "pass" {
			result.PassedPhases++
		}

		// Record progress
		if err := r.progress.RecordPhase(i, phaseResult); err != nil {
			log.Warn().Err(err).Int("phase", i).Msg("Failed to record progress")
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			result.Success = false
			result.FailureReason = "Cancelled by timeout or user"
			return result, ctx.Err()
		default:
		}
	}

	// Generate artifacts from phases 0-6
	if result.Success {
		artifactGen := NewArtifactGenerator(r.config.ArtifactsDir)
		if err := artifactGen.Generate(result); err != nil {
			log.Warn().Err(err).Msg("Failed to generate artifacts")
		} else {
			result.ArtifactsPath = r.config.ArtifactsDir
		}
	}

	// Phase 7: Acceptance Verification (if enabled and phases 0-6 passed)
	if r.config.Verify && result.Success {
		if err := r.executeAcceptanceVerification(result); err != nil {
			return result, nil // Result already has failure details
		}
	}

	endTime := time.Now()
	result.EndTime = endTime
	result.TotalDuration = endTime.Sub(startTime)

	r.printer.Complete(result)

	return result, nil
}

func (r *Runner) executePhase(ctx context.Context, phaseNum int, phase Phase) PhaseResult {
	startTime := time.Now()

	phaseResult := PhaseResult{
		Phase:     phaseNum,
		Name:      phase.Name(),
		StartTime: startTime,
		Metrics:   make(map[string]interface{}),
	}

	log.Debug().Int("phase", phaseNum).Str("name", phase.Name()).Msg("Starting QA phase")

	// Create phase context with timeout
	phaseCtx, cancel := context.WithTimeout(ctx, phase.Timeout())
	defer cancel()

	// Execute phase
	err := phase.Execute(phaseCtx, &phaseResult)

	endTime := time.Now()
	phaseResult.EndTime = endTime
	phaseResult.Duration = endTime.Sub(startTime)

	if err != nil {
		phaseResult.Status = "fail"
		phaseResult.Error = err.Error()
		log.Error().Err(err).Int("phase", phaseNum).Str("name", phase.Name()).Msg("QA phase failed")
	} else {
		phaseResult.Status = "pass"
		log.Debug().Int("phase", phaseNum).Str("name", phase.Name()).Dur("duration", phaseResult.Duration).Msg("QA phase passed")
	}

	return phaseResult
}

func (r *Runner) ensureDirectories() error {
	dirs := []string{
		r.config.ArtifactsDir,
		r.config.AuditDir,
		filepath.Join(r.config.ArtifactsDir, "microstructure"),
		filepath.Join(r.config.ArtifactsDir, "provider_health"),
		filepath.Join(r.config.ArtifactsDir, "vadr_checks"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (r *Runner) executeNoStubGate(result *RunResult) error {
	startTime := time.Now()

	log.Info().Msg("Running Phase -1: No-Stub Gate")

	err := ValidateNoStubs(".", []string{})

	duration := time.Since(startTime)

	if err != nil {
		// No-stub gate failed - this is a hard fail before any network work
		result.Success = false
		result.FailureReason = fmt.Sprintf("FAIL SCAFFOLDS_FOUND +hint: %s", err.Error())
		result.Hint = "Remove TODO/STUB/not-implemented patterns from non-test Go files before QA execution"

		// Print the failure immediately and exit
		fmt.Printf("❌ FAIL SCAFFOLDS_FOUND +hint: remove TODO/STUB/not-implemented\n")

		return err
	}

	log.Info().Dur("duration", duration).Msg("Phase -1: No-Stub Gate passed")
	return nil
}

func (r *Runner) executeAcceptanceVerification(result *RunResult) error {
	startTime := time.Now()

	log.Info().Msg("Running Phase 7: Acceptance Verification")

	validator := NewAcceptanceValidator(r.config.ArtifactsDir, r.config.AuditDir)
	acceptResult, err := validator.Validate()

	duration := time.Since(startTime)

	if err != nil {
		result.Success = false
		result.FailureReason = fmt.Sprintf("Acceptance verification failed: %s", err.Error())
		result.Hint = "Check acceptance validator implementation and ensure all artifacts are properly generated"
		return err
	}

	// Create phase result for acceptance verification
	phaseResult := PhaseResult{
		Phase:     7,
		Name:      "Acceptance Verification",
		StartTime: startTime,
		EndTime:   time.Now(),
		Duration:  duration,
		Metrics: map[string]interface{}{
			"validated_files":  len(acceptResult.ValidatedFiles),
			"violations":       len(acceptResult.Violations),
			"metrics_status":   acceptResult.MetricsStatus,
			"determinism_hash": acceptResult.DeterminismHash,
		},
		Artifacts: []string{"accept_fail.json"},
	}

	if acceptResult.Success {
		phaseResult.Status = "pass"
		result.PassedPhases++
		log.Info().Dur("duration", duration).Int("files", len(acceptResult.ValidatedFiles)).Msg("Phase 7: Acceptance Verification passed")
	} else {
		phaseResult.Status = "fail"
		phaseResult.Error = acceptResult.FailureReason
		result.Success = false
		result.FailureReason = fmt.Sprintf("FAIL %s +hint: %s", acceptResult.FailureCode, acceptResult.Hint)
		result.Hint = acceptResult.Hint

		// Write acceptance failure details
		if err := validator.WriteAcceptanceFailure(acceptResult); err != nil {
			log.Warn().Err(err).Msg("Failed to write acceptance failure report")
		}

		log.Error().
			Str("failure_code", acceptResult.FailureCode).
			Int("violations", len(acceptResult.Violations)).
			Dur("duration", duration).
			Msg("Phase 7: Acceptance Verification failed")
	}

	result.PhaseResults = append(result.PhaseResults, phaseResult)
	r.printer.Phase(phaseResult)

	// Record acceptance phase progress
	if err := r.progress.RecordPhase(7, phaseResult); err != nil {
		log.Warn().Err(err).Msg("Failed to record acceptance phase progress")
	}

	return nil
}

// GetQAPhases returns the standard QA phases 0-6 as defined in QA.MAX.50
func GetQAPhases() []Phase {
	return []Phase{
		NewEnvPhase(),            // Phase 0: Environment validation
		NewStaticPhase(),         // Phase 1: Static analysis
		NewLiveIndexPhase(),      // Phase 2: Live index diffs
		NewMicrostructurePhase(), // Phase 3: Microstructure validation
		NewDeterminismPhase(),    // Phase 4: Determinism checks
		NewExplainabilityPhase(), // Phase 5: Explainability validation
		NewUXPhase(),             // Phase 6: UX validation
	}
}
