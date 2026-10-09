package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
)

// SearchRunbooksTool searches organizational SOPs and runbooks to provide responders with verified remediations.
type SearchRunbooksTool struct {
	store *store.Store
}

func NewSearchRunbooksTool(st *store.Store) *SearchRunbooksTool {
	return &SearchRunbooksTool{
		store: st,
	}
}

func (t *SearchRunbooksTool) Name() string {
	return "search_runbooks"
}

func (t *SearchRunbooksTool) Description() string {
	return "Searches internal runbooks and standard operating procedures (SOPs) for mitigation steps, emergency commands, and troubleshooting guides."
}

func (t *SearchRunbooksTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Required: []string{"query"},
		Properties: map[string]PropertyDoc{
			"query": {
				Type:        "string",
				Description: "Keywords, symptoms, or error terms to search in runbooks (e.g. 'rabbitmq queue backlog', 'database connection pool', 'oom kill').",
			},
			"limit": {
				Type:        "integer",
				Description: "Maximum number of runbooks to return (default: 3, max: 10).",
				Default:     3,
			},
		},
	}
}

type searchRunbooksInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type RunbookMatch struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content_markdown"`
	Snippet     string `json:"snippet"`
}

func (t *SearchRunbooksTool) Execute(ctx context.Context, orgID pgtype.UUID, rawInput json.RawMessage) (ToolResult, error) {
	var in searchRunbooksInput
	if len(rawInput) > 0 {
		if err := json.Unmarshal(rawInput, &in); err != nil {
			return ToolResult{
				Success: false,
				Error:   fmt.Sprintf("invalid input parameters: %v", err),
			}, nil
		}
	}

	if in.Query == "" {
		return ToolResult{
			Success: false,
			Error:   "parameter 'query' is required",
		}, nil
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 3
	}
	if limit > 10 {
		limit = 10
	}

	var matches []RunbookMatch

	if t.store != nil {
		err := t.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
			rows, err := q.SearchRunbooksByText(ctx, db.SearchRunbooksByTextParams{
				OrgID:   orgID,
				Column2: pgtype.Text{String: in.Query, Valid: true},
				Limit:   int32(limit),
			})
			if err != nil {
				return err
			}

			for _, row := range rows {
				idStr := fmt.Sprintf("%x-%x-%x-%x-%x", row.ID.Bytes[0:4], row.ID.Bytes[4:6], row.ID.Bytes[6:8], row.ID.Bytes[8:10], row.ID.Bytes[10:16])
				snippet := extractSnippet(row.ContentMarkdown, in.Query)

				matches = append(matches, RunbookMatch{
					ID:          idStr,
					Title:       row.Title,
					Description: row.Description,
					Content:     row.ContentMarkdown,
					Snippet:     snippet,
				})
			}
			return nil
		})
		if err != nil {
			return ToolResult{
				Success: false,
				Error:   fmt.Sprintf("runbook search query failed: %v", err),
			}, nil
		}
	} else {
		// Mock fallback for disconnected or unit testing
		if strings.Contains(strings.ToLower(in.Query), "rabbitmq") || strings.Contains(strings.ToLower(in.Query), "queue") {
			matches = append(matches, RunbookMatch{
				ID:          "rb-mock-001",
				Title:       "RabbitMQ Consumer Backpressure and Queue Drain Runbook",
				Description: "Standard remediation guide for high queue depth and unacknowledged message spikes.",
				Content:     "# RabbitMQ Remediation\n\n1. Inspect consumer pod status\n2. Scale consumer replicas from 2 to 10\n3. Verify prefetch configuration",
				Snippet:     "Scale consumer replicas from 2 to 10 to clear queue backlog.",
			})
		} else {
			matches = append(matches, RunbookMatch{
				ID:          "rb-mock-002",
				Title:       "Generic Incident First-Response SOP",
				Description: "Standard triage protocol for web and backend services.",
				Content:     "# Triage Protocol\n\n1. Identify failing service\n2. Check recent deploys\n3. Restart pod if memory leak detected",
				Snippet:     "Identify failing service and check recent deploys.",
			})
		}
	}

	return ToolResult{
		Success: true,
		Data: map[string]any{
			"query":         in.Query,
			"total_matches": len(matches),
			"runbooks":      matches,
		},
		Metadata: map[string]any{
			"count": len(matches),
		},
	}, nil
}

func extractSnippet(content, query string) string {
	idx := strings.Index(strings.ToLower(content), strings.ToLower(query))
	if idx == -1 {
		if len(content) > 200 {
			return content[:200] + "..."
		}
		return content
	}

	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 80
	if end > len(content) {
		end = len(content)
	}

	prefix := ""
	if start > 0 {
		prefix = "..."
	}
	suffix := ""
	if end < len(content) {
		suffix = "..."
	}

	return prefix + strings.TrimSpace(content[start:end]) + suffix
}
