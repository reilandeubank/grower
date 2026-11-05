-- name: CreateTask :one
INSERT INTO care_tasks (id, plant_id, kind, label, interval_days, anchor, start_on, seasonal, moisture_check, water_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTask :one
SELECT * FROM care_tasks WHERE id = ?;

-- name: UpdateTask :one
UPDATE care_tasks
SET kind = ?, label = ?, interval_days = ?, anchor = ?, start_on = ?, seasonal = ?, moisture_check = ?, water_at = ?, updated_at = ?
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

-- The events of active plants' tasks that their schedules depend on (care.Schedule), so
-- Today reads a few per task instead of the whole log. "Latest" is in the order schedules
-- sort events (by date, then insertion), which care_events_task_order covers, so each
-- lookup walks back from a task's newest event and stops at the first match. Every task
-- keeps its latest event (checks count only on soil-check tasks), completion, done and
-- reading. A soil-check task keeps every event from the day of its fourth-latest watering,
-- or of its latest reading if that's earlier: enough for its drying cycles (care.Today
-- checks, and loads the whole log when it isn't). Insertion order, like ListTaskEvents.
-- name: ListScheduleEvents :many
WITH task AS (
    SELECT care_tasks.id, care_tasks.moisture_check FROM care_tasks
    JOIN plants ON plants.id = care_tasks.plant_id
    WHERE plants.archived_at IS NULL
),
latest AS (
    SELECT (SELECT e.id FROM care_events e
            WHERE e.task_id = task.id AND e.voided_at IS NULL AND (e.kind <> 'checked' OR task.moisture_check = 1)
            ORDER BY e.local_date DESC, e.created_at DESC, e.id DESC LIMIT 1) AS id
    FROM task
    UNION ALL
    SELECT (SELECT e.id FROM care_events e
            WHERE e.task_id = task.id AND e.voided_at IS NULL AND e.kind IN ('done', 'skipped')
            ORDER BY e.local_date DESC, e.created_at DESC, e.id DESC LIMIT 1)
    FROM task
    UNION ALL
    SELECT (SELECT e.id FROM care_events e
            WHERE e.task_id = task.id AND e.voided_at IS NULL AND e.kind = 'done'
            ORDER BY e.local_date DESC, e.created_at DESC, e.id DESC LIMIT 1)
    FROM task
    UNION ALL
    SELECT (SELECT e.id FROM care_events e
            WHERE e.task_id = task.id AND e.voided_at IS NULL AND e.reading IS NOT NULL
            ORDER BY e.local_date DESC, e.created_at DESC, e.id DESC LIMIT 1)
    FROM task
),
soil AS (
    SELECT task.id,
           -- '' (before every date) when there are fewer than four waterings: the whole log.
           MIN(
               COALESCE((SELECT e.local_date FROM care_events e
                         WHERE e.task_id = task.id AND e.voided_at IS NULL AND e.kind = 'done'
                         ORDER BY e.local_date DESC, e.created_at DESC, e.id DESC LIMIT 1 OFFSET 3), ''),
               COALESCE((SELECT e.local_date FROM care_events e
                         WHERE e.task_id = task.id AND e.voided_at IS NULL AND e.reading IS NOT NULL
                         ORDER BY e.local_date DESC, e.created_at DESC, e.id DESC LIMIT 1), '9999-12-31')
           ) AS since
    FROM task
    WHERE task.moisture_check = 1
)
SELECT care_events.* FROM care_events
WHERE care_events.id IN (SELECT id FROM latest)
UNION
SELECT care_events.* FROM soil
JOIN care_events ON care_events.task_id = soil.id AND care_events.local_date >= soil.since
WHERE care_events.voided_at IS NULL
ORDER BY created_at, id;

-- name: ListPlantHistory :many
SELECT sqlc.embed(care_events), care_tasks.kind AS task_kind, care_tasks.label AS task_label
FROM care_events JOIN care_tasks ON care_tasks.id = care_events.task_id
WHERE care_tasks.plant_id = ? AND care_events.voided_at IS NULL
ORDER BY care_events.local_date DESC, care_events.created_at DESC, care_events.id DESC
LIMIT ?;

-- name: CreateEvent :one
INSERT INTO care_events (id, task_id, kind, local_date, snooze_until, reading, note, occurred_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- Undo keeps the event but ignores it from now on. Voiding twice keeps the first time.
-- name: VoidEvent :one
UPDATE care_events SET voided_at = COALESCE(voided_at, sqlc.arg(voided_at))
WHERE id = sqlc.arg(id)
RETURNING task_id;

-- The task "apply species care" changes: the plant's oldest water task.
-- name: FirstWaterTask :one
SELECT * FROM care_tasks WHERE plant_id = ? AND kind = 'water' ORDER BY created_at, id LIMIT 1;
