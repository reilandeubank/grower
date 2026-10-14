-- +goose Up
-- Photo files live under <data>/media/photos/<id>/ (original + small.jpg + medium.jpg);
-- this table is their index. A re-upload of the same bytes to the same plant is a no-op.

CREATE TABLE photos (
    id         TEXT PRIMARY KEY,
    plant_id   TEXT NOT NULL REFERENCES plants (id) ON DELETE CASCADE,
    taken_at   TEXT NOT NULL,
    caption    TEXT NOT NULL DEFAULT '',
    mime       TEXT NOT NULL CHECK (mime IN ('image/jpeg', 'image/png')),
    width      INTEGER NOT NULL,
    height     INTEGER NOT NULL,
    bytes      INTEGER NOT NULL,
    sha256     TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (plant_id, sha256)
);
CREATE INDEX photos_plant_id_taken_at ON photos (plant_id, taken_at);

ALTER TABLE plants ADD COLUMN cover_photo_id TEXT REFERENCES photos (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE plants DROP COLUMN cover_photo_id;
DROP TABLE photos;
