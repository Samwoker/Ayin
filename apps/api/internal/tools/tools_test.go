package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/ingest"
	"github.com/seraph/triage/apps/api/internal/store"
)

func parseUUID(s string) pgtype.UUID {
	var id pgtype.UUID
	_ = id.Scan(s)
	return id
}

type panicTool struct{}

func (p *panicTool) Name() string        { return "panic_tool" }
func (p *panicTool) Description() string { return "Tool that simulates panic" }
func (p *panicTool) Schema() ToolSchema  { return ToolSchema{Type: "object"} }
func (p *panicTool) Execute(ctx context.Context, orgID pgtype.UUID, input json.RawMessage) (ToolResult, error) {
	panic("simulated panic in tool")
}

type slowTool struct{}

func (s *slowTool) Name() string        { return "slow_tool" }
func (s *slowTool) Description() string { return "Tool that sleeps" }
func (s *slowTool) Schema() ToolSchema  { return ToolSchema{Type: "object"} }
func (s *slowTool) Execute(ctx context.Context, orgID pgtype.UUID, input json.RawMessage) (ToolResult, error) {
	select {
	case <-time.After(2 * time.Second):
		return ToolResult{Success: true}, nil
	case <-ctx.Done():
		return ToolResult{Success: false, Error: ctx.Err().Error()}, ctx.Err()
	}
}

func TestToolRegistryAndPolicy(t *testing.T) {
	org1 := parseUUID("00000000-0000-0000-0000-000000000001")
	org2 := parseUUID("00000000-0000-0000-0000-000000000002")

	policy := NewAllowlistPolicyProvider()
	registry := NewToolRegistry(policy, 100*time.Millisecond)

	t.Run("Register validation", func(t *testing.T) {
		if err := registry.Register(nil); err == nil {
			t.Errorf("expected error when registering nil tool")
		}

		deploysTool := NewRecentDeploysTool(nil)
		if err := registry.Register(deploysTool); err != nil {
			t.Fatalf("unexpected error registering deploys tool: %v", err)
		}

		// Duplicate registration rejected
		if err := registry.Register(deploysTool); err == nil {
			t.Errorf("expected error registering duplicate tool")
		}

		if len(registry.List()) != 1 {
			t.Errorf("expected 1 registered tool, got %d", len(registry.List()))
		}
	})

	t.Run("Policy provider allowlist and denylist", func(t *testing.T) {
		ctx := context.Background()
		toolName := "recent_deploys"

		// Default allowed
		if !policy.IsToolAllowed(ctx, org1, toolName) {
			t.Errorf("expected tool to be allowed by default")
		}

		// Org-specific allowlist restricts other tools
		policy.SetOrgAllowed(org1, []string{"log_search"})
		if policy.IsToolAllowed(ctx, org1, toolName) {
			t.Errorf("expected tool to be disallowed when not in org allowlist")
		}
		if !policy.IsToolAllowed(ctx, org1, "log_search") {
			t.Errorf("expected log_search to be allowed in org allowlist")
		}

		// Other org without allowlist is unrestricted
		if !policy.IsToolAllowed(ctx, org2, toolName) {
			t.Errorf("expected org2 to remain allowed")
		}

		// Org-specific deny
		policy.SetOrgToolDeny(org2, toolName, true)
		if policy.IsToolAllowed(ctx, org2, toolName) {
			t.Errorf("expected tool to be denied for org2")
		}

		// Global deny
		policy.SetGlobalDeny("log_search", true)
		if policy.IsToolAllowed(ctx, org1, "log_search") {
			t.Errorf("expected log_search to be denied globally")
		}
	})

	t.Run("ListAvailable filters tools by org policy", func(t *testing.T) {
		ctx := context.Background()
		avail := registry.ListAvailable(ctx, org1)
		if len(avail) != 0 {
			t.Errorf("expected 0 available tools for restricted org1, got %d", len(avail))
		}
	})

	t.Run("Execute missing tool returns failure", func(t *testing.T) {
		res := registry.Execute(context.Background(), org1, "non_existent", nil)
		if res.Success {
			t.Errorf("expected execution to fail for missing tool")
		}
		if !strings.Contains(res.Error, "not found") {
			t.Errorf("expected not found error message, got: %s", res.Error)
		}
	})

	t.Run("Execute disallowed tool returns policy error", func(t *testing.T) {
		res := registry.Execute(context.Background(), org2, "recent_deploys", nil)
		if res.Success {
			t.Errorf("expected execution to fail for disallowed tool")
		}
		if !strings.Contains(res.Error, "disabled or disallowed") {
			t.Errorf("expected policy disallowed error, got: %s", res.Error)
		}
	})

	t.Run("Execute panic recovery", func(t *testing.T) {
		unrestrictedRegistry := NewToolRegistry(NewAllowAllPolicyProvider(), 100*time.Millisecond)
		_ = unrestrictedRegistry.Register(&panicTool{})

		res := unrestrictedRegistry.Execute(context.Background(), org1, "panic_tool", nil)
		if res.Success {
			t.Errorf("expected panic tool to fail")
		}
		if !strings.Contains(res.Error, "panic in tool") {
			t.Errorf("expected panic caught, got: %s", res.Error)
		}
	})

	t.Run("Execute timeout enforcement", func(t *testing.T) {
		timeoutRegistry := NewToolRegistry(NewAllowAllPolicyProvider(), 50*time.Millisecond)
		_ = timeoutRegistry.Register(&slowTool{})

		res := timeoutRegistry.Execute(context.Background(), org1, "slow_tool", nil)
		if res.Success {
			t.Errorf("expected slow tool to timeout")
		}
		if res.LatencyMs < 40 {
			t.Errorf("expected latency at least ~50ms, got %d ms", res.LatencyMs)
		}
	})
}

func TestLogSearchTool(t *testing.T) {
	orgID := parseUUID("00000000-0000-0000-0000-000000000001")

	// Set up temporary log directory with mock logs
	tmpDir, err := os.MkdirTemp("", "triage-tool-logs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logContent := `{"time":"2026-10-09T08:00:00Z","level":"info","service":"checkout-worker","msg":"worker started"}
{"time":"2026-10-09T08:05:00Z","level":"error","service":"checkout-worker","msg":"database deadlock detected on table orders"}
{"time":"2026-10-09T08:10:00Z","level":"warn","service":"checkout-worker","msg":"retrying connection to rabbitmq"}
`
	_ = os.WriteFile(filepath.Join(tmpDir, "checkout-worker.log"), []byte(logContent), 0644)

	adapter := ingest.NewFileLogAdapter(tmpDir)
	tool := NewLogSearchTool(adapter)

	if tool.Name() != "log_search" {
		t.Errorf("expected tool name 'log_search', got %s", tool.Name())
	}
	if len(tool.Description()) == 0 {
		t.Errorf("expected non-empty description")
	}
	if tool.Schema().Properties["service"].Type != "string" {
		t.Errorf("expected service property in schema")
	}

	t.Run("Search logs matching pattern", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{
			"service": "checkout-worker",
			"pattern": "deadlock",
			"limit":   10,
		})

		res, err := tool.Execute(context.Background(), orgID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Fatalf("expected success, got error: %s", res.Error)
		}

		dataMap, ok := res.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected data map, got %T", res.Data)
		}
		if dataMap["total_found"].(int) != 1 {
			t.Errorf("expected 1 match, got %v", dataMap["total_found"])
		}
		if dataMap["error_count"].(int) != 1 {
			t.Errorf("expected 1 error count, got %v", dataMap["error_count"])
		}
	})

	t.Run("Missing required service parameter", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{
			"pattern": "deadlock",
		})
		res, _ := tool.Execute(context.Background(), orgID, input)
		if res.Success {
			t.Errorf("expected failure when service is missing")
		}
		if !strings.Contains(res.Error, "service") {
			t.Errorf("expected service required error, got: %s", res.Error)
		}
	})

	t.Run("Nil adapter error", func(t *testing.T) {
		nilTool := NewLogSearchTool(nil)
		res, _ := nilTool.Execute(context.Background(), orgID, []byte(`{"service":"test"}`))
		if res.Success {
			t.Errorf("expected failure when adapter is nil")
		}
	})
}

func TestRecentDeploysTool(t *testing.T) {
	orgID := parseUUID("00000000-0000-0000-0000-000000000001")
	tool := NewRecentDeploysTool(nil)

	if tool.Name() != "recent_deploys" {
		t.Errorf("expected 'recent_deploys', got %s", tool.Name())
	}
	if tool.Schema().Properties["environment"].Type != "string" {
		t.Errorf("expected environment in schema")
	}

	t.Run("Fetch recent deploys for seeded service", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{
			"service":     "checkout-worker",
			"environment": "production",
			"limit":       5,
		})

		res, err := tool.Execute(context.Background(), orgID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Fatalf("expected success, got error: %s", res.Error)
		}

		dataMap := res.Data.(map[string]any)
		deploys := dataMap["recent_deploys"].([]Deployment)
		if len(deploys) == 0 {
			t.Fatalf("expected at least 1 deployment for checkout-worker")
		}

		latest := deploys[0]
		if latest.Service != "checkout-worker" {
			t.Errorf("expected checkout-worker service, got %s", latest.Service)
		}
		if latest.GitCommit == "" {
			t.Errorf("expected non-empty git commit")
		}
		if latest.RollbackVersion == "" {
			t.Errorf("expected non-empty rollback version")
		}
	})

	t.Run("Missing service parameter", func(t *testing.T) {
		res, _ := tool.Execute(context.Background(), orgID, []byte(`{}`))
		if res.Success {
			t.Errorf("expected error when service is missing")
		}
	})
}

func TestServiceTopologyTool(t *testing.T) {
	orgID := parseUUID("00000000-0000-0000-0000-000000000001")
	tool := NewServiceTopologyTool(nil)

	if tool.Name() != "service_topology" {
		t.Errorf("expected 'service_topology', got %s", tool.Name())
	}

	t.Run("Inspect service topology depth 1", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{
			"service": "checkout-worker",
			"depth":   1,
		})

		res, err := tool.Execute(context.Background(), orgID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Fatalf("expected success, got error: %s", res.Error)
		}

		topo := res.Data.(*ServiceTopology)
		if topo.TargetService != "checkout-worker" {
			t.Errorf("expected target checkout-worker, got %s", topo.TargetService)
		}
		if topo.Primary.Tier != "tier-1" {
			t.Errorf("expected tier-1, got %s", topo.Primary.Tier)
		}
		if len(topo.Primary.Downstream) != 2 {
			t.Errorf("expected 2 downstream services, got %d", len(topo.Primary.Downstream))
		}
	})

	t.Run("Inspect service topology depth 2 populates related nodes", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{
			"service": "checkout-worker",
			"depth":   2,
		})

		res, err := tool.Execute(context.Background(), orgID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		topo := res.Data.(*ServiceTopology)
		if len(topo.Related) == 0 {
			t.Errorf("expected related services populated at depth 2")
		}
		if _, ok := topo.Related["payment-api"]; !ok {
			t.Errorf("expected payment-api in related nodes")
		}
	})

	t.Run("Fallback for unregistered service", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{
			"service": "billing-exporter",
			"depth":   1,
		})

		res, err := tool.Execute(context.Background(), orgID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Fatalf("expected success for fallback service")
		}
		topo := res.Data.(*ServiceTopology)
		if topo.Primary.Service != "billing-exporter" {
			t.Errorf("expected billing-exporter, got %s", topo.Primary.Service)
		}
	})

	t.Run("Missing service parameter", func(t *testing.T) {
		res, _ := tool.Execute(context.Background(), orgID, []byte(`{}`))
		if res.Success {
			t.Errorf("expected failure when service is missing")
		}
	})
}

func TestQueryMetricsTool(t *testing.T) {
	orgID := parseUUID("00000000-0000-0000-0000-000000000001")
	tool := NewQueryMetricsTool(nil)

	if tool.Name() != "query_metrics" {
		t.Errorf("expected 'query_metrics', got %s", tool.Name())
	}

	t.Run("Query queue depth metric", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{
			"query":   "rabbitmq_queue_messages_ready{queue='orders'}",
			"service": "checkout-worker",
			"step":    "1m",
		})

		res, err := tool.Execute(context.Background(), orgID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Fatalf("expected success, got: %s", res.Error)
		}

		dataMap := res.Data.(map[string]any)
		seriesList := dataMap["series"].([]MetricSeries)
		if len(seriesList) == 0 {
			t.Fatalf("expected at least 1 metric series")
		}

		series := seriesList[0]
		if len(series.Points) == 0 {
			t.Fatalf("expected sample points")
		}
		if series.Max <= series.Min {
			t.Errorf("expected max > min for spike metric")
		}
		if series.Latest <= 0 {
			t.Errorf("expected positive latest value")
		}
	})

	t.Run("Query CPU usage metric", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{
			"query":   "container_cpu_usage_seconds_total",
			"service": "payment-api",
		})

		res, err := tool.Execute(context.Background(), orgID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		dataMap := res.Data.(map[string]any)
		seriesList := dataMap["series"].([]MetricSeries)
		if seriesList[0].Max < 80.0 {
			t.Errorf("expected CPU spike above 80%%, got %v", seriesList[0].Max)
		}
	})

	t.Run("Missing query parameter", func(t *testing.T) {
		res, _ := tool.Execute(context.Background(), orgID, []byte(`{"service":"test"}`))
		if res.Success {
			t.Errorf("expected error when query is missing")
		}
	})
}

func TestSearchRunbooksTool(t *testing.T) {
	orgID := parseUUID("00000000-0000-0000-0000-000000000001")

	t.Run("Fallback in-memory runbook search", func(t *testing.T) {
		tool := NewSearchRunbooksTool(nil)

		if tool.Name() != "search_runbooks" {
			t.Errorf("expected 'search_runbooks', got %s", tool.Name())
		}

		input, _ := json.Marshal(map[string]any{
			"query": "RabbitMQ queue backlog escalation",
			"limit": 3,
		})

		res, err := tool.Execute(context.Background(), orgID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Fatalf("expected success, got: %s", res.Error)
		}

		dataMap := res.Data.(map[string]any)
		runbooks := dataMap["runbooks"].([]RunbookMatch)
		if len(runbooks) == 0 {
			t.Fatalf("expected matched runbooks")
		}
		if !strings.Contains(runbooks[0].Title, "RabbitMQ") {
			t.Errorf("expected RabbitMQ runbook title, got: %s", runbooks[0].Title)
		}
		if runbooks[0].Snippet == "" {
			t.Errorf("expected non-empty snippet")
		}
	})

	t.Run("Missing query parameter", func(t *testing.T) {
		tool := NewSearchRunbooksTool(nil)
		res, _ := tool.Execute(context.Background(), orgID, []byte(`{}`))
		if res.Success {
			t.Errorf("expected failure when query is missing")
		}
	})

	t.Run("Live store search if postgres reachable", func(t *testing.T) {
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			dbURL = "postgres://triage:triage_secret_dev@localhost:5433/triage?sslmode=disable"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		st, err := store.NewStore(ctx, dbURL)
		if err != nil {
			t.Skipf("skipping live database test: postgres not reachable (%v)", err)
			return
		}
		defer st.Close()

		tool := NewSearchRunbooksTool(st)
		input, _ := json.Marshal(map[string]any{
			"query": "RabbitMQ",
			"limit": 5,
		})

		res, err := tool.Execute(ctx, orgID, input)
		if err != nil {
			t.Fatalf("unexpected live execution error: %v", err)
		}
		if !res.Success {
			t.Fatalf("expected success in live runbook search, got: %s", res.Error)
		}
	})
}

func TestSimilarIncidentsTool(t *testing.T) {
	orgID := parseUUID("00000000-0000-0000-0000-000000000001")

	t.Run("Fallback in-memory similar incidents search", func(t *testing.T) {
		tool := NewSimilarIncidentsTool(nil)

		if tool.Name() != "similar_incidents" {
			t.Errorf("expected 'similar_incidents', got %s", tool.Name())
		}

		input, _ := json.Marshal(map[string]any{
			"service": "checkout-worker",
			"query":   "Queue Depth Escalation",
			"limit":   5,
		})

		res, err := tool.Execute(context.Background(), orgID, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Success {
			t.Fatalf("expected success, got error: %s", res.Error)
		}

		dataMap := res.Data.(map[string]any)
		incidents := dataMap["similar_incidents"].([]HistoricalIncidentSummary)
		if len(incidents) == 0 {
			t.Fatalf("expected similar incidents returned")
		}
		if incidents[0].Service != "checkout-worker" {
			t.Errorf("expected checkout-worker service, got %s", incidents[0].Service)
		}
	})

	t.Run("Live store search if postgres reachable", func(t *testing.T) {
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			dbURL = "postgres://triage:triage_secret_dev@localhost:5433/triage?sslmode=disable"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		st, err := store.NewStore(ctx, dbURL)
		if err != nil {
			t.Skipf("skipping live database test: postgres not reachable (%v)", err)
			return
		}
		defer st.Close()

		tool := NewSimilarIncidentsTool(st)
		input, _ := json.Marshal(map[string]any{
			"service": "checkout-worker",
			"limit":   5,
		})

		res, err := tool.Execute(ctx, orgID, input)
		if err != nil {
			t.Fatalf("unexpected live execution error: %v", err)
		}
		if !res.Success {
			t.Fatalf("expected success, got: %s", res.Error)
		}
	})
}
