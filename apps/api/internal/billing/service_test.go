package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/config"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
)

func generateStripeSignature(payload []byte, secret string, timestamp int64) string {
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	sigHex := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, sigHex)
}

func TestCatalogAndPlans(t *testing.T) {
	svc := NewService(config.Config{}, nil, slog.Default())
	plans := svc.GetPlans()

	if len(plans) != 3 {
		t.Fatalf("expected 3 plans, got %d", len(plans))
	}

	tiers := make(map[PlanTier]PlanInfo)
	for _, p := range plans {
		tiers[p.Tier] = p
	}

	free, ok := tiers[PlanFree]
	if !ok || free.PriceUSD != 0 || free.MonthlyRunsLimit != 10 {
		t.Errorf("unexpected free plan: %+v", free)
	}

	pro, ok := tiers[PlanPro]
	if !ok || pro.PriceUSD != 49 || pro.MonthlyRunsLimit != 250 {
		t.Errorf("unexpected pro plan: %+v", pro)
	}

	enterprise, ok := tiers[PlanEnterprise]
	if !ok || enterprise.PriceUSD != 299 || enterprise.MonthlyRunsLimit != 2500 {
		t.Errorf("unexpected enterprise plan: %+v", enterprise)
	}
}

func TestVerifyStripeSignature(t *testing.T) {
	secret := "whsec_test_secret_12345"
	payload := []byte(`{"id":"evt_123","type":"checkout.session.completed"}`)
	now := time.Now().Unix()

	validSig := generateStripeSignature(payload, secret, now)

	// 1. Valid signature
	err := VerifyStripeSignature(payload, validSig, secret, 300*time.Second)
	if err != nil {
		t.Fatalf("expected valid signature to pass, got: %v", err)
	}

	// 2. Tampered payload
	tampered := []byte(`{"id":"evt_999","type":"checkout.session.completed"}`)
	err = VerifyStripeSignature(tampered, validSig, secret, 300*time.Second)
	if err == nil {
		t.Fatal("expected tampered payload to fail verification")
	}

	// 3. Wrong secret
	err = VerifyStripeSignature(payload, validSig, "whsec_wrong_secret", 300*time.Second)
	if err == nil {
		t.Fatal("expected wrong secret to fail verification")
	}

	// 4. Expired timestamp (> tolerance)
	expiredSig := generateStripeSignature(payload, secret, now-400)
	err = VerifyStripeSignature(payload, expiredSig, secret, 300*time.Second)
	if err == nil {
		t.Fatal("expected expired timestamp to fail verification")
	}

	// 5. Missing header
	err = VerifyStripeSignature(payload, "", secret, 300*time.Second)
	if err == nil {
		t.Fatal("expected empty header to fail")
	}

	// 6. Malformed header
	err = VerifyStripeSignature(payload, "invalid_header_format", secret, 300*time.Second)
	if err == nil {
		t.Fatal("expected malformed header to fail")
	}
}

func TestCreateCheckoutSession_MockMode(t *testing.T) {
	cfg := config.Config{
		AppURL:          "http://localhost:5173",
		StripeSecretKey: "", // empty triggers mock mode
	}

	svc := NewService(cfg, nil, slog.Default())
	var orgID pgtype.UUID
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	url, err := svc.CreateCheckoutSession(context.Background(), orgID, "alice@acme.com", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPrefix := "http://localhost:5173/billing?mock_checkout=success"
	if len(url) < len(expectedPrefix) || url[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected checkout url to start with %s, got %s", expectedPrefix, url)
	}
}

func TestCreateCheckoutSession_LiveAPI(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkout/sessions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk_test_secret_key" {
			t.Errorf("unexpected authorization header: %s", r.Header.Get("Authorization"))
		}

		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		if len(bodyStr) == 0 {
			t.Error("expected non-empty form body")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"cs_test_12345","url":"https://checkout.stripe.com/pay/cs_test_12345"}`))
	}))
	defer mockServer.Close()

	cfg := config.Config{
		AppURL:           "http://localhost:5173",
		StripeBaseURL:    mockServer.URL,
		StripeSecretKey:  "sk_test_secret_key",
		StripeProPriceID: "price_pro_monthly",
	}

	svc := NewService(cfg, nil, slog.Default())
	var orgID pgtype.UUID
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	url, err := svc.CreateCheckoutSession(context.Background(), orgID, "alice@acme.com", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url != "https://checkout.stripe.com/pay/cs_test_12345" {
		t.Errorf("unexpected checkout url: %s", url)
	}
}

func TestCreateCheckoutSession_APIError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid price ID"}}`))
	}))
	defer mockServer.Close()

	cfg := config.Config{
		AppURL:           "http://localhost:5173",
		StripeBaseURL:    mockServer.URL,
		StripeSecretKey:  "sk_test_secret_key",
		StripeProPriceID: "bad_price",
	}

	svc := NewService(cfg, nil, slog.Default())
	var orgID pgtype.UUID
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	_, err := svc.CreateCheckoutSession(context.Background(), orgID, "alice@acme.com", "", "")
	if err == nil {
		t.Fatal("expected error from 400 bad request, got nil")
	}
}

func TestCreatePortalSession_MockMode(t *testing.T) {
	cfg := config.Config{
		AppURL:          "http://localhost:5173",
		StripeSecretKey: "mock_key_dev",
	}

	svc := NewService(cfg, nil, slog.Default())
	var orgID pgtype.UUID
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	url, err := svc.CreatePortalSession(context.Background(), orgID, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPrefix := "http://localhost:5173/billing?mock_portal=true"
	if len(url) < len(expectedPrefix) || url[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected portal url to start with %s, got %s", expectedPrefix, url)
	}
}

func TestHandleWebhook_SignatureEnforcement(t *testing.T) {
	secret := "whsec_test_secret"
	cfg := config.Config{
		StripeWebhookSecret: secret,
	}
	svc := NewService(cfg, nil, slog.Default())

	payload := []byte(`{"id":"evt_1","type":"customer.subscription.updated","data":{"object":{}}}`)
	now := time.Now().Unix()

	// 1. Invalid signature
	err := svc.HandleWebhook(context.Background(), payload, "bad_sig")
	if err != ErrInvalidSignature {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}

	// 2. Valid signature
	validSig := generateStripeSignature(payload, secret, now)
	err = svc.HandleWebhook(context.Background(), payload, validSig)
	if err != nil {
		t.Fatalf("expected valid signature to be handled, got %v", err)
	}

	// 3. Invalid JSON payload
	err = svc.HandleWebhook(context.Background(), []byte(`{invalid`), "")
	if err == nil {
		t.Fatal("expected error on invalid JSON payload")
	}
}

func TestGetSubscription_NilStoreFallback(t *testing.T) {
	cfg := config.Config{
		StripePublishableKey: "pk_test_123",
		StripeProPriceID:     "price_123",
	}
	svc := NewService(cfg, nil, slog.Default())

	var orgID pgtype.UUID
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	sub, err := svc.GetSubscription(context.Background(), orgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.PlanTier != "free" || sub.MonthlyRunsLimit != 10 || sub.MonthlyRunsRemaining != 10 {
		t.Errorf("unexpected subscription fallback: %+v", sub)
	}
	if sub.PublishableKey != "pk_test_123" || sub.ProPriceID != "price_123" {
		t.Errorf("unexpected keys in subscription: %+v", sub)
	}
}

func TestLiveBillingStoreOperations(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://triage:triage_secret_dev@localhost:5433/triage?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, err := store.NewStore(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database test: postgres not reachable (%v)", err)
		return
	}
	defer s.Close()

	if err := s.Migrate(ctx); err != nil {
		t.Skipf("skipping: migration failed: %v", err)
		return
	}

	var globexOrgID pgtype.UUID
	_ = globexOrgID.Scan("00000000-0000-0000-0000-000000000002")

	cfg := config.Config{
		AppURL:               "http://localhost:5173",
		StripePublishableKey: "pk_test_live",
		StripeProPriceID:     "price_live_pro",
	}
	svc := NewService(cfg, s, slog.Default())

	// 1. Get initial subscription
	sub, err := svc.GetSubscription(ctx, globexOrgID)
	if err != nil {
		t.Fatalf("failed to get globex subscription: %v", err)
	}
	if sub.OrgID == "" {
		t.Errorf("expected non-empty org ID in sub")
	}

	// 2. Process checkout.session.completed webhook for globex
	checkoutEvt := []byte(fmt.Sprintf(`{
		"id": "evt_test_checkout",
		"type": "checkout.session.completed",
		"data": {
			"object": {
				"client_reference_id": "00000000-0000-0000-0000-000000000002",
				"customer": "cus_globex_999",
				"subscription": "sub_globex_888",
				"metadata": {
					"org_id": "00000000-0000-0000-0000-000000000002"
				}
			}
		}
	}`))
	if err := svc.HandleWebhook(ctx, checkoutEvt, ""); err != nil {
		t.Fatalf("handle checkout webhook failed: %v", err)
	}

	// Verify globex is upgraded to Pro
	updatedSub, err := svc.GetSubscription(ctx, globexOrgID)
	if err != nil {
		t.Fatalf("failed to reload subscription: %v", err)
	}
	if updatedSub.PlanTier != "pro" || updatedSub.MonthlyRunsLimit != 250 {
		t.Errorf("expected pro tier with 250 limit, got: %+v", updatedSub)
	}
	if updatedSub.StripeCustomerID != "cus_globex_999" || updatedSub.StripeSubscriptionID != "sub_globex_888" {
		t.Errorf("unexpected stripe IDs: %+v", updatedSub)
	}

	// 3. Process customer.subscription.updated (e.g. past_due)
	subUpdateEvt := []byte(`{
		"id": "evt_test_sub_upd",
		"type": "customer.subscription.updated",
		"data": {
			"object": {
				"id": "sub_globex_888",
				"customer": "cus_globex_999",
				"status": "past_due",
				"current_period_end": 1799000000
			}
		}
	}`)
	if err := svc.HandleWebhook(ctx, subUpdateEvt, ""); err != nil {
		t.Fatalf("handle sub update webhook failed: %v", err)
	}

	// 4. Process invoice.payment_succeeded (monthly cycle renewal resets usage)
	invoiceEvt := []byte(`{
		"id": "evt_test_invoice",
		"type": "invoice.payment_succeeded",
		"data": {
			"object": {
				"customer": "cus_globex_999",
				"subscription": "sub_globex_888",
				"billing_reason": "subscription_cycle"
			}
		}
	}`)
	if err := svc.HandleWebhook(ctx, invoiceEvt, ""); err != nil {
		t.Fatalf("handle invoice webhook failed: %v", err)
	}

	// 5. Process customer.subscription.deleted (downgrade to Free)
	subDelEvt := []byte(`{
		"id": "evt_test_sub_del",
		"type": "customer.subscription.deleted",
		"data": {
			"object": {
				"id": "sub_globex_888",
				"customer": "cus_globex_999"
			}
		}
	}`)
	if err := svc.HandleWebhook(ctx, subDelEvt, ""); err != nil {
		t.Fatalf("handle sub delete webhook failed: %v", err)
	}

	downgradedSub, err := svc.GetSubscription(ctx, globexOrgID)
	if err != nil {
		t.Fatalf("failed to reload subscription after deletion: %v", err)
	}
	if downgradedSub.PlanTier != "free" || downgradedSub.MonthlyRunsLimit != 10 {
		t.Errorf("expected downgraded free tier, got: %+v", downgradedSub)
	}

	// 6. Test Live Portal Session
	mockPortalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"bps_123","url":"https://billing.stripe.com/session/bps_123"}`))
	}))
	defer mockPortalServer.Close()

	// Re-assign customer ID so portal succeeds
	_, _ = s.Queries().UpdateOrgSubscription(ctx, db.UpdateOrgSubscriptionParams{
		ID:                   globexOrgID,
		PlanTier:             "free",
		StripeCustomerID:     pgtype.Text{String: "cus_globex_999", Valid: true},
		StripeSubscriptionID: pgtype.Text{Valid: false},
		SubscriptionStatus:   "active",
		MonthlyRunsLimit:     10,
	})

	livePortalCfg := config.Config{
		AppURL:          "http://localhost:5173",
		StripeBaseURL:   mockPortalServer.URL,
		StripeSecretKey: "sk_test_portal_secret",
	}
	portalSvc := NewService(livePortalCfg, s, slog.Default())
	portalURL, err := portalSvc.CreatePortalSession(ctx, globexOrgID, "")
	if err != nil {
		t.Fatalf("failed to create portal session: %v", err)
	}
	if portalURL != "https://billing.stripe.com/session/bps_123" {
		t.Errorf("unexpected portal url: %s", portalURL)
	}
}
