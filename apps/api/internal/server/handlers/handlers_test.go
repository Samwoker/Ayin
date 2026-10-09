package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/auth"
	"github.com/seraph/triage/apps/api/internal/server/middleware"
	"github.com/seraph/triage/apps/api/internal/server/sse"
	"github.com/seraph/triage/apps/api/internal/tools"
)

func withURLParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func mockTenantContext() middleware.TenantContext {
	var uID, oID pgtype.UUID
	_ = uID.Scan("10000000-0000-0000-0000-000000000001")
	_ = oID.Scan("20000000-0000-0000-0000-000000000001")

	return middleware.TenantContext{
		UserID:   uID,
		OrgID:    oID,
		Role:     auth.RoleResponder,
		Email:    "test@triage.com",
		IsAPIKey: false,
	}
}

func TestAuthHandlerUnauthenticated(t *testing.T) {
	h := NewAuthHandler(nil, nil)

	t.Run("Me without auth context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		rec := httptest.NewRecorder()

		h.Me(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("Me with valid auth context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		rec := httptest.NewRecorder()

		h.Me(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("Login with invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString("{bad-json"))
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("Refresh with empty token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(`{"refresh_token":""}`))
		rec := httptest.NewRecorder()

		h.Refresh(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("CreateAPIKey without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewBufferString(`{"name":"test"}`))
		rec := httptest.NewRecorder()

		h.CreateAPIKey(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("CreateAPIKey with empty name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewBufferString(`{"name":""}`))
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		rec := httptest.NewRecorder()

		h.CreateAPIKey(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("ListAPIKeys without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/api-keys", nil)
		rec := httptest.NewRecorder()

		h.ListAPIKeys(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("RevokeAPIKey without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/123", nil)
		rec := httptest.NewRecorder()

		h.RevokeAPIKey(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("RevokeAPIKey invalid ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/api-keys/not-a-uuid", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "id", "not-a-uuid")
		rec := httptest.NewRecorder()

		h.RevokeAPIKey(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

func TestIncidentsHandlerValidation(t *testing.T) {
	h := NewIncidentHandler(nil)

	t.Run("CreateIncident without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents", bytes.NewBufferString(`{"title":"test"}`))
		rec := httptest.NewRecorder()

		h.CreateIncident(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("CreateIncident missing service", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents", bytes.NewBufferString(`{"title":"test"}`))
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		rec := httptest.NewRecorder()

		h.CreateIncident(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("ListIncidents without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
		rec := httptest.NewRecorder()

		h.ListIncidents(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("GetIncident invalid UUID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/invalid-uuid", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "id", "invalid-uuid")
		rec := httptest.NewRecorder()

		h.GetIncident(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("UpdateIncidentStatus invalid UUID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/incidents/invalid-uuid/status", bytes.NewBufferString(`{"status":"resolved"}`))
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "id", "invalid-uuid")
		rec := httptest.NewRecorder()

		h.UpdateIncidentStatus(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

func TestRunsHandlerValidation(t *testing.T) {
	broker := sse.NewBroker()
	h := NewRunHandler(nil, broker)

	t.Run("CreateRun without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/123/runs", nil)
		rec := httptest.NewRecorder()

		h.CreateRun(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("CreateRun invalid incident ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/invalid-uuid/runs", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "id", "invalid-uuid")
		rec := httptest.NewRecorder()

		h.CreateRun(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("GetRun invalid run ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/invalid-uuid", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "id", "invalid-uuid")
		rec := httptest.NewRecorder()

		h.GetRun(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("GetRunSteps invalid run ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/invalid-uuid/steps", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "id", "invalid-uuid")
		rec := httptest.NewRecorder()

		h.GetRunSteps(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("ListRunsForIncident invalid incident ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/invalid-uuid/runs", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "id", "invalid-uuid")
		rec := httptest.NewRecorder()

		h.ListRunsForIncident(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("StreamRun unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/60000000-0000-0000-0000-000000000001/stream", nil)
		rec := httptest.NewRecorder()

		h.StreamRun(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("StreamRun invalid run ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/not-a-valid-uuid/stream", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "id", "not-a-valid-uuid")
		rec := httptest.NewRecorder()

		h.StreamRun(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

func TestRunbooksHandlerValidation(t *testing.T) {
	h := NewRunbookHandler(nil)

	t.Run("CreateRunbook without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks", bytes.NewBufferString(`{"title":"test"}`))
		rec := httptest.NewRecorder()

		h.CreateRunbook(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("CreateRunbook missing markdown", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/runbooks", bytes.NewBufferString(`{"title":"test","content_markdown":""}`))
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		rec := httptest.NewRecorder()

		h.CreateRunbook(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("GetRunbook invalid UUID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/runbooks/bad-id", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "id", "bad-id")
		rec := httptest.NewRecorder()

		h.GetRunbook(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("UpdateRunbook invalid UUID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/runbooks/bad-id", bytes.NewBufferString(`{"title":"t","content_markdown":"c"}`))
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "id", "bad-id")
		rec := httptest.NewRecorder()

		h.UpdateRunbook(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("DeleteRunbook invalid UUID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/runbooks/bad-id", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "id", "bad-id")
		rec := httptest.NewRecorder()

		h.DeleteRunbook(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

// MockFlusherRecorder implements http.ResponseWriter and http.Flusher for SSE testing
type MockFlusherRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (m *MockFlusherRecorder) Flush() {
	m.flushed = true
}

func TestStreamRunActiveConnection(t *testing.T) {
	broker := sse.NewBroker()
	h := NewRunHandler(nil, broker)

	runID := "60000000-0000-0000-0000-000000000001"

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/"+runID+"/stream", nil).WithContext(ctx)
	req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
	req = withURLParam(req, "id", runID)

	rec := &MockFlusherRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}

	done := make(chan struct{})
	go func() {
		h.StreamRun(rec, req)
		close(done)
	}()

	// Wait brief moment for subscription
	time.Sleep(30 * time.Millisecond)

	// Broadcast an event
	broker.Broadcast(runID, "step_completed", map[string]string{"step": "query_metrics"})

	// Allow message delivery
	time.Sleep(30 * time.Millisecond)

	// Cancel context to close SSE stream
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("StreamRun did not terminate cleanly on context cancel")
	}

	body := rec.Body.String()
	if !bytes.Contains(rec.Body.Bytes(), []byte("event: connected")) {
		t.Errorf("expected initial connected event, got: %s", body)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("event: step_completed")) {
		t.Errorf("expected broadcasted step_completed event, got: %s", body)
	}
}

func TestToolsHandlerValidationAndExecution(t *testing.T) {
	registry := tools.NewToolRegistry(tools.NewAllowAllPolicyProvider(), 5*time.Second)
	_ = registry.Register(tools.NewRecentDeploysTool(nil))
	_ = registry.Register(tools.NewServiceTopologyTool(nil))

	handler := NewToolsHandler(registry)

	t.Run("ListTools without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
		rec := httptest.NewRecorder()
		handler.ListTools(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("ListTools with auth returns registered tools", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		rec := httptest.NewRecorder()
		handler.ListTools(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte("recent_deploys")) {
			t.Errorf("expected recent_deploys in list, got: %s", rec.Body.String())
		}
	})

	t.Run("ExecuteTool without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tools/recent_deploys/execute", bytes.NewReader([]byte(`{"service":"checkout-worker"}`)))
		req = withURLParam(req, "name", "recent_deploys")
		rec := httptest.NewRecorder()
		handler.ExecuteTool(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("ExecuteTool with valid request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tools/recent_deploys/execute", bytes.NewReader([]byte(`{"service":"checkout-worker"}`)))
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		req = withURLParam(req, "name", "recent_deploys")
		rec := httptest.NewRecorder()
		handler.ExecuteTool(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte("checkout-worker")) {
			t.Errorf("expected checkout-worker in result: %s", rec.Body.String())
		}
	})

	t.Run("ExecuteTool with missing tool name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tools//execute", nil)
		req = req.WithContext(middleware.SetTenantContext(req.Context(), mockTenantContext()))
		rec := httptest.NewRecorder()
		handler.ExecuteTool(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})
}
