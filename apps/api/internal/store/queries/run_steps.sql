-- name: CreateRunStep :one
INSERT INTO run_steps (
    org_id, run_id, step_number, step_type, tool_name,
    tool_input, tool_output, tokens_in, tokens_out, cost_usd, latency_ms, error
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: ListRunSteps :many
SELECT * FROM run_steps
WHERE org_id = $1 AND run_id = $2
ORDER BY step_number ASC;

-- name: GetRunStepByNumber :one
SELECT * FROM run_steps
WHERE org_id = $1 AND run_id = $2 AND step_number = $3;
