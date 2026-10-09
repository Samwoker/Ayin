package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/seraph/triage/apps/api/internal/config"
	"github.com/seraph/triage/apps/api/internal/server/handlers"
	"github.com/seraph/triage/apps/api/internal/telemetry"
)

func TestServerRoutes(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:        ":0",
		ShutdownTimeout: 2 * time.Second,
		Environment:     "test",
	}
	logger := telemetry.SetupLogger("error", "test")
	health := handlers.NewHealthHandler()
	srv := NewServer(cfg, logger, health, nil, nil, nil, nil)

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	t.Run("GET /healthz", func(t *testing.T) {
		res, err := http.Get(ts.URL + "/healthz")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.StatusCode)
		}
	})

	t.Run("GET /readyz", func(t *testing.T) {
		res, err := http.Get(ts.URL + "/readyz")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.StatusCode)
		}
	})

	t.Run("GET /metrics", func(t *testing.T) {
		res, err := http.Get(ts.URL + "/metrics")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		if len(body) == 0 {
			t.Fatalf("expected non-empty metrics output")
		}
	})
}

func TestServerGracefulShutdown(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:        "127.0.0.1:0",
		ShutdownTimeout: 1 * time.Second,
		Environment:     "test",
	}
	logger := telemetry.SetupLogger("error", "test")
	health := handlers.NewHealthHandler()
	srv := NewServer(cfg, logger, health, nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start(ctx)
	}()

	// Allow server to spin up
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown
	cancel()

	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("expected clean shutdown, got error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("server shutdown timed out")
	}
}
