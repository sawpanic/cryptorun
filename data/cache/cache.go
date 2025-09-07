package cache

import (
	"os"
	"sync"
	"time"

	"context"
	redis "github.com/redis/go-redis/v9"
)

type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, val []byte, ttl time.Duration)
}

type memory struct {
	mu sync.Mutex
	m  map[string]entry
}
type entry struct {
	b   []byte
	exp time.Time
}

func New() Cache { return &memory{m: make(map[string]entry)} }

func (c *memory) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || (!e.exp.IsZero() && time.Now().After(e.exp)) {
		return nil, false
	}
	return e.b, true
}
func (c *memory) Set(key string, val []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := entry{b: append([]byte(nil), val...)}
	if ttl > 0 {
		e.exp = time.Now().Add(ttl)
	}
	c.m[key] = e
}

// Optional Redis adapter when REDIS_ADDR is set
type redisCache struct{ r *redis.Client }

func NewAuto() Cache {
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		return &redisCache{r: redis.NewClient(&redis.Options{Addr: addr})}
	}
	return New()
}

func (r *redisCache) Get(key string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	v, err := r.r.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	return v, true
}
func (r *redisCache) Set(key string, val []byte, ttl time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = r.r.Set(ctx, key, val, ttl).Err()
}
