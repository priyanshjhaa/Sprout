-- name: ListApplicationsForMember :many
SELECT
    application.id,
    application.workspace_id,
    application.name,
    application.slug,
    application.description,
    application.lifecycle,
    application.default_hostname,
    application.created_by,
    application.created_at,
    application.updated_at,
    application.access_mode
FROM applications AS application
JOIN workspaces AS workspace ON workspace.id = application.workspace_id
JOIN workspace_memberships AS membership ON membership.workspace_id = workspace.id
WHERE workspace.slug = sqlc.arg(workspace_slug)
  AND membership.user_id = sqlc.arg(user_id)
  AND (
    membership.role = 'owner'
    OR application.created_by = sqlc.arg(user_id)
    OR application.access_mode = 'workspace'
    OR EXISTS (
      SELECT 1 FROM application_access_grants AS app_grant
      WHERE app_grant.application_id = application.id AND app_grant.user_id = sqlc.arg(user_id)
    )
  )
ORDER BY application.updated_at DESC, application.id;

-- name: GetApplicationForMember :one
SELECT
    application.id,
    application.workspace_id,
    application.name,
    application.slug,
    application.description,
    application.lifecycle,
    application.default_hostname,
    application.created_by,
    application.created_at,
    application.updated_at,
    application.access_mode
FROM applications AS application
JOIN workspaces AS workspace ON workspace.id = application.workspace_id
JOIN workspace_memberships AS membership ON membership.workspace_id = workspace.id
WHERE workspace.slug = sqlc.arg(workspace_slug)
  AND application.id = sqlc.arg(application_id)
  AND membership.user_id = sqlc.arg(user_id)
  AND (
    membership.role = 'owner'
    OR application.created_by = sqlc.arg(user_id)
    OR application.access_mode = 'workspace'
    OR EXISTS (
      SELECT 1 FROM application_access_grants AS app_grant
      WHERE app_grant.application_id = application.id AND app_grant.user_id = sqlc.arg(user_id)
    )
  );

-- name: GetApplicationAuthorization :one
SELECT membership.role AS workspace_role,
       application.access_mode,
       application.created_by = sqlc.arg(user_id) AS is_creator,
       app_grant.role AS grant_role
FROM applications AS application
JOIN workspaces AS workspace ON workspace.id = application.workspace_id
JOIN workspace_memberships AS membership ON membership.workspace_id = workspace.id
LEFT JOIN application_access_grants AS app_grant
  ON app_grant.application_id = application.id AND app_grant.user_id = sqlc.arg(user_id)
WHERE workspace.slug = sqlc.arg(workspace_slug)
  AND application.id = sqlc.arg(application_id)
  AND membership.user_id = sqlc.arg(user_id);

-- name: CreateApplicationForMember :one
INSERT INTO applications (
    workspace_id,
    name,
    slug,
    description,
    lifecycle,
    default_hostname,
    created_by
)
SELECT
    workspace.id,
    sqlc.arg(name),
    sqlc.arg(slug),
    sqlc.arg(description),
    'active',
    sqlc.arg(default_hostname),
    sqlc.arg(user_id)
FROM workspaces AS workspace
JOIN workspace_memberships AS membership ON membership.workspace_id = workspace.id
WHERE workspace.slug = sqlc.arg(workspace_slug)
  AND membership.user_id = sqlc.arg(user_id)
  AND membership.role IN ('owner', 'editor')
RETURNING id, workspace_id, name, slug, description, lifecycle, default_hostname,
          created_by, created_at, updated_at, access_mode;

-- name: UpdateApplicationForMember :one
UPDATE applications AS application
SET
    name = COALESCE(sqlc.narg(name)::text, application.name),
    description = COALESCE(sqlc.narg(description)::text, application.description),
    lifecycle = COALESCE(
        sqlc.narg(lifecycle)::application_lifecycle,
        application.lifecycle
    ),
    updated_at = now()
FROM workspaces AS workspace, workspace_memberships AS membership
WHERE application.id = sqlc.arg(application_id)
  AND application.workspace_id = workspace.id
  AND workspace.slug = sqlc.arg(workspace_slug)
  AND membership.workspace_id = workspace.id
  AND membership.user_id = sqlc.arg(user_id)
  AND (
    membership.role = 'owner'
    OR application.created_by = sqlc.arg(user_id)
    OR (application.access_mode = 'workspace' AND membership.role = 'editor')
    OR EXISTS (
      SELECT 1 FROM application_access_grants AS app_grant
      WHERE app_grant.application_id = application.id
        AND app_grant.user_id = sqlc.arg(user_id)
        AND app_grant.role = 'editor'
    )
  )
RETURNING application.id, application.workspace_id, application.name,
          application.slug, application.description, application.lifecycle,
          application.default_hostname, application.created_by,
          application.created_at, application.updated_at, application.access_mode;
