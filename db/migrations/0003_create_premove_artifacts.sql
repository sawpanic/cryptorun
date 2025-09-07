-- Create premove artifacts table for entry gate results and scoring history
-- Point-in-time integrity critical for backtest accuracy
CREATE TABLE premove_artifacts (
    id BIGSERIAL PRIMARY KEY,
    ts TIMESTAMPTZ NOT NULL,
    symbol TEXT NOT NULL,
    venue TEXT NOT NULL CHECK (venue IN ('binance','okx','coinbase','kraken')),
    
    -- Entry gates (hard requirements - Score≥75 + VADR≥1.8 + funding divergence≥2σ)
    gate_score BOOLEAN DEFAULT FALSE,
    gate_vadr BOOLEAN DEFAULT FALSE, 
    gate_funding BOOLEAN DEFAULT FALSE,
    gate_microstructure BOOLEAN DEFAULT FALSE,
    gate_freshness BOOLEAN DEFAULT FALSE,
    gate_fatigue BOOLEAN DEFAULT FALSE,
    
    -- Composite scoring results
    score DOUBLE PRECISION CHECK (score >= 0 AND score <= 100),
    momentum_core DOUBLE PRECISION CHECK (momentum_core >= 0 AND momentum_core <= 100),
    technical_residual DOUBLE PRECISION,
    volume_residual DOUBLE PRECISION,
    quality_residual DOUBLE PRECISION,
    social_residual DOUBLE PRECISION CHECK (social_residual IS NULL OR social_residual <= 10), -- Social cap
    
    -- Factor details and attribution
    factors JSONB,
    regime TEXT CHECK (regime IN ('trending','choppy','highvol','mixed')),
    confidence_score DOUBLE PRECISION DEFAULT 0.5 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    processing_latency_ms INTEGER,
    
    -- Metadata
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE (ts, symbol, venue)
);

-- PIT-optimized indexes for backtesting and analysis
CREATE INDEX premove_artifacts_symbol_ts_idx ON premove_artifacts (symbol, ts DESC);
CREATE INDEX premove_artifacts_venue_ts_idx ON premove_artifacts (venue, ts DESC);

-- Entry gate performance indexes
CREATE INDEX premove_artifacts_gates_passed_idx ON premove_artifacts (
    gate_score, gate_vadr, gate_funding, gate_microstructure, ts DESC
) WHERE gate_score = TRUE AND gate_vadr = TRUE AND gate_funding = TRUE;

-- Scoring analysis indexes
CREATE INDEX premove_artifacts_score_idx ON premove_artifacts (score DESC, ts DESC) WHERE score IS NOT NULL;
CREATE INDEX premove_artifacts_regime_score_idx ON premove_artifacts (regime, score DESC, ts DESC);

-- Performance monitoring index
CREATE INDEX premove_artifacts_latency_idx ON premove_artifacts (processing_latency_ms DESC, ts DESC);

-- Composite index for symbol performance analysis
CREATE INDEX premove_artifacts_symbol_score_gates_idx ON premove_artifacts (
    symbol, score DESC, 
    gate_score, gate_vadr, gate_funding, 
    ts DESC
);

-- Add validation constraint for factors JSON structure
ALTER TABLE premove_artifacts ADD CONSTRAINT factors_structure_check 
    CHECK (
        factors IS NULL OR (
            jsonb_typeof(factors) = 'object' AND
            (factors ? 'momentum_timeframes' OR factors = '{}')
        )
    );

-- Add table comments for schema documentation  
COMMENT ON TABLE premove_artifacts IS 'Pre-movement detection artifacts with entry gates - CryptoRun v3.2.1';
COMMENT ON COLUMN premove_artifacts.ts IS 'Point-in-time timestamp for signal generation (PIT critical)';
COMMENT ON COLUMN premove_artifacts.score IS 'Unified composite score [0-100] after orthogonalization';
COMMENT ON COLUMN premove_artifacts.momentum_core IS 'Protected momentum core (never orthogonalized)';
COMMENT ON COLUMN premove_artifacts.social_residual IS 'Social factor residual (capped at +10)';
COMMENT ON COLUMN premove_artifacts.factors IS 'Factor breakdown and attribution details';
COMMENT ON COLUMN premove_artifacts.gate_score IS 'Hard gate: Score ≥ 75';
COMMENT ON COLUMN premove_artifacts.gate_vadr IS 'Hard gate: VADR ≥ 1.8';
COMMENT ON COLUMN premove_artifacts.gate_funding IS 'Hard gate: Funding divergence ≥ 2σ';