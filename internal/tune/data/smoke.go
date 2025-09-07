package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SmokeResult represents a single smoke90 backtest result
type SmokeResult struct {
	Symbol        string    `json:"symbol"`
	Timestamp     time.Time `json:"timestamp"`
	Score         float64   `json:"score"`
	Regime        string    `json:"regime"`
	ForwardReturn float64   `json:"forward_return"`
	Hit           bool      `json:"hit"`    // True if forward return met threshold
	Window        string    `json:"window"` // 1h, 4h, 12h, 24h
	EntryPrice    float64   `json:"entry_price"`
	ExitPrice     float64   `json:"exit_price"`
}

// SmokeDataLoader loads and filters smoke90 backtest results
type SmokeDataLoader struct {
	artifactDir string
}

// NewSmokeDataLoader creates a new smoke data loader
func NewSmokeDataLoader(artifactDir string) *SmokeDataLoader {
	if artifactDir == "" {
		artifactDir = "./artifacts/smoke90"
	}
	return &SmokeDataLoader{
		artifactDir: artifactDir,
	}
}

// LoadResults loads smoke90 results filtered by regime and windows
func (sdl *SmokeDataLoader) LoadResults(regimes []string, windows []string) ([]SmokeResult, error) {
	var allResults []SmokeResult

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

	// Scan artifact directory for smoke90 results
	err := filepath.Walk(sdl.artifactDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(info.Name(), "_smoke90.json") {
			return nil
		}

		results, err := sdl.loadResultFile(path)
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
		return nil, fmt.Errorf("failed to walk artifact directory: %w", err)
	}

	// Sort results by timestamp for consistent ordering
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Timestamp.Before(allResults[j].Timestamp)
	})

	return allResults, nil
}

// loadResultFile loads a single smoke90 result file
func (sdl *SmokeDataLoader) loadResultFile(filePath string) ([]SmokeResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var results []SmokeResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}

	// Calculate hit status based on forward returns
	for i := range results {
		results[i].Hit = sdl.calculateHit(&results[i])
	}

	return results, nil
}

// calculateHit determines if a result is a "hit" based on forward return thresholds
func (sdl *SmokeDataLoader) calculateHit(result *SmokeResult) bool {
	return sdl.CalculateHitPublic(result)
}

// CalculateHitPublic exposes hit calculation for testing
func (sdl *SmokeDataLoader) CalculateHitPublic(result *SmokeResult) bool {
	// Define hit thresholds by window (example thresholds)
	thresholds := map[string]float64{
		"1h":  0.015, // 1.5% for 1-hour window
		"4h":  0.025, // 2.5% for 4-hour window
		"12h": 0.035, // 3.5% for 12-hour window
		"24h": 0.045, // 4.5% for 24-hour window
	}

	threshold, exists := thresholds[result.Window]
	if !exists {
		threshold = 0.025 // Default 2.5% threshold
	}

	return result.ForwardReturn >= threshold
}

// GetMetricsByRegime calculates performance metrics grouped by regime
func (sdl *SmokeDataLoader) GetMetricsByRegime(results []SmokeResult) map[string]RegimeMetrics {
	regimeGroups := make(map[string][]SmokeResult)

	// Group results by regime
	for _, result := range results {
		regimeGroups[result.Regime] = append(regimeGroups[result.Regime], result)
	}

	// Calculate metrics for each regime
	metrics := make(map[string]RegimeMetrics)
	for regime, regimeResults := range regimeGroups {
		metrics[regime] = sdl.calculateMetrics(regimeResults)
	}

	return metrics
}

// RegimeMetrics holds performance metrics for a regime
type RegimeMetrics struct {
	Regime       string     `json:"regime"`
	TotalSignals int        `json:"total_signals"`
	Hits         int        `json:"hits"`
	HitRate      float64    `json:"hit_rate"`
	AvgReturn    float64    `json:"avg_return"`
	SpearmanCorr float64    `json:"spearman_correlation"`
	ScoreBounds  [2]float64 `json:"score_bounds"`  // [min, max]
	ReturnBounds [2]float64 `json:"return_bounds"` // [min, max]
}

// calculateMetrics computes performance metrics for a set of results
func (sdl *SmokeDataLoader) calculateMetrics(results []SmokeResult) RegimeMetrics {
	return sdl.CalculateMetricsPublic(results)
}

// CalculateMetricsPublic exposes metrics calculation for testing
func (sdl *SmokeDataLoader) CalculateMetricsPublic(results []SmokeResult) RegimeMetrics {
	if len(results) == 0 {
		return RegimeMetrics{}
	}

	var hits int
	var totalReturn float64
	var minScore, maxScore = results[0].Score, results[0].Score
	var minReturn, maxReturn = results[0].ForwardReturn, results[0].ForwardReturn

	scores := make([]float64, len(results))
	returns := make([]float64, len(results))

	for i, result := range results {
		if result.Hit {
			hits++
		}
		totalReturn += result.ForwardReturn

		scores[i] = result.Score
		returns[i] = result.ForwardReturn

		if result.Score < minScore {
			minScore = result.Score
		}
		if result.Score > maxScore {
			maxScore = result.Score
		}
		if result.ForwardReturn < minReturn {
			minReturn = result.ForwardReturn
		}
		if result.ForwardReturn > maxReturn {
			maxReturn = result.ForwardReturn
		}
	}

	hitRate := float64(hits) / float64(len(results))
	avgReturn := totalReturn / float64(len(results))
	spearmanCorr := calculateSpearmanCorrelation(scores, returns)

	return RegimeMetrics{
		Regime:       results[0].Regime,
		TotalSignals: len(results),
		Hits:         hits,
		HitRate:      hitRate,
		AvgReturn:    avgReturn,
		SpearmanCorr: spearmanCorr,
		ScoreBounds:  [2]float64{minScore, maxScore},
		ReturnBounds: [2]float64{minReturn, maxReturn},
	}
}

// calculateSpearmanCorrelation computes Spearman rank correlation
func calculateSpearmanCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0.0
	}

	n := len(x)

	// Create rank arrays
	xRanks := getRanks(x)
	yRanks := getRanks(y)

	// Calculate differences and sum of squared differences
	var sumDiff2 float64
	for i := 0; i < n; i++ {
		diff := xRanks[i] - yRanks[i]
		sumDiff2 += diff * diff
	}

	// Spearman correlation formula
	spearman := 1.0 - (6.0*sumDiff2)/float64(n*(n*n-1))

	return spearman
}

// getRanks converts values to ranks (1-based)
func getRanks(values []float64) []float64 {
	n := len(values)

	// Create index-value pairs for sorting
	type IndexValue struct {
		Index int
		Value float64
	}

	pairs := make([]IndexValue, n)
	for i, v := range values {
		pairs[i] = IndexValue{Index: i, Value: v}
	}

	// Sort by value
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Value < pairs[j].Value
	})

	// Assign ranks
	ranks := make([]float64, n)
	for rank, pair := range pairs {
		ranks[pair.Index] = float64(rank + 1) // 1-based ranks
	}

	return ranks
}

// GetAvailableRegimes scans artifacts and returns available regime labels
func (sdl *SmokeDataLoader) GetAvailableRegimes() ([]string, error) {
	regimeSet := make(map[string]bool)

	err := filepath.Walk(sdl.artifactDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(info.Name(), "_smoke90.json") {
			return nil
		}

		results, err := sdl.loadResultFile(path)
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

// CreateMockResults creates mock smoke90 results for testing
func CreateMockResults(regimes []string, windows []string, count int) []SmokeResult {
	var results []SmokeResult
	baseTime := time.Now().Add(-24 * time.Hour)

	for i := 0; i < count; i++ {
		for _, regime := range regimes {
			for _, window := range windows {
				// Create deterministic but varied mock data
				seed := float64(i*100 + hashString(regime)*10 + hashString(window))
				score := 70.0 + (seed * 0.1237) // Pseudo-random score 70-100
				if score > 100 {
					score = 100
				}

				// Forward return correlated with score but with noise
				forwardReturn := (score-70)/1000 + (seed * 0.0032) - 0.015 // Some correlation + noise

				result := SmokeResult{
					Symbol:        fmt.Sprintf("ASSET%d", i%10),
					Timestamp:     baseTime.Add(time.Duration(i) * time.Hour),
					Score:         score,
					Regime:        regime,
					ForwardReturn: forwardReturn,
					Window:        window,
					EntryPrice:    50000.0 + seed*100,
					ExitPrice:     0, // Will be calculated
				}

				result.ExitPrice = result.EntryPrice * (1.0 + result.ForwardReturn)
				result.Hit = result.ForwardReturn >= 0.025 // 2.5% threshold

				results = append(results, result)
			}
		}
	}

	return results
}

// hashString creates a simple hash of a string for deterministic pseudo-random values
func hashString(s string) int {
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash % 1000
}
