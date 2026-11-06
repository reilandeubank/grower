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

-- The first account's settings: the time zone and hemisphere that household-wide
-- integrations (MQTT) keep their calendar in.
-- name: GetHouseholdSettings :one
SELECT sqlc.embed(settings)
FROM settings
JOIN users ON users.id = settings.user_id
ORDER BY users.created_at, users.id
LIMIT 1;
