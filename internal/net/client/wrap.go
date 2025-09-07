package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cryptorun/internal/config"
	"cryptorun/internal/net/budget"
	"cryptorun/internal/net/circuit"
	"cryptorun/internal/net/ratelimit"
)

// WrapperConfig configures the HTTP client wrapper
type WrapperConfig struct {
	Provider       string
	ProviderConfig *config.ProviderConfig
	RateLimiter    *ratelimit.Limiter
	CircuitBreaker *circuit.Breaker
	BudgetTracker  *budget.Tracker
	Cache          Cache // Optional cache interface
}

// Cache interface for caching HTTP responses
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration)
}

// Wrapper wraps an HTTP RoundTripper with rate limiting, circuit breaking, budgets, and caching
type Wrapper struct {
	config    WrapperConfig
	transport http.RoundTripper
	userAgent string
}

// NewWrapper creates a new HTTP client wrapper with all middleware
func NewWrapper(config WrapperConfig, transport http.RoundTripper) *Wrapper {
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &Wrapper{
		config:    config,
		transport: transport,
		userAgent: "CryptoRun/3.2.1 (Free-tier; respect-robots.txt)",
	}
}

// RoundTrip implements http.RoundTripper with full middleware stack
func (w *Wrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Set user agent
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", w.userAgent)
	}

	// Check cache first (if enabled and GET request)
	if w.config.Cache != nil && req.Method == "GET" {
		cacheKey := w.getCacheKey(req)
		if data, found := w.config.Cache.Get(req.Context(), cacheKey); found {
			return w.createCachedResponse(req, data), nil
		}
	}

	// Check budget first (fail fast if exhausted)
	if w.config.BudgetTracker != nil {
		if err := w.config.BudgetTracker.Allow(); err != nil {
			return nil, &ProviderError{
				Provider: w.config.Provider,
				Type:     "budget",
				Err:      err,
			}
		}
	}

	// Rate limiting
	if w.config.RateLimiter != nil {
		if err := w.config.RateLimiter.Wait(req.Context(), w.config.ProviderConfig.Host); err != nil {
			return nil, &ProviderError{
				Provider: w.config.Provider,
				Type:     "rate_limit",
				Err:      fmt.Errorf("rate limit wait failed: %w", err),
			}
		}
	}

	// Execute request through circuit breaker
	var response *http.Response
	var requestErr error

	executeRequest := func(ctx context.Context) error {
		// Consume budget (after rate limiting passes)
		if w.config.BudgetTracker != nil {
			if err := w.config.BudgetTracker.Consume(); err != nil {
				// Budget warning is not a hard failure, but budget exhaustion is
				if _, isExhausted := err.(*budget.BudgetExhaustedError); isExhausted {
					return &ProviderError{
						Provider: w.config.Provider,
						Type:     "budget",
						Err:      err,
					}
				}
				// Log warning but continue (budget warning)
			}
		}

		// Execute HTTP request
		response, requestErr = w.transport.RoundTrip(req.WithContext(ctx))
		if requestErr != nil {
			return &ProviderError{
				Provider: w.config.Provider,
				Type:     "transport",
				Err:      requestErr,
			}
		}

		// Check for HTTP error status
		if response.StatusCode >= 400 {
			return &ProviderError{
				Provider:   w.config.Provider,
				Type:       "http_error",
				StatusCode: response.StatusCode,
				Err:        fmt.Errorf("HTTP %d error", response.StatusCode),
			}
		}

		return nil
	}

	// Use circuit breaker if configured
	var err error
	if w.config.CircuitBreaker != nil {
		err = w.config.CircuitBreaker.Call(req.Context(), executeRequest)
	} else {
		err = executeRequest(req.Context())
	}

	if err != nil {
		return nil, err
	}

	// Cache successful response (if enabled and cacheable)
	if w.config.Cache != nil && req.Method == "GET" && response.StatusCode == 200 {
		w.cacheResponse(req, response)
	}

	return response, nil
}

// getCacheKey generates a cache key for the request
func (w *Wrapper) getCacheKey(req *http.Request) string {
	return fmt.Sprintf("%s:%s:%s", w.config.Provider, req.Method, req.URL.String())
}

// createCachedResponse creates an HTTP response from cached data
func (w *Wrapper) createCachedResponse(req *http.Request, data []byte) *http.Response {
	// This is a simplified implementation - in practice you'd want to cache headers too
	return &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       &cachedResponseBody{data: data},
		Request:    req,
	}
}

// cacheResponse stores the response in cache
func (w *Wrapper) cacheResponse(req *http.Request, resp *http.Response) {
	// This is a simplified implementation - in practice you'd want to cache more response data
	cacheKey := w.getCacheKey(req)
	ttl := w.config.ProviderConfig.GetCacheTTL()

	// Note: This would need to read and cache the response body appropriately
	// For now, this is a placeholder
	w.config.Cache.Set(req.Context(), cacheKey, []byte{}, ttl)
}

// cachedResponseBody implements io.ReadCloser for cached response data
type cachedResponseBody struct {
	data []byte
	pos  int
}

func (c *cachedResponseBody) Read(p []byte) (int, error) {
	if c.pos >= len(c.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, c.data[c.pos:])
	c.pos += n
	return n, nil
}

func (c *cachedResponseBody) Close() error {
	return nil
}

// ProviderError represents an error from a provider with context
type ProviderError struct {
	Provider   string `json:"provider"`
	Type       string `json:"type"` // "rate_limit", "budget", "circuit", "transport", "http_error"
	StatusCode int    `json:"status_code,omitempty"`
	Err        error  `json:"-"`
}

func (e *ProviderError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("provider %s %s error (HTTP %d): %v", e.Provider, e.Type, e.StatusCode, e.Err)
	}
	return fmt.Sprintf("provider %s %s error: %v", e.Provider, e.Type, e.Err)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// IsRateLimited returns true if the error is due to rate limiting
func (e *ProviderError) IsRateLimited() bool {
	return e.Type == "rate_limit"
}

// IsBudgetExhausted returns true if the error is due to budget exhaustion
func (e *ProviderError) IsBudgetExhausted() bool {
	return e.Type == "budget"
}

// IsCircuitOpen returns true if the error is due to circuit breaker being open
func (e *ProviderError) IsCircuitOpen() bool {
	return e.Type == "circuit"
}

// GetUserFriendlyReason returns a user-friendly explanation of the error
func (e *ProviderError) GetUserFriendlyReason() string {
	switch e.Type {
	case "rate_limit":
		return fmt.Sprintf("Rate limited by %s - too many requests", e.Provider)
	case "budget":
		if budgetErr, ok := e.Err.(*budget.BudgetExhaustedError); ok {
			return fmt.Sprintf("Daily budget exhausted for %s, resets at %s",
				e.Provider, budgetErr.ETA.Format("15:04 UTC"))
		}
		if budgetWarn, ok := e.Err.(*budget.BudgetWarningError); ok {
			return fmt.Sprintf("Budget warning for %s: %.1f%% used",
				e.Provider, float64(budgetWarn.Used)/float64(budgetWarn.Limit)*100)
		}
		return fmt.Sprintf("Budget issue with %s", e.Provider)
	case "circuit":
		return fmt.Sprintf("Service %s temporarily unavailable (circuit breaker open)", e.Provider)
	case "http_error":
		return fmt.Sprintf("HTTP error from %s (status %d)", e.Provider, e.StatusCode)
	case "transport":
		return fmt.Sprintf("Network error connecting to %s", e.Provider)
	default:
		return fmt.Sprintf("Error from provider %s", e.Provider)
	}
}

// Manager manages wrapped HTTP clients for multiple providers
type Manager struct {
	clients      map[string]*http.Client
	rateLimitMgr *ratelimit.Manager
	circuitMgr   *circuit.Manager
	budgetMgr    *budget.Manager
	cache        Cache
	globalConfig *config.GlobalConfig
}

// NewManager creates a new client manager
func NewManager(rateLimitMgr *ratelimit.Manager, circuitMgr *circuit.Manager, budgetMgr *budget.Manager, cache Cache, globalConfig *config.GlobalConfig) *Manager {
	return &Manager{
		clients:      make(map[string]*http.Client),
		rateLimitMgr: rateLimitMgr,
		circuitMgr:   circuitMgr,
		budgetMgr:    budgetMgr,
		cache:        cache,
		globalConfig: globalConfig,
	}
}

// AddProvider creates a wrapped HTTP client for a provider
func (m *Manager) AddProvider(name string, providerConfig *config.ProviderConfig) {
	// Get components for this provider
	rateLimiter, _ := m.rateLimitMgr.GetLimiter(name)
	circuitBreaker, _ := m.circuitMgr.GetBreaker(name)
	budgetTracker, _ := m.budgetMgr.GetTracker(name)

	// Create wrapper configuration
	wrapperConfig := WrapperConfig{
		Provider:       name,
		ProviderConfig: providerConfig,
		RateLimiter:    rateLimiter,
		CircuitBreaker: circuitBreaker,
		BudgetTracker:  budgetTracker,
		Cache:          m.cache,
	}

	// Create wrapped transport
	wrapper := NewWrapper(wrapperConfig, http.DefaultTransport)

	// Create HTTP client with wrapped transport
	client := &http.Client{
		Transport: wrapper,
		Timeout:   providerConfig.GetRequestTimeout(),
	}

	m.clients[name] = client
}

// GetClient returns the HTTP client for a specific provider
func (m *Manager) GetClient(provider string) (*http.Client, bool) {
	client, exists := m.clients[provider]
	return client, exists
}

// GetStats returns comprehensive statistics for all providers
func (m *Manager) GetStats() ProviderStats {
	return ProviderStats{
		RateLimit: m.rateLimitMgr.Stats(),
		Circuit:   m.circuitMgr.Stats(),
		Budget:    m.budgetMgr.Stats(),
	}
}

// ProviderStats represents comprehensive provider statistics
type ProviderStats struct {
	RateLimit map[string]map[string]ratelimit.LimiterStats `json:"rate_limit"`
	Circuit   map[string]circuit.Stats                     `json:"circuit"`
	Budget    map[string]budget.Stats                      `json:"budget"`
}

// GetHealthySummary returns a summary of healthy vs unhealthy providers
func (m *Manager) GetHealthySummary() HealthSummary {
	circuitStats := m.circuitMgr.Stats()
	budgetStats := m.budgetMgr.Stats()

	healthy := make([]string, 0)
	unhealthy := make([]string, 0)
	warnings := make([]string, 0)

	// Check all providers
	allProviders := make(map[string]bool)
	for provider := range circuitStats {
		allProviders[provider] = true
	}
	for provider := range budgetStats {
		allProviders[provider] = true
	}

	for provider := range allProviders {
		circuitStat := circuitStats[provider]
		budgetStat := budgetStats[provider]

		if budgetStat.IsExhausted || !circuitStat.IsHealthy() {
			unhealthy = append(unhealthy, provider)
		} else if budgetStat.IsWarning {
			warnings = append(warnings, provider)
		} else {
			healthy = append(healthy, provider)
		}
	}

	return HealthSummary{
		Healthy:   healthy,
		Unhealthy: unhealthy,
		Warnings:  warnings,
		Total:     len(allProviders),
	}
}

// HealthSummary represents overall provider health
type HealthSummary struct {
	Healthy   []string `json:"healthy"`
	Unhealthy []string `json:"unhealthy"`
	Warnings  []string `json:"warnings"`
	Total     int      `json:"total"`
}
