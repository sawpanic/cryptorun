package microstructure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/sawpanic/cryptorun/internal/data/venue/types"
)

// ProofGenerator creates validation proof artifacts
type ProofGenerator struct {
	artifactsDir string
}

// NewProofGenerator creates a proof artifact generator
func NewProofGenerator(artifactsDir string) *ProofGenerator {
	return &ProofGenerator{
		artifactsDir: artifactsDir,
	}
}

// GenerateProofBundle creates a comprehensive proof bundle for validation results
func (pg *ProofGenerator) GenerateProofBundle(result *ValidationResult) (*types.ProofBundle, error) {
	if len(result.EligibleVenues) == 0 {
		return pg.generateFailureProof(result)
	}

	// Use the first eligible venue for primary proof
	primaryVenue := result.EligibleVenues[0]
	venueValidation := result.VenueResults[primaryVenue]

	if venueValidation.OrderBook == nil || venueValidation.Metrics == nil {
		return nil, fmt.Errorf("missing orderbook or metrics for venue %s", primaryVenue)
	}

	proofBundle := &types.ProofBundle{
		AssetSymbol:           result.Symbol,
		TimestampMono:         result.TimestampMono,
		ProvenValid:           result.OverallValid,
		OrderBookSnapshot:     venueValidation.OrderBook,
		MicrostructureMetrics: venueValidation.Metrics,
		VenueUsed:             primaryVenue,
		ProofGeneratedAt:      time.Now(),
		ProofID:               pg.generateProofID(result.Symbol),
	}

	// Generate validation proofs for each metric
	proofBundle.SpreadProof = pg.createSpreadProof(venueValidation.Metrics)
	proofBundle.DepthProof = pg.createDepthProof(venueValidation.Metrics)
	proofBundle.VADRProof = pg.createVADRProof(venueValidation.Metrics)

	log.Info().
		Str("symbol", result.Symbol).
		Str("proof_id", proofBundle.ProofID).
		Str("venue_used", primaryVenue).
		Bool("proven_valid", proofBundle.ProvenValid).
		Msg("Generated proof bundle")

	return proofBundle, nil
}

// generateFailureProof creates a proof bundle for failed validation
func (pg *ProofGenerator) generateFailureProof(result *ValidationResult) (*types.ProofBundle, error) {
	// Use the first venue (even if failed) for failure evidence
	var primaryVenue string
	var venueValidation *VenueValidation

	for venue, validation := range result.VenueResults {
		primaryVenue = venue
		venueValidation = validation
		break
	}

	if venueValidation == nil {
		return nil, fmt.Errorf("no venue results available for %s", result.Symbol)
	}

	proofBundle := &types.ProofBundle{
		AssetSymbol:      result.Symbol,
		TimestampMono:    result.TimestampMono,
		ProvenValid:      false,
		VenueUsed:        primaryVenue,
		ProofGeneratedAt: time.Now(),
		ProofID:          pg.generateProofID(result.Symbol),
	}

	// Include orderbook and metrics if available
	if venueValidation.OrderBook != nil {
		proofBundle.OrderBookSnapshot = venueValidation.OrderBook
	}
	if venueValidation.Metrics != nil {
		proofBundle.MicrostructureMetrics = venueValidation.Metrics
		proofBundle.SpreadProof = pg.createSpreadProof(venueValidation.Metrics)
		proofBundle.DepthProof = pg.createDepthProof(venueValidation.Metrics)
		proofBundle.VADRProof = pg.createVADRProof(venueValidation.Metrics)
	}

	return proofBundle, nil
}

// createSpreadProof generates proof for spread validation
func (pg *ProofGenerator) createSpreadProof(metrics *types.MicrostructureMetrics) types.ValidationProof {
	operator := "<"
	evidence := fmt.Sprintf("Spread %.1f bps meets required max 50.0 bps", metrics.SpreadBPS)

	if !metrics.SpreadValid {
		evidence = fmt.Sprintf("Spread %.1f bps exceeds required max 50.0 bps", metrics.SpreadBPS)
	}

	return types.ValidationProof{
		Metric:        "spread_bps",
		ActualValue:   metrics.SpreadBPS,
		RequiredValue: 50.0, // TODO: Make configurable
		Operator:      operator,
		Passed:        metrics.SpreadValid,
		Evidence:      evidence,
	}
}

// createDepthProof generates proof for depth validation
func (pg *ProofGenerator) createDepthProof(metrics *types.MicrostructureMetrics) types.ValidationProof {
	operator := ">="
	evidence := fmt.Sprintf("Depth $%.0f meets required min $100,000 within ±2%%", metrics.DepthUSDPlusMinus2Pct)

	if !metrics.DepthValid {
		evidence = fmt.Sprintf("Depth $%.0f below required min $100,000 within ±2%%", metrics.DepthUSDPlusMinus2Pct)
	}

	return types.ValidationProof{
		Metric:        "depth_usd_plus_minus_2pct",
		ActualValue:   metrics.DepthUSDPlusMinus2Pct,
		RequiredValue: 100000, // TODO: Make configurable
		Operator:      operator,
		Passed:        metrics.DepthValid,
		Evidence:      evidence,
	}
}

// createVADRProof generates proof for VADR validation
func (pg *ProofGenerator) createVADRProof(metrics *types.MicrostructureMetrics) types.ValidationProof {
	operator := ">="
	evidence := fmt.Sprintf("VADR %.2fx meets required min 1.75x", metrics.VADR)

	if !metrics.VADRValid {
		evidence = fmt.Sprintf("VADR %.2fx below required min 1.75x", metrics.VADR)
	}

	return types.ValidationProof{
		Metric:        "vadr",
		ActualValue:   metrics.VADR,
		RequiredValue: 1.75, // TODO: Make configurable
		Operator:      operator,
		Passed:        metrics.VADRValid,
		Evidence:      evidence,
	}
}

// SaveProofBundle writes proof bundle to disk with proper directory structure
func (pg *ProofGenerator) SaveProofBundle(proofBundle *types.ProofBundle) (string, error) {
	// Create date-based directory structure
	dateStr := proofBundle.TimestampMono.Format("2006-01-02")
	proofDir := filepath.Join(pg.artifactsDir, "proofs", dateStr, "microstructure")

	if err := os.MkdirAll(proofDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create proof directory: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("%s_master_proof.json", strings.ToUpper(proofBundle.AssetSymbol))
	filePath := filepath.Join(proofDir, filename)

	// Serialize proof bundle
	data, err := json.MarshalIndent(proofBundle, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal proof bundle: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write proof bundle: %w", err)
	}

	log.Info().
		Str("file_path", filePath).
		Str("symbol", proofBundle.AssetSymbol).
		Str("proof_id", proofBundle.ProofID).
		Int("file_size_bytes", len(data)).
		Msg("Saved proof bundle to disk")

	return filePath, nil
}

// GenerateBatchReport creates a summary report for multiple asset validations
func (pg *ProofGenerator) GenerateBatchReport(results []*ValidationResult) (*BatchReport, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no validation results provided")
	}

	report := &BatchReport{
		GeneratedAt:    time.Now(),
		TotalAssets:    len(results),
		EligibleAssets: 0,
		VenueStats:     make(map[string]*VenueStats),
		AssetSummaries: make([]*AssetSummary, 0, len(results)),
	}

	// Initialize venue stats
	venueNames := []string{"binance", "okx", "coinbase"}
	for _, venue := range venueNames {
		report.VenueStats[venue] = &VenueStats{
			Venue:        venue,
			TotalChecked: 0,
			PassedCount:  0,
			PassRate:     0.0,
			AvgSpreadBPS: 0.0,
			AvgDepthUSD:  0.0,
		}
	}

	// Process each result
	var totalSpreadByVenue = make(map[string]float64)
	var totalDepthByVenue = make(map[string]float64)

	for _, result := range results {
		if result.OverallValid {
			report.EligibleAssets++
		}

		// Create asset summary
		summary := &AssetSummary{
			Symbol:         result.Symbol,
			OverallValid:   result.OverallValid,
			EligibleVenues: result.EligibleVenues,
			FailedVenues:   result.FailedVenues,
		}
		report.AssetSummaries = append(report.AssetSummaries, summary)

		// Update venue stats
		for venue, venueResult := range result.VenueResults {
			stats := report.VenueStats[venue]
			stats.TotalChecked++

			if venueResult.Valid {
				stats.PassedCount++
			}

			if venueResult.Metrics != nil {
				totalSpreadByVenue[venue] += venueResult.Metrics.SpreadBPS
				totalDepthByVenue[venue] += venueResult.Metrics.DepthUSDPlusMinus2Pct
			}
		}
	}

	// Calculate final venue statistics
	for venue, stats := range report.VenueStats {
		if stats.TotalChecked > 0 {
			stats.PassRate = float64(stats.PassedCount) / float64(stats.TotalChecked) * 100
			stats.AvgSpreadBPS = totalSpreadByVenue[venue] / float64(stats.TotalChecked)
			stats.AvgDepthUSD = totalDepthByVenue[venue] / float64(stats.TotalChecked)
		}
	}

	report.EligibilityRate = float64(report.EligibleAssets) / float64(report.TotalAssets) * 100

	return report, nil
}

// SaveBatchReport writes batch report to disk
func (pg *ProofGenerator) SaveBatchReport(report *BatchReport) (string, error) {
	// Create reports directory
	dateStr := report.GeneratedAt.Format("2006-01-02")
	reportsDir := filepath.Join(pg.artifactsDir, "proofs", dateStr, "reports")

	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create reports directory: %w", err)
	}

	// Generate filename with timestamp
	timeStr := report.GeneratedAt.Format("150405")
	filename := fmt.Sprintf("microstructure_audit_%s.json", timeStr)
	filePath := filepath.Join(reportsDir, filename)

	// Serialize report
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal batch report: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write batch report: %w", err)
	}

	log.Info().
		Str("file_path", filePath).
		Int("total_assets", report.TotalAssets).
		Int("eligible_assets", report.EligibleAssets).
		Float64("eligibility_rate", report.EligibilityRate).
		Msg("Saved batch report to disk")

	return filePath, nil
}

// generateProofID creates a unique proof identifier
func (pg *ProofGenerator) generateProofID(symbol string) string {
	timestamp := time.Now().Format("20060102-150405")
	shortUUID := strings.Split(uuid.New().String(), "-")[0]
	return fmt.Sprintf("%s-%s-%s", strings.ToUpper(symbol), timestamp, shortUUID)
}

// BatchReport contains summary statistics for multiple asset validations
type BatchReport struct {
	GeneratedAt     time.Time              `json:"generated_at"`
	TotalAssets     int                    `json:"total_assets"`
	EligibleAssets  int                    `json:"eligible_assets"`
	EligibilityRate float64                `json:"eligibility_rate_pct"`
	VenueStats      map[string]*VenueStats `json:"venue_stats"`
	AssetSummaries  []*AssetSummary        `json:"asset_summaries"`
}

// VenueStats contains aggregated statistics for a venue
type VenueStats struct {
	Venue        string  `json:"venue"`
	TotalChecked int     `json:"total_checked"`
	PassedCount  int     `json:"passed_count"`
	PassRate     float64 `json:"pass_rate_pct"`
	AvgSpreadBPS float64 `json:"avg_spread_bps"`
	AvgDepthUSD  float64 `json:"avg_depth_usd"`
}

// AssetSummary contains summary info for a single asset
type AssetSummary struct {
	Symbol         string   `json:"symbol"`
	OverallValid   bool     `json:"overall_valid"`
	EligibleVenues []string `json:"eligible_venues"`
	FailedVenues   []string `json:"failed_venues"`
}
