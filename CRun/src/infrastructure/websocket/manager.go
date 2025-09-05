package websocket

import (
    "context"
    "time"

    "github.com/rs/zerolog/log"
)

// Manager is a generic reconnecting WS loop skeleton.
type Manager struct {
    Name string
}

func (m *Manager) Run(ctx context.Context) {
    backoff := time.Second
    for {
        select {
        case <-ctx.Done():
            log.Info().Str("ws", m.Name).Msg("shutdown")
            return
        default:
        }
        log.Info().Str("ws", m.Name).Msg("connect (placeholder)")
        // ... establish connection and read loop ...
        time.Sleep(backoff)
        if backoff < 30*time.Second {
            backoff *= 2
        }
    }
}

