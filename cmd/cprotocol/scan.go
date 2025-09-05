package main

import (
    "context"
    "fmt"
    "sort"
    "strings"
    "time"

    "github.com/rs/zerolog/log"

    "github.com/sawpanic/CProtocol/data"
    "github.com/sawpanic/CProtocol/exchanges/binance"
    "github.com/sawpanic/CProtocol/regime"
    "github.com/sawpanic/CProtocol/signals"
    "github.com/sawpanic/CProtocol/ui"
    "github.com/spf13/cobra"
)

func scanCmd(ctx context.Context) *cobra.Command {
    var (
        pairs  string
        venue  string
        window string
        limit  int
    )
    cmd := &cobra.Command{
        Use:   "scan",
        Short: "Scan for 6-48h momentum opportunities",
        RunE: func(cmd *cobra.Command, args []string) error {
            syms := parsePairs(pairs)
            if len(syms) == 0 { return fmt.Errorf("no pairs provided") }

            // book provider (binance vertical slice)
            var book interface{ Metrics(context.Context, string) (binance.OrderbookMetrics, error) }
            switch strings.ToLower(venue) {
            case "binance":
                book = binance.NewBookProvider()
            default:
                return fmt.Errorf("unsupported venue: %s", venue)
            }

            // data source
            ds := data.NewPrices()

            // regime detection (lite)
            reg := regime.DetectDefaultChoppy()

            type row struct{
                Pair string
                Mom  float64
                Met  binance.OrderbookMetrics
                Badges []string
            }
            var rows []row

            for _, p := range syms {
                // prices
                closes, vols, err := ds.Klines(cmd.Context(), venue, p, window, 200)
                if err != nil { log.Warn().Err(err).Str("pair", p).Msg("klines fetch failed"); continue }
                // momentum core
                mom := signals.MomentumCore(closes)
                // ATR, RSI, accel
                atr := signals.ATR(closes, 14)
                rsi := signals.RSI(closes, 14)
                accel := signals.Accel4h(closes)
                // VADR proxy
                vadr := signals.VADR(vols)
                // orderbook metrics
                met, err := book.Metrics(cmd.Context(), p)
                if err != nil { log.Warn().Err(err).Str("pair", p).Msg("book metrics failed"); continue }
                // gates
                gr := signals.EvaluateGates(signals.GateInputs{
                    Close: closes, Volumes: vols,
                    ATR1h: atr, RSI4h: rsi, Accel4h: accel, VADR: vadr,
                    SpreadBps: met.SpreadBps, DepthUSD2pc: met.DepthUSD2pc,
                    TriggerPrice: signals.Last(closes), Now: time.Now(), SignalTime: time.Now().Add(-10*time.Second),
                })
                if !gr.Pass { log.Info().Str("pair", p).Str("reason", gr.Reason).Msg("gated out"); continue }
                // badges
                badges := []string{
                    fmt.Sprintf("spread=%.1fbps", met.SpreadBps),
                    fmt.Sprintf("depth@2%%=$%.0fk", met.DepthUSD2pc/1000),
                    fmt.Sprintf("vadr=%.2fx", vadr),
                    fmt.Sprintf("fresh<=2bars"),
                }
                rows = append(rows, row{Pair: p, Mom: mom, Met: met, Badges: badges})
            }

            sort.Slice(rows, func(i,j int) bool { return rows[i].Mom > rows[j].Mom })
            if limit > 0 && len(rows) > limit { rows = rows[:limit] }

            ui.PrintHeader(reg, 1, 1)
            // Convert rows to expected type
            printRows := make([]struct{Pair string; Mom float64; Met any; Badges []string}, len(rows))
            for i, r := range rows {
                printRows[i] = struct{Pair string; Mom float64; Met any; Badges []string}{
                    Pair: r.Pair,
                    Mom: r.Mom,
                    Met: r.Met,
                    Badges: r.Badges,
                }
            }
            ui.PrintTable(printRows)
            return nil
        },
    }
    cmd.Flags().StringVar(&pairs, "pairs", "BTCUSDT,ETHUSDT", "comma-separated pairs")
    cmd.Flags().StringVar(&venue, "venue", "binance", "venue: binance|coinbase|okx")
    cmd.Flags().StringVar(&window, "window", "4h", "bar window: 1h|4h|12h|24h")
    cmd.Flags().IntVar(&limit, "limit", 20, "max ranks")
    return cmd
}

func parsePairs(s string) []string {
    v := strings.Split(s, ",")
    out := make([]string,0,len(v))
    for _, x := range v { x = strings.TrimSpace(x); if x != "" { out = append(out, strings.ToUpper(x)) } }
    return out
}

