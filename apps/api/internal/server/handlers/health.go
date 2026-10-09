package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthHandler manages /healthz and /readyz probes.
type HealthHandler struct {
	mu     sync.RWMutex
	ready  bool
	checks map[string]func(context.Context) error
}

// NewHealthHandler creates an initialized HealthHandler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		ready:  true,
		checks: make(map[string]func(context.Context) error),
	}
}

// RegisterCheck registers a readiness check function.
func (h *HealthHandler) RegisterCheck(name string, check func(context.Context) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

// SetReady toggles the overall readiness status.
func (h *HealthHandler) SetReady(ready bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ready = ready
}

// Healthz responds with basic liveness information.
func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "triage-api",
	})
}

// Readyz responds with dependency readiness checks.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	isReady := h.ready
	checks := make(map[string]func(context.Context) error, len(h.checks))
	for k, v := range h.checks {
		checks[k] = v
	}
	h.mu.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	results := make(map[string]string)
	allPassed := isReady

	for name, check := range checks {
		if err := check(ctx); err != nil {
			results[name] = err.Error()
			allPassed = false
		} else {
			results[name] = "ok"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	status := http.StatusOK
	statusStr := "ready"

	if !allPassed {
		status = http.StatusServiceUnavailable
		statusStr = "not_ready"
	}

	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    statusStr,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks":    results,
	})
}
