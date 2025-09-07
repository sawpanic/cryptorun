OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
PROMPT_ID=SUPERPACK.premove.v33.part1.PART_1_OF_3

CONTINUATION RULES
- This is part 1/3.
- If prior parts referenced WRITE-SCOPE/PATCH-ONLY, keep them identical here.
- Begin where the last part ended; do not repeat previous content.
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT
SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=SUPERPACK.PREMOVE.V33.PART1

WRITE-SCOPE — ALLOW ONLY:
  - src/domain/premove/ports/**
  - src/infrastructure/percentiles/**
  - src/domain/premove/cvd/**
  - src/domain/premove/proxy/**
  - src/application/premove/runner.go
  - internal/testdata/premove/**
  - tests/unit/premove/**
  - docs/PREMOVE.md
  - docs/*CHANGELOG*
PATCH-ONLY — Emit unified diffs or full file bodies only; no prose.

SUPERPACK: Premove v3.3 — Percentiles + CVD Residuals + Supply-Squeeze Proxy + Runner Wiring + Tests
INDEX
  [S1] PRE-FLIGHT
  [S2] Percentile Engine
  [S3] CVD Residuals
  [S4] Supply-Squeeze Proxy
  [S5] Runner Wiring
  [S6] Tests & Fixtures
  [S7] Docs + CHANGELOG
  [S8] POST-FLIGHT

CONVENTIONS
- STOP-ON-FAIL if any step fails.
- Deterministic fakes only; no network.
- GREEN-ONLY MERGE: must end with go fmt/vet/test green.

[S1] PR