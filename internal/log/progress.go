package log

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ProgressIndicator provides visual feedback for long-running operations
type ProgressIndicator struct {
	mu           sync.Mutex
	name         string
	total        int
	current      int
	startTime    time.Time
	lastUpdate   time.Time
	spinner      *Spinner
	showSpinner  bool
	showProgress bool
	showETA      bool
}

// Spinner provides rotating visual feedback
type Spinner struct {
	chars    []string
	current  int
	interval time.Duration
	stop     chan bool
	running  bool
	mu       sync.Mutex
}

// ProgressConfig configures progress indicator behavior
type ProgressConfig struct {
	ShowSpinner  bool
	ShowProgress bool
	ShowETA      bool
	SpinnerStyle SpinnerStyle
}

// SpinnerStyle defines different spinner animations
type SpinnerStyle string

const (
	SpinnerDots     SpinnerStyle = "dots"
	SpinnerLine     SpinnerStyle = "line"
	SpinnerClock    SpinnerStyle = "clock"
	SpinnerBounce   SpinnerStyle = "bounce"
	SpinnerPipeline SpinnerStyle = "pipeline"
)

// NewProgressIndicator creates a new progress indicator
func NewProgressIndicator(name string, total int, config ProgressConfig) *ProgressIndicator {
	pi := &ProgressIndicator{
		name:         name,
		total:        total,
		current:      0,
		startTime:    time.Now(),
		lastUpdate:   time.Now(),
		showSpinner:  config.ShowSpinner,
		showProgress: config.ShowProgress,
		showETA:      config.ShowETA,
	}

	if config.ShowSpinner {
		pi.spinner = NewSpinner(config.SpinnerStyle)
		pi.spinner.Start()
	}

	return pi
}

// NewSpinner creates a new spinner with the specified style
func NewSpinner(style SpinnerStyle) *Spinner {
	s := &Spinner{
		interval: 100 * time.Millisecond,
		stop:     make(chan bool, 1),
	}

	switch style {
	case SpinnerDots:
		s.chars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	case SpinnerLine:
		s.chars = []string{"-", "\\", "|", "/"}
	case SpinnerClock:
		s.chars = []string{"🕐", "🕑", "🕒", "🕓", "🕔", "🕕", "🕖", "🕗", "🕘", "🕙", "🕚", "🕛"}
	case SpinnerBounce:
		s.chars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃", "▁"}
	case SpinnerPipeline:
		s.chars = []string{"⚡", "🔄", "⚙️", "🔧", "⚡"}
		s.interval = 200 * time.Millisecond
	default:
		s.chars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	}

	return s
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	s.running = true
	go s.spin()
}

// Stop terminates the spinner animation
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	s.stop <- true
}

// spin runs the spinner animation loop
func (s *Spinner) spin() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.mu.Lock()
			s.current = (s.current + 1) % len(s.chars)
			s.mu.Unlock()
		}
	}
}

// Current returns the current spinner character
func (s *Spinner) Current() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.chars[s.current]
}

// Increment advances progress by one step
func (pi *ProgressIndicator) Increment() {
	pi.Update(pi.current + 1)
}

// Update sets the current progress value
func (pi *ProgressIndicator) Update(current int) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	pi.current = current
	pi.lastUpdate = time.Now()

	if pi.showProgress || pi.showETA {
		pi.printProgress()
	}
}

// UpdateWithMessage sets progress and displays a custom message
func (pi *ProgressIndicator) UpdateWithMessage(current int, message string) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	pi.current = current
	pi.lastUpdate = time.Now()
	pi.printProgressWithMessage(message)
}

// Finish completes the progress indicator
func (pi *ProgressIndicator) Finish() {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	if pi.spinner != nil {
		pi.spinner.Stop()
	}

	duration := time.Since(pi.startTime)
	fmt.Printf("\r✅ %s completed (%d items, %v)\n", pi.name, pi.total, duration.Round(time.Millisecond))
}

// FinishWithMessage completes the progress indicator with a custom message
func (pi *ProgressIndicator) FinishWithMessage(message string) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	if pi.spinner != nil {
		pi.spinner.Stop()
	}

	duration := time.Since(pi.startTime)
	fmt.Printf("\r✅ %s: %s (%v)\n", pi.name, message, duration.Round(time.Millisecond))
}

// Fail marks the progress as failed
func (pi *ProgressIndicator) Fail(reason string) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	if pi.spinner != nil {
		pi.spinner.Stop()
	}

	duration := time.Since(pi.startTime)
	fmt.Printf("\r❌ %s failed: %s (%v)\n", pi.name, reason, duration.Round(time.Millisecond))
}

// printProgress displays current progress without message
func (pi *ProgressIndicator) printProgress() {
	pi.printProgressWithMessage("")
}

// printProgressWithMessage displays current progress with optional message
func (pi *ProgressIndicator) printProgressWithMessage(message string) {
	var output strings.Builder

	// Clear line and return to beginning
	output.WriteString("\r\033[K")

	// Add spinner if enabled
	if pi.spinner != nil && pi.showSpinner {
		output.WriteString(pi.spinner.Current())
		output.WriteString(" ")
	}

	// Add name
	output.WriteString(pi.name)

	// Add progress bar if enabled
	if pi.showProgress && pi.total > 0 {
		percentage := float64(pi.current) / float64(pi.total) * 100
		barWidth := 20
		filled := int(float64(barWidth) * float64(pi.current) / float64(pi.total))

		output.WriteString(" [")
		for i := 0; i < barWidth; i++ {
			if i < filled {
				output.WriteString("█")
			} else {
				output.WriteString("░")
			}
		}
		output.WriteString(fmt.Sprintf("] %d/%d (%.1f%%)", pi.current, pi.total, percentage))
	} else if pi.total > 0 {
		output.WriteString(fmt.Sprintf(" (%d/%d)", pi.current, pi.total))
	}

	// Add ETA if enabled
	if pi.showETA && pi.total > 0 && pi.current > 0 {
		elapsed := time.Since(pi.startTime)
		rate := float64(pi.current) / elapsed.Seconds()
		remaining := pi.total - pi.current
		eta := time.Duration(float64(remaining)/rate) * time.Second

		if eta > time.Hour {
			output.WriteString(fmt.Sprintf(" ETA: %v", eta.Round(time.Minute)))
		} else {
			output.WriteString(fmt.Sprintf(" ETA: %v", eta.Round(time.Second)))
		}
	}

	// Add custom message if provided
	if message != "" {
		output.WriteString(" - ")
		output.WriteString(message)
	}

	fmt.Print(output.String())
}

// StepLogger provides step-by-step progress logging for pipelines
type StepLogger struct {
	steps       []string
	currentStep int
	startTime   time.Time
	stepTimes   []time.Duration
	progress    *ProgressIndicator
}

// NewStepLogger creates a new step logger for pipeline operations
func NewStepLogger(name string, steps []string) *StepLogger {
	config := ProgressConfig{
		ShowSpinner:  true,
		ShowProgress: true,
		ShowETA:      true,
		SpinnerStyle: SpinnerPipeline,
	}

	return &StepLogger{
		steps:       steps,
		currentStep: -1,
		startTime:   time.Now(),
		stepTimes:   make([]time.Duration, len(steps)),
		progress:    NewProgressIndicator(name, len(steps), config),
	}
}

// StartStep begins a new pipeline step
func (sl *StepLogger) StartStep(stepName string) {
	stepIndex := -1
	for i, step := range sl.steps {
		if step == stepName {
			stepIndex = i
			break
		}
	}

	if stepIndex == -1 {
		log.Warn().Str("step", stepName).Msg("Unknown pipeline step")
		return
	}

	// Record previous step time
	if sl.currentStep >= 0 {
		sl.stepTimes[sl.currentStep] = time.Since(sl.startTime) - sl.getTotalElapsed()
	}

	sl.currentStep = stepIndex
	sl.progress.UpdateWithMessage(stepIndex+1, stepName)

	log.Info().
		Str("step", stepName).
		Int("step_number", stepIndex+1).
		Int("total_steps", len(sl.steps)).
		Msg("Starting pipeline step")
}

// CompleteStep marks the current step as completed
func (sl *StepLogger) CompleteStep() {
	if sl.currentStep >= 0 {
		stepDuration := time.Since(sl.startTime) - sl.getTotalElapsed()
		sl.stepTimes[sl.currentStep] = stepDuration

		log.Info().
			Str("step", sl.steps[sl.currentStep]).
			Dur("duration", stepDuration).
			Msg("Pipeline step completed")
	}
}

// Finish completes the step logger
func (sl *StepLogger) Finish() {
	sl.CompleteStep()
	totalDuration := time.Since(sl.startTime)

	sl.progress.FinishWithMessage(fmt.Sprintf("All %d steps completed", len(sl.steps)))

	// Log step timing summary
	log.Info().
		Dur("total_duration", totalDuration).
		Msg("Pipeline completed - step timing summary:")

	for i, step := range sl.steps {
		if i < len(sl.stepTimes) {
			percentage := float64(sl.stepTimes[i]) / float64(totalDuration) * 100
			log.Info().
				Str("step", step).
				Dur("duration", sl.stepTimes[i]).
				Float64("percentage", percentage).
				Msgf("  %d. %s", i+1, step)
		}
	}
}

// Fail marks the step logger as failed
func (sl *StepLogger) Fail(reason string) {
	sl.progress.Fail(reason)

	log.Error().
		Str("failed_step", sl.getCurrentStepName()).
		Int("completed_steps", sl.currentStep).
		Int("total_steps", len(sl.steps)).
		Str("reason", reason).
		Msg("Pipeline failed")
}

// getCurrentStepName returns the name of the current step
func (sl *StepLogger) getCurrentStepName() string {
	if sl.currentStep >= 0 && sl.currentStep < len(sl.steps) {
		return sl.steps[sl.currentStep]
	}
	return "unknown"
}

// getTotalElapsed returns total time elapsed for completed steps
func (sl *StepLogger) getTotalElapsed() time.Duration {
	var total time.Duration
	for i := 0; i < sl.currentStep; i++ {
		if i < len(sl.stepTimes) {
			total += sl.stepTimes[i]
		}
	}
	return total
}

// DefaultProgressConfig returns default progress indicator configuration
func DefaultProgressConfig() ProgressConfig {
	return ProgressConfig{
		ShowSpinner:  true,
		ShowProgress: true,
		ShowETA:      true,
		SpinnerStyle: SpinnerDots,
	}
}

// QuietProgressConfig returns minimal progress indicator configuration
func QuietProgressConfig() ProgressConfig {
	return ProgressConfig{
		ShowSpinner:  false,
		ShowProgress: false,
		ShowETA:      false,
		SpinnerStyle: SpinnerDots,
	}
}
