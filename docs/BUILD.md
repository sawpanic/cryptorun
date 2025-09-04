# Build and Run (Go Only)

This project must be built using Go tooling only. Do not use PowerShell or Python build scripts.

## One‑Time Environment
- Install Go 1.21+ and ensure `go` is on your PATH.
- Network: outbound HTTPS to `api.kraken.com` (and optional providers).

## Build With Jerusalem Timestamp (Go‑only)
The binary shows a Jerusalem timestamp in the banner via a `-ldflags` injection.

1) Generate the stamp using Go:
- Cross‑platform (prints a single line timestamp):
  - `go run ./tools/buildstamp`
  - Example output: `2025-09-04 11:32 Jerusalem`

2) Build with the stamp:
- Paste the exact stamp from step 1:
  - `go build -ldflags "-X main.BuildStamp=2025-09-04 11:32 Jerusalem" -o cprotocol.exe .`

Notes:
- Keep the exact spacing and capitalization in the stamp.
- If `Asia/Jerusalem` tz cannot be loaded at runtime, the app falls back to UTC.

## Run (Menu‑Only)
- `cprotocol.exe`
- Command‑line arguments are ignored by design; use the interactive menu.

## Live‑Data QA Quick Check
- Pair filtering prints non‑zero “Tradable pairs after filters” and a positive pass ratio.
- No “CRITICAL … using emergency static data” warnings.
- Momentum filter summary prints: `MOMENTUM FILTER: X flat assets excluded, Y moving assets remain`.
- Factor breakdown table appears below results (MOMO_z / TECH_orth / VOL+LIQ / QUAL_res / SOC_res / VOL(USD) / COMPOSITE).

## Optional API Keys
- Set CoinMarketCap key to reduce fallback use:
  - Windows: `setx COINMARKETCAP_API_KEY "YOUR_KEY"`
- Restart the app after setting env vars.

## Rebuild
- Repeat the two‑step stamp + build whenever you ship a new binary:
  - `go run ./tools/buildstamp`
  - `go build -ldflags "-X main.BuildStamp=<STAMP>" -o cprotocol.exe .`

