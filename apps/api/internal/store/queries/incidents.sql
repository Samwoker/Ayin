-- name: CreateIncident :one
INSERT INTO incidents (org_id, title, service, severity, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetIncidentByID :one
SELECT * FROM incidents
WHERE org_id = $1 AND id = $2;

-- name: ListIncidents :many
SELECT * FROM incidents
WHERE org_id = $1
ORDER BY opened_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateIncidentStatus :one
UPDATE incidents
SET status = $3, resolved_at = $4, updated_at = now()
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: CountIncidentsByStatus :many
SELECT status, count(*) as count
FROM incidents
WHERE org_id = $1
GROUP BY status;

-- name: GetOpenIncidentForService :one
SELECT * FROM incidents
WHERE org_id = $1 AND service = $2 AND status IN ('open', 'investigating') AND opened_at > $3
ORDER BY opened_at DESC
LIMIT 1;

-- name: SearchHistoricalIncidents :many
SELECT * FROM incidents
WHERE org_id = $1
  AND (sqlc.narg('service')::text IS NULL OR service = sqlc.narg('service'))
  AND (sqlc.narg('exclude_id')::uuid IS NULL OR id != sqlc.narg('exclude_id'))
  AND (sqlc.narg('query')::text IS NULL OR title ILIKE '%' || sqlc.narg('query') || '%')
ORDER BY opened_at DESC
LIMIT $2;

