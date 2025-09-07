package data

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BenchResult represents a top gainers benchmark result
type BenchResult struct {
	Symbol       string    `json:"symbol"`
	Timestamp    time.Time `json:"timestamp"`
	Score        float64   `json:"score"`
	Regime       string    `json:"regime"`
	Rank         int       `json:"rank"`          // Rank within batch (1-based)
	BatchSize    int       `json:"batch_size"`    // Total symbols in batch
	ActualGain   float64   `json:"actual_gain"`   // Actual price gain (benchmark period)
	BenchmarkHit bool      `json:"benchmark_hit"` // True if in top 20% of actual gainers
	Window       string    `json:"window"`        // Benchmark window (24h typical)
}

// BenchDataLoader loads and processes top gainers benchmark results
type BenchDataLoader struct {
	artifactDir string
}

// NewBenchDataLoader creates a new bench data loader
func NewBenchDataLoader(artifactDir string) *BenchDataLoader {
	if artifactDir == "" {
		artifactDir = "./artifacts/bench"
	}
	return &BenchDataLoader{
		artifactDir: artifactDir,
	}
}

// LoadResults loads benchmark results filtered by regime and windows
func (bdl *BenchDataLoader) LoadResults(regimes []string, windows []string) ([]BenchResult, error) {
	var allResults []BenchResult

	// Create regime filter map for fast lookup
	regimeFilter := make(map[string]bool)
	for _, regime := range regimes {
		regimeFilter[regime] = true
	}

	// Create window filter map
	windowFilter := make(map[string]bool)
	for _, window := range windows {
		windowFilter[window] = true
	}

	// Scan artifact directory for benchmark results
	err := filepath.Walk(bdl.artifactDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(info.Name(), "_topgainers.json") {
			return nil
		}

		results, err := bdl.loadBenchFile(path)
		if err != nil {
			// Log warning but continue processing other files
			fmt.Printf("Warning: failed to load %s: %v\n", path, err)
			return nil
		}

		// Filter results by regime and window
		for _, result := range results {
			if len(regimeFilter) > 0 && !regimeFilter[result.Regime] {
				continue
			}
			if len(windowFilter) > 0 && !windowFilter[result.Window] {
				continue
			}
			allResults = append(allResults, result)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk bench artifact directory: %w", err)
	}

	// Sort results by timestamp for consistent ordering
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Timestamp.Before(allResults[j].Timestamp)
	})

	return allResults, nil
}

// loadBenchFile loads a single benchmark result file
func (bdl *BenchDataLoader) loadBenchFile(filePath string) ([]BenchResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var results []BenchResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}

	// Calculate benchmark hit status for each batch
	bdl.calculateBenchmarkHits(results)

	return results, nil
}

// calculateBenchmarkHits determines which results are "hits" based on actual vs predicted performance
func (bdl *BenchDataLoader) calculateBenchmarkHits(results []BenchResult) {
	// Group results by timestamp (batch) for proper ranking comparison
	batches := make(map[time.Time][]int)
	for i, result := range results {
		batches[result.Timestamp] = append(batches[result.Timestamp], i)
	}

	// For each batch, determine top 20% by actual gains
	for _, batch := range batches {
		if len(batch) == 0 {
			continue
		}

		// Sort batch indices by actual gain (descending)
		sort.Slice(batch, func(i, j int) bool {
			return results[batch[i]].ActualGain > results[batch[j]].ActualGain
		})

		// Mark top 20% as benchmark hits
		topCount := max(1, len(batch)/5) // At least 1, or 20% of batch
		for i := 0; i < topCount && i < len(batch); i++ {
			results[batch[i]].BenchmarkHit = true
		}
	}
}

// GetBenchMetricsByRegime calculates benchmark performance metrics grouped by regime
func (bdl *BenchDataLoader) GetBenchMetricsByRegime(results []BenchResult) map[string]BenchMetrics {
	regimeGroups := make(map[string][]BenchResult)

	// Group results by regime
	for _, result := range results {
		regimeGroups[result.Regime] = append(regimeGroups[result.Regime], result)
	}

	// Calculate metrics for each regime
	metrics := make(map[string]BenchMetrics)
	for regime, regimeResults := range regimeGroups {
		metrics[regime] = bdl.calculateBenchMetrics(regimeResults)
	}

	return metrics
}

// BenchMetrics holds benchmark performance metrics for a regime
type BenchMetrics struct {
	Regime        string  `json:"regime"`
	TotalSymbols  int     `json:"total_symbols"`
	BenchmarkHits int     `json:"benchmark_hits"`
	HitRate       float64 `json:"hit_rate"`         // % that were actual top gainers
	AvgRank       float64 `json:"avg_rank"`         // Average predicted rank
	AvgActualGain float64 `json:"avg_actual_gain"`  // Average actual gain
	RankCorr      float64 `json:"rank_correlation"` // Spearman correlation: predicted rank vs actual gain
	PrecisionAt5  float64 `json:"precision_at_5"`   // % of top-5 predictions that were actual hits
	PrecisionAt10 float64 `json:"precision_at_10"`  // % of top-10 predictions that were actual hits
}

// calculateBenchMetrics computes benchmark metrics for a set of results
func (bdl *BenchDataLoader) calculateBenchMetrics(results []BenchResult) BenchMetrics {
	if len(results) == 0 {
		return BenchMetrics{}
	}

	var benchmarkHits int
	var totalRank, totalGain float64

	// Collect data for correlation analysis
	ranks := make([]float64, len(results))
	gains := make([]float64, len(results))

	// Count precision metrics
	var top5Hits, top10Hits int
	var top5Count, top10Count int

	for i, result := range results {
		if result.BenchmarkHit {
			benchmarkHits++
		}

		totalRank += float64(result.Rank)
		totalGain += result.ActualGain

		ranks[i] = float64(result.Rank)
		gains[i] = result.ActualGain

		// Precision@K calculations (within each batch)
		if result.Rank <= 5 {
			top5Count++
			if result.BenchmarkHit {
				top5Hits++
			}
		}
		if result.Rank <= 10 {
			top10Count++
			if result.BenchmarkHit {
				top10Hits++
			}
		}
	}

	hitRate := float64(benchmarkHits) / float64(len(results))
	avgRank := totalRank / float64(len(results))
	avgActualGain := totalGain / float64(len(results))

	// Calculate rank correlation (lower rank should correlate with higher gain)
	// Invert ranks for correlation calculation
	invertedRanks := make([]float64, len(ranks))
	for i, rank := range ranks {
		invertedRanks[i] = -rank // Negative so higher gain correlates positively
	}
	rankCorr := calculateSpearmanCorrelation(invertedRanks, gains)

	// Calculate precision metrics
	var precisionAt5, precisionAt10 float64
	if top5Count > 0 {
		precisionAt5 = float64(top5Hits) / float64(top5Count)
	}
	if top10Count > 0 {
		precisionAt10 = float64(top10Hits) / float64(top10Count)
	}

	return BenchMetrics{
		Regime:        results[0].Regime,
		TotalSymbols:  len(results),
		BenchmarkHits: benchmarkHits,
		HitRate:       hitRate,
		AvgRank:       avgRank,
		AvgActualGain: avgActualGain,
		RankCorr:      rankCorr,
		PrecisionAt5:  precisionAt5,
		PrecisionAt10: precisionAt10,
	}
}

// GetAvailableBenchRegimes scans bench artifacts and returns available regime labels
func (bdl *BenchDataLoader) GetAvailableBenchRegimes() ([]string, error) {
	regimeSet := make(map[string]bool)

	err := filepath.Walk(bdl.artifactDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(info.Name(), "_topgainers.json") {
			return nil
		}

		results, err := bdl.loadBenchFile(path)
		if err != nil {
			return nil // Skip files with errors
		}

		for _, result := range results {
			regimeSet[result.Regime] = true
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert set to sorted slice
	regimes := make([]string, 0, len(regimeSet))
	for regime := range regimeSet {
		regimes = append(regimes, regime)
	}
	sort.Strings(regimes)

	return regimes, nil
}

// CreateMockBenchResults creates mock benchmark results for testing
func CreateMockBenchResults(regimes []string, windows []string, batchSize int, batches int) []BenchResult {
	var results []BenchResult
	baseTime := time.Now().Add(-7 * 24 * time.Hour)

	for batch := 0; batch < batches; batch++ {
		timestamp := baseTime.Add(time.Duration(batch) * time.Hour)

		for _, regime := range regimes {
			for _, window := range windows {
				// Create one batch of results
				batchResults := make([]BenchResult, batchSize)

				for i := 0; i < batchSize; i++ {
					// Create deterministic but varied mock data
					seed := float64(batch*1000 + i*100 + hashString(regime)*10 + hashString(window))
					score := 60.0 + math.Mod(seed*0.0876, 40.0) // Score 60-100

					// Actual gain with some correlation to score + noise
					actualGain := (score-75)/2000 + math.Mod(seed*0.0045, 0.08) - 0.02 // -2% to +6%

					batchResults[i] = BenchResult{
						Symbol:     fmt.Sprintf("ASSET%d", i),
						Timestamp:  timestamp,
						Score:      score,
						Regime:     regime,
						Rank:       i + 1, // Will be re-ranked by score
						BatchSize:  batchSize,
						ActualGain: actualGain,
						Window:     window,
					}
				}

				// Sort by score to assign proper ranks
				sort.Slice(batchResults, func(i, j int) bool {
					return batchResults[i].Score > batchResults[j].Score
				})

				// Assign ranks and determine benchmark hits
				for i := range batchResults {
					batchResults[i].Rank = i + 1
				}

				// Calculate benchmark hits (top 20% by actual gain)
				loader := &BenchDataLoader{}
				loader.calculateBenchmarkHits(batchResults)

				results = append(results, batchResults...)
			}
		}
	}

	return results
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
