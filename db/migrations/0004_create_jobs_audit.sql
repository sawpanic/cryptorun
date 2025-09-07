-- Create jobs audit table for scheduler and processing task monitoring
-- Tracks job execution history for observability and performance analysis
CREATE TABLE jobs_audit (
    id BIGSERIAL PRIMARY KEY,
    job_id TEXT NOT NULL,
    job_type TEXT NOT NULL CHECK (job_type IN ('scan','monitor','report','calibrate','replay')),
    start_ts TIMESTAMPTZ NOT NULL,
    end_ts TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running','completed','failed','timeout','cancelled')),
    latency_ms INTEGER CHECK (latency_ms >= 0),
    
    -- Job context and parameters
    symbol TEXT,
    venue TEXT CHECK (venue IS NULL OR venue IN ('binance','okx','coinbase','kraken')),
    regime TEXT CHECK (regime IS NULL OR regime IN ('trending','choppy','highvol','mixed')),
    
    -- Results and diagnostics
    records_processed INTEGER DEFAULT 0 CHECK (records_processed >= 0),
    errors_count INTEGER DEFAULT 0 CHECK (errors_count >= 0),
    error_details JSONB,
    
    -- Performance metrics
    cpu_time_ms INTEGER,
    memory_peak_mb INTEGER,
    
    -- Metadata
    worker_id TEXT,
    correlation_id TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Ensure end_ts >= start_ts when both are present
    CONSTRAINT jobs_audit_time_order CHECK (end_ts IS NULL OR end_ts >= start_ts)
);

-- Performance indexes for job monitoring
CREATE INDEX jobs_audit_job_id_idx ON jobs_audit (job_id, start_ts DESC);
CREATE INDEX jobs_audit_status_idx ON jobs_audit (status, start_ts DESC);
CREATE INDEX jobs_audit_type_status_idx ON jobs_audit (job_type, status, start_ts DESC);

-- Latency analysis index
CREATE INDEX jobs_audit_latency_idx ON jobs_audit (latency_ms DESC, start_ts DESC) WHERE latency_ms IS NOT NULL;

-- Symbol/venue performance analysis
CREATE INDEX jobs_audit_symbol_venue_idx ON jobs_audit (symbol, venue, start_ts DESC) WHERE symbol IS NOT NULL;

-- Running jobs monitoring index
CREATE INDEX jobs_audit_running_jobs_idx ON jobs_audit (status, start_ts) WHERE status = 'running';

-- Job correlation tracking
CREATE INDEX jobs_audit_correlation_idx ON jobs_audit (correlation_id, start_ts DESC) WHERE correlation_id IS NOT NULL;

-- Add table comments for schema documentation
COMMENT ON TABLE jobs_audit IS 'Job execution audit log for scheduler and processing tasks - CryptoRun v3.2.1';
COMMENT ON COLUMN jobs_audit.job_id IS 'Unique identifier for job execution instance';
COMMENT ON COLUMN jobs_audit.start_ts IS 'Job start timestamp (NOT submission time)';
COMMENT ON COLUMN jobs_audit.latency_ms IS 'Total execution time from start to completion';
COMMENT ON COLUMN jobs_audit.records_processed IS 'Number of records processed during job execution';
COMMENT ON COLUMN jobs_audit.correlation_id IS 'Correlation ID for tracking related job executions';