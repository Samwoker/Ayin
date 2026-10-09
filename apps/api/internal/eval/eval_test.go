package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/seraph/triage/apps/api/internal/agent"
)

func TestLoaderAndValidation(t *testing.T) {
	// Test loading valid scenarios from repository root scenarios directory
	scenarios, err := LoadScenarios("../../../../scenarios")
	if err != nil {
		// Try relative to workspace
		scenarios, err = LoadScenarios("scenarios")
	}
	if err != nil {
		t.Fatalf("LoadScenarios failed: %v", err)
	}

	if len(scenarios) < 5 {
		t.Errorf("expected at least 5 benchmark scenarios, got %d", len(scenarios))
	}

	// Verify specific scenario fields
	var rabbitMQ *Scenario
	for _, sc := range scenarios {
		if sc.ID == "rabbitmq-backlog" {
			rabbitMQ = sc
			break
		}
	}

	if rabbitMQ == nil {
		t.Fatalf("expected to find scenario 'rabbitmq-backlog'")
	}
	if rabbitMQ.Service != "checkout-worker" {
		t.Errorf("expected service 'checkout-worker', got %s", rabbitMQ.Service)
	}
	if rabbitMQ.Severity != "sev1" {
		t.Errorf("expected severity 'sev1', got %s", rabbitMQ.Severity)
	}
	if len(rabbitMQ.Environment.Deploys) == 0 {
		t.Errorf("expected deploys in mock environment")
	}
	if len(rabbitMQ.Environment.Logs) == 0 {
		t.Errorf("expected logs in mock environment")
	}

	// Test single scenario by ID
	single, err := LoadScenarioByID("../../../../scenarios", "rabbitmq-backlog")
	if err != nil {
		single, err = LoadScenarioByID("scenarios", "rabbitmq-backlog")
	}
	if err != nil || single == nil || single.ID != "rabbitmq-backlog" {
		t.Errorf("LoadScenarioByID failed: %v", err)
	}
}

func TestMockToolsAndTracker(t *testing.T) {
	ctx := context.Background()
	sc, err := LoadScenarioByID("../../../../scenarios", "rabbitmq-backlog")
	if err != nil {
		sc, err = LoadScenarioByID("scenarios", "rabbitmq-backlog")
	}
	if err != nil {
		t.Fatalf("failed to load scenario: %v", err)
	}

	tracker := NewToolCallTracker()
	reg, err := BuildScenarioToolRegistry(sc, tracker)
	if err != nil {
		t.Fatalf("BuildScenarioToolRegistry failed: %v", err)
	}

	var orgID pgtype.UUID
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	// Execute recent_deploys
	resDeploys := reg.Execute(ctx, orgID, "recent_deploys", []byte(`{"service":"checkout-worker","limit":2}`))
	if !resDeploys.Success {
		t.Fatalf("recent_deploys tool failed: err=%s", resDeploys.Error)
	}

	// Execute query_metrics
	resMetrics := reg.Execute(ctx, orgID, "query_metrics", []byte(`{"query":"rabbitmq_queue_messages_ready"}`))
	if !resMetrics.Success {
		t.Fatalf("query_metrics tool failed: err=%s", resMetrics.Error)
	}

	// Execute log_search
	resLogs := reg.Execute(ctx, orgID, "log_search", []byte(`{"service":"checkout-worker","pattern":"deadlock"}`))
	if !resLogs.Success {
		t.Fatalf("log_search tool failed: err=%s", resLogs.Error)
	}

	// Execute service_topology
	resTopo := reg.Execute(ctx, orgID, "service_topology", []byte(`{"service":"checkout-worker"}`))
	if !resTopo.Success {
		t.Fatalf("service_topology tool failed: err=%s", resTopo.Error)
	}

	// Execute search_runbooks
	resRb := reg.Execute(ctx, orgID, "search_runbooks", []byte(`{"query":"RabbitMQ"}`))
	if !resRb.Success {
		t.Fatalf("search_runbooks tool failed: err=%s", resRb.Error)
	}

	// Verify tracker records
	records := tracker.Records()
	if len(records) != 5 {
		t.Errorf("expected 5 tracked calls, got %d", len(records))
	}
	calledTools := tracker.CalledTools()
	if len(calledTools) != 5 {
		t.Errorf("expected 5 distinct tools, got %d", len(calledTools))
	}
	if tracker.CountCalls("recent_deploys") != 1 {
		t.Errorf("expected 1 recent_deploys call")
	}
}

func TestScenarioAwareMockLLMAndScorer(t *testing.T) {
	ctx := context.Background()
	sc, err := LoadScenarioByID("../../../../scenarios", "rabbitmq-backlog")
	if err != nil {
		sc, err = LoadScenarioByID("scenarios", "rabbitmq-backlog")
	}
	if err != nil {
		t.Fatalf("failed to load scenario: %v", err)
	}

	llm := NewScenarioAwareMockLLM(sc)

	// Turn 1
	r1, err := llm.CreateChatCompletion(ctx, agent.ChatRequest{Model: "mock"})
	if err != nil || len(r1.Message.ToolCalls) == 0 {
		t.Fatalf("Turn 1 failed: %v", err)
	}

	// Turn 2
	r2, err := llm.CreateChatCompletion(ctx, agent.ChatRequest{Model: "mock"})
	if err != nil || len(r2.Message.ToolCalls) == 0 {
		t.Fatalf("Turn 2 failed: %v", err)
	}

	// Turn 3
	r3, err := llm.CreateChatCompletion(ctx, agent.ChatRequest{Model: "mock"})
	if err != nil || r3.FinishReason != "stop" {
		t.Fatalf("Turn 3 failed: %v", err)
	}

	// Score synthetic successful result
	tracker := NewToolCallTracker()
	tracker.Record(ToolCallRecord{ToolName: "recent_deploys"})
	tracker.Record(ToolCallRecord{ToolName: "query_metrics"})
	tracker.Record(ToolCallRecord{ToolName: "log_search"})

	res := &agent.InvestigationResult{
		Status:       "succeeded",
		StepsCount:   3,
		TotalTokens:  1850,
		TotalCostUSD: 0.00045,
		Duration:     150 * time.Millisecond,
		Hypotheses: []agent.HypothesisDTO{
			{
				Rank:       1,
				RootCause:  "Deploy v2.14.0 changed prefetch count to 1000 causing worker buffer saturation and unacknowledged message spike",
				Confidence: 0.92,
				Evidence: []string{
					"Deploy v2.14.0 modified prefetch count",
					"rabbitmq_queue_messages_ready surged to 58,200",
					"Logs confirm transaction lock deadlock",
				},
				Checklist: []string{
					"Execute rollback_deploy to checkout-worker v2.13.9",
					"Verify rabbitmq_queue_messages_ready decreases",
				},
			},
		},
	}

	scored := ScoreScenario(sc, res, tracker)
	if !scored.Passed {
		t.Fatalf("expected scenario to pass, failure reasons: %v", scored.FailureReasons)
	}
	if scored.AccuracyScore < 80.0 {
		t.Errorf("expected accuracy score >= 80, got %.1f", scored.AccuracyScore)
	}
	if scored.EvidenceScore < 80.0 {
		t.Errorf("expected evidence score >= 80, got %.1f", scored.EvidenceScore)
	}
	if scored.OverallScore < 85.0 {
		t.Errorf("expected overall score >= 85, got %.1f", scored.OverallScore)
	}
}

func TestScorerPenaltiesAndFailures(t *testing.T) {
	sc, err := LoadScenarioByID("../../../../scenarios", "rabbitmq-backlog")
	if err != nil {
		sc, err = LoadScenarioByID("scenarios", "rabbitmq-backlog")
	}
	if err != nil {
		t.Fatalf("failed to load scenario: %v", err)
	}

	// 1. Result with nil / zero hypotheses
	resNil := &agent.InvestigationResult{Status: "failed"}
	scoredNil := ScoreScenario(sc, resNil, nil)
	if scoredNil.Passed {
		t.Errorf("expected failed scenario for nil hypotheses")
	}

	// 2. Result with forbidden hallucinated cause
	tracker := NewToolCallTracker()
	resHalluc := &agent.InvestigationResult{
		Status:     "succeeded",
		StepsCount: 3,
		Hypotheses: []agent.HypothesisDTO{
			{
				Rank:       1,
				RootCause:  "prefetch issue caused by DDoS attack from malicious bots",
				Confidence: 0.95,
				Evidence:   []string{"58,200 ready messages"},
				Checklist:  []string{"rollback_deploy"},
			},
		},
	}
	scoredHalluc := ScoreScenario(sc, resHalluc, tracker)
	if scoredHalluc.AccuracyScore >= 80 {
		t.Errorf("expected accuracy score penalty for forbidden cause 'DDoS attack', got %.1f", scoredHalluc.AccuracyScore)
	}

	// 3. Hallucinating an uncalled tool citation
	resUncalled := &agent.InvestigationResult{
		Status:     "succeeded",
		StepsCount: 3,
		Hypotheses: []agent.HypothesisDTO{
			{
				Rank:       1,
				RootCause:  "prefetch v2.14.0 buffer saturation",
				Confidence: 0.90,
				Evidence: []string{
					"service_topology shows upstream queue failure", // service_topology was NOT called
				},
				Checklist: []string{"rollback_deploy"},
			},
		},
	}
	scoredUncalled := ScoreScenario(sc, resUncalled, tracker)
	if scoredUncalled.HallucinationScore == 0 {
		t.Errorf("expected hallucination penalty for citing uncalled tool service_topology")
	}
}

func TestRunEvaluationFullSuite(t *testing.T) {
	ctx := context.Background()

	scenariosDir := "../../../../scenarios"
	if _, err := os.Stat(scenariosDir); err != nil {
		scenariosDir = "scenarios"
	}

	cfg := EvalConfig{
		ScenariosDir: scenariosDir,
		Provider:     "mock",
	}

	report, err := RunEvaluation(ctx, cfg)
	if err != nil {
		t.Fatalf("RunEvaluation failed: %v", err)
	}

	if report.TotalScenarios < 5 {
		t.Errorf("expected at least 5 scenarios evaluated, got %d", report.TotalScenarios)
	}
	if report.PassedScenarios != report.TotalScenarios {
		t.Errorf("expected all %d scenarios to pass in mock evaluation, got %d passed",
			report.TotalScenarios, report.PassedScenarios)
	}
	if report.PassRate < 99.0 {
		t.Errorf("expected 100%% pass rate, got %.1f%%", report.PassRate)
	}
	if report.MeanOverallScore < 85.0 {
		t.Errorf("expected mean score >= 85, got %.1f", report.MeanOverallScore)
	}

	// Test Text report generation
	text := RenderTextReport(report)
	if len(text) == 0 {
		t.Errorf("RenderTextReport returned empty string")
	}

	// Test Markdown report generation
	md := RenderMarkdownReport(report)
	if len(md) == 0 {
		t.Errorf("RenderMarkdownReport returned empty string")
	}

	// Test JSON save & load
	tmpFile := filepath.Join(t.TempDir(), "test_report.json")
	if err := SaveReportJSON(report, tmpFile); err != nil {
		t.Fatalf("SaveReportJSON failed: %v", err)
	}

	loaded, err := LoadReportJSON(tmpFile)
	if err != nil {
		t.Fatalf("LoadReportJSON failed: %v", err)
	}
	if loaded.RunID != report.RunID {
		t.Errorf("loaded RunID mismatch: %s vs %s", loaded.RunID, report.RunID)
	}

	// Test comparison against self (should report 0 regressions)
	cmp := CompareWithBaseline(report, loaded)
	if cmp == nil {
		t.Fatalf("expected non-nil comparison report")
	}
	if cmp.Delta.RegressionsCount != 0 {
		t.Errorf("expected 0 regressions when comparing identical runs, got %d", cmp.Delta.RegressionsCount)
	}
}
