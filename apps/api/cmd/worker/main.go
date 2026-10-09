package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/seraph/triage/apps/api/internal/agent"
	"github.com/seraph/triage/apps/api/internal/config"
	"github.com/seraph/triage/apps/api/internal/ingest"
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
	logger.Info("starting triage worker",
		slog.String("env", cfg.Environment),
		slog.Int("concurrency", cfg.WorkerConcurrency),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize PostgreSQL store
	st, err := store.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer st.Close()

	// Initialize Tools Registry with all 6 read-only diagnostic tools
	toolRegistry := tools.NewToolRegistry(tools.NewAllowAllPolicyProvider(), 30*time.Second)
	_ = toolRegistry.Register(tools.NewLogSearchTool(nil))
	_ = toolRegistry.Register(tools.NewRecentDeploysTool(nil))
	_ = toolRegistry.Register(tools.NewServiceTopologyTool(nil))
	_ = toolRegistry.Register(tools.NewQueryMetricsTool(nil))
	_ = toolRegistry.Register(tools.NewSearchRunbooksTool(st))
	_ = toolRegistry.Register(tools.NewSimilarIncidentsTool(st))

	// Initialize SSE broker for real-time trace streaming
	broker := sse.NewBroker()

	// Initialize LLM Client (Gemini or OpenAI with fallback to deterministic mock)
	var llm agent.LLMClient
	switch {
	case cfg.LLMProvider == "gemini" && cfg.GeminiAPIKey != "":
		llm = agent.NewGeminiLLMClient(cfg.GeminiAPIKey, cfg.GeminiBaseURL, cfg.GeminiModel, nil)
		logger.Info("configured Google Gemini LLM client",
			slog.String("model", cfg.GeminiModel),
			slog.String("base_url", cfg.GeminiBaseURL),
		)
	case cfg.OpenAIAPIKey != "":
		llm = agent.NewOpenAILLMClient(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL, cfg.OpenAIModel, nil)
		logger.Info("configured OpenAI LLM client",
			slog.String("model", cfg.OpenAIModel),
			slog.String("base_url", cfg.OpenAIBaseURL),
		)
	default:
		llm = agent.NewMockLLMClient()
		logger.Info("configured deterministic mock LLM client (no LLM API keys configured)")
	}

	// Initialize Autonomous Agent Runner
	agentCfg := agent.DefaultAgentConfig()
	runner := agent.NewAgentRunner(st, llm, toolRegistry, broker, agentCfg, logger)

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

	sem := make(chan struct{}, cfg.WorkerConcurrency)
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			logger.Info("worker received shutdown signal, waiting for ongoing investigations to drain...")
			wg.Wait()
			logger.Info("worker shut down gracefully")
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

			sem <- struct{}{}
			wg.Add(1)

			go func(j ingest.InvestigationJob) {
				defer func() {
					<-sem
					wg.Done()
				}()

				investigateCtx, cancel := context.WithTimeout(context.Background(), agentCfg.RunTimeout)
				defer cancel()

				res, runErr := runner.RunInvestigation(investigateCtx, j)
				if runErr != nil {
					logger.Error("investigation run failed",
						slog.String("title", j.Title),
						slog.String("error", runErr.Error()),
					)
					return
				}

				logger.Info("investigation run completed",
					slog.String("run_id", fmt.Sprintf("%x-%x-%x-%x-%x", res.RunID.Bytes[0:4], res.RunID.Bytes[4:6], res.RunID.Bytes[6:8], res.RunID.Bytes[8:10], res.RunID.Bytes[10:16])),
					slog.String("status", res.Status),
					slog.Int("hypotheses", len(res.Hypotheses)),
					slog.Int("steps", res.StepsCount),
					slog.Int("tokens", res.TotalTokens),
					slog.Float64("cost_usd", res.TotalCostUSD),
					slog.Duration("duration", res.Duration),
				)
			}(*job)
		}
	}
}
