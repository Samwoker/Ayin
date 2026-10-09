package handlers

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/seraph/triage/apps/api/internal/server/middleware"
	"github.com/seraph/triage/apps/api/internal/tools"
)

// ToolsHandler handles tool discovery and on-demand tool execution for authorized responders.
type ToolsHandler struct {
	registry *tools.ToolRegistry
}

func NewToolsHandler(registry *tools.ToolRegistry) *ToolsHandler {
	return &ToolsHandler{registry: registry}
}

// ToolInfoDTO summarizes an available tool and its input parameter schema.
type ToolInfoDTO struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Schema      tools.ToolSchema `json:"schema"`
}

// ListTools lists all investigation tools permitted for the active organization.
func (h *ToolsHandler) ListTools(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	available := h.registry.ListAvailable(r.Context(), tc.OrgID)
	out := make([]ToolInfoDTO, 0, len(available))
	for _, t := range available {
		out = append(out, ToolInfoDTO{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"tools": out,
	})
}

// ExecuteTool allows responders to run an investigation tool on-demand within their tenant boundary.
func (h *ToolsHandler) ExecuteTool(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	toolName := chi.URLParam(r, "name")
	if toolName == "" {
		respondError(w, http.StatusBadRequest, "Tool name parameter is required")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB payload limit
	if err != nil {
		respondError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	res := h.registry.Execute(r.Context(), tc.OrgID, toolName, body)
	if !res.Success {
		respondJSON(w, http.StatusBadRequest, res)
		return
	}

	respondJSON(w, http.StatusOK, res)
}
