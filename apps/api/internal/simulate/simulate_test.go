package simulate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seraph/triage/apps/api/internal/eval"
)

func TestGenerateSimulationEvents(t *testing.T) {
	sc, err := eval.LoadScenarioByID("../../../../scenarios", "rabbitmq-backlog")
	if err != nil {
		sc, err = eval.LoadScenarioByID("scenarios", "rabbitmq-backlog")
	}
	if err != nil {
		t.Fatalf("failed to load scenario: %v", err)
	}

	// 1. Test Chaos None (0)
	events0 := GenerateSimulationEvents(sc, ChaosLevelNone, 42)
	if len(events0) < 3 {
		t.Errorf("expected at least 3 events for chaos 0, got %d", len(events0))
	}
	for _, e := range events0 {
		if e.IsNoise || e.IsAdversarial {
			t.Errorf("expected no noise/adversarial events in chaos 0")
		}
	}

	// 2. Test Chaos Noise (1)
	events1 := GenerateSimulationEvents(sc, ChaosLevelNoise, 42)
	if len(events1) <= len(events0) {
		t.Errorf("expected more events in chaos 1 than chaos 0 (%d vs %d)", len(events1), len(events0))
	}
	noiseCount := 0
	for _, e := range events1 {
		if e.IsNoise {
			noiseCount++
		}
	}
	if noiseCount == 0 {
		t.Errorf("expected noise events in chaos level 1")
	}

	// 3. Test Chaos Adversarial (3)
	events3 := GenerateSimulationEvents(sc, ChaosLevelAdversarial, 42)
	advCount := 0
	for _, e := range events3 {
		if e.IsAdversarial {
			advCount++
		}
	}
	if advCount < 2 {
		t.Errorf("expected at least 2 adversarial distractor events in chaos level 3, got %d", advCount)
	}

	// 4. Verify chronological sorting
	for i := 1; i < len(events3); i++ {
		if events3[i].Timestamp.Before(events3[i-1].Timestamp) {
			t.Errorf("events not sorted chronologically at index %d", i)
		}
	}
}

func TestDispatcherDryRun(t *testing.T) {
	ctx := context.Background()
	sc, err := eval.LoadScenarioByID("../../../../scenarios", "rabbitmq-backlog")
	if err != nil {
		sc, err = eval.LoadScenarioByID("scenarios", "rabbitmq-backlog")
	}
	if err != nil {
		t.Fatalf("failed to load scenario: %v", err)
	}

	cfg := SimulationConfig{
		ScenarioName: "rabbitmq-backlog",
		ChaosLevel:   ChaosLevelJitter,
		Seed:         1234,
		DryRun:       true,
	}

	d := NewDispatcher(nil)
	res, err := d.Run(ctx, cfg, sc)
	if err != nil {
		t.Fatalf("dispatcher Run failed: %v", err)
	}

	if res.EventsCount == 0 {
		t.Errorf("expected positive events count")
	}
	if res.AlertDispatched {
		t.Errorf("expected alert not dispatched in dry run mode")
	}
	if len(res.OutputLogs) == 0 {
		t.Errorf("expected output logs")
	}

	report := RenderSimulationReport(res)
	if len(report) == 0 {
		t.Errorf("RenderSimulationReport returned empty string")
	}
}

func TestDispatcherLiveServer(t *testing.T) {
	ctx := context.Background()
	sc, err := eval.LoadScenarioByID("../../../../scenarios", "rabbitmq-backlog")
	if err != nil {
		sc, err = eval.LoadScenarioByID("scenarios", "rabbitmq-backlog")
	}
	if err != nil {
		t.Fatalf("failed to load scenario: %v", err)
	}

	// Start test HTTP server simulating API webhook endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/ingest/alerts/generic" {
			http.NotFound(w, r)
			return
		}
		sig := r.Header.Get("X-Signature-256")
		if sig == "" {
			t.Errorf("missing X-Signature-256 header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"queued","incident_id":"inc-test-sim-001"}`))
	}))
	defer server.Close()

	cfg := SimulationConfig{
		ScenarioName:  "rabbitmq-backlog",
		ChaosLevel:    ChaosLevelNoise,
		Seed:          999,
		TargetURL:     server.URL,
		WebhookSecret: "test-secret-123",
		DryRun:        false,
	}

	d := NewDispatcher(server.Client())
	res, err := d.Run(ctx, cfg, sc)
	if err != nil {
		t.Fatalf("dispatcher live run failed: %v", err)
	}

	if !res.AlertDispatched {
		t.Errorf("expected alert to be dispatched")
	}
	if res.ResponseCode != http.StatusAccepted {
		t.Errorf("expected HTTP 202, got %d", res.ResponseCode)
	}
	if res.IncidentID != "inc-test-sim-001" {
		t.Errorf("expected incident_id 'inc-test-sim-001', got %s", res.IncidentID)
	}

	report := RenderSimulationReport(res)
	if len(report) == 0 {
		t.Errorf("RenderSimulationReport returned empty string")
	}
}
