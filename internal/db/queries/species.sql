-- Species reads leave out raw (only the export and later features need it) and count
-- active plants. The search is a case-insensitive substring match (no LIKE wildcards to
-- escape); an empty q matches everything.

-- name: ListSpecies :many
SELECT s.id, s.scientific_name, s.common_name, s.source, s.source_ref, s.water_interval_days, s.light, s.water_at,
       s.notes, s.fetched_at, s.user_edited,
       (SELECT COUNT(*) FROM plants p WHERE p.species_id = s.id AND p.archived_at IS NULL) AS plant_count
FROM species s
WHERE instr(lower(s.scientific_name), lower(sqlc.arg(q))) > 0 OR instr(lower(s.common_name), lower(sqlc.arg(q))) > 0
ORDER BY s.scientific_name;

-- name: GetSpecies :one
SELECT s.id, s.scientific_name, s.common_name, s.source, s.source_ref, s.water_interval_days, s.light, s.water_at,
       s.notes, s.fetched_at, s.user_edited,
       (SELECT COUNT(*) FROM plants p WHERE p.species_id = s.id AND p.archived_at IS NULL) AS plant_count
FROM species s
WHERE s.id = ?;

-- Matching is case-insensitive (the column is COLLATE NOCASE).
-- name: GetSpeciesByName :one
SELECT id, user_edited FROM species WHERE scientific_name = ?;

-- name: ListSpeciesNames :many
SELECT id, scientific_name FROM species;

-- name: CreateSpecies :exec
INSERT INTO species (id, scientific_name, common_name, source, source_ref, water_interval_days, light, water_at,
                     notes, raw, fetched_at, user_edited, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- A person's edit: their values win over later imports.
-- name: UpdateSpecies :execrows
UPDATE species
SET scientific_name = ?, common_name = ?, water_interval_days = ?, light = ?, water_at = ?, notes = ?,
    user_edited = 1, updated_at = ?
WHERE id = ?;

-- Re-import of a species nobody has edited: take the provider's values.
-- name: RefreshSpecies :exec
UPDATE species
SET common_name = ?, source = ?, source_ref = ?, water_interval_days = ?, light = ?, water_at = ?,
    raw = ?, fetched_at = ?, updated_at = ?
WHERE id = ?;

-- Re-import of an edited species: keep the person's values, refresh only the payload.
-- name: RefreshSpeciesRaw :exec
UPDATE species SET raw = ?, fetched_at = ?, updated_at = ? WHERE id = ?;
