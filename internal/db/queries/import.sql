-- Writes for `grower import` (docs/export-format.md): whole rows, keeping their ids and
-- timestamps.

-- User content an import must not mix with. Users, settings and sessions don't count.
-- name: CountContent :one
SELECT (SELECT COUNT(*) FROM locations) + (SELECT COUNT(*) FROM plants) + (SELECT COUNT(*) FROM species)
     + (SELECT COUNT(*) FROM care_tasks) + (SELECT COUNT(*) FROM care_events) + (SELECT COUNT(*) FROM photos)
     + (SELECT COUNT(*) FROM plant_measurements) AS n;

-- name: ListUsers :many
SELECT id, username FROM users ORDER BY username;

-- The cover is set once the plant's photos exist.
-- name: ImportPlant :exec
INSERT INTO plants (id, name, species, species_id, location_id, acquired_on, notes, pot_type, pot_size_in, drainage, soil,
                    light, window_distance, grow_light_hours, archived_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ImportEvent :exec
INSERT INTO care_events (id, task_id, kind, local_date, snooze_until, reading, note, occurred_at, voided_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
