package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/auth"
	"github.com/seraph/triage/apps/api/internal/telemetry"
)

func TestRequestIDMiddleware(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := GetRequestID(r.Context())
		if reqID == "" {
			t.Errorf("expected request id in context, got empty")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resID := rec.Header().Get("X-Request-ID")
	if resID == "" {
		t.Errorf("expected X-Request-ID header, got empty")
	}
}

func TestRecovererMiddleware(t *testing.T) {
	logger := telemetry.SetupLogger("error", "test")
	handler := Recoverer(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("simulated critical crash")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 status on panic recovery, got %d", rec.Code)
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// OPTIONS preflight check
	optReq := httptest.NewRequest(http.MethodOptions, "/test", nil)
	optRec := httptest.NewRecorder()
	handler.ServeHTTP(optRec, optReq)

	if optRec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS preflight, got %d", optRec.Code)
	}
	if optRec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected CORS allow origin header")
	}

	// GET check
	getReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET, got %d", getRec.Code)
	}
}

func TestAuthenticatorAndRequireAuth(t *testing.T) {
	jwtSecret := "super-secure-test-jwt-secret-key-32b"
	jwtMgr := auth.NewJWTManager(jwtSecret)

	var uID, oID pgtype.UUID
	_ = uID.Scan("10000000-0000-0000-0000-000000000001")
	_ = oID.Scan("20000000-0000-0000-0000-000000000001")

	tokenPair, err := jwtMgr.GenerateTokenPair(uID, oID, "test@triage.com", auth.RoleResponder)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	authMiddleware := NewAuthenticator(jwtMgr, nil)

	handler := authMiddleware.AuthenticateMiddleware(RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := GetTenant(r.Context())
		if !ok {
			t.Errorf("expected tenant context in handler")
		}
		if tc.Role != auth.RoleResponder {
			t.Errorf("expected role responder, got %s", tc.Role)
		}
		w.WriteHeader(http.StatusOK)
	})))

	t.Run("Valid Bearer Token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("Valid Query Token for SSE", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected?token="+tokenPair.AccessToken, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("Invalid Token Rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-junk-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("Missing Token Rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})
}

func TestRequireRoleHierarchy(t *testing.T) {
	jwtSecret := "super-secure-test-jwt-secret-key-32b"
	jwtMgr := auth.NewJWTManager(jwtSecret)

	var uID, oID pgtype.UUID
	_ = uID.Scan("10000000-0000-0000-0000-000000000001")
	_ = oID.Scan("20000000-0000-0000-0000-000000000001")

	// Viewer user
	viewerTokens, _ := jwtMgr.GenerateTokenPair(uID, oID, "viewer@triage.com", auth.RoleViewer)
	// Admin user
	adminTokens, _ := jwtMgr.GenerateTokenPair(uID, oID, "admin@triage.com", auth.RoleAdmin)

	authMiddleware := NewAuthenticator(jwtMgr, nil)

	// Route requiring Responder role
	protectedHandler := authMiddleware.AuthenticateMiddleware(
		RequireRole(auth.RoleResponder)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)

	t.Run("Viewer role rejected with 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/action", nil)
		req.Header.Set("Authorization", "Bearer "+viewerTokens.AccessToken)
		rec := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for viewer, got %d", rec.Code)
		}
	})

	t.Run("Admin role allowed (satisfies hierarchy)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/action", nil)
		req.Header.Set("Authorization", "Bearer "+adminTokens.AccessToken)
		rec := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK for admin, got %d", rec.Code)
		}
	})
}

func TestRateLimiterTokenBucket(t *testing.T) {
	// 3 requests allowed per 500ms
	limiter := NewRateLimiter(nil, 3, 500*time.Millisecond)

	handler := limiter.RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var orgUUID pgtype.UUID
	_ = orgUUID.Scan("30000000-0000-0000-0000-000000000001")
	tenantCtx := TenantContext{
		OrgID: orgUUID,
		Role:  auth.RoleResponder,
	}

	// 1st request - ok
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	req1 = req1.WithContext(SetTenantContext(context.Background(), tenantCtx))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("req1 expected 200, got %d", rec1.Code)
	}

	// 2nd request - ok
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	req2 = req2.WithContext(SetTenantContext(context.Background(), tenantCtx))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("req2 expected 200, got %d", rec2.Code)
	}

	// 3rd request - ok
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	req3 = req3.WithContext(SetTenantContext(context.Background(), tenantCtx))
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Errorf("req3 expected 200, got %d", rec3.Code)
	}

	// 4th request - Rate limit exceeded (429)
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	req4 = req4.WithContext(SetTenantContext(context.Background(), tenantCtx))
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusTooManyRequests {
		t.Errorf("req4 expected 429 Too Many Requests, got %d", rec4.Code)
	}

	// Different org should not be throttled!
	var otherOrgUUID pgtype.UUID
	_ = otherOrgUUID.Scan("40000000-0000-0000-0000-000000000002")
	otherTenantCtx := TenantContext{
		OrgID: otherOrgUUID,
		Role:  auth.RoleResponder,
	}
	reqOther := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	reqOther = reqOther.WithContext(SetTenantContext(context.Background(), otherTenantCtx))
	recOther := httptest.NewRecorder()
	handler.ServeHTTP(recOther, reqOther)
	if recOther.Code != http.StatusOK {
		t.Errorf("other org expected 200, got %d", recOther.Code)
	}
}

func TestTieredRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(nil, 120, time.Minute)
	handler := limiter.RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var orgID pgtype.UUID
	_ = orgID.Scan("50000000-0000-0000-0000-000000000001")

	// Free tier request
	freeCtx := TenantContext{
		OrgID:    orgID,
		Role:     auth.RoleResponder,
		PlanTier: "free",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	req = req.WithContext(SetTenantContext(context.Background(), freeCtx))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
	if rec.Header().Get("X-RateLimit-Tier") != "free" {
		t.Errorf("expected tier 'free', got %s", rec.Header().Get("X-RateLimit-Tier"))
	}
	if rec.Header().Get("X-RateLimit-Limit") != "10" {
		t.Errorf("expected limit '10', got %s", rec.Header().Get("X-RateLimit-Limit"))
	}

	// Pro tier request
	var proOrgID pgtype.UUID
	_ = proOrgID.Scan("50000000-0000-0000-0000-000000000002")
	proCtx := TenantContext{
		OrgID:    proOrgID,
		Role:     auth.RoleResponder,
		PlanTier: "pro",
	}
	reqPro := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	reqPro = reqPro.WithContext(SetTenantContext(context.Background(), proCtx))
	recPro := httptest.NewRecorder()
	handler.ServeHTTP(recPro, reqPro)

	if recPro.Header().Get("X-RateLimit-Tier") != "pro" {
		t.Errorf("expected tier 'pro', got %s", recPro.Header().Get("X-RateLimit-Tier"))
	}
	if recPro.Header().Get("X-RateLimit-Limit") != "60" {
		t.Errorf("expected limit '60', got %s", recPro.Header().Get("X-RateLimit-Limit"))
	}
}
