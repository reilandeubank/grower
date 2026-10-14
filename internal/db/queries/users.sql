-- name: CreateUser :exec
INSERT INTO users (id, username, password_hash, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: UpdateUserPassword :execrows
UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?;
