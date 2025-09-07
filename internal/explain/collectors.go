package explain

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/sawpanic/cryptorun/internal/scoring"
)

type DataCollector struct {
	scoringCalc   *scoring.Calculator
	version       string
	artifactsPath string
}

func NewDataCollector(version string, artifactsPath string) *DataCollector {
	return &DataCollector{
		scoringCalc:   scoring.NewCalculator(scoring.RegimeChoppy),
		version:       version,
		artifactsPath: artifactsPath,
	}
}

func (dc *DataCollector) GenerateReport(ctx context.Context, symbols []string, inputs map[string]interface{}) (*ExplainReport, error) {
	timestamp := time.Now().UTC()
	inputHash := GenerateInputHash(inputs)

	report := &ExplainReport{
		Meta: ReportMeta{
			Timestamp:   timestamp,
			InputHash:   inputHash,
			Version:     dc.version,
			ReportType:  "full_explainability",
			AssetsCount: len(symbols),
		},
		Universe: make([]AssetExplain, 0, len(symbols)),
	}

	configSnapshot, err := dc.collectConfigSnapshot()
	if err != nil {
		return nil, fmt.Errorf("failed to collect config: %w", err)
	}
	report.Config = configSnapshot

	systemHealth, err := dc.collectSystemHealth()
	if err != nil {
		return nil, fmt.Errorf("failed to collect system health: %w", err)
	}
	report.Health = systemHealth

	for _, symbol := range symbols {
		assetExplain, err := dc.collectAssetExplain(ctx, symbol, inputs)
		if err != nil {
			continue
		}
		report.Universe = append(report.Universe, *assetExplain)
	}

	dc.updateCountsAndRanking(report)
	report.SortForStability()

	return report, nil
}

func (dc *DataCollector) collectAssetExplain(ctx context.Context, symbol string, inputs map[string]interface{}) (*AssetExplain, error) {
	factorInput := dc.mockFactorInput(symbol)

	scoreResult, err := dc.scoringCalc.Calculate(factorInput)
	if err != nil {
		return nil, fmt.Errorf("scoring failed for %s: %w", symbol, err)
	}

	gateResults := dc.collectGateResults(symbol, scoreResult.Score)
	microstructure := dc.collectMicrostructure(symbol)
	catalystProfile := dc.collectCatalystProfile(symbol)
	attribution := dc.generateAttribution(symbol, scoreResult, gateResults)
	dataQuality := dc.collectDataQuality(symbol)

	decision := "excluded"
	if gateResults.OverallResult {
		decision = "included"
	}

	return &AssetExplain{
		Symbol:          symbol,
		Decision:        decision,
		Score:           scoreResult.Score,
		FactorParts:     scoreResult.Parts,
		GateResults:     gateResults,
		Microstructure:  microstructure,
		CatalystProfile: catalystProfile,
		Attribution:     attribution,
		DataQuality:     dataQuality,
	}, nil
}

func (dc *DataCollector) collectGateResults(symbol string, score float64) GateResults {
	entryGate := GateResult{
		Passed:    score >= 75.0,
		Value:     score,
		Threshold: 75.0,
		Reason:    "composite_score_threshold",
	}

	freshnessGate := GateResult{
		Passed:    true,
		Value:     1.0,
		Threshold: 1.2,
		Reason:    "within_atr_bounds",
	}

	fatigueGate := GateResult{
		Passed:    true,
		Value:     65.0,
		Threshold: 70.0,
		Reason:    "rsi_not_overbought",
	}

	lateFillGate := GateResult{
		Passed:    true,
		Value:     15.0,
		Threshold: 30.0,
		Reason:    "signal_timing_acceptable",
	}

	vadr := dc.mockVADR(symbol)
	microGate := GateResult{
		Passed:    vadr >= 1.8,
		Value:     vadr,
		Threshold: 1.8,
		Reason:    "vadr_liquidity_requirement",
	}

	overallResult := entryGate.Passed && freshnessGate.Passed &&
		fatigueGate.Passed && lateFillGate.Passed && microGate.Passed

	return GateResults{
		EntryGate:     entryGate,
		FreshnessGate: freshnessGate,
		FatigueGate:   fatigueGate,
		LateFillGate:  lateFillGate,
		MicroGate:     microGate,
		OverallResult: overallResult,
	}
}

func (dc *DataCollector) collectMicrostructure(symbol string) MicrostructureMetrics {
	return MicrostructureMetrics{
		SpreadBps:        dc.mockSpread(symbol),
		DepthUSD:         dc.mockDepth(symbol),
		VADR:             dc.mockVADR(symbol),
		Exchange:         "kraken",
		IsExchangeNative: true,
	}
}

func (dc *DataCollector) collectCatalystProfile(symbol string) CatalystProfile {
	nextEvent := time.Now().UTC().Add(time.Hour * 24 * 3)
	return CatalystProfile{
		HeatScore:  dc.mockHeatScore(symbol),
		TimeDecay:  0.8,
		EventTypes: []string{"earnings", "token_unlock"},
		NextEvent:  &nextEvent,
	}
}

func (dc *DataCollector) generateAttribution(symbol string, scoreResult *scoring.CompositeScore, gates GateResults) Attribution {
	var inclusionReasons []string
	var exclusionReasons []string

	if scoreResult.Score >= 75.0 {
		inclusionReasons = append(inclusionReasons, "strong_composite_score")
	} else {
		exclusionReasons = append(exclusionReasons, "weak_composite_score")
	}

	if scoreResult.Parts["momentum"] > 20.0 {
		inclusionReasons = append(inclusionReasons, "strong_momentum")
	}

	if !gates.MicroGate.Passed {
		exclusionReasons = append(exclusionReasons, "insufficient_liquidity")
	}

	if len(inclusionReasons) > 3 {
		inclusionReasons = inclusionReasons[:3]
	}
	if len(exclusionReasons) > 3 {
		exclusionReasons = exclusionReasons[:3]
	}

	weightBreakdown := make(map[string]string)
	for factor, weight := range scoreResult.Parts {
		if weight > 10.0 {
			weightBreakdown[factor] = "high_contribution"
		} else if weight > 5.0 {
			weightBreakdown[factor] = "medium_contribution"
		} else {
			weightBreakdown[factor] = "low_contribution"
		}
	}

	return Attribution{
		TopInclusionReasons: inclusionReasons,
		TopExclusionReasons: exclusionReasons,
		RegimeInfluence:     string(scoreResult.Meta.Regime),
		WeightBreakdown:     weightBreakdown,
	}
}

func (dc *DataCollector) collectDataQuality(symbol string) DataQuality {
	now := time.Now().UTC()

	ttls := map[string]time.Time{
		"price_data":  now.Add(time.Minute * 5),
		"volume_data": now.Add(time.Minute * 10),
		"social_data": now.Add(time.Hour * 1),
		"micro_data":  now.Add(time.Minute * 2),
	}

	cacheHits := map[string]bool{
		"price_data":  true,
		"volume_data": true,
		"social_data": false,
		"micro_data":  true,
	}

	freshnessAge := map[string]string{
		"price_data":  "45s",
		"volume_data": "1m30s",
		"social_data": "15m",
		"micro_data":  "30s",
	}

	return DataQuality{
		TTLs:          ttls,
		CacheHits:     cacheHits,
		FreshnessAge:  freshnessAge,
		MissingFields: []string{},
	}
}

func (dc *DataCollector) collectConfigSnapshot() (ConfigSnapshot, error) {
	regimeWeights := map[string]float64{
		"momentum":  0.40,
		"technical": 0.35,
		"volume":    0.15,
		"quality":   0.10,
	}

	gateThresholds := map[string]float64{
		"entry_score": 75.0,
		"vadr":        1.8,
		"spread_bps":  50.0,
		"depth_usd":   100000.0,
	}

	momentumWeights := map[string]float64{
		"1h":  0.20,
		"4h":  0.35,
		"12h": 0.30,
		"24h": 0.15,
	}

	return ConfigSnapshot{
		RegimeWeights:   regimeWeights,
		CurrentRegime:   "choppy",
		GateThresholds:  gateThresholds,
		SocialCap:       10.0,
		MomentumWeights: momentumWeights,
		ConfigVersion:   "v3.2.1",
	}, nil
}

func (dc *DataCollector) collectSystemHealth() (SystemHealth, error) {
	now := time.Now().UTC()

	providerStatus := map[string]ProviderHealth{
		"kraken": {
			Status:      "healthy",
			LastSuccess: now.Add(-time.Minute * 2),
			ErrorRate:   0.05,
			Latency:     "150ms",
		},
		"coingecko": {
			Status:      "degraded",
			LastSuccess: now.Add(-time.Minute * 10),
			ErrorRate:   0.15,
			Latency:     "800ms",
		},
	}

	circuitBreakers := map[string]bool{
		"kraken_api":    false,
		"coingecko_api": true,
	}

	rateLimits := map[string]RateLimitInfo{
		"kraken": {
			Remaining: 85,
			Reset:     now.Add(time.Minute * 5),
			Limit:     100,
		},
	}

	cacheStats := CacheStats{
		HitRate:      0.87,
		TotalHits:    1250,
		TotalMisses:  185,
		EvictionRate: 0.02,
	}

	return SystemHealth{
		ProviderStatus:  providerStatus,
		CircuitBreakers: circuitBreakers,
		RateLimits:      rateLimits,
		CacheStats:      cacheStats,
	}, nil
}

func (dc *DataCollector) updateCountsAndRanking(report *ExplainReport) {
	includedCount := 0
	excludedCount := 0

	sort.Slice(report.Universe, func(i, j int) bool {
		return report.Universe[i].Score > report.Universe[j].Score
	})

	for i := range report.Universe {
		report.Universe[i].Rank = i + 1
		if report.Universe[i].Decision == "included" {
			includedCount++
		} else {
			excludedCount++
		}
	}

	report.Meta.IncludedCount = includedCount
	report.Meta.ExcludedCount = excludedCount
}

func (dc *DataCollector) mockFactorInput(symbol string) scoring.FactorInput {
	baseReturn := dc.hashSymbol(symbol, 100) - 50

	return scoring.FactorInput{
		Symbol: symbol,
		Momentum: scoring.MomentumFactors{
			Return1h:  float64(baseReturn) * 0.2,
			Return4h:  float64(baseReturn) * 0.5,
			Return12h: float64(baseReturn) * 0.8,
			Return24h: float64(baseReturn) * 1.0,
			Return7d:  float64(baseReturn) * 1.2,
			Accel4h:   float64(baseReturn) * 0.1,
		},
		Technical: scoring.TechnicalFactors{
			RSI14:    50.0 + float64(dc.hashSymbol(symbol, 50)),
			MACD:     float64(baseReturn) * 0.3,
			BBWidth:  0.5 + float64(dc.hashSymbol(symbol, 100))/100.0,
			ATRRatio: 1.0 + float64(dc.hashSymbol(symbol, 50))/100.0,
		},
		Volume: scoring.VolumeFactors{
			VolumeRatio24h: 1.0 + float64(dc.hashSymbol(symbol, 200))/100.0,
			VWAP:           float64(baseReturn) * 100,
			OBV:            float64(baseReturn) * 1000,
			VolSpike:       1.0 + float64(dc.hashSymbol(symbol, 300))/100.0,
		},
		Quality: scoring.QualityFactors{
			Spread:    float64(dc.hashSymbol(symbol, 100)) / 10000.0,
			Depth:     100000 + float64(dc.hashSymbol(symbol, 500000)),
			VADR:      1.0 + float64(dc.hashSymbol(symbol, 200))/100.0,
			MarketCap: float64(dc.hashSymbol(symbol, 1000000000)),
		},
		Social: scoring.SocialFactors{
			Sentiment:    float64(dc.hashSymbol(symbol, 100))/100.0 - 0.5,
			Mentions:     float64(dc.hashSymbol(symbol, 2000)),
			SocialVolume: float64(dc.hashSymbol(symbol, 10000)),
			RedditScore:  float64(dc.hashSymbol(symbol, 100)),
		},
	}
}

func (dc *DataCollector) mockSpread(symbol string) float64 {
	return float64(dc.hashSymbol(symbol, 100)) / 2000.0
}

func (dc *DataCollector) mockDepth(symbol string) float64 {
	return 50000 + float64(dc.hashSymbol(symbol, 200000))
}

func (dc *DataCollector) mockVADR(symbol string) float64 {
	return 1.0 + float64(dc.hashSymbol(symbol, 150))/100.0
}

func (dc *DataCollector) mockHeatScore(symbol string) float64 {
	return float64(dc.hashSymbol(symbol, 100))
}

func (dc *DataCollector) hashSymbol(symbol string, mod int) int {
	hash := 0
	for _, c := range symbol {
		hash = (hash*31 + int(c)) % mod
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}
