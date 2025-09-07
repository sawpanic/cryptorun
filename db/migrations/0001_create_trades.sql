-- Create trades table for transaction logging
-- Point-in-time (PIT) integrity enforced via ts index
CREATE TABLE trades (
    id BIGSERIAL PRIMARY KEY,
    ts TIMESTAMPTZ NOT NULL,
    symbol TEXT NOT NULL,
    venue TEXT NOT NULL CHECK (venue IN ('binance','okx','coinbase','kraken')),
    side TEXT CHECK (side IN ('buy','sell')),
    price DOUBLE PRECISION NOT NULL CHECK (price > 0),
    qty DOUBLE PRECISION NOT NULL CHECK (qty > 0),
    order_id TEXT,
    attributes JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- PIT-optimized index: symbol first, then timestamp DESC for latest-first queries
CREATE INDEX trades_symbol_ts_idx ON trades (symbol, ts DESC);

-- Venue performance index for exchange-specific queries  
CREATE INDEX trades_venue_ts_idx ON trades (venue, ts DESC);

-- Order tracking index for reconciliation
CREATE INDEX trades_order_id_idx ON trades (order_id) WHERE order_id IS NOT NULL;

-- Composite index for common filtering patterns
CREATE INDEX trades_symbol_venue_ts_idx ON trades (symbol, venue, ts DESC);

-- Add table comment for schema documentation
COMMENT ON TABLE trades IS 'Exchange-native trade execution log with PIT integrity - CryptoRun v3.2.1';
COMMENT ON COLUMN trades.ts IS 'Point-in-time timestamp from exchange (NOT processing time)';
COMMENT ON COLUMN trades.venue IS 'Exchange identifier - must be exchange-native (no aggregators)';
COMMENT ON COLUMN trades.attributes IS 'Flexible JSON attributes for exchange-specific data';