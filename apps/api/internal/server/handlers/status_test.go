package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seraph/triage/apps/api/internal/config"
)

func TestStatusHandler(t *testing.T) {
	cfg := config.Config{
		Environment: "test",
	}
	h := NewStatusHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec := httptest.NewRecorder()

	h.GetStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp StatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if resp.Status != "online" {
		t.Errorf("expected status 'online', got %s", resp.Status)
	}
	if resp.Environment != "test" {
		t.Errorf("expected environment 'test', got %s", resp.Environment)
	}
	if resp.Service != "triage-api" {
		t.Errorf("expected service 'triage-api', got %s", resp.Service)
	}
}
