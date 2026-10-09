package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/seraph/triage/apps/api/internal/auth"
	"github.com/seraph/triage/apps/api/internal/config"
	"github.com/seraph/triage/apps/api/internal/ingest"
	"github.com/seraph/triage/apps/api/internal/server"
	"github.com/seraph/triage/apps/api/internal/server/handlers"
	"github.com/seraph/triage/apps/api/internal/server/middleware"
	"github.com/seraph/triage/apps/api/internal/server/sse"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/telemetry"
	"github.com/seraph/triage/apps/api/internal/tools"
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

	// Initialize Redis for token bucket rate limiter and async jobs
	var rdb *redis.Client
	if redisOpts, err := redis.ParseURL(cfg.RedisURL); err == nil {
		rdb = redis.NewClient(redisOpts)
		health.RegisterCheck("redis", func(ctx context.Context) error {
			return rdb.Ping(ctx).Err()
		})
		defer rdb.Close()
	}

	// Core services
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	rateLimiter := middleware.NewRateLimiter(rdb, 120, time.Minute)
	broker := sse.NewBroker()

	var queue ingest.Queue
	if rdb != nil {
		queue = ingest.NewRedisQueue(rdb, "stream:investigations", "triage-workers", "api-producer")
	} else {
		queue = ingest.NewMemoryQueue(100)
	}

	// Tool registry with standard diagnostic tools
	toolsPolicy := tools.NewAllowlistPolicyProvider()
	toolsRegistry := tools.NewToolRegistry(toolsPolicy, 15*time.Second)
	_ = toolsRegistry.Register(tools.NewLogSearchTool(ingest.NewFileLogAdapter("logs")))
	_ = toolsRegistry.Register(tools.NewRecentDeploysTool(nil))
	_ = toolsRegistry.Register(tools.NewServiceTopologyTool(nil))
	_ = toolsRegistry.Register(tools.NewQueryMetricsTool(nil))
	_ = toolsRegistry.Register(tools.NewSearchRunbooksTool(st))
	_ = toolsRegistry.Register(tools.NewSimilarIncidentsTool(st))

	srv := server.NewServer(cfg, logger, health, st, jwtManager, rateLimiter, broker, queue, toolsRegistry)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Start(ctx); err != nil {
		logger.Error("server fatal error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
