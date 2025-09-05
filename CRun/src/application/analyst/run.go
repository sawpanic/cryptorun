package analyst

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// AnalystRunner performs coverage analysis and quality enforcement
type AnalystRunner struct {
	fetcher      *KrakenWinnersFetcher
	outputDir    string
	candidatesFile string
	policy       QualityPolicy
}

// NewAnalystRunner creates a new analyst runner
func NewAnalystRunner(outputDir, candidatesFile string) *AnalystRunner {
	return &AnalystRunner{
		fetcher:        NewKrakenWinnersFetcher(),
		outputDir:      outputDir,
		candidatesFile: candidatesFile,
		policy:         loadQualityPolicy(),
	}
}

// RunCoverageAnalysis performs complete coverage analysis
func (ar *AnalystRunner) RunCoverageAnalysis(ctx context.Context) (*AnalystReport, error) {
	log.Info().Msg("Starting analyst coverage analysis")
	
	// Step 1: Fetch winners
	winners, err := ar.fetcher.FetchWinners(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch winners: %w", err)
	}
	
	log.Info().Str("source", winners.Source).
		Int("winners_1h", len(winners.Winners1h)).
		Int("winners_24h", len(winners.Winners24h)).
		Int("winners_7d", len(winners.Winners7d)).
		Msg("Winners fetched successfully")
	
	// Step 2: Load candidates
	candidates, err := ar.loadCandidates()
	if err != nil {
		return nil, fmt.Errorf("failed to load candidates: %w", err)
	}
	
	log.Info().Int("candidates", len(candidates)).Msg("Candidates loaded")
	
	// Step 3: Compute coverage for each timeframe
	coverage1h := ar.computeCoverage(winners.Winners1h, candidates, "1h")
	coverage24h := ar.computeCoverage(winners.Winners24h, candidates, "24h")
	coverage7d := ar.computeCoverage(winners.Winners7d, candidates, "7d")
	
	// Step 4: Analyze misses
	misses1h := ar.analyzeMisses(winners.Winners1h, candidates, "1h")
	misses24h := ar.analyzeMisses(winners.Winners24h, candidates, "24h")
	misses7d := ar.analyzeMisses(winners.Winners7d, candidates, "7d")
	
	allMisses := append(append(misses1h, misses24h...), misses7d...)
	
	// Step 5: Create report
	report := &AnalystReport{
		RunTime:     time.Now().UTC(),
		Coverage1h:  coverage1h,
		Coverage24h: coverage24h,
		Coverage7d:  coverage7d,
		TopMisses:   ar.selectTopMisses(allMisses, 10),
		PolicyPass:  ar.checkPolicyCompliance(coverage1h, coverage24h, coverage7d),
	}
	
	report.Summary = ar.generateSummary(report)
	
	// Step 6: Write outputs
	if err := ar.writeOutputs(winners, allMisses, report); err != nil {
		return nil, fmt.Errorf("failed to write outputs: %w", err)
	}
	
	return report, nil
}

// loadCandidates loads candidates from the latest scan
func (ar *AnalystRunner) loadCandidates() ([]CandidateResult, error) {
	if _, err := os.Stat(ar.candidatesFile); os.IsNotExist(err) {
		return []CandidateResult{}, nil // No candidates file is acceptable
	}
	
	data, err := os.ReadFile(ar.candidatesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read candidates file: %w", err)
	}
	
	// Parse JSONL format
	lines := strings.Split(string(data), "\n")
	var candidates []CandidateResult
	
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		var candidate CandidateResult
		if err := json.Unmarshal([]byte(line), &candidate); err != nil {
			log.Warn().Int("line", i+1).Err(err).Msg("Failed to parse candidate line, skipping")
			continue
		}
		
		candidates = append(candidates, candidate)
	}
	
	return candidates, nil
}

// computeCoverage calculates coverage metrics for a timeframe
func (ar *AnalystRunner) computeCoverage(winners []Winner, candidates []CandidateResult, timeframe string) Coverage {
	candidateMap := make(map[string]CandidateResult)
	for _, c := range candidates {
		candidateMap[c.Symbol] = c
	}
	
	hits := 0
	staleCount := 0
	
	for _, winner := range winners {
		if candidate, found := candidateMap[winner.Symbol]; found {
			// Check if candidate passed gates
			if candidate.Decision == "PASS" {
				hits++
			}
			
			// Check for stale data (older than 5 minutes)
			if time.Since(candidate.Meta.Timestamp) > 5*time.Minute {
				staleCount++
			}
		}
	}
	
	misses := len(winners) - hits
	totalWinners := len(winners)
	candidatesFound := len(candidates)
	
	// Calculate metrics
	recallAt20 := 0.0
	if totalWinners > 0 {
		recallAt20 = float64(hits) / float64(totalWinners)
	}
	
	goodFilterRate := 0.0
	if candidatesFound > 0 {
		goodFilterRate = float64(hits) / float64(candidatesFound)
	}
	
	badMissRate := 0.0
	if totalWinners > 0 {
		badMissRate = float64(misses) / float64(totalWinners)
	}
	
	staleDataRate := 0.0
	if totalWinners > 0 {
		staleDataRate = float64(staleCount) / float64(totalWinners)
	}
	
	// Check policy compliance
	policyThreshold := ar.policy.BadMissRateThresholds[timeframe]
	thresholdBreach := badMissRate > policyThreshold
	
	return Coverage{
		TimeFrame:       timeframe,
		TotalWinners:    totalWinners,
		CandidatesFound: candidatesFound,
		Hits:            hits,
		Misses:          misses,
		RecallAt20:      recallAt20,
		GoodFilterRate:  goodFilterRate,
		BadMissRate:     badMissRate,
		StaleDataRate:   staleDataRate,
		ThresholdBreach: thresholdBreach,
		PolicyThreshold: policyThreshold,
	}
}

// analyzeMisses identifies why winners were missed
func (ar *AnalystRunner) analyzeMisses(winners []Winner, candidates []CandidateResult, timeframe string) []Miss {
	candidateMap := make(map[string]CandidateResult)
	for _, c := range candidates {
		candidateMap[c.Symbol] = c
	}
	
	var misses []Miss
	
	for _, winner := range winners {
		candidate, wasCandidate := candidateMap[winner.Symbol]
		
		if !wasCandidate {
			// Not even a candidate
			misses = append(misses, Miss{
				Symbol:       winner.Symbol,
				TimeFrame:    timeframe,
				Performance:  winner.Performance,
				ReasonCode:   ReasonNotCandidate,
				Evidence:     map[string]interface{}{"detail": "symbol not in candidate list"},
				WasCandidate: false,
				Timestamp:    time.Now().UTC(),
			})
			continue
		}
		
		if candidate.Decision == "PASS" {
			// This is a hit, not a miss
			continue
		}
		
		// Analyze why the candidate failed
		reasonCode, evidence := ar.analyzeFailureReason(candidate)
		
		misses = append(misses, Miss{
			Symbol:       winner.Symbol,
			TimeFrame:    timeframe,
			Performance:  winner.Performance,
			ReasonCode:   reasonCode,
			Evidence:     evidence,
			WasCandidate: true,
			Timestamp:    time.Now().UTC(),
		})
	}
	
	return misses
}

// analyzeFailureReason determines why a candidate failed gates
func (ar *AnalystRunner) analyzeFailureReason(candidate CandidateResult) (string, map[string]interface{}) {
	// Check data staleness first
	if time.Since(candidate.Meta.Timestamp) > 5*time.Minute {
		return ReasonDataStale, map[string]interface{}{
			"timestamp": candidate.Meta.Timestamp,
			"age_minutes": time.Since(candidate.Meta.Timestamp).Minutes(),
		}
	}
	
	// Check microstructure gates
	if micro, ok := candidate.Gates.Microstructure["all_pass"]; ok {
		if allPass, ok := micro.(bool); ok && !allPass {
			// Check specific microstructure failures
			if spread, ok := candidate.Gates.Microstructure["spread"].(map[string]interface{}); ok {
				if spreadOk, ok := spread["ok"].(bool); ok && !spreadOk {
					return ReasonSpreadWide, map[string]interface{}{
						"bps": spread["value"],
						"threshold": spread["threshold"],
					}
				}
			}
			
			if depth, ok := candidate.Gates.Microstructure["depth"].(map[string]interface{}); ok {
				if depthOk, ok := depth["ok"].(bool); ok && !depthOk {
					return ReasonDepthLow, map[string]interface{}{
						"depth_usd": depth["value"],
						"threshold": depth["threshold"],
					}
				}
			}
			
			if vadr, ok := candidate.Gates.Microstructure["vadr"].(map[string]interface{}); ok {
				if vadrOk, ok := vadr["ok"].(bool); ok && !vadrOk {
					return ReasonVADRLow, map[string]interface{}{
						"vadr": vadr["value"],
						"threshold": vadr["threshold"],
					}
				}
			}
		}
	}
	
	// Check freshness gate
	if fresh, ok := candidate.Gates.Freshness["ok"].(bool); ok && !fresh {
		return ReasonFreshnessStale, map[string]interface{}{
			"bars_age": candidate.Gates.Freshness["bars_age"],
			"price_change_atr": candidate.Gates.Freshness["price_change_atr"],
		}
	}
	
	// Check fatigue gate
	if fatigue, ok := candidate.Gates.Fatigue["ok"].(bool); ok && !fatigue {
		return ReasonFatigue, map[string]interface{}{
			"status": candidate.Gates.Fatigue["status"],
			"momentum_24h": candidate.Factors.MomentumCore,
		}
	}
	
	// Check late fill gate
	if lateFill, ok := candidate.Gates.LateFill["ok"].(bool); ok && !lateFill {
		return ReasonLateFill, map[string]interface{}{
			"fill_delay_seconds": candidate.Gates.LateFill["fill_delay_seconds"],
		}
	}
	
	// Check if score was too low
	if candidate.Score.Score < 60.0 { // Arbitrary threshold for "low score"
		return ReasonScoreLow, map[string]interface{}{
			"score": candidate.Score.Score,
			"rank": candidate.Score.Rank,
		}
	}
	
	return ReasonUnknown, map[string]interface{}{
		"decision": candidate.Decision,
		"score": candidate.Score.Score,
	}
}

// selectTopMisses returns the most significant misses
func (ar *AnalystRunner) selectTopMisses(allMisses []Miss, count int) []Miss {
	// Sort by performance descending (worst misses first)
	sort.Slice(allMisses, func(i, j int) bool {
		return allMisses[i].Performance > allMisses[j].Performance
	})
	
	if len(allMisses) > count {
		return allMisses[:count]
	}
	
	return allMisses
}

// checkPolicyCompliance verifies coverage meets quality thresholds
func (ar *AnalystRunner) checkPolicyCompliance(coverage1h, coverage24h, coverage7d Coverage) bool {
	return !coverage1h.ThresholdBreach && !coverage24h.ThresholdBreach && !coverage7d.ThresholdBreach
}

// generateSummary creates a human-readable summary
func (ar *AnalystRunner) generateSummary(report *AnalystReport) string {
	policyStatus := "PASS"
	if !report.PolicyPass {
		policyStatus = "FAIL"
	}
	
	return fmt.Sprintf(
		"Coverage Analysis: %s | 1h: %.1f%% recall (%.1f%% miss rate) | 24h: %.1f%% recall (%.1f%% miss rate) | 7d: %.1f%% recall (%.1f%% miss rate) | Policy: %s",
		report.RunTime.Format("15:04:05"),
		report.Coverage1h.RecallAt20*100,
		report.Coverage1h.BadMissRate*100,
		report.Coverage24h.RecallAt20*100,
		report.Coverage24h.BadMissRate*100,
		report.Coverage7d.RecallAt20*100,
		report.Coverage7d.BadMissRate*100,
		policyStatus,
	)
}

// writeOutputs writes all analysis outputs to files
func (ar *AnalystRunner) writeOutputs(winners *WinnerSet, misses []Miss, report *AnalystReport) error {
	// Ensure output directory exists
	if err := os.MkdirAll(ar.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Write winners.json
	winnersFile := filepath.Join(ar.outputDir, "winners.json")
	if err := ar.writeJSONFile(winnersFile, winners); err != nil {
		return fmt.Errorf("failed to write winners.json: %w", err)
	}
	
	// Write misses.jsonl
	missesFile := filepath.Join(ar.outputDir, "misses.jsonl")
	if err := ar.writeJSONLFile(missesFile, misses); err != nil {
		return fmt.Errorf("failed to write misses.jsonl: %w", err)
	}
	
	// Write coverage.json
	coverageData := map[string]interface{}{
		"1h":  report.Coverage1h,
		"24h": report.Coverage24h,
		"7d":  report.Coverage7d,
		"policy_pass": report.PolicyPass,
		"run_time": report.RunTime,
	}
	
	coverageFile := filepath.Join(ar.outputDir, "coverage.json")
	if err := ar.writeJSONFile(coverageFile, coverageData); err != nil {
		return fmt.Errorf("failed to write coverage.json: %w", err)
	}
	
	// Write report.md
	reportFile := filepath.Join(ar.outputDir, "report.md")
	if err := ar.writeMarkdownReport(reportFile, report); err != nil {
		return fmt.Errorf("failed to write report.md: %w", err)
	}
	
	log.Info().Str("output_dir", ar.outputDir).Msg("All analyst outputs written successfully")
	
	return nil
}

// writeJSONFile writes data as JSON to a file
func (ar *AnalystRunner) writeJSONFile(filename string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(filename, jsonData, 0644)
}

// writeJSONLFile writes data as JSONL to a file
func (ar *AnalystRunner) writeJSONLFile(filename string, data []Miss) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	for _, miss := range data {
		if err := encoder.Encode(miss); err != nil {
			return err
		}
	}
	
	return nil
}

// writeMarkdownReport writes a markdown report
func (ar *AnalystRunner) writeMarkdownReport(filename string, report *AnalystReport) error {
	content := fmt.Sprintf(`# CryptoRun Coverage Analysis Report

## Summary
**Run Time:** %s  
**Policy Status:** %s  

%s

## Coverage Metrics

### 1 Hour Window
- **Total Winners:** %d
- **Candidates Found:** %d  
- **Hits:** %d
- **Misses:** %d
- **Recall@20:** %.1f%%
- **Good Filter Rate:** %.1f%%
- **Bad Miss Rate:** %.1f%% (threshold: %.1f%%)
- **Stale Data Rate:** %.1f%%
- **Threshold Breach:** %t

### 24 Hour Window
- **Total Winners:** %d
- **Candidates Found:** %d
- **Hits:** %d
- **Misses:** %d
- **Recall@20:** %.1f%%
- **Good Filter Rate:** %.1f%%
- **Bad Miss Rate:** %.1f%% (threshold: %.1f%%)
- **Stale Data Rate:** %.1f%%
- **Threshold Breach:** %t

### 7 Day Window
- **Total Winners:** %d
- **Candidates Found:** %d
- **Hits:** %d
- **Misses:** %d
- **Recall@20:** %.1f%%
- **Good Filter Rate:** %.1f%%
- **Bad Miss Rate:** %.1f%% (threshold: %.1f%%)
- **Stale Data Rate:** %.1f%%
- **Threshold Breach:** %t

## Top Misses

| Symbol | Timeframe | Performance | Reason | Evidence |
|--------|-----------|-------------|---------|-----------|
`,
		report.RunTime.Format("2006-01-02 15:04:05 UTC"),
		map[bool]string{true: "✅ PASS", false: "❌ FAIL"}[report.PolicyPass],
		report.Summary,
		
		// 1h metrics
		report.Coverage1h.TotalWinners,
		report.Coverage1h.CandidatesFound,
		report.Coverage1h.Hits,
		report.Coverage1h.Misses,
		report.Coverage1h.RecallAt20*100,
		report.Coverage1h.GoodFilterRate*100,
		report.Coverage1h.BadMissRate*100,
		report.Coverage1h.PolicyThreshold*100,
		report.Coverage1h.StaleDataRate*100,
		report.Coverage1h.ThresholdBreach,
		
		// 24h metrics
		report.Coverage24h.TotalWinners,
		report.Coverage24h.CandidatesFound,
		report.Coverage24h.Hits,
		report.Coverage24h.Misses,
		report.Coverage24h.RecallAt20*100,
		report.Coverage24h.GoodFilterRate*100,
		report.Coverage24h.BadMissRate*100,
		report.Coverage24h.PolicyThreshold*100,
		report.Coverage24h.StaleDataRate*100,
		report.Coverage24h.ThresholdBreach,
		
		// 7d metrics
		report.Coverage7d.TotalWinners,
		report.Coverage7d.CandidatesFound,
		report.Coverage7d.Hits,
		report.Coverage7d.Misses,
		report.Coverage7d.RecallAt20*100,
		report.Coverage7d.GoodFilterRate*100,
		report.Coverage7d.BadMissRate*100,
		report.Coverage7d.PolicyThreshold*100,
		report.Coverage7d.StaleDataRate*100,
		report.Coverage7d.ThresholdBreach,
	)
	
	// Add top misses table
	for _, miss := range report.TopMisses {
		evidenceStr := fmt.Sprintf("%v", miss.Evidence)
		if len(evidenceStr) > 50 {
			evidenceStr = evidenceStr[:47] + "..."
		}
		
		content += fmt.Sprintf("| %s | %s | %.1f%% | %s | %s |\n",
			miss.Symbol,
			miss.TimeFrame,
			miss.Performance,
			miss.ReasonCode,
			evidenceStr,
		)
	}
	
	content += `
## Files Generated
- ` + "`winners.json`" + ` - Top performers from Kraken/fixtures
- ` + "`misses.jsonl`" + ` - Line-by-line miss analysis
- ` + "`coverage.json`" + ` - Machine-readable coverage metrics
- ` + "`report.md`" + ` - This human-readable report
`
	
	return os.WriteFile(filename, []byte(content), 0644)
}

// loadQualityPolicy loads quality policy from config or returns defaults
func loadQualityPolicy() QualityPolicy {
	policyFile := "config/quality_policies.json"
	
	if _, err := os.Stat(policyFile); os.IsNotExist(err) {
		return DefaultQualityPolicy()
	}
	
	data, err := os.ReadFile(policyFile)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read quality policy file, using defaults")
		return DefaultQualityPolicy()
	}
	
	var policy QualityPolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		log.Warn().Err(err).Msg("Failed to parse quality policy file, using defaults")
		return DefaultQualityPolicy()
	}
	
	return policy
}