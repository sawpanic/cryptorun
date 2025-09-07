package smoke90

import (
	"context"
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/adapters"
)

// Config represents smoke90 backtest configuration
type Config struct {
	TopN      int           // Top N candidates to analyze (default 30)
	Stride    time.Duration // Time stride between windows (default 4h)
	Hold      time.Duration // Hold period for PnL calculation (default 48h)
	Horizon   time.Duration // Backtest horizon (default 90 days)
	UseCache  bool          // Only use cached data, skip gaps
	Progress  bool          // Show progress indicators
	OutputDir string        // Output directory for artifacts
}

// DefaultConfig returns default smoke90 configuration
func DefaultConfig() *Config {
	return &Config{
		TopN:      30,
		Stride:    4 * time.Hour,
		Hold:      48 * time.Hour,
		Horizon:   90 * 24 * time.Hour,
		UseCache:  true,
		Progress:  false,
		OutputDir: "./artifacts/smoke90",
	}
}

// Runner executes the 90-day smoke backtest
type Runner struct {
	config        *Config
	guardsAdapter *adapters.GuardsAdapter
	metrics       *Metrics
	writer        *Writer
	clock         Clock // Injectable for testing
}

// Clock interface for time operations (injectable for testing)
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using real time
type RealClock struct{}

func (r *RealClock) Now() time.Time {
	return time.Now()
}

// NewRunner creates a new smoke90 backtest runner
func NewRunner(config *Config, outputDir string) *Runner {
	if config == nil {
		config = DefaultConfig()
	}

	metrics := NewMetrics()
	writer := NewWriter(outputDir)

	return &Runner{
		config:        config,
		guardsAdapter: adapters.NewGuardsAdapter(),
		metrics:       metrics,
		writer:        writer,
		clock:         &RealClock{},
	}
}

// SetClock sets the clock implementation (for testing)
func (r *Runner) SetClock(clock Clock) {
	r.clock = clock
}

// Run executes the smoke90 backtest
func (r *Runner) Run(ctx context.Context) (*BacktestResults, error) {
	startTime := r.clock.Now()
	endTime := startTime.Add(-r.config.Horizon)

	if r.config.Progress {
		fmt.Printf("🔥 Starting Smoke90 backtest (cache-only)\n")
		fmt.Printf("📅 Period: %s to %s (90 days)\n", endTime.Format("2006-01-02"), startTime.Format("2006-01-02"))
		fmt.Printf("⚙️  Config: TopN=%d, Stride=%v, Hold=%v\n\n", r.config.TopN, r.config.Stride, r.config.Hold)
	}

	results := &BacktestResults{
		Config:    r.config,
		StartTime: startTime,
		EndTime:   endTime,
		Windows:   make([]*WindowResult, 0),
	}

	// Calculate total windows for progress
	totalWindows := int(r.config.Horizon / r.config.Stride)
	windowCount := 0

	// Process each time window
	for windowTime := endTime; windowTime.Before(startTime); windowTime = windowTime.Add(r.config.Stride) {
		windowCount++

		if r.config.Progress && windowCount%20 == 0 {
			progress := float64(windowCount) / float64(totalWindows) * 100
			fmt.Printf("⏳ [%.1f%%] Processing window %d/%d (%s)\n",
				progress, windowCount, totalWindows, windowTime.Format("2006-01-02 15:04"))
		}

		windowResult, err := r.processWindow(ctx, windowTime)
		if err != nil {
			// Log error but continue with next window
			r.metrics.RecordError(err.Error())
			if r.config.Progress {
				fmt.Printf("❌ Window %s: %v\n", windowTime.Format("2006-01-02 15:04"), err)
			}
			continue
		}

		if windowResult != nil {
			results.Windows = append(results.Windows, windowResult)
			r.metrics.RecordWindow(windowResult)
		}
	}

	// Finalize results
	results.TotalWindows = totalWindows
	results.ProcessedWindows = len(results.Windows)
	results.SkippedWindows = totalWindows - results.ProcessedWindows
	results.Metrics = r.metrics.GetSummary()

	// Write artifacts
	if err := r.writer.WriteResults(results); err != nil {
		return nil, fmt.Errorf("failed to write results: %w", err)
	}

	if err := r.writer.WriteReport(results); err != nil {
		return nil, fmt.Errorf("failed to write report: %w", err)
	}

	if r.config.Progress {
		fmt.Printf("\n✅ Smoke90 backtest completed\n")
		fmt.Printf("📊 Processed: %d/%d windows (%.1f%% coverage)\n",
			results.ProcessedWindows, results.TotalWindows,
			float64(results.ProcessedWindows)/float64(results.TotalWindows)*100)
		fmt.Printf("📁 Artifacts: %s\n", r.writer.GetOutputDir())
	}

	return results, nil
}

// processWindow processes a single time window
func (r *Runner) processWindow(ctx context.Context, windowTime time.Time) (*WindowResult, error) {
	// Try to load cached data for this window
	candidates, err := r.loadCachedCandidates(windowTime)
	if err != nil {
		return nil, fmt.Errorf("failed to load candidates: %w", err)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no cached data available (SKIP: cache miss)")
	}

	// Limit to TopN candidates
	if len(candidates) > r.config.TopN {
		candidates = candidates[:r.config.TopN]
	}

	windowResult := &WindowResult{
		Timestamp:      windowTime,
		Candidates:     make([]*CandidateResult, 0),
		GuardStats:     make(map[string]*GuardStat),
		ThrottleEvents: make([]*ThrottleEvent, 0),
		RelaxEvents:    make([]*RelaxEvent, 0),
		SkipReasons:    make([]string, 0),
	}

	// Process each candidate through the unified pipeline
	for i, candidate := range candidates {
		candidateResult := r.processCandidate(ctx, candidate, windowTime)
		windowResult.Candidates = append(windowResult.Candidates, candidateResult)

		// Collect guard statistics
		r.updateGuardStats(windowResult.GuardStats, candidateResult)

		// Check for throttling events
		if throttleEvent := r.checkThrottling(candidate.Symbol, windowTime); throttleEvent != nil {
			windowResult.ThrottleEvents = append(windowResult.ThrottleEvents, throttleEvent)
		}

		// Check for relax events (P99 latency relaxation)
		if relaxEvent := r.checkRelaxEvents(candidate.Symbol, windowTime); relaxEvent != nil {
			windowResult.RelaxEvents = append(windowResult.RelaxEvents, relaxEvent)
		}

		// Progress for large windows
		if r.config.Progress && len(candidates) > 10 && (i+1)%10 == 0 {
			fmt.Printf("  📈 Processed %d/%d candidates\n", i+1, len(candidates))
		}
	}

	// Calculate window-level metrics
	windowResult.PassedCount = r.countPassedCandidates(windowResult.Candidates)
	windowResult.FailedCount = len(windowResult.Candidates) - windowResult.PassedCount
	windowResult.GuardPassRate = float64(windowResult.PassedCount) / float64(len(windowResult.Candidates)) * 100

	return windowResult, nil
}

// processCandidate processes a single candidate through the unified pipeline
func (r *Runner) processCandidate(ctx context.Context, candidate *Candidate, windowTime time.Time) *CandidateResult {
	result := &CandidateResult{
		Symbol:      candidate.Symbol,
		Score:       candidate.Score,
		Timestamp:   windowTime,
		GuardResult: make(map[string]*GuardResult),
	}

	// Apply unified scoring (from cached data)
	if candidate.Score < 75.0 {
		result.Passed = false
		result.FailReason = fmt.Sprintf("Score %.1f < 75.0 threshold", candidate.Score)
		return result
	}

	// Apply hard gates - VADR check
	if candidate.VADR < 1.8 {
		result.Passed = false
		result.FailReason = fmt.Sprintf("VADR %.2fx < 1.8x threshold", candidate.VADR)
		return result
	}

	// Apply hard gates - funding divergence check
	if !candidate.HasFundingDivergence {
		result.Passed = false
		result.FailReason = "No funding divergence present"
		return result
	}

	// Apply guards pipeline (using cached results if available)
	guardsResult := r.applyGuards(ctx, candidate, windowTime)
	result.GuardResult = guardsResult

	// Check if any hard guard failed
	for guardName, guardResult := range guardsResult {
		if !guardResult.Passed && guardResult.Type == "hard" {
			result.Passed = false
			result.FailReason = fmt.Sprintf("Hard guard '%s' failed: %s", guardName, guardResult.Reason)
			return result
		}
	}

	// Apply microstructure validation (using cached results if available)
	microResult, err := r.applyMicrostructureValidation(ctx, candidate.Symbol, windowTime)
	if err != nil || !microResult.Passed {
		result.Passed = false
		result.MicroResult = microResult
		if err != nil {
			result.FailReason = fmt.Sprintf("Microstructure error: %v", err)
		} else {
			result.FailReason = fmt.Sprintf("Microstructure failed: %s", microResult.Reason)
		}
		return result
	}

	result.MicroResult = microResult

	// Calculate simulated PnL for hold period
	pnl, err := r.calculateSimulatedPnL(candidate.Symbol, windowTime, r.config.Hold)
	if err != nil {
		result.PnL = 0.0
		result.PnLError = err.Error()
	} else {
		result.PnL = pnl
	}

	result.Passed = true
	return result
}

// loadCachedCandidates loads candidates from cache for the given time window
func (r *Runner) loadCachedCandidates(windowTime time.Time) ([]*Candidate, error) {
	// This would load from actual cache in real implementation
	// For now, return mock data or error if no cache available

	// Try to load from cache files
	_ = fmt.Sprintf("candidates_%s", windowTime.Format("2006010215")) // Cache key for future use

	// Simulate cache miss for some windows (realistic scenario)
	if windowTime.Hour()%6 == 0 { // Miss every 6th hour
		return nil, fmt.Errorf("cache miss for window %s", windowTime.Format("2006-01-02 15:04"))
	}

	// Mock candidates for demonstration
	candidates := make([]*Candidate, r.config.TopN)
	for i := 0; i < r.config.TopN; i++ {
		candidates[i] = &Candidate{
			Symbol:               fmt.Sprintf("TEST%dUSD", i+1),
			Score:                75.0 + float64(i)*2.0,       // Scores from 75-135
			VADR:                 1.8 + float64(i)*0.1,        // VADR from 1.8-4.8
			HasFundingDivergence: i%3 != 0,                    // 2/3 have funding divergence
			Volume24h:            float64(1000000 + i*100000), // Volume 1M-4M
			PriceChange1h:        2.0 + float64(i)*0.5,        // 1h change 2-17%
			PriceChange24h:       5.0 + float64(i)*1.0,        // 24h change 5-35%
		}
	}

	return candidates, nil
}

// applyGuards applies the guards pipeline to a candidate
func (r *Runner) applyGuards(ctx context.Context, candidate *Candidate, windowTime time.Time) map[string]*GuardResult {
	result := make(map[string]*GuardResult)

	// Apply freshness guard
	result["freshness"] = &GuardResult{
		Type:   "hard",
		Passed: true, // Assume cached data passes freshness
		Reason: "within base threshold: cached data",
	}

	// Apply fatigue guard (24h momentum + RSI check)
	fatigueThreshold := 15.0 // Normal regime threshold
	if candidate.PriceChange24h > fatigueThreshold {
		result["fatigue"] = &GuardResult{
			Type:   "hard",
			Passed: false,
			Reason: fmt.Sprintf("24h momentum %.1f%% > %.1f%% limit", candidate.PriceChange24h, fatigueThreshold),
		}
	} else {
		result["fatigue"] = &GuardResult{
			Type:   "hard",
			Passed: true,
			Reason: fmt.Sprintf("24h momentum %.1f%% ≤ %.1f%% limit", candidate.PriceChange24h, fatigueThreshold),
		}
	}

	// Apply late-fill guard with P99 relaxation simulation
	lateThreshold := 30.0                                        // 30 second base threshold
	simulatedLatency := 25.0 + float64(windowTime.Hour()%12)*5.0 // Simulate varying latency

	if simulatedLatency > 30.0 {
		// Simulate P99 relaxation logic
		p99Latency := 350.0 + float64(windowTime.Minute())*2.0 // Simulate P99 latency
		if p99Latency > 400.0 {
			// Apply relaxation
			result["late_fill"] = &GuardResult{
				Type:   "hard",
				Passed: true,
				Reason: fmt.Sprintf("p99 relaxation applied: %.1fms ≤ 60.0s (base + grace)", simulatedLatency*1000),
			}
		} else {
			result["late_fill"] = &GuardResult{
				Type:   "hard",
				Passed: false,
				Reason: fmt.Sprintf("late fill: %.1fs > %.1fs base threshold (p99 %.1fms ≤ 400.0ms threshold)",
					simulatedLatency, lateThreshold, p99Latency),
			}
		}
	} else {
		result["late_fill"] = &GuardResult{
			Type:   "hard",
			Passed: true,
			Reason: fmt.Sprintf("within base threshold: %.1fs ≤ %.1fs", simulatedLatency, lateThreshold),
		}
	}

	return result
}

// applyMicrostructureValidation applies microstructure validation to a candidate
func (r *Runner) applyMicrostructureValidation(ctx context.Context, symbol string, windowTime time.Time) (*MicroResult, error) {
	// Simulate microstructure validation results
	// In real implementation, this would use cached orderbook data

	// Mock some failures for realism
	if symbol[len(symbol)-2:] == "0U" || symbol[len(symbol)-2:] == "5U" { // TEST10USD, TEST15USD, etc.
		return &MicroResult{
			Passed: false,
			Reason: "Spread 65.0 bps > 50.0 bps limit",
			Venues: []string{"binance"},
		}, nil
	}

	return &MicroResult{
		Passed: true,
		Reason: "Passed on 2/3 venues",
		Venues: []string{"binance", "okx"},
	}, nil
}

// calculateSimulatedPnL calculates simulated PnL for hold period
func (r *Runner) calculateSimulatedPnL(symbol string, entryTime time.Time, holdPeriod time.Duration) (float64, error) {
	// Simulate PnL based on symbol and timing
	// In real implementation, this would use cached price data

	exitTime := entryTime.Add(holdPeriod)

	// Simple PnL simulation based on symbol hash and time
	symbolHash := 0
	for _, c := range symbol {
		symbolHash += int(c)
	}

	// Simulate some variation in returns
	basePnL := float64((symbolHash+int(entryTime.Unix()))%200-100) / 10.0 // -10% to +10%

	// Add some time-based noise
	timeFactor := float64(exitTime.Hour()+exitTime.Day()) / 100.0

	return basePnL + timeFactor, nil
}

// Helper methods for metrics collection

func (r *Runner) updateGuardStats(stats map[string]*GuardStat, candidate *CandidateResult) {
	for guardName, guardResult := range candidate.GuardResult {
		if _, exists := stats[guardName]; !exists {
			stats[guardName] = &GuardStat{
				Name:   guardName,
				Type:   guardResult.Type,
				Total:  0,
				Passed: 0,
				Failed: 0,
			}
		}

		stat := stats[guardName]
		stat.Total++
		if guardResult.Passed {
			stat.Passed++
		} else {
			stat.Failed++
		}
		stat.PassRate = float64(stat.Passed) / float64(stat.Total) * 100
	}
}

func (r *Runner) countPassedCandidates(candidates []*CandidateResult) int {
	count := 0
	for _, candidate := range candidates {
		if candidate.Passed {
			count++
		}
	}
	return count
}

func (r *Runner) checkThrottling(symbol string, windowTime time.Time) *ThrottleEvent {
	// Simulate occasional throttling events
	if windowTime.Hour()%8 == 0 && symbol[len(symbol)-1:] == "5" {
		return &ThrottleEvent{
			Provider:  "binance",
			Reason:    "Rate limit exceeded: 5 RPS",
			Timestamp: windowTime,
			Symbol:    symbol,
		}
	}
	return nil
}

func (r *Runner) checkRelaxEvents(symbol string, windowTime time.Time) *RelaxEvent {
	// Simulate occasional P99 relaxation events
	if windowTime.Minute()%30 == 0 && symbol[len(symbol)-1:] == "2" {
		return &RelaxEvent{
			Type:      "latefill_relax",
			Symbol:    symbol,
			P99Ms:     450.2,
			GraceMs:   30000,
			Timestamp: windowTime,
			Reason:    "p99_exceeded:450.2ms,grace:30s",
		}
	}
	return nil
}
