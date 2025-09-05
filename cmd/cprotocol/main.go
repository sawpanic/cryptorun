package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

func main() {
    zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()
    if err := Execute(ctx); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}

