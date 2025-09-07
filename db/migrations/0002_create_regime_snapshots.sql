-- Create regime snapshots table for 4h regime detection system
-- Stores regime classification results and associated weight profiles
CREATE TABLE regime_snapshots (
    ts TIMESTAMPTZ PRIMARY KEY,
    realized_vol_7d DOUBLE PRECISION NOT NULL CHECK (realized_vol_7d >= 0),
    pct_above_20ma DOUBLE PRECISION NOT NULL CHECK (pct_above_20ma >= 0 AND pct_above_20ma <= 100),
    breadth_thrust DOUBLE PRECISION NOT NULL CHECK (breadth_thrust >= -1 AND breadth_thrust <= 1),
    regime TEXT NOT NULL CHECK (regime IN ('trending','choppy','highvol','mixed')),
    weights JSONB NOT NULL,
    confidence_score DOUBLE PRECISION DEFAULT 0.5 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    detection_method TEXT DEFAULT 'majority_vote',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Time-series index for regime history queries
CREATE INDEX regime_snapshots_ts_idx ON regime_snapshots (ts DESC);

-- Regime type analysis index
CREATE INDEX regime_snapshots_regime_idx ON regime_snapshots (regime, ts DESC);

-- Confidence filtering index for quality control
CREATE INDEX regime_snapshots_confidence_idx ON regime_snapshots (confidence_score DESC, ts DESC);

-- Composite index for regime performance analysis
CREATE INDEX regime_snapshots_regime_confidence_idx ON regime_snapshots (regime, confidence_score DESC, ts DESC);

-- Add validation constraint for weights JSON structure
ALTER TABLE regime_snapshots ADD CONSTRAINT weights_structure_check 
    CHECK (
        jsonb_typeof(weights) = 'object' AND
        weights ? 'momentum' AND
        weights ? 'technical' AND
        weights ? 'volume' AND
        weights ? 'quality' AND
        weights ? 'social' AND
        (weights->>'momentum')::numeric >= 0 AND
        (weights->>'technical')::numeric >= 0 AND
        (weights->>'volume')::numeric >= 0 AND
        (weights->>'quality')::numeric >= 0 AND
        (weights->>'social')::numeric >= 0 AND
        (weights->>'social')::numeric <= 10  -- Social cap enforcement
    );

-- Add table comments for schema documentation
COMMENT ON TABLE regime_snapshots IS 'Market regime detection results with 4h cadence - CryptoRun v3.2.1';
COMMENT ON COLUMN regime_snapshots.ts IS '4h boundary timestamp for regime detection window';
COMMENT ON COLUMN regime_snapshots.regime IS 'Detected market regime from majority vote classifier';
COMMENT ON COLUMN regime_snapshots.weights IS 'Factor weights JSON for this regime (momentum protected)';
COMMENT ON COLUMN regime_snapshots.confidence_score IS 'Regime detection confidence [0-1]';