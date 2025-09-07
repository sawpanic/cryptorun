package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"

	"github.com/sawpanic/cryptorun/internal/application"
)

// Job represents a scheduled job configuration
type Job struct {
	Name        string `yaml:"name"`
	Schedule    string `yaml:"schedule"`    // cron format: "*/15 * * * *" for every 15 minutes
	Type        string `yaml:"type"`        // "scan.hot", "scan.warm", "regime.refresh"
	Description string `yaml:"description"`
	Enabled     bool   `yaml:"enabled"`
	Config      JobConfig `yaml:"config"`
}

// JobConfig holds job-specific configuration
type JobConfig struct {
	Universe       string   `yaml:"universe"`        // "top30", "remaining", "top50"
	Venues         []string `yaml:"venues"`          // ["kraken", "okx", "coinbase"]
	MaxSample      int      `yaml:"max_sample"`      // 30 for hot, 100 for warm
	TTL            int      `yaml:"ttl"`             // cache TTL seconds
	TopN           int      `yaml:"top_n"`           // number of candidates to select
	Premove        bool     `yaml:"premove"`         // include premove analysis
	OutputDir      string   `yaml:"output_dir"`      // artifacts output directory
	RequireGates   []string `yaml:"require_gates"`   // ["funding_divergence", "supply_squeeze", "whale_accumulation"]
	MinGatesPassed int      `yaml:"min_gates_passed"` // minimum gates that must pass (e.g., 2 for 2-of-3)
	RegimeAware    bool     `yaml:"regime_aware"`    // use regime-aware weights
	VolumeConfirm  bool     `yaml:"volume_confirm"`  // require volume confirmation in risk_off regime
}

// SchedulerConfig holds the main scheduler configuration
type SchedulerConfig struct {
	Jobs []Job `yaml:"jobs"`
	Global GlobalConfig `yaml:"global"`
}

// GlobalConfig holds global scheduler settings
type GlobalConfig struct {
	ArtifactsDir string `yaml:"artifacts_dir"`
	LogLevel     string `yaml:"log_level"`
	Timezone     string `yaml:"timezone"`
}

// Status represents scheduler status
type Status struct {
	Running      bool      `yaml:"running"`
	EnabledJobs  int       `yaml:"enabled_jobs"`
	DisabledJobs int       `yaml:"disabled_jobs"`
	NextRun      time.Time `yaml:"next_run"`
	LastRun      time.Time `yaml:"last_run"`
	Uptime       time.Duration `yaml:"uptime"`
}

// JobResult represents the result of a job execution
type JobResult struct {
	JobName   string        `yaml:"job_name"`
	StartTime time.Time     `yaml:"start_time"`
	EndTime   time.Time     `yaml:"end_time"`
	Duration  time.Duration `yaml:"duration"`
	Success   bool          `yaml:"success"`
	Error     string        `yaml:"error,omitempty"`
	Artifacts []string      `yaml:"artifacts"`
}

// Scheduler manages scheduled jobs
type Scheduler struct {
	config    SchedulerConfig
	startTime time.Time
	running   bool
}

// NewScheduler creates a new scheduler instance
func NewScheduler(configPath string) (*Scheduler, error) {
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &Scheduler{
		config: config,
	}, nil
}

// loadConfig loads scheduler configuration from YAML file
func loadConfig(configPath string) (SchedulerConfig, error) {
	var config SchedulerConfig

	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if config.Global.ArtifactsDir == "" {
		config.Global.ArtifactsDir = "artifacts/signals"
	}
	if config.Global.LogLevel == "" {
		config.Global.LogLevel = "info"
	}
	if config.Global.Timezone == "" {
		config.Global.Timezone = "UTC"
	}

	return config, nil
}

// ListJobs returns all configured jobs
func (s *Scheduler) ListJobs() ([]Job, error) {
	return s.config.Jobs, nil
}

// GetStatus returns current scheduler status
func (s *Scheduler) GetStatus() (*Status, error) {
	enabled := 0
	disabled := 0
	
	for _, job := range s.config.Jobs {
		if job.Enabled {
			enabled++
		} else {
			disabled++
		}
	}

	var uptime time.Duration
	if s.running {
		uptime = time.Since(s.startTime)
	}

	status := &Status{
		Running:      s.running,
		EnabledJobs:  enabled,
		DisabledJobs: disabled,
		NextRun:      time.Now().Add(time.Minute), // TODO: calculate actual next run
		LastRun:      time.Now().Add(-time.Hour),  // TODO: track actual last run
		Uptime:       uptime,
	}

	return status, nil
}

// Start begins the scheduler daemon
func (s *Scheduler) Start(ctx context.Context) error {
	s.running = true
	s.startTime = time.Now()
	
	log.Info().Int("jobs", len(s.config.Jobs)).Msg("Scheduler starting")

	// TODO: Implement cron scheduling logic
	// For now, just simulate running
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.running = false
			return ctx.Err()
		case <-ticker.C:
			// Check if any jobs need to run
			s.checkAndRunJobs(ctx)
		}
	}
}

// checkAndRunJobs checks if any jobs need to run and executes them
func (s *Scheduler) checkAndRunJobs(ctx context.Context) {
	now := time.Now()
	
	for _, job := range s.config.Jobs {
		if !job.Enabled {
			continue
		}
		
		// TODO: Implement proper cron schedule checking
		// For now, just log that we would check
		log.Debug().Str("job", job.Name).Time("now", now).Msg("Checking job schedule")
	}
}

// RunJob executes a specific job immediately
func (s *Scheduler) RunJob(ctx context.Context, jobName string, dryRun bool) (*JobResult, error) {
	// Find the job
	var job *Job
	for i, j := range s.config.Jobs {
		if j.Name == jobName {
			job = &s.config.Jobs[i]
			break
		}
	}
	
	if job == nil {
		return nil, fmt.Errorf("job not found: %s", jobName)
	}

	startTime := time.Now()
	result := &JobResult{
		JobName:   jobName,
		StartTime: startTime,
		Success:   true,
		Artifacts: []string{},
	}

	log.Info().Str("job", jobName).Str("type", job.Type).Bool("dry_run", dryRun).Msg("Executing job")

	// Execute based on job type
	switch job.Type {
	case "scan.hot":
		artifacts, err := s.runHotScan(ctx, job, dryRun)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Artifacts = artifacts
		}
	case "scan.warm":
		artifacts, err := s.runWarmScan(ctx, job, dryRun)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Artifacts = artifacts
		}
	case "regime.refresh":
		artifacts, err := s.runRegimeRefresh(ctx, job, dryRun)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Artifacts = artifacts
		}
	case "providers.health":
		artifacts, err := s.runProvidersHealth(ctx, job, dryRun)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Artifacts = artifacts
		}
	case "premove.hourly":
		artifacts, err := s.runPremoveHourly(ctx, job, dryRun)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Artifacts = artifacts
		}
	default:
		result.Success = false
		result.Error = fmt.Sprintf("unknown job type: %s", job.Type)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)

	return result, nil
}

// runHotScan executes a hot scan job
func (s *Scheduler) runHotScan(ctx context.Context, job *Job, dryRun bool) ([]string, error) {
	log.Info().Str("universe", job.Config.Universe).Int("top_n", job.Config.TopN).Msg("Running hot scan")
	
	if dryRun {
		log.Info().Msg("Dry run - would execute hot scan with top30 ADV universe, momentum + premove, regime-aware weights")
		return []string{
			filepath.Join(s.config.Global.ArtifactsDir, fmt.Sprintf("%s_signals.csv", time.Now().Format("20060102_150405"))),
			filepath.Join(s.config.Global.ArtifactsDir, fmt.Sprintf("%s_premove.csv", time.Now().Format("20060102_150405"))),
			filepath.Join(s.config.Global.ArtifactsDir, fmt.Sprintf("%s_explain.json", time.Now().Format("20060102_150405"))),
		}, nil
	}

	// Create timestamp for this run
	timestamp := time.Now().Format("20060102_150405")
	outputDir := filepath.Join(s.config.Global.ArtifactsDir, timestamp)
	
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Initialize scan pipeline with regime awareness
	pipeline := application.NewScanPipeline(filepath.Join(outputDir, "microstructure"))
	
	// Set regime to auto-detect or use current cached regime
	pipeline.SetRegime("auto")
	
	log.Info().Str("venues", fmt.Sprintf("%v", job.Config.Venues)).
		Int("max_sample", job.Config.MaxSample).
		Int("ttl", job.Config.TTL).
		Msg("Executing hot momentum scan with regime-aware weights")

	// Execute scan with top30 ADV universe
	candidates, err := pipeline.ScanUniverse(ctx)
	if err != nil {
		return nil, fmt.Errorf("hot scan failed: %w", err)
	}

	// Limit to requested top-N candidates
	if len(candidates) > job.Config.TopN {
		candidates = candidates[:job.Config.TopN]
	}

	log.Info().Int("candidates", len(candidates)).Msg("Hot scan completed, generating artifacts")

	// Generate artifacts
	artifacts := []string{
		filepath.Join(outputDir, "signals.csv"),
	}

	if job.Config.Premove {
		artifacts = append(artifacts, filepath.Join(outputDir, "premove.csv"))
	}

	artifacts = append(artifacts, filepath.Join(outputDir, "explain.json"))

	// Write signals CSV with hot scan format
	if err := s.writeSignalsCSV(filepath.Join(outputDir, "signals.csv"), candidates, "hot"); err != nil {
		return nil, fmt.Errorf("failed to write signals CSV: %w", err)
	}

	// Write premove CSV if enabled
	if job.Config.Premove {
		if err := s.writePremoveCSV(filepath.Join(outputDir, "premove.csv"), candidates); err != nil {
			return nil, fmt.Errorf("failed to write premove CSV: %w", err)
		}
	}

	// Write explain JSON
	if err := s.writeExplainJSON(filepath.Join(outputDir, "explain.json"), candidates, job.Type); err != nil {
		return nil, fmt.Errorf("failed to write explain JSON: %w", err)
	}

	log.Info().Int("artifacts", len(artifacts)).Int("top_candidates", len(candidates)).Msg("Hot scan artifacts generated")
	return artifacts, nil
}

// runWarmScan executes a warm scan job
func (s *Scheduler) runWarmScan(ctx context.Context, job *Job, dryRun bool) ([]string, error) {
	log.Info().Str("universe", job.Config.Universe).Int("max_sample", job.Config.MaxSample).Msg("Running warm scan")
	
	if dryRun {
		log.Info().Msg("Dry run - would execute warm scan with remaining universe and cached sources")
		return []string{
			filepath.Join(s.config.Global.ArtifactsDir, fmt.Sprintf("%s_warm_signals.csv", time.Now().Format("20060102_150405"))),
		}, nil
	}

	// Create timestamp for this run
	timestamp := time.Now().Format("20060102_150405")
	outputDir := filepath.Join(s.config.Global.ArtifactsDir, timestamp)
	
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Initialize scan pipeline for warm scan (cached sources, lower QPS)
	pipeline := application.NewScanPipeline(filepath.Join(outputDir, "microstructure"))
	
	// Set regime to use cached value from regime refresh
	pipeline.SetRegime("auto")
	
	log.Info().Str("venues", fmt.Sprintf("%v", job.Config.Venues)).
		Int("max_sample", job.Config.MaxSample).
		Int("ttl", job.Config.TTL).
		Bool("cached_sources", true).
		Msg("Executing warm scan with remaining universe and cached sources")

	// Execute scan with remaining universe (larger sample, cached)
	candidates, err := pipeline.ScanUniverse(ctx)
	if err != nil {
		return nil, fmt.Errorf("warm scan failed: %w", err)
	}

	// Filter candidates for warm scan - lower threshold
	warmCandidates := []application.CandidateResult{}
	for _, candidate := range candidates {
		// Include candidates with score >= 65 for warm scan (lower than hot scan threshold)
		if candidate.Score.Score >= 65.0 {
			warmCandidates = append(warmCandidates, candidate)
		}
	}

	// Limit to requested top-N candidates
	if len(warmCandidates) > job.Config.TopN {
		warmCandidates = warmCandidates[:job.Config.TopN]
	}

	log.Info().Int("total_candidates", len(candidates)).
		Int("warm_candidates", len(warmCandidates)).
		Msg("Warm scan completed, generating artifacts")

	// Generate artifacts
	artifacts := []string{
		filepath.Join(outputDir, "warm_signals.csv"),
	}

	// Write warm signals CSV with cached indicator
	if err := s.writeWarmSignalsCSV(filepath.Join(outputDir, "warm_signals.csv"), warmCandidates); err != nil {
		return nil, fmt.Errorf("failed to write warm signals CSV: %w", err)
	}

	log.Info().Int("artifacts", len(artifacts)).Int("warm_candidates", len(warmCandidates)).Msg("Warm scan artifacts generated")
	return artifacts, nil
}

// runRegimeRefresh executes a regime refresh job
func (s *Scheduler) runRegimeRefresh(ctx context.Context, job *Job, dryRun bool) ([]string, error) {
	log.Info().Msg("Running regime refresh with realized_vol_7d, %>20MA, breadth thrust indicators")
	
	if dryRun {
		log.Info().Msg("Dry run - would refresh regime with realized_vol_7d, %>20MA, breadth thrust; majority vote; cache result + timestamp")
		return []string{
			filepath.Join(s.config.Global.ArtifactsDir, fmt.Sprintf("%s_regime.json", time.Now().Format("20060102_150405"))),
		}, nil
	}

	// Create timestamp for this run
	timestamp := time.Now().Format("20060102_150405")
	outputDir := filepath.Join(s.config.Global.ArtifactsDir, timestamp)
	
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	log.Info().Str("venues", fmt.Sprintf("%v", job.Config.Venues)).
		Int("max_sample", job.Config.MaxSample).
		Msg("Executing regime detection with 4h cadence")

	// Calculate regime indicators
	regimeData := s.calculateRegimeIndicators(ctx, job)
	
	// Perform majority vote to determine regime
	regime := s.performRegimeMajorityVote(regimeData)
	
	log.Info().Str("regime", regime).
		Float64("realized_vol_7d", regimeData.RealizedVol7d).
		Float64("pct_above_20ma", regimeData.PctAbove20MA).
		Float64("breadth_thrust", regimeData.BreadthThrust).
		Float64("confidence", regimeData.Confidence).
		Msg("Regime detection completed")

	// Generate artifacts
	artifacts := []string{
		filepath.Join(outputDir, "regime.json"),
	}

	// Write regime JSON with full indicator breakdown
	if err := s.writeRegimeJSON(filepath.Join(outputDir, "regime.json"), regime, regimeData); err != nil {
		return nil, fmt.Errorf("failed to write regime JSON: %w", err)
	}

	// Cache the regime result for use by hot/warm scans
	s.cacheRegimeResult(regime, regimeData)

	log.Info().Int("artifacts", len(artifacts)).Str("detected_regime", regime).Msg("Regime refresh completed")
	return artifacts, nil
}

// RegimeData holds regime detection indicators
type RegimeData struct {
	RealizedVol7d  float64 `json:"realized_vol_7d"`
	PctAbove20MA   float64 `json:"pct_above_20ma"`
	BreadthThrust  float64 `json:"breadth_thrust"`
	Confidence     float64 `json:"confidence"`
	Timestamp      time.Time `json:"timestamp"`
}

// calculateRegimeIndicators computes the three regime indicators
func (s *Scheduler) calculateRegimeIndicators(ctx context.Context, job *Job) RegimeData {
	// Mock implementation - in real system would fetch market data
	// and calculate actual indicators
	
	// Simulate regime indicator calculation
	realizedVol7d := 0.35  // 7-day realized volatility
	pctAbove20MA := 65.0   // % of universe above 20MA  
	breadthThrust := 0.42  // Breadth thrust indicator
	
	// Calculate confidence based on indicator agreement
	confidence := 0.85
	if realizedVol7d > 0.5 || pctAbove20MA < 40 || breadthThrust < 0.3 {
		confidence = 0.65 // Lower confidence if indicators diverge
	}
	
	return RegimeData{
		RealizedVol7d: realizedVol7d,
		PctAbove20MA:  pctAbove20MA,
		BreadthThrust: breadthThrust,
		Confidence:    confidence,
		Timestamp:     time.Now(),
	}
}

// performRegimeMajorityVote determines regime based on indicator majority
func (s *Scheduler) performRegimeMajorityVote(data RegimeData) string {
	votes := []string{}
	
	// Vote 1: Realized volatility
	if data.RealizedVol7d < 0.25 {
		votes = append(votes, "calm")
	} else if data.RealizedVol7d > 0.45 {
		votes = append(votes, "volatile")
	} else {
		votes = append(votes, "normal")
	}
	
	// Vote 2: % above 20MA
	if data.PctAbove20MA > 70 {
		votes = append(votes, "calm")  // Strong trend
	} else if data.PctAbove20MA < 45 {
		votes = append(votes, "volatile") // Bearish/choppy
	} else {
		votes = append(votes, "normal")
	}
	
	// Vote 3: Breadth thrust
	if data.BreadthThrust > 0.6 {
		votes = append(votes, "calm")  // Strong breadth
	} else if data.BreadthThrust < 0.3 {
		votes = append(votes, "volatile") // Weak breadth
	} else {
		votes = append(votes, "normal")
	}
	
	// Count votes
	voteCount := make(map[string]int)
	for _, vote := range votes {
		voteCount[vote]++
	}
	
	// Return majority winner
	maxVotes := 0
	winner := "normal"
	for regime, count := range voteCount {
		if count > maxVotes {
			maxVotes = count
			winner = regime
		}
	}
	
	return winner
}

// writeRegimeJSON writes regime detection results to JSON
func (s *Scheduler) writeRegimeJSON(path, regime string, data RegimeData) error {
	regimeResult := map[string]interface{}{
		"timestamp": data.Timestamp.Format(time.RFC3339),
		"regime": regime,
		"indicators": map[string]interface{}{
			"realized_vol_7d": data.RealizedVol7d,
			"pct_above_20ma": data.PctAbove20MA,
			"breadth_thrust": data.BreadthThrust,
		},
		"confidence": data.Confidence,
		"votes": map[string]interface{}{
			"vol_vote": s.getVolVote(data.RealizedVol7d),
			"ma_vote": s.getMAVote(data.PctAbove20MA),
			"breadth_vote": s.getBreadthVote(data.BreadthThrust),
		},
		"weight_blend": s.getWeightBlendForRegime(regime),
		"next_refresh": data.Timestamp.Add(4 * time.Hour).Format(time.RFC3339),
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create regime JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(regimeResult); err != nil {
		return fmt.Errorf("failed to encode regime JSON: %w", err)
	}

	log.Info().Str("path", path).Str("regime", regime).Msg("Regime JSON written")
	return nil
}

// Helper methods for regime voting
func (s *Scheduler) getVolVote(vol float64) string {
	if vol < 0.25 { return "calm" }
	if vol > 0.45 { return "volatile" }
	return "normal"
}

func (s *Scheduler) getMAVote(pct float64) string {
	if pct > 70 { return "calm" }
	if pct < 45 { return "volatile" }
	return "normal"
}

func (s *Scheduler) getBreadthVote(breadth float64) string {
	if breadth > 0.6 { return "calm" }
	if breadth < 0.3 { return "volatile" }
	return "normal"
}

func (s *Scheduler) getWeightBlendForRegime(regime string) map[string]float64 {
	switch regime {
	case "calm":
		return map[string]float64{
			"momentum": 0.4,
			"technical": 0.3,
			"volume": 0.2,
			"quality": 0.1,
		}
	case "volatile":
		return map[string]float64{
			"momentum": 0.3,
			"technical": 0.2,
			"volume": 0.3,
			"quality": 0.2,
		}
	default: // normal
		return map[string]float64{
			"momentum": 0.35,
			"technical": 0.25,
			"volume": 0.25,
			"quality": 0.15,
		}
	}
}

// cacheRegimeResult caches the regime for use by scan jobs
func (s *Scheduler) cacheRegimeResult(regime string, data RegimeData) {
	// TODO: Implement actual caching (Redis, file cache, etc.)
	log.Info().Str("regime", regime).Time("timestamp", data.Timestamp).Msg("Regime result cached")
}

// writeSignalsCSV writes scan candidates to CSV format
func (s *Scheduler) writeSignalsCSV(path string, candidates []application.CandidateResult, scanType string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	// Write header with required columns for hot scan loop
	header := "timestamp,symbol,score,momentum_core,vadr,spread_bps,depth_usd,regime,fresh,venue,sources\n"
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	timestamp := time.Now().Format(time.RFC3339)
	
	// Write candidate data
	for _, candidate := range candidates {
		// Format with hot scan specific columns
		freshIndicator := "●" // Fresh indicator
		if !candidate.Gates.Freshness.OK {
			freshIndicator = "○" // Not fresh
		}
		
		// Extract venue from gates or default to kraken
		venue := "kraken"
		
		// Count sources - mock for now
		sources := 3 // Default source count

		// Get microstructure data from gates
		vadr := candidate.Gates.Microstructure.VADR
		spreadBps := candidate.Gates.Microstructure.SpreadBps
		depthUsd := candidate.Gates.Microstructure.DepthUSD

		// Get momentum core from factors
		momentumCore := candidate.Factors.MomentumCore

		line := fmt.Sprintf("%s,%s,%.1f,%.1f,%.1f,%.0f,%.0f,%s,%s,%s,%d\n",
			timestamp,
			candidate.Symbol,
			candidate.Score.Score,
			momentumCore, // Protected momentum score
			vadr,
			spreadBps,
			depthUsd,
			"normal", // TODO: Get actual regime from context
			freshIndicator,
			venue,
			sources,
		)
		
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write candidate data: %w", err)
		}
	}

	log.Info().Str("path", path).Int("candidates", len(candidates)).Msg("Signals CSV written")
	return nil
}

// writeWarmSignalsCSV writes warm scan candidates to CSV format
func (s *Scheduler) writeWarmSignalsCSV(path string, candidates []application.CandidateResult) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create warm CSV file: %w", err)
	}
	defer file.Close()

	// Write header for warm scan with cached indicator
	header := "timestamp,symbol,score,momentum_core,cached,regime,venue,ttl_remaining\n"
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write warm header: %w", err)
	}

	timestamp := time.Now().Format(time.RFC3339)
	
	// Write warm candidate data
	for _, candidate := range candidates {
		// Extract venue or default to kraken
		venue := "kraken"

		// Mock TTL remaining for cached sources
		ttlRemaining := 1500 // Warm scan uses longer cache TTL
		
		// Get momentum core from factors
		momentumCore := candidate.Factors.MomentumCore
		
		line := fmt.Sprintf("%s,%s,%.1f,%.1f,true,%s,%s,%d\n",
			timestamp,
			candidate.Symbol,
			candidate.Score.Score,
			momentumCore,
			"normal", // TODO: Get actual regime from context
			venue,
			ttlRemaining,
		)
		
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write warm candidate data: %w", err)
		}
	}

	log.Info().Str("path", path).Int("candidates", len(candidates)).Msg("Warm signals CSV written")
	return nil
}

// writePremoveCSV writes premove analysis to CSV format
func (s *Scheduler) writePremoveCSV(path string, candidates []application.CandidateResult) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create premove CSV file: %w", err)
	}
	defer file.Close()

	// Write header for premove data
	header := "timestamp,symbol,score,premove_signal,execution_risk,venue_health,latency_ms\n"
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write premove header: %w", err)
	}

	timestamp := time.Now().Format(time.RFC3339)
	
	// Write premove data for each candidate
	for _, candidate := range candidates {
		// Mock premove analysis data - in real implementation this would come from premove module
		premoveSignal := "LONG"
		if candidate.Score.Score < 70 {
			premoveSignal = "NEUTRAL"
		}
		
		executionRisk := "LOW"
		if candidate.Gates.Microstructure.SpreadBps > 25 {
			executionRisk = "MEDIUM"
		}

		line := fmt.Sprintf("%s,%s,%.1f,%s,%s,HEALTHY,%.0f\n",
			timestamp,
			candidate.Symbol,
			candidate.Score.Score,
			premoveSignal,
			executionRisk,
			150.0, // Mock latency
		)
		
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write premove data: %w", err)
		}
	}

	log.Info().Str("path", path).Int("candidates", len(candidates)).Msg("Premove CSV written")
	return nil
}

// writeExplainJSON writes explanations to JSON format
func (s *Scheduler) writeExplainJSON(path string, candidates []application.CandidateResult, jobType string) error {
	explanation := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"job_type":  jobType,
		"regime":    "normal", // TODO: Get actual regime
		"candidates": len(candidates),
		"explanations": []map[string]interface{}{},
		"gates": map[string]interface{}{
			"entry_threshold": 75.0,
			"vadr_minimum": 1.8,
			"spread_maximum": 50.0,
			"depth_minimum": 100000.0,
		},
		"weights": map[string]interface{}{
			"momentum_core": "protected", // Never orthogonalized
			"technical_residual": 0.25,
			"volume_residual": 0.25,
			"quality_residual": 0.25,
			"social_residual": 0.25,
			"social_cap": 10.0, // Max +10 points outside weight allocation
		},
	}

	// Add explanations for top candidates
	for i, candidate := range candidates {
		if i >= 5 { // Limit explanations to top 5
			break
		}
		
		// Get momentum core from factors
		momentumCore := candidate.Factors.MomentumCore
		
		candidateExplain := map[string]interface{}{
			"symbol": candidate.Symbol,
			"score": candidate.Score.Score,
			"momentum_core": momentumCore,
			"gate_passed": candidate.Score.Score >= 75.0,
			"all_gates_pass": candidate.Gates.AllPass,
			"reason": fmt.Sprintf("Momentum core: %.1f, Gates: %t", momentumCore, candidate.Gates.AllPass),
		}
		
		explanation["explanations"] = append(explanation["explanations"].([]map[string]interface{}), candidateExplain)
	}

	data, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create explain JSON file: %w", err)
	}
	defer data.Close()

	encoder := json.NewEncoder(data)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(explanation); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	log.Info().Str("path", path).Msg("Explanation JSON written")
	return nil
}

// createPlaceholderArtifact creates a placeholder file for testing
func createPlaceholderArtifact(path, jobType string) error {
	content := ""
	
	switch filepath.Ext(path) {
	case ".csv":
		if jobType == "scan.hot" {
			content = "timestamp,symbol,score,momentum_core,vadr,spread_bps,depth_usd,regime,fresh,venue,sources\n"
			content += fmt.Sprintf("%s,BTC/USD,78.5,65.2,2.1,15,150000,normal,●,kraken,3\n", time.Now().Format(time.RFC3339))
			content += fmt.Sprintf("%s,ETH/USD,82.1,71.8,1.9,12,200000,normal,●,okx,4\n", time.Now().Format(time.RFC3339))
		} else {
			content = "timestamp,symbol,score,cached\n"
			content += fmt.Sprintf("%s,ADA/USD,68.2,true\n", time.Now().Format(time.RFC3339))
		}
	case ".json":
		if jobType == "regime.refresh" {
			content = fmt.Sprintf(`{"timestamp": "%s", "regime": "normal", "indicators": {"realized_vol_7d": 0.35, "pct_above_20ma": 65, "breadth_thrust": 0.42}, "confidence": 0.85}`, time.Now().Format(time.RFC3339))
		} else {
			content = fmt.Sprintf(`{"timestamp": "%s", "type": "%s", "explanations": []}`, time.Now().Format(time.RFC3339), jobType)
		}
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// runProvidersHealth executes provider health monitoring and fallback logic
func (s *Scheduler) runProvidersHealth(ctx context.Context, job *Job, dryRun bool) ([]string, error) {
	log.Info().Msg("Running providers health check with rate-limits, circuit breakers, and fallback chains")
	
	if dryRun {
		log.Info().Msg("Dry run - would check rate-limits, parse headers, catch 429/418, apply budgets, fallback to secondary/tertiary, double cache_ttl on degradation")
		return []string{
			filepath.Join(s.config.Global.ArtifactsDir, fmt.Sprintf("%s_provider_health.json", time.Now().Format("20060102_150405"))),
		}, nil
	}

	// Create timestamp for this run
	timestamp := time.Now().Format("20060102_150405")
	outputDir := filepath.Join(s.config.Global.ArtifactsDir, timestamp)
	
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	log.Info().Str("venues", fmt.Sprintf("%v", job.Config.Venues)).
		Msg("Executing provider health check with rate-limit enforcement")

	// Check health for each configured provider
	healthResults := s.checkProvidersHealth(ctx, job.Config.Venues)
	
	// Apply fallback logic if needed
	s.applyProviderFallbacks(healthResults)
	
	// Update cache TTLs based on degradation
	s.adjustCacheTTLs(healthResults)
	
	log.Info().Int("providers_checked", len(healthResults)).
		Msg("Provider health check completed")

	// Generate artifacts
	artifacts := []string{
		filepath.Join(outputDir, "provider_health.json"),
	}

	// Write provider health JSON with detailed status
	if err := s.writeProviderHealthJSON(filepath.Join(outputDir, "provider_health.json"), healthResults); err != nil {
		return nil, fmt.Errorf("failed to write provider health JSON: %w", err)
	}

	log.Info().Int("artifacts", len(artifacts)).Msg("Provider health artifacts generated")
	return artifacts, nil
}

// runPremoveHourly executes hourly premove sweep with 2-of-3 gate enforcement
func (s *Scheduler) runPremoveHourly(ctx context.Context, job *Job, dryRun bool) ([]string, error) {
	log.Info().Str("universe", job.Config.Universe).
		Int("min_gates_passed", job.Config.MinGatesPassed).
		Strs("required_gates", job.Config.RequireGates).
		Msg("Running hourly premove sweep with 2-of-3 gate enforcement")
	
	if dryRun {
		log.Info().Msg("Dry run - would sweep top50 ADV, require 2-of-3 gates (funding divergence, supply squeeze, whale accumulation), check risk_off regime for volume confirm")
		return []string{
			filepath.Join(s.config.Global.ArtifactsDir, fmt.Sprintf("%s_premove_alerts.json", time.Now().Format("20060102_150405"))),
		}, nil
	}

	// Create timestamp for this run
	timestamp := time.Now().Format("20060102_150405")
	outputDir := filepath.Join(s.config.Global.ArtifactsDir, timestamp)
	
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	log.Info().Str("venues", fmt.Sprintf("%v", job.Config.Venues)).
		Int("min_gates", job.Config.MinGatesPassed).
		Bool("volume_confirm", job.Config.VolumeConfirm).
		Msg("Executing premove sweep with gate enforcement")

	// Initialize premove pipeline for top50 ADV sweep
	pipeline := application.NewScanPipeline(filepath.Join(outputDir, "microstructure"))
	
	// Check current regime to determine if volume confirmation is needed
	currentRegime := s.getCurrentCachedRegime()
	requireVolumeConfirm := job.Config.VolumeConfirm && (currentRegime == "risk_off" || currentRegime == "btc_driven")
	
	log.Info().Str("current_regime", currentRegime).
		Bool("require_volume_confirm", requireVolumeConfirm).
		Msg("Regime checked for volume confirmation requirement")

	// Execute premove analysis on top50 ADV universe
	candidates, err := pipeline.ScanUniverse(ctx)
	if err != nil {
		return nil, fmt.Errorf("premove sweep failed: %w", err)
	}

	// Apply 2-of-3 gate filtering with regime-aware volume confirmation
	premoveAlerts := s.filterCandidatesByPremoveGates(candidates, job.Config.RequireGates, job.Config.MinGatesPassed, requireVolumeConfirm)
	
	log.Info().Int("total_candidates", len(candidates)).
		Int("premove_alerts", len(premoveAlerts)).
		Int("min_gates_required", job.Config.MinGatesPassed).
		Bool("volume_confirm_applied", requireVolumeConfirm).
		Msg("Premove gate filtering completed")

	// Generate artifacts
	artifacts := []string{
		filepath.Join(outputDir, "alerts.json"),
	}

	// Write premove alerts JSON with attribution
	if err := s.writePremoveAlertsJSON(filepath.Join(outputDir, "alerts.json"), premoveAlerts, job.Config, currentRegime); err != nil {
		return nil, fmt.Errorf("failed to write premove alerts JSON: %w", err)
	}

	log.Info().Int("artifacts", len(artifacts)).Int("alerts_generated", len(premoveAlerts)).Msg("Premove hourly artifacts generated")
	return artifacts, nil
}

// ProviderHealthResult represents health status for a single provider
type ProviderHealthResult struct {
	Provider      string    `json:"provider"`
	Healthy       bool      `json:"healthy"`
	LastCheck     time.Time `json:"last_check"`
	ResponseTime  int       `json:"response_time_ms"`
	RateLimit     ProviderRateLimit `json:"rate_limit"`
	CircuitState  string    `json:"circuit_state"`
	ErrorRate     float64   `json:"error_rate"`
	Fallback      string    `json:"fallback,omitempty"`
	CacheTTL      int       `json:"cache_ttl_seconds"`
}

// ProviderRateLimit represents rate limit status from headers
type ProviderRateLimit struct {
	Used      int     `json:"used"`
	Limit     int     `json:"limit"`
	Usage     float64 `json:"usage_percent"`
	Budget    ProviderBudget `json:"budget"`
}

// ProviderBudget represents monthly/daily budget tracking
type ProviderBudget struct {
	MonthlyUsed  int     `json:"monthly_used"`
	MonthlyLimit int     `json:"monthly_limit"`
	DailyUsed    int     `json:"daily_used"`
	DailyLimit   int     `json:"daily_limit"`
	BudgetAlert  bool    `json:"budget_alert"`
}

// checkProvidersHealth monitors all configured providers
func (s *Scheduler) checkProvidersHealth(ctx context.Context, venues []string) []ProviderHealthResult {
	results := []ProviderHealthResult{}
	
	for _, venue := range venues {
		result := s.checkSingleProviderHealth(ctx, venue)
		results = append(results, result)
	}
	
	return results
}

// checkSingleProviderHealth checks health for one provider
func (s *Scheduler) checkSingleProviderHealth(ctx context.Context, provider string) ProviderHealthResult {
	startTime := time.Now()
	
	// Mock implementation - in real system would make actual HTTP call
	// and parse rate limit headers like X-MBX-USED-WEIGHT-1M
	
	// Simulate different provider conditions
	responseTime := 150 // Base response time
	healthy := true
	circuitState := "CLOSED"
	errorRate := 0.02
	
	// Simulate provider-specific conditions
	switch provider {
	case "binance":
		responseTime = 120
		// Simulate rate limit usage from headers
		used := 800  // From X-MBX-USED-WEIGHT-1M header
		limit := 1200
		usage := float64(used) / float64(limit) * 100
		
		if usage > 90 {
			healthy = false
			circuitState = "HALF_OPEN"
			errorRate = 0.15
		}
		
		return ProviderHealthResult{
			Provider:     provider,
			Healthy:      healthy,
			LastCheck:    time.Now(),
			ResponseTime: responseTime,
			RateLimit: ProviderRateLimit{
				Used:  used,
				Limit: limit,
				Usage: usage,
				Budget: ProviderBudget{
					MonthlyUsed:  45000,
					MonthlyLimit: 100000,
					DailyUsed:    2100,
					DailyLimit:   5000,
					BudgetAlert:  false,
				},
			},
			CircuitState: circuitState,
			ErrorRate:    errorRate,
			CacheTTL:     300, // Normal TTL
		}
		
	case "okx":
		responseTime = 180
		// Simulate 429 rate limit hit
		healthy = false
		circuitState = "OPEN"
		errorRate = 0.25
		
		return ProviderHealthResult{
			Provider:     provider,
			Healthy:      healthy,
			LastCheck:    time.Now(),
			ResponseTime: responseTime,
			RateLimit: ProviderRateLimit{
				Used:  1200,
				Limit: 1200,
				Usage: 100.0, // Hit rate limit
				Budget: ProviderBudget{
					MonthlyUsed:  95000,
					MonthlyLimit: 100000,
					DailyUsed:    4900,
					DailyLimit:   5000,
					BudgetAlert:  true, // Near budget limit
				},
			},
			CircuitState: circuitState,
			ErrorRate:    errorRate,
			Fallback:     "coinbase", // Will fallback to coinbase
			CacheTTL:     600, // Doubled TTL due to degradation
		}
		
	default: // kraken, coinbase, etc.
		return ProviderHealthResult{
			Provider:     provider,
			Healthy:      true,
			LastCheck:    time.Now(),
			ResponseTime: responseTime,
			RateLimit: ProviderRateLimit{
				Used:  300,
				Limit: 1000,
				Usage: 30.0,
				Budget: ProviderBudget{
					MonthlyUsed:  25000,
					MonthlyLimit: 100000,
					DailyUsed:    1200,
					DailyLimit:   5000,
					BudgetAlert:  false,
				},
			},
			CircuitState: "CLOSED",
			ErrorRate:    0.01,
			CacheTTL:     300,
		}
	}
}

// applyProviderFallbacks implements fallback logic for degraded providers
func (s *Scheduler) applyProviderFallbacks(results []ProviderHealthResult) {
	for i, result := range results {
		if !result.Healthy || result.CircuitState == "OPEN" {
			// Apply fallback logic
			fallback := s.determineFallbackProvider(result.Provider)
			results[i].Fallback = fallback
			
			log.Warn().Str("provider", result.Provider).
				Str("fallback", fallback).
				Str("reason", result.CircuitState).
				Float64("error_rate", result.ErrorRate).
				Msg("Provider fallback applied")
		}
	}
}

// determineFallbackProvider determines which provider to fallback to
func (s *Scheduler) determineFallbackProvider(primary string) string {
	// Fallback hierarchy: okx -> coinbase -> kraken (in order of preference)
	fallbackMap := map[string][]string{
		"binance":  {"okx", "coinbase", "kraken"},
		"okx":      {"coinbase", "kraken", "binance"},
		"coinbase": {"kraken", "binance", "okx"},
		"kraken":   {"binance", "okx", "coinbase"},
	}
	
	if fallbacks, exists := fallbackMap[primary]; exists && len(fallbacks) > 0 {
		return fallbacks[0] // Return primary fallback
	}
	
	return "kraken" // Default fallback
}

// adjustCacheTTLs doubles cache TTL for degraded providers
func (s *Scheduler) adjustCacheTTLs(results []ProviderHealthResult) {
	for i, result := range results {
		if !result.Healthy || result.RateLimit.Usage > 80 {
			// Double cache TTL for degraded providers
			results[i].CacheTTL = result.CacheTTL * 2
			
			log.Info().Str("provider", result.Provider).
				Int("original_ttl", result.CacheTTL/2).
				Int("new_ttl", result.CacheTTL).
				Float64("usage_percent", result.RateLimit.Usage).
				Msg("Cache TTL doubled for degraded provider")
		}
	}
}

// getCurrentCachedRegime gets the current regime from cache
func (s *Scheduler) getCurrentCachedRegime() string {
	// TODO: Implement actual regime cache lookup
	// For now, return mock regime
	return "normal" // Could be "normal", "risk_off", "btc_driven", etc.
}

// PremoveAlert represents a premove alert with attribution
type PremoveAlert struct {
	Symbol             string    `json:"symbol"`
	TotalScore         float64   `json:"total_score"`
	GatesPassed        []string  `json:"gates_passed"`
	MicrostructureVerdict string `json:"microstructure_verdict"`
	WhyPassed          string    `json:"why_passed"`
	WhyNotPassed       string    `json:"why_not_passed,omitempty"`
	ETAWindow          string    `json:"eta_window"`
	Timestamp          time.Time `json:"timestamp"`
	VolumeConfirmed    bool      `json:"volume_confirmed"`
}

// filterCandidatesByPremoveGates applies 2-of-3 gate filtering
func (s *Scheduler) filterCandidatesByPremoveGates(candidates []application.CandidateResult, requiredGates []string, minGates int, requireVolumeConfirm bool) []PremoveAlert {
	alerts := []PremoveAlert{}
	
	for _, candidate := range candidates {
		// Check each required gate
		gatesPassed := []string{}
		gateReasons := []string{}
		
		for _, gateName := range requiredGates {
			passed, reason := s.evaluatePremoveGate(candidate, gateName)
			if passed {
				gatesPassed = append(gatesPassed, gateName)
			}
			gateReasons = append(gateReasons, fmt.Sprintf("%s: %s", gateName, reason))
		}
		
		// Check volume confirmation if required by regime
		volumeConfirmed := true
		if requireVolumeConfirm {
			volumeConfirmed = candidate.Gates.Volume.OK && candidate.Factors.VolumeScore > 70
		}
		
		// Check if minimum gates passed and volume confirmed (if required)
		if len(gatesPassed) >= minGates && volumeConfirmed {
			// Generate microstructure verdict
			microVerdict := "PASS"
			if candidate.Gates.Microstructure.SpreadBps > 25 {
				microVerdict = "CAUTION"
			}
			if !candidate.Gates.Microstructure.OK {
				microVerdict = "FAIL"
			}
			
			// Generate why passed explanation
			whyPassed := fmt.Sprintf("Gates passed: %d/%d (%s)", len(gatesPassed), len(requiredGates), 
				fmt.Sprintf("%v", gatesPassed))
			if requireVolumeConfirm {
				whyPassed += fmt.Sprintf(", Volume confirmed: %t", volumeConfirmed)
			}
			
			// Generate ETA window based on momentum acceleration
			etaWindow := "2-6h"
			if candidate.Factors.MomentumCore > 80 {
				etaWindow = "30m-2h" // Faster for high momentum
			}
			
			alert := PremoveAlert{
				Symbol:                candidate.Symbol,
				TotalScore:            candidate.Score.Score,
				GatesPassed:           gatesPassed,
				MicrostructureVerdict: microVerdict,
				WhyPassed:             whyPassed,
				ETAWindow:             etaWindow,
				Timestamp:             time.Now(),
				VolumeConfirmed:       volumeConfirmed,
			}
			
			alerts = append(alerts, alert)
			
		} else {
			// Log why alert was not generated
			whyNot := fmt.Sprintf("Gates passed: %d/%d (need %d)", len(gatesPassed), len(requiredGates), minGates)
			if requireVolumeConfirm && !volumeConfirmed {
				whyNot += ", Volume confirmation failed"
			}
			
			log.Debug().Str("symbol", candidate.Symbol).
				Float64("score", candidate.Score.Score).
				Int("gates_passed", len(gatesPassed)).
				Int("min_required", minGates).
				Bool("volume_confirmed", volumeConfirmed).
				Str("reason", whyNot).
				Msg("Premove alert not generated")
		}
	}
	
	return alerts
}

// evaluatePremoveGate evaluates a single premove gate
func (s *Scheduler) evaluatePremoveGate(candidate application.CandidateResult, gateName string) (bool, string) {
	switch gateName {
	case "funding_divergence":
		// Mock funding divergence check
		passed := candidate.Factors.FundingScore > 2.0 // 2σ threshold
		reason := fmt.Sprintf("funding z-score: %.2f (need >2.0)", candidate.Factors.FundingScore)
		return passed, reason
		
	case "supply_squeeze":
		// Mock supply squeeze check  
		passed := candidate.Factors.QualityScore > 70 && candidate.Gates.Microstructure.DepthUSD < 80000
		reason := fmt.Sprintf("quality: %.1f, depth: %.0f (squeeze detected: %t)", 
			candidate.Factors.QualityScore, candidate.Gates.Microstructure.DepthUSD, passed)
		return passed, reason
		
	case "whale_accumulation":
		// Mock whale accumulation check
		passed := candidate.Factors.VolumeScore > 75 && candidate.Factors.MomentumCore > 70
		reason := fmt.Sprintf("volume: %.1f, momentum: %.1f (whale activity: %t)", 
			candidate.Factors.VolumeScore, candidate.Factors.MomentumCore, passed)
		return passed, reason
		
	default:
		return false, fmt.Sprintf("unknown gate: %s", gateName)
	}
}

// writeProviderHealthJSON writes provider health status to JSON
func (s *Scheduler) writeProviderHealthJSON(path string, results []ProviderHealthResult) error {
	healthReport := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"overall_status": s.calculateOverallHealthStatus(results),
		"providers": results,
		"health_banner": s.generateHealthBanner(results),
		"fallbacks_active": s.countActiveFallbacks(results),
		"degraded_providers": s.listDegradedProviders(results),
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create provider health JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(healthReport); err != nil {
		return fmt.Errorf("failed to encode provider health JSON: %w", err)
	}

	log.Info().Str("path", path).Int("providers", len(results)).Msg("Provider health JSON written")
	return nil
}

// writePremoveAlertsJSON writes premove alerts with attribution to JSON
func (s *Scheduler) writePremoveAlertsJSON(path string, alerts []PremoveAlert, config JobConfig, regime string) error {
	alertReport := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"regime": regime,
		"gates_config": map[string]interface{}{
			"required_gates": config.RequireGates,
			"min_gates_passed": config.MinGatesPassed,
			"volume_confirm": config.VolumeConfirm,
		},
		"alerts_generated": len(alerts),
		"alerts": alerts,
		"next_sweep": time.Now().Add(time.Hour).Format(time.RFC3339),
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create premove alerts JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(alertReport); err != nil {
		return fmt.Errorf("failed to encode premove alerts JSON: %w", err)
	}

	log.Info().Str("path", path).Int("alerts", len(alerts)).Msg("Premove alerts JSON written")
	return nil
}

// calculateOverallHealthStatus determines overall system health
func (s *Scheduler) calculateOverallHealthStatus(results []ProviderHealthResult) string {
	healthyCount := 0
	totalCount := len(results)
	
	for _, result := range results {
		if result.Healthy {
			healthyCount++
		}
	}
	
	healthyPercent := float64(healthyCount) / float64(totalCount) * 100
	
	if healthyPercent >= 80 {
		return "HEALTHY"
	} else if healthyPercent >= 50 {
		return "DEGRADED"
	} else {
		return "CRITICAL"
	}
}

// generateHealthBanner creates health status banner for CLI
func (s *Scheduler) generateHealthBanner(results []ProviderHealthResult) string {
	banner := "API Health: "
	
	for i, result := range results {
		if i > 0 {
			banner += " | "
		}
		
		status := "✓"
		if !result.Healthy {
			status = "✗"
		}
		
		banner += fmt.Sprintf("%s %s (%dms)", result.Provider, status, result.ResponseTime)
	}
	
	return banner
}

// countActiveFallbacks counts how many fallbacks are currently active
func (s *Scheduler) countActiveFallbacks(results []ProviderHealthResult) int {
	count := 0
	for _, result := range results {
		if result.Fallback != "" {
			count++
		}
	}
	return count
}

// listDegradedProviders lists providers that are degraded
func (s *Scheduler) listDegradedProviders(results []ProviderHealthResult) []string {
	degraded := []string{}
	for _, result := range results {
		if !result.Healthy || result.RateLimit.Usage > 80 {
			degraded = append(degraded, result.Provider)
		}
	}
	return degraded
}