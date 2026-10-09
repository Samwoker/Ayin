package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/seraph/triage/apps/api/internal/config"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Fallback for seed standalone run
		cfg = config.Config{
			DatabaseURL: os.Getenv("DATABASE_URL"),
			Environment: "development",
			LogLevel:    "info",
		}
		if cfg.DatabaseURL == "" {
			cfg.DatabaseURL = "postgres://triage:triage_secret_dev@localhost:5433/triage?sslmode=disable"
		}
	}

	logger := telemetry.SetupLogger(cfg.LogLevel, cfg.Environment)
	logger.Info("running database migrations and seed script...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := store.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to postgres", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer s.Close()

	if err := s.Migrate(ctx); err != nil {
		logger.Error("failed to apply migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("schema migrations applied successfully")

	// Execute Idempotent Seed SQL
	seedSQL := `
    -- 1. Demo Organizations
    INSERT INTO orgs (id, name, slug)
    VALUES 
        ('00000000-0000-0000-0000-000000000001', 'Acme Corp', 'acme'),
        ('00000000-0000-0000-0000-000000000002', 'Globex Logistics', 'globex')
    ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name;

    -- 2. Demo Users (Argon2id or dev hashes)
    INSERT INTO users (id, email, name, password_hash)
    VALUES 
        ('10000000-0000-0000-0000-000000000001', 'alice@acme.com', 'Alice Engineer', '$argon2id$v=19$m=65536,t=3,p=2$devsalt$passwordhash1'),
        ('10000000-0000-0000-0000-000000000002', 'bob@acme.com', 'Bob SRE', '$argon2id$v=19$m=65536,t=3,p=2$devsalt$passwordhash2'),
        ('10000000-0000-0000-0000-000000000003', 'carol@globex.com', 'Carol DevOps', '$argon2id$v=19$m=65536,t=3,p=2$devsalt$passwordhash3'),
        ('10000000-0000-0000-0000-000000000004', 'dan@globex.com', 'Dan Responder', '$argon2id$v=19$m=65536,t=3,p=2$devsalt$passwordhash4')
    ON CONFLICT (email) DO NOTHING;

    -- 3. Memberships
    INSERT INTO memberships (org_id, user_id, role)
    VALUES 
        ('00000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'owner'),
        ('00000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000002', 'responder'),
        ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000003', 'owner'),
        ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000004', 'viewer')
    ON CONFLICT (org_id, user_id) DO NOTHING;

    -- 4. API Keys
    INSERT INTO api_keys (id, org_id, name, key_prefix, key_hash)
    VALUES 
        ('20000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'Alertmanager Webhook', 'tr_live_acme', 'hash_acme_webhook_key_123'),
        ('20000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000002', 'Grafana Webhook', 'tr_live_glob', 'hash_globex_webhook_key_456')
    ON CONFLICT (id) DO NOTHING;

    -- 5. Org Budgets
    INSERT INTO org_budgets (org_id, daily_limit_usd, monthly_limit_usd, per_run_cap_usd, hard_stop)
    VALUES 
        ('00000000-0000-0000-0000-000000000001', 50.0, 1000.0, 2.0, true),
        ('00000000-0000-0000-0000-000000000002', 100.0, 2500.0, 3.5, true)
    ON CONFLICT (org_id) DO NOTHING;

    -- 6. Prompt Versions
    INSERT INTO prompt_versions (id, version_tag, content, content_hash, changelog, author, is_active)
    VALUES 
        ('30000000-0000-0000-0000-000000000001', 'v1.0.0', 'System prompt v1 baseline for incident investigation', 'hash_prompt_v1_0_0', 'Initial release', 'system', true),
        ('30000000-0000-0000-0000-000000000002', 'v1.1.0-exp', 'Experimental prompt with strict JSON schema and anti-hallucination guard', 'hash_prompt_v1_1_0', 'Added anti-hallucination verification step', 'engineer', false)
    ON CONFLICT (version_tag) DO NOTHING;

    -- 7. Demo Incident for Acme Corp
    INSERT INTO incidents (id, org_id, title, service, severity, status)
    VALUES 
        ('40000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'RabbitMQ queue backlog on checkout-worker', 'checkout-worker', 'sev1', 'investigating')
    ON CONFLICT (id) DO NOTHING;

    -- 8. Demo Alert
    INSERT INTO alerts (id, org_id, incident_id, fingerprint, source, title, raw_payload, status)
    VALUES 
        ('50000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', '40000000-0000-0000-0000-000000000001', 'fp-rabbitmq-checkout-depth', 'alertmanager', 'RabbitMQQueueDepthHigh: checkout-queue > 50,000 messages', '{"queue": "checkout-queue", "depth": 54200, "threshold": 10000}', 'firing')
    ON CONFLICT (id) DO NOTHING;

    -- 9. Demo Run & Steps
    INSERT INTO runs (id, org_id, incident_id, prompt_version_id, status, total_tokens, total_cost_usd)
    VALUES 
        ('60000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', '40000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 'succeeded', 2480, 0.007440)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO run_steps (org_id, run_id, step_number, step_type, tool_name, tool_input, tool_output, tokens_in, tokens_out, latency_ms)
    VALUES 
        ('00000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000001', 1, 'tool_call', 'query_metrics', '{"query": "rabbitmq_queue_messages_ready{queue=\"checkout-queue\"}"}', '{"result": 54200, "status": "success"}', 340, 60, 42),
        ('00000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000001', 2, 'tool_call', 'recent_deploys', '{"service": "checkout-worker", "limit": 3}', '{"deploys": [{"version": "v2.14.0", "time": "20m ago", "author": "dev@acme.com"}]}', 290, 85, 38)
    ON CONFLICT (id) DO NOTHING;

    -- 10. Hypotheses
    INSERT INTO hypotheses (id, org_id, run_id, rank, root_cause, confidence, evidence, checklist)
    VALUES 
        ('70000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000001', 1, 'Downstream payment gateway latency spike caused consumer thread exhaustion in checkout-worker v2.14.0', 0.94, '[{"tool": "query_metrics", "ref": "step-1", "excerpt": "queue depth surged from 120 to 54,200 immediately post-deploy"}]', '["Verify payment gateway latency", "Scale checkout-worker to 6 replicas", "Rollback to v2.13.9 if backlog does not drain in 10m"]')
    ON CONFLICT (id) DO NOTHING;

    -- 11. Runbook
    INSERT INTO runbooks (id, org_id, title, description, content_markdown)
    VALUES 
        ('80000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'RabbitMQ Backlog Triage', 'Step by step recovery for queue spikes', '# RabbitMQ Backlog Runbook\n\n1. Check consumer count\n2. Inspect DLQ\n3. Scale pods\n')
    ON CONFLICT (id) DO NOTHING;

    -- 12. Proposed Remediation
    INSERT INTO remediations (id, org_id, incident_id, hypothesis_id, action_type, target, params, rationale, risk_level, expected_effect, rollback_plan, status, expires_at)
    VALUES 
        ('90000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', '40000000-0000-0000-0000-000000000001', '70000000-0000-0000-0000-000000000001', 'scale_service', 'checkout-worker', '{"replicas": 6}', 'Drain accumulated 54,000 backlog messages', 'low', 'Consumer throughput will double, clearing backlog within 8 minutes', 'Scale replicas back down to 3', 'proposed', now() + interval '30 minutes')
    ON CONFLICT (id) DO NOTHING;
    `

	_, err = s.Pool().Exec(ctx, seedSQL)
	if err != nil {
		logger.Error("failed to execute seed SQL", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("seed completed successfully: two demo orgs, demo incidents, runs, and remediations seeded")
	fmt.Println("Seed complete: Acme Corp & Globex Logistics populated.")
}
