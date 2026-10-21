-- +goose NO TRANSACTION
-- +goose Up
-- Journal (PLAN §10 ideas 4, 7): measurements over time, and a 'check_in' task kind.
--
-- Adding a kind means rebuilding care_tasks (SQLite can't alter a CHECK). care_events
-- references care_tasks with ON DELETE CASCADE, so the rebuild must run with foreign keys
-- off, or dropping the old table would delete every event. PRAGMA foreign_keys is ignored
-- inside a transaction, hence NO TRANSACTION here and an explicit one below. goose runs
-- these statements one by one on one connection (the writer), so the pragma applies.
-- This is SQLite's documented procedure: https://www.sqlite.org/lang_altertable.html#otheralter

PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE care_tasks_new (
    id             TEXT PRIMARY KEY,
    plant_id       TEXT NOT NULL REFERENCES plants (id) ON DELETE CASCADE,
    kind           TEXT NOT NULL CHECK (kind IN ('water', 'fertilize', 'mist', 'repot', 'rotate', 'custom', 'check_in')),
    label          TEXT NOT NULL DEFAULT '',
    interval_days  INTEGER NOT NULL CHECK (interval_days BETWEEN 1 AND 365),
    anchor         TEXT NOT NULL DEFAULT 'from_completion' CHECK (anchor IN ('from_completion', 'fixed')),
    start_on       TEXT NOT NULL,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    seasonal       INTEGER NOT NULL DEFAULT 0 CHECK (seasonal IN (0, 1)),
    moisture_check INTEGER NOT NULL DEFAULT 0 CHECK (moisture_check IN (0, 1)),
    water_at       INTEGER NOT NULL DEFAULT 3 CHECK (water_at BETWEEN 1 AND 9)
);
INSERT INTO care_tasks_new (id, plant_id, kind, label, interval_days, anchor, start_on, created_at, updated_at, seasonal, moisture_check, water_at)
    SELECT id, plant_id, kind, label, interval_days, anchor, start_on, created_at, updated_at, seasonal, moisture_check, water_at FROM care_tasks;
DROP TABLE care_tasks;
ALTER TABLE care_tasks_new RENAME TO care_tasks;
CREATE INDEX care_tasks_plant_id ON care_tasks (plant_id);

-- Sizes are in inches. photo_id links the photo taken with it (e.g. during a check-in).
CREATE TABLE plant_measurements (
    id          TEXT PRIMARY KEY,
    plant_id    TEXT NOT NULL REFERENCES plants (id) ON DELETE CASCADE,
    measured_on TEXT NOT NULL,
    height_in   REAL CHECK (height_in > 0 AND height_in <= 600),
    width_in    REAL CHECK (width_in > 0 AND width_in <= 600),
    stage       TEXT CHECK (stage IN ('seedling', 'young', 'mature')),
    note        TEXT NOT NULL DEFAULT '',
    photo_id    TEXT REFERENCES photos (id) ON DELETE SET NULL,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    CHECK (height_in IS NOT NULL OR width_in IS NOT NULL OR stage IS NOT NULL OR note <> '' OR photo_id IS NOT NULL)
);
CREATE INDEX plant_measurements_plant_id_measured_on ON plant_measurements (plant_id, measured_on);

COMMIT;

-- No PRAGMA foreign_key_check: goose discards its rows, so it could not stop anything. The
-- copy keeps every id, and migrate_journal_test.go checks that the keys still hold.
PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

BEGIN;

DROP TABLE plant_measurements;
DELETE FROM care_events WHERE task_id IN (SELECT id FROM care_tasks WHERE kind = 'check_in');
DELETE FROM care_tasks WHERE kind = 'check_in';
CREATE TABLE care_tasks_old (
    id             TEXT PRIMARY KEY,
    plant_id       TEXT NOT NULL REFERENCES plants (id) ON DELETE CASCADE,
    kind           TEXT NOT NULL CHECK (kind IN ('water', 'fertilize', 'mist', 'repot', 'rotate', 'custom')),
    label          TEXT NOT NULL DEFAULT '',
    interval_days  INTEGER NOT NULL CHECK (interval_days BETWEEN 1 AND 365),
    anchor         TEXT NOT NULL DEFAULT 'from_completion' CHECK (anchor IN ('from_completion', 'fixed')),
    start_on       TEXT NOT NULL,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    seasonal       INTEGER NOT NULL DEFAULT 0 CHECK (seasonal IN (0, 1)),
    moisture_check INTEGER NOT NULL DEFAULT 0 CHECK (moisture_check IN (0, 1)),
    water_at       INTEGER NOT NULL DEFAULT 3 CHECK (water_at BETWEEN 1 AND 9)
);
INSERT INTO care_tasks_old SELECT id, plant_id, kind, label, interval_days, anchor, start_on, created_at, updated_at, seasonal, moisture_check, water_at FROM care_tasks;
DROP TABLE care_tasks;
ALTER TABLE care_tasks_old RENAME TO care_tasks;
CREATE INDEX care_tasks_plant_id ON care_tasks (plant_id);

COMMIT;

PRAGMA foreign_keys = ON;
