# Export format (version 1)

`GET /api/v1/export` (Settings → Export data) downloads `grower-export-YYYY-MM-DD.zip`.
It holds everything needed to rebuild your data elsewhere, in plain JSON and the original photo files.
`grower import` reads the same format (see [Import](#import)).

```
grower-export-2026-10-14.zip
├── export.json
└── photos/
    └── <photo id>.jpg | .png      original files as uploaded (thumbnails are not included; they're regenerated)
```

## export.json

```json
{
  "format": "grower-export",
  "version": 1,
  "exported_at": "2026-10-14T18:30:00Z",
  "app_version": "v0.4.0",
  "settings": { "timezone": "America/Chicago", "digest_time": "08:00", "hemisphere": "north" },
  "locations": [ ... ],
  "species": [ ... ],
  "plants": [ ... ],
  "tasks": [ ... ],
  "events": [ ... ],
  "photos": [ ... ],
  "measurements": [ ... ]
}
```

Every array holds objects with the same field names and types as the API (`api/openapi.yaml`), except where noted:

| Key | Shape | Notes |
|---|---|---|
| `locations` | `Location` without `plant_count` and `suggested_light` | `windows` lists each facing direction once, in compass order (n, ne, e, se, s, sw, w, nw). Older exports have no `windows`; treat it as empty. |
| `species` | `Species` without `plant_count` | The provider payload (`raw`) is not exported. Older exports have no `species`; treat it as empty. |
| `plants` | `Plant` without `effective_light` and `growth`, with `cover_photo_id` (string or null) in place of `cover`, and `archived_at` (date-time or null) in place of `archived` | Archived plants are included. Older exports have no `species_id`, `light`, `window_distance` or `grow_light_hours`; treat them as null. `effective_light` and `growth` are derived, never stored. |
| `tasks` | `CareTask` without `next_due`, `last_done`, `current_interval_days`, `drying_days` and `last_reading` | Those are derived from events and the date, never stored. Older exports have no `seasonal`; treat it as false. Older exports have no `moisture_check` or `water_at`; treat them as false and 3. |
| `events` | `CareEvent` plus `voided_at` (date-time or null) | All events, including undone ones, so the log is complete. `reading` (1–10 or null) is set on soil checks and optionally on waterings; older exports have no `reading`, so treat it as null. |
| `photos` | `Photo` without `urls`, plus `file` (path inside the zip) and `sha256` | |
| `measurements` | `Measurement` with `photo_id` (string or null) in place of `photo` | Sizes are in inches. Older exports have no `measurements`; treat it as empty. |

Rules:

- IDs are the app's own ULIDs and are stable across exports, so references (`plant_id`, `task_id`, `location_id`, `species_id`, `cover_photo_id`, `photo_id`) resolve within the file.
- Dates are `YYYY-MM-DD` civil dates, and timestamps are RFC 3339 in UTC.
- Accounts, password hashes, sessions and push subscriptions are never exported.
- A breaking change bumps `version`.
  Additive fields don't, so readers must ignore unknown keys.

## Import

`grower import [--username NAME] FILE.zip` loads an export into an empty grower (no locations, plants, species, tasks, events, photos or measurements); see [backup.md](backup.md#moving-to-a-new-server).
It:

- reads `export.json` and refuses any `format` other than `grower-export` or any `version` other than 1;
- checks every id and reference listed under Rules, and that each photo's `file` is in the zip with a matching `sha256`.
  A photo listed without its file fails the import: the export itself lacked it (exports skip photo files missing on the server);
- only then writes the photos (thumbnails are regenerated) and inserts every row in one transaction, keeping ids and timestamps.
  Voided events stay voided.
  If the transaction fails, the photos it wrote are removed, so nothing is half-loaded;
- applies `settings` to the user (the only one, or `--username`).

Fields missing from older exports take the defaults listed above, and settings an export lacks keep the user's current values.
Timestamps the format doesn't carry (`created_at` of most rows) come from the ULID ids; `updated_at` is the import time.
The provider payload of species (`raw`) isn't exported, so it starts empty.
