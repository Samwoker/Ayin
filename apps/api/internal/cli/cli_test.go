package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientIncidents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "test-api-key" {
			t.Errorf("expected X-API-Key 'test-api-key', got %s", apiKey)
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/incidents":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]IncidentDTO{
					{
						ID:        "inc-1",
						Title:     "RabbitMQ Backlog",
						Service:   "checkout-worker",
						Severity:  "sev1",
						Status:    "open",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				})
			} else if r.Method == http.MethodPost {
				_ = json.NewEncoder(w).Encode(IncidentDTO{
					ID:       "inc-new",
					Title:    "Manual Incident",
					Service:  "payment-api",
					Severity: "sev2",
					Status:   "open",
				})
			}
		case "/api/v1/incidents/inc-1":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(IncidentDTO{
					ID:       "inc-1",
					Title:    "RabbitMQ Backlog",
					Service:  "checkout-worker",
					Severity: "sev1",
					Status:   "open",
				})
			} else if r.Method == http.MethodPatch {
				w.WriteHeader(http.StatusOK)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		APIURL: server.URL,
		APIKey: "test-api-key",
		OrgID:  "00000000-0000-0000-0000-000000000001",
	}, server.Client())

	ctx := context.Background()

	// List
	list, err := client.ListIncidents(ctx, "open", "checkout-worker", "sev1", 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListIncidents failed: %v", err)
	}

	// Get
	inc, err := client.GetIncident(ctx, "inc-1")
	if err != nil || inc.ID != "inc-1" {
		t.Fatalf("GetIncident failed: %v", err)
	}

	// Create
	created, err := client.CreateIncident(ctx, "Manual Incident", "payment-api", "sev2", "desc")
	if err != nil || created.ID != "inc-new" {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	// Resolve
	if err := client.ResolveIncident(ctx, "inc-1"); err != nil {
		t.Fatalf("ResolveIncident failed: %v", err)
	}
}

func TestClientRunsAndRemediations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/incidents/inc-1/runs":
			_ = json.NewEncoder(w).Encode([]RunDTO{
				{
					ID:           "run-1",
					IncidentID:   "inc-1",
					Status:       "succeeded",
					TotalTokens:  1250,
					TotalCostUSD: 0.0012,
					StepsCount:   3,
					CreatedAt:    time.Now(),
				},
			})
		case "/api/v1/runs/run-1":
			_ = json.NewEncoder(w).Encode(RunDTO{
				ID:         "run-1",
				IncidentID: "inc-1",
				Status:     "succeeded",
			})
		case "/api/v1/incidents/inc-1/remediations":
			_ = json.NewEncoder(w).Encode([]RemediationDTO{
				{
					ID:        "rem-1",
					Action:    "rollback_deploy",
					RiskTier:  "high",
					Status:    "proposed",
					ExpiresAt: time.Now().Add(15 * time.Minute),
				},
			})
		case "/api/v1/remediations/rem-1":
			_ = json.NewEncoder(w).Encode(RemediationDTO{
				ID:     "rem-1",
				Action: "rollback_deploy",
				Status: "proposed",
			})
		case "/api/v1/remediations/rem-1/approve",
			"/api/v1/remediations/rem-1/reject",
			"/api/v1/remediations/rem-1/rollback":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(Config{APIURL: server.URL}, server.Client())
	ctx := context.Background()

	// Runs
	runs, err := client.ListRuns(ctx, "inc-1")
	if err != nil || len(runs) != 1 {
		t.Fatalf("ListRuns failed: %v", err)
	}
	run, err := client.GetRun(ctx, "run-1")
	if err != nil || run.ID != "run-1" {
		t.Fatalf("GetRun failed: %v", err)
	}

	// Remediations
	rems, err := client.ListRemediations(ctx, "inc-1")
	if err != nil || len(rems) != 1 {
		t.Fatalf("ListRemediations failed: %v", err)
	}
	rem, err := client.GetRemediation(ctx, "rem-1")
	if err != nil || rem.ID != "rem-1" {
		t.Fatalf("GetRemediation failed: %v", err)
	}

	if err := client.ApproveRemediation(ctx, "rem-1", "test"); err != nil {
		t.Fatalf("ApproveRemediation failed: %v", err)
	}
	if err := client.RejectRemediation(ctx, "rem-1", "test"); err != nil {
		t.Fatalf("RejectRemediation failed: %v", err)
	}
	if err := client.RollbackRemediation(ctx, "rem-1", "test"); err != nil {
		t.Fatalf("RollbackRemediation failed: %v", err)
	}
}

func TestClientToolsAndStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/tools" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]ToolDTO{
				{Name: "log_search", Description: "Searches logs"},
			})
			return
		}
		if r.URL.Path == "/api/v1/tools/log_search/execute" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tool":    "log_search",
				"success": true,
				"data":    map[string]any{"count": 2},
			})
			return
		}
		if r.URL.Path == "/api/v1/incidents/inc-1/stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("expected flusher")
			}
			_, _ = w.Write([]byte("event: run_started\ndata: {\"service\":\"checkout-worker\"}\n\n"))
			flusher.Flush()
			_, _ = w.Write([]byte("event: run_completed\ndata: {\"status\":\"succeeded\"}\n\n"))
			flusher.Flush()
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(Config{APIURL: server.URL}, server.Client())
	ctx := context.Background()

	// Tools
	tools, err := client.ListTools(ctx)
	if err != nil || len(tools) != 1 {
		t.Fatalf("ListTools failed: %v", err)
	}
	execRes, err := client.ExecuteTool(ctx, "log_search", []byte(`{"service":"checkout-worker"}`))
	if err != nil || execRes["success"] != true {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Stream
	eventsReceived := 0
	streamCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	_ = client.StreamIncident(streamCtx, "inc-1", func(eventType, data string) {
		eventsReceived++
	})
	if eventsReceived < 2 {
		t.Errorf("expected at least 2 streamed events, got %d", eventsReceived)
	}
}

func TestTableRenderers(t *testing.T) {
	// Incidents Table
	incTable := RenderIncidentsTable([]IncidentDTO{
		{ID: "inc-1", Title: "Backlog", Service: "worker", Severity: "sev1", Status: "open", CreatedAt: time.Now()},
	})
	if len(incTable) == 0 {
		t.Errorf("expected non-empty incidents table")
	}

	// Incident Detail
	incDetail := RenderIncidentDetail(&IncidentDTO{
		ID: "inc-1", Title: "Backlog", Service: "worker", Severity: "sev1", Status: "open", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	if len(incDetail) == 0 {
		t.Errorf("expected non-empty incident detail")
	}

	// Runs Table
	runsTable := RenderRunsTable([]RunDTO{
		{ID: "run-1", Status: "succeeded", StepsCount: 3, TotalTokens: 100, CreatedAt: time.Now()},
	})
	if len(runsTable) == 0 {
		t.Errorf("expected non-empty runs table")
	}

	// Remediations Table & Detail
	remsTable := RenderRemediationsTable([]RemediationDTO{
		{ID: "rem-1", Action: "rollback", RiskTier: "high", Status: "proposed", Target: "worker", ExpiresAt: time.Now().Add(10 * time.Minute)},
	})
	if len(remsTable) == 0 {
		t.Errorf("expected non-empty remediations table")
	}

	remDetail := RenderRemediationDetail(&RemediationDTO{
		ID: "rem-1", Action: "rollback", RiskTier: "high", Status: "proposed", Target: "worker", ExpiresAt: time.Now().Add(10 * time.Minute)},
	)
	if len(remDetail) == 0 {
		t.Errorf("expected non-empty remediation detail")
	}

	// Tools Table
	toolsTable := RenderToolsTable([]ToolDTO{
		{Name: "log_search", Description: "searches logs"},
	})
	if len(toolsTable) == 0 {
		t.Errorf("expected non-empty tools table")
	}
}

func TestCobraRootCommands(t *testing.T) {
	root := NewRootCmd()
	if root == nil {
		t.Fatalf("expected non-nil root command")
	}

	// Verify subcommands exist
	expectedCmds := []string{"version", "whoami", "incidents", "runs", "remediations", "tools"}
	for _, cmdName := range expectedCmds {
		cmd, _, err := root.Find([]string{cmdName})
		if err != nil || cmd == nil || cmd.Name() != cmdName {
			t.Errorf("expected subcommand %q to exist", cmdName)
		}
	}
}

func TestCobraExecutionWithServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/incidents":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]IncidentDTO{
					{ID: "inc-1", Title: "Test", Service: "svc", Severity: "sev2", Status: "open", CreatedAt: time.Now()},
				})
			} else if r.Method == http.MethodPost {
				_ = json.NewEncoder(w).Encode(IncidentDTO{ID: "inc-new", Title: "Created", Service: "svc", Severity: "sev2"})
			}
		case "/api/v1/incidents/inc-1":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(IncidentDTO{ID: "inc-1", Title: "Test", Service: "svc", Severity: "sev2", Status: "open"})
			} else if r.Method == http.MethodPatch {
				w.WriteHeader(http.StatusOK)
			}
		case "/api/v1/incidents/inc-1/runs":
			_ = json.NewEncoder(w).Encode([]RunDTO{
				{ID: "run-1", IncidentID: "inc-1", Status: "succeeded", StepsCount: 2, TotalTokens: 100, CreatedAt: time.Now()},
			})
		case "/api/v1/runs/run-1":
			_ = json.NewEncoder(w).Encode(RunDTO{ID: "run-1", IncidentID: "inc-1", Status: "succeeded"})
		case "/api/v1/incidents/inc-1/remediations":
			_ = json.NewEncoder(w).Encode([]RemediationDTO{
				{ID: "rem-1", Action: "restart", RiskTier: "low", Status: "proposed", ExpiresAt: time.Now().Add(5 * time.Minute)},
			})
		case "/api/v1/remediations/rem-1":
			_ = json.NewEncoder(w).Encode(RemediationDTO{ID: "rem-1", Action: "restart", Status: "proposed"})
		case "/api/v1/remediations/rem-1/approve",
			"/api/v1/remediations/rem-1/reject",
			"/api/v1/remediations/rem-1/rollback":
			w.WriteHeader(http.StatusOK)
		case "/api/v1/tools":
			_ = json.NewEncoder(w).Encode([]ToolDTO{
				{Name: "log_search", Description: "Log search tool"},
			})
		case "/api/v1/tools/log_search/execute":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "tool": "log_search"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	commandsToTest := [][]string{
		{"version"},
		{"version", "--json"},
		{"whoami", "--api-url=" + server.URL},
		{"incidents", "list", "--api-url=" + server.URL},
		{"incidents", "list", "--api-url=" + server.URL, "--json"},
		{"incidents", "get", "inc-1", "--api-url=" + server.URL},
		{"incidents", "create", "--title=TestInc", "--service=svc", "--api-url=" + server.URL},
		{"incidents", "resolve", "inc-1", "--api-url=" + server.URL},
		{"runs", "list", "inc-1", "--api-url=" + server.URL},
		{"runs", "get", "run-1", "--api-url=" + server.URL},
		{"remediations", "list", "inc-1", "--api-url=" + server.URL},
		{"remediations", "get", "rem-1", "--api-url=" + server.URL},
		{"remediations", "approve", "rem-1", "--yes", "--api-url=" + server.URL},
		{"remediations", "reject", "rem-1", "--api-url=" + server.URL},
		{"remediations", "rollback", "rem-1", "--api-url=" + server.URL},
		{"tools", "list", "--api-url=" + server.URL},
		{"tools", "exec", "log_search", "--params={\"service\":\"svc\"}", "--api-url=" + server.URL},
	}

	for _, args := range commandsToTest {
		cmd := NewRootCmd()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Errorf("command failed for args %v: %v", args, err)
		}
	}
}
