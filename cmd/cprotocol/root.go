package main

import (
    "context"

    "github.com/rs/zerolog/log"
    "github.com/spf13/cobra"
)

func Execute(ctx context.Context) error {
    root := &cobra.Command{Use: "cprotocol", Short: "CProtocol CLI"}
    root.AddCommand(scanCmd(ctx))
    log.Info().Msg("cprotocol starting")
    return root.ExecuteContext(ctx)
}

