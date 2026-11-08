-- +goose Up
-- Per-user API tokens for clients that can't hold the session cookie, such as Home
-- Assistant (issue #4). Like sessions, only the SHA-256 of the secret is stored.
-- scope 'write' is reserved for endpoints that change data; none accept a token yet.

CREATE TABLE api_tokens (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    label        TEXT NOT NULL,
    secret_hash  TEXT NOT NULL UNIQUE,
    scope        TEXT NOT NULL DEFAULT 'read' CHECK (scope IN ('read', 'write')),
    created_at   TEXT NOT NULL,
    last_used_at TEXT
);
CREATE INDEX api_tokens_user_id ON api_tokens (user_id);

-- +goose Down
DROP TABLE api_tokens;
