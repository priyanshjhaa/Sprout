-- Personal access tokens. Only the SHA-256 hash of a token is ever stored.

-- name: InsertAPIToken :one
INSERT INTO api_tokens (user_id, name, token_hash, hint, expires_at)
VALUES (sqlc.arg(user_id), sqlc.arg(name), sqlc.arg(token_hash), sqlc.arg(hint), sqlc.arg(expires_at))
RETURNING id, user_id, name, hint, created_at, expires_at, last_used_at, revoked_at;

-- A user's tokens that were not revoked, newest first. Expired ones stay listed
-- (marked by their expiry) so people can see and remove them.
-- name: ListAPITokens :many
SELECT id, user_id, name, hint, created_at, expires_at, last_used_at, revoked_at
FROM api_tokens
WHERE user_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC, id;

-- Active tokens count against a per-user cap.
-- name: CountActiveAPITokens :one
SELECT count(*) FROM api_tokens WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: RevokeAPIToken :execrows
UPDATE api_tokens SET revoked_at = now()
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- The authentication lookup: an unrevoked, unexpired token by hash.
-- name: GetAPITokenByHash :one
SELECT id, user_id FROM api_tokens
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now();

-- Best-effort "last used" stamp, written at most once a minute per token.
-- name: TouchAPIToken :exec
UPDATE api_tokens SET last_used_at = now()
WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute');
