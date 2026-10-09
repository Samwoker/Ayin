package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/seraph/triage/apps/api/internal/auth"
	"github.com/seraph/triage/apps/api/internal/config"
	"github.com/seraph/triage/apps/api/internal/ingest"
	"github.com/seraph/triage/apps/api/internal/server/handlers"
	"github.com/seraph/triage/apps/api/internal/server/middleware"
	"github.com/seraph/triage/apps/api/internal/server/sse"
	"github.com/seraph/triage/apps/api/internal/store"
)

// Server represents the HTTP server for triage.
type Server struct {
	cfg        config.Config
	logger     *slog.Logger
	router     *chi.Mux
	httpServer *http.Server
	health     *handlers.HealthHandler
}

// NewServer configures and returns a new Server instance.
func NewServer(
	cfg config.Config,
	logger *slog.Logger,
	health *handlers.HealthHandler,
	st *store.Store,
	jwtManager *auth.JWTManager,
	rateLimiter *middleware.RateLimiter,
	broker *sse.Broker,
	queue ingest.Queue,
) *Server {
	r := chi.NewRouter()

	// Defaults if nil (testing support)
	if jwtManager == nil {
		jwtManager = auth.NewJWTManager(cfg.JWTSecret)
	}
	if rateLimiter == nil {
		rateLimiter = middleware.NewRateLimiter(nil, 120, time.Minute)
	}
	if broker == nil {
		broker = sse.NewBroker()
	}
	if queue == nil {
		queue = ingest.NewMemoryQueue(100)
	}

	authenticator := middleware.NewAuthenticator(jwtManager, st)

	// Base middleware stack
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.AccessLogger(logger))
	r.Use(middleware.Recoverer(logger))
	r.Use(middleware.CORS)
	r.Use(authenticator.AuthenticateMiddleware)
	r.Use(rateLimiter.RateLimitMiddleware)

	// Liveness & Readiness probes
	r.Get("/healthz", health.Healthz)
	r.Get("/readyz", health.Readyz)

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// Handlers
	statusHandler := handlers.NewStatusHandler(cfg)

	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Get("/status", statusHandler.GetStatus)

		if st != nil {
			authHandler := handlers.NewAuthHandler(st, jwtManager)
			incidentHandler := handlers.NewIncidentHandler(st)
			runHandler := handlers.NewRunHandler(st, broker)
			runbookHandler := handlers.NewRunbookHandler(st)

			// Ingestion & Deduplication
			grouper := ingest.NewAlertGrouper(st, queue, 30*time.Minute)
			ingestHandler := ingest.NewIngestHandler(st, nil, grouper)

			// Webhook Ingestion endpoints (authenticated via HMAC)
			r.Post("/ingest/webhooks/{source}", ingestHandler.HandleWebhook)
			r.Post("/ingest/{org_id}/webhooks/{source}", ingestHandler.HandleWebhook)

			// Auth public endpoints
			r.Post("/auth/login", authHandler.Login)
			r.Post("/auth/refresh", authHandler.Refresh)

			// Protected routes group (enforces tenant authentication)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAuth)

				// Real-time SSE Stream (authenticated via header or query token)
				r.Get("/runs/{id}/stream", runHandler.StreamRun)

				// Identity & API keys
				r.Get("/auth/me", authHandler.Me)
				r.Get("/auth/api-keys", authHandler.ListAPIKeys)
				r.With(middleware.RequireRole(auth.RoleAdmin)).Post("/auth/api-keys", authHandler.CreateAPIKey)
				r.With(middleware.RequireRole(auth.RoleAdmin)).Delete("/auth/api-keys/{id}", authHandler.RevokeAPIKey)

				// Incidents CRUD
				r.Get("/incidents", incidentHandler.ListIncidents)
				r.Get("/incidents/{id}", incidentHandler.GetIncident)
				r.With(middleware.RequireRole(auth.RoleResponder)).Post("/incidents", incidentHandler.CreateIncident)
				r.With(middleware.RequireRole(auth.RoleResponder)).Patch("/incidents/{id}/status", incidentHandler.UpdateIncidentStatus)

				// Runs & Steps
				r.Get("/runs/{id}", runHandler.GetRun)
				r.Get("/runs/{id}/steps", runHandler.GetRunSteps)
				r.Get("/incidents/{id}/runs", runHandler.ListRunsForIncident)
				r.With(middleware.RequireRole(auth.RoleResponder)).Post("/incidents/{id}/runs", runHandler.CreateRun)

				// Runbooks CRUD
				r.Get("/runbooks", runbookHandler.ListRunbooks)
				r.Get("/runbooks/{id}", runbookHandler.GetRunbook)
				r.With(middleware.RequireRole(auth.RoleResponder)).Post("/runbooks", runbookHandler.CreateRunbook)
				r.With(middleware.RequireRole(auth.RoleResponder)).Put("/runbooks/{id}", runbookHandler.UpdateRunbook)
				r.With(middleware.RequireRole(auth.RoleAdmin)).Delete("/runbooks/{id}", runbookHandler.DeleteRunbook)
			})
		}
	})

	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		cfg:        cfg,
		logger:     logger,
		router:     r,
		httpServer: httpServer,
		health:     health,
	}
}

// Router returns the underlying Chi router for testing or custom mounting.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// Start begins listening and serving HTTP requests with graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		s.logger.Info("starting HTTP server",
			slog.String("addr", s.cfg.HTTPAddr),
			slog.String("env", s.cfg.Environment),
		)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return fmt.Errorf("http server failure: %w", err)
	case <-ctx.Done():
		s.logger.Info("shutdown signal received, commencing graceful shutdown...")
		s.health.SetReady(false)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("server shutdown forced", slog.String("error", err.Error()))
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		s.logger.Info("server exited cleanly")
		return nil
	}
}
