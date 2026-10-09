-- name: CreateRun :one
INSERT INTO runs (org_id, incident_id, prompt_version_id, status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetRunByID :one
SELECT * FROM runs
WHERE org_id = $1 AND id = $2;

-- name: UpdateRunStatus :one
UPDATE runs
SET status = $3, completed_at = $4, total_tokens = $5, total_cost_usd = $6, error_message = $7
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: ListRunsForIncident :many
SELECT * FROM runs
WHERE org_id = $1 AND incident_id = $2
ORDER BY started_at DESC;
