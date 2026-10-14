-- name: CreateSettings :exec
INSERT INTO settings (user_id, timezone, created_at, updated_at)
VALUES (?, ?, ?, ?);

-- name: GetSettings :one
SELECT * FROM settings WHERE user_id = ?;

-- name: UpdateTimezone :one
UPDATE settings SET timezone = ?, updated_at = ? WHERE user_id = ?
RETURNING *;
