-- name: CreateAlert :one
INSERT INTO alerts (org_id, incident_id, fingerprint, source, title, raw_payload, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAlertByFingerprint :one
SELECT * FROM alerts
WHERE org_id = $1 AND fingerprint = $2 AND received_at > $3
ORDER BY received_at DESC
LIMIT 1;

-- name: ListAlertsForIncident :many
SELECT * FROM alerts
WHERE org_id = $1 AND incident_id = $2
ORDER BY received_at DESC;

-- name: AssociateAlertWithIncident :one
UPDATE alerts
SET incident_id = $3
WHERE org_id = $1 AND id = $2
RETURNING *;
