-- name: CreatePhoto :one
INSERT INTO photos (id, plant_id, taken_at, caption, mime, width, height, bytes, sha256, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetPhoto :one
SELECT * FROM photos WHERE id = ?;

-- name: GetPlantPhotoByHash :one
SELECT * FROM photos WHERE plant_id = ? AND sha256 = ?;

-- name: ListPlantPhotos :many
SELECT * FROM photos WHERE plant_id = ?
ORDER BY taken_at DESC, created_at DESC, id DESC;

-- name: ListPlantPhotoIDs :many
SELECT id FROM photos WHERE plant_id = ?;

-- Every plant's cover in one query, so plant lists stay N+1-free.
-- name: ListCoverPhotos :many
SELECT photos.* FROM photos JOIN plants ON plants.cover_photo_id = photos.id;

-- name: UpdatePhoto :one
UPDATE photos SET caption = ?, taken_at = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeletePhoto :execrows
DELETE FROM photos WHERE id = ?;

-- name: SetPlantCover :one
UPDATE plants SET cover_photo_id = sqlc.narg(cover_photo_id), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
RETURNING *;

-- A new photo becomes the cover only when the plant has none.
-- name: SetCoverIfNone :exec
UPDATE plants SET cover_photo_id = sqlc.arg(photo_id)
WHERE id = sqlc.arg(plant_id) AND cover_photo_id IS NULL;

-- Before a cover is deleted, the newest other photo of its plant (or none) takes over.
-- name: ReplaceCover :exec
UPDATE plants SET cover_photo_id = (
    SELECT p.id FROM photos p
    WHERE p.plant_id = plants.id AND p.id <> sqlc.arg(photo_id)
    ORDER BY p.taken_at DESC, p.created_at DESC, p.id DESC
    LIMIT 1
)
WHERE cover_photo_id = sqlc.arg(photo_id);
