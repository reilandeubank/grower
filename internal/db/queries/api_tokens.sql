-- name: CreateAPIToken :exec
INSERT INTO api_tokens (id, user_id, label, secret_hash, scope, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListUserAPITokens :many
SELECT id, label, scope, created_at, last_used_at
FROM api_tokens
WHERE user_id = ?
ORDER BY created_at, id;

-- name: GetAPITokenWithUser :one
SELECT t.id, t.user_id, t.scope, t.last_used_at, u.username
FROM api_tokens t
JOIN users u ON u.id = t.user_id
WHERE t.secret_hash = ?;

-- name: TouchAPIToken :exec
UPDATE api_tokens SET last_used_at = ? WHERE id = ?;

-- name: DeleteUserAPIToken :execrows
DELETE FROM api_tokens WHERE id = ? AND user_id = ?;
