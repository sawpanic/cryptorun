package universe

import (
    "context"
    "errors"
    "strings"
    "time"
)

// KrakenLite defines the minimal methods the universe builder needs from the Kraken client.
type KrakenLite interface {
    // ListUSDPairs returns tradable symbols on Kraken that end with /USD and their 24h USD volumes.
    ListUSDPairs(ctx context.Context) ([]Pair24h, error)
    // HasOrderBook indicates if a pair currently has an active order book (non-empty).
    HasOrderBook(ctx context.Context, symbol string) (bool, error)
    // HasMinHistoryDays checks if the pair has at least N days of price history.
    HasMinHistoryDays(ctx context.Context, symbol string, days int) (bool, error)
}

type Criteria struct {
    OnlyUSD           bool
    MinUSDVolume      float64
    MinHistoryDays    int
    RequireOrderBook  bool
    ExcludeStableBase bool
}

type Pair24h struct {
    Symbol      string
    USDVolume24 float64
}

type Universe struct {
    AsOf    time.Time
    Symbols []string
}

type Builder struct {
    k KrakenLite
}

func NewBuilder(k KrakenLite) *Builder { return &Builder{k: k} }

func (b *Builder) BuildDaily(ctx context.Context, c Criteria) (*Universe, error) {
    pairs, err := b.k.ListUSDPairs(ctx)
    if err != nil {
        return nil, err
    }
    var out []string
    for _, p := range pairs {
        if c.OnlyUSD && !strings.HasSuffix(strings.ToUpper(p.Symbol), "/USD") {
            continue
        }
        if c.ExcludeStableBase {
            base := strings.ToUpper(strings.TrimSuffix(p.Symbol, "/USD"))
            switch base { // no stablecoin bases
            case "USDT", "USDC", "DAI", "BUSD", "TUSD", "USDP":
                continue
            }
        }
        if p.USDVolume24 < c.MinUSDVolume {
            continue
        }
        if c.RequireOrderBook {
            ok, err := b.k.HasOrderBook(ctx, p.Symbol)
            if err != nil || !ok {
                continue
            }
        }
        if c.MinHistoryDays > 0 {
            ok, err := b.k.HasMinHistoryDays(ctx, p.Symbol, c.MinHistoryDays)
            if err != nil || !ok {
                continue
            }
        }
        out = append(out, p.Symbol)
    }
    if len(out) == 0 {
        return nil, errors.New("no symbols matched USD universe criteria")
    }
    return &Universe{AsOf: time.Now().UTC(), Symbols: out}, nil
}

