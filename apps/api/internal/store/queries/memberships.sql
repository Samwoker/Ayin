-- name: CreateMembership :one
INSERT INTO memberships (org_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetMembership :one
SELECT m.*, u.email, u.name as user_name
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1 AND m.user_id = $2;

-- name: ListOrgMembers :many
SELECT m.*, u.email, u.name as user_name
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1
ORDER BY m.created_at ASC;

-- name: UpdateMembershipRole :one
UPDATE memberships
SET role = $3, updated_at = now()
WHERE org_id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteMembership :exec
DELETE FROM memberships
WHERE org_id = $1 AND user_id = $2;

-- name: ListUserMemberships :many
SELECT m.org_id, m.user_id, m.role, m.created_at, o.name as org_name, o.slug as org_slug
FROM memberships m
JOIN orgs o ON o.id = m.org_id
WHERE m.user_id = $1
ORDER BY m.created_at ASC;

