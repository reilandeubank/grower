-- +goose Up
-- Plants and locations are household-wide, not per user; a second user shares everything.

CREATE TABLE locations (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE COLLATE NOCASE,
    light      TEXT CHECK (light IN ('low', 'medium', 'bright_indirect', 'direct')),
    notes      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- acquired_on is a civil date (YYYY-MM-DD). Deleting a location leaves its plants unplaced.
CREATE TABLE plants (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    species     TEXT NOT NULL DEFAULT '',
    location_id TEXT REFERENCES locations (id) ON DELETE SET NULL,
    acquired_on TEXT,
    notes       TEXT NOT NULL DEFAULT '',
    archived_at TEXT,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
CREATE INDEX plants_location_id ON plants (location_id);

-- +goose Down
DROP TABLE plants;
DROP TABLE locations;
