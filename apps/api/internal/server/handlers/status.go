package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/seraph/triage/apps/api/internal/config"
)

// StatusHandler reports system status, uptime, and component versions.
type StatusHandler struct {
	cfg       config.Config
	startTime time.Time
}

// NewStatusHandler creates a new StatusHandler instance.
func NewStatusHandler(cfg config.Config) *StatusHandler {
	return &StatusHandler{
		cfg:       cfg,
		startTime: time.Now().UTC(),
	}
}

// StatusResponse is the payload returned by GET /api/v1/status.
type StatusResponse struct {
	Status        string            `json:"status"`
	Version       string            `json:"version"`
	Service       string            `json:"service"`
	Environment   string            `json:"environment"`
	UptimeSeconds int64             `json:"uptime_seconds"`
	Timestamp     string            `json:"timestamp"`
	Components    map[string]string `json:"components"`
}

// GetStatus returns the operational status and uptime.
func (h *StatusHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	uptime := int64(now.Sub(h.startTime).Seconds())

	resp := StatusResponse{
		Status:        "online",
		Version:       "0.1.0",
		Service:       "triage-api",
		Environment:   h.cfg.Environment,
		UptimeSeconds: uptime,
		Timestamp:     now.Format(time.RFC3339),
		Components: map[string]string{
			"database": "configured",
			"redis":    "configured",
			"agent":    "ready",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
