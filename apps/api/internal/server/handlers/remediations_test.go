package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/seraph/triage/apps/api/internal/remediation"
	"github.com/seraph/triage/apps/api/internal/server/middleware"
	"github.com/seraph/triage/apps/api/internal/store"
)

func TestRemediationHandlerValidationAndAuth(t *testing.T) {
	svc := remediation.NewRemediationService(nil, nil, nil, nil)
	h := NewRemediationHandler(svc)

	t.Run("Unauthenticated calls rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/1/remediations", nil)
		rec := httptest.NewRecorder()
		h.ListRemediationsForIncident(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}

		req = httptest.NewRequest(http.MethodPost, "/api/v1/incidents/1/remediations", bytes.NewBufferString("{}"))
		rec = httptest.NewRecorder()
		h.ProposeRemediation(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/v1/remediations/1", nil)
		rec = httptest.NewRecorder()
		h.GetRemediation(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}

		req = httptest.NewRequest(http.MethodPost, "/api/v1/remediations/1/approve", bytes.NewBufferString("{}"))
		rec = httptest.NewRecorder()
		h.ApproveRemediation(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}

		req = httptest.NewRequest(http.MethodPost, "/api/v1/remediations/1/reject", bytes.NewBufferString("{}"))
		rec = httptest.NewRecorder()
		h.RejectRemediation(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}

		req = httptest.NewRequest(http.MethodPost, "/api/v1/remediations/1/rollback", bytes.NewBufferString("{}"))
		rec = httptest.NewRecorder()
		h.RollbackRemediation(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("Invalid UUID format rejected with 400", func(t *testing.T) {
		tc := mockTenantContext()
		ctx := middleware.SetTenantContext(context.Background(), tc)

		req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/incidents/invalid-uuid/remediations", nil).WithContext(ctx), "id", "invalid-uuid")
		rec := httptest.NewRecorder()
		h.ListRemediationsForIncident(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}

		req = withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/remediations/invalid-uuid", nil).WithContext(ctx), "id", "invalid-uuid")
		rec = httptest.NewRecorder()
		h.GetRemediation(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("Propose invalid payload rejected with 400", func(t *testing.T) {
		tc := mockTenantContext()
		ctx := middleware.SetTenantContext(context.Background(), tc)

		req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/incidents/40000000-0000-0000-0000-000000000001/remediations", bytes.NewBufferString("not-json")).WithContext(ctx), "id", "40000000-0000-0000-0000-000000000001")
		rec := httptest.NewRecorder()
		h.ProposeRemediation(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}

		// Missing required fields
		req = withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/incidents/40000000-0000-0000-0000-000000000001/remediations", bytes.NewBufferString(`{"action_type":""}`)).WithContext(ctx), "id", "40000000-0000-0000-0000-000000000001")
		rec = httptest.NewRecorder()
		h.ProposeRemediation(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})
}

func TestRemediationHandlerLiveDatabase(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://triage:triage_secret_dev@localhost:5433/triage?sslmode=disable"
	}

	ctx := context.Background()
	st, err := store.NewStore(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database test: postgres not reachable (%v)", err)
		return
	}
	defer st.Close()

	tc := mockTenantContext()
	_ = tc.OrgID.Scan("00000000-0000-0000-0000-000000000001")
	_ = tc.UserID.Scan("10000000-0000-0000-0000-000000000001")
	authCtx := middleware.SetTenantContext(ctx, tc)

	incID := "40000000-0000-0000-0000-000000000001"

	svc := remediation.NewRemediationService(st, nil, nil, nil)
	h := NewRemediationHandler(svc)

	// 1. Propose remediation
	propBody := ProposeRemediationRequest{
		ActionType:     remediation.ActionScaleService,
		Target:         "payment-api",
		Params:         map[string]any{"replicas": 3.0, "previous_replicas": 1.0},
		Rationale:      "High CPU saturation, scale up payment-api",
		RiskLevel:      remediation.RiskLow,
		ExpectedEffect: "CPU drops below 60%",
		RollbackPlan:   "Scale back down to 1 replica",
		TTLSeconds:     600,
	}
	propBytes, _ := json.Marshal(propBody)

	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/incidents/"+incID+"/remediations", bytes.NewBuffer(propBytes)).WithContext(authCtx), "id", incID)
	rec := httptest.NewRecorder()
	h.ProposeRemediation(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode created remediation: %v", err)
	}
	if created.Status != "proposed" {
		t.Errorf("expected status proposed, got %s", created.Status)
	}

	// 2. List remediations
	req = withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/incidents/"+incID+"/remediations", nil).WithContext(authCtx), "id", incID)
	rec = httptest.NewRecorder()
	h.ListRemediationsForIncident(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	// 3. Get remediation
	req = withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/remediations/"+created.ID, nil).WithContext(authCtx), "id", created.ID)
	rec = httptest.NewRecorder()
	h.GetRemediation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	// 4. Approve remediation
	appBody := ApproveRemediationRequest{Reason: "Approved for scale up"}
	appBytes, _ := json.Marshal(appBody)
	req = withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/remediations/"+created.ID+"/approve", bytes.NewBuffer(appBytes)).WithContext(authCtx), "id", created.ID)
	rec = httptest.NewRecorder()
	h.ApproveRemediation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. Rollback remediation
	rbBody := RollbackRemediationRequest{Reason: "Incident mitigated, returning to normal scale"}
	rbBytes, _ := json.Marshal(rbBody)
	req = withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/remediations/"+created.ID+"/rollback", bytes.NewBuffer(rbBytes)).WithContext(authCtx), "id", created.ID)
	rec = httptest.NewRecorder()
	h.RollbackRemediation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
}
