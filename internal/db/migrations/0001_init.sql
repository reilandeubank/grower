-- +goose Up
-- IDs are ULIDs generated in Go; timestamps are TEXT in RFC 3339 UTC
-- ("2006-01-02T15:04:05Z", fixed width so they also sort correctly as text).

CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

-- id_hash is the hex SHA-256 of the session token: a leaked DB never yields usable cookies.
CREATE TABLE sessions (
    id_hash      TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    expires_at   TEXT NOT NULL,
    user_agent   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX sessions_user_id ON sessions (user_id);
CREATE INDEX sessions_expires_at ON sessions (expires_at);

CREATE TABLE settings (
    user_id           TEXT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    timezone          TEXT NOT NULL,
    hemisphere        TEXT NOT NULL DEFAULT 'north' CHECK (hemisphere IN ('north', 'south')),
    digest_local_time TEXT NOT NULL DEFAULT '08:00',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);

CREATE TABLE jobs (
    id           TEXT PRIMARY KEY,
    kind         TEXT NOT NULL,
    dedupe_key   TEXT UNIQUE,
    run_after    TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'running', 'done', 'failed')),
    attempts     INTEGER NOT NULL DEFAULT 0,
    last_error   TEXT,
    payload      TEXT NOT NULL DEFAULT '{}',
    locked_until TEXT,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
CREATE INDEX jobs_status_run_after ON jobs (status, run_after);

-- +goose Down
DROP TABLE jobs;
DROP TABLE settings;
DROP TABLE sessions;
DROP TABLE users;
