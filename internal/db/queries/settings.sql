-- name: CreateSettings :exec
INSERT INTO settings (user_id, timezone, created_at, updated_at)
VALUES (?, ?, ?, ?);

-- name: GetSettings :one
SELECT * FROM settings WHERE user_id = ?;

-- Every user's settings, for the daily reminder.
-- name: ListSettings :many
SELECT * FROM settings ORDER BY user_id;

-- name: UpdateSettings :one
UPDATE settings SET timezone = ?, digest_local_time = ?, later_local_time = ?, hemisphere = ?, updated_at = ? WHERE user_id = ?
RETURNING *;
