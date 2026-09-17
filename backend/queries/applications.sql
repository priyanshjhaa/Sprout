-- name: ListApplicationsByWorkspaceSlug :many
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
WHERE workspace.slug = sqlc.arg(workspace_slug)
ORDER BY application.updated_at DESC, application.id;

-- name: GetApplicationByWorkspaceAndID :one
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
WHERE workspace.slug = sqlc.arg(workspace_slug)
  AND application.id = sqlc.arg(application_id);
