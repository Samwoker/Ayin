package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/server/middleware"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
)

// RunbookHandler manages runbook knowledge documents.
type RunbookHandler struct {
	store *store.Store
}

// NewRunbookHandler creates a new RunbookHandler.
func NewRunbookHandler(st *store.Store) *RunbookHandler {
	return &RunbookHandler{store: st}
}

type CreateRunbookRequest struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	ContentMarkdown string `json:"content_markdown"`
}

// CreateRunbook registers a new runbook for semantic retrieval and agent guidance.
func (h *RunbookHandler) CreateRunbook(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreateRunbookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" || req.ContentMarkdown == "" {
		respondError(w, http.StatusBadRequest, "Title and content_markdown are required")
		return
	}

	var runbook db.Runbook
	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		var err error
		runbook, err = q.CreateRunbook(r.Context(), db.CreateRunbookParams{
			OrgID:           tc.OrgID,
			Title:           req.Title,
			Description:     req.Description,
			ContentMarkdown: req.ContentMarkdown,
		})
		return err
	})

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create runbook: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, runbook)
}

// ListRunbooks lists all runbooks for the active organization.
func (h *RunbookHandler) ListRunbooks(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var runbooks []db.Runbook
	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		var err error
		runbooks, err = q.ListRunbooks(r.Context(), tc.OrgID)
		return err
	})

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list runbooks")
		return
	}

	respondJSON(w, http.StatusOK, runbooks)
}

// GetRunbook returns a runbook by ID.
func (h *RunbookHandler) GetRunbook(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	idStr := chi.URLParam(r, "id")
	var id pgtype.UUID
	if err := id.Scan(idStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid runbook ID")
		return
	}

	var runbook db.Runbook
	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		var err error
		runbook, err = q.GetRunbookByID(r.Context(), db.GetRunbookByIDParams{
			OrgID: tc.OrgID,
			ID:    id,
		})
		return err
	})

	if err != nil {
		respondError(w, http.StatusNotFound, "Runbook not found")
		return
	}

	respondJSON(w, http.StatusOK, runbook)
}

// UpdateRunbook updates an existing runbook.
func (h *RunbookHandler) UpdateRunbook(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	idStr := chi.URLParam(r, "id")
	var id pgtype.UUID
	if err := id.Scan(idStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid runbook ID")
		return
	}

	var req CreateRunbookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" || req.ContentMarkdown == "" {
		respondError(w, http.StatusBadRequest, "Title and content_markdown are required")
		return
	}

	var updated db.Runbook
	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		var err error
		updated, err = q.UpdateRunbook(r.Context(), db.UpdateRunbookParams{
			OrgID:           tc.OrgID,
			ID:              id,
			Title:           req.Title,
			Description:     req.Description,
			ContentMarkdown: req.ContentMarkdown,
		})
		return err
	})

	if err != nil {
		respondError(w, http.StatusNotFound, "Runbook not found or update failed")
		return
	}

	respondJSON(w, http.StatusOK, updated)
}

// DeleteRunbook deletes an existing runbook.
func (h *RunbookHandler) DeleteRunbook(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	idStr := chi.URLParam(r, "id")
	var id pgtype.UUID
	if err := id.Scan(idStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid runbook ID")
		return
	}

	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		return q.DeleteRunbook(r.Context(), db.DeleteRunbookParams{
			OrgID: tc.OrgID,
			ID:    id,
		})
	})

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete runbook")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
