package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/server/middleware"
	"github.com/seraph/triage/apps/api/internal/server/sse"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
)

// RunHandler manages investigation runs, step logs, and real-time SSE streaming.
type RunHandler struct {
	store  *store.Store
	broker *sse.Broker
}

// NewRunHandler creates a new RunHandler.
func NewRunHandler(st *store.Store, broker *sse.Broker) *RunHandler {
	return &RunHandler{
		store:  st,
		broker: broker,
	}
}

// CreateRun initiates a new investigation run for an incident.
func (h *RunHandler) CreateRun(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	incidentIDStr := chi.URLParam(r, "id")
	var incidentID pgtype.UUID
	if err := incidentID.Scan(incidentIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid incident ID")
		return
	}

	var run db.Run
	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		// Verify incident belongs to org
		_, err := q.GetIncidentByID(r.Context(), db.GetIncidentByIDParams{
			OrgID: tc.OrgID,
			ID:    incidentID,
		})
		if err != nil {
			return fmt.Errorf("incident not found: %w", err)
		}

		// Retrieve active prompt version if any
		activePrompt, _ := q.GetActivePromptVersion(r.Context())

		var promptVersionID pgtype.UUID
		if activePrompt.ID.Valid {
			promptVersionID = activePrompt.ID
		}

		run, err = q.CreateRun(r.Context(), db.CreateRunParams{
			OrgID:           tc.OrgID,
			IncidentID:      incidentID,
			PromptVersionID: promptVersionID,
			Status:          "pending",
		})
		if err != nil {
			return err
		}

		// Initial step entry: plan creation
		toolInput, _ := json.Marshal(map[string]any{"incident_id": incidentIDStr})
		_, _ = q.CreateRunStep(r.Context(), db.CreateRunStepParams{
			OrgID:      tc.OrgID,
			RunID:      run.ID,
			StepNumber: 1,
			StepType:   "plan",
			ToolName:   pgtype.Text{String: "planner", Valid: true},
			ToolInput:  toolInput,
		})

		return nil
	})

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create run: "+err.Error())
		return
	}

	// Broadcast run creation event to any connected SSE listeners
	runIDStr := uuidToString(run.ID)
	h.broker.Broadcast(runIDStr, "run_created", map[string]any{
		"run_id":   runIDStr,
		"status":   run.Status,
		"progress": "initialized",
	})

	respondJSON(w, http.StatusCreated, run)
}

// GetRun retrieves an investigation run, its hypotheses, and summary metrics.
func (h *RunHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	runIDStr := chi.URLParam(r, "id")
	var runID pgtype.UUID
	if err := runID.Scan(runIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid run ID")
		return
	}

	var run db.Run
	var hypotheses []db.Hypothesis
	var steps []db.RunStep

	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		var err error
		run, err = q.GetRunByID(r.Context(), db.GetRunByIDParams{
			OrgID: tc.OrgID,
			ID:    runID,
		})
		if err != nil {
			return err
		}

		hypotheses, _ = q.ListHypothesesForRun(r.Context(), db.ListHypothesesForRunParams{
			OrgID: tc.OrgID,
			RunID: runID,
		})

		steps, _ = q.ListRunSteps(r.Context(), db.ListRunStepsParams{
			OrgID: tc.OrgID,
			RunID: runID,
		})

		return nil
	})

	if err != nil {
		respondError(w, http.StatusNotFound, "Run not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"run":        run,
		"hypotheses": hypotheses,
		"steps":      steps,
	})
}

// GetRunSteps retrieves step-by-step logs for a specific run.
func (h *RunHandler) GetRunSteps(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	runIDStr := chi.URLParam(r, "id")
	var runID pgtype.UUID
	if err := runID.Scan(runIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid run ID")
		return
	}

	var steps []db.RunStep
	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		var err error
		steps, err = q.ListRunSteps(r.Context(), db.ListRunStepsParams{
			OrgID: tc.OrgID,
			RunID: runID,
		})
		return err
	})

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve run steps")
		return
	}

	respondJSON(w, http.StatusOK, steps)
}

// ListRunsForIncident retrieves all investigation runs for a specific incident.
func (h *RunHandler) ListRunsForIncident(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	incidentIDStr := chi.URLParam(r, "id")
	var incidentID pgtype.UUID
	if err := incidentID.Scan(incidentIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid incident ID")
		return
	}

	var runs []db.Run
	err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
		var err error
		runs, err = q.ListRunsForIncident(r.Context(), db.ListRunsForIncidentParams{
			OrgID:      tc.OrgID,
			IncidentID: incidentID,
		})
		return err
	})

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list runs for incident")
		return
	}

	respondJSON(w, http.StatusOK, runs)
}

// StreamRun provides a Server-Sent Events (SSE) endpoint to stream run steps in real time.
func (h *RunHandler) StreamRun(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "Streaming unsupported by client/server")
		return
	}

	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	runIDStr := chi.URLParam(r, "id")
	var runID pgtype.UUID
	if err := runID.Scan(runIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid run ID")
		return
	}

	// Verify run belongs to tenant organization
	if h.store != nil {
		err := h.store.WithOrgTx(r.Context(), tc.OrgID, func(q *db.Queries) error {
			_, err := q.GetRunByID(r.Context(), db.GetRunByIDParams{
				OrgID: tc.OrgID,
				ID:    runID,
			})
			return err
		})
		if err != nil {
			respondError(w, http.StatusNotFound, "Run not found")
			return
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	eventChan, unsubscribe := h.broker.Subscribe(runIDStr)
	defer unsubscribe()

	// Send initial connection packet
	initMsg := fmt.Sprintf("event: connected\ndata: {\"run_id\":\"%s\"}\n\n", runIDStr)
	_, _ = w.Write([]byte(initMsg))
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-eventChan:
			if !open {
				return
			}
			_, _ = w.Write(msg)
			flusher.Flush()
		}
	}
}
