-- name: CreateHypothesis :one
INSERT INTO hypotheses (org_id, run_id, rank, root_cause, confidence, evidence, checklist)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListHypothesesForRun :many
SELECT * FROM hypotheses
WHERE org_id = $1 AND run_id = $2
ORDER BY rank ASC;

-- name: GetHypothesisByID :one
SELECT * FROM hypotheses
WHERE org_id = $1 AND id = $2;
