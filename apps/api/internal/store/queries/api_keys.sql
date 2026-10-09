-- name: CreateAPIKey :one
INSERT INTO api_keys (org_id, name, key_prefix, key_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys
WHERE key_hash = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now());

-- name: ListAPIKeys :many
SELECT id, org_id, name, key_prefix, created_at, expires_at, revoked_at
FROM api_keys
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: RevokeAPIKey :one
UPDATE api_keys
SET revoked_at = now()
WHERE org_id = $1 AND id = $2
RETURNING *;
