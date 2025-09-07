HOUSE RULES (apply to every phase)

──────────────────────────────────

OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS.

DOCS MANDATE — UPDATE MD ON EVERY PROMPT (link exact files/anchors).

ALREADY IMPLEMENTED PRE-FLIGHT — Before edits, grep+scan the repo for existing implementations. If acceptance criteria are already met:

&nbsp; → STOP. Emit \*\*ALREADY IMPLEMENTED\*\* with exact file:line quotes and acceptance mapping.

&nbsp; → If tiny gaps remain, emit a minimal diff plan (patch-only), then proceed.

WRITE-SCOPE — Patch-only unless explicitly told to add a new file. No rewrites of working subsystems.

EVIDENCE — Every material claim must include a direct repo quote with (path:line). Maintain an Evidence Ledger table per phase.

CONSTRAINTS — Exchange-native microstructure (no aggregators for depth/spread), USD pairs only by default, prioritize Kraken venue; MomentumCore protected; Gram–Schmidt orthogonalization; social contribution capped (+10) and applied \*\*after\*\* momentum/volume; regime-adaptive weights updated at 4h cadence; point-in-time (PIT) integrity; hot/warm/cold tiers; source authority \& cascades; VADR stability; late-fill guard; freshness; liquidity gates.



GLOBAL ORCHESTRATOR (Queue-Runner, Serial, Auto-Continue)

──────────────────────────────────────────────────────────

• Repo: current working directory (branch=main unless otherwise stated).

• Master spec file: THIS FILE (prompts/EPICS.next4.md).

• AUTO-CONTINUE: Do not pause for user approval. If response nears token limits, split into subphases (A1.1, A1.2, …) and continue until acceptance passes.

• After each subphase:

&nbsp; – Update checkboxes in this file.

&nbsp; – Update CHANGELOG.md (today) with files/paths/spec refs.

&nbsp; – Update PROGRESS.yaml (phase counters with timestamps).

&nbsp; – Run: `go build ./... \&\& go test ./...` (emit summary).

&nbsp; – Update/cross-link docs (docs/\*, product.md, mission.md, ENGINEERING\_TRANSPARENCY\_LOG.md).

• Always emit an Evidence Ledger (Feature | Status | Evidence Quote | File:Line | Spec Ref) per subphase.



ACCEPTANCE PRIMER (used everywhere)

────────────────────────────────────

• Build/test pass (`go build ./... \&\& go test ./...`).

• Evidence Ledger present and specific (paths/lines).

• Docs updated + CHANGELOG updated (with relative links).

• New metrics visible at `/metrics` where applicable; CLI flags help text updated.

• For networked components: dry-run or mock tests that demonstrate behavior with PIT discipline.



SERIAL EPICS (Run A → B → C → D)

=================================



EPIC A — DATA INFRASTRUCTURE ENHANCEMENTS

=========================================



A1) Parquet Support (cold tier; PIT-safe)

-----------------------------------------

▢ A1.1 Add config keys to `config/data\_sources.yaml`:

```yaml

cold:

&nbsp; format: csv|parquet

&nbsp; enable\_parquet: true

&nbsp; parquet\_schema:

&nbsp;   table: ohlcv

&nbsp;   fields:

&nbsp;     - { name: "ts",      type: "timestamp(ms)", required: true, primary: true }

&nbsp;     - { name: "symbol",  type: "string",        required: true, index: true }

&nbsp;     - { name: "open",    type: "double" }

&nbsp;     - { name: "high",    type: "double" }

&nbsp;     - { name: "low",     type: "double" }

&nbsp;     - { name: "close",   type: "double" }

&nbsp;     - { name: "volume",  type: "double" }

&nbsp; path: data/cold/ohlcv

▢ A1.2 Implement Parquet reader/writer in internal/data/cold.go (new methods; keep CSV path intact):



go

Copy code

type TimeRange struct{ From, To time.Time }



type ParquetOptions struct{

&nbsp;   Compression string // "gzip"|"lz4"|"snappy"|"zstd"|"uncompressed"

&nbsp;   Schema      \*parquet.Schema

&nbsp;   RowGroupSize int // e.g., 128\*1024

}



type ColdStore interface {

&nbsp;   WriteParquet(ctx context.Context, table string, rows any, opts ParquetOptions) error

&nbsp;   ReadParquet(ctx context.Context, table string, tr TimeRange, columns \[]string) (iter.Iterator\[Row], error)

}

Implementation details:



Use a well-supported Go parquet writer/reader (arrow-parquet or x/exp parquet) already vendored/added by go.mod; ensure deterministic row ordering by ts then symbol.



Partitioning: write files under data/cold/<table>/dt=YYYY-MM-DD/part-\*.parquet.



Time-window filtering: pushdown via row-group stats when available, else filter in stream; must not load entire file in memory.



PIT integrity: do not coalesce beyond tr.To; ensure no look-ahead reads (shift if internal buffers are speculative).



Schema validation against parquet\_schema; fail fast on mismatch; log prominent error with sample row.



▢ A1.3 Unit tests:



tests/unit/data/cold\_parquet\_test.go



Generate synthetic OHLCV rows for ±3 days.



Write with options: Compression=\[gzip, lz4]; RowGroupSize small to test pruning.



Read windows: (From=midday D1, To=midday D2) and assert row count \& ts bounds.



Verify mapping types (float64 vs int64 for ts) and schema field order.



▢ A1.4 CLI: enhance existing cold-tier debug command (or add new)



cmd/cryptorun/main.go add cryptorun cold dump --table ohlcv --from 2025-09-01T00:00:00Z --to 2025-09-02T00:00:00Z --columns ts,close --format parquet



Help text documents pitfalls and performance flags.



Acceptance:



Toggle via enable\_parquet works (CSV unaffected).



Tests prove round-trip integrity + correct time-window filtering.



CLI successfully dumps a narrow window without loading full dataset.



A2) Compression \& Streaming (cold→bus)

▢ A2.1 Add compression support (cold tier files, read \& write):



Support gzip and lz4 at minimum; maintain content-type detection by file extension.



Config key: cold.compression: gzip|lz4|none; default gzip.



▢ A2.2 Define streaming message envelope (new package internal/stream/envelope.go):



go

Copy code

type Envelope struct{

&nbsp;   Ts        time.Time   `json:"ts"`

&nbsp;   Symbol    string      `json:"symbol"`

&nbsp;   Source    string      `json:"source"`  // venue or pipe name

&nbsp;   Payload   json.RawMessage `json:"payload"`

&nbsp;   Checksum  string      `json:"checksum"`// hex(blake3(payload||ts||symbol||source))

&nbsp;   Version   int         `json:"version"` // start at 1

}

func (e \*Envelope) ComputeChecksum() string { /\* blake3 \*/ }

func Validate(e \*Envelope) error { /\* non-empty fields, checksum match \*/ }

▢ A2.3 Kafka/Pulsar producers (compile-time selectable):



Add internal/stream/kafka/producer.go and internal/stream/pulsar/producer.go.



Config:



yaml

Copy code

stream:

&nbsp; enabled: true

&nbsp; kind: "kafka" # or "pulsar"

&nbsp; topic: "cryptorun.ohlcv"

&nbsp; batch\_bytes: 1048576

&nbsp; acks: "all"

&nbsp; compression: "lz4"  # broker side

Producer API:



go

Copy code

type Producer interface{ Send(ctx context.Context, topic string, msgs \[]Envelope) error }

func NewProducer(cfg Config) (Producer, error)

Add a cold-tier publisher: internal/data/publish\_cold.go that pages through time windows and streams envelopes.



▢ A2.4 STREAMING.md



Topics naming, partitions, retention (7–30d), broker compression, DLQ pattern, checksum validation at consumer, retry backoff, idempotence.



Example commands and service diagrams.



Acceptance:



Local smoke test with dockerized Kafka/Pulsar: publish 1k envelopes, consumer validates checksums; failure on checksum mismatch.



Config toggles correctly; producer clean shutdown with context cancel.



A3) Multi-Region Replication \& Validation

▢ A3.1 Replication rules design doc (docs/DATA\_MULTI\_REGION.md):



Active-active for hot tier (WS mirrors per region), active-passive for warm/cold with lag SLOs.



Source authority \& cascades; “worst-feed-wins” freshness penalty recorded.



▢ A3.2 Validation layer (new pkg internal/data/validate):



Schema checks: field presence, type match; staleness guard (ts skew > X sec).



Anomaly flags: outliers by robust z-score (MAD) for price/volume; optional quarantine.



▢ A3.3 Prometheus metrics (new file internal/metrics/data.go):



cryptorun\_replication\_lag\_seconds{tier,region,source}



cryptorun\_data\_consistency\_errors\_total{check}



cryptorun\_anomaly\_quarantine\_total{kind}



▢ A3.4 Automated failover tests:



tests/integration/multiregion\_failover\_test.go spins two fake regions; kill leader; assert warm/cold backfill; metrics reflect lag then recovery.



Acceptance:



/metrics exposes counters/gauges; dashboards render (see EPIC D).



Failover test green; doc shows step-by-step for simulated loss.



Commit checklist for A-phases (Claude must output):



&nbsp;CHANGELOG.md updated with features, files, spec refs



&nbsp;Docs updated (list exact files)



&nbsp;Tests added/updated (paths + assertions)



&nbsp;PROGRESS.yaml incremented with timestamps



&nbsp;No aggregator used for microstructure; PIT honored



EPIC B — DEPLOYMENT \& PERSISTENCE LAYER

B1) Database Implementation (Postgres/Timescale)

▢ B1.1 Choose Postgres/Timescale (Timescale if hypertables needed for OHLCV). Create migrations (Goose) under db/migrations:



0001\_create\_trades.sql



sql

Copy code

CREATE TABLE trades (

&nbsp; id BIGSERIAL PRIMARY KEY,

&nbsp; ts TIMESTAMPTZ NOT NULL,

&nbsp; symbol TEXT NOT NULL,

&nbsp; venue TEXT NOT NULL,

&nbsp; side TEXT CHECK (side IN ('buy','sell')),

&nbsp; price DOUBLE PRECISION NOT NULL,

&nbsp; qty DOUBLE PRECISION NOT NULL,

&nbsp; order\_id TEXT,

&nbsp; attributes JSONB DEFAULT '{}'

);

CREATE INDEX ON trades (symbol, ts DESC);

0002\_create\_regime\_snapshots.sql



sql

Copy code

CREATE TABLE regime\_snapshots (

&nbsp; ts TIMESTAMPTZ PRIMARY KEY,

&nbsp; realized\_vol\_7d DOUBLE PRECISION NOT NULL,

&nbsp; pct\_above\_20ma DOUBLE PRECISION NOT NULL,

&nbsp; breadth\_thrust DOUBLE PRECISION NOT NULL,

&nbsp; regime TEXT NOT NULL, -- trending|choppy|highvol|mixed

&nbsp; weights JSONB NOT NULL

);

0003\_create\_premove\_artifacts.sql



sql

Copy code

CREATE TABLE premove\_artifacts (

&nbsp; id BIGSERIAL PRIMARY KEY,

&nbsp; ts TIMESTAMPTZ NOT NULL,

&nbsp; symbol TEXT NOT NULL,

&nbsp; gate\_a BOOLEAN,

&nbsp; gate\_b BOOLEAN,

&nbsp; gate\_c BOOLEAN,

&nbsp; score DOUBLE PRECISION,

&nbsp; factors JSONB,

&nbsp; UNIQUE (ts, symbol)

);

▢ B1.2 Go repository interfaces under internal/persistence:



go

Copy code

type TradesRepo interface {

&nbsp; Insert(ctx context.Context, t Trade) error

&nbsp; ListBySymbol(ctx context.Context, symbol string, tr TimeRange, limit int) (\[]Trade, error)

}

type RegimeRepo interface {

&nbsp; Upsert(ctx context.Context, r RegimeSnapshot) error

&nbsp; Latest(ctx context.Context) (RegimeSnapshot, error)

}

type PremoveRepo interface {

&nbsp; Upsert(ctx context.Context, p PremoveArtifact) error

&nbsp; Window(ctx context.Context, tr TimeRange) (\[]PremoveArtifact, error)

}

Implement postgres/\*\_repo.go with prepared statements, context timeouts, retry on transient errors, PIT by ts.



Wire into data facade alongside file storage (feature-flag via config).



▢ B1.3 CI task to run goose migrations automatically for tests:



Add Makefile or Taskfile.yml target dbtest that spins Postgres and applies migrations; go test uses DSN from env.



Acceptance:



Migrations run cleanly (idempotent).



Unit tests cover basic CRUD; PIT verified (no future reads).



B2) Docker \& Kubernetes

▢ B2.1 Multi-stage Dockerfile at repo root:



Stage 1: build static cryptorun binary.



Stage 2: distroless/base with CA certs; run as non-root; healthcheck.



▢ B2.2 docker-compose.yml (dev): services for cryptorun, redis, postgres; health endpoints; volumes.



▢ B2.3 K8s manifests under deploy/k8s/:



deployment.yaml, service.yaml, configmap.yaml, secret.yaml, ingress.yaml (optional).



Resource requests/limits; liveness/readiness probes; env var wiring from Secrets and ConfigMaps.



▢ B2.4 DEPLOYMENT.md:



Env var matrix; how to store keys in K8s Secrets; sample kubectl apply flow; minimal RBAC.



Acceptance:



docker compose up boots end-to-end locally (health OK).



kubectl manifests lint and deploy to a test namespace (emit commands and outputs).



B3) Secret Management \& Security

▢ B3.1 Move sensitive config to env vars or secret manager (abstraction: internal/secrets).



Prohibit logging secrets; redact patterns.



▢ B3.2 CI checks:



Add commit-time or CI step for secret scanning (e.g., gitleaks).



Add Trivy container scan in pipeline; fail on HIGH/CRITICAL with allowlist for false positives.



▢ B3.3 SECURITY.md:



Checklist for secret handling, dep audit cadence, code review gates, incident response.



Acceptance:



CI fails on leaked secret patterns; Trivy report attached to artifacts.



SECURITY.md exists and is referenced from README.



Commit checklist for B-phases:



&nbsp;CHANGELOG updated



&nbsp;Docs updated (DEPLOYMENT.md, SECURITY.md)



&nbsp;Tests green



&nbsp;PROGRESS.yaml incremented



EPIC C — PROVIDER \& DATA SOURCE EXPANSION

C1) Exchange Adapters (Kraken first-class; aggregator guard)

▢ C1.1 Kraken adapter under internal/providers/kraken/:



REST: trades, ticker.



WebSocket: L1/L2 order book, trades stream.



Retries with backoff/jitter; per-endpoint rate-limit tracking.



Metrics:



cryptorun\_provider\_requests\_total{provider,endpoint,code}



cryptorun\_ws\_reconnects\_total{provider}



cryptorun\_l2\_snapshot\_age\_seconds{provider,symbol}



Normalization: ensure USD pairs default; symbol map (e.g., XBTUSD→BTCUSD) is explicit and tested.



▢ C1.2 Microstructure extraction:



L2 depth within ±2% of mid; spread basis points; VADR computation per venue (no cross-venue blends).



Health signals: staleness, sequence gaps, drift vs trade prints.



▢ C1.3 Aggregator fallback adapters (optional) behind compile-time guards:



Build tags: //go:build with\_agg for any aggregator code.



Entry gates MUST never use aggregator depth/spread. Add a unit test that fails if with\_agg symbol is present and microstructure gates import those types.



▢ C1.4 Tests:



Mocks for REST/WS; deterministically replay fixtures; sequence gap simulation; backoff verify; VADR math.



Acceptance:



Dry-run against live Kraken in a short window (or via recorded fixtures) shows L2/VADR/spread metrics; no aggregator leakage into gates; metrics exported.



C2) Derivatives \& DeFi Metrics

▢ C2.1 Derivatives provider interface internal/providers/derivs:



go

Copy code

type DerivMetrics struct{

&nbsp; Ts time.Time

&nbsp; Symbol string

&nbsp; Venue string

&nbsp; Funding float64

&nbsp; OpenInterest float64

&nbsp; Basis float64 // (futures - spot)/spot over tenor

}

type Derivs interface{

&nbsp; FundingWindow(ctx context.Context, symbol string, tr TimeRange) (\[]DerivMetrics, error)

&nbsp; Latest(ctx context.Context, symbol string) (DerivMetrics, error)

}

Implement Binance/OKX/Coinbase derivatives as available, with PIT alignment (shift to prevent look-ahead).



Unit tests: z-score funding using 30-period lookback; confirm PIT shift(1).



▢ C2.2 DeFi metrics provider internal/providers/defi:



TVL and AMM volumes (The Graph or other free API).



Rate-limit aware; PIT caching.



Normalization to USD; venue/source attribution.



▢ C2.3 Factor integration:



Update factor engine to optionally augment VolumeResidual with on-chain volume when robust; cap influence; maintain Gram–Schmidt ordering; SocialResidual cap +10 after momentum/volume.



▢ C2.4 Mocks \& unit tests:



Error injection for HTTP 5xx/429; verify circuit breakers; verify capped social contribution; verify orthogonalization keeps MomentumCore protected.



Acceptance:



Funding Z, OI, basis available per venue; factor pipeline passes unit tests; orthogonalization order preserved; caps enforced.



Commit checklist for C-phases:



&nbsp;CHANGELOG updated



&nbsp;Docs updated (provider guides)



&nbsp;Tests added/updated



&nbsp;PROGRESS.yaml incremented



EPIC D — REPORTING, MONITORING \& OBSERVABILITY

D1) Performance \& Portfolio Reporting

▢ D1.1 Methods (package internal/report/perf):



Spec-P\&L vs raw P\&L (include fees/slippage toggles).



Sharpe (annualized), max drawdown, hit rate, exposure-weighted returns.



▢ D1.2 CLI:



cryptorun report --performance --from <ts> --to <ts> --format md|csv --outfile <path>



cryptorun report --portfolio --format md|csv --outfile <path>



▢ D1.3 Persistence fetchers:



Pull positions/returns from Postgres via internal/persistence.



▢ D1.4 Output templates:



artifacts/templates/perf\_report.md.tmpl and .csv.tmpl.



Portfolio report: holdings table (qty, sector, beta, correlation), expiry dates, triggered alerts, exit status.



Charts (matrices, sector pies) using a non-GUI plotting lib (PNG/SVG artifacts).



▢ D1.5 Threshold alerting:



If Sharpe < 1.0 (configurable) or pairwise corr > 0.65, emit alert to alerting bus (Slack/Email hooks in D2).



Acceptance:



Example reports generated in artifacts/reports/ with deterministic test data; tests assert summary stats and threshold triggers.



D2) Metrics, Dashboards \& Health

▢ D2.1 Prometheus metrics (in addition to A3/C1):



cryptorun\_rate\_limit\_remaining{provider,endpoint}



cryptorun\_cache\_hit\_ratio{tier}



cryptorun\_latency\_seconds\_bucket{component}



cryptorun\_sse\_throughput{stream}



▢ D2.2 Grafana dashboards (JSON in deploy/grafana/):



Panels: data tiers, scheduler, Premove detector, regime tuner, provider health, error rates.



▢ D2.3 Alert thresholds:



Error rate > X%, latency p95/p99, hit-rate regression triggers.



▢ D2.4 cryptorun health CLI:



Prints current health status and last error (aggregates provider health, replication lag, queue backlogs).



Acceptance:



/metrics shows new counters/gauges; Grafana JSON checked-in; health command returns a single-line OK/FAIL with reason.



D3) Load \& Regression Testing

▢ D3.1 Load tests (k6 or Locust) scripts in tests/load/:



Simulate high-throughput scans/backtests via CLI or HTTP if present; record CPU/Mem/I/O/Net.



▢ D3.2 Regression suite:



Compare hit-rate, correlation, latency vs previous baseline (store baseline in artifacts/baselines/).



Generate HTML/MD diffs with red/green indicators; CI fails on threshold breach.



Acceptance:



CI job executes load sample (short duration) on PR; regression gates protect critical metrics.



D4) Documentation \& Governance

▢ D4.1 Docs:



Expand DEPLOYMENT.md (end-to-end), MONITORING.md (metrics, dashboards, alerts), SECURITY.md (policies).



User guides for performance/portfolio reports with screenshots (saved artifacts).



Add STREAMING.md (from A2).



▢ D4.2 Changelog automation:



Pre-commit or pre-push hook to append CHANGELOG stubs from filenames/commit tags.



Commit message convention: <scope>: <subject> enforced with commit-lint (CI).



▢ D4.3 Docs-first CI check:



CI fails if code references undocumented features (grep tags, or require doc label files).



Acceptance:



Hooks installed and tested; CI lint passes; docs parity check green.



Commit checklist for D-phases:



&nbsp;CHANGELOG updated



&nbsp;Docs updated (list)



&nbsp;Tests/load/regression artifacts attached



&nbsp;PROGRESS.yaml incremented



EVIDENCE LEDGER (Claude must emit per subphase)

───────────────────────────────────────────────

Feature | Status | Evidence Quote | File:Line | Spec Ref



GIT COMMIT CHECKLIST (Claude must include in every patch output)

────────────────────────────────────────────────────────────────



&nbsp;CHANGELOG.md updated (today) with feature/fix summary, file paths, spec refs



&nbsp;Docs updated (paths)



&nbsp;Tests added/updated (paths + brief assertions)



&nbsp;Breaking changes: \[none|describe]



&nbsp;PROGRESS.yaml milestone incremented; QA runner passes



pgsql

Copy code



---



\### Notes for your future self

\- This is exhaustive by design. Claude will \*\*auto-continue\*\* from A→B→C→D, splitting into subphases if the output gets long.  

\- It will refuse to re-implement already-done work (pre-flight), report \*ALREADY IMPLEMENTED\* with file:line receipts, and only patch gaps.  

\- It bakes in your non-negotiables: exchange-native microstructure, USD pairs, Kraken default, MomentumCore protection, Gram–Schmidt ordering, capped social, PIT integrity, hot/warm/cold tiers, VADR/fatigue/late-fill/freshness gates, and docs/CHANGELOG enforcement.



If you want, I can also generate \*\*seed fixtures\*\* (sample OHLCV/deri

