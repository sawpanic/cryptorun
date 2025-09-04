package cprotocol

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func Execute(ctx context.Context) error {
	root := &cobra.Command{Use: "cprotocol", Short: "CProtocol CLI"}
	var (
		exchange string
		pairs string
		dryRun bool
		blacklist []string
		pause bool
		forceRegime string
	)
	root.PersistentFlags().StringVar(&exchange, "exchange", "kraken", "primary exchange")
	root.PersistentFlags().StringVar(&pairs, "pairs", "USD", "pair filter (USD-only)")
	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "dry run")
	root.PersistentFlags().StringSliceVar(&blacklist, "blacklist", nil, "emergency blacklist symbols")
	root.PersistentFlags().BoolVar(&pause, "pause", false, "global pause")
	root.PersistentFlags().StringVar(&forceRegime, "force-regime", "", "force regime name")

	root.AddCommand(scanCmd(ctx))
	root.AddCommand(backtestCmd(ctx))
	root.AddCommand(monitorCmd(ctx))
	root.AddCommand(healthCmd(ctx))

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Info().Msg("cprotocol starting")
	return root.Execute()
}

func scanCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{Use: "scan", RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("scan placeholder (hot/warm/cold mesh)")
		return nil
	}}
}

func backtestCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{Use: "backtest", RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("backtest placeholder")
		return nil
	}}
}

func monitorCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{Use: "monitor", RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("monitor placeholder (venue health, cache, ws)")
		return nil
	}}
}

func healthCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{Use: "health", RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("health OK")
		return nil
	}}
}
