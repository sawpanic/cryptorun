package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

    "cprotocol/cmd/cprotocol"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := cprotocol.Execute(ctx); err != nil {
		fmt.Println(err)
		log.Error().Err(err).Msg("exit with error")
		os.Exit(1)
	}
}
