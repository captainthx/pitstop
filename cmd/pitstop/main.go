package main

import (
	"context"
	"os"
	"os/signal"
	"pitstop/internal/cli"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(cli.ExecuteContext(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
