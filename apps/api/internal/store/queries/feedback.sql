-- name: CreateFeedback :one
INSERT INTO feedback (org_id, hypothesis_id, user_id, is_correct, notes)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetFeedbackByHypothesis :one
SELECT * FROM feedback
WHERE org_id = $1 AND hypothesis_id = $2;

-- name: ListFeedbackByOrg :many
SELECT f.*, h.root_cause, h.confidence
FROM feedback f
JOIN hypotheses h ON h.id = f.hypothesis_id
WHERE f.org_id = $1
ORDER BY f.created_at DESC;
