package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/remediation"
	"github.com/seraph/triage/apps/api/internal/server/middleware"
)

// RemediationHandler manages human-in-the-loop incident remediation lifecycles.
type RemediationHandler struct {
	service *remediation.RemediationService
}

// NewRemediationHandler constructs a new RemediationHandler.
func NewRemediationHandler(svc *remediation.RemediationService) *RemediationHandler {
	return &RemediationHandler{service: svc}
}

type ProposeRemediationRequest struct {
	ActionType     string         `json:"action_type"`
	Target         string         `json:"target"`
	Params         map[string]any `json:"params"`
	Rationale      string         `json:"rationale"`
	RiskLevel      string         `json:"risk_level"`
	ExpectedEffect string         `json:"expected_effect"`
	RollbackPlan   string         `json:"rollback_plan"`
	HypothesisID   string         `json:"hypothesis_id,omitempty"`
	TTLSeconds     int            `json:"ttl_seconds,omitempty"`
}

type ApproveRemediationRequest struct {
	Reason string `json:"reason,omitempty"`
}

type RejectRemediationRequest struct {
	Reason string `json:"reason,omitempty"`
}

type RollbackRemediationRequest struct {
	Reason string `json:"reason,omitempty"`
}

// ListRemediationsForIncident lists all proposed, approved, or executed remediations for an incident.
func (h *RemediationHandler) ListRemediationsForIncident(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	incIDStr := chi.URLParam(r, "id")
	var incID pgtype.UUID
	if err := incID.Scan(incIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid incident ID format")
		return
	}

	rems, err := h.service.ListForIncident(r.Context(), tc.OrgID, incID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"remediations": rems,
		"count":        len(rems),
	})
}

// ProposeRemediation submits a new remediation for human review.
func (h *RemediationHandler) ProposeRemediation(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	incIDStr := chi.URLParam(r, "id")
	var incID pgtype.UUID
	if err := incID.Scan(incIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid incident ID format")
		return
	}

	var req ProposeRemediationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request JSON payload")
		return
	}

	if req.ActionType == "" || req.Target == "" || req.Rationale == "" {
		respondError(w, http.StatusBadRequest, "Action type, target, and rationale are required")
		return
	}
	if req.RiskLevel == "" {
		req.RiskLevel = remediation.RiskMedium
	}

	var hypID pgtype.UUID
	if req.HypothesisID != "" {
		_ = hypID.Scan(req.HypothesisID)
	}

	ttl := 15 * time.Minute
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}

	input := remediation.CreateRemediationInput{
		IncidentID:     incID,
		HypothesisID:   hypID,
		ActionType:     req.ActionType,
		Target:         req.Target,
		Params:         req.Params,
		Rationale:      req.Rationale,
		RiskLevel:      req.RiskLevel,
		ExpectedEffect: req.ExpectedEffect,
		RollbackPlan:   req.RollbackPlan,
		TTL:            ttl,
	}

	rem, err := h.service.Propose(r.Context(), tc.OrgID, input)
	if err != nil {
		if errors.Is(err, remediation.ErrInvalidAction) || errors.Is(err, remediation.ErrInvalidRisk) || errors.Is(err, remediation.ErrMissingTarget) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, rem)
}

// GetRemediation retrieves a remediation and its full transition audit event trail.
func (h *RemediationHandler) GetRemediation(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	remIDStr := chi.URLParam(r, "id")
	var remID pgtype.UUID
	if err := remID.Scan(remIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid remediation ID format")
		return
	}

	rem, events, err := h.service.Get(r.Context(), tc.OrgID, remID)
	if err != nil {
		if errors.Is(err, remediation.ErrRemediationNotFound) {
			respondError(w, http.StatusNotFound, "Remediation not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"remediation": rem,
		"events":      events,
	})
}

// ApproveRemediation approves and executes a proposed remediation.
func (h *RemediationHandler) ApproveRemediation(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	remIDStr := chi.URLParam(r, "id")
	var remID pgtype.UUID
	if err := remID.Scan(remIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid remediation ID format")
		return
	}

	var req ApproveRemediationRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	rem, execRes, err := h.service.Approve(r.Context(), tc.OrgID, remID, tc.UserID, req.Reason)
	if err != nil {
		if errors.Is(err, remediation.ErrRemediationNotFound) {
			respondError(w, http.StatusNotFound, "Remediation not found")
			return
		}
		if errors.Is(err, remediation.ErrRemediationExpired) {
			respondError(w, http.StatusGone, "Remediation has expired and cannot be approved")
			return
		}
		if errors.Is(err, remediation.ErrInvalidTransition) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"remediation": rem,
		"execution":   execRes,
	})
}

// RejectRemediation rejects a proposed remediation.
func (h *RemediationHandler) RejectRemediation(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	remIDStr := chi.URLParam(r, "id")
	var remID pgtype.UUID
	if err := remID.Scan(remIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid remediation ID format")
		return
	}

	var req RejectRemediationRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	rem, err := h.service.Reject(r.Context(), tc.OrgID, remID, tc.UserID, req.Reason)
	if err != nil {
		if errors.Is(err, remediation.ErrRemediationNotFound) {
			respondError(w, http.StatusNotFound, "Remediation not found")
			return
		}
		if errors.Is(err, remediation.ErrRemediationExpired) {
			respondError(w, http.StatusGone, "Remediation has expired and cannot be rejected")
			return
		}
		if errors.Is(err, remediation.ErrInvalidTransition) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, rem)
}

// RollbackRemediation triggers a rollback of a succeeded remediation.
func (h *RemediationHandler) RollbackRemediation(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	remIDStr := chi.URLParam(r, "id")
	var remID pgtype.UUID
	if err := remID.Scan(remIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid remediation ID format")
		return
	}

	var req RollbackRemediationRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	rem, execRes, err := h.service.Rollback(r.Context(), tc.OrgID, remID, tc.UserID, req.Reason)
	if err != nil {
		if errors.Is(err, remediation.ErrRemediationNotFound) {
			respondError(w, http.StatusNotFound, "Remediation not found")
			return
		}
		if errors.Is(err, remediation.ErrInvalidTransition) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"remediation": rem,
		"rollback":    execRes,
	})
}
