-- name: CreateTask :one
INSERT INTO care_tasks (id, plant_id, kind, label, interval_days, anchor, start_on, seasonal, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTask :one
SELECT * FROM care_tasks WHERE id = ?;

-- name: UpdateTask :one
UPDATE care_tasks
SET kind = ?, label = ?, interval_days = ?, anchor = ?, start_on = ?, seasonal = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteTask :execrows
DELETE FROM care_tasks WHERE id = ?;

-- name: ListPlantTasks :many
SELECT * FROM care_tasks WHERE plant_id = ? ORDER BY kind, label, id;

-- Tasks of active plants with their plant, for Today.
-- name: ListActiveTasks :many
SELECT sqlc.embed(care_tasks), sqlc.embed(plants)
FROM care_tasks JOIN plants ON plants.id = care_tasks.plant_id
WHERE plants.archived_at IS NULL
ORDER BY plants.name COLLATE NOCASE, care_tasks.kind, care_tasks.label, care_tasks.id;

-- Scheduling input: non-voided events in insertion order.
-- name: ListTaskEvents :many
SELECT * FROM care_events
WHERE task_id = ? AND voided_at IS NULL
ORDER BY created_at, id;

-- The same for every task of an active plant, so Today needs one query, not one per task.
-- name: ListActiveEvents :many
SELECT care_events.* FROM care_events
JOIN care_tasks ON care_tasks.id = care_events.task_id
JOIN plants ON plants.id = care_tasks.plant_id
WHERE care_events.voided_at IS NULL AND plants.archived_at IS NULL
ORDER BY care_events.created_at, care_events.id;

-- name: ListPlantHistory :many
SELECT sqlc.embed(care_events), care_tasks.kind AS task_kind, care_tasks.label AS task_label
FROM care_events JOIN care_tasks ON care_tasks.id = care_events.task_id
WHERE care_tasks.plant_id = ? AND care_events.voided_at IS NULL
ORDER BY care_events.local_date DESC, care_events.created_at DESC, care_events.id DESC
LIMIT ?;

-- name: CreateEvent :one
INSERT INTO care_events (id, task_id, kind, local_date, snooze_until, note, occurred_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- Undo keeps the event but ignores it from now on. Voiding twice keeps the first time.
-- name: VoidEvent :one
UPDATE care_events SET voided_at = COALESCE(voided_at, sqlc.arg(voided_at))
WHERE id = sqlc.arg(id)
RETURNING task_id;
