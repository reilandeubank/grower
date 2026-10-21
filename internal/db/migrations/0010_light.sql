-- +goose Up
-- Light environment (PLAN §10 ideas 1, 2, 9): windows per location, and per-plant light,
-- distance from the window and grow light. Effective light is derived, never stored.

CREATE TABLE location_windows (
    location_id TEXT NOT NULL REFERENCES locations (id) ON DELETE CASCADE,
    direction   TEXT NOT NULL CHECK (direction IN ('n', 'ne', 'e', 'se', 's', 'sw', 'w', 'nw')),
    count       INTEGER NOT NULL CHECK (count BETWEEN 1 AND 9),
    PRIMARY KEY (location_id, direction)
);

ALTER TABLE plants ADD COLUMN light TEXT CHECK (light IN ('low', 'medium', 'bright_indirect', 'direct'));
ALTER TABLE plants ADD COLUMN window_distance TEXT CHECK (window_distance IN ('at_window', 'near', 'across_room'));
ALTER TABLE plants ADD COLUMN grow_light_hours INTEGER CHECK (grow_light_hours BETWEEN 1 AND 24);

-- +goose Down
ALTER TABLE plants DROP COLUMN grow_light_hours;
ALTER TABLE plants DROP COLUMN window_distance;
ALTER TABLE plants DROP COLUMN light;
DROP TABLE location_windows;
