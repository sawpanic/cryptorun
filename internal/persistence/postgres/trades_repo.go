package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/sawpanic/cryptorun/internal/persistence"
)

// tradesRepo implements TradesRepo interface for PostgreSQL
type tradesRepo struct {
	db      *sqlx.DB
	timeout time.Duration
}

// NewTradesRepo creates a new PostgreSQL trades repository
func NewTradesRepo(db *sqlx.DB, timeout time.Duration) persistence.TradesRepo {
	return &tradesRepo{
		db:      db,
		timeout: timeout,
	}
}

// Insert adds a new trade record with exchange-native validation
func (r *tradesRepo) Insert(ctx context.Context, trade persistence.Trade) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Validate exchange-native venue
	if !isExchangeNative(trade.Venue) {
		return fmt.Errorf("invalid venue: %s - only exchange-native venues allowed", trade.Venue)
	}

	// Convert attributes to JSONB
	attributesJSON, err := json.Marshal(trade.Attributes)
	if err != nil {
		return fmt.Errorf("failed to marshal attributes: %w", err)
	}

	query := `
		INSERT INTO trades (ts, symbol, venue, side, price, qty, order_id, attributes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`

	err = r.db.QueryRowxContext(ctx, query,
		trade.Timestamp, trade.Symbol, trade.Venue, trade.Side,
		trade.Price, trade.Qty, trade.OrderID, attributesJSON).
		Scan(&trade.ID, &trade.CreatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("duplicate trade: %w", err)
		}
		return fmt.Errorf("failed to insert trade: %w", err)
	}

	return nil
}

// InsertBatch adds multiple trades atomically for high-throughput scenarios
func (r *tradesRepo) InsertBatch(ctx context.Context, trades []persistence.Trade) error {
	if len(trades) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout*time.Duration(len(trades)/100+1))
	defer cancel()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO trades (ts, symbol, venue, side, price, qty, order_id, attributes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, trade := range trades {
		// Validate exchange-native venue
		if !isExchangeNative(trade.Venue) {
			return fmt.Errorf("invalid venue in batch: %s - only exchange-native venues allowed", trade.Venue)
		}

		attributesJSON, err := json.Marshal(trade.Attributes)
		if err != nil {
			return fmt.Errorf("failed to marshal attributes for trade: %w", err)
		}

		_, err = stmt.ExecContext(ctx,
			trade.Timestamp, trade.Symbol, trade.Venue, trade.Side,
			trade.Price, trade.Qty, trade.OrderID, attributesJSON)
		if err != nil {
			return fmt.Errorf("failed to insert trade in batch: %w", err)
		}
	}

	return tx.Commit()
}

// ListBySymbol retrieves trades for a symbol within time range (PIT-ordered)
func (r *tradesRepo) ListBySymbol(ctx context.Context, symbol string, tr persistence.TimeRange, limit int) ([]persistence.Trade, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT id, ts, symbol, venue, side, price, qty, order_id, attributes, created_at
		FROM trades
		WHERE symbol = $1 AND ts >= $2 AND ts <= $3
		ORDER BY ts DESC
		LIMIT $4`

	rows, err := r.db.QueryxContext(ctx, query, symbol, tr.From, tr.To, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades by symbol: %w", err)
	}
	defer rows.Close()

	return r.scanTrades(rows)
}

// ListByVenue retrieves trades for a venue within time range
func (r *tradesRepo) ListByVenue(ctx context.Context, venue string, tr persistence.TimeRange, limit int) ([]persistence.Trade, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Validate exchange-native venue
	if !isExchangeNative(venue) {
		return nil, fmt.Errorf("invalid venue: %s - only exchange-native venues allowed", venue)
	}

	query := `
		SELECT id, ts, symbol, venue, side, price, qty, order_id, attributes, created_at
		FROM trades
		WHERE venue = $1 AND ts >= $2 AND ts <= $3
		ORDER BY ts DESC
		LIMIT $4`

	rows, err := r.db.QueryxContext(ctx, query, venue, tr.From, tr.To, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades by venue: %w", err)
	}
	defer rows.Close()

	return r.scanTrades(rows)
}

// GetByOrderID finds trade by exchange order ID for reconciliation
func (r *tradesRepo) GetByOrderID(ctx context.Context, orderID string) (*persistence.Trade, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT id, ts, symbol, venue, side, price, qty, order_id, attributes, created_at
		FROM trades
		WHERE order_id = $1
		ORDER BY ts DESC
		LIMIT 1`

	row := r.db.QueryRowxContext(ctx, query, orderID)
	
	trade, err := r.scanTrade(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get trade by order ID: %w", err)
	}

	return trade, nil
}

// GetLatest returns most recent trades across all symbols/venues
func (r *tradesRepo) GetLatest(ctx context.Context, limit int) ([]persistence.Trade, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT id, ts, symbol, venue, side, price, qty, order_id, attributes, created_at
		FROM trades
		ORDER BY ts DESC
		LIMIT $1`

	rows, err := r.db.QueryxContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest trades: %w", err)
	}
	defer rows.Close()

	return r.scanTrades(rows)
}

// Count returns total trades in time range for statistics
func (r *tradesRepo) Count(ctx context.Context, tr persistence.TimeRange) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT COUNT(*)
		FROM trades
		WHERE ts >= $1 AND ts <= $2`

	var count int64
	err := r.db.QueryRowxContext(ctx, query, tr.From, tr.To).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count trades: %w", err)
	}

	return count, nil
}

// CountByVenue returns trade counts grouped by venue
func (r *tradesRepo) CountByVenue(ctx context.Context, tr persistence.TimeRange) (map[string]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	query := `
		SELECT venue, COUNT(*)
		FROM trades
		WHERE ts >= $1 AND ts <= $2
		GROUP BY venue
		ORDER BY venue`

	rows, err := r.db.QueryxContext(ctx, query, tr.From, tr.To)
	if err != nil {
		return nil, fmt.Errorf("failed to count trades by venue: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var venue string
		var count int64
		if err := rows.Scan(&venue, &count); err != nil {
			return nil, fmt.Errorf("failed to scan venue count: %w", err)
		}
		counts[venue] = count
	}

	return counts, nil
}

// Helper methods

func (r *tradesRepo) scanTrades(rows *sqlx.Rows) ([]persistence.Trade, error) {
	var trades []persistence.Trade

	for rows.Next() {
		trade, err := r.scanTradeFromRows(rows)
		if err != nil {
			return nil, err
		}
		trades = append(trades, *trade)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return trades, nil
}

func (r *tradesRepo) scanTrade(row *sqlx.Row) (*persistence.Trade, error) {
	var trade persistence.Trade
	var attributesJSON []byte

	err := row.Scan(
		&trade.ID, &trade.Timestamp, &trade.Symbol, &trade.Venue,
		&trade.Side, &trade.Price, &trade.Qty, &trade.OrderID,
		&attributesJSON, &trade.CreatedAt)

	if err != nil {
		return nil, err
	}

	if len(attributesJSON) > 0 {
		if err := json.Unmarshal(attributesJSON, &trade.Attributes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
		}
	} else {
		trade.Attributes = make(map[string]interface{})
	}

	return &trade, nil
}

func (r *tradesRepo) scanTradeFromRows(rows *sqlx.Rows) (*persistence.Trade, error) {
	var trade persistence.Trade
	var attributesJSON []byte

	err := rows.Scan(
		&trade.ID, &trade.Timestamp, &trade.Symbol, &trade.Venue,
		&trade.Side, &trade.Price, &trade.Qty, &trade.OrderID,
		&attributesJSON, &trade.CreatedAt)

	if err != nil {
		return nil, err
	}

	if len(attributesJSON) > 0 {
		if err := json.Unmarshal(attributesJSON, &trade.Attributes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
		}
	} else {
		trade.Attributes = make(map[string]interface{})
	}

	return &trade, nil
}

// isExchangeNative validates venue against allowed exchange-native sources
func isExchangeNative(venue string) bool {
	allowedVenues := map[string]bool{
		"binance":  true,
		"okx":      true,
		"coinbase": true,
		"kraken":   true,
	}
	return allowedVenues[venue]
}