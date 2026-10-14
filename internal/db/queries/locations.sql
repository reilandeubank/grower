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
