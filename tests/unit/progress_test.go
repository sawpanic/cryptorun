package unit

import (
	"context"
	"strings"
	"testing"
	"time"

	pipelineexec "github.com/sawpanic/cryptorun/internal/application/pipeline"
	httpmetrics "github.com/sawpanic/cryptorun/internal/interfaces/http"
	logprogress "github.com/sawpanic/cryptorun/internal/log"
)

func TestProgressIndicatorBasicFunctionality(t *testing.T) {
	config := logprogress.DefaultProgressConfig()
	config.ShowSpinner = false // Disable for testing

	progress := logprogress.NewProgressIndicator("Test Progress", 10, config)

	if progress == nil {
		t.Fatal("Failed to create progress indicator")
	}

	// Test increment
	progress.Increment()

	// Test update
	progress.Update(5)

	// Test finish
	progress.Finish()

	// Should complete without errors
}

func TestProgressIndicatorWithSpinner(t *testing.T) {
	config := logprogress.ProgressConfig{
		ShowSpinner:  true,
		ShowProgress: true,
		ShowETA:      true,
		SpinnerStyle: logprogress.SpinnerDots,
	}

	progress := logprogress.NewProgressIndicator("Spinner Test", 5, config)
	defer progress.Finish()

	for i := 0; i < 5; i++ {
		progress.UpdateWithMessage(i+1, "Processing item")
		time.Sleep(50 * time.Millisecond)
	}
}

func TestSpinnerStyles(t *testing.T) {
	styles := []logprogress.SpinnerStyle{
		logprogress.SpinnerDots,
		logprogress.SpinnerLine,
		logprogress.SpinnerClock,
		logprogress.SpinnerBounce,
		logprogress.SpinnerPipeline,
	}

	for _, style := range styles {
		spinner := logprogress.NewSpinner(style)
		if spinner == nil {
			t.Errorf("Failed to create spinner with style %s", style)
			continue
		}

		spinner.Start()

		// Verify spinner produces different characters
		char1 := spinner.Current()
		time.Sleep(150 * time.Millisecond)
		char2 := spinner.Current()

		spinner.Stop()

		// Characters should be different (spinner is animating)
		if char1 == char2 {
			t.Errorf("Spinner style %s not animating: %s == %s", style, char1, char2)
		}
	}
}

func TestStepLoggerPipelineExecution(t *testing.T) {
	steps := []string{"Initialize", "Process", "Finalize"}
	logger := logprogress.NewStepLogger("Test Pipeline", steps)

	if logger == nil {
		t.Fatal("Failed to create step logger")
	}

	// Execute steps
	for _, step := range steps {
		logger.StartStep(step)
		time.Sleep(10 * time.Millisecond) // Simulate work
		logger.CompleteStep()
	}

	logger.Finish()
}

func TestStepLoggerErrorHandling(t *testing.T) {
	steps := []string{"Step1", "Step2", "Step3"}
	logger := logprogress.NewStepLogger("Error Test", steps)

	logger.StartStep("Step1")
	logger.CompleteStep()

	logger.StartStep("Step2")
	// Simulate failure
	logger.Fail("Test error condition")
}

func TestMetricsStepTimerIntegration(t *testing.T) {
	// Initialize metrics
	httpmetrics.InitializeMetrics()
	metrics := httpmetrics.DefaultMetrics

	if metrics == nil {
		t.Fatal("Metrics not initialized")
	}

	// Test step timer
	timer := metrics.StartStepTimer("test_step")
	if timer == nil {
		t.Fatal("Failed to create step timer")
	}

	time.Sleep(10 * time.Millisecond)
	timer.Stop("success")

	// Test cache metrics
	metrics.RecordCacheHit("test_cache")
	metrics.RecordCacheMiss("test_cache")

	// Test WebSocket latency
	metrics.RecordWSLatency("test_exchange", "test_endpoint", 123.45)

	// Test pipeline error
	metrics.RecordPipelineError("test_step", "test_error")

	// Test scan counters
	metrics.IncrementActiveScans()
	metrics.DecrementActiveScans()
}

func TestPipelineExecutorWithProgress(t *testing.T) {
	config := pipelineexec.PipelineConfig{
		MaxSymbols:     5,
		TimeoutPerStep: 5 * time.Second,
		EnableMetrics:  true,
		EnableProgress: false, // Disable visual progress for testing
		ProgressStyle:  "plain",
	}

	executor := pipelineexec.NewPipelineExecutor(config)
	if executor == nil {
		t.Fatal("Failed to create pipeline executor")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := executor.ExecutePipeline(ctx, config)
	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("Pipeline result is nil")
	}

	if !result.Success {
		t.Errorf("Pipeline should have succeeded, got errors: %+v", result.Errors)
	}

	if len(result.StepDurations) == 0 {
		t.Error("Pipeline should have recorded step durations")
	}

	if result.TotalDuration == 0 {
		t.Error("Pipeline should have recorded total duration")
	}

	if result.ProcessedCount == 0 {
		t.Error("Pipeline should have processed symbols")
	}

	// Verify all expected steps are present
	expectedSteps := pipelineexec.GetStepNames()
	for _, step := range expectedSteps {
		if _, exists := result.StepDurations[step]; !exists {
			t.Errorf("Missing step duration for: %s", step)
		}
	}
}

func TestProgressConfigurationModes(t *testing.T) {
	// Test default config
	defaultConfig := logprogress.DefaultProgressConfig()
	if !defaultConfig.ShowSpinner || !defaultConfig.ShowProgress || !defaultConfig.ShowETA {
		t.Error("Default config should enable all progress features")
	}

	// Test quiet config
	quietConfig := logprogress.QuietProgressConfig()
	if quietConfig.ShowSpinner || quietConfig.ShowProgress || quietConfig.ShowETA {
		t.Error("Quiet config should disable all progress features")
	}
}

func TestProgressIndicatorETA(t *testing.T) {
	config := logprogress.ProgressConfig{
		ShowSpinner:  false,
		ShowProgress: true,
		ShowETA:      true,
		SpinnerStyle: logprogress.SpinnerDots,
	}

	progress := logprogress.NewProgressIndicator("ETA Test", 100, config)

	// Simulate processing with consistent timing
	for i := 0; i < 10; i++ {
		progress.Update(i + 1)
		time.Sleep(5 * time.Millisecond)
	}

	// ETA should be calculable after some progress
	// This is mainly testing that no panics occur
	progress.Update(50)
	progress.Finish()
}

func TestProgressBarRendering(t *testing.T) {
	config := logprogress.ProgressConfig{
		ShowSpinner:  false,
		ShowProgress: true,
		ShowETA:      false,
		SpinnerStyle: logprogress.SpinnerDots,
	}

	// Test different progress levels
	progress := logprogress.NewProgressIndicator("Render Test", 4, config)

	testCases := []int{0, 1, 2, 3, 4}
	for _, current := range testCases {
		progress.Update(current)
		time.Sleep(10 * time.Millisecond)
	}

	progress.Finish()
}

func TestStepLoggerTimingAccuracy(t *testing.T) {
	steps := []string{"Quick", "Medium", "Slow"}
	logger := logprogress.NewStepLogger("Timing Test", steps)

	expectedDurations := []time.Duration{
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
	}

	for i, step := range steps {
		startTime := time.Now()
		logger.StartStep(step)
		time.Sleep(expectedDurations[i])
		logger.CompleteStep()
		actualDuration := time.Since(startTime)

		// Allow for some variance in timing
		if actualDuration < expectedDurations[i] {
			t.Errorf("Step %s duration too short: expected >= %v, got %v", step, expectedDurations[i], actualDuration)
		}

		// Should be within reasonable bounds (2x expected duration)
		if actualDuration > 2*expectedDurations[i] {
			t.Errorf("Step %s duration too long: expected ~%v, got %v", step, expectedDurations[i], actualDuration)
		}
	}

	logger.Finish()
}

func TestProgressIndicatorMessageHandling(t *testing.T) {
	config := logprogress.DefaultProgressConfig()
	config.ShowSpinner = false

	progress := logprogress.NewProgressIndicator("Message Test", 3, config)

	messages := []string{
		"Processing item 1",
		"Handling complex operation",
		"Finalizing results",
	}

	for i, message := range messages {
		progress.UpdateWithMessage(i+1, message)
		time.Sleep(20 * time.Millisecond)
	}

	progress.FinishWithMessage("All processing completed successfully")
}

func TestProgressFailureScenarios(t *testing.T) {
	config := logprogress.DefaultProgressConfig()
	config.ShowSpinner = false

	// Test progress failure
	progress := logprogress.NewProgressIndicator("Failure Test", 10, config)
	progress.Update(5)
	progress.Fail("Simulated failure condition")

	// Test step logger failure
	steps := []string{"Start", "Process", "End"}
	logger := logprogress.NewStepLogger("Failure Pipeline", steps)
	logger.StartStep("Start")
	logger.CompleteStep()
	logger.StartStep("Process")
	logger.Fail("Process failed with error")
}

func TestConcurrentProgressIndicators(t *testing.T) {
	config := logprogress.DefaultProgressConfig()
	config.ShowSpinner = false // Disable for cleaner test output

	// Test that multiple progress indicators can run concurrently
	done := make(chan bool, 2)

	// First progress indicator
	go func() {
		progress1 := logprogress.NewProgressIndicator("Concurrent 1", 5, config)
		for i := 0; i < 5; i++ {
			progress1.Update(i + 1)
			time.Sleep(10 * time.Millisecond)
		}
		progress1.Finish()
		done <- true
	}()

	// Second progress indicator
	go func() {
		progress2 := logprogress.NewProgressIndicator("Concurrent 2", 3, config)
		for i := 0; i < 3; i++ {
			progress2.Update(i + 1)
			time.Sleep(15 * time.Millisecond)
		}
		progress2.Finish()
		done <- true
	}()

	// Wait for both to complete
	for i := 0; i < 2; i++ {
		select {
		case <-done:
			// Progress indicator completed
		case <-time.After(2 * time.Second):
			t.Fatal("Concurrent progress indicators took too long")
		}
	}
}

func TestMetricsCacheHitRatioCalculation(t *testing.T) {
	httpmetrics.InitializeMetrics()
	metrics := httpmetrics.DefaultMetrics

	// Record some cache hits and misses
	cacheType := "test_cache_ratio"

	// 7 hits, 3 misses = 70% hit rate
	for i := 0; i < 7; i++ {
		metrics.RecordCacheHit(cacheType)
	}
	for i := 0; i < 3; i++ {
		metrics.RecordCacheMiss(cacheType)
	}

	// Note: The actual hit ratio calculation happens internally
	// This test mainly ensures no panics occur during the process
}

func TestProgressIndicatorZeroTotal(t *testing.T) {
	config := logprogress.DefaultProgressConfig()
	config.ShowProgress = true

	// Test with zero total (indeterminate progress)
	progress := logprogress.NewProgressIndicator("Indeterminate", 0, config)
	progress.UpdateWithMessage(0, "Working...")
	time.Sleep(50 * time.Millisecond)
	progress.FinishWithMessage("Done")
}

func TestStepLoggerUnknownStep(t *testing.T) {
	steps := []string{"Step1", "Step2"}
	logger := logprogress.NewStepLogger("Unknown Step Test", steps)

	// Try to start a step not in the list
	logger.StartStep("UnknownStep")
	logger.CompleteStep()

	// Should handle gracefully without crashing
	logger.Finish()
}
