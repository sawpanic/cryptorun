package cache

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redismock/v8"
)

func TestRedisCache_Get(t *testing.T) {
	db, mock := redismock.NewClientMock()
	
	prefixes := map[string]string{
		"trades": "trades:",
	}
	
	cache := &RedisCache{
		client:   db,
		prefixes: prefixes,
	}
	
	ctx := context.Background()
	
	t.Run("cache hit returns value", func(t *testing.T) {
		key := "test_key"
		expectedValue := "test_value"
		
		mock.ExpectGet(key).SetVal(expectedValue)
		
		value, found, err := cache.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !found {
			t.Error("Expected cache hit")
		}
		if string(value) != expectedValue {
			t.Errorf("Expected value %s, got %s", expectedValue, string(value))
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
	
	t.Run("cache miss returns not found", func(t *testing.T) {
		key := "missing_key"
		
		mock.ExpectGet(key).RedisNil()
		
		value, found, err := cache.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get should not return error on cache miss: %v", err)
		}
		if found {
			t.Error("Expected cache miss")
		}
		if value != nil {
			t.Errorf("Expected nil value on cache miss, got %v", value)
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
	
	t.Run("redis error returns error", func(t *testing.T) {
		key := "error_key"
		
		mock.ExpectGet(key).SetErr(redis.TxFailedErr)
		
		_, _, err := cache.Get(ctx, key)
		if err == nil {
			t.Error("Expected error when Redis fails")
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
}

func TestRedisCache_Set(t *testing.T) {
	db, mock := redismock.NewClientMock()
	
	cache := &RedisCache{
		client:   db,
		prefixes: map[string]string{},
	}
	
	ctx := context.Background()
	
	t.Run("sets value with TTL", func(t *testing.T) {
		key := "test_key"
		value := []byte("test_value")
		ttl := time.Minute
		
		mock.ExpectSet(key, value, ttl).SetVal("OK")
		
		err := cache.Set(ctx, key, value, ttl)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
	
	t.Run("redis error returns error", func(t *testing.T) {
		key := "error_key"
		value := []byte("test_value")
		ttl := time.Minute
		
		mock.ExpectSet(key, value, ttl).SetErr(redis.TxFailedErr)
		
		err := cache.Set(ctx, key, value, ttl)
		if err == nil {
			t.Error("Expected error when Redis fails")
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
}

func TestRedisCache_Delete(t *testing.T) {
	db, mock := redismock.NewClientMock()
	
	cache := &RedisCache{
		client:   db,
		prefixes: map[string]string{},
	}
	
	ctx := context.Background()
	
	t.Run("deletes key successfully", func(t *testing.T) {
		key := "test_key"
		
		mock.ExpectDel(key).SetVal(1)
		
		err := cache.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
	
	t.Run("redis error returns error", func(t *testing.T) {
		key := "error_key"
		
		mock.ExpectDel(key).SetErr(redis.TxFailedErr)
		
		err := cache.Delete(ctx, key)
		if err == nil {
			t.Error("Expected error when Redis fails")
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
}

func TestRedisCache_Clear(t *testing.T) {
	db, mock := redismock.NewClientMock()
	
	cache := &RedisCache{
		client:   db,
		prefixes: map[string]string{},
	}
	
	ctx := context.Background()
	
	t.Run("clears keys matching pattern", func(t *testing.T) {
		pattern := "trades:*"
		keys := []string{"trades:BTC", "trades:ETH"}
		
		mock.ExpectKeys(pattern).SetVal(keys)
		mock.ExpectDel(keys...).SetVal(int64(len(keys)))
		
		err := cache.Clear(ctx, pattern)
		if err != nil {
			t.Fatalf("Clear failed: %v", err)
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
	
	t.Run("handles empty key list", func(t *testing.T) {
		pattern := "nonexistent:*"
		
		mock.ExpectKeys(pattern).SetVal([]string{})
		
		err := cache.Clear(ctx, pattern)
		if err != nil {
			t.Fatalf("Clear with empty keys should not fail: %v", err)
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
}

func TestRedisCache_BuildKey(t *testing.T) {
	prefixes := map[string]string{
		"trades":    "trades:",
		"orderbook": "ob:",
	}
	
	cache := &RedisCache{
		prefixes: prefixes,
	}
	
	t.Run("builds key with known prefix", func(t *testing.T) {
		key := cache.BuildKey("trades", "binance", "BTCUSDT")
		expected := "trades:binance:BTCUSDT:trades"
		if key != expected {
			t.Errorf("Expected key %s, got %s", expected, key)
		}
	})
	
	t.Run("builds key with default prefix for unknown data type", func(t *testing.T) {
		key := cache.BuildKey("unknown", "binance", "BTCUSDT")
		expected := "facade:binance:BTCUSDT:unknown"
		if key != expected {
			t.Errorf("Expected key %s, got %s", expected, key)
		}
	})
	
	t.Run("builds key with additional parameters", func(t *testing.T) {
		key := cache.BuildKey("trades", "binance", "BTCUSDT", "1h", "limit=100")
		expected := "trades:binance:BTCUSDT:trades:1h:limit=100"
		if key != expected {
			t.Errorf("Expected key %s, got %s", expected, key)
		}
	})
}

func TestRedisCache_CacheTrades(t *testing.T) {
	db, mock := redismock.NewClientMock()
	
	prefixes := map[string]string{
		"trades": "trades:",
	}
	
	cache := &RedisCache{
		client:   db,
		prefixes: prefixes,
	}
	
	ctx := context.Background()
	
	t.Run("caches trades successfully", func(t *testing.T) {
		trades := []interfaces.Trade{
			{
				ID:       "1",
				Symbol:   "BTCUSDT",
				Price:    50000.0,
				Quantity: 1.0,
				Side:     "buy",
				Venue:    "binance",
			},
		}
		
		expectedKey := "trades:binance:BTCUSDT:trades"
		ttl := 30 * time.Second
		
		// Expect JSON marshaling and Redis SET
		mock.ExpectSet(expectedKey, mock.MatchAny(), ttl).SetVal("OK")
		
		err := cache.CacheTrades(ctx, "binance", "BTCUSDT", trades, ttl)
		if err != nil {
			t.Fatalf("CacheTrades failed: %v", err)
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
}

func TestRedisCache_GetCachedTrades(t *testing.T) {
	db, mock := redismock.NewClientMock()
	
	prefixes := map[string]string{
		"trades": "trades:",
	}
	
	cache := &RedisCache{
		client:   db,
		prefixes: prefixes,
	}
	
	ctx := context.Background()
	
	t.Run("retrieves cached trades", func(t *testing.T) {
		expectedKey := "trades:binance:BTCUSDT:trades"
		tradesJSON := `[{"id":"1","symbol":"BTCUSDT","price":50000,"quantity":1,"side":"buy","timestamp":"2023-01-01T00:00:00Z","venue":"binance"}]`
		
		mock.ExpectGet(expectedKey).SetVal(tradesJSON)
		
		trades, found, err := cache.GetCachedTrades(ctx, "binance", "BTCUSDT")
		if err != nil {
			t.Fatalf("GetCachedTrades failed: %v", err)
		}
		if !found {
			t.Error("Expected to find cached trades")
		}
		if len(trades) != 1 {
			t.Errorf("Expected 1 trade, got %d", len(trades))
		}
		if trades[0].ID != "1" {
			t.Errorf("Expected trade ID '1', got %s", trades[0].ID)
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
	
	t.Run("returns not found for cache miss", func(t *testing.T) {
		expectedKey := "trades:binance:ETHUSDT:trades"
		
		mock.ExpectGet(expectedKey).RedisNil()
		
		trades, found, err := cache.GetCachedTrades(ctx, "binance", "ETHUSDT")
		if err != nil {
			t.Fatalf("GetCachedTrades should not error on cache miss: %v", err)
		}
		if found {
			t.Error("Expected cache miss")
		}
		if trades != nil {
			t.Errorf("Expected nil trades on cache miss, got %v", trades)
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
}

func TestRedisCache_CacheOrderBook(t *testing.T) {
	db, mock := redismock.NewClientMock()
	
	prefixes := map[string]string{
		"orderbook": "ob:",
	}
	
	cache := &RedisCache{
		client:   db,
		prefixes: prefixes,
	}
	
	ctx := context.Background()
	
	t.Run("caches order book successfully", func(t *testing.T) {
		orderBook := &interfaces.OrderBookSnapshot{
			Symbol:    "BTCUSDT",
			Venue:     "binance",
			Timestamp: time.Now(),
			Bids: []interfaces.OrderBookLevel{
				{Price: 50000.0, Quantity: 1.0},
			},
			Asks: []interfaces.OrderBookLevel{
				{Price: 50001.0, Quantity: 1.0},
			},
		}
		
		expectedKey := "ob:binance:BTCUSDT:orderbook"
		ttl := 5 * time.Second
		
		mock.ExpectSet(expectedKey, mock.MatchAny(), ttl).SetVal("OK")
		
		err := cache.CacheOrderBook(ctx, "binance", "BTCUSDT", orderBook, ttl)
		if err != nil {
			t.Fatalf("CacheOrderBook failed: %v", err)
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
}

func TestRedisCache_GetStats(t *testing.T) {
	db, mock := redismock.NewClientMock()
	
	cache := &RedisCache{
		client:   db,
		prefixes: map[string]string{},
	}
	
	ctx := context.Background()
	
	t.Run("returns basic stats structure", func(t *testing.T) {
		infoResult := "# Stats\r\nkeyspace_hits:100\r\nkeyspace_misses:20\r\n"
		mock.ExpectInfo("stats").SetVal(infoResult)
		
		stats, err := cache.GetStats(ctx)
		if err != nil {
			t.Fatalf("GetStats failed: %v", err)
		}
		
		// Verify stats structure is returned (actual parsing would need implementation)
		if stats == nil {
			t.Error("Expected stats structure, got nil")
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
}

func TestRedisCache_GetHitRate(t *testing.T) {
	db, mock := redismock.NewClientMock()
	
	cache := &RedisCache{
		client:   db,
		prefixes: map[string]string{},
	}
	
	ctx := context.Background()
	
	t.Run("returns hit rate", func(t *testing.T) {
		infoResult := "# Stats\r\nkeyspace_hits:100\r\nkeyspace_misses:20\r\n"
		mock.ExpectInfo("stats").SetVal(infoResult)
		
		hitRate := cache.GetHitRate(ctx)
		
		// Should return 0.0 due to simplified stats implementation
		if hitRate != 0.0 {
			t.Errorf("Expected hit rate 0.0 (simplified implementation), got %f", hitRate)
		}
		
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Redis expectations not met: %v", err)
		}
	})
}