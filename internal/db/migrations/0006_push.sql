-- +goose Up
-- Devices subscribed to the daily reminder. The digest itself is a row in jobs
-- (kind 'digest', dedupe_key 'digest:<user id>:<local date>'), so a restart never sends it twice.

CREATE TABLE push_subscriptions (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    endpoint     TEXT NOT NULL UNIQUE,
    p256dh       TEXT NOT NULL,
    auth         TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    last_ok_at   TEXT,
    failed_count INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX push_subscriptions_user_id ON push_subscriptions (user_id);

-- +goose Down
DROP TABLE push_subscriptions;
