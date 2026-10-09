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
