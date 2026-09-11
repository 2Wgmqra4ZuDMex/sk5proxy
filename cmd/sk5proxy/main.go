package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"sk5proxy/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, logger); err != nil {
		logger.Error("service.failed", "error", err)
		os.Exit(1)
	}
}
