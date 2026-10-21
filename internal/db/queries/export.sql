-- Full-table reads for the export (docs/export-format.md), in ID (creation) order.

-- name: ExportLocations :many
SELECT * FROM locations ORDER BY id;

-- name: ExportPlants :many
SELECT * FROM plants ORDER BY id;

-- name: ExportTasks :many
SELECT * FROM care_tasks ORDER BY id;

-- Voided events too, so the log is complete.
-- name: ExportEvents :many
SELECT * FROM care_events ORDER BY id;

-- name: ExportPhotos :many
SELECT * FROM photos ORDER BY id;

-- name: ExportSpecies :many
SELECT * FROM species ORDER BY id;

-- name: ExportMeasurements :many
SELECT * FROM plant_measurements ORDER BY id;
