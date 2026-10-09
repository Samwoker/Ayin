package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/seraph/triage/apps/api/internal/config"
	"github.com/seraph/triage/apps/api/internal/ingest"
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

	var queue ingest.Queue
	if redisOpts, err := redis.ParseURL(cfg.RedisURL); err == nil {
		rdb := redis.NewClient(redisOpts)
		defer rdb.Close()
		queue = ingest.NewRedisQueue(rdb, "stream:investigations", "triage-workers", "worker-1")
		logger.Info("connected to redis stream for investigation queue")
	} else {
		logger.Warn("redis connection failed, falling back to in-memory queue", slog.String("error", err.Error()))
		queue = ingest.NewMemoryQueue(100)
	}

	logger.Info("worker listening for investigation jobs...")

	for {
		select {
		case <-ctx.Done():
			logger.Info("worker shutting down gracefully...")
			return
		default:
			job, err := queue.Dequeue(ctx, 2*time.Second)
			if err != nil {
				if errors.Is(err, ingest.ErrQueueEmpty) || errors.Is(err, context.Canceled) {
					continue
				}
				logger.Error("failed to dequeue investigation job", slog.String("error", err.Error()))
				time.Sleep(500 * time.Millisecond)
				continue
			}

			logger.Info("processing investigation job",
				slog.String("title", job.Title),
				slog.String("service", job.Service),
				slog.String("severity", job.Severity),
				slog.String("source", job.Source),
			)
		}
	}
}
