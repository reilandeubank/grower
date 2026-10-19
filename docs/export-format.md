# Export format (version 1)

`GET /api/v1/export` (Settings → Export data) downloads `grower-export-YYYY-MM-DD.zip`. It holds everything needed to rebuild your data elsewhere, in plain JSON and the original photo files. Import (M9) reads the same format.

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
  "plants": [ ... ],
  "tasks": [ ... ],
  "events": [ ... ],
  "photos": [ ... ]
}
```

Every array holds objects with the same field names and types as the API (`api/openapi.yaml`), except where noted:

| Key | Shape | Notes |
|---|---|---|
| `locations` | `Location` without `plant_count` | |
| `plants` | `Plant` with `cover_photo_id` (string or null) in place of `cover`, and `archived_at` (date-time or null) in place of `archived` | Archived plants are included. |
| `tasks` | `CareTask` without `next_due`, `last_done`, `current_interval_days`, `drying_days` and `last_reading` | Those are derived from events and the date, never stored. Older exports have no `seasonal`; treat it as false. Older exports have no `moisture_check` or `water_at`; treat them as false and 3. |
| `events` | `CareEvent` plus `voided_at` (date-time or null) | All events, including undone ones, so the log is complete. `reading` (1–10 or null) is set on soil checks and optionally on waterings; older exports have no `reading`, so treat it as null. |
| `photos` | `Photo` without `urls`, plus `file` (path inside the zip) and `sha256` | |

Rules:

- IDs are the app's own ULIDs and are stable across exports, so references (`plant_id`, `task_id`, `location_id`, `cover_photo_id`) resolve within the file.
- Dates are `YYYY-MM-DD` civil dates, and timestamps are RFC 3339 in UTC.
- Accounts, password hashes, sessions and push subscriptions are never exported.
- A breaking change bumps `version`. Additive fields don't, so readers must ignore unknown keys.
