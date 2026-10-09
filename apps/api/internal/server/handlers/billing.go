package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/seraph/triage/apps/api/internal/billing"
	"github.com/seraph/triage/apps/api/internal/server/middleware"
)

type BillingHandler struct {
	service billing.Service
}

func NewBillingHandler(svc billing.Service) *BillingHandler {
	return &BillingHandler{
		service: svc,
	}
}

// GetPlans returns the pricing tiers catalog.
func (h *BillingHandler) GetPlans(w http.ResponseWriter, r *http.Request) {
	plans := h.service.GetPlans()
	respondJSON(w, http.StatusOK, map[string]any{
		"plans": plans,
	})
}

// GetSubscription returns the current organization's subscription status and usage limits.
func (h *BillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	sub, err := h.service.GetSubscription(r.Context(), tc.OrgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load subscription details")
		return
	}

	respondJSON(w, http.StatusOK, sub)
}

type CreateCheckoutRequest struct {
	SuccessURL string `json:"success_url,omitempty"`
	CancelURL  string `json:"cancel_url,omitempty"`
}

// CreateCheckout initiates a Stripe Checkout session to upgrade to Pro.
func (h *BillingHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreateCheckoutRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	checkoutURL, err := h.service.CreateCheckoutSession(r.Context(), tc.OrgID, tc.Email, req.SuccessURL, req.CancelURL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create checkout session: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"checkout_url": checkoutURL,
	})
}

type CreatePortalRequest struct {
	ReturnURL string `json:"return_url,omitempty"`
}

// CreatePortal creates a Stripe Billing Customer Portal session to manage subscriptions and invoices.
func (h *BillingHandler) CreatePortal(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreatePortalRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	portalURL, err := h.service.CreatePortalSession(r.Context(), tc.OrgID, req.ReturnURL)
	if err != nil {
		if errors.Is(err, billing.ErrNoStripeCustomer) {
			respondError(w, http.StatusBadRequest, "No active Stripe customer found. Upgrade to Pro first.")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to create customer portal session: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"portal_url": portalURL,
	})
}

// HandleWebhook processes inbound webhooks from Stripe with HMAC verification.
func (h *BillingHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// 64KB max webhook body
	payload, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if err := h.service.HandleWebhook(r.Context(), payload, sigHeader); err != nil {
		if errors.Is(err, billing.ErrInvalidSignature) {
			respondError(w, http.StatusBadRequest, "Invalid webhook signature")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to process webhook event: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"received": true,
	})
}
