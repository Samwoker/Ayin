-- name: CreateAuditEntry :one
INSERT INTO audit_log (
    org_id, actor_id, actor_type, action, target_type, target_id, metadata, prev_hash, curr_hash
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListAuditEntries :many
SELECT * FROM audit_log
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetLatestAuditEntry :one
SELECT * FROM audit_log
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT 1;
