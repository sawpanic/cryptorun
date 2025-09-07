package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/sawpanic/cryptorun/internal/persistence"
	"github.com/sawpanic/cryptorun/internal/persistence/postgres"
)

// Config holds database connection configuration
type Config struct {
	DSN             string        `yaml:"dsn" env:"PG_DSN"`
	MaxOpenConns    int           `yaml:"max_open_conns" env:"PG_MAX_OPEN_CONNS"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env:"PG_MAX_IDLE_CONNS"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"PG_CONN_MAX_LIFETIME"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" env:"PG_CONN_MAX_IDLE_TIME"`
	QueryTimeout    time.Duration `yaml:"query_timeout" env:"PG_QUERY_TIMEOUT"`
	Enabled         bool          `yaml:"enabled" env:"PG_ENABLED"`
}

// DefaultConfig returns reasonable defaults for database connections
func DefaultConfig() Config {
	return Config{
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		QueryTimeout:    30 * time.Second,
		Enabled:         false, // Disabled by default - requires explicit configuration
	}
}

// Manager manages database connections and repository instances
type Manager struct {
	db       *sqlx.DB
	config   Config
	repos    *persistence.Repository
	health   *healthChecker
}

// NewManager creates a new database manager with the given configuration
func NewManager(config Config) (*Manager, error) {
	if !config.Enabled {
		return &Manager{
			config: config,
			health: &healthChecker{enabled: false},
		}, nil
	}

	if config.DSN == "" {
		return nil, fmt.Errorf("database DSN is required when enabled")
	}

	db, err := sqlx.Open("postgres", config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Initialize repositories
	repos := &persistence.Repository{
		Trades:  postgres.NewTradesRepo(db, config.QueryTimeout),
		Regimes: postgres.NewRegimeRepo(db, config.QueryTimeout),
		Premove: postgres.NewPremoveRepo(db, config.QueryTimeout),
	}

	healthChecker := &healthChecker{
		enabled: true,
		db:      db,
		timeout: config.QueryTimeout,
	}

	return &Manager{
		db:     db,
		config: config,
		repos:  repos,
		health: healthChecker,
	}, nil
}

// Repository returns the repository collection, or nil if database is disabled
func (m *Manager) Repository() *persistence.Repository {
	return m.repos
}

// Health returns the health checker interface
func (m *Manager) Health() persistence.RepositoryHealth {
	return m.health
}

// DB returns the underlying database connection (for migrations, etc.)
func (m *Manager) DB() *sqlx.DB {
	return m.db
}

// IsEnabled returns whether database persistence is enabled
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled && m.db != nil
}

// Close closes the database connection
func (m *Manager) Close() error {
	if m.db == nil {
		return nil
	}
	return m.db.Close()
}

// healthChecker implements persistence.RepositoryHealth
type healthChecker struct {
	enabled bool
	db      *sqlx.DB
	timeout time.Duration
}

// Health returns current repository health status
func (h *healthChecker) Health(ctx context.Context) persistence.HealthCheck {
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
		errors = append(errors, fmt.Sprintf("ping failed: %v", err))
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

// Ping tests basic connectivity to database
func (h *healthChecker) Ping(ctx context.Context) error {
	if !h.enabled {
		return nil
	}
	
	pingCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	
	return h.db.PingContext(pingCtx)
}

// Stats returns connection pool and query statistics
func (h *healthChecker) Stats(ctx context.Context) map[string]interface{} {
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