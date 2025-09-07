package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"
	
	"github.com/go-redis/redis/v8"
)

// RedisCache implements CacheLayer using Redis
type RedisCache struct {
	client   *redis.Client
	prefixes map[string]string
}

// NewRedisCache creates a new Redis-based cache
func NewRedisCache(addr, password string, db int, prefixes map[string]string) (*RedisCache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:               addr,
		Password:           password,
		DB:                 db,
		PoolSize:           10,
		DialTimeout:        5 * time.Second,
		ReadTimeout:        3 * time.Second,
		WriteTimeout:       3 * time.Second,
		PoolTimeout:        4 * time.Second,
		IdleTimeout:        5 * time.Minute,
		IdleCheckFrequency: 1 * time.Minute,
	})
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}
	
	return &RedisCache{
		client:   rdb,
		prefixes: prefixes,
	}, nil
}

// Get retrieves a value from cache
func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil // Cache miss
		}
		return nil, false, fmt.Errorf("redis get: %w", err)
	}
	
	return []byte(val), true, nil
}

// Set stores a value in cache with TTL
func (r *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	err := r.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	
	return nil
}

// Delete removes a key from cache
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("redis delete: %w", err)
	}
	
	return nil
}

// Clear removes all keys matching a pattern
func (r *RedisCache) Clear(ctx context.Context, pattern string) error {
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("redis keys: %w", err)
	}
	
	if len(keys) > 0 {
		err = r.client.Del(ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("redis clear: %w", err)
		}
	}
	
	return nil
}

// GetStats returns cache statistics
func (r *RedisCache) GetStats(ctx context.Context) (*interfaces.CacheStats, error) {
	info, err := r.client.Info(ctx, "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("redis info: %w", err)
	}
	
	// Parse Redis INFO output for stats
	// This is a simplified implementation
	stats := &interfaces.CacheStats{
		Hits:      0, // Would parse from info
		Misses:    0, // Would parse from info  
		Sets:      0, // Would parse from info
		Deletes:   0, // Would parse from info
		HitRate:   0.0,
		Size:      0, // Would parse from info
		ItemCount: 0, // Would parse from info
		AvgTTL:    0,
	}
	
	// Calculate hit rate
	if stats.Hits+stats.Misses > 0 {
		stats.HitRate = float64(stats.Hits) / float64(stats.Hits+stats.Misses)
	}
	
	return stats, nil
}

// GetHitRate returns the cache hit rate
func (r *RedisCache) GetHitRate(ctx context.Context) float64 {
	stats, err := r.GetStats(ctx)
	if err != nil {
		return 0.0
	}
	return stats.HitRate
}

// BuildKey constructs a cache key with proper prefix
func (r *RedisCache) BuildKey(dataType, venue, symbol string, params ...string) string {
	prefix, exists := r.prefixes[dataType]
	if !exists {
		prefix = "facade:"
	}
	
	key := fmt.Sprintf("%s%s:%s:%s", prefix, venue, symbol, dataType)
	
	for _, param := range params {
		key += ":" + param
	}
	
	return key
}

// CacheTrades stores trades with appropriate TTL
func (r *RedisCache) CacheTrades(ctx context.Context, venue, symbol string, trades []interfaces.Trade, ttl time.Duration) error {
	key := r.BuildKey("trades", venue, symbol)
	
	data, err := json.Marshal(trades)
	if err != nil {
		return fmt.Errorf("marshal trades: %w", err)
	}
	
	return r.Set(ctx, key, data, ttl)
}

// GetCachedTrades retrieves cached trades
func (r *RedisCache) GetCachedTrades(ctx context.Context, venue, symbol string) ([]interfaces.Trade, bool, error) {
	key := r.BuildKey("trades", venue, symbol)
	
	data, found, err := r.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}
	
	if !found {
		return nil, false, nil
	}
	
	var trades []interfaces.Trade
	if err := json.Unmarshal(data, &trades); err != nil {
		return nil, false, fmt.Errorf("unmarshal trades: %w", err)
	}
	
	return trades, true, nil
}

// CacheKlines stores klines with appropriate TTL
func (r *RedisCache) CacheKlines(ctx context.Context, venue, symbol, interval string, klines []interfaces.Kline, ttl time.Duration) error {
	key := r.BuildKey("klines", venue, symbol, interval)
	
	data, err := json.Marshal(klines)
	if err != nil {
		return fmt.Errorf("marshal klines: %w", err)
	}
	
	return r.Set(ctx, key, data, ttl)
}

// GetCachedKlines retrieves cached klines
func (r *RedisCache) GetCachedKlines(ctx context.Context, venue, symbol, interval string) ([]interfaces.Kline, bool, error) {
	key := r.BuildKey("klines", venue, symbol, interval)
	
	data, found, err := r.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}
	
	if !found {
		return nil, false, nil
	}
	
	var klines []interfaces.Kline
	if err := json.Unmarshal(data, &klines); err != nil {
		return nil, false, fmt.Errorf("unmarshal klines: %w", err)
	}
	
	return klines, true, nil
}

// CacheOrderBook stores order book with appropriate TTL
func (r *RedisCache) CacheOrderBook(ctx context.Context, venue, symbol string, orderBook *interfaces.OrderBookSnapshot, ttl time.Duration) error {
	key := r.BuildKey("orderbook", venue, symbol)
	
	data, err := json.Marshal(orderBook)
	if err != nil {
		return fmt.Errorf("marshal orderbook: %w", err)
	}
	
	return r.Set(ctx, key, data, ttl)
}

// GetCachedOrderBook retrieves cached order book
func (r *RedisCache) GetCachedOrderBook(ctx context.Context, venue, symbol string) (*interfaces.OrderBookSnapshot, bool, error) {
	key := r.BuildKey("orderbook", venue, symbol)
	
	data, found, err := r.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}
	
	if !found {
		return nil, false, nil
	}
	
	var orderBook interfaces.OrderBookSnapshot
	if err := json.Unmarshal(data, &orderBook); err != nil {
		return nil, false, fmt.Errorf("unmarshal orderbook: %w", err)
	}
	
	return &orderBook, true, nil
}

// CacheFunding stores funding rate with appropriate TTL
func (r *RedisCache) CacheFunding(ctx context.Context, venue, symbol string, funding *interfaces.FundingRate, ttl time.Duration) error {
	key := r.BuildKey("funding", venue, symbol)
	
	data, err := json.Marshal(funding)
	if err != nil {
		return fmt.Errorf("marshal funding: %w", err)
	}
	
	return r.Set(ctx, key, data, ttl)
}

// GetCachedFunding retrieves cached funding rate
func (r *RedisCache) GetCachedFunding(ctx context.Context, venue, symbol string) (*interfaces.FundingRate, bool, error) {
	key := r.BuildKey("funding", venue, symbol)
	
	data, found, err := r.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}
	
	if !found {
		return nil, false, nil
	}
	
	var funding interfaces.FundingRate
	if err := json.Unmarshal(data, &funding); err != nil {
		return nil, false, fmt.Errorf("unmarshal funding: %w", err)
	}
	
	return &funding, true, nil
}

// CacheOpenInterest stores open interest with appropriate TTL
func (r *RedisCache) CacheOpenInterest(ctx context.Context, venue, symbol string, oi *interfaces.OpenInterest, ttl time.Duration) error {
	key := r.BuildKey("openinterest", venue, symbol)
	
	data, err := json.Marshal(oi)
	if err != nil {
		return fmt.Errorf("marshal open interest: %w", err)
	}
	
	return r.Set(ctx, key, data, ttl)
}

// GetCachedOpenInterest retrieves cached open interest
func (r *RedisCache) GetCachedOpenInterest(ctx context.Context, venue, symbol string) (*interfaces.OpenInterest, bool, error) {
	key := r.BuildKey("openinterest", venue, symbol)
	
	data, found, err := r.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}
	
	if !found {
		return nil, false, nil
	}
	
	var oi interfaces.OpenInterest
	if err := json.Unmarshal(data, &oi); err != nil {
		return nil, false, fmt.Errorf("unmarshal open interest: %w", err)
	}
	
	return &oi, true, nil
}

// Close closes the Redis connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}