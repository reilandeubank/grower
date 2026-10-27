-- +goose Up
-- The time a postponed daily reminder is sent again (issue #2). Tapping "Remind me at
-- 17:30" on a reminder queues a second jobs row (kind 'digest',
-- dedupe_key 'digest-later:<user id>:<local date>') for that time today.

ALTER TABLE settings ADD COLUMN later_local_time TEXT NOT NULL DEFAULT '17:30';

-- +goose Down
ALTER TABLE settings DROP COLUMN later_local_time;
