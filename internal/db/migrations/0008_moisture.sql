-- +goose Up
-- Moisture-meter watering (PLAN §10 idea 6, §5.5). A water task can be a soil check:
-- readings on the meter's 1–10 scale (dry 1–3, moist 4–7, wet 8–10) decide when to water.

ALTER TABLE care_tasks ADD COLUMN moisture_check INTEGER NOT NULL DEFAULT 0 CHECK (moisture_check IN (0, 1));
ALTER TABLE care_tasks ADD COLUMN water_at INTEGER NOT NULL DEFAULT 3 CHECK (water_at BETWEEN 1 AND 9);

-- SQLite can't change a CHECK constraint in place, so care_events is rebuilt with the
-- 'checked' kind and a reading: required for checks, optional on a watering (measured
-- just before), absent otherwise. Nothing references care_events, so dropping it is safe.
CREATE TABLE care_events_new (
    id           TEXT PRIMARY KEY,
    task_id      TEXT NOT NULL REFERENCES care_tasks (id) ON DELETE CASCADE,
    kind         TEXT NOT NULL CHECK (kind IN ('done', 'skipped', 'snoozed', 'checked')),
    local_date   TEXT NOT NULL,
    snooze_until TEXT CHECK ((kind = 'snoozed') = (snooze_until IS NOT NULL)),
    -- IS NULL first: NULL AND FALSE is FALSE, which would reject every skip and snooze.
    reading      INTEGER CHECK (
                     reading IS NULL
                     OR (reading BETWEEN 1 AND 10 AND kind IN ('checked', 'done'))
                 ),
    note         TEXT NOT NULL DEFAULT '',
    occurred_at  TEXT NOT NULL,
    voided_at    TEXT,
    created_at   TEXT NOT NULL,
    CHECK (kind <> 'checked' OR reading IS NOT NULL)
);
INSERT INTO care_events_new (id, task_id, kind, local_date, snooze_until, note, occurred_at, voided_at, created_at)
    SELECT id, task_id, kind, local_date, snooze_until, note, occurred_at, voided_at, created_at FROM care_events;
DROP TABLE care_events;
ALTER TABLE care_events_new RENAME TO care_events;
CREATE INDEX care_events_task_id_local_date ON care_events (task_id, local_date);

-- +goose Down
DELETE FROM care_events WHERE kind = 'checked';
CREATE TABLE care_events_old (
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
INSERT INTO care_events_old (id, task_id, kind, local_date, snooze_until, note, occurred_at, voided_at, created_at)
    SELECT id, task_id, kind, local_date, snooze_until, note, occurred_at, voided_at, created_at FROM care_events;
DROP TABLE care_events;
ALTER TABLE care_events_old RENAME TO care_events;
CREATE INDEX care_events_task_id_local_date ON care_events (task_id, local_date);
ALTER TABLE care_tasks DROP COLUMN water_at;
ALTER TABLE care_tasks DROP COLUMN moisture_check;
