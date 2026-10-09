package tools

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
)

// Tool represents an executable read-only diagnostic or context-gathering tool for incident triage.
type Tool interface {
	// Name returns the unique tool identifier (e.g. "log_search").
	Name() string

	// Description returns a concise human/LLM-readable description of the tool's purpose and usage.
	Description() string

	// Schema returns the JSON schema definition for the tool's input parameters.
	Schema() ToolSchema

	// Execute runs the tool within the tenant's security boundary.
	Execute(ctx context.Context, orgID pgtype.UUID, input json.RawMessage) (ToolResult, error)
}

// ToolSchema defines a JSON Schema structure for tool parameters.
type ToolSchema struct {
	Type        string                 `json:"type"`
	Properties  map[string]PropertyDoc `json:"properties"`
	Required    []string               `json:"required,omitempty"`
	Description string                 `json:"description,omitempty"`
}

// PropertyDoc describes an individual field in the tool parameter schema.
type PropertyDoc struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Format      string   `json:"format,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// ToolResult encapsulates the structured output of a tool invocation.
type ToolResult struct {
	Tool      string         `json:"tool"`
	Success   bool           `json:"success"`
	Data      any            `json:"data,omitempty"`
	Error     string         `json:"error,omitempty"`
	LatencyMs int64          `json:"latency_ms"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}
