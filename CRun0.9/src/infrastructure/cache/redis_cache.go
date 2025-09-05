package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct { c *redis.Client; ttl time.Duration }

func NewRedis(addr string, db int, ttl time.Duration) *RedisCache {
	return &RedisCache{ c: redis.NewClient(&redis.Options{Addr: addr, DB: db}), ttl: ttl }
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return r.c.Get(ctx, key).Result()
}

func (r *RedisCache) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	if ttl == 0 { ttl = r.ttl }
	return r.c.Set(ctx, key, val, ttl).Err()
}
