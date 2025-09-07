package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sawpanic/cryptorun/internal/persistence"
)

// premoveRepo implements PremoveRepo interface for PostgreSQL
type premoveRepo struct {
	db      *sqlx.DB
	timeout time.Duration
}

// NewPremoveRepo creates a new PostgreSQL premove repository
func NewPremoveRepo(db *sqlx.DB, timeout time.Duration) persistence.PremoveRepo {
	return &premoveRepo{
		db:      db,
		timeout: timeout,
	}
}

// Upsert inserts or updates premove artifact (unique per ts/symbol/venue)
func (r *premoveRepo) Upsert(ctx context.Context, artifact persistence.PremoveArtifact) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Validate exchange-native venue
	if !isExchangeNative(artifact.Venue) {
		return fmt.Errorf("invalid venue: %s - only exchange-native venues allowed", artifact.Venue)
	}

	// Validate social residual cap
	if artifact.SocialResidual != nil && *artifact.SocialResidual > 10.0 {
		return fmt.Errorf("social residual exceeds cap: %f > 10", *artifact.SocialResidual)
	}

	// Convert factors to JSON
	var factorsJSON []byte
	var err error
	if artifact.Factors != nil {
		factorsJSON, err = json.Marshal(artifact.Factors)
		if err != nil {
			return fmt.Errorf("failed to marshal factors: %w", err)
		}
	}

	query := `
		INSERT INTO premove_artifacts 
		(ts, symbol, venue, gate_score, gate_vadr, gate_funding, gate_microstructure, 
		 gate_freshness, gate_fatigue, score, momentum_core, technical_residual, 
		 volume_residual, quality_residual, social_residual, factors, regime, 
		 confidence_score, processing_latency_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		ON CONFLICT (ts, symbol, venue) DO UPDATE SET
			gate_score = EXCLUDED.gate_score,
			gate_vadr = EXCLUDED.gate_vadr,
			gate_funding = EXCLUDED.gate_funding,
			gate_microstructure = EXCLUDED.gate_microstructure,
			gate_freshness = EXCLUDED.gate_freshness,
			gate_fatigue = EXCLUDED.gate_fatigue,
			score = EXCLUDED.score,
			momentum_core = EXCLUDED.momentum_core,
			technical_residual = EXCLUDED.technical_residual,
			volume_residual = EXCLUDED.volume_residual,
			quality_residual = EXCLUDED.quality_residual,
			social_residual = EXCLUDED.social_residual,
			factors = EXCLUDED.factors,
			regime = EXCLUDED.regime,
			confidence_score = EXCLUDED.confidence_score,
			processing_latency_ms = EXCLUDED.processing_latency_ms
		RETURNING id, created_at`

	err = r.db.QueryRowxContext(ctx, query,
		artifact.Timestamp, artifact.Symbol, artifact.Venue,
		artifact.GateScore, artifact.GateVADR, artifact.GateFunding,
		artifact.GateMicrostructure, artifact.GateFreshness, artifact.GateFatigue,
		artifact.Score, artifact.MomentumCore, artifact.TechnicalResidual,
		artifact.VolumeResidual, artifact.QualityResidual, artifact.SocialResidual,
		factorsJSON, artifact.Regime, artifact.ConfidenceScore, artifact.ProcessingLatencyMS).
		Scan(&artifact.ID, &artifact.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert premove artifact: %w", err)
	}

	return nil
}

// UpsertBatch processes multiple artifacts atomically
func (r *premoveRepo) UpsertBatch(ctx context.Context, artifacts []persistence.PremoveArtifact) error {
	if len(artifacts) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout*time.Duration(len(artifacts)/50+1))
	defer cancel()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO premove_artifacts 
		(ts, symbol, venue, gate_score, gate_vadr, gate_funding, gate_microstructure, 
		 gate_freshness, gate_fatigue, score, momentum_core, technical_residual, 
		 volume_residual, quality_residual, social_residual, factors, regime, 
		 confidence_score, processing_latency_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		ON CONFLICT (ts, symbol, venue) DO UPDATE SET
			gate_score = EXCLUDED.gate_score,
			gate_vadr = EXCLUDED.gate_vadr,
			gate_funding = EXCLUDED.gate_funding,
			gate_microstructure = EXCLUDED.gate_microstructure,
			gate_freshness = EXCLUDED.gate_freshness,
			gate_fatigue = EXCLUDED.gate_fatigue,
			score = EXCLUDED.score,
			momentum_core = EXCLUDED.momentum_core,
			technical_residual = EXCLUDED.technical_residual,
			volume_residual = EXCLUDED.volume_residual,
			quality_residual = EXCLUDED.quality_residual,
			social_residual = EXCLUDED.social_residual,
			factors = EXCLUDED.factors,
			regime = EXCLUDED.regime,
			confidence_score = EXCLUDED.confidence_score,
			processing_latency_ms = EXCLUDED.processing_latency_ms`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, artifact := range artifacts {
		// Validate exchange-native venue
		if !isExchangeNative(artifact.Venue) {
			return fmt.Errorf("invalid venue in batch: %s", artifact.Venue)
		}

		// Validate social residual cap
		if artifact.SocialResidual != nil && *artifact.SocialResidual > 10.0 {
			return fmt.Errorf("social residual exceeds cap in batch: %f > 10", *artifact.SocialResidual)
		}

		var factorsJSON []byte
		if artifact.Factors != nil {
			factorsJSON, err = json.Marshal(artifact.Factors)
			if err != nil {
				return fmt.Errorf("failed to marshal factors in batch: %w", err)
			}
		}

		_, err = stmt.ExecContext(ctx,
			artifact.Timestamp, artifact.Symbol, artifact.Venue,
			artifact.GateScore, artifact.GateVADR, artifact.GateFunding,
			artifact.GateMicrostructure, artifact.GateFreshness, artifact.GateFatigue,
			artifact.Score, artifact.MomentumCore, artifact.TechnicalResidual,
			artifact.VolumeResidual, artifact.QualityResidual, artifact.SocialResidual,
			factorsJSON, artifact.Regime, artifact.ConfidenceScore, artifact.ProcessingLatencyMS)
		if err != nil {
			return fmt.Errorf("failed to upsert artifact in batch: %w", err)
		}
	}

	return tx.Commit()
}

// Window retrieves artifacts within time range for backtesting
func (r *premoveRepo) Window(ctx context.Context, tr persistence.TimeRange) ([]persistence.PremoveArtifact, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT id, ts, symbol, venue, gate_score, gate_vadr, gate_funding, 
		       gate_microstructure, gate_freshness, gate_fatigue, score, momentum_core,
		       technical_residual, volume_residual, quality_residual, social_residual,
		       factors, regime, confidence_score, processing_latency_ms, created_at
		FROM premove_artifacts
		WHERE ts >= $1 AND ts <= $2
		ORDER BY ts DESC`

	rows, err := r.db.QueryxContext(ctx, query, tr.From, tr.To)
	if err != nil {
		return nil, fmt.Errorf("failed to query artifacts window: %w", err)
	}
	defer rows.Close()

	return r.scanPremoveArtifacts(rows)
}

// ListBySymbol retrieves artifacts for specific symbol (PIT-ordered)
func (r *premoveRepo) ListBySymbol(ctx context.Context, symbol string, tr persistence.TimeRange, limit int) ([]persistence.PremoveArtifact, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT id, ts, symbol, venue, gate_score, gate_vadr, gate_funding,
		       gate_microstructure, gate_freshness, gate_fatigue, score, momentum_core,
		       technical_residual, volume_residual, quality_residual, social_residual,
		       factors, regime, confidence_score, processing_latency_ms, created_at
		FROM premove_artifacts
		WHERE symbol = $1 AND ts >= $2 AND ts <= $3
		ORDER BY ts DESC
		LIMIT $4`

	rows, err := r.db.QueryxContext(ctx, query, symbol, tr.From, tr.To, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query artifacts by symbol: %w", err)
	}
	defer rows.Close()

	return r.scanPremoveArtifacts(rows)
}

// ListPassed retrieves artifacts that passed all entry gates
func (r *premoveRepo) ListPassed(ctx context.Context, tr persistence.TimeRange, limit int) ([]persistence.PremoveArtifact, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT id, ts, symbol, venue, gate_score, gate_vadr, gate_funding,
		       gate_microstructure, gate_freshness, gate_fatigue, score, momentum_core,
		       technical_residual, volume_residual, quality_residual, social_residual,
		       factors, regime, confidence_score, processing_latency_ms, created_at
		FROM premove_artifacts
		WHERE ts >= $1 AND ts <= $2
		  AND gate_score = true
		  AND gate_vadr = true
		  AND gate_funding = true
		  AND gate_microstructure = true
		  AND gate_freshness = true
		  AND gate_fatigue = true
		ORDER BY ts DESC
		LIMIT $3`

	rows, err := r.db.QueryxContext(ctx, query, tr.From, tr.To, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query passed artifacts: %w", err)
	}
	defer rows.Close()

	return r.scanPremoveArtifacts(rows)
}

// ListByScore retrieves artifacts above score threshold
func (r *premoveRepo) ListByScore(ctx context.Context, minScore float64, tr persistence.TimeRange, limit int) ([]persistence.PremoveArtifact, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT id, ts, symbol, venue, gate_score, gate_vadr, gate_funding,
		       gate_microstructure, gate_freshness, gate_fatigue, score, momentum_core,
		       technical_residual, volume_residual, quality_residual, social_residual,
		       factors, regime, confidence_score, processing_latency_ms, created_at
		FROM premove_artifacts
		WHERE ts >= $1 AND ts <= $2
		  AND score >= $3
		ORDER BY score DESC, ts DESC
		LIMIT $4`

	rows, err := r.db.QueryxContext(ctx, query, tr.From, tr.To, minScore, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query artifacts by score: %w", err)
	}
	defer rows.Close()

	return r.scanPremoveArtifacts(rows)
}

// ListByRegime retrieves artifacts for specific market regime
func (r *premoveRepo) ListByRegime(ctx context.Context, regime string, tr persistence.TimeRange, limit int) ([]persistence.PremoveArtifact, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT id, ts, symbol, venue, gate_score, gate_vadr, gate_funding,
		       gate_microstructure, gate_freshness, gate_fatigue, score, momentum_core,
		       technical_residual, volume_residual, quality_residual, social_residual,
		       factors, regime, confidence_score, processing_latency_ms, created_at
		FROM premove_artifacts
		WHERE regime = $1 AND ts >= $2 AND ts <= $3
		ORDER BY ts DESC
		LIMIT $4`

	rows, err := r.db.QueryxContext(ctx, query, regime, tr.From, tr.To, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query artifacts by regime: %w", err)
	}
	defer rows.Close()

	return r.scanPremoveArtifacts(rows)
}

// GetGateStats returns entry gate pass/fail statistics
func (r *premoveRepo) GetGateStats(ctx context.Context, tr persistence.TimeRange) (map[string]map[string]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT 
			COUNT(CASE WHEN gate_score = true THEN 1 END) as gate_score_pass,
			COUNT(CASE WHEN gate_score = false THEN 1 END) as gate_score_fail,
			COUNT(CASE WHEN gate_vadr = true THEN 1 END) as gate_vadr_pass,
			COUNT(CASE WHEN gate_vadr = false THEN 1 END) as gate_vadr_fail,
			COUNT(CASE WHEN gate_funding = true THEN 1 END) as gate_funding_pass,
			COUNT(CASE WHEN gate_funding = false THEN 1 END) as gate_funding_fail,
			COUNT(CASE WHEN gate_microstructure = true THEN 1 END) as gate_microstructure_pass,
			COUNT(CASE WHEN gate_microstructure = false THEN 1 END) as gate_microstructure_fail,
			COUNT(CASE WHEN gate_freshness = true THEN 1 END) as gate_freshness_pass,
			COUNT(CASE WHEN gate_freshness = false THEN 1 END) as gate_freshness_fail,
			COUNT(CASE WHEN gate_fatigue = true THEN 1 END) as gate_fatigue_pass,
			COUNT(CASE WHEN gate_fatigue = false THEN 1 END) as gate_fatigue_fail
		FROM premove_artifacts
		WHERE ts >= $1 AND ts <= $2`

	var stats struct {
		GateScorePass        int64 `db:"gate_score_pass"`
		GateScoreFail        int64 `db:"gate_score_fail"`
		GateVADRPass         int64 `db:"gate_vadr_pass"`
		GateVADRFail         int64 `db:"gate_vadr_fail"`
		GateFundingPass      int64 `db:"gate_funding_pass"`
		GateFundingFail      int64 `db:"gate_funding_fail"`
		GateMicrostructurePass int64 `db:"gate_microstructure_pass"`
		GateMicrostructureFail int64 `db:"gate_microstructure_fail"`
		GateFreshnessPass    int64 `db:"gate_freshness_pass"`
		GateFreshnessFail    int64 `db:"gate_freshness_fail"`
		GateFatiguePass      int64 `db:"gate_fatigue_pass"`
		GateFatigueFail      int64 `db:"gate_fatigue_fail"`
	}

	err := r.db.GetContext(ctx, &stats, query, tr.From, tr.To)
	if err != nil {
		return nil, fmt.Errorf("failed to query gate stats: %w", err)
	}

	result := map[string]map[string]int64{
		"gate_score":        {"pass": stats.GateScorePass, "fail": stats.GateScoreFail},
		"gate_vadr":         {"pass": stats.GateVADRPass, "fail": stats.GateVADRFail},
		"gate_funding":      {"pass": stats.GateFundingPass, "fail": stats.GateFundingFail},
		"gate_microstructure": {"pass": stats.GateMicrostructurePass, "fail": stats.GateMicrostructureFail},
		"gate_freshness":    {"pass": stats.GateFreshnessPass, "fail": stats.GateFreshnessFail},
		"gate_fatigue":      {"pass": stats.GateFatiguePass, "fail": stats.GateFatigueFail},
	}

	return result, nil
}

// GetScoreDistribution returns score histogram for performance analysis
func (r *premoveRepo) GetScoreDistribution(ctx context.Context, tr persistence.TimeRange, buckets int) (map[string]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		WITH score_buckets AS (
			SELECT 
				CASE 
					WHEN score IS NULL THEN 'null'
					WHEN score < 25 THEN '0-25'
					WHEN score < 50 THEN '25-50'
					WHEN score < 75 THEN '50-75'
					WHEN score < 90 THEN '75-90'
					ELSE '90-100'
				END as bucket,
				COUNT(*) as count
			FROM premove_artifacts
			WHERE ts >= $1 AND ts <= $2
			GROUP BY bucket
		)
		SELECT bucket, count FROM score_buckets ORDER BY bucket`

	rows, err := r.db.QueryxContext(ctx, query, tr.From, tr.To)
	if err != nil {
		return nil, fmt.Errorf("failed to query score distribution: %w", err)
	}
	defer rows.Close()

	distribution := make(map[string]int64)
	for rows.Next() {
		var bucket string
		var count int64
		if err := rows.Scan(&bucket, &count); err != nil {
			return nil, fmt.Errorf("failed to scan score distribution: %w", err)
		}
		distribution[bucket] = count
	}

	return distribution, nil
}

// GetLatencyStats returns processing latency percentiles
func (r *premoveRepo) GetLatencyStats(ctx context.Context, tr persistence.TimeRange) (map[string]float64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT 
			percentile_cont(0.50) WITHIN GROUP (ORDER BY processing_latency_ms) as p50,
			percentile_cont(0.95) WITHIN GROUP (ORDER BY processing_latency_ms) as p95,
			percentile_cont(0.99) WITHIN GROUP (ORDER BY processing_latency_ms) as p99,
			AVG(processing_latency_ms::float) as mean,
			MIN(processing_latency_ms) as min,
			MAX(processing_latency_ms) as max
		FROM premove_artifacts
		WHERE ts >= $1 AND ts <= $2 AND processing_latency_ms IS NOT NULL`

	var stats struct {
		P50  *float64 `db:"p50"`
		P95  *float64 `db:"p95"`
		P99  *float64 `db:"p99"`
		Mean *float64 `db:"mean"`
		Min  *int     `db:"min"`
		Max  *int     `db:"max"`
	}

	err := r.db.GetContext(ctx, &stats, query, tr.From, tr.To)
	if err != nil {
		return nil, fmt.Errorf("failed to query latency stats: %w", err)
	}

	result := make(map[string]float64)
	if stats.P50 != nil {
		result["p50"] = *stats.P50
	}
	if stats.P95 != nil {
		result["p95"] = *stats.P95
	}
	if stats.P99 != nil {
		result["p99"] = *stats.P99
	}
	if stats.Mean != nil {
		result["mean"] = *stats.Mean
	}
	if stats.Min != nil {
		result["min"] = float64(*stats.Min)
	}
	if stats.Max != nil {
		result["max"] = float64(*stats.Max)
	}

	return result, nil
}

// Helper methods

func (r *premoveRepo) scanPremoveArtifacts(rows *sqlx.Rows) ([]persistence.PremoveArtifact, error) {
	var artifacts []persistence.PremoveArtifact

	for rows.Next() {
		artifact, err := r.scanPremoveArtifactFromRows(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, *artifact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return artifacts, nil
}

func (r *premoveRepo) scanPremoveArtifactFromRows(rows *sqlx.Rows) (*persistence.PremoveArtifact, error) {
	var artifact persistence.PremoveArtifact
	var factorsJSON []byte

	err := rows.Scan(
		&artifact.ID, &artifact.Timestamp, &artifact.Symbol, &artifact.Venue,
		&artifact.GateScore, &artifact.GateVADR, &artifact.GateFunding,
		&artifact.GateMicrostructure, &artifact.GateFreshness, &artifact.GateFatigue,
		&artifact.Score, &artifact.MomentumCore, &artifact.TechnicalResidual,
		&artifact.VolumeResidual, &artifact.QualityResidual, &artifact.SocialResidual,
		&factorsJSON, &artifact.Regime, &artifact.ConfidenceScore,
		&artifact.ProcessingLatencyMS, &artifact.CreatedAt)

	if err != nil {
		return nil, err
	}

	// Unmarshal factors if present
	if len(factorsJSON) > 0 {
		if err := json.Unmarshal(factorsJSON, &artifact.Factors); err != nil {
			return nil, fmt.Errorf("failed to unmarshal factors: %w", err)
		}
	} else {
		artifact.Factors = make(map[string]interface{})
	}

	return &artifact, nil
}