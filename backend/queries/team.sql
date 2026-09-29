-- name: LockTeamWorkspace :one
SELECT id, slug, name FROM workspaces WHERE slug = $1 FOR UPDATE;

-- name: GetTeamRole :one
SELECT role FROM workspace_memberships WHERE workspace_id = $1 AND user_id = $2;

-- name: ListMyWorkspaces :many
SELECT w.id, w.slug, w.name FROM workspaces w
JOIN workspace_memberships m ON m.workspace_id = w.id
WHERE m.user_id = $1 ORDER BY w.created_at, w.id;

-- name: ListPendingInvitations :many
SELECT id, email, role, expires_at FROM workspace_invitations
WHERE workspace_id = $1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;

-- name: ExpireTeamInvitations :exec
UPDATE workspace_invitations SET revoked_at = now()
WHERE workspace_id = $1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at <= now();

-- name: TeamHasEmail :one
SELECT EXISTS(SELECT 1 FROM workspace_memberships m JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = $1 AND lower(u.email) = sqlc.arg(email)::text)::boolean;

-- name: CreateTeamInvitation :one
INSERT INTO workspace_invitations (workspace_id, email, role, token_hash, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5, now() + interval '7 days')
RETURNING id, email, role, expires_at;

-- name: FindInvitationWorkspace :one
SELECT w.slug FROM workspace_invitations i JOIN workspaces w ON w.id = i.workspace_id
WHERE i.token_hash = $1;

-- name: ConsumeTeamInvitation :one
UPDATE workspace_invitations SET accepted_at = now()
WHERE workspace_id = $1 AND token_hash = $2 AND email = $3
AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > clock_timestamp()
RETURNING role;

-- name: JoinTeam :exec
INSERT INTO workspace_memberships (workspace_id, user_id, role) VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, user_id) DO NOTHING;

-- name: RevokeTeamInvitation :execrows
UPDATE workspace_invitations SET revoked_at = now()
WHERE workspace_id = $1 AND id = $2 AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: ChangeTeamRole :execrows
UPDATE workspace_memberships SET role = $3
WHERE workspace_id = $1 AND user_id = $2 AND role <> 'owner';

-- name: RemoveTeamMember :execrows
DELETE FROM workspace_memberships WHERE workspace_id = $1 AND user_id = $2 AND role <> 'owner';

-- name: RemoveTeamMemberGrants :exec
DELETE FROM application_access_grants g USING applications a
WHERE g.application_id = a.id AND a.workspace_id = $1 AND g.user_id = $2;

-- name: RevokeMemberInvitations :exec
UPDATE workspace_invitations SET revoked_at = now()
WHERE workspace_id = $1 AND email = (SELECT lower(u.email) FROM users u WHERE u.id = sqlc.arg(user_id))
AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: CheckInvitationIdentity :one
SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND auth_provider = 'clerk' AND auth_subject = $2)::boolean;
