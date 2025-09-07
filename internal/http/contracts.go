package http

import "time"

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string                    `json:"status"`
	Timestamp time.Time                 `json:"timestamp"`
	Providers map[string]ProviderHealth `json:"providers"`
	Circuits  map[string]CircuitHealth  `json:"circuits"`
	Latencies LatencyMetrics            `json:"latencies"`
}

// ProviderHealth represents individual provider status
type ProviderHealth struct {
	Name         string    `json:"name"`
	Status       string    `json:"status"` // healthy, degraded, down
	RateLimit    RateLimit `json:"rate_limit"`
	LastResponse time.Time `json:"last_response"`
	ErrorRate    float64   `json:"error_rate"`
}

// RateLimit represents rate limiting status
type RateLimit struct {
	Current   int `json:"current"`
	Limit     int `json:"limit"`
	Remaining int `json:"remaining"`
	ResetTime int `json:"reset_time"`
}

// CircuitHealth represents circuit breaker status
type CircuitHealth struct {
	Name        string  `json:"name"`
	State       string  `json:"state"` // closed, open, half-open
	FailureRate float64 `json:"failure_rate"`
	Requests    int64   `json:"requests"`
	Failures    int64   `json:"failures"`
}

// LatencyMetrics represents system latency information
type LatencyMetrics struct {
	P95Handler time.Duration `json:"p95_handler"`
	P99Handler time.Duration `json:"p99_handler"`
	AvgHandler time.Duration `json:"avg_handler"`
	Target     time.Duration `json:"target"`
}

// CandidatesResponse represents the candidates endpoint response
type CandidatesResponse struct {
	Candidates []CandidateInfo `json:"candidates"`
	Pagination PaginationInfo  `json:"pagination"`
	Generated  time.Time       `json:"generated"`
}

// CandidateInfo represents a single candidate with composite score and gate status
type CandidateInfo struct {
	Symbol         string             `json:"symbol"`
	CompositeScore float64            `json:"composite_score"`
	Rank           int                `json:"rank"`
	GateStatus     map[string]bool    `json:"gate_status"`
	Factors        map[string]float64 `json:"factors"`
	Timestamp      time.Time          `json:"timestamp"`
}

// PaginationInfo represents pagination metadata
type PaginationInfo struct {
	Total    int  `json:"total"`
	Page     int  `json:"page"`
	PageSize int  `json:"page_size"`
	HasNext  bool `json:"has_next"`
	HasPrev  bool `json:"has_prev"`
}

// ExplainResponse represents the explain endpoint response
type ExplainResponse struct {
	Symbol      string                 `json:"symbol"`
	Explanation map[string]interface{} `json:"explanation"`
	Source      string                 `json:"source"` // artifacts, live, cache
	Timestamp   time.Time              `json:"timestamp"`
	CacheHit    bool                   `json:"cache_hit"`
}

// RegimeResponse represents the regime endpoint response
type RegimeResponse struct {
	ActiveRegime  string             `json:"active_regime"`
	Confidence    float64            `json:"confidence"`
	Weights       map[string]float64 `json:"weights"`
	LastDetection time.Time          `json:"last_detection"`
	NextUpdate    time.Time          `json:"next_update"`
	Signals       map[string]float64 `json:"signals"`
	RegimeHistory []RegimeChange     `json:"regime_history"`
}

// RegimeChange represents a regime transition
type RegimeChange struct {
	FromRegime string    `json:"from_regime"`
	ToRegime   string    `json:"to_regime"`
	Timestamp  time.Time `json:"timestamp"`
	Confidence float64   `json:"confidence"`
}

// ErrorResponse represents API error responses
type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Code      string    `json:"code"`
	RequestID string    `json:"request_id"`
	Timestamp time.Time `json:"timestamp"`
}
