package websocket

import (
	"context"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type KrakenWS struct {
	URL string
	conn *websocket.Conn
}

func (k *KrakenWS) Connect(ctx context.Context) error {
	u, _ := url.Parse(k.URL)
	var d websocket.Dialer
	c, _, err := d.DialContext(ctx, u.String(), nil)
	if err != nil { return err }
	k.conn = c
	return nil
}

func (k *KrakenWS) ReadLoop(ctx context.Context, handle func([]byte)) {
	retry := 0
	for {
		if k.conn == nil {
			if err := k.Connect(ctx); err != nil {
				back := time.Duration(1<<retry) * time.Second
				if back > 30*time.Second { back = 30*time.Second }
				log.Warn().Err(err).Dur("backoff", back).Msg("kraken ws reconnect")
				t := time.NewTimer(back); select { case <-ctx.Done(): return; case <-t.C: }
				retry++
				continue
			}
			retry = 0
		}
		_, data, err := k.conn.ReadMessage()
		if err != nil {
			log.Error().Err(err).Msg("ws read error; closing and retrying")
			k.conn.Close(); k.conn = nil
			continue
		}
		handle(data)
	}
}
