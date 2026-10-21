-- name: ListLocations :many
SELECT l.id, l.name, l.light, l.notes,
       (SELECT COUNT(*) FROM plants p WHERE p.location_id = l.id AND p.archived_at IS NULL) AS plant_count
FROM locations l
ORDER BY l.name COLLATE NOCASE;

-- name: GetLocation :one
SELECT l.id, l.name, l.light, l.notes,
       (SELECT COUNT(*) FROM plants p WHERE p.location_id = l.id AND p.archived_at IS NULL) AS plant_count
FROM locations l
WHERE l.id = ?;

-- name: CreateLocation :exec
INSERT INTO locations (id, name, light, notes, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: UpdateLocation :execrows
UPDATE locations SET name = ?, light = ?, notes = ?, updated_at = ? WHERE id = ?;

-- name: DeleteLocation :execrows
DELETE FROM locations WHERE id = ?;

-- Every location's set light, for deriving plants' effective light.
-- name: ListLocationLights :many
SELECT id, light FROM locations;

-- Every location's windows; callers group them by location and order them by direction.
-- name: ListWindows :many
SELECT * FROM location_windows;

-- name: ListLocationWindows :many
SELECT * FROM location_windows WHERE location_id = ?;

-- name: DeleteLocationWindows :exec
DELETE FROM location_windows WHERE location_id = ?;

-- name: CreateLocationWindow :exec
INSERT INTO location_windows (location_id, direction, count) VALUES (?, ?, ?);
