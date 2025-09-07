package async

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionPool manages HTTP client connections with pooling and circuit breaking
type ConnectionPool struct {
	clients         map[string]*PooledClient
	config          PoolConfig
	metrics         *PoolMetrics
	mu              sync.RWMutex
	circuitBreakers map[string]*CircuitBreaker
}

// PoolConfig defines connection pool configuration
type PoolConfig struct {
	MaxIdleConns        int           // Maximum idle connections per host
	MaxIdleConnsPerHost int           // Maximum idle connections total
	IdleConnTimeout     time.Duration // How long idle connections are kept
	DialTimeout         time.Duration // Connection dial timeout
	RequestTimeout      time.Duration // Per-request timeout
	TLSHandshakeTimeout time.Duration // TLS handshake timeout
	MaxRetries          int           // Maximum retry attempts
	RetryBackoff        time.Duration // Base retry backoff
	KeepAlive           time.Duration // TCP keep-alive interval
	DisableCompression  bool          // Disable gzip compression
}

// DefaultPoolConfig returns production-ready pool configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialTimeout:         10 * time.Second,
		RequestTimeout:      30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		MaxRetries:          3,
		RetryBackoff:        100 * time.Millisecond,
		KeepAlive:           30 * time.Second,
		DisableCompression:  false,
	}
}

// PooledClient wraps an HTTP client with additional metadata
type PooledClient struct {
	Client      *http.Client
	Host        string
	CreatedAt   time.Time
	RequestCount int64
	ErrorCount   int64
	LastUsed     time.Time
	mu           sync.RWMutex
}

// PoolMetrics tracks connection pool performance
type PoolMetrics struct {
	TotalRequests     int64
	TotalErrors       int64
	TotalRetries      int64
	ConnectionsCreated int64
	ConnectionsReused int64
	ActiveConnections int64
	PoolHits          int64
	PoolMisses        int64
	
	// Per-host metrics
	HostMetrics map[string]*HostMetrics
	mu          sync.RWMutex
}

// HostMetrics tracks per-host statistics
type HostMetrics struct {
	Requests     int64
	Errors       int64
	AvgLatency   time.Duration
	LastRequest  time.Time
	CircuitState string
}

// CircuitBreaker provides circuit breaker functionality for HTTP requests
type CircuitBreaker struct {
	failureCount    int64
	successCount    int64
	lastFailureTime time.Time
	state           CircuitState
	mu              sync.RWMutex
	config          CircuitConfig
}

// CircuitState represents circuit breaker states
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitHalfOpen
	CircuitOpen
)

// CircuitConfig defines circuit breaker behavior
type CircuitConfig struct {
	FailureThreshold int           // Number of failures to open circuit
	SuccessThreshold int           // Number of successes to close circuit
	Timeout          time.Duration // How long circuit stays open
}

// NewConnectionPool creates a new HTTP connection pool
func NewConnectionPool(config PoolConfig) *ConnectionPool {
	return &ConnectionPool{
		clients:         make(map[string]*PooledClient),
		config:          config,
		metrics:         &PoolMetrics{HostMetrics: make(map[string]*HostMetrics)},
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
}

// GetClient returns a pooled HTTP client for the specified host
func (cp *ConnectionPool) GetClient(host string) *PooledClient {
	cp.mu.RLock()
	client, exists := cp.clients[host]
	cp.mu.RUnlock()
	
	if exists {
		atomic.AddInt64(&cp.metrics.PoolHits, 1)
		client.updateLastUsed()
		return client
	}
	
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	// Double-check after acquiring write lock
	if client, exists := cp.clients[host]; exists {
		atomic.AddInt64(&cp.metrics.PoolHits, 1)
		client.updateLastUsed()
		return client
	}
	
	// Create new client
	client = cp.createClient(host)
	cp.clients[host] = client
	atomic.AddInt64(&cp.metrics.PoolMisses, 1)
	atomic.AddInt64(&cp.metrics.ConnectionsCreated, 1)
	
	return client
}

// createClient creates a new HTTP client with optimized transport
func (cp *ConnectionPool) createClient(host string) *PooledClient {
	// Create custom dialer
	dialer := &net.Dialer{
		Timeout:   cp.config.DialTimeout,
		KeepAlive: cp.config.KeepAlive,
		DualStack: true,
	}
	
	// Create custom transport
	transport := &http.Transport{
		DialContext: dialer.DialContext,
		
		// Connection pooling settings
		MaxIdleConns:        cp.config.MaxIdleConns,
		MaxIdleConnsPerHost: cp.config.MaxIdleConnsPerHost,
		IdleConnTimeout:     cp.config.IdleConnTimeout,
		
		// TLS settings
		TLSHandshakeTimeout: cp.config.TLSHandshakeTimeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
		
		// Performance settings
		DisableCompression: cp.config.DisableCompression,
		ForceAttemptHTTP2:  true,
		
		// Proxy settings
		Proxy: http.ProxyFromEnvironment,
		
		// Response header timeout
		ResponseHeaderTimeout: cp.config.RequestTimeout / 2,
		
		// Expect continue timeout
		ExpectContinueTimeout: 1 * time.Second,
	}
	
	// Create HTTP client
	client := &http.Client{
		Transport: transport,
		Timeout:   cp.config.RequestTimeout,
		
		// Custom redirect policy
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	
	return &PooledClient{
		Client:    client,
		Host:      host,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}
}

// DoRequest performs an HTTP request with retry logic and circuit breaking
func (cp *ConnectionPool) DoRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	host := req.URL.Host
	client := cp.GetClient(host)
	
	// Check circuit breaker
	if !cp.canMakeRequest(host) {
		return nil, fmt.Errorf("circuit breaker is open for host %s", host)
	}
	
	start := time.Now()
	var lastErr error
	
	// Retry loop
	for attempt := 0; attempt <= cp.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Apply backoff
			backoff := time.Duration(attempt) * cp.config.RetryBackoff
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			
			atomic.AddInt64(&cp.metrics.TotalRetries, 1)
		}
		
		// Clone request for retry safety
		reqClone := req.Clone(ctx)
		
		// Execute request
		resp, err := client.Client.Do(reqClone)
		
		// Update client metrics
		atomic.AddInt64(&client.RequestCount, 1)
		atomic.AddInt64(&cp.metrics.TotalRequests, 1)
		
		if err == nil && resp.StatusCode < 500 {
			// Success - update circuit breaker
			cp.recordSuccess(host)
			cp.updateHostMetrics(host, time.Since(start), false)
			return resp, nil
		}
		
		// Record error
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			resp.Body.Close()
		}
		
		atomic.AddInt64(&client.ErrorCount, 1)
		atomic.AddInt64(&cp.metrics.TotalErrors, 1)
		
		// Don't retry certain errors
		if !cp.shouldRetry(lastErr, resp) {
			break
		}
	}
	
	// All retries exhausted - record failure
	cp.recordFailure(host)
	cp.updateHostMetrics(host, time.Since(start), true)
	
	return nil, fmt.Errorf("request failed after %d attempts: %w", cp.config.MaxRetries+1, lastErr)
}

// shouldRetry determines if a request should be retried
func (cp *ConnectionPool) shouldRetry(err error, resp *http.Response) bool {
	// Don't retry context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	
	// Don't retry client errors (4xx)
	if resp != nil && resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return false
	}
	
	// Retry network errors and server errors (5xx)
	return true
}

// canMakeRequest checks if circuit breaker allows the request
func (cp *ConnectionPool) canMakeRequest(host string) bool {
	cp.mu.RLock()
	breaker, exists := cp.circuitBreakers[host]
	cp.mu.RUnlock()
	
	if !exists {
		// Create new circuit breaker
		cp.mu.Lock()
		if breaker, exists = cp.circuitBreakers[host]; !exists {
			breaker = &CircuitBreaker{
				state: CircuitClosed,
				config: CircuitConfig{
					FailureThreshold: 5,
					SuccessThreshold: 3,
					Timeout:          60 * time.Second,
				},
			}
			cp.circuitBreakers[host] = breaker
		}
		cp.mu.Unlock()
	}
	
	breaker.mu.RLock()
	defer breaker.mu.RUnlock()
	
	switch breaker.state {
	case CircuitClosed:
		return true
	case CircuitHalfOpen:
		return true
	case CircuitOpen:
		// Check if timeout has elapsed
		if time.Since(breaker.lastFailureTime) > breaker.config.Timeout {
			// Transition to half-open
			breaker.mu.RUnlock()
			breaker.mu.Lock()
			breaker.state = CircuitHalfOpen
			breaker.mu.Unlock()
			breaker.mu.RLock()
			return true
		}
		return false
	default:
		return false
	}
}

// recordSuccess records a successful request for circuit breaker
func (cp *ConnectionPool) recordSuccess(host string) {
	cp.mu.RLock()
	breaker, exists := cp.circuitBreakers[host]
	cp.mu.RUnlock()
	
	if !exists {
		return
	}
	
	breaker.mu.Lock()
	defer breaker.mu.Unlock()
	
	atomic.AddInt64(&breaker.successCount, 1)
	
	if breaker.state == CircuitHalfOpen {
		if breaker.successCount >= int64(breaker.config.SuccessThreshold) {
			breaker.state = CircuitClosed
			breaker.failureCount = 0
			breaker.successCount = 0
		}
	}
}

// recordFailure records a failed request for circuit breaker
func (cp *ConnectionPool) recordFailure(host string) {
	cp.mu.RLock()
	breaker, exists := cp.circuitBreakers[host]
	cp.mu.RUnlock()
	
	if !exists {
		return
	}
	
	breaker.mu.Lock()
	defer breaker.mu.Unlock()
	
	atomic.AddInt64(&breaker.failureCount, 1)
	breaker.lastFailureTime = time.Now()
	
	if breaker.state == CircuitClosed || breaker.state == CircuitHalfOpen {
		if breaker.failureCount >= int64(breaker.config.FailureThreshold) {
			breaker.state = CircuitOpen
		}
	}
}

// updateHostMetrics updates per-host performance metrics
func (cp *ConnectionPool) updateHostMetrics(host string, latency time.Duration, isError bool) {
	cp.metrics.mu.Lock()
	defer cp.metrics.mu.Unlock()
	
	hostMetrics, exists := cp.metrics.HostMetrics[host]
	if !exists {
		hostMetrics = &HostMetrics{}
		cp.metrics.HostMetrics[host] = hostMetrics
	}
	
	atomic.AddInt64(&hostMetrics.Requests, 1)
	hostMetrics.LastRequest = time.Now()
	
	if isError {
		atomic.AddInt64(&hostMetrics.Errors, 1)
	}
	
	// Update average latency (exponential moving average)
	if hostMetrics.AvgLatency == 0 {
		hostMetrics.AvgLatency = latency
	} else {
		// Weight: 90% old, 10% new
		hostMetrics.AvgLatency = time.Duration(
			float64(hostMetrics.AvgLatency)*0.9 + float64(latency)*0.1,
		)
	}
	
	// Update circuit state
	cp.mu.RLock()
	if breaker, exists := cp.circuitBreakers[host]; exists {
		breaker.mu.RLock()
		switch breaker.state {
		case CircuitClosed:
			hostMetrics.CircuitState = "closed"
		case CircuitHalfOpen:
			hostMetrics.CircuitState = "half-open"
		case CircuitOpen:
			hostMetrics.CircuitState = "open"
		}
		breaker.mu.RUnlock()
	}
	cp.mu.RUnlock()
}

// updateLastUsed updates the last used timestamp for a client
func (pc *PooledClient) updateLastUsed() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.LastUsed = time.Now()
}

// GetMetrics returns current pool metrics
func (cp *ConnectionPool) GetMetrics() PoolMetrics {
	cp.metrics.mu.RLock()
	defer cp.metrics.mu.RUnlock()
	
	// Create deep copy
	metrics := PoolMetrics{
		TotalRequests:      atomic.LoadInt64(&cp.metrics.TotalRequests),
		TotalErrors:        atomic.LoadInt64(&cp.metrics.TotalErrors),
		TotalRetries:       atomic.LoadInt64(&cp.metrics.TotalRetries),
		ConnectionsCreated: atomic.LoadInt64(&cp.metrics.ConnectionsCreated),
		ConnectionsReused:  atomic.LoadInt64(&cp.metrics.ConnectionsReused),
		ActiveConnections:  atomic.LoadInt64(&cp.metrics.ActiveConnections),
		PoolHits:           atomic.LoadInt64(&cp.metrics.PoolHits),
		PoolMisses:         atomic.LoadInt64(&cp.metrics.PoolMisses),
		HostMetrics:        make(map[string]*HostMetrics),
	}
	
	for host, hostMetrics := range cp.metrics.HostMetrics {
		metrics.HostMetrics[host] = &HostMetrics{
			Requests:     atomic.LoadInt64(&hostMetrics.Requests),
			Errors:       atomic.LoadInt64(&hostMetrics.Errors),
			AvgLatency:   hostMetrics.AvgLatency,
			LastRequest:  hostMetrics.LastRequest,
			CircuitState: hostMetrics.CircuitState,
		}
	}
	
	return metrics
}

// Cleanup removes idle connections and updates metrics
func (cp *ConnectionPool) Cleanup() {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	now := time.Now()
	activeConns := int64(0)
	
	for host, client := range cp.clients {
		client.mu.RLock()
		lastUsed := client.LastUsed
		client.mu.RUnlock()
		
		// Remove idle connections
		if now.Sub(lastUsed) > cp.config.IdleConnTimeout {
			client.Client.CloseIdleConnections()
			delete(cp.clients, host)
		} else {
			activeConns++
		}
	}
	
	atomic.StoreInt64(&cp.metrics.ActiveConnections, activeConns)
}

// Close closes all pooled connections
func (cp *ConnectionPool) Close() {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	for _, client := range cp.clients {
		client.Client.CloseIdleConnections()
	}
	
	cp.clients = make(map[string]*PooledClient)
	atomic.StoreInt64(&cp.metrics.ActiveConnections, 0)
}