# Changelog

Notable changes in each release.
Versions follow [semantic versioning](https://semver.org/); every release's database migrations run automatically on start, so back up before updating ([docs/update.md](docs/update.md)).

## Unreleased

- First-run setup page: on a fresh install, open grower and create your account, with no `docker exec` needed.
  It's open for 10 minutes after startup; `grower user create` still works and adds further users.
- Postpone the daily reminder: the notification offers "Remind me at 5:30 PM", which sends it again at a later time you choose in Settings.
- Multi-arch images: `linux/arm64` alongside `linux/amd64`.
- Images are no longer tagged `:sha-<commit>`; pin a release instead (existing `:sha-` tags stay available).
- Open-source release: MIT license, generic deployment docs (Docker Compose, any reverse proxy), contributing and security policies.

## 1.7.0 (2026-10-21)

- Add a plant from a photo.
  With a Pl@ntNet API key, grower suggests the species; the photo becomes the cover.
- `grower import` loads an export zip into an empty grower, for moving to a new server.
  It checks the format, every reference and each photo's checksum before writing anything, then imports in one transaction.

## 1.6.0 (2026-10-21)

- Growth journal: dated measurements (height, width, growth stage, note, photo) with a Growth tab, height chart and timeline.
- Check-in reminders: a task kind that opens a quick sheet for a photo, size, stage and note.

## 1.5.0 (2026-10-21)

- Locations record their windows by direction, and grower suggests the room's light from them (by hemisphere).
- Plants record distance from the window, an optional grow light and an optional light level of their own.
  The plant page shows the result and hints when a linked species likely needs more light.

## 1.4.0 (2026-10-19)

- Saved species with editable care defaults (watering interval, light, water-at level).
- Optional lookup in Open Plantbook and Perenual, on request only, cached and within the free tiers' daily limits.
  A care guide card applies a species' defaults to the plant's watering task.

## 1.3.0 (2026-10-19)

- Moisture-check watering: a watering task can check the soil first.
  Readings on a 1–10 meter scale decide whether to water, and grower learns each plant's drying time to schedule the next check.

## 1.2.0 (2026-10-19)

- Seasonal adjustment: built-in monthly curves stretch watering, misting and fertilizing in winter and shorten watering a little in summer, by hemisphere.
  Per-task switch, on by default for water, mist and fertilize.

## 1.1.0 (2026-10-14)

- Daily care reminder via web push, at a time you choose, only when something is due.
  Settings can send a test notification.

## 1.0.0 (2026-10-14)

- Export everything as a zip ([documented format](docs/export-format.md)).
- Nightly consistent database backups, keeping the newest 14.
- Settings for theme, time zone, password and storage use.
- Pot and soil details on plants.
- Read-only offline mode with cached Today and photos.
- Accessibility and Android back-button fixes.

## 0.3.0 (2026-10-14)

- Photos: upload from the phone (resized in the browser), thumbnails, cover photos, a month-grouped timeline and a photo viewer.

## 0.2.0 (2026-10-14)

- Plants and locations, with archive, restore and a location filter.
- Care tasks (water, fertilize, mist, repot, rotate, custom) counted from completion or on a fixed calendar, with snooze, skip and undo.
- Today: overdue, due and coming up, with one-tap done.

## 0.1.0 (2026-10-14)

First release.

- Login with long-lived sessions; accounts are created from the command line.
- Installable PWA with an app shell and an empty Today screen.
- Single container image with SQLite, health check and automatic migrations.
