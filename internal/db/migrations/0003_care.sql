-- +goose Up
-- Care scheduling. Next-due dates are never stored: internal/schedule derives them from
-- the task and its event log (PLAN §5). Dates are civil (YYYY-MM-DD) in the user's time zone.

CREATE TABLE care_tasks (
    id            TEXT PRIMARY KEY,
    plant_id      TEXT NOT NULL REFERENCES plants (id) ON DELETE CASCADE,
    kind          TEXT NOT NULL CHECK (kind IN ('water', 'fertilize', 'mist', 'repot', 'rotate', 'custom')),
    label         TEXT NOT NULL DEFAULT '',
    interval_days INTEGER NOT NULL CHECK (interval_days BETWEEN 1 AND 365),
    anchor        TEXT NOT NULL DEFAULT 'from_completion' CHECK (anchor IN ('from_completion', 'fixed')),
    start_on      TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);
CREATE INDEX care_tasks_plant_id ON care_tasks (plant_id);

-- Append-only, except voided_at, which undo sets. snooze_until is set exactly for snoozes.
CREATE TABLE care_events (
    id           TEXT PRIMARY KEY,
    task_id      TEXT NOT NULL REFERENCES care_tasks (id) ON DELETE CASCADE,
    kind         TEXT NOT NULL CHECK (kind IN ('done', 'skipped', 'snoozed')),
    local_date   TEXT NOT NULL,
    snooze_until TEXT CHECK ((kind = 'snoozed') = (snooze_until IS NOT NULL)),
    note         TEXT NOT NULL DEFAULT '',
    occurred_at  TEXT NOT NULL,
    voided_at    TEXT,
    created_at   TEXT NOT NULL
);
CREATE INDEX care_events_task_id_local_date ON care_events (task_id, local_date);

-- +goose Down
DROP TABLE care_events;
DROP TABLE care_tasks;
