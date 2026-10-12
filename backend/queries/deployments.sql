-- Deployments are worker jobs. Simulations and real builds share the same rows,
-- stages, and lifecycle; reads are filtered by kind so each API sees only its own.

-- name: CreateDeployment :one
INSERT INTO deployments(application_id, triggered_by, simulated)
SELECT a.id, sqlc.arg(user_id), sqlc.arg(simulated) FROM applications a
JOIN workspaces w ON w.id = a.workspace_id
JOIN workspace_memberships m ON m.workspace_id = w.id AND m.user_id = sqlc.arg(user_id)
WHERE w.slug = sqlc.arg(workspace_slug) AND a.id = sqlc.arg(application_id) AND a.lifecycle = 'active'
AND (m.role = 'owner' OR a.created_by = m.user_id OR (a.access_mode = 'workspace' AND m.role = 'editor')
 OR EXISTS(SELECT 1 FROM application_access_grants g WHERE g.application_id = a.id AND g.user_id = m.user_id AND g.role = 'editor'))
RETURNING *;

-- name: ListDeployments :many
SELECT d.* FROM deployments d
JOIN applications a ON a.id = d.application_id
JOIN workspaces w ON w.id = a.workspace_id
JOIN workspace_memberships m ON m.workspace_id = w.id AND m.user_id = sqlc.arg(user_id)
WHERE w.slug = sqlc.arg(workspace_slug) AND a.id = sqlc.arg(application_id) AND d.simulated = sqlc.arg(simulated)
AND (m.role = 'owner' OR a.created_by = m.user_id OR a.access_mode = 'workspace'
 OR EXISTS(SELECT 1 FROM application_access_grants g WHERE g.application_id = a.id AND g.user_id = m.user_id))
ORDER BY d.created_at DESC, d.id DESC LIMIT 50;

-- name: GetDeployment :one
SELECT d.* FROM deployments d
JOIN applications a ON a.id = d.application_id
JOIN workspaces w ON w.id = a.workspace_id
JOIN workspace_memberships m ON m.workspace_id = w.id AND m.user_id = sqlc.arg(user_id)
WHERE w.slug = sqlc.arg(workspace_slug) AND a.id = sqlc.arg(application_id)
AND d.id = sqlc.arg(deployment_id) AND d.simulated = sqlc.arg(simulated)
AND (m.role = 'owner' OR a.created_by = m.user_id OR a.access_mode = 'workspace'
 OR EXISTS(SELECT 1 FROM application_access_grants g WHERE g.application_id = a.id AND g.user_id = m.user_id));

-- name: LockDeployment :one
SELECT * FROM deployments WHERE id = $1 FOR UPDATE;

-- name: InitializeDeploymentStages :exec
INSERT INTO deployment_stage_events(deployment_id, stage)
VALUES ($1, 'source'), ($1, 'build'), ($1, 'package');

-- name: ListDeploymentStages :many
SELECT * FROM deployment_stage_events WHERE deployment_id = $1
ORDER BY stage;

-- name: StartDeployment :execrows
UPDATE deployments SET status = 'building', started_at = clock_timestamp()
WHERE id = $1 AND status = 'queued';

-- name: AdvanceDeploymentStage :execrows
UPDATE deployment_stage_events SET status = sqlc.arg(status),
 started_at = COALESCE(started_at, clock_timestamp()),
 finished_at = CASE WHEN sqlc.arg(status)::operation_status = 'succeeded' THEN clock_timestamp() ELSE NULL END
WHERE deployment_id = $1 AND stage = $2
 AND ((status = 'pending' AND sqlc.arg(status)::operation_status = 'running')
   OR (status = 'running' AND sqlc.arg(status)::operation_status = 'succeeded'));

-- name: FinishDeployment :execrows
UPDATE deployments SET status = sqlc.arg(status), failure_code = sqlc.narg(failure_code),
 artifact_id = sqlc.narg(artifact_id), artifact_sha256 = sqlc.narg(artifact_sha256), artifact_bytes = sqlc.narg(artifact_bytes),
 finished_at = clock_timestamp(),
 duration_ms = CASE WHEN started_at IS NULL THEN NULL ELSE LEAST(2147483647, GREATEST(0, EXTRACT(EPOCH FROM (clock_timestamp() - started_at))*1000))::integer END
WHERE id = sqlc.arg(id) AND status IN ('queued','building');

-- name: FinishDeploymentStages :exec
UPDATE deployment_stage_events SET status = CASE WHEN status = 'pending' THEN 'skipped'::operation_status ELSE 'failed'::operation_status END,
 failure_code = sqlc.narg(failure_code), finished_at = clock_timestamp()
WHERE deployment_id = $1 AND status IN ('pending', 'running');

-- Deployments left unfinished by a previous process are failed, never replayed.
-- name: RecoverInterruptedDeployments :many
UPDATE deployments SET status = 'failed', failure_code = 'process_interrupted', finished_at = clock_timestamp()
WHERE status IN ('queued','building') RETURNING id;

-- Artifacts still referenced by a deployment; anything else in the store is an orphan.
-- name: ListReferencedArtifacts :many
SELECT artifact_id::text FROM deployments WHERE artifact_id IS NOT NULL;

-- The previous live release becomes an ordinary succeeded one (its artifact is
-- kept for rollback). Called in the same transaction that promotes the new one.
-- name: DemoteLiveDeployments :many
UPDATE deployments SET status = 'succeeded'
WHERE application_id = $1 AND status = 'live' AND id <> $2
RETURNING id, artifact_id;

-- Routing lookup for a public request: no permissions, because live apps are
-- public. The lifecycle lets the caller answer paused/archived distinctly.
-- name: GetLiveRelease :one
SELECT a.id AS application_id, a.lifecycle, d.id AS deployment_id,
 d.artifact_id, d.artifact_sha256, d.artifact_bytes
FROM applications a
JOIN workspaces w ON w.id = a.workspace_id
LEFT JOIN deployments d ON d.application_id = a.id AND d.status = 'live'
WHERE w.slug = sqlc.arg(workspace_slug) AND a.slug = sqlc.arg(application_slug);

-- Every live release with its application's lifecycle: the reconciler's desired state.
-- name: ListLiveReleases :many
SELECT a.id AS application_id, a.lifecycle, d.id AS deployment_id,
 d.artifact_id, d.artifact_sha256, d.artifact_bytes
FROM deployments d JOIN applications a ON a.id = d.application_id
WHERE d.status = 'live';

-- Real builds continue past packaging: the release is provisioned, started and
-- health-checked before it counts as live. Simulations never get these rows.
-- name: InitializeRuntimeStages :exec
INSERT INTO deployment_stage_events(deployment_id, stage)
VALUES ($1, 'provision'), ($1, 'release'), ($1, 'health_check');

-- Real deployments that are queued or building. Their runtime resources exist
-- before the deployment is live and must not be reclaimed as orphans.
-- name: ListInProgressDeployments :many
SELECT id FROM deployments WHERE status IN ('queued', 'building') AND NOT simulated;
