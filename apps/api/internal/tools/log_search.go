package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/ingest"
)

// LogSearchTool inspects log streams for specific error patterns and time windows.
type LogSearchTool struct {
	adapter ingest.LogAdapter
}

func NewLogSearchTool(adapter ingest.LogAdapter) *LogSearchTool {
	return &LogSearchTool{
		adapter: adapter,
	}
}

func (t *LogSearchTool) Name() string {
	return "log_search"
}

func (t *LogSearchTool) Description() string {
	return "Searches log streams for a target service over an optional time window and regex pattern to extract stack traces, exceptions, and error anomalies."
}

func (t *LogSearchTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Required: []string{"service"},
		Properties: map[string]PropertyDoc{
			"service": {
				Type:        "string",
				Description: "The name of the service whose logs should be searched (e.g. 'checkout-worker', 'payment-api').",
			},
			"pattern": {
				Type:        "string",
				Description: "Case-insensitive substring or regex pattern to search for (e.g. 'timeout', 'connection refused', 'NullPointerException').",
			},
			"start_time": {
				Type:        "string",
				Format:      "date-time",
				Description: "Optional RFC3339 start timestamp for the search window (e.g. '2026-10-09T08:00:00Z'). Defaults to 30 minutes ago.",
			},
			"end_time": {
				Type:        "string",
				Format:      "date-time",
				Description: "Optional RFC3339 end timestamp for the search window. Defaults to now.",
			},
			"limit": {
				Type:        "integer",
				Description: "Maximum number of log entries to retrieve (default: 50, maximum: 200).",
				Default:     50,
			},
		},
	}
}

type logSearchInput struct {
	Service   string `json:"service"`
	Pattern   string `json:"pattern"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Limit     int    `json:"limit"`
}

func (t *LogSearchTool) Execute(ctx context.Context, orgID pgtype.UUID, rawInput json.RawMessage) (ToolResult, error) {
	if t.adapter == nil {
		return ToolResult{
			Success: false,
			Error:   "no log adapter configured for log_search tool",
		}, nil
	}

	var in logSearchInput
	if len(rawInput) > 0 {
		if err := json.Unmarshal(rawInput, &in); err != nil {
			return ToolResult{
				Success: false,
				Error:   fmt.Sprintf("invalid input parameters: %v", err),
			}, nil
		}
	}

	if in.Service == "" {
		return ToolResult{
			Success: false,
			Error:   "parameter 'service' is required",
		}, nil
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var start, end time.Time
	if in.StartTime != "" {
		if parsed, err := time.Parse(time.RFC3339, in.StartTime); err == nil {
			start = parsed
		}
	}
	if in.EndTime != "" {
		if parsed, err := time.Parse(time.RFC3339, in.EndTime); err == nil {
			end = parsed
		}
	}

	entries, err := t.adapter.QueryLogs(ctx, ingest.LogQuery{
		Service:   in.Service,
		Pattern:   in.Pattern,
		StartTime: start,
		EndTime:   end,
		Limit:     limit,
	})
	if err != nil {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to query logs from adapter (%s): %v", t.adapter.Name(), err),
		}, nil
	}

	var errorCount, warnCount, infoCount int
	for _, e := range entries {
		switch e.Level {
		case "error", "fatal", "panic":
			errorCount++
		case "warn":
			warnCount++
		default:
			infoCount++
		}
	}

	return ToolResult{
		Success: true,
		Data: map[string]any{
			"service":      in.Service,
			"pattern":      in.Pattern,
			"total_found":  len(entries),
			"error_count":  errorCount,
			"warn_count":   warnCount,
			"info_count":   infoCount,
			"log_adapter":  t.adapter.Name(),
			"entries":      entries,
		},
		Metadata: map[string]any{
			"adapter": t.adapter.Name(),
			"count":   len(entries),
		},
	}, nil
}
