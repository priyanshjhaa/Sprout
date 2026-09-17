-- name: GetWorkspaceAccess :one
SELECT workspace.id AS workspace_id, membership.role
FROM workspaces AS workspace
JOIN workspace_memberships AS membership ON membership.workspace_id = workspace.id
WHERE workspace.slug = sqlc.arg(workspace_slug)
  AND membership.user_id = sqlc.arg(user_id);
