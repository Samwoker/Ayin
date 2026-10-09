-- name: CreateRunbook :one
INSERT INTO runbooks (org_id, title, description, content_markdown)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetRunbookByID :one
SELECT * FROM runbooks
WHERE org_id = $1 AND id = $2;

-- name: ListRunbooks :many
SELECT * FROM runbooks
WHERE org_id = $1
ORDER BY updated_at DESC;

-- name: DeleteRunbook :exec
DELETE FROM runbooks
WHERE org_id = $1 AND id = $2;

-- name: UpdateRunbook :one
UPDATE runbooks
SET title = $3, description = $4, content_markdown = $5, updated_at = now()
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: CreateRunbookChunk :one
INSERT INTO runbook_chunks (org_id, runbook_id, chunk_index, chunk_text, embedding)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListRunbookChunks :many
SELECT * FROM runbook_chunks
WHERE org_id = $1 AND runbook_id = $2
ORDER BY chunk_index ASC;

-- name: DeleteRunbookChunks :exec
DELETE FROM runbook_chunks
WHERE org_id = $1 AND runbook_id = $2;

-- name: SearchSimilarRunbookChunks :many
SELECT c.*, r.title as runbook_title, 1 - (c.embedding <=> $2) as similarity
FROM runbook_chunks c
JOIN runbooks r ON r.id = c.runbook_id
WHERE c.org_id = $1
ORDER BY c.embedding <=> $2
LIMIT $3;

-- name: SearchRunbooksByText :many
SELECT * FROM runbooks
WHERE org_id = $1
  AND (title ILIKE '%' || $2 || '%' OR description ILIKE '%' || $2 || '%' OR content_markdown ILIKE '%' || $2 || '%')
ORDER BY updated_at DESC
LIMIT $3;

