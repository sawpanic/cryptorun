package guards

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ProviderGuard wraps API calls with caching, rate limiting, circuit breaking, and telemetry
type ProviderGuard struct {
	cache       *Cache
	rateLimiter *RateLimiter
	circuit     *CircuitBreaker
	telemetry   *Telemetry
	config      ProviderConfig
}

// ProviderConfig holds configuration for a specific provider
type ProviderConfig struct {
	Name            string  `yaml:"name"`
	TTLSeconds      int     `yaml:"ttl_seconds"`       // Cache TTL, default 300s
	BurstLimit      int     `yaml:"burst_limit"`       // Token bucket burst
	SustainedRate   float64 `yaml:"sustained_rate"`    // Requests per second
	MaxRetries      int     `yaml:"max_retries"`       // Retry attempts
	BackoffBaseMs   int     `yaml:"backoff_base_ms"`   // Base backoff in ms
	FailureThresh   float64 `yaml:"failure_thresh"`    // Circuit breaker threshold
	WindowRequests  int     `yaml:"window_requests"`   // Circuit breaker window size
	ProbeInterval   int     `yaml:"probe_interval"`    // Half-open probe interval
	EnableFileCache bool    `yaml:"enable_file_cache"` // File-backed cache
	CachePath       string  `yaml:"cache_path"`        // Cache file location
}

// GuardedResponse represents a response with metadata
type GuardedResponse struct {
	Data        []byte
	StatusCode  int
	Headers     http.Header
	Cached      bool
	Age         time.Duration
	RetryCount  int
	CircuitOpen bool
}

// GuardedRequest represents a request with caching key components
type GuardedRequest struct {
	Method   string
	URL      string
	Headers  map[string]string
	Body     []byte
	CacheKey string
}

// ProviderError represents provider-specific errors with retry guidance
type ProviderError struct {
	Provider   string
	StatusCode int
	Message    string
	Retryable  bool
	RetryAfter time.Duration
}

func (e *ProviderError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("provider %s error (status %d): %s (retry after %v)",
			e.Provider, e.StatusCode, e.Message, e.RetryAfter)
	}
	return fmt.Sprintf("provider %s error (status %d): %s",
		e.Provider, e.StatusCode, e.Message)
}

// NewProviderGuard creates a new guard with the given configuration
func NewProviderGuard(config ProviderConfig) *ProviderGuard {
	return &ProviderGuard{
		cache:       NewCache(config),
		rateLimiter: NewRateLimiter(config),
		circuit:     NewCircuitBreaker(config),
		telemetry:   NewTelemetry(config.Name),
		config:      config,
	}
}

// Execute performs a guarded API call with all middleware applied
func (g *ProviderGuard) Execute(ctx context.Context, req GuardedRequest, fetcher func(context.Context, GuardedRequest) (*GuardedResponse, error)) (*GuardedResponse, error) {
	startTime := time.Now()

	// Check circuit breaker first
	if g.circuit.IsOpen() {
		g.telemetry.RecordCircuitOpen()
		return nil, &ProviderError{
			Provider:  g.config.Name,
			Message:   "circuit breaker open",
			Retryable: false,
		}
	}

	// Check cache
	if cached, found := g.cache.Get(req.CacheKey); found {
		g.telemetry.RecordCacheHit(time.Since(startTime))
		return &GuardedResponse{
			Data:        cached.Data,
			StatusCode:  cached.StatusCode,
			Headers:     cached.Headers,
			Cached:      true,
			Age:         time.Since(cached.Timestamp),
			CircuitOpen: false,
		}, nil
	}

	g.telemetry.RecordCacheMiss()

	// Rate limiting check
	if !g.rateLimiter.Allow() {
		g.telemetry.RecordRateLimit()
		return nil, &ProviderError{
			Provider:   g.config.Name,
			Message:    "rate limit exceeded",
			Retryable:  true,
			RetryAfter: time.Second, // Basic retry guidance
		}
	}

	// Execute with retries
	var lastErr error
	for attempt := 0; attempt <= g.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := g.calculateBackoff(attempt)
			g.telemetry.RecordBackoff(backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err := fetcher(ctx, req)

		if err == nil && resp != nil {
			// Success - cache and return
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				g.cache.Set(req.CacheKey, CacheEntry{
					Data:       resp.Data,
					StatusCode: resp.StatusCode,
					Headers:    resp.Headers,
					Timestamp:  time.Now(),
				})
				g.circuit.RecordSuccess()
				g.telemetry.RecordSuccess(time.Since(startTime))
				resp.RetryCount = attempt
				return resp, nil
			}

			// HTTP error - check if retryable
			if g.isRetryableStatus(resp.StatusCode) {
				g.circuit.RecordFailure()
				g.telemetry.RecordFailure(resp.StatusCode)
				lastErr = &ProviderError{
					Provider:   g.config.Name,
					StatusCode: resp.StatusCode,
					Message:    "HTTP error",
					Retryable:  true,
					RetryAfter: g.extractRetryAfter(resp.Headers),
				}
				continue
			}

			// Non-retryable HTTP error
			g.circuit.RecordFailure()
			g.telemetry.RecordFailure(resp.StatusCode)
			return resp, &ProviderError{
				Provider:   g.config.Name,
				StatusCode: resp.StatusCode,
				Message:    "non-retryable HTTP error",
				Retryable:  false,
			}
		}

		// Network or other error
		g.circuit.RecordFailure()
		g.telemetry.RecordError()
		lastErr = &ProviderError{
			Provider:  g.config.Name,
			Message:   fmt.Sprintf("request failed: %v", err),
			Retryable: true,
		}
	}

	return nil, lastErr
}

// calculateBackoff returns exponential backoff with jitter
func (g *ProviderGuard) calculateBackoff(attempt int) time.Duration {
	baseMs := g.config.BackoffBaseMs
	if baseMs <= 0 {
		baseMs = 100 // Default 100ms
	}

	// Exponential backoff: base * 2^attempt
	backoffMs := baseMs * (1 << uint(attempt-1))

	// Cap at 30 seconds
	if backoffMs > 30000 {
		backoffMs = 30000
	}

	// Add jitter (±25%)
	jitter := int(float64(backoffMs) * 0.25)
	if jitter > 0 {
		backoffMs += (backoffMs % (jitter * 2)) - jitter
	}

	return time.Duration(backoffMs) * time.Millisecond
}

// isRetryableStatus checks if an HTTP status code should trigger a retry
func (g *ProviderGuard) isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case 429, // Too Many Requests
		500, 502, 503, 504: // Server errors
		return true
	default:
		return false
	}
}

// extractRetryAfter parses Retry-After header
func (g *ProviderGuard) extractRetryAfter(headers http.Header) time.Duration {
	retryAfter := headers.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Parse as seconds (most common format)
	if seconds, err := time.ParseDuration(retryAfter + "s"); err == nil {
		return seconds
	}

	return 0
}

// Cache returns the provider guard's cache instance for external access
func (g *ProviderGuard) Cache() *Cache {
	return g.cache
}

// Health returns the current health status of the provider
func (g *ProviderGuard) Health() ProviderHealth {
	return ProviderHealth{
		Provider:     g.config.Name,
		CircuitOpen:  g.circuit.IsOpen(),
		CacheHitRate: g.telemetry.CacheHitRate(),
		RequestCount: g.telemetry.RequestCount(),
		ErrorRate:    g.telemetry.ErrorRate(),
		AvgLatency:   g.telemetry.AvgLatency(),
		LastSuccess:  g.telemetry.LastSuccess(),
		LastFailure:  g.telemetry.LastFailure(),
	}
}

// ProviderHealth represents the health status of a provider
type ProviderHealth struct {
	Provider     string        `json:"provider"`
	CircuitOpen  bool          `json:"circuit_open"`
	CacheHitRate float64       `json:"cache_hit_rate"`
	RequestCount int64         `json:"request_count"`
	ErrorRate    float64       `json:"error_rate"`
	AvgLatency   time.Duration `json:"avg_latency"`
	LastSuccess  time.Time     `json:"last_success"`
	LastFailure  time.Time     `json:"last_failure"`
}
