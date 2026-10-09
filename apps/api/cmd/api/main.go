package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/seraph/triage/apps/api/internal/config"
	"github.com/seraph/triage/apps/api/internal/server"
	"github.com/seraph/triage/apps/api/internal/server/handlers"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger := telemetry.SetupLogger(cfg.LogLevel, cfg.Environment)
	logger.Info("initializing triage api service",
		slog.String("env", cfg.Environment),
		slog.String("http_addr", cfg.HTTPAddr),
	)

	health := handlers.NewHealthHandler()

	// Initialize data layer
	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
	st, err := store.NewStore(initCtx, cfg.DatabaseURL)
	initCancel()
	if err != nil {
		logger.Warn("database connection failed during startup, registering degraded health check",
			slog.String("error", err.Error()),
		)
		health.RegisterCheck("database", func(ctx context.Context) error {
			return err
		})
	} else {
		logger.Info("database pool connected and ready")
		health.RegisterCheck("database", st.Ping)
		defer st.Close()
	}

	srv := server.NewServer(cfg, logger, health)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Start(ctx); err != nil {
		logger.Error("server fatal error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
