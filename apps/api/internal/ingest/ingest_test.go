package ingest

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"github.com/seraph/triage/apps/api/internal/server/middleware"
	"github.com/seraph/triage/apps/api/internal/store"
)

func withURLParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		rctx = chi.NewRouteContext()
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
	rctx.URLParams.Add(key, val)
	return r
}

func TestHMACVerification(t *testing.T) {
	secret := "secret-webhook-key-32bytes-123456"
	body := []byte(`{"alert":"rabbitmq_depth_high","service":"checkout"}`)

	t.Run("Valid raw body signature", func(t *testing.T) {
		sig := ComputeWebhookSignature(secret, body, "")
		err := VerifyWebhookSignature(secret, body, "sha256="+sig, "", 5*time.Minute)
		if err != nil {
			t.Fatalf("expected valid signature, got error: %v", err)
		}
	})

	t.Run("Valid timestamped signature", func(t *testing.T) {
		now := time.Now().UTC()
		tsStr := fmt.Sprintf("%d", now.Unix())
		sig := ComputeWebhookSignature(secret, body, tsStr)
		err := VerifyWebhookSignature(secret, body, sig, tsStr, 5*time.Minute)
		if err != nil {
			t.Fatalf("expected valid timestamped signature, got error: %v", err)
		}
	})

	t.Run("Mismatched signature rejected", func(t *testing.T) {
		err := VerifyWebhookSignature(secret, body, "sha256=baddeadbeef0011223344", "", 5*time.Minute)
		if err != ErrInvalidSignature {
			t.Fatalf("expected ErrInvalidSignature, got: %v", err)
		}
	})

	t.Run("Missing signature rejected", func(t *testing.T) {
		err := VerifyWebhookSignature(secret, body, "", "", 5*time.Minute)
		if err != ErrMissingSignature {
			t.Fatalf("expected ErrMissingSignature, got: %v", err)
		}
	})

	t.Run("Expired timestamp rejected", func(t *testing.T) {
		oldTime := time.Now().Add(-10 * time.Minute)
		tsStr := fmt.Sprintf("%d", oldTime.Unix())
		sig := ComputeWebhookSignature(secret, body, tsStr)
		err := VerifyWebhookSignature(secret, body, sig, tsStr, 5*time.Minute)
		if err != ErrTimestampExpired {
			t.Fatalf("expected ErrTimestampExpired, got: %v", err)
		}
	})
}

func TestAlertmanagerParser(t *testing.T) {
	parser := NewAlertmanagerParser()

	payload := `{
		"version": "4",
		"status": "firing",
		"commonLabels": {
			"service": "checkout-worker",
			"severity": "critical"
		},
		"commonAnnotations": {
			"summary": "RabbitMQ Queue Backlog"
		},
		"alerts": [
			{
				"status": "firing",
				"labels": {
					"alertname": "RabbitMQDepthHigh",
					"service": "checkout-worker",
					"instance": "pod-1"
				},
				"annotations": {
					"summary": "Queue depth over 50k on pod-1"
				},
				"startsAt": "2026-10-09T08:00:00Z",
				"fingerprint": "am-fp-001"
			}
		]
	}`

	alerts, err := parser.Parse(context.Background(), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	a := alerts[0]
	if a.Service != "checkout-worker" {
		t.Errorf("expected service checkout-worker, got %s", a.Service)
	}
	if a.Severity != "sev1" {
		t.Errorf("expected severity sev1, got %s", a.Severity)
	}
	if a.Fingerprint != "am-fp-001" {
		t.Errorf("expected fingerprint am-fp-001, got %s", a.Fingerprint)
	}
	if a.Status != "firing" {
		t.Errorf("expected status firing, got %s", a.Status)
	}
}

func TestGrafanaParser(t *testing.T) {
	parser := NewGrafanaParser()

	payload := `{
		"title": "[FIRING:1] HighMemory payment-api",
		"state": "alerting",
		"alerts": [
			{
				"status": "firing",
				"labels": {
					"alertname": "HighMemory",
					"service": "payment-api",
					"severity": "high"
				},
				"annotations": {
					"summary": "Payment API memory leak detected"
				},
				"startsAt": "2026-10-09T08:00:00Z",
				"fingerprint": "grafana-fp-002"
			}
		]
	}`

	alerts, err := parser.Parse(context.Background(), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	a := alerts[0]
	if a.Service != "payment-api" {
		t.Errorf("expected service payment-api, got %s", a.Service)
	}
	if a.Severity != "sev2" {
		t.Errorf("expected severity sev2, got %s", a.Severity)
	}
	if a.Fingerprint != "grafana-fp-002" {
		t.Errorf("expected fingerprint grafana-fp-002, got %s", a.Fingerprint)
	}
}

func TestPagerDutyParser(t *testing.T) {
	parser := NewPagerDutyParser()

	payload := `{
		"event": {
			"id": "01ABCDEF",
			"event_type": "incident.triggered",
			"occurred_at": "2026-10-09T08:00:00Z",
			"data": {
				"id": "PD999",
				"title": "Payment Gateway Timeout",
				"urgency": "high",
				"status": "triggered",
				"service": {
					"id": "SERV1",
					"name": "payment-gateway"
				}
			}
		}
	}`

	alerts, err := parser.Parse(context.Background(), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	a := alerts[0]
	if a.Service != "payment-gateway" {
		t.Errorf("expected service payment-gateway, got %s", a.Service)
	}
	if a.Severity != "sev1" {
		t.Errorf("expected severity sev1 for high urgency, got %s", a.Severity)
	}
	if a.Fingerprint != "pd-PD999" {
		t.Errorf("expected fingerprint pd-PD999, got %s", a.Fingerprint)
	}
}

func TestGenericParser(t *testing.T) {
	parser := NewGenericParser()

	payload := `[
		{
			"title": "Database deadlocks spike",
			"service": "order-service",
			"severity": "sev2",
			"status": "firing",
			"fingerprint": "gen-001"
		},
		{
			"title": "Cache miss storm",
			"service": "cache-layer",
			"severity": "sev3",
			"status": "firing"
		}
	]`

	alerts, err := parser.Parse(context.Background(), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(alerts))
	}

	if alerts[0].Fingerprint != "gen-001" {
		t.Errorf("expected gen-001 fingerprint, got %s", alerts[0].Fingerprint)
	}
	if alerts[1].Service != "cache-layer" {
		t.Errorf("expected cache-layer service, got %s", alerts[1].Service)
	}
}

func TestInvestigationQueue(t *testing.T) {
	queue := NewMemoryQueue(10)

	var orgID, incID pgtype.UUID
	_ = orgID.Scan("10000000-0000-0000-0000-000000000001")
	_ = incID.Scan("20000000-0000-0000-0000-000000000001")

	job := InvestigationJob{
		OrgID:      orgID,
		IncidentID: incID,
		Title:      "RabbitMQ Queue Depth Spike",
		Service:    "checkout-worker",
		Severity:   "sev1",
		Source:     "alertmanager",
		EnqueuedAt: time.Now().UTC(),
	}

	ctx := context.Background()
	err := queue.Enqueue(ctx, job)
	if err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	dequeued, err := queue.Dequeue(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to dequeue job: %v", err)
	}

	if dequeued.Title != job.Title {
		t.Errorf("expected title %s, got %s", job.Title, dequeued.Title)
	}
	if dequeued.Service != job.Service {
		t.Errorf("expected service %s, got %s", job.Service, dequeued.Service)
	}
}

func TestFileLogAdapter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "triage-logs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFilePath := filepath.Join(tmpDir, "checkout-worker.log")
	logContent := `{"time":"2026-10-09T08:00:00Z","level":"info","service":"checkout-worker","msg":"worker started"}
{"time":"2026-10-09T08:05:00Z","level":"error","service":"checkout-worker","msg":"payment gateway connection timeout"}
{"time":"2026-10-09T08:10:00Z","level":"warn","service":"checkout-worker","msg":"retrying payment request"}
`
	if err := os.WriteFile(logFilePath, []byte(logContent), 0644); err != nil {
		t.Fatalf("failed to write test log file: %v", err)
	}

	adapter := NewFileLogAdapter(tmpDir)

	t.Run("Query by pattern", func(t *testing.T) {
		entries, err := adapter.QueryLogs(context.Background(), LogQuery{
			Service: "checkout-worker",
			Pattern: "timeout",
			Limit:   10,
		})
		if err != nil {
			t.Fatalf("unexpected error querying logs: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry matching 'timeout', got %d", len(entries))
		}
		if entries[0].Level != "error" {
			t.Errorf("expected level error, got %s", entries[0].Level)
		}
	})

	t.Run("Query by time window", func(t *testing.T) {
		start, _ := time.Parse(time.RFC3339, "2026-10-09T08:04:00Z")
		end, _ := time.Parse(time.RFC3339, "2026-10-09T08:06:00Z")

		entries, err := adapter.QueryLogs(context.Background(), LogQuery{
			Service:   "checkout-worker",
			StartTime: start,
			EndTime:   end,
			Limit:     10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry in window, got %d", len(entries))
		}
	})
}

func TestLokiLogAdapter(t *testing.T) {
	// Mock Loki HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/query_range" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"status": "success",
			"data": {
				"resultType": "streams",
				"result": [
					{
						"stream": { "service": "payment-api", "env": "prod" },
						"values": [
							[ "1700000000000000000", "{\"level\":\"error\",\"msg\":\"connection refused to database\"}" ]
						]
					}
				]
			}
		}`)
	}))
	defer mockServer.Close()

	adapter := NewLokiLogAdapter(mockServer.URL, mockServer.Client(), "test-tenant")

	entries, err := adapter.QueryLogs(context.Background(), LogQuery{
		Service: "payment-api",
		Pattern: "refused",
		Limit:   5,
	})
	if err != nil {
		t.Fatalf("unexpected Loki query error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 Loki log entry, got %d", len(entries))
	}
	if entries[0].Service != "payment-api" {
		t.Errorf("expected payment-api service, got %s", entries[0].Service)
	}
}

func TestIngestHandlerWebhookFlow(t *testing.T) {
	secret := "dev_secret"
	queue := NewMemoryQueue(10)
	grouper := NewAlertGrouper(nil, queue, 30*time.Minute)
	handler := NewIngestHandler(nil, nil, grouper)

	payload := []byte(`{
		"title": "RabbitMQ Queue Spiking",
		"service": "checkout-worker",
		"severity": "sev1",
		"status": "firing",
		"fingerprint": "fp-checkout-rabbitmq-1"
	}`)

	sig := ComputeWebhookSignature(secret, payload, "")

	var orgID pgtype.UUID
	_ = orgID.Scan("10000000-0000-0000-0000-000000000001")

	t.Run("Valid webhook ingestion", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/webhooks/generic", bytes.NewReader(payload))
		req.Header.Set("X-Signature-256", sig)

		// Set tenant context for org resolution in handler
		tc := middleware.TenantContext{
			OrgID: orgID,
		}
		req = req.WithContext(middleware.SetTenantContext(req.Context(), tc))

		rctx := withURLParam(req, "source", "generic")

		rec := httptest.NewRecorder()
		handler.HandleWebhook(rec, rctx)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Unsupported source returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/webhooks/invalid-source", bytes.NewReader(payload))
		req = withURLParam(req, "source", "invalid-source")
		rec := httptest.NewRecorder()

		handler.HandleWebhook(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})
}

func TestAlertGrouperWithStore(t *testing.T) {
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

	var orgID pgtype.UUID
	if err := orgID.Scan("00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("failed to scan org ID: %v", err)
	}

	queue := NewMemoryQueue(20)
	grouper := NewAlertGrouper(st, queue, 30*time.Minute)

	fp := fmt.Sprintf("test-fp-%d", time.Now().UnixNano())
	service := fmt.Sprintf("service-%d", time.Now().UnixNano())

	// 1. First alert creates new incident & enqueues job
	alert1 := NormalizedAlert{
		Title:       "High CPU Usage",
		Service:     service,
		Severity:    "sev1",
		Status:      "firing",
		Fingerprint: fp,
		Source:      "alertmanager",
		Labels:      map[string]string{"host": "node-1"},
	}

	inc1, isNew, err := grouper.ProcessAlert(ctx, orgID, alert1)
	if err != nil {
		t.Fatalf("failed to process alert1: %v", err)
	}
	if !isNew {
		t.Errorf("expected isNew=true for initial alert")
	}
	if inc1 == nil || !inc1.ID.Valid {
		t.Fatalf("expected valid incident created")
	}

	// Verify job enqueued
	job1, err := queue.Dequeue(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("expected job enqueued: %v", err)
	}
	if job1.IncidentID != inc1.ID {
		t.Errorf("expected job incident ID %v, got %v", inc1.ID, job1.IncidentID)
	}

	// 2. Duplicate alert with identical fingerprint should deduplicate
	alert2 := NormalizedAlert{
		Title:       "High CPU Usage Repeat",
		Service:     service,
		Severity:    "sev1",
		Status:      "firing",
		Fingerprint: fp,
		Source:      "alertmanager",
	}

	inc2, isNew2, err := grouper.ProcessAlert(ctx, orgID, alert2)
	if err != nil {
		t.Fatalf("failed to process alert2: %v", err)
	}
	if isNew2 {
		t.Errorf("expected isNew=false for duplicate fingerprint")
	}
	if inc2.ID != inc1.ID {
		t.Errorf("expected incident ID %v, got %v", inc1.ID, inc2.ID)
	}

	// 3. Different fingerprint but same service should group into open incident
	fp2 := fmt.Sprintf("%s-diff", fp)
	alert3 := NormalizedAlert{
		Title:       "Memory Leak on same service",
		Service:     service,
		Severity:    "sev2",
		Status:      "firing",
		Fingerprint: fp2,
		Source:      "grafana",
	}

	inc3, isNew3, err := grouper.ProcessAlert(ctx, orgID, alert3)
	if err != nil {
		t.Fatalf("failed to process alert3: %v", err)
	}
	if isNew3 {
		t.Errorf("expected isNew=false for same service within window")
	}
	if inc3.ID != inc1.ID {
		t.Errorf("expected incident ID %v, got %v", inc1.ID, inc3.ID)
	}

	// 4. Different service opens a brand new incident
	diffService := fmt.Sprintf("other-%s", service)
	alert4 := NormalizedAlert{
		Title:       "Disk Full",
		Service:     diffService,
		Severity:    "sev3",
		Status:      "firing",
		Fingerprint: fmt.Sprintf("%s-disk", fp),
		Source:      "generic",
	}

	inc4, isNew4, err := grouper.ProcessAlert(ctx, orgID, alert4)
	if err != nil {
		t.Fatalf("failed to process alert4: %v", err)
	}
	if !isNew4 {
		t.Errorf("expected isNew=true for different service")
	}
	if inc4.ID == inc1.ID {
		t.Errorf("expected different incident ID")
	}
}

func TestRedisQueueLive(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6380"
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		opts = &redis.Options{Addr: redisURL}
	}

	rdb := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping live redis test: redis not reachable (%v)", err)
		return
	}
	defer rdb.Close()

	streamKey := fmt.Sprintf("test:stream:%d", time.Now().UnixNano())
	defer rdb.Del(ctx, streamKey)

	queue := NewRedisQueue(rdb, streamKey, "test-group", "test-consumer")

	var orgID, incID pgtype.UUID
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")
	_ = incID.Scan("00000000-0000-0000-0000-000000000002")

	job := InvestigationJob{
		OrgID:      orgID,
		IncidentID: incID,
		Title:      "Redis stream test job",
		Service:    "payment-api",
		Severity:   "sev1",
		Source:     "pagerduty",
		EnqueuedAt: time.Now().UTC(),
	}

	err = queue.Enqueue(ctx, job)
	if err != nil {
		t.Fatalf("failed to enqueue to redis queue: %v", err)
	}

	dequeued, err := queue.Dequeue(ctx, 1*time.Second)
	if err != nil {
		t.Fatalf("failed to dequeue from redis queue: %v", err)
	}
	if dequeued == nil {
		t.Fatalf("expected non-nil dequeued job")
	}
	if dequeued.Title != job.Title {
		t.Errorf("expected job title %s, got %s", job.Title, dequeued.Title)
	}

	// Test nil client error
	nilQueue := &RedisQueue{client: nil}
	if err := nilQueue.Enqueue(ctx, job); err == nil {
		t.Errorf("expected error when client is nil")
	}
	if _, err := nilQueue.Dequeue(ctx, 100*time.Millisecond); err == nil {
		t.Errorf("expected error when client is nil")
	}
}

func TestLogAdaptersExtended(t *testing.T) {
	t.Run("File Log Adapter names and edge cases", func(t *testing.T) {
		adapter := NewFileLogAdapter("/non-existent-path-12345")
		if adapter.Name() != "file" {
			t.Errorf("expected Name() to be 'file', got %s", adapter.Name())
		}

		// Non-existent directory returns error
		_, err := adapter.QueryLogs(context.Background(), LogQuery{Service: "any"})
		if err == nil {
			t.Errorf("expected error for non-existent directory")
		}

		// Test non-matching file and text log lines with various levels
		tmpDir, err := os.MkdirTemp("", "triage-logs-levels-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		logContent := `2026-10-09 08:00:00 [DEBUG] cache hit on key: foo
2026-10-09 08:01:00 [INFO] order processed successfully
2026-10-09 08:02:00 [WARN] slow query detected
2026-10-09 08:03:00 [ERROR] database connection pool exhausted
2026-10-09 08:04:00 [FATAL] unrecoverable storage failure
2026-10-09 08:05:00 [PANIC] nil pointer dereference
`
		_ = os.WriteFile(filepath.Join(tmpDir, "order-service.log"), []byte(logContent), 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "unrelated.txt"), []byte("random text"), 0644)

		validAdapter := NewFileLogAdapter(tmpDir)
		entries, err := validAdapter.QueryLogs(context.Background(), LogQuery{
			Service: "order-service",
			Pattern: "pool exhausted",
			Limit:   5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 log entry, got %d", len(entries))
		}
		if entries[0].Level != "error" {
			t.Errorf("expected level error, got %s", entries[0].Level)
		}

		// Query non-matching service returns empty
		entriesEmpty, err := validAdapter.QueryLogs(context.Background(), LogQuery{
			Service: "unknown-service",
			Limit:   5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entriesEmpty) != 0 {
			t.Errorf("expected 0 entries for unknown-service, got %d", len(entriesEmpty))
		}
	})

	t.Run("Loki Log Adapter names and error handling", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query().Get("query")
			if strings.Contains(q, "error=500") {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if strings.Contains(q, "invalid_json=true") {
				w.Write([]byte("not valid json"))
				return
			}
			if strings.Contains(q, "streams_data=true") {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{
					"status": "success",
					"data": {
						"resultType": "streams",
						"result": [
							{
								"stream": { "service": "payment-api" },
								"values": [ [ "1700000000000000000", "database alive" ] ]
							}
						]
					}
				}`)
				return
			}
			w.Write([]byte(`{"status":"success","data":{"resultType":"streams","result":[]}}`))
		}))
		defer mockServer.Close()

		adapter := NewLokiLogAdapter(mockServer.URL, mockServer.Client(), "tenant-1")
		if adapter.Name() != "loki" {
			t.Errorf("expected Name() to be 'loki', got %s", adapter.Name())
		}

		// 500 status code
		_, err := adapter.QueryLogs(context.Background(), LogQuery{
			Service: "payment-api",
			Pattern: "error=500",
		})
		if err == nil {
			t.Errorf("expected error for 500 response")
		}

		// Invalid JSON
		_, err = adapter.QueryLogs(context.Background(), LogQuery{
			Service: "payment-api",
			Pattern: "invalid_json=true",
		})
		if err == nil {
			t.Errorf("expected error for invalid JSON")
		}

		// Streams with values
		entries, err := adapter.QueryLogs(context.Background(), LogQuery{
			Service: "payment-api",
			Pattern: "streams_data=true",
		})
		if err != nil {
			t.Fatalf("unexpected error for streams: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry for streams, got %d", len(entries))
		}
	})
}

func TestParsersExtended(t *testing.T) {
	t.Run("Alertmanager without fingerprint computes label fingerprint and handles warning/info", func(t *testing.T) {
		parser := NewAlertmanagerParser()
		payload := `{
			"version": "4",
			"status": "resolved",
			"alerts": [
				{
					"status": "resolved",
					"labels": {
						"alertname": "DiskUsageHigh",
						"service": "postgres",
						"severity": "warning"
					},
					"annotations": {
						"description": "Disk capacity at 85%"
					}
				},
				{
					"status": "firing",
					"labels": {
						"alertname": "InfoNotice",
						"service": "indexer",
						"severity": "info"
					}
				}
			]
		}`
		alerts, err := parser.Parse(context.Background(), []byte(payload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) != 2 {
			t.Fatalf("expected 2 alerts, got %d", len(alerts))
		}
		if alerts[0].Severity != "sev3" {
			t.Errorf("expected sev3 for warning, got %s", alerts[0].Severity)
		}
		if alerts[0].Status != "resolved" {
			t.Errorf("expected status resolved, got %s", alerts[0].Status)
		}
		if alerts[0].Fingerprint == "" {
			t.Errorf("expected computed fingerprint when absent")
		}
		if alerts[1].Severity != "sev4" {
			t.Errorf("expected sev4 for info, got %s", alerts[1].Severity)
		}
	})

	t.Run("Grafana legacy webhook format", func(t *testing.T) {
		parser := NewGrafanaParser()
		payload := `{
			"title": "Database connection pool saturated",
			"state": "alerting",
			"message": "Connections > 90%",
			"ruleUrl": "http://grafana/rules/1",
			"evalMatches": [
				{ "metric": "active_conns", "value": 95 }
			]
		}`
		alerts, err := parser.Parse(context.Background(), []byte(payload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) != 1 {
			t.Fatalf("expected 1 alert, got %d", len(alerts))
		}
		if alerts[0].Title != "Database connection pool saturated" {
			t.Errorf("expected title, got %s", alerts[0].Title)
		}
		if alerts[0].Severity != "sev2" {
			t.Errorf("expected sev2 default, got %s", alerts[0].Severity)
		}
	})

	t.Run("PagerDuty resolved and low urgency", func(t *testing.T) {
		parser := NewPagerDutyParser()
		payload := `{
			"event": {
				"id": "02GHIJKL",
				"event_type": "incident.resolved",
				"occurred_at": "2026-10-09T08:00:00Z",
				"data": {
					"id": "PD100",
					"title": "Minor latency blip",
					"urgency": "low",
					"status": "resolved",
					"service": {
						"id": "S1",
						"name": "auth-service"
					}
				}
			}
		}`
		alerts, err := parser.Parse(context.Background(), []byte(payload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) != 1 {
			t.Fatalf("expected 1 alert, got %d", len(alerts))
		}
		if alerts[0].Severity != "sev3" {
			t.Errorf("expected sev3 for low urgency, got %s", alerts[0].Severity)
		}
		if alerts[0].Status != "resolved" {
			t.Errorf("expected resolved status, got %s", alerts[0].Status)
		}
	})

	t.Run("Generic parser single object with defaults", func(t *testing.T) {
		parser := NewGenericParser()
		payload := `{
			"message": "Unhandled exception in background worker",
			"severity": "sev1"
		}`
		alerts, err := parser.Parse(context.Background(), []byte(payload))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) != 1 {
			t.Fatalf("expected 1 alert, got %d", len(alerts))
		}
		if alerts[0].Service != "default-service" {
			t.Errorf("expected default-service fallback, got %s", alerts[0].Service)
		}
		if alerts[0].Title != "Unhandled exception in background worker" {
			t.Errorf("expected title fallback to message, got %s", alerts[0].Title)
		}
	})
}

func TestIngestHandlerExtended(t *testing.T) {
	secret := "dev_secret"
	queue := NewMemoryQueue(10)
	grouper := NewAlertGrouper(nil, queue, 30*time.Minute)
	handler := NewIngestHandler(nil, nil, grouper)

	orgIDStr := "10000000-0000-0000-0000-000000000001"

	payload := []byte(`{"title":"RabbitMQ Spike","service":"checkout","severity":"sev1","status":"firing"}`)
	sig := ComputeWebhookSignature(secret, payload, "")

	t.Run("Webhook with org_id in URL path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/"+orgIDStr+"/webhooks/generic", bytes.NewReader(payload))
		req.Header.Set("X-Signature-256", sig)
		req = withURLParam(req, "org_id", orgIDStr)
		req = withURLParam(req, "source", "generic")

		rec := httptest.NewRecorder()
		handler.HandleWebhook(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Webhook with org_id query parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/webhooks/generic?org_id="+orgIDStr, bytes.NewReader(payload))
		req.Header.Set("X-Signature-256", sig)
		req = withURLParam(req, "source", "generic")

		rec := httptest.NewRecorder()
		handler.HandleWebhook(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Webhook with X-Org-ID header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/webhooks/generic", bytes.NewReader(payload))
		req.Header.Set("X-Signature-256", sig)
		req.Header.Set("X-Org-ID", orgIDStr)
		req = withURLParam(req, "source", "generic")

		rec := httptest.NewRecorder()
		handler.HandleWebhook(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Missing org returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/webhooks/generic", bytes.NewReader(payload))
		req.Header.Set("X-Signature-256", sig)
		req = withURLParam(req, "source", "generic")

		rec := httptest.NewRecorder()
		handler.HandleWebhook(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("Invalid HMAC signature returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/"+orgIDStr+"/webhooks/generic", bytes.NewReader(payload))
		req.Header.Set("X-Signature-256", "sha256=invalid-signature")
		req = withURLParam(req, "org_id", orgIDStr)
		req = withURLParam(req, "source", "generic")

		rec := httptest.NewRecorder()
		handler.HandleWebhook(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("Empty non-actionable payload returns 200 ignored", func(t *testing.T) {
		emptyAlerts := []byte(`{"version":"4","alerts":[]}`)
		emptySig := ComputeWebhookSignature(secret, emptyAlerts, "")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/"+orgIDStr+"/webhooks/alertmanager", bytes.NewReader(emptyAlerts))
		req.Header.Set("X-Signature-256", emptySig)
		req = withURLParam(req, "org_id", orgIDStr)
		req = withURLParam(req, "source", "alertmanager")

		rec := httptest.NewRecorder()
		handler.HandleWebhook(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
