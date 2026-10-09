-- name: CreateOrg :one
INSERT INTO orgs (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: GetOrgByID :one
SELECT * FROM orgs
WHERE id = $1;

-- name: GetOrgBySlug :one
SELECT * FROM orgs
WHERE slug = $1;

-- name: ListOrgs :many
SELECT * FROM orgs
ORDER BY created_at DESC;
