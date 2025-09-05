package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"

    domainPairs "cprotocol/domain/pairs"
    "cprotocol/application/universe"
    "cprotocol/infrastructure/apis/kraken"
    httpiface "cprotocol/interfaces/http"
)

const (
    appName = "CProtocol"
    version = "v3.2.1"
)

func main() {
    // Structured logging with sane defaults
    zerolog.TimeFieldFormat = time.RFC3339
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.Kitchen})

    if len(os.Args) < 2 {
        usage()
        os.Exit(1)
    }

    cmd := os.Args[1]
    args := os.Args[2:]

    // Global flags
    fs := flag.NewFlagSet(cmd, flag.ExitOnError)
    exchange := fs.String("exchange", "kraken", "Primary exchange (default: kraken)")
    pairs := fs.String("pairs", "USD-only", "Pair filter (USD-only)")
    dryRun := fs.Bool("dry-run", false, "Do not execute external side effects")
    regimeOverride := fs.String("regime", "", "Force regime override")
    // Human override actions
    blacklist := fs.String("blacklist", "", "Temporarily blacklist a symbol (24h)")
    pause := fs.Bool("pause", false, "Pause scanning")

    _ = fs.Parse(args)

    switch strings.ToLower(cmd) {
    case "scan":
        runScan(*exchange, *pairs, *dryRun, *regimeOverride, *blacklist, *pause)
    case "backtest":
        log.Info().Str("app", appName).Str("version", version).Msg("backtest stub — use internal backtest tooling")
    case "monitor":
        addr := ":8088"
        if v := os.Getenv("METRICS_ADDR"); v != "" { addr = v }
        log.Info().Str("addr", addr).Msg("starting monitor server")
        if err := httpiface.RunUntilSignal(addr); err != nil { log.Warn().Err(err).Msg("monitor shutdown") }
    case "health":
        log.Info().Str("exchange", *exchange).Msg("OK — Kraken primary configured")
    default:
        usage()
        os.Exit(1)
    }
}

func usage() {
    fmt.Println("Usage: cprotocol <scan|backtest|monitor|health> [flags]")
    fmt.Println("  --exchange kraken   --pairs USD-only   --dry-run   --regime NAME   --blacklist SYMBOL   --pause")
}

func runScan(exchange, pairsFilter string, dryRun bool, regimeOverride, blacklist string, pause bool) {
    log.Info().Str("exchange", exchange).Str("pairs", pairsFilter).Bool("dry_run", dryRun).Msg("Starting scan...")
    if pause {
        log.Warn().Msg("scan paused by operator")
        return
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if strings.ToLower(exchange) != "kraken" {
        log.Warn().Msg("Non-kraken exchange requested; Kraken is primary. Proceeding with Kraken.")
    }

    // Kraken API client (primary). Real implementation lives behind interface.
    k := kraken.NewClient(kraken.Config{Primary: true})

    // Build universe (USD-only on Kraken, no stablecoin bases)
    builder := universe.NewBuilder(k)
    uni, err := builder.BuildDaily(ctx, universe.Criteria{
        OnlyUSD:           true,
        MinUSDVolume:      200_000,
        MinHistoryDays:    3,
        RequireOrderBook:  true,
        ExcludeStableBase: true,
    })
    if err != nil {
        log.Error().Err(err).Msg("failed building universe")
        os.Exit(1)
    }

    // Apply explicit USD-only and stable exclusions defensively
    symbols := make([]string, 0, len(uni.Symbols))
    for _, s := range uni.Symbols {
        if domainPairs.IsValidUSDPair(s) {
            symbols = append(symbols, s)
        }
    }

    log.Info().Int("count", len(symbols)).Msg("universe ready (USD pairs on Kraken)")
    if len(symbols)==0 { log.Info().Msg("Top 10: none (universe empty)"); return }
    // Print a simple Top 10 table (unscored stub)
    topN := 10
    if len(symbols) < topN { topN = len(symbols) }
    fmt.Println("\nTop 10 (unscored)")
    fmt.Println("#   SYMBOL")
    for i := 0; i < topN; i++ {
        fmt.Printf("%2d  %s\n", i+1, symbols[i])
    }
    if topN == 0 {
        fmt.Println("(none)")
    }
    log.Info().Msg("Scan stub complete — scoring engine wiring pending.")
}
