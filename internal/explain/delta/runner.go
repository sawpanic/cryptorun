package delta

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"cryptorun/internal/application"
	"cryptorun/internal/application/universe"
)

// Runner executes delta analysis against baselines
type Runner struct {
	config     *Config
	comparator *Comparator
	tolerance  *ToleranceConfig
}

// NewRunner creates a new delta analysis runner
func NewRunner(config *Config) *Runner {
	return &Runner{
		config:     config,
		comparator: NewComparator(),
		tolerance:  loadToleranceConfig(),
	}
}

// Run executes the complete delta analysis pipeline
func (r *Runner) Run(ctx context.Context) (*Results, error) {
	log.Info().
		Str("universe", r.config.Universe).
		Str("baseline", r.config.BaselinePath).
		Msg("Starting explain delta analysis")

	// Parse universe specification
	pairs, err := r.parseUniverse(r.config.Universe)
	if err != nil {
		return nil, fmt.Errorf("failed to parse universe: %w", err)
	}

	if r.config.Progress {
		fmt.Printf("⏳ [20%%] Parsed universe: %d pairs\n", len(pairs))
	}

	// Load current factors for universe
	currentFactors, regime, err := r.loadCurrentFactors(ctx, pairs)
	if err != nil {
		return nil, fmt.Errorf("failed to load current factors: %w", err)
	}

	if r.config.Progress {
		fmt.Printf("⏳ [40%%] Loaded current factors for regime: %s\n", regime)
	}

	// Load baseline snapshot
	baseline, err := r.loadBaseline(r.config.BaselinePath, r.config.Universe)
	if err != nil {
		return nil, fmt.Errorf("failed to load baseline: %w", err)
	}

	if r.config.Progress {
		fmt.Printf("⏳ [60%%] Loaded baseline from %s\n", baseline.Timestamp.Format("2006-01-02T15:04Z"))
	}

	// Perform delta comparison
	results, err := r.comparator.Compare(baseline, currentFactors, regime, r.tolerance)
	if err != nil {
		return nil, fmt.Errorf("delta comparison failed: %w", err)
	}

	if r.config.Progress {
		fmt.Printf("⏳ [80%%] Completed delta analysis\n")
	}

	// Populate results metadata
	results.Universe = r.config.Universe
	results.Regime = regime
	results.BaselineTimestamp = baseline.Timestamp
	results.CurrentTimestamp = time.Now()
	results.ToleranceConfig = r.tolerance

	if r.config.Progress {
		fmt.Printf("⏳ [100%%] Analysis complete\n")
	}

	log.Info().
		Int("total_assets", results.TotalAssets).
		Int("fail_count", results.FailCount).
		Int("warn_count", results.WarnCount).
		Int("ok_count", results.OKCount).
		Msg("Delta analysis completed")

	return results, nil
}

// parseUniverse parses universe specification into symbol list
func (r *Runner) parseUniverse(universeSpec string) ([]string, error) {
	// Handle topN=X format
	if strings.HasPrefix(universeSpec, "topN=") {
		topNStr := strings.TrimPrefix(universeSpec, "topN=")
		var topN int
		if _, err := fmt.Sscanf(topNStr, "%d", &topN); err != nil {
			return nil, fmt.Errorf("invalid topN format: %s", topNStr)
		}

		// Load universe and take top N
		universeBuilder := universe.NewBuilder()
		pairs, err := universeBuilder.LoadUniverse()
		if err != nil {
			return nil, fmt.Errorf("failed to load universe: %w", err)
		}

		if len(pairs) > topN {
			pairs = pairs[:topN]
		}

		return pairs, nil
	}

	// Handle comma-separated list
	if strings.Contains(universeSpec, ",") {
		return strings.Split(universeSpec, ","), nil
	}

	// Single symbol
	return []string{universeSpec}, nil
}

// loadCurrentFactors generates current factor snapshot for given pairs
func (r *Runner) loadCurrentFactors(ctx context.Context, pairs []string) (map[string]*AssetFactors, string, error) {
	// Create scanner to get current regime and factors
	scanner := application.NewScanPipeline("out/microstructure/snapshots")

	// Get current regime
	regime := scanner.GetCurrentRegime()

	factors := make(map[string]*AssetFactors)

	// Generate factors for each pair
	for _, symbol := range pairs {
		// Generate current explain data
		explain, err := scanner.ExplainSymbol(ctx, symbol)
		if err != nil {
			log.Warn().Str("symbol", symbol).Err(err).Msg("Failed to explain symbol, using zeros")
			factors[symbol] = &AssetFactors{
				Symbol:         symbol,
				Regime:         regime,
				MomentumCore:   0.0,
				TechnicalResid: 0.0,
				VolumeResid:    0.0,
				QualityResid:   0.0,
				SocialResid:    0.0,
				CompositeScore: 0.0,
				Gates:          make(map[string]bool),
			}
			continue
		}

		factors[symbol] = &AssetFactors{
			Symbol:         symbol,
			Regime:         regime,
			MomentumCore:   explain.MomentumCore,
			TechnicalResid: explain.TechnicalResid,
			VolumeResid:    explain.VolumeResid,
			QualityResid:   explain.QualityResid,
			SocialResid:    explain.SocialResid,
			CompositeScore: explain.CompositeScore,
			Gates:          explain.Gates,
		}
	}

	return factors, regime, nil
}

// loadBaseline loads baseline snapshot from specified path
func (r *Runner) loadBaseline(baselinePath, universeSpec string) (*BaselineSnapshot, error) {
	if baselinePath == "latest" {
		return r.loadLatestBaseline(universeSpec)
	}

	// Check if it's a date format (YYYY-MM-DD)
	if len(baselinePath) == 10 && strings.Count(baselinePath, "-") == 2 {
		return r.loadBaselineByDate(baselinePath, universeSpec)
	}

	// Treat as explicit file path
	return r.loadBaselineFromPath(baselinePath)
}

// loadLatestBaseline finds and loads the most recent baseline
func (r *Runner) loadLatestBaseline(universeSpec string) (*BaselineSnapshot, error) {
	baselineDir := "artifacts/explain_baselines"
	if _, err := os.Stat(baselineDir); os.IsNotExist(err) {
		return r.generateSyntheticBaseline(universeSpec)
	}

	// Find latest baseline file
	entries, err := os.ReadDir(baselineDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read baseline directory: %w", err)
	}

	var latestFile string
	var latestTime time.Time

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = entry.Name()
		}
	}

	if latestFile == "" {
		return r.generateSyntheticBaseline(universeSpec)
	}

	return r.loadBaselineFromPath(filepath.Join(baselineDir, latestFile))
}

// loadBaselineByDate loads baseline for specific date
func (r *Runner) loadBaselineByDate(date, universeSpec string) (*BaselineSnapshot, error) {
	baselineFile := fmt.Sprintf("artifacts/explain_baselines/baseline_%s.json", date)
	if _, err := os.Stat(baselineFile); os.IsNotExist(err) {
		log.Warn().Str("date", date).Msg("Baseline not found for date, generating synthetic")
		return r.generateSyntheticBaseline(universeSpec)
	}

	return r.loadBaselineFromPath(baselineFile)
}

// loadBaselineFromPath loads baseline from explicit file path
func (r *Runner) loadBaselineFromPath(path string) (*BaselineSnapshot, error) {
	// For now, return synthetic baseline since we don't have JSON loading implemented
	// This would normally load and unmarshal JSON
	log.Warn().Str("path", path).Msg("JSON loading not implemented, generating synthetic baseline")
	return r.generateSyntheticBaseline("synthetic")
}

// generateSyntheticBaseline creates a synthetic baseline for testing
func (r *Runner) generateSyntheticBaseline(universeSpec string) (*BaselineSnapshot, error) {
	log.Info().Str("universe", universeSpec).Msg("Generating synthetic baseline for testing")

	// Create synthetic factors with small variations
	factors := map[string]*AssetFactors{
		"BTCUSD": {
			Symbol:         "BTCUSD",
			Regime:         "bull",
			MomentumCore:   75.2,
			TechnicalResid: 12.1,
			VolumeResid:    8.7,
			QualityResid:   4.3,
			SocialResid:    2.1,
			CompositeScore: 78.4,
			Gates:          map[string]bool{"freshness": true, "fatigue": true, "late_fill": true},
		},
		"ETHUSD": {
			Symbol:         "ETHUSD",
			Regime:         "bull",
			MomentumCore:   68.9,
			TechnicalResid: 15.2,
			VolumeResid:    11.4,
			QualityResid:   3.8,
			SocialResid:    4.2,
			CompositeScore: 72.3,
			Gates:          map[string]bool{"freshness": true, "fatigue": true, "late_fill": false},
		},
		"SOLUSD": {
			Symbol:         "SOLUSD",
			Regime:         "bull",
			MomentumCore:   82.1,
			TechnicalResid: 9.8,
			VolumeResid:    6.2,
			QualityResid:   5.1,
			SocialResid:    7.3,
			CompositeScore: 85.4,
			Gates:          map[string]bool{"freshness": true, "fatigue": false, "late_fill": true},
		},
	}

	return &BaselineSnapshot{
		Timestamp:  time.Now().Add(-24 * time.Hour), // Yesterday
		Universe:   universeSpec,
		Regime:     "bull",
		AssetCount: len(factors),
		Factors:    factors,
	}, nil
}

// loadToleranceConfig loads tolerance configuration from YAML
func loadToleranceConfig() *ToleranceConfig {
	// Return default configuration for now
	// This would normally load from config/explain_tolerances.yaml
	return &ToleranceConfig{
		Regimes: map[string]*RegimeTolerance{
			"bull": {
				Name: "bull",
				FactorTolerances: map[string]*FactorTolerance{
					"momentum_core":   {Factor: "momentum_core", WarnAt: 8.0, FailAt: 15.0, Direction: "both"},
					"technical_resid": {Factor: "technical_resid", WarnAt: 5.0, FailAt: 10.0, Direction: "both"},
					"volume_resid":    {Factor: "volume_resid", WarnAt: 4.0, FailAt: 8.0, Direction: "both"},
					"quality_resid":   {Factor: "quality_resid", WarnAt: 3.0, FailAt: 6.0, Direction: "both"},
					"social_resid":    {Factor: "social_resid", WarnAt: 2.0, FailAt: 5.0, Direction: "both"},
					"composite_score": {Factor: "composite_score", WarnAt: 10.0, FailAt: 20.0, Direction: "both"},
				},
			},
			"choppy": {
				Name: "choppy",
				FactorTolerances: map[string]*FactorTolerance{
					"momentum_core":   {Factor: "momentum_core", WarnAt: 12.0, FailAt: 20.0, Direction: "both"},
					"technical_resid": {Factor: "technical_resid", WarnAt: 8.0, FailAt: 15.0, Direction: "both"},
					"volume_resid":    {Factor: "volume_resid", WarnAt: 6.0, FailAt: 12.0, Direction: "both"},
					"quality_resid":   {Factor: "quality_resid", WarnAt: 5.0, FailAt: 10.0, Direction: "both"},
					"social_resid":    {Factor: "social_resid", WarnAt: 3.0, FailAt: 7.0, Direction: "both"},
					"composite_score": {Factor: "composite_score", WarnAt: 15.0, FailAt: 25.0, Direction: "both"},
				},
			},
			"high_vol": {
				Name: "high_vol",
				FactorTolerances: map[string]*FactorTolerance{
					"momentum_core":   {Factor: "momentum_core", WarnAt: 15.0, FailAt: 25.0, Direction: "both"},
					"technical_resid": {Factor: "technical_resid", WarnAt: 10.0, FailAt: 18.0, Direction: "both"},
					"volume_resid":    {Factor: "volume_resid", WarnAt: 8.0, FailAt: 15.0, Direction: "both"},
					"quality_resid":   {Factor: "quality_resid", WarnAt: 7.0, FailAt: 12.0, Direction: "both"},
					"social_resid":    {Factor: "social_resid", WarnAt: 5.0, FailAt: 10.0, Direction: "both"},
					"composite_score": {Factor: "composite_score", WarnAt: 20.0, FailAt: 35.0, Direction: "both"},
				},
			},
		},
	}
}
