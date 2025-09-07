package explain

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSchemaRoundTrip(t *testing.T) {
	collector := NewDataCollector("test-v1.0", "./test_artifacts")

	symbols := []string{"BTC-USD", "ETH-USD", "ADA-USD"}
	inputs := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"regime":    "choppy",
		"scan_type": "momentum",
	}

	report, err := collector.GenerateReport(context.Background(), symbols, inputs)
	if err != nil {
		t.Fatalf("generate report failed: %v", err)
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var roundTrip ExplainReport
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if report.Meta.AssetsCount != roundTrip.Meta.AssetsCount {
		t.Errorf("assets count mismatch: %d vs %d", report.Meta.AssetsCount, roundTrip.Meta.AssetsCount)
	}

	if len(report.Universe) != len(roundTrip.Universe) {
		t.Errorf("universe length mismatch: %d vs %d", len(report.Universe), len(roundTrip.Universe))
	}

	if report.Config.CurrentRegime != roundTrip.Config.CurrentRegime {
		t.Errorf("regime mismatch: %s vs %s", report.Config.CurrentRegime, roundTrip.Config.CurrentRegime)
	}
}

func TestStableOrdering(t *testing.T) {
	collector := NewDataCollector("test-v1.0", "./test_artifacts")

	symbols := []string{"ETH-USD", "BTC-USD", "ADA-USD"}
	inputs := map[string]interface{}{
		"timestamp": time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		"regime":    "choppy",
	}

	report1, err := collector.GenerateReport(context.Background(), symbols, inputs)
	if err != nil {
		t.Fatalf("generate report1 failed: %v", err)
	}

	report2, err := collector.GenerateReport(context.Background(), symbols, inputs)
	if err != nil {
		t.Fatalf("generate report2 failed: %v", err)
	}

	if report1.Meta.InputHash != report2.Meta.InputHash {
		t.Errorf("input hashes differ: %s vs %s", report1.Meta.InputHash, report2.Meta.InputHash)
	}

	if len(report1.Universe) != len(report2.Universe) {
		t.Fatalf("universe lengths differ: %d vs %d", len(report1.Universe), len(report2.Universe))
	}

	for i := range report1.Universe {
		if report1.Universe[i].Symbol != report2.Universe[i].Symbol {
			t.Errorf("symbol order differs at index %d: %s vs %s",
				i, report1.Universe[i].Symbol, report2.Universe[i].Symbol)
		}

		if report1.Universe[i].Score != report2.Universe[i].Score {
			t.Errorf("scores differ for %s: %.6f vs %.6f",
				report1.Universe[i].Symbol, report1.Universe[i].Score, report2.Universe[i].Score)
		}
	}
}

func TestCSVRowGeneration(t *testing.T) {
	asset := AssetExplain{
		Symbol:   "BTC-USD",
		Decision: "included",
		Score:    85.5,
		Rank:     1,
		FactorParts: map[string]float64{
			"momentum":  45.2,
			"technical": 12.8,
			"volume":    8.3,
			"quality":   4.1,
			"social":    3.0,
		},
		GateResults: GateResults{
			EntryGate: GateResult{Passed: true, Value: 85.5, Threshold: 75.0},
		},
		Microstructure: MicrostructureMetrics{
			SpreadBps: 0.025,
			DepthUSD:  150000,
			VADR:      2.1,
			Exchange:  "kraken",
		},
		CatalystProfile: CatalystProfile{
			HeatScore: 75.0,
		},
		Attribution: Attribution{
			TopInclusionReasons: []string{"strong_momentum", "good_liquidity"},
			RegimeInfluence:     "choppy",
		},
		DataQuality: DataQuality{
			CacheHits: map[string]bool{
				"price_data":  true,
				"volume_data": true,
				"social_data": false,
			},
		},
	}

	csvRow := asset.ToCSVRow()

	if csvRow.Symbol != "BTC-USD" {
		t.Errorf("expected symbol BTC-USD, got %s", csvRow.Symbol)
	}

	if csvRow.Decision != "included" {
		t.Errorf("expected decision included, got %s", csvRow.Decision)
	}

	if csvRow.Score != 85.5 {
		t.Errorf("expected score 85.5, got %.6f", csvRow.Score)
	}

	if csvRow.Momentum != 45.2 {
		t.Errorf("expected momentum 45.2, got %.6f", csvRow.Momentum)
	}

	if csvRow.TopReason != "strong_momentum" {
		t.Errorf("expected top reason strong_momentum, got %s", csvRow.TopReason)
	}

	expectedCacheHitRate := 2.0 / 3.0
	if csvRow.CacheHitRate < expectedCacheHitRate-0.01 || csvRow.CacheHitRate > expectedCacheHitRate+0.01 {
		t.Errorf("expected cache hit rate ~%.3f, got %.6f", expectedCacheHitRate, csvRow.CacheHitRate)
	}
}

func TestMockWriter(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewMockAtomicWriter(tempDir)

	collector := NewDataCollector("test-v1.0", tempDir)
	symbols := []string{"BTC-USD", "ETH-USD"}
	inputs := map[string]interface{}{"test": true}

	report, err := collector.GenerateReport(context.Background(), symbols, inputs)
	if err != nil {
		t.Fatalf("generate report failed: %v", err)
	}

	if err := writer.WriteExplainReport(report, "test"); err != nil {
		t.Fatalf("write explain report failed: %v", err)
	}

	jsonFiles, _ := filepath.Glob(filepath.Join(tempDir, "*-test-explain.json"))
	if len(jsonFiles) != 1 {
		t.Errorf("expected 1 JSON file, got %d", len(jsonFiles))
	}

	csvFiles, _ := filepath.Glob(filepath.Join(tempDir, "*-test-explain.csv"))
	if len(csvFiles) != 1 {
		t.Errorf("expected 1 CSV file, got %d", len(csvFiles))
	}
}

func TestInputHashConsistency(t *testing.T) {
	inputs1 := map[string]interface{}{
		"regime":    "choppy",
		"timestamp": "2024-01-15T10:30:00Z",
		"scan_type": "momentum",
	}

	inputs2 := map[string]interface{}{
		"scan_type": "momentum",
		"regime":    "choppy",
		"timestamp": "2024-01-15T10:30:00Z",
	}

	hash1 := GenerateInputHash(inputs1)
	hash2 := GenerateInputHash(inputs2)

	if hash1 != hash2 {
		t.Errorf("input hashes should be equal regardless of key order: %s vs %s", hash1, hash2)
	}
}

func TestGateResultsValidation(t *testing.T) {
	collector := NewDataCollector("test-v1.0", "./test_artifacts")

	gateResults := collector.collectGateResults("BTC-USD", 80.0)

	if !gateResults.EntryGate.Passed {
		t.Error("entry gate should pass with score 80.0")
	}

	if gateResults.EntryGate.Threshold != 75.0 {
		t.Errorf("expected entry gate threshold 75.0, got %.1f", gateResults.EntryGate.Threshold)
	}

	if !gateResults.OverallResult {
		t.Error("overall result should be true when all gates pass")
	}
}

func TestDataQualityCollection(t *testing.T) {
	collector := NewDataCollector("test-v1.0", "./test_artifacts")

	dataQuality := collector.collectDataQuality("BTC-USD")

	if len(dataQuality.TTLs) == 0 {
		t.Error("expected TTLs to be populated")
	}

	if len(dataQuality.CacheHits) == 0 {
		t.Error("expected cache hits to be populated")
	}

	if len(dataQuality.FreshnessAge) == 0 {
		t.Error("expected freshness age to be populated")
	}

	hitCount := 0
	for _, hit := range dataQuality.CacheHits {
		if hit {
			hitCount++
		}
	}

	if hitCount == 0 {
		t.Error("expected at least some cache hits")
	}
}
