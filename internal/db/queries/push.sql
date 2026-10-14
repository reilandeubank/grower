-- Re-subscribing an endpoint (new keys, or another user on a shared device) takes it over.
-- name: UpsertPushSubscription :exec
INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, created_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (endpoint) DO UPDATE SET
    user_id = excluded.user_id, p256dh = excluded.p256dh, auth = excluded.auth, failed_count = 0;

-- name: DeleteUserPushSubscription :exec
DELETE FROM push_subscriptions WHERE user_id = ? AND endpoint = ?;

-- name: DeletePushSubscription :exec
DELETE FROM push_subscriptions WHERE id = ?;

-- name: ListUserPushSubscriptions :many
SELECT * FROM push_subscriptions WHERE user_id = ? ORDER BY created_at, id;

-- name: MarkPushDelivered :exec
UPDATE push_subscriptions SET last_ok_at = ?, failed_count = 0 WHERE id = ?;

-- name: MarkPushFailed :exec
UPDATE push_subscriptions SET failed_count = failed_count + 1 WHERE id = ?;
