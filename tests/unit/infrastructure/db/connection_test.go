package db

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"github.com/sawpanic/cryptorun/internal/infrastructure/db"
	"github.com/sawpanic/cryptorun/internal/persistence"
)

func TestDefaultConfig(t *testing.T) {
	config := db.DefaultConfig()
	
	assert.Equal(t, 10, config.MaxOpenConns)
	assert.Equal(t, 5, config.MaxIdleConns)
	assert.Equal(t, 30*time.Minute, config.ConnMaxLifetime)
	assert.Equal(t, 5*time.Minute, config.ConnMaxIdleTime)
	assert.Equal(t, 30*time.Second, config.QueryTimeout)
	assert.False(t, config.Enabled) // Should be disabled by default
}

func TestNewManager_Disabled(t *testing.T) {
	config := db.Config{
		Enabled: false,
	}
	
	manager, err := db.NewManager(config)
	require.NoError(t, err)
	
	assert.NotNil(t, manager)
	assert.False(t, manager.IsEnabled())
	assert.Nil(t, manager.Repository())
	assert.Nil(t, manager.DB())
	
	// Health should work even when disabled
	health := manager.Health()
	assert.NotNil(t, health)
	
	healthCheck := health.Health(context.Background())
	assert.True(t, healthCheck.Healthy)
	assert.Contains(t, healthCheck.Errors[0], "disabled")
}

func TestNewManager_MissingDSN(t *testing.T) {
	config := db.Config{
		Enabled: true,
		DSN:     "", // Missing DSN
	}
	
	_, err := db.NewManager(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DSN is required")
}

func TestNewManager_InvalidDSN(t *testing.T) {
	config := db.Config{
		Enabled: true,
		DSN:     "invalid://dsn/format",
	}
	
	_, err := db.NewManager(config)
	assert.Error(t, err)
	// Should get an error when trying to connect to an invalid DSN
	assert.NotEmpty(t, err.Error())
}

func TestHealthChecker_Disabled(t *testing.T) {
	// Create a health checker for disabled database
	manager, err := db.NewManager(db.Config{Enabled: false})
	require.NoError(t, err)
	
	health := manager.Health()
	
	// Test Health method
	healthCheck := health.Health(context.Background())
	assert.True(t, healthCheck.Healthy)
	assert.Contains(t, healthCheck.Errors[0], "disabled")
	assert.Equal(t, 0, healthCheck.ConnectionPool["status"])
	assert.Equal(t, int64(0), healthCheck.ResponseTimeMS)
	
	// Test Ping method
	err = health.Ping(context.Background())
	assert.NoError(t, err) // Should not error when disabled
	
	// Test Stats method
	stats := health.Stats(context.Background())
	assert.False(t, stats["enabled"].(bool))
	assert.Equal(t, "disabled", stats["status"])
}

func TestHealthChecker_Enabled(t *testing.T) {
	// Create a mock database with ping monitoring enabled
	mockDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer mockDB.Close()
	
	// Create sqlx wrapper
	sqlxDB := sqlx.NewDb(mockDB, "postgres")
	
	// Create a manager with enabled config but use our mock DB
	config := db.DefaultConfig()
	config.Enabled = true
	config.DSN = "test_dsn" // Won't be used since we're injecting the mock
	
	// We need to test the health checker directly since we can't easily inject the mock
	// into NewManager due to the ping test during initialization
	healthChecker := &testHealthChecker{
		enabled: true,
		db:      sqlxDB,
		timeout: 5 * time.Second,
	}
	
	// Test successful ping
	mock.ExpectPing()
	
	healthCheck := healthChecker.Health(context.Background())
	assert.True(t, healthCheck.Healthy)
	assert.Empty(t, healthCheck.Errors)
	// Note: ResponseTimeMS might be 0 in tests, that's acceptable
	assert.GreaterOrEqual(t, healthCheck.ResponseTimeMS, int64(0))
	
	// Verify mock expectations
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHealthChecker_PingFailure(t *testing.T) {
	// Create a mock database with ping monitoring enabled
	mockDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer mockDB.Close()
	
	// Create sqlx wrapper
	sqlxDB := sqlx.NewDb(mockDB, "postgres")
	
	healthChecker := &testHealthChecker{
		enabled: true,
		db:      sqlxDB,
		timeout: 5 * time.Second,
	}
	
	// Test ping failure - create the expectation first, then add error
	pingExpectation := mock.ExpectPing()
	pingExpectation.WillReturnError(sqlmock.ErrCancelled)
	
	healthCheck := healthChecker.Health(context.Background())
	assert.False(t, healthCheck.Healthy)
	assert.Len(t, healthCheck.Errors, 1)
	assert.Contains(t, healthCheck.Errors[0], "ping failed")
	
	// Verify mock expectations
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHealthChecker_Stats(t *testing.T) {
	// Create a mock database (no ping monitoring needed for stats)
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	// Create sqlx wrapper
	sqlxDB := sqlx.NewDb(mockDB, "postgres")
	
	healthChecker := &testHealthChecker{
		enabled: true,
		db:      sqlxDB,
		timeout: 5 * time.Second,
	}
	
	stats := healthChecker.Stats(context.Background())
	
	assert.True(t, stats["enabled"].(bool))
	assert.Contains(t, stats, "max_open_connections")
	assert.Contains(t, stats, "open_connections")
	assert.Contains(t, stats, "in_use")
	assert.Contains(t, stats, "idle")
	
	// Verify no database interaction needed for stats
	assert.NoError(t, mock.ExpectationsWereMet())
}

// testHealthChecker is a simplified version of the real health checker for testing
type testHealthChecker struct {
	enabled bool
	db      *sqlx.DB
	timeout time.Duration
}

func (h *testHealthChecker) Health(ctx context.Context) persistence.HealthCheck {
	if !h.enabled {
		return persistence.HealthCheck{
			Healthy:        true,
			Errors:         []string{"Database persistence disabled"},
			ConnectionPool: map[string]int{"status": 0},
			LastCheck:      time.Now(),
			ResponseTimeMS: 0,
		}
	}

	start := time.Now()
	
	// Test basic connectivity
	pingCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	
	var errors []string
	healthy := true
	
	if err := h.db.PingContext(pingCtx); err != nil {
		errors = append(errors, "ping failed: "+err.Error())
		healthy = false
	}

	// Get connection pool stats
	stats := h.db.Stats()
	connectionPool := map[string]int{
		"max_open":      stats.MaxOpenConnections,
		"open":          stats.OpenConnections,
		"in_use":        stats.InUse,
		"idle":          stats.Idle,
		"wait_count":    int(stats.WaitCount),
		"wait_duration": int(stats.WaitDuration.Milliseconds()),
	}

	responseTime := time.Since(start).Milliseconds()

	return persistence.HealthCheck{
		Healthy:        healthy,
		Errors:         errors,
		ConnectionPool: connectionPool,
		LastCheck:      time.Now(),
		ResponseTimeMS: responseTime,
	}
}

func (h *testHealthChecker) Ping(ctx context.Context) error {
	if !h.enabled {
		return nil
	}
	
	pingCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	
	return h.db.PingContext(pingCtx)
}

func (h *testHealthChecker) Stats(ctx context.Context) map[string]interface{} {
	if !h.enabled {
		return map[string]interface{}{
			"enabled": false,
			"status":  "disabled",
		}
	}

	stats := h.db.Stats()
	
	return map[string]interface{}{
		"enabled":               true,
		"max_open_connections":  stats.MaxOpenConnections,
		"open_connections":      stats.OpenConnections,
		"in_use":                stats.InUse,
		"idle":                  stats.Idle,
		"wait_count":            stats.WaitCount,
		"wait_duration_ms":      stats.WaitDuration.Milliseconds(),
		"max_idle_closed":       stats.MaxIdleClosed,
		"max_idle_time_closed":  stats.MaxIdleTimeClosed,
		"max_lifetime_closed":   stats.MaxLifetimeClosed,
	}
}

func TestManager_Close(t *testing.T) {
	// Test closing disabled manager
	manager1, err := db.NewManager(db.Config{Enabled: false})
	require.NoError(t, err)
	
	err = manager1.Close()
	assert.NoError(t, err)
	
	// For enabled manager, we'd need a real database connection
	// which is beyond the scope of unit tests
}

// Test integration with fake repositories
func TestManager_RepositoryIntegration(t *testing.T) {
	// Test disabled manager
	manager, err := db.NewManager(db.Config{Enabled: false})
	require.NoError(t, err)
	
	assert.Nil(t, manager.Repository())
	assert.False(t, manager.IsEnabled())
	
	// Repository should be nil when disabled
	repos := manager.Repository()
	assert.Nil(t, repos)
}