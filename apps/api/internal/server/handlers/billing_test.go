package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/billing"
	"github.com/seraph/triage/apps/api/internal/server/middleware"
)

type mockBillingService struct {
	plans         []billing.PlanInfo
	sub           *billing.SubscriptionInfo
	checkoutURL   string
	portalURL     string
	handleErr     error
	portalErr     error
	webhookCalled bool
}

func (m *mockBillingService) GetPlans() []billing.PlanInfo {
	return m.plans
}

func (m *mockBillingService) GetSubscription(ctx context.Context, orgID pgtype.UUID) (*billing.SubscriptionInfo, error) {
	return m.sub, nil
}

func (m *mockBillingService) CreateCheckoutSession(ctx context.Context, orgID pgtype.UUID, userEmail, successURL, cancelURL string) (string, error) {
	return m.checkoutURL, nil
}

func (m *mockBillingService) CreatePortalSession(ctx context.Context, orgID pgtype.UUID, returnURL string) (string, error) {
	if m.portalErr != nil {
		return "", m.portalErr
	}
	return m.portalURL, nil
}

func (m *mockBillingService) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	m.webhookCalled = true
	return m.handleErr
}

func TestBillingHandler_GetPlans(t *testing.T) {
	mockSvc := &mockBillingService{
		plans: billing.Catalog,
	}
	handler := NewBillingHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing/plans", nil)
	rec := httptest.NewRecorder()

	handler.GetPlans(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var resp struct {
		Plans []billing.PlanInfo `json:"plans"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Plans) != 3 {
		t.Errorf("expected 3 plans, got %d", len(resp.Plans))
	}
}

func TestBillingHandler_GetSubscription(t *testing.T) {
	mockSvc := &mockBillingService{
		sub: &billing.SubscriptionInfo{
			OrgID:                "00000000-0000-0000-0000-000000000001",
			PlanTier:             "pro",
			Status:               "active",
			MonthlyRunsLimit:     250,
			MonthlyRunsUsed:      15,
			MonthlyRunsRemaining: 235,
			ResetAt:              time.Now().Add(10 * 24 * time.Hour),
		},
	}
	handler := NewBillingHandler(mockSvc)

	var orgID pgtype.UUID
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	// 1. Unauthenticated -> 401
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/billing/subscription", nil)
	unauthRec := httptest.NewRecorder()
	handler.GetSubscription(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", unauthRec.Code)
	}

	// 2. Authenticated -> 200
	authReq := httptest.NewRequest(http.MethodGet, "/api/v1/billing/subscription", nil)
	authReq = authReq.WithContext(middleware.SetTenantContext(context.Background(), middleware.TenantContext{
		OrgID:    orgID,
		Role:     "admin",
		PlanTier: "pro",
	}))
	authRec := httptest.NewRecorder()
	handler.GetSubscription(authRec, authReq)

	if authRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", authRec.Code)
	}

	var sub billing.SubscriptionInfo
	if err := json.Unmarshal(authRec.Body.Bytes(), &sub); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if sub.PlanTier != "pro" || sub.MonthlyRunsLimit != 250 {
		t.Errorf("unexpected subscription response: %+v", sub)
	}
}

func TestBillingHandler_CreateCheckout(t *testing.T) {
	mockSvc := &mockBillingService{
		checkoutURL: "https://checkout.stripe.com/pay/cs_mock_123",
	}
	handler := NewBillingHandler(mockSvc)

	var orgID pgtype.UUID
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", bytes.NewReader([]byte(`{}`)))
	req = req.WithContext(middleware.SetTenantContext(context.Background(), middleware.TenantContext{
		OrgID: orgID,
		Email: "alice@acme.com",
	}))
	rec := httptest.NewRecorder()

	handler.CreateCheckout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var resp struct {
		CheckoutURL string `json:"checkout_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.CheckoutURL != "https://checkout.stripe.com/pay/cs_mock_123" {
		t.Errorf("unexpected checkout url: %s", resp.CheckoutURL)
	}
}

func TestBillingHandler_CreatePortal(t *testing.T) {
	mockSvc := &mockBillingService{
		portalURL: "https://billing.stripe.com/p/session/bps_mock_123",
	}
	handler := NewBillingHandler(mockSvc)

	var orgID pgtype.UUID
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/portal", bytes.NewReader([]byte(`{}`)))
	req = req.WithContext(middleware.SetTenantContext(context.Background(), middleware.TenantContext{
		OrgID: orgID,
	}))
	rec := httptest.NewRecorder()

	handler.CreatePortal(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var resp struct {
		PortalURL string `json:"portal_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.PortalURL != "https://billing.stripe.com/p/session/bps_mock_123" {
		t.Errorf("unexpected portal url: %s", resp.PortalURL)
	}
}

func TestBillingHandler_HandleWebhook(t *testing.T) {
	mockSvc := &mockBillingService{}
	handler := NewBillingHandler(mockSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/webhook", bytes.NewReader([]byte(`{"id":"evt_1"}`)))
	req.Header.Set("Stripe-Signature", "t=1,v1=abc")
	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if !mockSvc.webhookCalled {
		t.Error("expected webhook to be called on service")
	}

	// Signature failure
	mockSvc.handleErr = billing.ErrInvalidSignature
	failReq := httptest.NewRequest(http.MethodPost, "/api/v1/billing/webhook", bytes.NewReader([]byte(`{"id":"evt_2"}`)))
	failRec := httptest.NewRecorder()

	handler.HandleWebhook(failRec, failReq)
	if failRec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on invalid signature, got %d", failRec.Code)
	}
}
