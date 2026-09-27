-- name: ListWorkspaceMembers :many
SELECT member.id, member.email, member.display_name, membership.role
FROM workspaces AS workspace
JOIN workspace_memberships AS membership ON membership.workspace_id = workspace.id
JOIN users AS member ON member.id = membership.user_id
WHERE workspace.slug = sqlc.arg(workspace_slug)
  AND EXISTS (
    SELECT 1 FROM workspace_memberships AS actor
    WHERE actor.workspace_id = workspace.id AND actor.user_id = sqlc.arg(actor_id)
  )
ORDER BY membership.created_at, member.id;

-- name: ListApplicationGrants :many
SELECT member.id, member.email, member.display_name, app_grant.role
FROM application_access_grants AS app_grant
JOIN users AS member ON member.id = app_grant.user_id
JOIN applications AS application ON application.id = app_grant.application_id
JOIN workspaces AS workspace ON workspace.id = application.workspace_id
JOIN workspace_memberships AS membership
  ON membership.workspace_id = workspace.id AND membership.user_id = member.id
WHERE workspace.slug = sqlc.arg(workspace_slug)
  AND application.id = sqlc.arg(application_id)
ORDER BY app_grant.created_at, member.id;

-- name: SetApplicationAccessMode :one
UPDATE applications AS application
SET access_mode = sqlc.arg(access_mode), updated_at = now()
FROM workspaces AS workspace, workspace_memberships AS actor
WHERE application.id = sqlc.arg(application_id)
  AND application.workspace_id = workspace.id
  AND workspace.slug = sqlc.arg(workspace_slug)
  AND actor.workspace_id = workspace.id
  AND actor.user_id = sqlc.arg(actor_id)
  AND (actor.role = 'owner' OR application.created_by = sqlc.arg(actor_id))
RETURNING application.access_mode;

-- name: UpsertApplicationGrant :one
INSERT INTO application_access_grants (application_id, user_id, role, granted_by)
SELECT application.id, target.user_id, sqlc.arg(role), sqlc.arg(actor_id)
FROM applications AS application
JOIN workspaces AS workspace ON workspace.id = application.workspace_id
JOIN workspace_memberships AS actor
  ON actor.workspace_id = workspace.id AND actor.user_id = sqlc.arg(actor_id)
JOIN workspace_memberships AS target
  ON target.workspace_id = workspace.id AND target.user_id = sqlc.arg(target_id)
WHERE workspace.slug = sqlc.arg(workspace_slug)
  AND application.id = sqlc.arg(application_id)
  AND (actor.role = 'owner' OR application.created_by = sqlc.arg(actor_id))
  AND target.role != 'owner'
  AND target.user_id != application.created_by
ON CONFLICT (application_id, user_id)
DO UPDATE SET role = EXCLUDED.role, granted_by = EXCLUDED.granted_by
RETURNING user_id, role;

-- name: RevokeApplicationGrant :one
DELETE FROM application_access_grants AS app_grant
USING applications AS application, workspaces AS workspace, workspace_memberships AS actor
WHERE app_grant.application_id = application.id
  AND app_grant.user_id = sqlc.arg(target_id)
  AND application.id = sqlc.arg(application_id)
  AND application.workspace_id = workspace.id
  AND workspace.slug = sqlc.arg(workspace_slug)
  AND actor.workspace_id = workspace.id
  AND actor.user_id = sqlc.arg(actor_id)
  AND (actor.role = 'owner' OR application.created_by = sqlc.arg(actor_id))
RETURNING app_grant.user_id;
