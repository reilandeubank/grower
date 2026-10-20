-- +goose Up
-- Species with care defaults (M8). Rows come from an online provider (openplantbook or
-- perenual) or are made by hand; the user can edit any of them. raw keeps the provider's
-- payload so later features can use more of it without fetching again.

CREATE TABLE species (
    id                  TEXT PRIMARY KEY,
    scientific_name     TEXT NOT NULL UNIQUE COLLATE NOCASE,
    common_name         TEXT NOT NULL DEFAULT '',
    source              TEXT NOT NULL CHECK (source IN ('openplantbook', 'perenual', 'manual')),
    source_ref          TEXT NOT NULL DEFAULT '',
    water_interval_days INTEGER CHECK (water_interval_days BETWEEN 1 AND 365),
    light               TEXT CHECK (light IN ('low', 'medium', 'bright_indirect', 'direct')),
    water_at            INTEGER CHECK (water_at BETWEEN 1 AND 9),
    notes               TEXT NOT NULL DEFAULT '',
    raw                 TEXT NOT NULL DEFAULT '{}',
    fetched_at          TEXT,
    user_edited         INTEGER NOT NULL DEFAULT 0 CHECK (user_edited IN (0, 1)),
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

ALTER TABLE plants ADD COLUMN species_id TEXT REFERENCES species (id) ON DELETE SET NULL;
CREATE INDEX plants_species_id ON plants (species_id);

-- +goose Down
DROP INDEX plants_species_id;
ALTER TABLE plants DROP COLUMN species_id;
DROP TABLE species;
