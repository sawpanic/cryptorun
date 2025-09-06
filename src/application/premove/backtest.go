// Package premove provides point-in-time replay and isotonic calibration for the pre-movement detector.
package premove

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PITRecord represents a point-in-time record from artifacts/premove/*.jsonl
type PITRecord struct {
	Symbol      string                 `json:"symbol"`
	Timestamp   time.Time              `json:"ts"`
	Score       float64                `json:"score"`
	State       string                 `json:"state"`
	SubScores   map[string]float64     `json:"sub_scores"`
	PassedGates int                    `json:"passed_gates"`
	Penalties   map[string]float64     `json:"penalties"`
	TopReasons  []string               `json:"top_reasons"`
	Sources     map[string]interface{} `json:"sources"`
	Regime      string                 `json:"regime,omitempty"`
	ActualMove  *float64               `json:"actual_move_48h,omitempty"` // Added for backtest validation
}

// HitRate represents hit rate statistics by state and regime
type HitRate struct {
	State          string     `json:"state"`
	Regime         string     `json:"regime"`
	TotalSignals   int        `json:"total_signals"`
	SuccessfulHits int        `json:"successful_hits"`
	HitRate        float64    `json:"hit_rate"`
	ConfidenceCI   [2]float64 `json:"confidence_95_ci"`
}

// IsotonicPoint represents a point on the isotonic calibration curve
type IsotonicPoint struct {
	Score       float64 `json:"score"`
	Probability float64 `json:"probability"`
	SampleCount int     `json:"sample_count"`
}

// CalibrationCurve represents the full isotonic calibration mapping
type CalibrationCurve struct {
	Points      []IsotonicPoint `json:"points"`
	GeneratedAt time.Time       `json:"generated_at"`
	MonthWindow string          `json:"month_window"`
	FrozenUntil time.Time       `json:"frozen_until"`
	R2Score     float64         `json:"r2_score"`
}

// CVDResidualFit represents daily CVD residual R² tracking
type CVDResidualFit struct {
	Date    string  `json:"date"`
	Symbol  string  `json:"symbol"`
	R2Score float64 `json:"r2_score"`
	Samples int     `json:"samples"`
}

// BacktestHarness orchestrates PIT replay and calibration
type BacktestHarness struct {
	artifactsDir   string
	outputDir      string
	movementThresh float64 // 5% movement threshold
	windowHours    int     // 48h window
}

// NewBacktestHarness creates a new backtest harness
func NewBacktestHarness(artifactsDir, outputDir string) *BacktestHarness {
	return &BacktestHarness{
		artifactsDir:   artifactsDir,
		outputDir:      outputDir,
		movementThresh: 0.05, // 5%
		windowHours:    48,
	}
}

// RunBacktest executes the full backtest pipeline
func (b *BacktestHarness) RunBacktest() error {
	log.Println("Starting PIT replay backtest...")

	// Load all PIT records from artifacts
	records, err := b.loadPITRecords()
	if err != nil {
		return fmt.Errorf("failed to load PIT records: %w", err)
	}

	if len(records) == 0 {
		log.Println("No PIT records found - creating empty outputs")
		return b.createEmptyOutputs()
	}

	log.Printf("Loaded %d PIT records for analysis", len(records))

	// Compute hit rates by state and regime
	hitRates, err := b.computeHitRates(records)
	if err != nil {
		return fmt.Errorf("failed to compute hit rates: %w", err)
	}

	// Fit isotonic calibration curve
	curve, err := b.fitIsotonicCalibration(records)
	if err != nil {
		return fmt.Errorf("failed to fit isotonic calibration: %w", err)
	}

	// Track daily CVD residual R² scores
	cvdFits, err := b.trackCVDResidualFits(records)
	if err != nil {
		return fmt.Errorf("failed to track CVD residual fits: %w", err)
	}

	// Write outputs
	if err := b.writeHitRates(hitRates); err != nil {
		return fmt.Errorf("failed to write hit rates: %w", err)
	}

	if err := b.writeCalibrationCurve(curve); err != nil {
		return fmt.Errorf("failed to write calibration curve: %w", err)
	}

	if err := b.writeCVDResidualFits(cvdFits); err != nil {
		return fmt.Errorf("failed to write CVD residual fits: %w", err)
	}

	log.Println("Backtest completed successfully")
	return nil
}

// loadPITRecords loads all PIT records from artifacts directory
func (b *BacktestHarness) loadPITRecords() ([]PITRecord, error) {
	var records []PITRecord

	pattern := filepath.Join(b.artifactsDir, "premove", "*.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob artifacts: %w", err)
	}

	for _, file := range files {
		fileRecords, err := b.loadJSONLFile(file)
		if err != nil {
			log.Printf("Warning: failed to load %s: %v", file, err)
			continue
		}
		records = append(records, fileRecords...)
	}

	// Sort by timestamp for chronological processing
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	return records, nil
}

// loadJSONLFile loads records from a single JSONL file
func (b *BacktestHarness) loadJSONLFile(filename string) ([]PITRecord, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var records []PITRecord
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record PITRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			log.Printf("Warning: failed to parse line in %s: %v", filename, err)
			continue
		}

		// Set default regime if missing
		if record.Regime == "" {
			record.Regime = "unknown"
		}

		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan file: %w", err)
	}

	return records, nil
}

// computeHitRates calculates hit rates by state and regime
func (b *BacktestHarness) computeHitRates(records []PITRecord) ([]HitRate, error) {
	// Group records by state and regime
	groups := make(map[string]map[string][]PITRecord)

	for _, record := range records {
		state := b.normalizeState(record.State)
		regime := record.Regime

		if groups[state] == nil {
			groups[state] = make(map[string][]PITRecord)
		}
		groups[state][regime] = append(groups[state][regime], record)
	}

	var hitRates []HitRate

	for state, regimeGroups := range groups {
		for regime, stateRecords := range regimeGroups {
			if len(stateRecords) == 0 {
				continue
			}

			successCount := 0
			validRecords := 0

			for _, record := range stateRecords {
				// Only count records with actual movement data
				if record.ActualMove != nil {
					validRecords++
					if math.Abs(*record.ActualMove) >= b.movementThresh {
						successCount++
					}
				}
			}

			if validRecords == 0 {
				continue
			}

			rate := float64(successCount) / float64(validRecords)
			ci := b.calculateBinomialCI(successCount, validRecords, 0.95)

			hitRates = append(hitRates, HitRate{
				State:          state,
				Regime:         regime,
				TotalSignals:   validRecords,
				SuccessfulHits: successCount,
				HitRate:        rate,
				ConfidenceCI:   ci,
			})
		}
	}

	return hitRates, nil
}

// fitIsotonicCalibration fits an isotonic regression curve
func (b *BacktestHarness) fitIsotonicCalibration(records []PITRecord) (*CalibrationCurve, error) {
	// Filter records with actual movement data
	var validRecords []PITRecord
	for _, record := range records {
		if record.ActualMove != nil {
			validRecords = append(validRecords, record)
		}
	}

	if len(validRecords) < 10 {
		// Return stub calibration for insufficient data
		return b.createStubCalibration(), nil
	}

	// Sort by score for isotonic regression
	sort.Slice(validRecords, func(i, j int) bool {
		return validRecords[i].Score < validRecords[j].Score
	})

	// Create score bins (every 10 points)
	binSize := 10.0
	var points []IsotonicPoint

	currentBin := math.Floor(validRecords[0].Score/binSize) * binSize
	var binRecords []PITRecord

	for i, record := range validRecords {
		expectedBin := math.Floor(record.Score/binSize) * binSize

		if expectedBin != currentBin || i == len(validRecords)-1 {
			// Process current bin
			if len(binRecords) > 0 {
				point := b.createIsotonicPoint(currentBin+binSize/2, binRecords)
				points = append(points, point)
			}

			// Start new bin
			currentBin = expectedBin
			binRecords = []PITRecord{record}
		} else {
			binRecords = append(binRecords, record)
		}
	}

	// Ensure monotonicity (isotonic property)
	points = b.enforceMonotonicity(points)

	// Calculate R² score
	r2 := b.calculateR2Score(validRecords, points)

	now := time.Now()
	monthWindow := now.Format("2006-01")
	frozenUntil := now.AddDate(0, 1, 0) // Frozen for 1 month

	return &CalibrationCurve{
		Points:      points,
		GeneratedAt: now,
		MonthWindow: monthWindow,
		FrozenUntil: frozenUntil,
		R2Score:     r2,
	}, nil
}

// trackCVDResidualFits tracks daily CVD residual R² scores
func (b *BacktestHarness) trackCVDResidualFits(records []PITRecord) ([]CVDResidualFit, error) {
	// Group by date and symbol
	daily := make(map[string]map[string][]PITRecord)

	for _, record := range records {
		date := record.Timestamp.Format("2006-01-02")
		if daily[date] == nil {
			daily[date] = make(map[string][]PITRecord)
		}
		daily[date][record.Symbol] = append(daily[date][record.Symbol], record)
	}

	var fits []CVDResidualFit

	for date, symbols := range daily {
		for symbol, dayRecords := range symbols {
			if len(dayRecords) < 3 {
				continue // Need minimum samples for R²
			}

			// Extract CVD residual data from sub_scores
			var cvdResiduals, prices []float64
			for _, record := range dayRecords {
				if cvdResid, exists := record.SubScores["cvd_residual"]; exists {
					if price, priceExists := record.SubScores["price"]; priceExists {
						cvdResiduals = append(cvdResiduals, cvdResid)
						prices = append(prices, price)
					}
				}
			}

			if len(cvdResiduals) < 3 {
				continue
			}

			r2 := b.calculateLinearR2(cvdResiduals, prices)

			fits = append(fits, CVDResidualFit{
				Date:    date,
				Symbol:  symbol,
				R2Score: r2,
				Samples: len(cvdResiduals),
			})
		}
	}

	// Sort by date for output
	sort.Slice(fits, func(i, j int) bool {
		return fits[i].Date < fits[j].Date
	})

	return fits, nil
}

// Helper functions

func (b *BacktestHarness) normalizeState(state string) string {
	state = strings.ToUpper(strings.TrimSpace(state))
	switch {
	case strings.Contains(state, "QUIET") || state == "":
		return "QUIET"
	case strings.Contains(state, "WATCH"):
		return "WATCH"
	case strings.Contains(state, "PREPARE"):
		return "PREPARE"
	case strings.Contains(state, "PRIME"):
		return "PRIME"
	case strings.Contains(state, "EXECUTE"):
		return "EXECUTE"
	default:
		return "UNKNOWN"
	}
}

func (b *BacktestHarness) calculateBinomialCI(successes, trials int, confidence float64) [2]float64 {
	if trials == 0 {
		return [2]float64{0, 0}
	}

	p := float64(successes) / float64(trials)
	z := 1.96 // 95% confidence

	if confidence != 0.95 {
		// Could extend for other confidence levels
		z = 1.96
	}

	margin := z * math.Sqrt(p*(1-p)/float64(trials))
	lower := math.Max(0, p-margin)
	upper := math.Min(1, p+margin)

	return [2]float64{lower, upper}
}

func (b *BacktestHarness) createIsotonicPoint(score float64, records []PITRecord) IsotonicPoint {
	successCount := 0
	for _, record := range records {
		if record.ActualMove != nil && math.Abs(*record.ActualMove) >= b.movementThresh {
			successCount++
		}
	}

	probability := float64(successCount) / float64(len(records))

	return IsotonicPoint{
		Score:       score,
		Probability: probability,
		SampleCount: len(records),
	}
}

func (b *BacktestHarness) enforceMonotonicity(points []IsotonicPoint) []IsotonicPoint {
	if len(points) <= 1 {
		return points
	}

	// Simple isotonic regression using pool-adjacent-violators algorithm
	for i := 1; i < len(points); i++ {
		if points[i].Probability < points[i-1].Probability {
			// Average the violating points
			totalSamples := points[i-1].SampleCount + points[i].SampleCount
			avgProb := (points[i-1].Probability*float64(points[i-1].SampleCount) +
				points[i].Probability*float64(points[i].SampleCount)) / float64(totalSamples)

			points[i-1].Probability = avgProb
			points[i].Probability = avgProb

			// Propagate backwards if needed
			for j := i - 1; j > 0; j-- {
				if points[j].Probability < points[j-1].Probability {
					totalSamples := points[j-1].SampleCount + points[j].SampleCount
					avgProb := (points[j-1].Probability*float64(points[j-1].SampleCount) +
						points[j].Probability*float64(points[j].SampleCount)) / float64(totalSamples)
					points[j-1].Probability = avgProb
					points[j].Probability = avgProb
				} else {
					break
				}
			}
		}
	}

	return points
}

func (b *BacktestHarness) calculateR2Score(records []PITRecord, points []IsotonicPoint) float64 {
	if len(points) == 0 {
		return 0.0
	}

	// Calculate predicted vs actual for R²
	var actualMovements []float64
	var predictedProbs []float64

	for _, record := range records {
		if record.ActualMove == nil {
			continue
		}

		// Find corresponding probability from calibration curve
		prob := b.interpolateProb(record.Score, points)
		actualBinary := 0.0
		if math.Abs(*record.ActualMove) >= b.movementThresh {
			actualBinary = 1.0
		}

		actualMovements = append(actualMovements, actualBinary)
		predictedProbs = append(predictedProbs, prob)
	}

	return b.calculateLinearR2(predictedProbs, actualMovements)
}

func (b *BacktestHarness) interpolateProb(score float64, points []IsotonicPoint) float64 {
	if len(points) == 0 {
		return 0.5 // Default probability
	}

	if score <= points[0].Score {
		return points[0].Probability
	}

	if score >= points[len(points)-1].Score {
		return points[len(points)-1].Probability
	}

	// Linear interpolation
	for i := 1; i < len(points); i++ {
		if score <= points[i].Score {
			t := (score - points[i-1].Score) / (points[i].Score - points[i-1].Score)
			return points[i-1].Probability + t*(points[i].Probability-points[i-1].Probability)
		}
	}

	return points[len(points)-1].Probability
}

func (b *BacktestHarness) calculateLinearR2(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0.0
	}

	// Calculate means
	var sumX, sumY float64
	n := float64(len(x))
	for i := range x {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / n
	meanY := sumY / n

	// Calculate sums of squares
	var ssXY, ssX, ssY float64
	for i := range x {
		dx := x[i] - meanX
		dy := y[i] - meanY
		ssXY += dx * dy
		ssX += dx * dx
		ssY += dy * dy
	}

	if ssX == 0 || ssY == 0 {
		return 0.0
	}

	r := ssXY / math.Sqrt(ssX*ssY)
	return r * r
}

func (b *BacktestHarness) createStubCalibration() *CalibrationCurve {
	now := time.Now()

	// Create linear stub mapping
	points := []IsotonicPoint{
		{Score: 0, Probability: 0.1, SampleCount: 1},
		{Score: 25, Probability: 0.2, SampleCount: 1},
		{Score: 50, Probability: 0.3, SampleCount: 1},
		{Score: 75, Probability: 0.5, SampleCount: 1},
		{Score: 100, Probability: 0.7, SampleCount: 1},
		{Score: 125, Probability: 0.85, SampleCount: 1},
		{Score: 150, Probability: 0.95, SampleCount: 1},
	}

	return &CalibrationCurve{
		Points:      points,
		GeneratedAt: now,
		MonthWindow: now.Format("2006-01") + "_stub",
		FrozenUntil: now.AddDate(0, 1, 0),
		R2Score:     0.0,
	}
}

// Output writers

func (b *BacktestHarness) writeHitRates(hitRates []HitRate) error {
	if err := os.MkdirAll(b.outputDir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(b.outputDir, "hit_rates_by_state_and_regime.json")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(hitRates)
}

func (b *BacktestHarness) writeCalibrationCurve(curve *CalibrationCurve) error {
	filename := filepath.Join(b.outputDir, "isotonic_calibration_curve.json")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(curve)
}

func (b *BacktestHarness) writeCVDResidualFits(fits []CVDResidualFit) error {
	filename := filepath.Join(b.outputDir, "cvd_resid_r2_daily.csv")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write CSV header
	fmt.Fprintln(file, "date,symbol,r2_score,samples")

	for _, fit := range fits {
		fmt.Fprintf(file, "%s,%s,%.6f,%d\n", fit.Date, fit.Symbol, fit.R2Score, fit.Samples)
	}

	return nil
}

func (b *BacktestHarness) createEmptyOutputs() error {
	if err := os.MkdirAll(b.outputDir, 0755); err != nil {
		return err
	}

	// Empty hit rates
	if err := b.writeHitRates([]HitRate{}); err != nil {
		return err
	}

	// Stub calibration curve
	curve := b.createStubCalibration()
	if err := b.writeCalibrationCurve(curve); err != nil {
		return err
	}

	// Empty CVD fits
	if err := b.writeCVDResidualFits([]CVDResidualFit{}); err != nil {
		return err
	}

	return nil
}

// RunBacktestInternal provides CLI-free invocation for testing
func RunBacktestInternal(artifactsDir, outputDir string) error {
	harness := NewBacktestHarness(artifactsDir, outputDir)
	return harness.RunBacktest()
}
