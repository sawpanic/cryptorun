package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter provides per-host rate limiting using token bucket algorithm
type Limiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	rps      float64 // Requests per second
	burst    int     // Burst capacity
}

// NewLimiter creates a new rate limiter with the specified RPS and burst capacity
func NewLimiter(rps float64, burst int) *Limiter {
	return &Limiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rps,
		burst:    burst,
	}
}

// getLimiter returns or creates a rate limiter for the specified host
func (l *Limiter) getLimiter(host string) *rate.Limiter {
	l.mu.RLock()
	limiter, exists := l.limiters[host]
	l.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create new limiter with write lock
	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := l.limiters[host]; exists {
		return limiter
	}

	// Create new rate limiter for this host
	limiter = rate.NewLimiter(rate.Limit(l.rps), l.burst)
	l.limiters[host] = limiter
	return limiter
}

// Allow returns true if a request for the specified host is allowed
func (l *Limiter) Allow(host string) bool {
	limiter := l.getLimiter(host)
	return limiter.Allow()
}

// Wait blocks until a request for the specified host is allowed or context is cancelled
func (l *Limiter) Wait(ctx context.Context, host string) error {
	limiter := l.getLimiter(host)
	return limiter.Wait(ctx)
}

// Reserve reserves a token for the specified host and returns a Reservation
func (l *Limiter) Reserve(host string) *rate.Reservation {
	limiter := l.getLimiter(host)
	return limiter.Reserve()
}

// SetRPS updates the requests per second for all limiters
func (l *Limiter) SetRPS(rps float64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.rps = rps
	for _, limiter := range l.limiters {
		limiter.SetLimit(rate.Limit(rps))
	}
}

// SetBurst updates the burst capacity for all limiters
func (l *Limiter) SetBurst(burst int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.burst = burst
	for _, limiter := range l.limiters {
		limiter.SetBurst(burst)
	}
}

// Stats returns statistics for all host limiters
func (l *Limiter) Stats() map[string]LimiterStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := make(map[string]LimiterStats)
	now := time.Now()

	for host, limiter := range l.limiters {
		reservation := limiter.Reserve()
		delay := reservation.Delay()
		reservation.Cancel() // Cancel the reservation since we're just checking

		stats[host] = LimiterStats{
			Host:            host,
			RPS:             float64(limiter.Limit()),
			Burst:           limiter.Burst(),
			TokensAvailable: limiter.Tokens(),
			NextAllowedAt:   now.Add(delay),
			Delay:           delay,
		}
	}

	return stats
}

// Reset clears all host limiters
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.limiters = make(map[string]*rate.Limiter)
}

// LimiterStats represents statistics for a single host limiter
type LimiterStats struct {
	Host            string        `json:"host"`
	RPS             float64       `json:"rps"`
	Burst           int           `json:"burst"`
	TokensAvailable float64       `json:"tokens_available"`
	NextAllowedAt   time.Time     `json:"next_allowed_at"`
	Delay           time.Duration `json:"delay"`
}

// IsThrottled returns true if the limiter is currently throttling requests
func (s *LimiterStats) IsThrottled() bool {
	return s.Delay > 0
}

// Manager manages multiple rate limiters for different providers
type Manager struct {
	limiters map[string]*Limiter
	mu       sync.RWMutex
}

// NewManager creates a new rate limiter manager
func NewManager() *Manager {
	return &Manager{
		limiters: make(map[string]*Limiter),
	}
}

// AddProvider adds a rate limiter for a specific provider
func (m *Manager) AddProvider(name string, rps float64, burst int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.limiters[name] = NewLimiter(rps, burst)
}

// GetLimiter returns the rate limiter for a specific provider
func (m *Manager) GetLimiter(provider string) (*Limiter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limiter, exists := m.limiters[provider]
	return limiter, exists
}

// Allow returns true if a request is allowed for the specified provider and host
func (m *Manager) Allow(provider, host string) bool {
	limiter, exists := m.GetLimiter(provider)
	if !exists {
		return true // No limiter configured, allow request
	}
	return limiter.Allow(host)
}

// Wait blocks until a request is allowed for the specified provider and host
func (m *Manager) Wait(ctx context.Context, provider, host string) error {
	limiter, exists := m.GetLimiter(provider)
	if !exists {
		return nil // No limiter configured, allow immediately
	}
	return limiter.Wait(ctx, host)
}

// Stats returns statistics for all providers and their hosts
func (m *Manager) Stats() map[string]map[string]LimiterStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]map[string]LimiterStats)
	for provider, limiter := range m.limiters {
		stats[provider] = limiter.Stats()
	}
	return stats
}

// Reset clears all rate limiters
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, limiter := range m.limiters {
		limiter.Reset()
	}
}
