package kraken

import (
    "context"
    "time"

    "github.com/rs/zerolog/log"
    appuni "cprotocol/application/universe"
)

type Config struct {
    Primary bool
    // future: credentials, rate limits, endpoints
}

type Client struct {
    cfg Config
}

func NewClient(cfg Config) *Client { return &Client{cfg: cfg} }

// ListUSDPairs returns a minimal placeholder set to allow compile-time wiring.
// In production, this will hit Kraken REST and obey rate limits and caching.
func (c *Client) ListUSDPairs(ctx context.Context) ([]appuni.Pair24h, error) {
    // Placeholder: small set for bootstrapping; real impl queries Kraken.
    return []appuni.Pair24h{
        {Symbol: "XBT/USD", USDVolume24: 2_500_000},
        {Symbol: "ETH/USD", USDVolume24: 1_900_000},
        {Symbol: "SOL/USD", USDVolume24: 850_000},
    }, nil
}

func (c *Client) HasOrderBook(ctx context.Context, symbol string) (bool, error) {
    // Placeholder true; real impl: Kraken order book snapshot check
    return true, nil
}

func (c *Client) HasMinHistoryDays(ctx context.Context, symbol string, days int) (bool, error) {
    // Placeholder true; real impl: Kraken OHLC check
    return true, nil
}

// WebSocket manager skeleton with reconnect/backoff (interface-free placeholder)
func (c *Client) RunWebSocket(ctx context.Context) {
    backoff := time.Second
    for {
        select {
        case <-ctx.Done():
            log.Info().Msg("Kraken WS: context canceled")
            return
        default:
        }
        // Connect placeholder
        log.Info().Msg("Kraken WS: connecting (placeholder)")
        // On error: backoff and retry
        time.Sleep(backoff)
        if backoff < 30*time.Second {
            backoff *= 2
        }
    }
}
