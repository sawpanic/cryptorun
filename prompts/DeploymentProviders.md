Deployment & Providers
Prompt 1 — DEPLOY.DB.PERSISTENCE
HOUSE RULES
––––––––––––––––––––––––––––––––––––––––––––
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT
ALREADY IMPLEMENTED PRE-FLIGHT — STOP if code/docs already implement the item; output ALREADY IMPLEMENTED + path/lines
WRITE-SCOPE — Patch-only, list exact files/lines; no rewrites
CITATIONS — Every claim must cite docs/specs:contentReference[oaicite:1]{index=1}:contentReference[oaicite:2]{index=2}

TASK: Implement the **Database & Persistence Layer** per roadmap:contentReference[oaicite:3]{index=3}.

PHASES (Serial; agent may auto-split if tokens near limit):

1. **Schema & Migrations**
   - Choose Postgres/TimescaleDB (open source, free tier).
   - Create schemas/tables:  
     • `trades` (timestamp, pair, venue, price, size, side, provenance)  
     • `regime_snapshots` (regime, vol, %>20MA, breadth, ts, metadata)  
     • `premove_artifacts` (asset, score, gates, factors JSON, state, ts).
   - Write Goose migration scripts (`db/migrations/*.sql`).
   - Add to CI/CD pipeline (build/test runs `goose up` on test DB).

2. **Go Repository Interfaces**
   - Implement repository interfaces in `internal/infrastructure/db/`.
   - CRUD for trades, snapshot insert/query by ts, premove artifact persistence.
   - Add unit tests with fakes (no live DB in tests).

3. **Data Facade Update**
   - Extend data facade: write to Postgres in addition to file storage.
   - Make DB optional via config/env var (`PG_DSN`:contentReference[oaicite:4]{index=4}).

4. **Docs & QA**
   - Update `docs/DATA_PERSISTENCE.md` with schemas, migration steps, env vars.
   - Add QA tests for point-in-time integrity, PIT replays.

ACCEPTANCE
–––––––––––––––––––––––––
- Migrations run without errors (`goose up`).
- Interfaces compile and integrate with data facade.
- CI passes with Postgres disabled (file-only mode).
- Docs updated, CHANGELOG entry added.


Git Commit Checklist for Prompt 1

 Update CHANGELOG.md with DB persistence features

 Add docs/DATA_PERSISTENCE.md

 Add tests: tests/unit/infrastructure/db/*

 Patch only: internal/infrastructure/db/*.go, config/config.yaml, db/migrations/*

 PROGRESS.yaml milestone bump (Deployment layer)

Prompt 2 — DEPLOY.DOCKER.K8S.SECURITY
HOUSE RULES …
TASK: Implement **Docker, Kubernetes, and Security/Secrets**:contentReference[oaicite:5]{index=5}:contentReference[oaicite:6]{index=6}.

PHASES:

1. **Docker & Compose**
   - Multi-stage Dockerfile in repo root (`Dockerfile`): build → runtime.
   - docker-compose.yml with services:  
     • `cryptorun` (Go service)  
     • `redis`  
     • `postgres`  
   - Healthchecks for each.

2. **Kubernetes Manifests**
   - Manifests in `deploy/k8s/`:  
     • Deployments (cryptorun, redis, postgres)  
     • Services (ClusterIP for redis/db, LoadBalancer for cryptorun)  
     • ConfigMaps for non-secret config  
     • Secrets for API keys/env vars.

3. **Secret Management & Security**
   - Switch configs to env vars or secret manager.
   - Add CI linter to block logging of secrets.
   - Integrate Trivy vulnerability scanner into CI.

4. **Docs & SECURITY.md**
   - Write `SECURITY.md` checklist: secret handling, reviews, dependency scans.

ACCEPTANCE
–––––––––––––––––––––––––
- `docker build` and `docker-compose up` succeed locally.
- K8s manifests deploy into Minikube/kind.
- Secrets mounted via K8s Secret, not in code.
- SECURITY.md reviewed in PR.


Git Commit Checklist for Prompt 2

 Add Dockerfile, docker-compose.yml, deploy/k8s/*

 Add SECURITY.md

 Integrate Trivy in CI (.github/workflows/ci.yml)

 Update docs: docs/DEPLOYMENT.md

 PROGRESS.yaml milestone bump (Deployment)

Prompt 3 — PROVIDERS.EXCHANGES.DEFI
HOUSE RULES …
TASK: Implement **Provider & Data Source Expansion**:contentReference[oaicite:7]{index=7}:contentReference[oaicite:8]{index=8}.

PHASES:

1. **Exchange Adapters**
   - Complete Kraken adapter: L1/L2 order book, trades, ticker with retries & metrics.
   - Add DEX ingestion via DEXScreener API (volume/events only; NOT depth/spread — aggregator ban enforced:contentReference[oaicite:9]{index=9}).
   - Add fallback aggregator adapters (CoinGecko, CoinPaprika); wrap in compile-time guards.

2. **Derivative & DeFi Metrics**
   - Extend derivatives provider: open interest, funding rate, basis (Binance, OKX, Bybit).
   - Implement DeFi provider: TVL + AMM volumes (DefiLlama/DEXScreener).
   - Update factor definitions: VolumeResidual adjusted for on-chain volume:contentReference[oaicite:10]{index=10}.

3. **Guards & Tests**
   - Write tests confirming aggregator ban: ensure depth/spread never from DEXScreener.
   - Add mocks for derivatives & DeFi metrics.
   - Verify error handling paths.

4. **Docs**
   - Update `docs/DATA_SOURCES.md` and `docs/PROVIDERS.md` with API endpoints, rate limits, cache TTLs:contentReference[oaicite:11]{index=11}.

ACCEPTANCE
–––––––––––––––––––––––––
- Kraken adapter streams healthy L1/L2 and trades.
- Derivatives metrics fetch and parse correctly.
- DeFi metrics flow into VolumeResidual.
- Tests pass: aggregator ban enforced.
- Docs updated, CHANGELOG entry created.


Git Commit Checklist for Prompt 3

 Add/patch: internal/infrastructure/providers/kraken.go, dexscreener.go, derivatives.go, defi.go

 Add tests: tests/unit/providers/*

 Update docs: docs/DATA_SOURCES.md, docs/PROVIDERS.md

 Update CHANGELOG.md with provider expansion

 PROGRESS.yaml milestone bump (Provider expansion)