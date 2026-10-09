package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/ingest"
	"github.com/seraph/triage/apps/api/internal/server/sse"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
	"github.com/seraph/triage/apps/api/internal/tools"
)

func parseUUID(s string) pgtype.UUID {
	var id pgtype.UUID
	_ = id.Scan(s)
	return id
}

func setupTestToolRegistry() *tools.ToolRegistry {
	reg := tools.NewToolRegistry(tools.NewAllowAllPolicyProvider(), 5*time.Second)
	_ = reg.Register(tools.NewRecentDeploysTool(nil))
	_ = reg.Register(tools.NewQueryMetricsTool(nil))
	_ = reg.Register(tools.NewServiceTopologyTool(nil))
	_ = reg.Register(tools.NewSearchRunbooksTool(nil))
	return reg
}

func TestMockLLMClientMultiTurn(t *testing.T) {
	client := NewMockLLMClient()
	ctx := context.Background()

	// Turn 1
	resp1, err := client.CreateChatCompletion(ctx, ChatRequest{})
	if err != nil {
		t.Fatalf("turn 1 error: %v", err)
	}
	if len(resp1.Message.ToolCalls) != 2 {
		t.Errorf("expected 2 tool calls in turn 1, got %d", len(resp1.Message.ToolCalls))
	}

	// Turn 2
	resp2, err := client.CreateChatCompletion(ctx, ChatRequest{})
	if err != nil {
		t.Fatalf("turn 2 error: %v", err)
	}
	if len(resp2.Message.ToolCalls) != 2 {
		t.Errorf("expected 2 tool calls in turn 2, got %d", len(resp2.Message.ToolCalls))
	}

	// Turn 3
	resp3, err := client.CreateChatCompletion(ctx, ChatRequest{})
	if err != nil {
		t.Fatalf("turn 3 error: %v", err)
	}
	if len(resp3.Message.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call in turn 3, got %d", len(resp3.Message.ToolCalls))
	}

	// Turn 4: Conclusion
	resp4, err := client.CreateChatCompletion(ctx, ChatRequest{})
	if err != nil {
		t.Fatalf("turn 4 error: %v", err)
	}
	if len(resp4.Message.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls in turn 4, got %d", len(resp4.Message.ToolCalls))
	}

	hypotheses, err := ExtractHypothesesFromText(resp4.Message.Content)
	if err != nil {
		t.Fatalf("failed to extract hypotheses: %v", err)
	}
	if len(hypotheses) != 2 {
		t.Fatalf("expected 2 hypotheses, got %d", len(hypotheses))
	}
	if hypotheses[0].Rank != 1 || hypotheses[0].Confidence < 0.85 {
		t.Errorf("expected rank 1 with high confidence, got %v", hypotheses[0])
	}

	t.Run("Injected error", func(t *testing.T) {
		errClient := NewMockLLMClient()
		errClient.SetError(fmt.Errorf("api rate limited"))
		_, err := errClient.CreateChatCompletion(ctx, ChatRequest{})
		if err == nil {
			t.Fatalf("expected error from injected error client")
		}
	})

	t.Run("Custom scripted turns", func(t *testing.T) {
		scriptClient := NewMockLLMClient()
		scriptClient.SetCustomTurns([]ChatResponse{
			{
				Message:   ChatMessage{Role: RoleAssistant, Content: "custom response 1"},
				TokensIn:  10,
				TokensOut: 10,
			},
		})
		r, err := scriptClient.CreateChatCompletion(ctx, ChatRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Message.Content != "custom response 1" {
			t.Errorf("expected custom response, got: %s", r.Message.Content)
		}
	})
}

func TestPromptManagerAndFormatting(t *testing.T) {
	mgr := NewPromptManager(nil)
	sysPrompt, id := mgr.GetSystemPrompt(context.Background())
	if !strings.Contains(sysPrompt, "autonomous multi-tenant incident triage agent") {
		t.Errorf("expected default system prompt, got: %s", sysPrompt)
	}
	if id.Valid {
		t.Errorf("expected zero ID for fallback prompt")
	}

	job := ingest.InvestigationJob{
		Service:    "checkout-worker",
		Title:      "RabbitMQ Queue Backlog Surge",
		Severity:   "sev1",
		Source:     "alertmanager",
		EnqueuedAt: time.Now().UTC(),
	}

	formatted := FormatIncidentContext(job)
	if !strings.Contains(formatted, "checkout-worker") || !strings.Contains(formatted, "RabbitMQ") {
		t.Errorf("expected formatted context to contain service and title, got: %s", formatted)
	}
}

func TestAgentRunnerInvestigationFlow(t *testing.T) {
	orgID := parseUUID("00000000-0000-0000-0000-000000000001")
	incID := parseUUID("40000000-0000-0000-0000-000000000001")

	llm := NewMockLLMClient()
	toolsReg := setupTestToolRegistry()
	broker := sse.NewBroker()

	cfg := DefaultAgentConfig()
	cfg.MaxSteps = 6

	runner := NewAgentRunner(nil, llm, toolsReg, broker, cfg, nil)

	job := ingest.InvestigationJob{
		OrgID:      orgID,
		IncidentID: incID,
		Title:      "RabbitMQ queue backlog on checkout-worker",
		Service:    "checkout-worker",
		Severity:   "sev1",
		Source:     "alertmanager",
		EnqueuedAt: time.Now().UTC(),
	}

	res, err := runner.RunInvestigation(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	if res.Status != "succeeded" {
		t.Errorf("expected succeeded status, got: %s", res.Status)
	}
	if len(res.Hypotheses) != 2 {
		t.Fatalf("expected 2 synthesized hypotheses, got %d", len(res.Hypotheses))
	}

	topHypothesis := res.Hypotheses[0]
	if topHypothesis.Rank != 1 {
		t.Errorf("expected top rank 1, got %d", topHypothesis.Rank)
	}
	if !strings.Contains(topHypothesis.RootCause, "prefetch count") {
		t.Errorf("expected root cause to mention prefetch count: %s", topHypothesis.RootCause)
	}
	if len(topHypothesis.Evidence) == 0 {
		t.Errorf("expected evidence citations")
	}
	if len(topHypothesis.Checklist) == 0 {
		t.Errorf("expected first-response checklist steps")
	}

	if res.TotalTokens <= 0 {
		t.Errorf("expected positive total tokens, got %d", res.TotalTokens)
	}
	if res.TotalCostUSD <= 0 {
		t.Errorf("expected positive total cost, got %f", res.TotalCostUSD)
	}
}

func TestAgentRunnerBudgetExhaustion(t *testing.T) {
	orgID := parseUUID("00000000-0000-0000-0000-000000000001")
	incID := parseUUID("40000000-0000-0000-0000-000000000001")

	job := ingest.InvestigationJob{
		OrgID:      orgID,
		IncidentID: incID,
		Title:      "High CPU Payment Gateway",
		Service:    "payment-api",
		Severity:   "sev2",
		Source:     "grafana",
		EnqueuedAt: time.Now().UTC(),
	}

	t.Run("Max steps cutoff", func(t *testing.T) {
		llm := NewMockLLMClient()
		toolsReg := setupTestToolRegistry()
		cfg := DefaultAgentConfig()
		cfg.MaxSteps = 1 // Cuts off after step 1 before conclusion

		runner := NewAgentRunner(nil, llm, toolsReg, nil, cfg, nil)
		res, _ := runner.RunInvestigation(context.Background(), job)

		if res.Status != "budget_exhausted" {
			t.Errorf("expected budget_exhausted status for 1-step cap, got: %s", res.Status)
		}
		if res.StepsCount != 1 {
			t.Errorf("expected exactly 1 step count, got %d", res.StepsCount)
		}
	})

	t.Run("Token budget ceiling", func(t *testing.T) {
		llm := NewMockLLMClient()
		toolsReg := setupTestToolRegistry()
		cfg := DefaultAgentConfig()
		cfg.MaxTokens = 400 // Turn 1 exceeds 400 tokens

		runner := NewAgentRunner(nil, llm, toolsReg, nil, cfg, nil)
		res, _ := runner.RunInvestigation(context.Background(), job)

		if res.Status != "budget_exhausted" {
			t.Errorf("expected budget_exhausted for low token ceiling, got: %s", res.Status)
		}
	})

	t.Run("Context timeout", func(t *testing.T) {
		llm := NewMockLLMClient()
		toolsReg := setupTestToolRegistry()
		cfg := DefaultAgentConfig()
		cfg.RunTimeout = 1 * time.Millisecond // Immediate timeout

		runner := NewAgentRunner(nil, llm, toolsReg, nil, cfg, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		time.Sleep(2 * time.Millisecond)
		res, err := runner.RunInvestigation(ctx, job)
		if err == nil && res.Status != "failed" {
			t.Errorf("expected failed status on expired context, got: %s", res.Status)
		}
	})
}

func TestOpenAILLMClient(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"id": "chatcmpl-123",
			"choices": [
				{
					"message": {
						"role": "assistant",
						"content": "Analysis completed."
					},
					"finish_reason": "stop"
				}
			],
			"usage": {
				"prompt_tokens": 120,
				"completion_tokens": 45,
				"total_tokens": 165
			}
		}`)
	}))
	defer mockServer.Close()

	client := NewOpenAILLMClient("test-api-key", mockServer.URL, "gpt-4o-mini", mockServer.Client())

	req := ChatRequest{
		Messages: []ChatMessage{
			{Role: RoleUser, Content: "Hello triage"},
		},
	}

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}

	if resp.Message.Content != "Analysis completed." {
		t.Errorf("expected content 'Analysis completed.', got: %s", resp.Message.Content)
	}
	if resp.TokensIn != 120 || resp.TokensOut != 45 {
		t.Errorf("unexpected tokens: in=%d, out=%d", resp.TokensIn, resp.TokensOut)
	}
	if resp.CostUSD <= 0 {
		t.Errorf("expected calculated cost > 0, got %f", resp.CostUSD)
	}
}

func TestAgentRunnerLiveDatabase(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://triage:triage_secret_dev@localhost:5433/triage?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	st, err := store.NewStore(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database test: postgres not reachable (%v)", err)
		return
	}
	defer st.Close()

	orgID := parseUUID("00000000-0000-0000-0000-000000000001")
	incID := parseUUID("40000000-0000-0000-0000-000000000001")

	llm := NewMockLLMClient()
	toolsReg := setupTestToolRegistry()
	broker := sse.NewBroker()
	cfg := DefaultAgentConfig()

	runner := NewAgentRunner(st, llm, toolsReg, broker, cfg, nil)

	job := ingest.InvestigationJob{
		OrgID:      orgID,
		IncidentID: incID,
		Title:      "Database Connection Saturation",
		Service:    "payment-api",
		Severity:   "sev1",
		Source:     "pagerduty",
		EnqueuedAt: time.Now().UTC(),
	}

	res, err := runner.RunInvestigation(ctx, job)
	if err != nil {
		t.Fatalf("unexpected live investigation error: %v", err)
	}

	if res.Status != "succeeded" {
		t.Fatalf("expected succeeded status, got: %s", res.Status)
	}

	// Verify database persistence of run, steps, and hypotheses under RLS
	err = st.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
		run, err := q.GetRunByID(ctx, db.GetRunByIDParams{
			OrgID: orgID,
			ID:    res.RunID,
		})
		if err != nil {
			return fmt.Errorf("failed to fetch run: %w", err)
		}
		if run.Status != "succeeded" {
			t.Errorf("expected DB run status succeeded, got %s", run.Status)
		}

		steps, err := q.ListRunSteps(ctx, db.ListRunStepsParams{
			OrgID: orgID,
			RunID: res.RunID,
		})
		if err != nil {
			return fmt.Errorf("failed to list run steps: %w", err)
		}
		if len(steps) == 0 {
			t.Errorf("expected run steps persisted in DB")
		}

		hypotheses, err := q.ListHypothesesForRun(ctx, db.ListHypothesesForRunParams{
			OrgID: orgID,
			RunID: res.RunID,
		})
		if err != nil {
			return fmt.Errorf("failed to list hypotheses: %w", err)
		}
		if len(hypotheses) == 0 {
			t.Errorf("expected hypotheses persisted in DB")
		}

		return nil
	})

	if err != nil {
		t.Fatalf("database verification error: %v", err)
	}
}
