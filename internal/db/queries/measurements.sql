-- name: CreateMeasurement :one
INSERT INTO plant_measurements (id, plant_id, measured_on, height_in, width_in, stage, note, photo_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetMeasurement :one
SELECT * FROM plant_measurements WHERE id = ?;

-- name: UpdateMeasurement :one
UPDATE plant_measurements
SET measured_on = ?, height_in = ?, width_in = ?, stage = ?, note = ?, photo_id = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteMeasurement :execrows
DELETE FROM plant_measurements WHERE id = ?;

-- name: ListPlantMeasurements :many
SELECT * FROM plant_measurements WHERE plant_id = ?
ORDER BY measured_on DESC, created_at DESC, id DESC;

-- Each plant's latest size and stage, each from the most recent measurement that has
-- it, in one query for every plant (or just one, given plant_id), so lists stay N+1-free.
-- name: ListGrowth :many
SELECT m.plant_id, CAST(MAX(m.measured_on) AS TEXT) AS measured_on,
    (SELECT h.height_in FROM plant_measurements h WHERE h.plant_id = m.plant_id AND h.height_in IS NOT NULL
     ORDER BY h.measured_on DESC, h.created_at DESC, h.id DESC LIMIT 1) AS height_in,
    (SELECT w.width_in FROM plant_measurements w WHERE w.plant_id = m.plant_id AND w.width_in IS NOT NULL
     ORDER BY w.measured_on DESC, w.created_at DESC, w.id DESC LIMIT 1) AS width_in,
    (SELECT s.stage FROM plant_measurements s WHERE s.plant_id = m.plant_id AND s.stage IS NOT NULL
     ORDER BY s.measured_on DESC, s.created_at DESC, s.id DESC LIMIT 1) AS stage
FROM plant_measurements m
WHERE m.plant_id = sqlc.narg(plant_id) OR sqlc.narg(plant_id) IS NULL
GROUP BY m.plant_id;
