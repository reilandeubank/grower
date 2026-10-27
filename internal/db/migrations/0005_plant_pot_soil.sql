-- +goose Up
-- Pot and soil details. All optional: NULL means "not recorded".

ALTER TABLE plants ADD COLUMN pot_type TEXT
    CHECK (pot_type IN ('plastic', 'terracotta', 'ceramic', 'fabric', 'self_watering', 'other'));
ALTER TABLE plants ADD COLUMN pot_size_in REAL CHECK (pot_size_in > 0 AND pot_size_in <= 48);
ALTER TABLE plants ADD COLUMN drainage INTEGER CHECK (drainage IN (0, 1));
ALTER TABLE plants ADD COLUMN soil TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE plants DROP COLUMN soil;
ALTER TABLE plants DROP COLUMN drainage;
ALTER TABLE plants DROP COLUMN pot_size_in;
ALTER TABLE plants DROP COLUMN pot_type;
