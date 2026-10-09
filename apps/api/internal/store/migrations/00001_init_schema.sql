-- +goose Up
-- Enable pgcrypto and pgvector
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";

-- 1. Organizations
CREATE TABLE orgs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 3. Memberships (Tenant Table)
CREATE TABLE memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'responder', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_memberships_org_user UNIQUE (org_id, user_id)
);
CREATE INDEX idx_memberships_org_role ON memberships (org_id, role);
CREATE INDEX idx_memberships_user ON memberships (user_id);

-- 4. API Keys (Tenant Table)
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    key_prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX idx_api_keys_org ON api_keys (org_id, created_at);
CREATE INDEX idx_api_keys_hash ON api_keys (key_hash) WHERE revoked_at IS NULL;

-- 5. Incidents (Tenant Table)
CREATE TABLE incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    service TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('sev1', 'sev2', 'sev3', 'sev4', 'sev5')),
    status TEXT NOT NULL CHECK (status IN ('open', 'investigating', 'mitigated', 'resolved', 'closed')),
    opened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_incidents_org_status ON incidents (org_id, status);
CREATE INDEX idx_incidents_org_opened ON incidents (org_id, opened_at DESC);
CREATE INDEX idx_incidents_org_service ON incidents (org_id, service);

-- 6. Alerts (Tenant Table)
CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    incident_id UUID REFERENCES incidents(id) ON DELETE SET NULL,
    fingerprint TEXT NOT NULL,
    source TEXT NOT NULL,
    title TEXT NOT NULL,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'firing',
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_alerts_org_fingerprint ON alerts (org_id, fingerprint);
CREATE INDEX idx_alerts_org_incident ON alerts (org_id, incident_id);
CREATE INDEX idx_alerts_org_received ON alerts (org_id, received_at DESC);

-- 7. Incident Events (Tenant Table)
CREATE TABLE incident_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_incident_events_org_incident ON incident_events (org_id, incident_id, created_at);

-- 8. Prompt Versions
CREATE TABLE prompt_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_tag TEXT NOT NULL UNIQUE,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL UNIQUE,
    changelog TEXT NOT NULL DEFAULT '',
    author TEXT NOT NULL DEFAULT 'system',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 9. Investigation Runs (Tenant Table)
CREATE TABLE runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    prompt_version_id UUID REFERENCES prompt_versions(id) ON DELETE SET NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled', 'budget_exhausted')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    total_tokens INT NOT NULL DEFAULT 0,
    total_cost_usd NUMERIC(10, 6) NOT NULL DEFAULT 0.0,
    error_message TEXT
);
CREATE INDEX idx_runs_org_incident ON runs (org_id, incident_id);
CREATE INDEX idx_runs_org_status ON runs (org_id, status);

-- 10. Run Steps (Tenant Table)
CREATE TABLE run_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    step_number INT NOT NULL,
    step_type TEXT NOT NULL CHECK (step_type IN ('plan', 'tool_call', 'tool_observation', 'llm_response', 'conclusion')),
    tool_name TEXT,
    tool_input JSONB,
    tool_output JSONB,
    tokens_in INT NOT NULL DEFAULT 0,
    tokens_out INT NOT NULL DEFAULT 0,
    cost_usd NUMERIC(10, 6) NOT NULL DEFAULT 0.0,
    latency_ms INT NOT NULL DEFAULT 0,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_run_steps_org_run ON run_steps (org_id, run_id, step_number);

-- 11. Hypotheses (Tenant Table)
CREATE TABLE hypotheses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    rank INT NOT NULL,
    root_cause TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL,
    evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
    checklist JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_hypotheses_org_run ON hypotheses (org_id, run_id, rank);

-- 12. Feedback (Tenant Table)
CREATE TABLE feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    hypothesis_id UUID NOT NULL REFERENCES hypotheses(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    is_correct BOOLEAN NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_feedback_org_hypo ON feedback (org_id, hypothesis_id);

-- 13. Runbooks (Tenant Table)
CREATE TABLE runbooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    content_markdown TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_runbooks_org_title ON runbooks (org_id, title);

-- 14. Runbook Chunks with pgvector (Tenant Table)
CREATE TABLE runbook_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    runbook_id UUID NOT NULL REFERENCES runbooks(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    chunk_text TEXT NOT NULL,
    embedding vector(1536),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_runbook_chunks_org_book ON runbook_chunks (org_id, runbook_id);

-- 15. Remediations (Tenant Table)
CREATE TABLE remediations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    hypothesis_id UUID REFERENCES hypotheses(id) ON DELETE SET NULL,
    action_type TEXT NOT NULL CHECK (action_type IN ('restart_service', 'scale_service', 'rollback_deploy', 'clear_cache', 'pause_consumer', 'toggle_feature_flag')),
    target TEXT NOT NULL,
    params JSONB NOT NULL DEFAULT '{}'::jsonb,
    rationale TEXT NOT NULL,
    risk_level TEXT NOT NULL CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    expected_effect TEXT NOT NULL,
    rollback_plan TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('proposed', 'approved', 'rejected', 'expired', 'executing', 'succeeded', 'failed', 'rolled_back')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_remediations_org_status ON remediations (org_id, status);
CREATE INDEX idx_remediations_org_incident ON remediations (org_id, incident_id);

-- 16. Remediation Events (Tenant Table)
CREATE TABLE remediation_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    remediation_id UUID NOT NULL REFERENCES remediations(id) ON DELETE CASCADE,
    transition_from TEXT NOT NULL,
    transition_to TEXT NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_remediation_events_org_rem ON remediation_events (org_id, remediation_id);

-- 17. Experiments
CREATE TABLE experiments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    variant_a_prompt_id UUID NOT NULL REFERENCES prompt_versions(id),
    variant_b_prompt_id UUID NOT NULL REFERENCES prompt_versions(id),
    traffic_split DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 18. Experiment Assignments (Tenant Table)
CREATE TABLE experiment_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    experiment_id UUID NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    assigned_variant TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_exp_assign_org_incident ON experiment_assignments (org_id, incident_id);

-- 19. Org Budgets (Tenant Table)
CREATE TABLE org_budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL UNIQUE REFERENCES orgs(id) ON DELETE CASCADE,
    daily_limit_usd NUMERIC(10, 4) NOT NULL DEFAULT 50.0,
    monthly_limit_usd NUMERIC(10, 4) NOT NULL DEFAULT 1000.0,
    per_run_cap_usd NUMERIC(10, 4) NOT NULL DEFAULT 2.0,
    hard_stop BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_org_budgets_org ON org_budgets (org_id);

-- 20. Usage Ledger (Tenant Table)
CREATE TABLE usage_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    run_id UUID REFERENCES runs(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    tokens_in INT NOT NULL,
    tokens_out INT NOT NULL,
    cost_usd NUMERIC(10, 6) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_usage_ledger_org_created ON usage_ledger (org_id, created_at);

-- 21. Eval Scenarios
CREATE TABLE eval_scenarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    scenario_yaml TEXT NOT NULL,
    expected_root_cause TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 22. Eval Runs
CREATE TABLE eval_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    prompt_version_id UUID REFERENCES prompt_versions(id) ON DELETE SET NULL,
    top1_accuracy DOUBLE PRECISION NOT NULL,
    top3_accuracy DOUBLE PRECISION NOT NULL,
    precision DOUBLE PRECISION NOT NULL,
    hallucination_rate DOUBLE PRECISION NOT NULL,
    avg_latency_ms INT NOT NULL,
    avg_cost_usd NUMERIC(10, 6) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 23. Eval Results
CREATE TABLE eval_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    eval_run_id UUID NOT NULL REFERENCES eval_runs(id) ON DELETE CASCADE,
    scenario_id UUID NOT NULL REFERENCES eval_scenarios(id) ON DELETE CASCADE,
    passed BOOLEAN NOT NULL,
    predicted_cause TEXT NOT NULL,
    latency_ms INT NOT NULL,
    cost_usd NUMERIC(10, 6) NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_eval_results_run ON eval_results (eval_run_id);

-- 24. Retention Policies (Tenant Table)
CREATE TABLE retention_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    data_class TEXT NOT NULL,
    retention_days INT NOT NULL DEFAULT 90,
    legal_hold BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_retention_org_class UNIQUE (org_id, data_class)
);
CREATE INDEX idx_retention_org ON retention_policies (org_id);

-- 25. Audit Log (Tenant Table, Append-Only)
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor_id UUID,
    actor_type TEXT NOT NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    prev_hash TEXT NOT NULL DEFAULT '',
    curr_hash TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_log_org_created ON audit_log (org_id, created_at);

-- =========================================================
-- ROW-LEVEL SECURITY (RLS) POLICIES
-- =========================================================

-- Helper function to extract org_id from PostgreSQL session variable
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION current_app_org_id() RETURNS UUID AS $$
BEGIN
    RETURN NULLIF(current_setting('app.org_id', true), '')::UUID;
EXCEPTION
    WHEN OTHERS THEN
        RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;
-- +goose StatementEnd

-- Macro-like RLS enabling for each tenant table:
-- +goose StatementBegin
DO $$
DECLARE
    tbl text;
    tenant_tables text[] := ARRAY[
        'memberships',
        'api_keys',
        'incidents',
        'alerts',
        'incident_events',
        'runs',
        'run_steps',
        'hypotheses',
        'feedback',
        'runbooks',
        'runbook_chunks',
        'remediations',
        'remediation_events',
        'experiment_assignments',
        'org_budgets',
        'usage_ledger',
        'retention_policies',
        'audit_log'
    ];
BEGIN
    FOREACH tbl IN ARRAY tenant_tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY;', tbl);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY;', tbl);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_policy ON %I;', tbl);
        EXECUTE format('CREATE POLICY tenant_isolation_policy ON %I FOR ALL USING (org_id = current_app_org_id()) WITH CHECK (org_id = current_app_org_id());', tbl);
    END LOOP;

    -- Ensure non-superuser app role exists for RLS enforcement
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'triage_app') THEN
        CREATE ROLE triage_app WITH LOGIN PASSWORD 'triage_secret_app' NOSUPERUSER NOBYPASSRLS;
    END IF;
    GRANT ALL ON ALL TABLES IN SCHEMA public TO triage_app;
    GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO triage_app;
    GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO triage_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO triage_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO triage_app;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS audit_log CASCADE;
DROP TABLE IF EXISTS retention_policies CASCADE;
DROP TABLE IF EXISTS eval_results CASCADE;
DROP TABLE IF EXISTS eval_runs CASCADE;
DROP TABLE IF EXISTS eval_scenarios CASCADE;
DROP TABLE IF EXISTS usage_ledger CASCADE;
DROP TABLE IF EXISTS org_budgets CASCADE;
DROP TABLE IF EXISTS experiment_assignments CASCADE;
DROP TABLE IF EXISTS experiments CASCADE;
DROP TABLE IF EXISTS remediation_events CASCADE;
DROP TABLE IF EXISTS remediations CASCADE;
DROP TABLE IF EXISTS runbook_chunks CASCADE;
DROP TABLE IF EXISTS runbooks CASCADE;
DROP TABLE IF EXISTS feedback CASCADE;
DROP TABLE IF EXISTS hypotheses CASCADE;
DROP TABLE IF EXISTS run_steps CASCADE;
DROP TABLE IF EXISTS runs CASCADE;
DROP TABLE IF EXISTS prompt_versions CASCADE;
DROP TABLE IF EXISTS incident_events CASCADE;
DROP TABLE IF EXISTS alerts CASCADE;
DROP TABLE IF EXISTS incidents CASCADE;
DROP TABLE IF EXISTS api_keys CASCADE;
DROP TABLE IF EXISTS memberships CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS orgs CASCADE;
DROP FUNCTION IF EXISTS current_app_org_id();
