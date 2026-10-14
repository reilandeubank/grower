-- A job with a dedupe key is created at most once, however often the producer runs.
-- name: EnqueueJob :exec
INSERT INTO jobs (id, kind, dedupe_key, run_after, payload, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (dedupe_key) DO NOTHING;

-- Claims the next due job of a kind. A running job whose lock expired was left by a
-- crash or restart and is claimed again.
-- name: ClaimJob :one
UPDATE jobs SET status = 'running', locked_until = sqlc.arg(locked_until), updated_at = sqlc.arg(now)
WHERE id = (
    SELECT j.id FROM jobs AS j
    WHERE j.kind = sqlc.arg(job_kind) AND (
        (j.status = 'pending' AND j.run_after <= sqlc.arg(now))
        OR (j.status = 'running' AND j.locked_until <= sqlc.arg(now)))
    ORDER BY j.run_after, j.id
    LIMIT 1)
RETURNING *;

-- name: FinishJob :exec
UPDATE jobs SET status = 'done', last_error = ?, locked_until = NULL, updated_at = ?
WHERE id = ?;

-- Records a failed attempt: status is 'pending' to retry at run_after, or 'failed'.
-- name: FailJob :exec
UPDATE jobs SET status = ?, attempts = attempts + 1, last_error = ?, run_after = ?,
    locked_until = NULL, updated_at = ?
WHERE id = ?;
