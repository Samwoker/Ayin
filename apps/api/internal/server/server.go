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

	"github.com/seraph/triage/apps/api/internal/config"
	"github.com/seraph/triage/apps/api/internal/server/handlers"
	"github.com/seraph/triage/apps/api/internal/server/middleware"
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
func NewServer(cfg config.Config, logger *slog.Logger, health *handlers.HealthHandler) *Server {
	r := chi.NewRouter()

	// Base middleware stack
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.AccessLogger(logger))
	r.Use(middleware.Recoverer(logger))
	r.Use(middleware.CORS)

	// Liveness & Readiness probes
	r.Get("/healthz", health.Healthz)
	r.Get("/readyz", health.Readyz)

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// API routes
	statusHandler := handlers.NewStatusHandler(cfg)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/status", statusHandler.GetStatus)
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
