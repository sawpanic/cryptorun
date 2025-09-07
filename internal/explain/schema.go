package explain

import (
	"sort"
	"time"
)

type ExplainReport struct {
	Meta     ReportMeta     `json:"meta"`
	Universe []AssetExplain `json:"universe"`
	Config   ConfigSnapshot `json:"config"`
	Health   SystemHealth   `json:"health"`
}

type ReportMeta struct {
	Timestamp     time.Time `json:"timestamp"`
	InputHash     string    `json:"input_hash"`
	Version       string    `json:"version"`
	ReportType    string    `json:"report_type"`
	AssetsCount   int       `json:"assets_count"`
	IncludedCount int       `json:"included_count"`
	ExcludedCount int       `json:"excluded_count"`
}

type AssetExplain struct {
	Symbol          string                `json:"symbol"`
	Decision        string                `json:"decision"`
	Score           float64               `json:"score"`
	Rank            int                   `json:"rank"`
	FactorParts     map[string]float64    `json:"factor_parts"`
	GateResults     GateResults           `json:"gate_results"`
	Microstructure  MicrostructureMetrics `json:"microstructure"`
	CatalystProfile CatalystProfile       `json:"catalyst_profile"`
	Attribution     Attribution           `json:"attribution"`
	DataQuality     DataQuality           `json:"data_quality"`
}

type GateResults struct {
	EntryGate     GateResult `json:"entry_gate"`
	FreshnessGate GateResult `json:"freshness_gate"`
	FatigueGate   GateResult `json:"fatigue_gate"`
	LateFillGate  GateResult `json:"late_fill_gate"`
	MicroGate     GateResult `json:"micro_gate"`
	OverallResult bool       `json:"overall_result"`
}

type GateResult struct {
	Passed    bool    `json:"passed"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Reason    string  `json:"reason"`
}

type MicrostructureMetrics struct {
	SpreadBps        float64 `json:"spread_bps"`
	DepthUSD         float64 `json:"depth_usd"`
	VADR             float64 `json:"vadr"`
	Exchange         string  `json:"exchange"`
	IsExchangeNative bool    `json:"is_exchange_native"`
}

type CatalystProfile struct {
	HeatScore  float64    `json:"heat_score"`
	TimeDecay  float64    `json:"time_decay"`
	EventTypes []string   `json:"event_types"`
	NextEvent  *time.Time `json:"next_event,omitempty"`
}

type Attribution struct {
	TopInclusionReasons []string          `json:"top_inclusion_reasons"`
	TopExclusionReasons []string          `json:"top_exclusion_reasons"`
	RegimeInfluence     string            `json:"regime_influence"`
	WeightBreakdown     map[string]string `json:"weight_breakdown"`
}

type DataQuality struct {
	TTLs          map[string]time.Time `json:"ttls"`
	CacheHits     map[string]bool      `json:"cache_hits"`
	FreshnessAge  map[string]string    `json:"freshness_age"`
	MissingFields []string             `json:"missing_fields"`
}

type ConfigSnapshot struct {
	RegimeWeights   map[string]float64 `json:"regime_weights"`
	CurrentRegime   string             `json:"current_regime"`
	GateThresholds  map[string]float64 `json:"gate_thresholds"`
	SocialCap       float64            `json:"social_cap"`
	MomentumWeights map[string]float64 `json:"momentum_weights"`
	ConfigVersion   string             `json:"config_version"`
}

type SystemHealth struct {
	ProviderStatus  map[string]ProviderHealth `json:"provider_status"`
	CircuitBreakers map[string]bool           `json:"circuit_breakers"`
	RateLimits      map[string]RateLimitInfo  `json:"rate_limits"`
	CacheStats      CacheStats                `json:"cache_stats"`
}

type ProviderHealth struct {
	Status      string    `json:"status"`
	LastSuccess time.Time `json:"last_success"`
	ErrorRate   float64   `json:"error_rate"`
	Latency     string    `json:"latency"`
}

type RateLimitInfo struct {
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
	Limit     int       `json:"limit"`
}

type CacheStats struct {
	HitRate      float64 `json:"hit_rate"`
	TotalHits    int64   `json:"total_hits"`
	TotalMisses  int64   `json:"total_misses"`
	EvictionRate float64 `json:"eviction_rate"`
}

type CSVRow struct {
	Symbol       string  `csv:"symbol"`
	Decision     string  `csv:"decision"`
	Score        float64 `csv:"score"`
	Rank         int     `csv:"rank"`
	Momentum     float64 `csv:"momentum"`
	Technical    float64 `csv:"technical"`
	Volume       float64 `csv:"volume"`
	Quality      float64 `csv:"quality"`
	Social       float64 `csv:"social"`
	EntryGate    bool    `csv:"entry_gate"`
	SpreadBps    float64 `csv:"spread_bps"`
	DepthUSD     float64 `csv:"depth_usd"`
	VADR         float64 `csv:"vadr"`
	HeatScore    float64 `csv:"heat_score"`
	Regime       string  `csv:"regime"`
	Exchange     string  `csv:"exchange"`
	TopReason    string  `csv:"top_reason"`
	CacheHitRate float64 `csv:"cache_hit_rate"`
}

func (r *ExplainReport) SortForStability() {
	sort.Slice(r.Universe, func(i, j int) bool {
		if r.Universe[i].Symbol != r.Universe[j].Symbol {
			return r.Universe[i].Symbol < r.Universe[j].Symbol
		}
		return r.Universe[i].Score > r.Universe[j].Score
	})

	for i := range r.Universe {
		r.Universe[i].Rank = i + 1
	}
}

func (a *AssetExplain) ToCSVRow() CSVRow {
	momentum := a.FactorParts["momentum"]
	technical := a.FactorParts["technical"]
	volume := a.FactorParts["volume"]
	quality := a.FactorParts["quality"]
	social := a.FactorParts["social"]

	topReason := "no_reason"
	if len(a.Attribution.TopInclusionReasons) > 0 {
		topReason = a.Attribution.TopInclusionReasons[0]
	} else if len(a.Attribution.TopExclusionReasons) > 0 {
		topReason = a.Attribution.TopExclusionReasons[0]
	}

	cacheHitCount := 0
	totalCacheOps := len(a.DataQuality.CacheHits)
	for _, hit := range a.DataQuality.CacheHits {
		if hit {
			cacheHitCount++
		}
	}

	cacheHitRate := 0.0
	if totalCacheOps > 0 {
		cacheHitRate = float64(cacheHitCount) / float64(totalCacheOps)
	}

	return CSVRow{
		Symbol:       a.Symbol,
		Decision:     a.Decision,
		Score:        a.Score,
		Rank:         a.Rank,
		Momentum:     momentum,
		Technical:    technical,
		Volume:       volume,
		Quality:      quality,
		Social:       social,
		EntryGate:    a.GateResults.EntryGate.Passed,
		SpreadBps:    a.Microstructure.SpreadBps,
		DepthUSD:     a.Microstructure.DepthUSD,
		VADR:         a.Microstructure.VADR,
		HeatScore:    a.CatalystProfile.HeatScore,
		Regime:       a.Attribution.RegimeInfluence,
		Exchange:     a.Microstructure.Exchange,
		TopReason:    topReason,
		CacheHitRate: cacheHitRate,
	}
}

func GenerateInputHash(inputs map[string]interface{}) string {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	hash := "v1"
	for _, k := range keys {
		hash += "_" + k + ":" + toString(inputs[k])
	}
	return hash
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return string(rune(val))
	case float64:
		if val == float64(int(val)) {
			return string(rune(int(val)))
		}
		return "f64"
	default:
		return "obj"
	}
}
