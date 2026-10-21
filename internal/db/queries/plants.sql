-- name: ListPlants :many
SELECT * FROM plants
WHERE (archived_at IS NOT NULL) = CAST(sqlc.arg(archived) AS BOOLEAN)
ORDER BY name COLLATE NOCASE;

-- name: GetPlant :one
SELECT * FROM plants WHERE id = ?;

-- name: CreatePlant :one
INSERT INTO plants (id, name, species, species_id, location_id, acquired_on, notes, pot_type, pot_size_in, drainage, soil,
                    light, window_distance, grow_light_hours, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- Archiving stamps archived_at with the update time but keeps an existing stamp.
-- name: UpdatePlant :one
UPDATE plants
SET name = sqlc.arg(name), species = sqlc.arg(species), species_id = sqlc.narg(species_id), location_id = sqlc.narg(location_id),
    acquired_on = sqlc.narg(acquired_on), notes = sqlc.arg(notes), pot_type = sqlc.narg(pot_type),
    pot_size_in = sqlc.narg(pot_size_in), drainage = sqlc.narg(drainage), soil = sqlc.arg(soil),
    light = sqlc.narg(light), window_distance = sqlc.narg(window_distance), grow_light_hours = sqlc.narg(grow_light_hours),
    updated_at = sqlc.arg(updated_at),
    archived_at = CASE WHEN CAST(sqlc.arg(archived) AS BOOLEAN) THEN COALESCE(archived_at, sqlc.arg(updated_at)) END
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeletePlant :execrows
DELETE FROM plants WHERE id = ?;
