package ui

import (
    "fmt"
)

func PrintHeader(regime string, healthy, total int) {
    fmt.Printf("MOMENTUM SIGNALS (6-48h Opportunities) | Regime: %s | APIs: %d/%d Healthy\n", regime, healthy, total)
    fmt.Println("═════════════════════════════════════════════════════════════════════════════")
}

type Row interface {
}

// PrintTable renders a simple table used by scan.go
func PrintTable(rows []struct{ Pair string; Mom float64; Met any; Badges []string }) {
    fmt.Printf("%-8s %-8s %-10s %-12s %s\n", "#", "PAIR", "MOMENTUM", "SPREAD/DEPTH", "BADGES")
    for i, r := range rows {
        var spread, depth string
        switch m := r.Met.(type) {
        case interface{ GetSpread() float64; GetDepth() float64 }:
            spread = fmt.Sprintf("%.1fbps", m.GetSpread())
            depth = fmt.Sprintf("$%.0fk", m.GetDepth()/1000)
        default:
            // attempt generic via dynamic (binance metrics has fields)
            spread = "?bps"
            depth = "$?"
        }
        fmt.Printf("%-8d %-8s %-10.2f %-12s %s\n", i+1, r.Pair, r.Mom, spread+"/"+depth, fmt.Sprintf("%v", r.Badges))
    }
}

