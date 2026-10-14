-- name: CreateSession :exec
INSERT INTO sessions (id_hash, user_id, created_at, last_seen_at, expires_at, user_agent)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetSessionWithUser :one
SELECT s.id_hash, s.user_id, s.last_seen_at, s.expires_at, u.username
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id_hash = ?;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE id_hash = ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id_hash = ?;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = ?;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= ?;
