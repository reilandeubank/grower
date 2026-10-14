-- name: ListPlants :many
SELECT * FROM plants
WHERE (archived_at IS NOT NULL) = CAST(sqlc.arg(archived) AS BOOLEAN)
ORDER BY name COLLATE NOCASE;

-- name: GetPlant :one
SELECT * FROM plants WHERE id = ?;

-- name: CreatePlant :one
INSERT INTO plants (id, name, species, location_id, acquired_on, notes, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- Archiving stamps archived_at with the update time but keeps an existing stamp.
-- name: UpdatePlant :one
UPDATE plants
SET name = sqlc.arg(name), species = sqlc.arg(species), location_id = sqlc.narg(location_id),
    acquired_on = sqlc.narg(acquired_on), notes = sqlc.arg(notes), updated_at = sqlc.arg(updated_at),
    archived_at = CASE WHEN CAST(sqlc.arg(archived) AS BOOLEAN) THEN COALESCE(archived_at, sqlc.arg(updated_at)) END
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeletePlant :execrows
DELETE FROM plants WHERE id = ?;
