# Architecture

The API contract in [`api/openapi.yaml`](../api/openapi.yaml) and the migrations in `internal/db/migrations` are the source of truth; this document explains the reasoning behind them.
Code comments refer to the sections below by name.

## Stack

| Layer        | Choice                                                                            | Reason                                                                 |
| ------------ | --------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| Language     | Go                                                                                | I like Go ¯\\\_(ツ)\_/¯                                                |
| HTTP         | stdlib `net/http` (method/path routing)                                           | No router dependency.                                                  |
| API contract | `api/openapi.yaml` → `oapi-codegen` (strict server interfaces)                    | Server interfaces and client types are generated from one spec.       |
| DB driver    | `modernc.org/sqlite`                                                              | Pure Go, no CGO; static binary, distroless image, simple cross-builds. |
| Queries      | `sqlc`                                                                            | Typed Go from plain SQL.                                               |
| Migrations   | `goose`, embedded with `embed.FS`, applied at startup                             | Versioned SQL migrations shipped in the binary.                        |
| Frontend     | React + Vite + TypeScript, TanStack Query, React Router                           |                                                                        |
| API client   | `openapi-typescript` + `openapi-fetch`                                            | Generated types, small runtime.                                        |
| Styling      | CSS custom-property tokens + CSS Modules; Radix primitives for dialogs and sheets | Accessible overlays without a component framework.                     |
| PWA          | `vite-plugin-pwa` (injectManifest, hand-written service worker)                   | Push and notification-click handling stay in our code.                 |
| Push         | `webpush-go`                                                                      | VAPID web push, pure Go.                                               |
| Images       | stdlib `image/jpeg`, `golang.org/x/image/draw`, own EXIF orientation reader       | Pure Go. No pure-Go WebP/AVIF encoder, so thumbnails are JPEG. HEIC is converted to JPEG in the browser. |
| Logging      | `log/slog`                                                                        |                                                                        |

## Data

- Migrations: goose up/down files, applied on boot in a transaction.
  The binary refuses to start against a newer database.
- Pragmas: `journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000`, `synchronous=NORMAL`.
  One writer connection, a read pool.
- IDs are text ULIDs generated in Go.
  Timestamps are `TEXT` RFC 3339 UTC.
  Care dates are civil `YYYY-MM-DD`.
  Queries avoid SQLite-only functions.
- Plants, locations and care data are household-wide.
  All users share them.
- `care_events` is append-only (`done`, `skipped`, `snoozed`, `checked`).
  Undo sets `voided_at`; the scheduler ignores voided rows.
  Each event stores its `local_date`, so a later time zone change does not alter history.

Data directory:

```
/data
  grower.db (+ -wal, -shm)
  media/photos/<id>/     originals (downscaled in the browser) and JPEG thumbnails
  backups/               nightly VACUUM INTO copies, newest 14 kept
  vapid.json             web push key pair, created on first start
```

## Scheduling

`internal/schedule` computes due dates from a task and its event log.
It has no I/O and does not call `time.Now`.
Snooze and skip are events, so next-due is a function of (task, events, today).

### Core types

```go
type Task  struct { Interval int; Anchor AnchorMode; StartOn Date; Paused bool; Seasonal bool; Kind string }
type Event struct { Kind EventKind; LocalDate Date; SnoozeUntil *Date; Voided bool }

func NextDue(t Task, events []Event, h Hemisphere) Date
func CurrentInterval(t Task, today Date, h Hemisphere) int
func Today(now time.Time, loc *time.Location) Date // the only time-zone-aware function
```

### Scheduling rules

1. Voided events are ignored.
   Events are ordered by `LocalDate`, then insertion order.
2. `last` is the date of the latest `done` or `skipped` event.
   With none, `due = StartOn`.
3. Anchor `from_completion` (default): `due = last + effective interval`.
   Anchor `fixed`: the smallest `StartOn + k·interval` after `last`.
4. Snooze: if the latest event is `snoozed` and newer than `last`, `due = max(due, SnoozeUntil)`.
   A later done or skip clears it.
   Snooze does not move the anchor.
5. Paused tasks have no due date.
6. No catch-up: an overdue task appears once, overdue by `today − due` days.
7. Due: `due ≤ today`.
   Overdue: `due < today`.
8. Seasonal interval: `max(1, round(interval × factor(kind, month of last, hemisphere)))`, for seasonal tasks with anchor `from_completion` only.
   Using the month of `last` keeps a pending due date stable across a month boundary.
   Factors in percent, January to December:
   - water, mist: 150 140 120 100 100 90 85 90 100 120 140 150
   - fertilize: 200 200 125 100 100 100 100 100 125 150 200 200
   - other kinds: 100

   The southern hemisphere shifts the curves by six months.

### Time zones and DST

- Only `Today()` uses `time.Location`.
  Date arithmetic is done on civil dates at noon UTC, so 23- and 25-hour days do not shift due dates.
- The daily reminder time is resolved per day in the configured zone.
  A time in a spring-forward gap moves forward.
  A repeated fall-back hour sends once; the job is keyed `digest:<user>:<local date>`.
- A reminder sent before the user's later time (`later_time`, default 17:30) carries its local date and that time, and the notification offers "Remind me at 5:30 PM".
  The action calls `POST /push/remind-later`, which queues a second digest job keyed `digest-later:<user>:<local date>` at the later time, so repeated taps queue it once.
  Only today's reminder can be postponed, and only before the later time.
  The second reminder recomputes what is due, offers no action, and is skipped when nothing is left.

### Moisture-check watering

A water task with `moisture_check` is a soil check.
Its due date is the next check; readings on a 1–10 meter scale (dry 1–3, moist 4–7, wet 8–10) decide when to water.
`water_at` (default 3) is the threshold: a reading at or below it means water.
Check tasks always use `from_completion`, and seasonal factors do not apply.

`checked` events require a reading.
`done` events may carry the reading taken before watering.

Learned drying time (`drying_days`):
1. Each watering (`done`) on date W starts a cycle.
2. The cycle's observation is the first reading ≤ `water_at` after W (from a `checked` or the next `done`), otherwise the next `done`'s date.
3. D = observation − W; values under 1 are discarded.
   `drying_days` is the median of the last 3 (lower median for even counts), or null.
4. `learned` = `drying_days` if known, else `interval_days`.

Next check (voids, ordering, snooze and pause as above):
- No events: `StartOn`.
- Latest is `done` or `skipped` on L: L + `learned`.
- Latest is `checked` on C with reading r:
  - r ≤ `water_at`: C + 1.
  - Otherwise, with W the last watering before C: if C > W and r < 10, rate = (10 − r) / (C − W) points per day and remaining = ceil((r − `water_at`) / rate).
    Else remaining = max(1, `learned` − (C − W)) if W exists, otherwise 2.
  - Due = C + clamp(remaining, 1, 14).

`current_interval_days` for a check task is `learned`.
The UI labels these tasks "Check soil".

## Light environment

`internal/light` is mirrored in `web/src/light/rules.ts`; both are pure and table-tested.
Levels: low < medium < bright_indirect < direct.

- Suggested room light, from windows and hemisphere (best window wins):
  - Equator-facing (S in the north, N in the south): direct.
  - E, W and equator-side diagonals: bright_indirect.
  - Pole-facing: medium.
    Two or more windows raise medium to bright_indirect.
  - No windows: null.
- Room light: the location's `light` if set, else the suggestion.
- Plant light, first match:
  1. The plant's own `light`.
  2. Room light lowered by `window_distance` (at window 0 steps, near 1, across room 2), minimum low.
  3. A grow light raises the result to at least bright_indirect (≥ 10 h/day) or medium (< 10 h/day).
  4. Otherwise null.
- Species hint: shown when the linked species' light is two or more steps above the plant's.

## Auth and operations

- **Accounts:** the first one through the first-run setup page, which only works while there are no users and for 10 minutes after startup; its check that no user exists and the insert share one write transaction, so racing requests create one account.
  Further accounts come from `grower user create`; there is no signup endpoint. argon2id password hashes.
  In-memory login rate limits per IP and globally.
- **Sessions:** 32-byte random token, stored as SHA-256.
  Cookie `__Host-grower_session` over https, `grower_session` over plain http (local development); `HttpOnly; Secure; SameSite=Lax`.
  90-day sliding expiry.
- **CSRF:** SameSite=Lax.
  Mutating requests must match `GROWER_BASE_URL`'s origin (or send `Sec-Fetch-Site: same-origin` without an Origin header) and use JSON bodies.
  Multipart endpoints (photo upload, identification) rely on the origin checks only.
- **Proxies:** `X-Forwarded-For` and `X-Forwarded-Proto` are trusted only from `GROWER_TRUSTED_PROXIES`.
  `GROWER_BASE_URL` determines absolute URLs and cookie security.
- **Jobs:** a ticker claims due rows from `jobs`.
  Producers insert with a `dedupe_key`; failures back off.
  Push is sent before the job is marked done: a crash can cause a duplicate reminder, not a missed one.
- **Backups:** nightly `VACUUM INTO` to `backups/`.
  Snapshots of the live WAL database are crash-consistent; plain file copies are not.
  Restore from `backups/` ([backup.md](backup.md)).
- **Species providers:** implement `Provider`.
  Optional, called on demand only, with daily budgets below each free tier.

## Tests

| Layer           | Tool                                                   | Scope                                                      |
| --------------- | ------------------------------------------------------ | ---------------------------------------------------------- |
| Unit            | `go test`                                              | Scheduling (table-driven), light, auth, media, export      |
| API integration | `go test` + `httptest` + temp SQLite with migrations   | Each endpoint: happy path, auth, validation                |
| Frontend unit   | Vitest + Testing Library                               | Client logic and screens                                   |
| End to end      | Playwright (mobile Chromium) against the built binary  | Login, Today, photos, quick add, journal                   |
