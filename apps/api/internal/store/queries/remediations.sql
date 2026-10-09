-- name: CreateRemediation :one
INSERT INTO remediations (
    org_id, incident_id, hypothesis_id, action_type, target,
    params, rationale, risk_level, expected_effect, rollback_plan,
    status, expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetRemediationByID :one
SELECT * FROM remediations
WHERE org_id = $1 AND id = $2;

-- name: ListRemediationsByIncident :many
SELECT * FROM remediations
WHERE org_id = $1 AND incident_id = $2
ORDER BY created_at DESC;

-- name: UpdateRemediationStatus :one
UPDATE remediations
SET status = $3, updated_at = now()
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: CreateRemediationEvent :one
INSERT INTO remediation_events (
    org_id, remediation_id, transition_from, transition_to, user_id, reason
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListRemediationEvents :many
SELECT * FROM remediation_events
WHERE org_id = $1 AND remediation_id = $2
ORDER BY created_at ASC;
