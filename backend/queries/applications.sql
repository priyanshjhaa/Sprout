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
    application.updated_at
FROM applications AS application
JOIN workspaces AS workspace ON workspace.id = application.workspace_id
JOIN workspace_memberships AS membership ON membership.workspace_id = workspace.id
WHERE workspace.slug = sqlc.arg(workspace_slug)
  AND membership.user_id = sqlc.arg(user_id)
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
    application.updated_at
FROM applications AS application
JOIN workspaces AS workspace ON workspace.id = application.workspace_id
JOIN workspace_memberships AS membership ON membership.workspace_id = workspace.id
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
          created_by, created_at, updated_at;

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
  AND membership.role IN ('owner', 'editor')
RETURNING application.id, application.workspace_id, application.name,
          application.slug, application.description, application.lifecycle,
          application.default_hostname, application.created_by,
          application.created_at, application.updated_at;
