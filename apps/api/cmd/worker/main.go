package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/seraph/triage/apps/api/internal/config"
	"github.com/seraph/triage/apps/api/internal/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger := telemetry.SetupLogger(cfg.LogLevel, cfg.Environment)
	logger.Info("starting triage worker",
		slog.String("env", cfg.Environment),
		slog.Int("concurrency", cfg.WorkerConcurrency),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Worker process event loop stub for Phase 1
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	logger.Info("worker listening for investigation jobs...")

	for {
		select {
		case <-ctx.Done():
			logger.Info("worker shutting down gracefully...")
			return
		case <-ticker.C:
			logger.Debug("worker heartbeat check")
		}
	}
}
