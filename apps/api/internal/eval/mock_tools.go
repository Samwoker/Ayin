package eval

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/seraph/triage/apps/api/internal/tools"
)

// ToolCallTracker records all tool executions during a scenario evaluation.
type ToolCallTracker struct {
	mu      sync.Mutex
	records []ToolCallRecord
}

func NewToolCallTracker() *ToolCallTracker {
	return &ToolCallTracker{
		records: make([]ToolCallRecord, 0),
	}
}

func (t *ToolCallTracker) Record(rec ToolCallRecord) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, rec)
}

func (t *ToolCallTracker) Records() []ToolCallRecord {
	t.mu.Lock()
	defer t.mu.Unlock()
	copied := make([]ToolCallRecord, len(t.records))
	copy(copied, t.records)
	return copied
}

func (t *ToolCallTracker) CalledTools() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	seen := make(map[string]bool)
	var list []string
	for _, r := range t.records {
		if !seen[r.ToolName] {
			seen[r.ToolName] = true
			list = append(list, r.ToolName)
		}
	}
	return list
}

func (t *ToolCallTracker) CountCalls(toolName string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	count := 0
	for _, r := range t.records {
		if r.ToolName == toolName {
			count++
		}
	}
	return count
}

// scenarioMockTool implements tools.Tool using scenario fixtures.
type scenarioMockTool struct {
	name        string
	description string
	schema      tools.ToolSchema
	tracker     *ToolCallTracker
	handler     func(ctx context.Context, input json.RawMessage) (any, error)
}

func (m *scenarioMockTool) Name() string {
	return m.name
}

func (m *scenarioMockTool) Description() string {
	return m.description
}

func (m *scenarioMockTool) Schema() tools.ToolSchema {
	return m.schema
}

func (m *scenarioMockTool) Execute(ctx context.Context, orgID pgtype.UUID, input json.RawMessage) (tools.ToolResult, error) {
	start := time.Now()
	var args map[string]any
	_ = json.Unmarshal(input, &args)

	data, err := m.handler(ctx, input)
	latency := time.Since(start).Milliseconds()

	outBytes, _ := json.Marshal(data)
	if m.tracker != nil {
		m.tracker.Record(ToolCallRecord{
			ToolName:   m.name,
			Arguments:  args,
			Success:    err == nil,
			LatencyMs:  latency,
			Timestamp:  time.Now(),
			OutputSize: len(outBytes),
		})
	}

	if err != nil {
		return tools.ToolResult{
			Tool:      m.name,
			Success:   false,
			Error:     err.Error(),
			LatencyMs: latency,
		}, nil
	}

	return tools.ToolResult{
		Tool:      m.name,
		Success:   true,
		Data:      data,
		LatencyMs: latency,
	}, nil
}

// BuildScenarioToolRegistry creates a populated ToolRegistry populated with the scenario's fixtures.
func BuildScenarioToolRegistry(sc *Scenario, tracker *ToolCallTracker) (*tools.ToolRegistry, error) {
	reg := tools.NewToolRegistry(tools.NewAllowAllPolicyProvider(), 15*time.Second)

	// 1. recent_deploys
	err := reg.Register(&scenarioMockTool{
		name:        "recent_deploys",
		description: "Retrieves recent deployment history, commit diffs, and release metadata for services.",
		schema: tools.ToolSchema{
			Type: "object",
			Properties: map[string]tools.PropertyDoc{
				"service": {Type: "string", Description: "Target service name"},
				"limit":   {Type: "integer", Description: "Maximum deployment events to return"},
			},
			Required: []string{"service"},
		},
		tracker: tracker,
		handler: func(ctx context.Context, input json.RawMessage) (any, error) {
			return map[string]any{
				"service": sc.Service,
				"deploys": sc.Environment.Deploys,
				"count":   len(sc.Environment.Deploys),
			}, nil
		},
	})
	if err != nil {
		return nil, err
	}

	// 2. query_metrics
	err = reg.Register(&scenarioMockTool{
		name:        "query_metrics",
		description: "Queries Prometheus/OTel metric time-series or rate calculations.",
		schema: tools.ToolSchema{
			Type: "object",
			Properties: map[string]tools.PropertyDoc{
				"query":   {Type: "string", Description: "Metric name or PromQL query"},
				"service": {Type: "string", Description: "Target service name"},
			},
			Required: []string{"query"},
		},
		tracker: tracker,
		handler: func(ctx context.Context, input json.RawMessage) (any, error) {
			var params struct {
				Query string `json:"query"`
			}
			_ = json.Unmarshal(input, &params)

			query := strings.ToLower(params.Query)
			matchedMetrics := make(map[string]MockMetricData)

			for mName, mData := range sc.Environment.Metrics {
				if strings.Contains(strings.ToLower(mName), query) || strings.Contains(query, strings.ToLower(mName)) || query == "" {
					matchedMetrics[mName] = mData
				}
			}

			if len(matchedMetrics) == 0 {
				// Return first available metric if query didn't match exactly
				for mName, mData := range sc.Environment.Metrics {
					matchedMetrics[mName] = mData
					break
				}
			}

			return map[string]any{
				"query":   params.Query,
				"service": sc.Service,
				"metrics": matchedMetrics,
			}, nil
		},
	})
	if err != nil {
		return nil, err
	}

	// 3. log_search
	err = reg.Register(&scenarioMockTool{
		name:        "log_search",
		description: "Searches structured and unstructured logs with regex filtering.",
		schema: tools.ToolSchema{
			Type: "object",
			Properties: map[string]tools.PropertyDoc{
				"service": {Type: "string", Description: "Service name"},
				"pattern": {Type: "string", Description: "Regex pattern to match"},
			},
			Required: []string{"service"},
		},
		tracker: tracker,
		handler: func(ctx context.Context, input json.RawMessage) (any, error) {
			var params struct {
				Pattern string `json:"pattern"`
			}
			_ = json.Unmarshal(input, &params)

			pattern := strings.ToLower(params.Pattern)
			var matched []MockLogRecord

			for _, l := range sc.Environment.Logs {
				if pattern == "" || strings.Contains(strings.ToLower(l.Message), pattern) || strings.Contains(strings.ToLower(l.Level), pattern) {
					matched = append(matched, l)
				}
			}

			// If no strict pattern matched, return all scenario logs
			if len(matched) == 0 {
				matched = sc.Environment.Logs
			}

			return map[string]any{
				"service":     sc.Service,
				"match_count": len(matched),
				"logs":        matched,
			}, nil
		},
	})
	if err != nil {
		return nil, err
	}

	// 4. service_topology
	err = reg.Register(&scenarioMockTool{
		name:        "service_topology",
		description: "Inspects dependency topology graphs, upstream callers, and downstream dependencies.",
		schema: tools.ToolSchema{
			Type: "object",
			Properties: map[string]tools.PropertyDoc{
				"service": {Type: "string", Description: "Service name"},
			},
			Required: []string{"service"},
		},
		tracker: tracker,
		handler: func(ctx context.Context, input json.RawMessage) (any, error) {
			topo := sc.Environment.Topology
			if topo.Service == "" {
				topo.Service = sc.Service
			}
			return topo, nil
		},
	})
	if err != nil {
		return nil, err
	}

	// 5. search_runbooks
	err = reg.Register(&scenarioMockTool{
		name:        "search_runbooks",
		description: "Performs semantic search across engineering Standard Operating Procedures (SOPs).",
		schema: tools.ToolSchema{
			Type: "object",
			Properties: map[string]tools.PropertyDoc{
				"query": {Type: "string", Description: "Search query"},
			},
			Required: []string{"query"},
		},
		tracker: tracker,
		handler: func(ctx context.Context, input json.RawMessage) (any, error) {
			return map[string]any{
				"runbooks": sc.Environment.Runbooks,
				"count":    len(sc.Environment.Runbooks),
			}, nil
		},
	})
	if err != nil {
		return nil, err
	}

	// 6. similar_incidents
	err = reg.Register(&scenarioMockTool{
		name:        "similar_incidents",
		description: "Searches past historical incidents with pgvector semantic similarity.",
		schema: tools.ToolSchema{
			Type: "object",
			Properties: map[string]tools.PropertyDoc{
				"query": {Type: "string", Description: "Query text"},
			},
		},
		tracker: tracker,
		handler: func(ctx context.Context, input json.RawMessage) (any, error) {
			return map[string]any{
				"incidents": sc.Environment.SimilarIncidents,
				"count":     len(sc.Environment.SimilarIncidents),
			}, nil
		},
	})
	if err != nil {
		return nil, err
	}

	return reg, nil
}
