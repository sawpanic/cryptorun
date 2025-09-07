package httpclient

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type ClientConfig struct {
	MaxConcurrency int
	RequestTimeout time.Duration
	JitterRange    [2]int // Min/max jitter in milliseconds
	MaxRetries     int
	BackoffBase    time.Duration
	BackoffMax     time.Duration
	UserAgent      string
}

type ClientPool struct {
	config    ClientConfig
	semaphore chan struct{}
	client    *http.Client
	mu        sync.RWMutex
	stats     ClientStats
}

type ClientStats struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TimeoutRequests int64
	RetriedRequests int64
	TotalLatency    time.Duration
	P50Latency      time.Duration
	P95Latency      time.Duration
}

func NewClientPool(config ClientConfig) *ClientPool {
	return &ClientPool{
		config:    config,
		semaphore: make(chan struct{}, config.MaxConcurrency),
		client: &http.Client{
			Timeout: config.RequestTimeout,
		},
	}
}

func (cp *ClientPool) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	startTime := time.Now()

	// Apply concurrency limit
	select {
	case cp.semaphore <- struct{}{}:
		defer func() { <-cp.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Add user agent if configured
	if cp.config.UserAgent != "" {
		req.Header.Set("User-Agent", cp.config.UserAgent)
	}

	// Apply jitter before request
	if err := cp.applyJitter(ctx); err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= cp.config.MaxRetries; attempt++ {
		if attempt > 0 {
			cp.incrementStat("retried")

			// Apply exponential backoff
			backoff := cp.calculateBackoff(attempt)
			log.Debug().
				Dur("backoff", backoff).
				Int("attempt", attempt).
				Str("url", req.URL.String()).
				Msg("Retrying HTTP request")

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err := cp.client.Do(req.WithContext(ctx))

		duration := time.Since(startTime)
		cp.recordLatency(duration)

		if err != nil {
			lastErr = err
			cp.incrementStat("failed")

			if isRetryableError(err) {
				continue
			}
			break
		}

		// Check for retryable HTTP status codes
		if isRetryableStatus(resp.StatusCode) && attempt < cp.config.MaxRetries {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			continue
		}

		cp.incrementStat("success")
		return resp, nil
	}

	cp.incrementStat("failed")
	return nil, lastErr
}

func (cp *ClientPool) applyJitter(ctx context.Context) error {
	if cp.config.JitterRange[0] >= cp.config.JitterRange[1] {
		return nil // No jitter configured
	}

	min := cp.config.JitterRange[0]
	max := cp.config.JitterRange[1]
	jitter := time.Duration(rand.Intn(max-min)+min) * time.Millisecond

	select {
	case <-time.After(jitter):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cp *ClientPool) calculateBackoff(attempt int) time.Duration {
	backoff := cp.config.BackoffBase * time.Duration(1<<uint(attempt))
	if backoff > cp.config.BackoffMax {
		backoff = cp.config.BackoffMax
	}

	// Add up to 10% jitter to backoff
	jitter := time.Duration(rand.Float64() * 0.1 * float64(backoff))
	return backoff + jitter
}

func (cp *ClientPool) GetStats() ClientStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.stats
}

func (cp *ClientPool) incrementStat(statType string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.stats.TotalRequests++

	switch statType {
	case "success":
		cp.stats.SuccessRequests++
	case "failed":
		cp.stats.FailedRequests++
	case "timeout":
		cp.stats.TimeoutRequests++
	case "retried":
		cp.stats.RetriedRequests++
	}
}

func (cp *ClientPool) recordLatency(duration time.Duration) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.stats.TotalLatency += duration

	// Simple percentile tracking - in production would use histogram
	if cp.stats.TotalRequests == 0 {
		cp.stats.P50Latency = duration
		cp.stats.P95Latency = duration
	} else {
		// Exponential moving average approximation
		alpha := 0.1
		cp.stats.P50Latency = time.Duration(float64(cp.stats.P50Latency)*(1-alpha) + float64(duration)*alpha)

		// P95 uses slower decay
		alpha95 := 0.05
		if duration > cp.stats.P95Latency {
			alpha95 = 0.2 // React faster to higher latencies
		}
		cp.stats.P95Latency = time.Duration(float64(cp.stats.P95Latency)*(1-alpha95) + float64(duration)*alpha95)
	}
}

func isRetryableError(err error) bool {
	// Network errors, timeouts, etc. are retryable
	if err == nil {
		return false
	}

	errStr := err.Error()
	retryableErrors := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"network is unreachable",
		"no such host",
	}

	for _, retryable := range retryableErrors {
		if containsIgnoreCase(errStr, retryable) {
			return true
		}
	}

	return false
}

func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case 429, // Too Many Requests
		502, // Bad Gateway
		503, // Service Unavailable
		504: // Gateway Timeout
		return true
	}
	return false
}

func containsIgnoreCase(haystack, needle string) bool {
	// Simple case-insensitive substring check
	haystack = toLower(haystack)
	needle = toLower(needle)

	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i, b := range []byte(s) {
		if b >= 'A' && b <= 'Z' {
			result[i] = b + 32
		} else {
			result[i] = b
		}
	}
	return string(result)
}
