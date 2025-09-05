package limits

import (
    "sync"
    "time"
)

// Simple token bucket per key: 60 rpm
type PerKeyLimiter struct{ mu sync.Mutex; last map[string]time.Time; interval time.Duration }

func NewPerKeyLimiter() *PerKeyLimiter { return &PerKeyLimiter{last: make(map[string]time.Time), interval: time.Minute/60} }

func (l *PerKeyLimiter) Allow(key string) bool {
    l.mu.Lock(); defer l.mu.Unlock()
    now := time.Now()
    if t, ok := l.last[key]; ok {
        if now.Sub(t) < l.interval { return false }
    }
    l.last[key] = now
    return true
}

