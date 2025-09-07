package microstructure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/venue/types"
)

// ProofGenerator handles creation and persistence of microstructure proof bundles
type ProofGenerator struct {
	artifactsDir string
}

// NewProofGenerator creates a proof generator with specified artifacts directory
func NewProofGenerator(artifactsDir string) *ProofGenerator {
	return &ProofGenerator{
		artifactsDir: artifactsDir,
	}
}

// GenerateProofBundle creates and persists a proof bundle for an asset
func (pg *ProofGenerator) GenerateProofBundle(ctx context.Context, result *AssetEligibilityResult) error {
	// Create date-based directory structure
	dateDir := time.Now().Format("2006-01-02")
	proofsDir := filepath.Join(pg.artifactsDir, "proofs", dateDir, "microstructure")

	if err := os.MkdirAll(proofsDir, 0755); err != nil {
		return fmt.Errorf("failed to create proofs directory: %w", err)
	}

	// Generate master proof bundle
	masterBundle := &MasterProofBundle{
		AssetSymbol:     result.Symbol,
		CheckedAt:       result.CheckedAt,
		OverallEligible: result.OverallEligible,
		EligibleVenues:  result.EligibleVenues,
		VenueErrors:     result.VenueErrors,
		VenueProofs:     result.ProofBundles,
		GeneratedAt:     time.Now(),
		ProofVersion:    "1.0",
	}

	// Write master bundle
	masterFile := filepath.Join(proofsDir, fmt.Sprintf("%s_master_proof.json", result.Symbol))
	if err := pg.writeJSONFile(masterBundle, masterFile); err != nil {
		return fmt.Errorf("failed to write master proof bundle: %w", err)
	}

	// Write individual venue proofs
	for venueName, proof := range result.ProofBundles {
		venueFile := filepath.Join(proofsDir, fmt.Sprintf("%s_%s_proof.json", result.Symbol, venueName))
		if err := pg.writeJSONFile(proof, venueFile); err != nil {
			return fmt.Errorf("failed to write %s venue proof: %w", venueName, err)
		}
	}

	// Write venue metrics summary
	metricsFile := filepath.Join(proofsDir, fmt.Sprintf("%s_metrics_summary.json", result.Symbol))
	if err := pg.writeJSONFile(result.VenueResults, metricsFile); err != nil {
		return fmt.Errorf("failed to write metrics summary: %w", err)
	}

	return nil
}

// LoadProofBundle loads a previously generated proof bundle
func (pg *ProofGenerator) LoadProofBundle(ctx context.Context, symbol, date string) (*MasterProofBundle, error) {
	proofsDir := filepath.Join(pg.artifactsDir, "proofs", date, "microstructure")
	masterFile := filepath.Join(proofsDir, fmt.Sprintf("%s_master_proof.json", symbol))

	data, err := os.ReadFile(masterFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read proof bundle: %w", err)
	}

	var bundle MasterProofBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proof bundle: %w", err)
	}

	return &bundle, nil
}

// ListAvailableProofs returns a list of available proof bundles
func (pg *ProofGenerator) ListAvailableProofs(ctx context.Context) ([]ProofSummary, error) {
	proofsBaseDir := filepath.Join(pg.artifactsDir, "proofs")

	var summaries []ProofSummary

	// Walk through date directories
	err := filepath.Walk(proofsBaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for master proof files
		if filepath.Base(path) == "master_proof.json" ||
			(info != nil && !info.IsDir() && filepath.Ext(path) == ".json" &&
				filepath.Base(filepath.Dir(path)) == "microstructure" &&
				strings.Contains(filepath.Base(path), "_master_proof.json")) {

			// Extract date and symbol from path
			parts := filepath.SplitList(path)
			if len(parts) >= 3 {
				date := parts[len(parts)-3]
				fileName := filepath.Base(path)
				symbol := fileName[:len(fileName)-len("_master_proof.json")]

				summaries = append(summaries, ProofSummary{
					Symbol: symbol,
					Date:   date,
					Path:   path,
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list proofs: %w", err)
	}

	return summaries, nil
}

// GenerateAuditReport creates a comprehensive audit report
func (pg *ProofGenerator) GenerateAuditReport(ctx context.Context, results []*AssetEligibilityResult) (*AuditReport, error) {
	report := &AuditReport{
		GeneratedAt:      time.Now(),
		TotalAssets:      len(results),
		EligibleAssets:   []string{},
		IneligibleAssets: []string{},
		VenueStats:       make(map[string]*VenueAuditStats),
		Summary:          &AuditSummary{},
	}

	// Initialize venue stats
	for _, result := range results {
		for venueName := range result.VenueResults {
			if _, exists := report.VenueStats[venueName]; !exists {
				report.VenueStats[venueName] = &VenueAuditStats{
					VenueName:     venueName,
					TotalChecked:  0,
					PassedChecks:  0,
					FailedChecks:  0,
					AverageSpread: 0,
					AverageDepth:  0,
				}
			}
		}
	}

	// Process results
	for _, result := range results {
		if result.OverallEligible {
			report.EligibleAssets = append(report.EligibleAssets, result.Symbol)
			report.Summary.EligibleCount++
		} else {
			report.IneligibleAssets = append(report.IneligibleAssets, result.Symbol)
			report.Summary.IneligibleCount++
		}

		// Update venue statistics
		for venueName, metrics := range result.VenueResults {
			stats := report.VenueStats[venueName]
			stats.TotalChecked++

			if metrics.OverallValid {
				stats.PassedChecks++
			} else {
				stats.FailedChecks++
			}

			// Running averages (simplified)
			stats.AverageSpread = (stats.AverageSpread*float64(stats.TotalChecked-1) + metrics.SpreadBPS) / float64(stats.TotalChecked)
			stats.AverageDepth = (stats.AverageDepth*float64(stats.TotalChecked-1) + metrics.DepthUSDPlusMinus2Pct) / float64(stats.TotalChecked)
		}
	}

	// Calculate summary statistics
	if report.TotalAssets > 0 {
		report.Summary.EligibilityRate = float64(report.Summary.EligibleCount) / float64(report.TotalAssets) * 100
	}

	// Write audit report
	dateDir := time.Now().Format("2006-01-02")
	reportsDir := filepath.Join(pg.artifactsDir, "proofs", dateDir, "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create reports directory: %w", err)
	}

	reportFile := filepath.Join(reportsDir, fmt.Sprintf("microstructure_audit_%s.json", time.Now().Format("150405")))
	if err := pg.writeJSONFile(report, reportFile); err != nil {
		return nil, fmt.Errorf("failed to write audit report: %w", err)
	}

	return report, nil
}

// writeJSONFile writes an object as JSON to a file
func (pg *ProofGenerator) writeJSONFile(obj interface{}, filePath string) error {
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// MasterProofBundle contains all proof data for an asset
type MasterProofBundle struct {
	AssetSymbol     string                        `json:"asset_symbol"`
	CheckedAt       time.Time                     `json:"checked_at"`
	OverallEligible bool                          `json:"overall_eligible"`
	EligibleVenues  []string                      `json:"eligible_venues"`
	VenueErrors     []string                      `json:"venue_errors"`
	VenueProofs     map[string]*types.ProofBundle `json:"venue_proofs"`
	GeneratedAt     time.Time                     `json:"generated_at"`
	ProofVersion    string                        `json:"proof_version"`
}

// ProofSummary provides basic information about available proofs
type ProofSummary struct {
	Symbol string `json:"symbol"`
	Date   string `json:"date"`
	Path   string `json:"path"`
}

// AuditReport provides comprehensive audit statistics
type AuditReport struct {
	GeneratedAt      time.Time                   `json:"generated_at"`
	TotalAssets      int                         `json:"total_assets"`
	EligibleAssets   []string                    `json:"eligible_assets"`
	IneligibleAssets []string                    `json:"ineligible_assets"`
	VenueStats       map[string]*VenueAuditStats `json:"venue_stats"`
	Summary          *AuditSummary               `json:"summary"`
}

// VenueAuditStats contains statistics for a specific venue
type VenueAuditStats struct {
	VenueName     string  `json:"venue_name"`
	TotalChecked  int     `json:"total_checked"`
	PassedChecks  int     `json:"passed_checks"`
	FailedChecks  int     `json:"failed_checks"`
	AverageSpread float64 `json:"average_spread_bps"`
	AverageDepth  float64 `json:"average_depth_usd"`
}

// AuditSummary provides high-level audit statistics
type AuditSummary struct {
	EligibleCount   int     `json:"eligible_count"`
	IneligibleCount int     `json:"ineligible_count"`
	EligibilityRate float64 `json:"eligibility_rate_percent"`
}
