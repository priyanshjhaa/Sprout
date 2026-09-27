-- name: GetUserByIdentity :one
SELECT id, email, display_name
FROM users
WHERE auth_provider = 'clerk' AND auth_subject = sqlc.arg(auth_subject);

-- name: InsertClerkUser :one
INSERT INTO users (auth_provider, auth_subject, email, display_name)
VALUES ('clerk', sqlc.arg(auth_subject), sqlc.arg(email), sqlc.arg(display_name))
ON CONFLICT (auth_provider, auth_subject)
DO UPDATE SET auth_subject = EXCLUDED.auth_subject
RETURNING id, email, display_name;

-- name: GetHomeWorkspace :one
SELECT workspace.id, workspace.slug, workspace.name
FROM workspaces AS workspace
JOIN workspace_memberships AS membership ON membership.workspace_id = workspace.id
WHERE membership.user_id = sqlc.arg(user_id)
ORDER BY membership.created_at, workspace.id
LIMIT 1;

-- name: InsertPersonalWorkspace :one
INSERT INTO workspaces (slug, name, created_by)
VALUES (sqlc.arg(slug), 'My workspace', sqlc.arg(user_id))
ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
WHERE workspaces.created_by = EXCLUDED.created_by
RETURNING id, slug, name;

-- name: InsertOwnerMembership :exec
INSERT INTO workspace_memberships (workspace_id, user_id, role)
VALUES (sqlc.arg(workspace_id), sqlc.arg(user_id), 'owner')
ON CONFLICT (workspace_id, user_id) DO NOTHING;

-- name: GetWorkspaceForMember :one
SELECT workspace.id, workspace.slug, workspace.name
FROM workspaces AS workspace
JOIN workspace_memberships AS membership ON membership.workspace_id = workspace.id
WHERE workspace.slug = sqlc.arg(workspace_slug)
  AND membership.user_id = sqlc.arg(user_id);
