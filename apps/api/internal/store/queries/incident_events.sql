-- name: CreateIncidentEvent :one
INSERT INTO incident_events (org_id, incident_id, event_type, payload)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListIncidentEvents :many
SELECT * FROM incident_events
WHERE org_id = $1 AND incident_id = $2
ORDER BY created_at ASC;
