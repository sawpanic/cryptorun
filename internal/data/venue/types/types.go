package types

import "time"

// OrderBook represents normalized L1/L2 data from any venue
type OrderBook struct {
	// Metadata
	Symbol        string    `json:"symbol"`
	Venue         string    `json:"venue"`
	TimestampMono time.Time `json:"timestamp_mono"`
	SequenceNum   int64     `json:"sequence_num"`

	// L1 Data (Best Bid/Ask)
	BestBidPrice float64 `json:"best_bid_price"`
	BestBidQty   float64 `json:"best_bid_qty"`
	BestAskPrice float64 `json:"best_ask_price"`
	BestAskQty   float64 `json:"best_ask_qty"`

	// Derived Metrics
	MidPrice              float64 `json:"mid_price"`
	SpreadBPS             float64 `json:"spread_bps"`
	DepthUSDPlusMinus2Pct float64 `json:"depth_usd_plus_minus_2pct"`

	// L2 Data (Full Book)
	Bids []Level `json:"bids"`
	Asks []Level `json:"asks"`
}

// Level represents a single price level in the order book
type Level struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	ValueUSD float64 `json:"value_usd"`
}

// CachedOrderBook wraps OrderBook with cache metadata
type CachedOrderBook struct {
	OrderBook *OrderBook `json:"order_book"`
	Timestamp time.Time  `json:"cached_at"`
}

// MicrostructureMetrics contains validation metrics for gates
type MicrostructureMetrics struct {
	Symbol        string    `json:"symbol"`
	Venue         string    `json:"venue"`
	TimestampMono time.Time `json:"timestamp_mono"`

	// Primary Metrics
	SpreadBPS             float64 `json:"spread_bps"`
	DepthUSDPlusMinus2Pct float64 `json:"depth_usd_plus_minus_2pct"`
	VADR                  float64 `json:"vadr"` // Volume-Adjusted Daily Range
	ADV                   float64 `json:"adv"`  // Average Daily Volume

	// Validation Results
	SpreadValid  bool `json:"spread_valid"`  // < 50 bps
	DepthValid   bool `json:"depth_valid"`   // >= $100k
	VADRValid    bool `json:"vadr_valid"`    // >= 1.75x
	OverallValid bool `json:"overall_valid"` // All gates pass

	// Attribution
	DataSource     string `json:"data_source"` // "binance", "okx", "coinbase"
	CacheHit       bool   `json:"cache_hit"`
	FetchLatencyMs int64  `json:"fetch_latency_ms"`
}

// ProofBundle contains microstructure validation evidence
type ProofBundle struct {
	AssetSymbol   string    `json:"asset_symbol"`
	TimestampMono time.Time `json:"timestamp_mono"`
	ProvenValid   bool      `json:"proven_valid"`

	// Evidence
	OrderBookSnapshot     *OrderBook             `json:"order_book_snapshot"`
	MicrostructureMetrics *MicrostructureMetrics `json:"microstructure_metrics"`

	// Validation Chain
	SpreadProof ValidationProof `json:"spread_proof"`
	DepthProof  ValidationProof `json:"depth_proof"`
	VADRProof   ValidationProof `json:"vadr_proof"`

	// Metadata
	ProofGeneratedAt time.Time `json:"proof_generated_at"`
	VenueUsed        string    `json:"venue_used"`
	ProofID          string    `json:"proof_id"`
}

// ValidationProof documents a single validation check
type ValidationProof struct {
	Metric        string  `json:"metric"`
	ActualValue   float64 `json:"actual_value"`
	RequiredValue float64 `json:"required_value"`
	Operator      string  `json:"operator"` // "<", ">=", etc.
	Passed        bool    `json:"passed"`
	Evidence      string  `json:"evidence"` // Human-readable explanation
}
