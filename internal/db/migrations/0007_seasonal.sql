-- +goose Up
-- Seasonal adjustment (M7). On by default for the kinds whose needs follow the seasons;
-- existing tasks of those kinds opt in too. settings.hemisphere already exists (0001).

ALTER TABLE care_tasks ADD COLUMN seasonal INTEGER NOT NULL DEFAULT 0 CHECK (seasonal IN (0, 1));
UPDATE care_tasks SET seasonal = 1 WHERE kind IN ('water', 'mist', 'fertilize');

-- +goose Down
ALTER TABLE care_tasks DROP COLUMN seasonal;
