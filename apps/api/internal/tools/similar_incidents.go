package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
)

// HistoricalIncidentSummary outlines a prior resolved incident and its recovery timeline.
type HistoricalIncidentSummary struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Service            string `json:"service"`
	Severity           string `json:"severity"`
	Status             string `json:"status"`
	OpenedAt           string `json:"opened_at"`
	ResolvedAt         string `json:"resolved_at,omitempty"`
	DurationMinutes    int    `json:"duration_minutes,omitempty"`
	ResolutionSummary  string `json:"resolution_summary,omitempty"`
}

// SimilarIncidentsTool searches past incidents for similar symptoms and proven remediations.
type SimilarIncidentsTool struct {
	store *store.Store
}

func NewSimilarIncidentsTool(st *store.Store) *SimilarIncidentsTool {
	return &SimilarIncidentsTool{
		store: st,
	}
}

func (t *SimilarIncidentsTool) Name() string {
	return "similar_incidents"
}

func (t *SimilarIncidentsTool) Description() string {
	return "Finds previous resolved incidents matching a service name or symptoms to identify recurring problems, known root causes, and successful mitigations."
}

func (t *SimilarIncidentsTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]PropertyDoc{
			"service": {
				Type:        "string",
				Description: "Service name to filter historical incidents (e.g. 'checkout-worker', 'payment-api').",
			},
			"query": {
				Type:        "string",
				Description: "Keywords to match against incident titles (e.g. 'deadlock', 'backpressure', 'timeout').",
			},
			"exclude_id": {
				Type:        "string",
				Format:      "uuid",
				Description: "The current incident UUID to exclude from results.",
			},
			"limit": {
				Type:        "integer",
				Description: "Maximum number of historical incidents to return (default: 5, max: 15).",
				Default:     5,
			},
		},
	}
}

type similarIncidentsInput struct {
	Service   string `json:"service"`
	Query     string `json:"query"`
	ExcludeID string `json:"exclude_id"`
	Limit     int    `json:"limit"`
}

func (t *SimilarIncidentsTool) Execute(ctx context.Context, orgID pgtype.UUID, rawInput json.RawMessage) (ToolResult, error) {
	var in similarIncidentsInput
	if len(rawInput) > 0 {
		if err := json.Unmarshal(rawInput, &in); err != nil {
			return ToolResult{
				Success: false,
				Error:   fmt.Sprintf("invalid input parameters: %v", err),
			}, nil
		}
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 15 {
		limit = 15
	}

	var results []HistoricalIncidentSummary

	if t.store != nil {
		var serviceText pgtype.Text
		if in.Service != "" {
			serviceText = pgtype.Text{String: in.Service, Valid: true}
		}

		var queryText pgtype.Text
		if in.Query != "" {
			queryText = pgtype.Text{String: in.Query, Valid: true}
		}

		var excludeUUID pgtype.UUID
		if in.ExcludeID != "" {
			_ = excludeUUID.Scan(in.ExcludeID)
		}

		err := t.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
			rows, err := q.SearchHistoricalIncidents(ctx, db.SearchHistoricalIncidentsParams{
				OrgID:     orgID,
				Limit:     int32(limit),
				Service:   serviceText,
				Query:     queryText,
				ExcludeID: excludeUUID,
			})
			if err != nil {
				return err
			}

			for _, row := range rows {
				idStr := fmt.Sprintf("%x-%x-%x-%x-%x", row.ID.Bytes[0:4], row.ID.Bytes[4:6], row.ID.Bytes[6:8], row.ID.Bytes[8:10], row.ID.Bytes[10:16])
				durationMin := 0
				resolvedAtStr := ""
				if row.ResolvedAt.Valid {
					resolvedAtStr = row.ResolvedAt.Time.Format(time.RFC3339)
					durationMin = int(row.ResolvedAt.Time.Sub(row.OpenedAt.Time).Minutes())
					if durationMin < 0 {
						durationMin = 0
					}
				}

				results = append(results, HistoricalIncidentSummary{
					ID:              idStr,
					Title:           row.Title,
					Service:         row.Service,
					Severity:        row.Severity,
					Status:          row.Status,
					OpenedAt:        row.OpenedAt.Time.Format(time.RFC3339),
					ResolvedAt:      resolvedAtStr,
					DurationMinutes: durationMin,
				})
			}
			return nil
		})
		if err != nil {
			return ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to search historical incidents: %v", err),
			}, nil
		}
	} else {
		// Mock fallback for disconnected or unit testing
		now := time.Now().UTC()
		svc := in.Service
		if svc == "" {
			svc = "checkout-worker"
		}

		results = append(results, HistoricalIncidentSummary{
			ID:                 "inc-hist-001",
			Title:              fmt.Sprintf("Prior %s Queue Depth Escalation", svc),
			Service:            svc,
			Severity:           "sev1",
			Status:             "resolved",
			OpenedAt:           now.Add(-48 * time.Hour).Format(time.RFC3339),
			ResolvedAt:         now.Add(-47 * time.Hour).Format(time.RFC3339),
			DurationMinutes:    60,
			ResolutionSummary:  "Scaled worker pods from 2 to 10 and cleared unacknowledged messages.",
		})
	}

	return ToolResult{
		Success: true,
		Data: map[string]any{
			"service":           in.Service,
			"query":             in.Query,
			"total_found":       len(results),
			"similar_incidents": results,
		},
		Metadata: map[string]any{
			"count": len(results),
		},
	}, nil
}
