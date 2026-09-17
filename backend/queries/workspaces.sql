-- name: GetWorkspaceBySlug :one
SELECT id, slug, name, created_by, created_at, updated_at
FROM workspaces
WHERE slug = $1;
