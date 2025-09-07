package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sawpanic/cryptorun/internal/persistence"
)

// regimeRepo implements RegimeRepo interface for PostgreSQL
type regimeRepo struct {
	db      *sqlx.DB
	timeout time.Duration
}

// NewRegimeRepo creates a new PostgreSQL regime repository
func NewRegimeRepo(db *sqlx.DB, timeout time.Duration) persistence.RegimeRepo {
	return &regimeRepo{
		db:      db,
		timeout: timeout,
	}
}

// Upsert inserts or updates regime snapshot for timestamp (4h boundary)
func (r *regimeRepo) Upsert(ctx context.Context, snapshot persistence.RegimeSnapshot) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Validate regime type
	if !isValidRegime(snapshot.Regime) {
		return fmt.Errorf("invalid regime type: %s", snapshot.Regime)
	}

	// Validate weight structure and social cap
	if err := validateWeights(snapshot.Weights); err != nil {
		return fmt.Errorf("invalid weights: %w", err)
	}

	// Convert weights and metadata to JSON
	weightsJSON, err := json.Marshal(snapshot.Weights)
	if err != nil {
		return fmt.Errorf("failed to marshal weights: %w", err)
	}

	metadataJSON, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO regime_snapshots 
		(ts, realized_vol_7d, pct_above_20ma, breadth_thrust, regime, weights, 
		 confidence_score, detection_method, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (ts) DO UPDATE SET
			realized_vol_7d = EXCLUDED.realized_vol_7d,
			pct_above_20ma = EXCLUDED.pct_above_20ma,
			breadth_thrust = EXCLUDED.breadth_thrust,
			regime = EXCLUDED.regime,
			weights = EXCLUDED.weights,
			confidence_score = EXCLUDED.confidence_score,
			detection_method = EXCLUDED.detection_method,
			metadata = EXCLUDED.metadata
		RETURNING created_at`

	err = r.db.QueryRowxContext(ctx, query,
		snapshot.Timestamp, snapshot.RealizedVol7d, snapshot.PctAbove20MA,
		snapshot.BreadthThrust, snapshot.Regime, weightsJSON,
		snapshot.ConfidenceScore, snapshot.DetectionMethod, metadataJSON).
		Scan(&snapshot.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert regime snapshot: %w", err)
	}

	return nil
}

// Latest returns the most recent regime classification
func (r *regimeRepo) Latest(ctx context.Context) (*persistence.RegimeSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT ts, realized_vol_7d, pct_above_20ma, breadth_thrust, regime, weights,
		       confidence_score, detection_method, metadata, created_at
		FROM regime_snapshots
		ORDER BY ts DESC
		LIMIT 1`

	row := r.db.QueryRowxContext(ctx, query)
	snapshot, err := r.scanRegimeSnapshot(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest regime: %w", err)
	}

	return snapshot, nil
}

// GetByTimestamp retrieves specific regime snapshot
func (r *regimeRepo) GetByTimestamp(ctx context.Context, ts time.Time) (*persistence.RegimeSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT ts, realized_vol_7d, pct_above_20ma, breadth_thrust, regime, weights,
		       confidence_score, detection_method, metadata, created_at
		FROM regime_snapshots
		WHERE ts = $1`

	row := r.db.QueryRowxContext(ctx, query, ts)
	snapshot, err := r.scanRegimeSnapshot(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get regime by timestamp: %w", err)
	}

	return snapshot, nil
}

// ListRange retrieves regime history within time window
func (r *regimeRepo) ListRange(ctx context.Context, tr persistence.TimeRange) ([]persistence.RegimeSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT ts, realized_vol_7d, pct_above_20ma, breadth_thrust, regime, weights,
		       confidence_score, detection_method, metadata, created_at
		FROM regime_snapshots
		WHERE ts >= $1 AND ts <= $2
		ORDER BY ts DESC`

	rows, err := r.db.QueryxContext(ctx, query, tr.From, tr.To)
	if err != nil {
		return nil, fmt.Errorf("failed to query regime range: %w", err)
	}
	defer rows.Close()

	return r.scanRegimeSnapshots(rows)
}

// ListByRegime retrieves all snapshots of a specific regime type
func (r *regimeRepo) ListByRegime(ctx context.Context, regime string, limit int) ([]persistence.RegimeSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if !isValidRegime(regime) {
		return nil, fmt.Errorf("invalid regime type: %s", regime)
	}

	query := `
		SELECT ts, realized_vol_7d, pct_above_20ma, breadth_thrust, regime, weights,
		       confidence_score, detection_method, metadata, created_at
		FROM regime_snapshots
		WHERE regime = $1
		ORDER BY ts DESC
		LIMIT $2`

	rows, err := r.db.QueryxContext(ctx, query, regime, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query regime by type: %w", err)
	}
	defer rows.Close()

	return r.scanRegimeSnapshots(rows)
}

// GetRegimeStats returns regime distribution statistics
func (r *regimeRepo) GetRegimeStats(ctx context.Context, tr persistence.TimeRange) (map[string]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT regime, COUNT(*)
		FROM regime_snapshots
		WHERE ts >= $1 AND ts <= $2
		GROUP BY regime
		ORDER BY regime`

	rows, err := r.db.QueryxContext(ctx, query, tr.From, tr.To)
	if err != nil {
		return nil, fmt.Errorf("failed to query regime stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int64)
	for rows.Next() {
		var regime string
		var count int64
		if err := rows.Scan(&regime, &count); err != nil {
			return nil, fmt.Errorf("failed to scan regime stats: %w", err)
		}
		stats[regime] = count
	}

	return stats, nil
}

// GetWeightsHistory returns weight evolution over time for analysis
func (r *regimeRepo) GetWeightsHistory(ctx context.Context, tr persistence.TimeRange) ([]persistence.RegimeSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT ts, realized_vol_7d, pct_above_20ma, breadth_thrust, regime, weights,
		       confidence_score, detection_method, metadata, created_at
		FROM regime_snapshots
		WHERE ts >= $1 AND ts <= $2
		ORDER BY ts ASC`

	rows, err := r.db.QueryxContext(ctx, query, tr.From, tr.To)
	if err != nil {
		return nil, fmt.Errorf("failed to query weights history: %w", err)
	}
	defer rows.Close()

	return r.scanRegimeSnapshots(rows)
}

// Helper methods

func (r *regimeRepo) scanRegimeSnapshots(rows *sqlx.Rows) ([]persistence.RegimeSnapshot, error) {
	var snapshots []persistence.RegimeSnapshot

	for rows.Next() {
		snapshot, err := r.scanRegimeSnapshotFromRows(rows)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, *snapshot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return snapshots, nil
}

func (r *regimeRepo) scanRegimeSnapshot(row *sqlx.Row) (*persistence.RegimeSnapshot, error) {
	var snapshot persistence.RegimeSnapshot
	var weightsJSON, metadataJSON []byte

	err := row.Scan(
		&snapshot.Timestamp, &snapshot.RealizedVol7d, &snapshot.PctAbove20MA,
		&snapshot.BreadthThrust, &snapshot.Regime, &weightsJSON,
		&snapshot.ConfidenceScore, &snapshot.DetectionMethod,
		&metadataJSON, &snapshot.CreatedAt)

	if err != nil {
		return nil, err
	}

	// Unmarshal weights
	if err := json.Unmarshal(weightsJSON, &snapshot.Weights); err != nil {
		return nil, fmt.Errorf("failed to unmarshal weights: %w", err)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &snapshot.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	} else {
		snapshot.Metadata = make(map[string]interface{})
	}

	return &snapshot, nil
}

func (r *regimeRepo) scanRegimeSnapshotFromRows(rows *sqlx.Rows) (*persistence.RegimeSnapshot, error) {
	var snapshot persistence.RegimeSnapshot
	var weightsJSON, metadataJSON []byte

	err := rows.Scan(
		&snapshot.Timestamp, &snapshot.RealizedVol7d, &snapshot.PctAbove20MA,
		&snapshot.BreadthThrust, &snapshot.Regime, &weightsJSON,
		&snapshot.ConfidenceScore, &snapshot.DetectionMethod,
		&metadataJSON, &snapshot.CreatedAt)

	if err != nil {
		return nil, err
	}

	// Unmarshal weights
	if err := json.Unmarshal(weightsJSON, &snapshot.Weights); err != nil {
		return nil, fmt.Errorf("failed to unmarshal weights: %w", err)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &snapshot.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	} else {
		snapshot.Metadata = make(map[string]interface{})
	}

	return &snapshot, nil
}

// isValidRegime validates regime type against allowed values
func isValidRegime(regime string) bool {
	validRegimes := map[string]bool{
		"trending": true,
		"choppy":   true,
		"highvol":  true,
		"mixed":    true,
	}
	return validRegimes[regime]
}

// validateWeights ensures weight structure compliance and social cap enforcement
func validateWeights(weights map[string]float64) error {
	requiredWeights := []string{"momentum", "technical", "volume", "quality", "social"}
	
	for _, weight := range requiredWeights {
		if val, exists := weights[weight]; !exists {
			return fmt.Errorf("missing required weight: %s", weight)
		} else if val < 0 {
			return fmt.Errorf("negative weight not allowed: %s = %f", weight, val)
		}
	}

	// Enforce social cap (+10 maximum)
	if socialWeight, exists := weights["social"]; exists && socialWeight > 10.0 {
		return fmt.Errorf("social weight exceeds cap: %f > 10", socialWeight)
	}

	return nil
}