-- name: CreatePromptVersion :one
INSERT INTO prompt_versions (version_tag, content, content_hash, changelog, author, is_active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetActivePromptVersion :one
SELECT * FROM prompt_versions
WHERE is_active = true
ORDER BY created_at DESC
LIMIT 1;

-- name: GetPromptVersionByTag :one
SELECT * FROM prompt_versions
WHERE version_tag = $1;

-- name: ListPromptVersions :many
SELECT * FROM prompt_versions
ORDER BY created_at DESC;

-- name: CreateEvalScenario :one
INSERT INTO eval_scenarios (name, scenario_yaml, expected_root_cause)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListEvalScenarios :many
SELECT * FROM eval_scenarios
ORDER BY name ASC;

-- name: CreateEvalRun :one
INSERT INTO eval_runs (
    provider, model, prompt_version_id, top1_accuracy, top3_accuracy,
    precision, hallucination_rate, avg_latency_ms, avg_cost_usd
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListEvalRuns :many
SELECT * FROM eval_runs
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CreateEvalResult :one
INSERT INTO eval_results (
    eval_run_id, scenario_id, passed, predicted_cause, latency_ms, cost_usd, details
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListEvalResultsForRun :many
SELECT r.*, s.name as scenario_name, s.expected_root_cause
FROM eval_results r
JOIN eval_scenarios s ON s.id = r.scenario_id
WHERE r.eval_run_id = $1
ORDER BY s.name ASC;
