package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/config"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
)

var (
	ErrInvalidSignature = errors.New("invalid stripe webhook signature")
	ErrOrgNotFound      = errors.New("organization not found")
	ErrNoStripeCustomer = errors.New("organization has no active stripe customer")
)

type PlanTier string

const (
	PlanFree       PlanTier = "free"
	PlanPro        PlanTier = "pro"
	PlanEnterprise PlanTier = "enterprise"
)

type PlanInfo struct {
	Tier             PlanTier `json:"tier"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	PriceUSD         int      `json:"price_usd"`
	MonthlyRunsLimit int32    `json:"monthly_runs_limit"`
	RateLimitRPM     int      `json:"rate_limit_rpm"`
	Concurrency      int      `json:"concurrency"`
	Features         []string `json:"features"`
}

var Catalog = []PlanInfo{
	{
		Tier:             PlanFree,
		Name:             "Free",
		Description:      "Essential automated incident triage for small engineering teams.",
		PriceUSD:         0,
		MonthlyRunsLimit: 10,
		RateLimitRPM:     10,
		Concurrency:      1,
		Features: []string{
			"10 AI investigation runs / month",
			"Gemini 2.5 Flash analysis",
			"Automated root cause hypothesis",
			"Standard runbook search",
			"Community support",
		},
	},
	{
		Tier:             PlanPro,
		Name:             "Pro",
		Description:      "Full incident investigation engine for scaling engineering organizations.",
		PriceUSD:         49,
		MonthlyRunsLimit: 250,
		RateLimitRPM:     60,
		Concurrency:      5,
		Features: []string{
			"250 AI investigation runs / month",
			"Gemini 2.5 Pro & Flash multi-agent engine",
			"Multi-hypothesis verification",
			"Automated remediation rollouts",
			"Real-time SSE event streaming",
			"Priority Slack & Webhook integrations",
			"Priority support & SLAs",
		},
	},
	{
		Tier:             PlanEnterprise,
		Name:             "Enterprise",
		Description:      "Tailored reliability infrastructure for mission-critical deployments.",
		PriceUSD:         299,
		MonthlyRunsLimit: 2500,
		RateLimitRPM:     300,
		Concurrency:      20,
		Features: []string{
			"2,500+ AI investigation runs / month",
			"Custom LLM bring-your-own-key",
			"Dedicated VPC deployment",
			"99.99% uptime SLA",
			"24/7 incident engineer on-call",
			"Audit log export & SSO/SAML",
		},
	},
}

type SubscriptionInfo struct {
	OrgID                string     `json:"org_id"`
	PlanTier             string     `json:"plan_tier"`
	Status               string     `json:"status"`
	StripeCustomerID     string     `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string     `json:"stripe_subscription_id,omitempty"`
	CurrentPeriodEnd     *time.Time `json:"current_period_end,omitempty"`
	MonthlyRunsLimit     int32      `json:"monthly_runs_limit"`
	MonthlyRunsUsed      int32      `json:"monthly_runs_used"`
	MonthlyRunsRemaining int32      `json:"monthly_runs_remaining"`
	ResetAt              time.Time  `json:"reset_at"`
	PublishableKey       string     `json:"publishable_key,omitempty"`
	ProPriceID           string     `json:"pro_price_id,omitempty"`
}

type Service interface {
	GetPlans() []PlanInfo
	GetSubscription(ctx context.Context, orgID pgtype.UUID) (*SubscriptionInfo, error)
	CreateCheckoutSession(ctx context.Context, orgID pgtype.UUID, userEmail, successURL, cancelURL string) (string, error)
	CreatePortalSession(ctx context.Context, orgID pgtype.UUID, returnURL string) (string, error)
	HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error
}

type billingService struct {
	cfg        config.Config
	store      *store.Store
	httpClient *http.Client
	logger     *slog.Logger
}

func NewService(cfg config.Config, st *store.Store, logger *slog.Logger) Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &billingService{
		cfg:   cfg,
		store: st,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger.With(slog.String("component", "billing_service")),
	}
}

func (s *billingService) GetPlans() []PlanInfo {
	return Catalog
}

func (s *billingService) GetSubscription(ctx context.Context, orgID pgtype.UUID) (*SubscriptionInfo, error) {
	if s.store == nil {
		return &SubscriptionInfo{
			OrgID:                fmt.Sprintf("%x", orgID.Bytes),
			PlanTier:             string(PlanFree),
			Status:               "active",
			MonthlyRunsLimit:     10,
			MonthlyRunsUsed:      0,
			MonthlyRunsRemaining: 10,
			ResetAt:              time.Now().Add(30 * 24 * time.Hour),
			PublishableKey:       s.cfg.StripePublishableKey,
			ProPriceID:           s.cfg.StripeProPriceID,
		}, nil
	}

	org, err := s.store.Queries().GetOrgByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch organization: %w", err)
	}

	tier := org.PlanTier
	if tier == "" {
		tier = string(PlanFree)
	}

	limit := org.MonthlyRunsLimit
	if limit <= 0 {
		limit = 10
	}

	remaining := limit - org.MonthlyRunsUsed
	if remaining < 0 {
		remaining = 0
	}

	info := &SubscriptionInfo{
		OrgID:                fmt.Sprintf("%x-%x-%x-%x-%x", org.ID.Bytes[0:4], org.ID.Bytes[4:6], org.ID.Bytes[6:8], org.ID.Bytes[8:10], org.ID.Bytes[10:16]),
		PlanTier:             tier,
		Status:               org.SubscriptionStatus,
		MonthlyRunsLimit:     limit,
		MonthlyRunsUsed:      org.MonthlyRunsUsed,
		MonthlyRunsRemaining: remaining,
		ResetAt:              org.MonthlyRunsResetAt.Time,
		PublishableKey:       s.cfg.StripePublishableKey,
		ProPriceID:           s.cfg.StripeProPriceID,
	}

	if org.StripeCustomerID.Valid {
		info.StripeCustomerID = org.StripeCustomerID.String
	}
	if org.StripeSubscriptionID.Valid {
		info.StripeSubscriptionID = org.StripeSubscriptionID.String
	}
	if org.CurrentPeriodEnd.Valid {
		t := org.CurrentPeriodEnd.Time
		info.CurrentPeriodEnd = &t
	}

	return info, nil
}

func (s *billingService) CreateCheckoutSession(ctx context.Context, orgID pgtype.UUID, userEmail, successURL, cancelURL string) (string, error) {
	orgIDStr := fmt.Sprintf("%x-%x-%x-%x-%x", orgID.Bytes[0:4], orgID.Bytes[4:6], orgID.Bytes[6:8], orgID.Bytes[8:10], orgID.Bytes[10:16])

	if successURL == "" {
		successURL = fmt.Sprintf("%s/billing?session_id={CHECKOUT_SESSION_ID}&upgraded=true", s.cfg.AppURL)
	}
	if cancelURL == "" {
		cancelURL = fmt.Sprintf("%s/billing?canceled=true", s.cfg.AppURL)
	}

	// In test/mock mode without live Stripe key, return a mock redirect URL
	if s.cfg.StripeSecretKey == "" || strings.HasPrefix(s.cfg.StripeSecretKey, "mock_") {
		return fmt.Sprintf("%s/billing?mock_checkout=success&org_id=%s", s.cfg.AppURL, orgIDStr), nil
	}

	var existingCustomerID string
	if s.store != nil {
		if org, err := s.store.Queries().GetOrgByID(ctx, orgID); err == nil && org.StripeCustomerID.Valid {
			existingCustomerID = org.StripeCustomerID.String
		}
	}

	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("payment_method_types[0]", "card")
	form.Set("line_items[0][price]", s.cfg.StripeProPriceID)
	form.Set("line_items[0][quantity]", "1")
	form.Set("client_reference_id", orgIDStr)
	form.Set("metadata[org_id]", orgIDStr)
	form.Set("subscription_data[metadata][org_id]", orgIDStr)
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)

	if existingCustomerID != "" {
		form.Set("customer", existingCustomerID)
	} else if userEmail != "" {
		form.Set("customer_email", userEmail)
	}

	endpoint := fmt.Sprintf("%s/checkout/sessions", s.cfg.StripeBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create checkout session request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.cfg.StripeSecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute stripe checkout request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("stripe checkout api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var sessionResp struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(bodyBytes, &sessionResp); err != nil {
		return "", fmt.Errorf("failed to decode stripe checkout response: %w", err)
	}

	if sessionResp.URL == "" {
		return "", errors.New("stripe response did not include checkout URL")
	}

	return sessionResp.URL, nil
}

func (s *billingService) CreatePortalSession(ctx context.Context, orgID pgtype.UUID, returnURL string) (string, error) {
	orgIDStr := fmt.Sprintf("%x-%x-%x-%x-%x", orgID.Bytes[0:4], orgID.Bytes[4:6], orgID.Bytes[6:8], orgID.Bytes[8:10], orgID.Bytes[10:16])

	if returnURL == "" {
		returnURL = fmt.Sprintf("%s/billing", s.cfg.AppURL)
	}

	if s.cfg.StripeSecretKey == "" || strings.HasPrefix(s.cfg.StripeSecretKey, "mock_") {
		return fmt.Sprintf("%s/billing?mock_portal=true&org_id=%s", s.cfg.AppURL, orgIDStr), nil
	}

	if s.store == nil {
		return "", ErrOrgNotFound
	}

	org, err := s.store.Queries().GetOrgByID(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to load org: %w", err)
	}

	if !org.StripeCustomerID.Valid || org.StripeCustomerID.String == "" {
		return "", ErrNoStripeCustomer
	}

	form := url.Values{}
	form.Set("customer", org.StripeCustomerID.String)
	form.Set("return_url", returnURL)

	endpoint := fmt.Sprintf("%s/billing_portal/sessions", s.cfg.StripeBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create portal session request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.cfg.StripeSecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute stripe portal request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("stripe portal api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var portalResp struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(bodyBytes, &portalResp); err != nil {
		return "", fmt.Errorf("failed to decode stripe portal response: %w", err)
	}

	if portalResp.URL == "" {
		return "", errors.New("stripe response did not include portal URL")
	}

	return portalResp.URL, nil
}

func (s *billingService) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	// Verify webhook signature if secret configured
	if s.cfg.StripeWebhookSecret != "" {
		if err := VerifyStripeSignature(payload, sigHeader, s.cfg.StripeWebhookSecret, 300*time.Second); err != nil {
			s.logger.Warn("invalid stripe webhook signature", slog.String("error", err.Error()))
			return ErrInvalidSignature
		}
	}

	var event struct {
		ID   string          `json:"id"`
		Type string          `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to parse stripe event JSON: %w", err)
	}

	s.logger.Info("processing stripe webhook event",
		slog.String("event_id", event.ID),
		slog.String("event_type", event.Type),
	)

	if s.store == nil {
		return nil
	}

	switch event.Type {
	case "checkout.session.completed":
		return s.handleCheckoutSessionCompleted(ctx, event.Data.Object)
	case "customer.subscription.updated":
		return s.handleSubscriptionUpdated(ctx, event.Data.Object)
	case "customer.subscription.deleted":
		return s.handleSubscriptionDeleted(ctx, event.Data.Object)
	case "invoice.payment_succeeded":
		return s.handleInvoicePaymentSucceeded(ctx, event.Data.Object)
	default:
		s.logger.Debug("unhandled stripe webhook event type", slog.String("type", event.Type))
		return nil
	}
}

func (s *billingService) handleCheckoutSessionCompleted(ctx context.Context, data []byte) error {
	var session struct {
		ClientReferenceID string `json:"client_reference_id"`
		Customer          string `json:"customer"`
		Subscription      string `json:"subscription"`
		Metadata          struct {
			OrgID string `json:"org_id"`
		} `json:"metadata"`
	}

	if err := json.Unmarshal(data, &session); err != nil {
		return fmt.Errorf("failed to parse checkout session data: %w", err)
	}

	targetOrgIDStr := session.Metadata.OrgID
	if targetOrgIDStr == "" {
		targetOrgIDStr = session.ClientReferenceID
	}
	if targetOrgIDStr == "" {
		return errors.New("no org_id found in checkout session metadata or client_reference_id")
	}

	var orgID pgtype.UUID
	if err := orgID.Scan(targetOrgIDStr); err != nil {
		return fmt.Errorf("invalid org_id UUID in checkout session: %w", err)
	}

	_, err := s.store.Queries().UpdateOrgSubscription(ctx, db.UpdateOrgSubscriptionParams{
		ID:                   orgID,
		PlanTier:             string(PlanPro),
		StripeCustomerID:     pgtype.Text{String: session.Customer, Valid: session.Customer != ""},
		StripeSubscriptionID: pgtype.Text{String: session.Subscription, Valid: session.Subscription != ""},
		SubscriptionStatus:   "active",
		CurrentPeriodEnd:     pgtype.Timestamptz{Time: time.Now().Add(30 * 24 * time.Hour), Valid: true},
		MonthlyRunsLimit:     250,
	})
	if err != nil {
		return fmt.Errorf("failed to upgrade organization to Pro: %w", err)
	}

	s.logger.Info("organization upgraded to Pro plan successfully",
		slog.String("org_id", targetOrgIDStr),
		slog.String("stripe_customer_id", session.Customer),
	)

	return nil
}

func (s *billingService) handleSubscriptionUpdated(ctx context.Context, data []byte) error {
	var sub struct {
		ID               string `json:"id"`
		Customer         string `json:"customer"`
		Status           string `json:"status"`
		CurrentPeriodEnd int64  `json:"current_period_end"`
	}

	if err := json.Unmarshal(data, &sub); err != nil {
		return fmt.Errorf("failed to parse subscription data: %w", err)
	}

	org, err := s.store.Queries().GetOrgByStripeSubscriptionID(ctx, pgtype.Text{String: sub.ID, Valid: true})
	if err != nil {
		// Fallback to customer ID lookup
		org, err = s.store.Queries().GetOrgByStripeCustomerID(ctx, pgtype.Text{String: sub.Customer, Valid: true})
		if err != nil {
			s.logger.Warn("organization not found for stripe subscription update",
				slog.String("subscription_id", sub.ID),
				slog.String("customer_id", sub.Customer),
			)
			return nil
		}
	}

	planTier := string(PlanPro)
	runsLimit := int32(250)
	if sub.Status == "canceled" || sub.Status == "unpaid" {
		planTier = string(PlanFree)
		runsLimit = 10
	}

	periodEnd := time.Now().Add(30 * 24 * time.Hour)
	if sub.CurrentPeriodEnd > 0 {
		periodEnd = time.Unix(sub.CurrentPeriodEnd, 0)
	}

	_, err = s.store.Queries().UpdateOrgSubscription(ctx, db.UpdateOrgSubscriptionParams{
		ID:                   org.ID,
		PlanTier:             planTier,
		StripeCustomerID:     pgtype.Text{String: sub.Customer, Valid: true},
		StripeSubscriptionID: pgtype.Text{String: sub.ID, Valid: true},
		SubscriptionStatus:   sub.Status,
		CurrentPeriodEnd:     pgtype.Timestamptz{Time: periodEnd, Valid: true},
		MonthlyRunsLimit:     runsLimit,
	})
	if err != nil {
		return fmt.Errorf("failed to update org subscription: %w", err)
	}

	s.logger.Info("organization subscription updated",
		slog.String("subscription_id", sub.ID),
		slog.String("status", sub.Status),
		slog.String("tier", planTier),
	)

	return nil
}

func (s *billingService) handleSubscriptionDeleted(ctx context.Context, data []byte) error {
	var sub struct {
		ID       string `json:"id"`
		Customer string `json:"customer"`
	}

	if err := json.Unmarshal(data, &sub); err != nil {
		return fmt.Errorf("failed to parse subscription deletion data: %w", err)
	}

	org, err := s.store.Queries().GetOrgByStripeSubscriptionID(ctx, pgtype.Text{String: sub.ID, Valid: true})
	if err != nil {
		org, err = s.store.Queries().GetOrgByStripeCustomerID(ctx, pgtype.Text{String: sub.Customer, Valid: true})
		if err != nil {
			return nil
		}
	}

	_, err = s.store.Queries().UpdateOrgSubscription(ctx, db.UpdateOrgSubscriptionParams{
		ID:                   org.ID,
		PlanTier:             string(PlanFree),
		StripeCustomerID:     org.StripeCustomerID,
		StripeSubscriptionID: pgtype.Text{String: sub.ID, Valid: true},
		SubscriptionStatus:   "canceled",
		CurrentPeriodEnd:     org.CurrentPeriodEnd,
		MonthlyRunsLimit:     10,
	})
	if err != nil {
		return fmt.Errorf("failed to downgrade org on subscription deletion: %w", err)
	}

	s.logger.Info("organization downgraded to Free following subscription deletion",
		slog.String("subscription_id", sub.ID),
	)

	return nil
}

func (s *billingService) handleInvoicePaymentSucceeded(ctx context.Context, data []byte) error {
	var invoice struct {
		Customer      string `json:"customer"`
		Subscription  string `json:"subscription"`
		BillingReason string `json:"billing_reason"`
	}

	if err := json.Unmarshal(data, &invoice); err != nil {
		return fmt.Errorf("failed to parse invoice data: %w", err)
	}

	org, err := s.store.Queries().GetOrgByStripeCustomerID(ctx, pgtype.Text{String: invoice.Customer, Valid: true})
	if err != nil {
		return nil
	}

	// Reset monthly runs on cycle renewal
	if invoice.BillingReason == "subscription_cycle" || invoice.BillingReason == "subscription_create" {
		_, err = s.store.Queries().ResetOrgMonthlyRuns(ctx, org.ID)
		if err != nil {
			s.logger.Warn("failed to reset monthly runs on invoice payment", slog.String("error", err.Error()))
		} else {
			s.logger.Info("monthly runs quota reset for organization after renewal",
				slog.String("customer_id", invoice.Customer),
			)
		}
	}

	return nil
}

// VerifyStripeSignature validates the Stripe-Signature header against the raw body.
func VerifyStripeSignature(payload []byte, sigHeader, secret string, tolerance time.Duration) error {
	if sigHeader == "" {
		return errors.New("missing Stripe-Signature header")
	}

	pairs := strings.Split(sigHeader, ",")
	var timestampStr string
	var signatures []string

	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			switch parts[0] {
			case "t":
				timestampStr = parts[1]
			case "v1":
				signatures = append(signatures, parts[1])
			}
		}
	}

	if timestampStr == "" || len(signatures) == 0 {
		return errors.New("malformed Stripe-Signature header")
	}

	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid signature timestamp: %w", err)
	}

	if tolerance > 0 {
		now := time.Now().Unix()
		if math.Abs(float64(now-ts)) > tolerance.Seconds() {
			return errors.New("stripe webhook timestamp outside tolerance window")
		}
	}

	// Signature is HMAC-SHA256 of "${timestamp}.${payload}"
	signedPayload := fmt.Sprintf("%s.%s", timestampStr, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expectedSig)) {
			return nil
		}
	}

	return errors.New("stripe webhook signature mismatch")
}
