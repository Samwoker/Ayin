-- name: UpsertOrgBudget :one
INSERT INTO org_budgets (
    org_id, daily_limit_usd, monthly_limit_usd, per_run_cap_usd, hard_stop
)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (org_id) DO UPDATE SET
    daily_limit_usd = EXCLUDED.daily_limit_usd,
    monthly_limit_usd = EXCLUDED.monthly_limit_usd,
    per_run_cap_usd = EXCLUDED.per_run_cap_usd,
    hard_stop = EXCLUDED.hard_stop,
    updated_at = now()
RETURNING *;

-- name: GetOrgBudget :one
SELECT * FROM org_budgets
WHERE org_id = $1;

-- name: RecordUsage :one
INSERT INTO usage_ledger (
    org_id, run_id, provider, model, tokens_in, tokens_out, cost_usd
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetOrgUsageSumSince :one
SELECT COALESCE(SUM(cost_usd), 0.0)::numeric as total_cost,
       COALESCE(SUM(tokens_in + tokens_out), 0)::bigint as total_tokens
FROM usage_ledger
WHERE org_id = $1 AND created_at >= $2;
