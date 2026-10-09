package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/server/middleware"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
)

// IncidentHandler manages incident lifecycle and events.
type IncidentHandler struct {
	store *store.Store
}

// NewIncidentHandler creates a new IncidentHandler.
func NewIncidentHandler(st *store.Store) *IncidentHandler {
	return &IncidentHandler{store: st}
}

type CreateIncidentRequest struct {
	Title    string `json:"title"`
	Service  string `json:"service"`
	Severity string `json:"severity"` // sev1 - sev5
	Status   string `json:"status"`   // default: open
}

// CreateIncident creates a new incident for the active organization.
func (h *IncidentHandler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreateIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" || req.Service == "" {
		respondError(w, http.StatusBadRequest, "Title and service are required")
		return
	}

	if req.Severity == "" {
		req.Severity = "sev2"
	}
	if req.Status == "" {
		req.Status = "open"
	}

	var incident db.Incident
	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		var err error
		incident, err = q.CreateIncident(r.Context(), db.CreateIncidentParams{
			OrgID:    tc.OrgID,
			Title:    req.Title,
			Service:  req.Service,
			Severity: req.Severity,
			Status:   req.Status,
		})
		if err != nil {
			return err
		}

		// Record initial event
		eventPayload, _ := json.Marshal(map[string]any{
			"action":  "incident_created",
			"service": req.Service,
			"actor":   tc.Email,
		})
		_, _ = q.CreateIncidentEvent(r.Context(), db.CreateIncidentEventParams{
			OrgID:      tc.OrgID,
			IncidentID: incident.ID,
			EventType:  "created",
			Payload:    eventPayload,
		})
		return nil
	})

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create incident: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, incident)
}

// ListIncidents lists incidents for the active organization with pagination.
func (h *IncidentHandler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var incidents []db.Incident
	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		var err error
		incidents, err = q.ListIncidents(r.Context(), db.ListIncidentsParams{
			OrgID:  tc.OrgID,
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		return err
	})

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list incidents")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"incidents": incidents,
		"limit":     limit,
		"offset":    offset,
	})
}

// GetIncident retrieves details for a single incident including alerts and events.
func (h *IncidentHandler) GetIncident(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	idStr := chi.URLParam(r, "id")
	var incidentID pgtype.UUID
	if err := incidentID.Scan(idStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid incident ID")
		return
	}

	var incident db.Incident
	var alerts []db.Alert
	var events []db.IncidentEvent

	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		var err error
		incident, err = q.GetIncidentByID(r.Context(), db.GetIncidentByIDParams{
			OrgID: tc.OrgID,
			ID:    incidentID,
		})
		if err != nil {
			return err
		}

		alerts, _ = q.ListAlertsForIncident(r.Context(), db.ListAlertsForIncidentParams{
			OrgID:      tc.OrgID,
			IncidentID: incidentID,
		})

		events, _ = q.ListIncidentEvents(r.Context(), db.ListIncidentEventsParams{
			OrgID:      tc.OrgID,
			IncidentID: incidentID,
		})
		return nil
	})

	if err != nil {
		respondError(w, http.StatusNotFound, "Incident not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"incident": incident,
		"alerts":   alerts,
		"events":   events,
	})
}

// UpdateIncidentStatus updates the status of an existing incident.
func (h *IncidentHandler) UpdateIncidentStatus(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	idStr := chi.URLParam(r, "id")
	var incidentID pgtype.UUID
	if err := incidentID.Scan(idStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid incident ID")
		return
	}

	var req struct {
		Status string `json:"status"` // open, investigating, mitigated, resolved, closed
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Status == "" {
		respondError(w, http.StatusBadRequest, "Status is required")
		return
	}

	var resolvedAt pgtype.Timestamptz
	if req.Status == "resolved" || req.Status == "closed" {
		resolvedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	}

	var updated db.Incident
	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		var err error
		updated, err = q.UpdateIncidentStatus(r.Context(), db.UpdateIncidentStatusParams{
			OrgID:      tc.OrgID,
			ID:         incidentID,
			Status:     req.Status,
			ResolvedAt: resolvedAt,
		})
		if err != nil {
			return err
		}

		eventPayload, _ := json.Marshal(map[string]any{
			"new_status": req.Status,
			"actor":      tc.Email,
		})
		_, _ = q.CreateIncidentEvent(r.Context(), db.CreateIncidentEventParams{
			OrgID:      tc.OrgID,
			IncidentID: incidentID,
			EventType:  "status_change",
			Payload:    eventPayload,
		})
		return nil
	})

	if err != nil {
		respondError(w, http.StatusNotFound, "Incident not found or update failed")
		return
	}

	respondJSON(w, http.StatusOK, updated)
}
