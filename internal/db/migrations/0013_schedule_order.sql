-- +goose Up
-- care_events in the order schedules read them: by date, then insertion (issue #10).
-- Today walks each task's events back from the newest and stops early, so the index has
-- to cover the whole order. It also serves everything the (task_id, local_date) one did.

DROP INDEX care_events_task_id_local_date;
CREATE INDEX care_events_task_order ON care_events (task_id, local_date, created_at, id);

-- +goose Down
DROP INDEX care_events_task_order;
CREATE INDEX care_events_task_id_local_date ON care_events (task_id, local_date);
