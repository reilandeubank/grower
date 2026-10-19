# Progress and checkpoints

The running log for picking work back up. Newest checkpoint first. Each checkpoint says what's done, what's in flight, and the exact next step.
Milestone definitions live in `PLAN.md` §2.

## Status board

| Milestone | Status |
|---|---|
| M1 Deployed skeleton + auth | done (deployed 2026-10-14) |
| M2 Plants and locations | done |
| M3 Care tasks + scheduling + Today | done (v0.2.0) |
| M4 Photos | done (v0.3.0) |
| M5 MVP hardening, v1.0 | done (v1.0.0) |
| M6 Web push (daily reminder) | done (v1.1.0) |
| M7 Seasonal adjustment | done (v1.2.0) |
| Moisture-check watering (§5.5, backlog 6) | done (v1.3.0) |
| M8 Species | not started |
| M9 Import + quick-add via photo | not started |

### M1 checklist
- [x] Repo, conventions (`AGENTS.md`), API contract for auth (`api/openapi.yaml`), logo (`assets/brand/`)
- [x] Design system foundations in Claude Design (tokens, README, cover)
- [x] Backend: config, db + migrations, auth/sessions, API handlers, SPA embed, CLI, tests
- [x] Frontend: Vite app, tokens.css, M1 components, login, app shell, Today empty state, PWA manifest + icons
- [x] Dockerfile, compose file, CI (test) and release (GHCR) workflows; Nix devshell
- [x] Docs: deploy (TrueNAS + Nginx Proxy Manager), update, local dev
- [x] First image published to GHCR (`ghcr.io/reilandeubank/grower:v0.1.0`, `:main`)
- [x] Deployed on TrueNAS behind NPM; logged in from the installed PWA

## Owner action items
- Snapshot, then update to v1.3.0 (migration 0008 rebuilds the care history table; your events are kept).
- Switch watering tasks to "Check the soil first" where you use the meter, and set each plant's water-at level (e.g. 2 for succulents, 4 for ferns).
- If not done yet: Settings → Notifications → Daily reminder.

## Checkpoints

### 2026-10-19 · Moisture frontend (PLAN §5.5)
- Done: `design/MoistureScale` (1–10 radio group: roving tab stop, arrows/Home/End, `aria-label` "3, dry"; zone colours dry `fertilize`, moist `brand`, wet `water`; 2 rows of 5 on phones, one row from 520px via a container query; optional threshold tick + "Water at 3 or below"; `max` caps it). `TaskIcon`/`taskName` take `check` (Gauge, "Check soil"); `TaskRow` `check` makes the round check open the sheet. `care/CheckSoil`: pick a reading and the verdict shows at once (`verdict()` in copy.ts, ≤ `water_at` waters); Watered = `done`+reading, Not now / Save reading = `checked`, "Watered without checking" = `done`; on Today it opens inside TaskActionsSheet ("Snooze or skip" switches to the usual actions), on the Care tab in its own sheet. `useLogEvent` sends `reading`; the checked toast names the next check from the returned task ("Saved reading 6 · Moist for Monstera. Next check Fri, Oct 23."). TaskSheet: for Water, "How do you decide when to water?" (On a schedule / Check the soil first); check mode relabels the interval "Check about every … days", adds "Water at or below" (1–9, "3 · Dry"), hides the anchor and seasons, sends `from_completion`. Care tab: "Usually dries out in N days" or "Check about every N days · learning from your readings", "Last reading 6 · Moist, Oct 18"; history "Checked soil · 6 Moist", "Watered · read 2 Dry".
- Tests: MoistureScale, verdict/zone copy edges, TaskSheet mode mapping, CheckSoil flow on Today (mocked API); e2e `water-check.spec.ts` (clientIP 5, runs last). lint, typecheck, 112 unit tests, build and `make e2e` (7) pass.
- Next: Today's "All caught up" line reads "Next: Check soil Monstera …"; a "for" would read better.

### 2026-10-19 · Moisture backend (PLAN §5.5)
- Done: `internal/schedule/moisture.go` (`Checked`, `Task.MoistureCheck`/`WaterAt`, `Event.Reading`; `DryingDays` = lower median of the last 3 observed cycles; `nextCheck` routes check tasks in `NextDue`, ignoring anchor and seasons, snooze on top; `CurrentInterval(t, events, today, h)` returns learned; `LastReading`). Non-check tasks unchanged; stray `checked` events are dropped for them. Table + property tests in `moisture_test.go`. Tasks read/write `moisture_check` (default false) and `water_at` (default 3; PUT resets omitted ones); responses add `drying_days`, `last_reading`. Events take `reading`; history and logged events return it. Digest says "Check soil for Monstera". Export adds `tasks[].moisture_check`, `water_at`, `events[].reading` (docs/export-format.md).
- Fix: 0008's `reading` CHECK rejected NULL readings on skips and snoozes (`NULL AND FALSE` is false), which would also have failed the upgrade on any DB with a skip. Fixed in place (unreleased); `internal/db/migrate_moisture_test.go` round-trips up/down/up with existing events.
- Problem details (400): "Only watering tasks can use soil checks.", "Water-at must be between 1 and 9.", "Soil checks always count from the last watering." (explicit fixed), "Add the meter reading.", "Only soil-check tasks take readings.", "Only a check or a watering takes a reading.", "Reading must be 1 to 10."; bad event kind now "Kind must be done, skipped, snoozed or checked."
- Checks: `go generate` no drift, build, vet, golangci-lint 0 issues, `go test ./...`.

### 2026-10-19 · M7: seasonal adjustment (backend + frontend)
- Done: `internal/schedule/season.go` (`Hemisphere`, built-in monthly curves in percent for water, mist and fertilize, south = six months on; `Task.Kind`/`Seasonal`; `NextDue(t, events, h)`; `CurrentInterval`), table + property tests in `season_test.go`. `care.Calendar` (today + hemisphere) flows from settings into task responses, Today, log/undo responses and the push digest. Tasks read/write `seasonal` (omitted = kind default on POST and PUT); responses carry `seasonal` and `current_interval_days`. Settings save `hemisphere` (400 "Hemisphere must be north or south."). Export adds `settings.hemisphere` and `tasks[].seasonal` (docs/export-format.md). Frontend: "Adjust for seasons" switch in the task sheet (default from kind on new tasks; disabled with a hint under a fixed schedule), "Every 7 days · 8 days this season" on the Care tab, Hemisphere (Northern/Southern) in Settings → General. Settings saves now also refresh plants (task lists).
- Behaviour: migration 0007 opted existing water, mist and fertilize tasks in, so their next due dates shift at once (e.g. weekly water done in October is due after 8 days, in January 11, in July 6).
- Deviation: the switch is shown only for kinds with a curve (water, mist, fertilize); for the others it would do nothing. The today e2e turns seasons off on its task so it doesn't depend on the month.
- Checks: `make gen` (no drift), golangci-lint 0 issues, `make lint test` (Go + 89 Vitest), `make build`, `make e2e` (6 passed).
- Next: M8 species.

### 2026-10-14 · M6: web push daily reminder (backend + frontend)
- Lead fix: logging out turns off push on that device first (best effort), so a signed-out phone stops getting reminders. Released as v1.1.0.
- Backend: `internal/push` (VAPID keys generated once into `<data>/vapid.json`, 0600; `Sender` posts to every device of a user via webpush-go, TTL 12 h, urgency normal, Topic = tag; 404/410 deletes the subscription, other failures bump `failed_count`, success sets `last_ok_at`; it errors only when no device got the message, so a retry never repeats a delivery). `Digest` runner (ticks every minute, injected clock, started/stopped by `serve` with the backup runner): once a user's local time passes `digest_time`, `INSERT … ON CONFLICT DO NOTHING` a `digest:<user>:<local date>` job; claims due/expired-lock jobs (lock 5 min), skips a day that has ended ("skipped: the day ended before it was sent") or nothing due ("nothing due"), sends, then marks done; failures back off 1/2/4/8 min, `failed` after 5 attempts. Today's sorting moved to `internal/care` (`care.Today`, `care.Schedule`), shared by GET /today and the reminder. New queries `jobs.sql`, `push.sql`; `UpdateSettings` replaces `UpdateTimezone`. Handlers `internal/api/push_handlers.go`; settings/me/export carry `digest_time`. New dependency: `github.com/SherClockHolmes/webpush-go` v1.4.0 (+ `golang-jwt/jwt/v5`).
- Wording: "1 plant needs care" / "3 plants need care" (distinct plants); body lists up to 3 tasks, overdue first: "Water Monstera, Mist Fern, Prune for Pothos and 1 more" (custom tasks read "<label> for <plant>").
- DST: Go puts a spring-forward-gap time an hour early, so `FireTime` shifts it forward by the gap (02:30 → 03:30, as PLAN §5.3 intended); a fall-back time fires at its first occurrence and the dedupe key stops the second.
- Problem details: 400 "Pick a time like 08:00.", "Endpoint must be an https URL.", "Subscription keys are required."; a failed test push is a 500.
- Frontend: SW is now `src/sw.ts` (injectManifest; precache + navigation fallback with the same denylist, the M5 `api`/`media` runtime caches, skipWaiting + clientsClaim, `push` → notification with icon and the new monochrome `icons/badge-96.png` from `scripts/icons.sh`, `notificationclick` focuses and navigates an open window or opens one). The payload mapping is pure (`lib/notification.ts`, same-site paths only). `tsconfig.sw.json` (WebWorker lib); the app tsconfig excludes `sw.ts`. New pinned devDeps: workbox-precaching/routing/strategies/expiration 7.4.1. Design: `Switch`. Settings → Notifications: Daily reminder switch (unsupported / blocked / off / on from `pushManager.getSubscription()`), native time input saved on change (settings saves now merge with the cached settings and run in order), Send test notification when on, Android note. Undoes the browser subscription if the server refuses it.
- Checks: gen reproducible, golangci-lint 0 issues, Go tests (DST, tz, dedupe across restarts, stale skip, backoff, 410/404 cleanup, expired lock, wording, settings and push endpoints against an httptest TLS push service in `internal/push/pushtest`), eslint + tsc, 87 Vitest, build, `make e2e` 6/6 (Settings asserts the section and that the time saves).
- Follow-ups: logout leaves this device subscribed (fine for one user; a household second user would get the first's reminders on a shared phone); subscriptions with a high `failed_count` are never pruned; changing the time after today's reminder went out takes effect tomorrow.
- Owner: release, update, then on the phone open the installed app → Settings → Notifications → Daily reminder → Allow → Send test notification.

### 2026-10-14 · CP6: M5 integrated, v1.0.0 (MVP complete)
- Export zip (format v1, docs/export-format.md), nightly `VACUUM INTO` backups (keep 14), settings (theme, time zone, password, storage, export), pot/soil fields, read-only offline, Android back closes sheets/viewer, embedded tzdata.
- Lead fixes: gzip for embedded text assets (Lighthouse flagged ~330 KiB uncompressed); e2e specs each get their own client IP through the trusted-proxy path, so the suite no longer runs against the login rate limit.
- Lighthouse: accessibility 100 (light + dark), best practices 100, SEO 100; performance measured before gzip at 73–82 mobile.
- Verified: no codegen drift, golangci-lint 0 issues, Go tests, 78 Vitest, 6 Playwright e2e.
- Off-box backups: Google Drive via TrueNAS Cloud Sync (owner), restoring from `backups/`.
- Next (phase 2, PLAN §2): M6 web push (switch the SW to injectManifest, daily digest via the jobs table). Then M7 seasonal, M8 species, M9 import + quick-add. Owner backlog in PLAN §10. Optional: publish the web/src/design components to the Claude Design system.

### 2026-10-14 · Frontend M5
- Done: `lib/useCloseOnBack` (opening an overlay pushes a same-URL history entry that carries the router's state plus a depth; Back closes only the top overlay; a UI close pops the entry a task later). It's used by BottomSheet and PhotoViewer. "Open plant" navigates with `replace` so it takes over the sheet's entry. Theme override (`lib/theme.ts`, plus `public/theme.js` before first paint because the CSP blocks inline scripts; theme-color metas follow the choice). Settings is now Appearance (System/Light/Dark `SegmentedControl`), General (Time zone sheet with search and a device suggestion, Locations), Account (log out, Change password sheet), Data (export link, storage, last backup) and About. Plant form has a "Pot and soil" group (`plants/form.ts` maps values to and from the API); the detail page shows a pot line. Offline: workbox NetworkFirst (3 s) for the screens' GETs into `api`, CacheFirst for small/medium photos into `media` (300 entries, 30 days); logout deletes both. `OfflineBanner` sits in AppShell; `OfflineToast` watches the mutation cache. Queries use `networkMode: offlineFirst` so the SW can answer them, and mutations use `always` so they fail at once instead of queueing.
- Fixed: after a plant is deleted, Back to its page redirects (replace) to /plants (`wasDeleted`, a per-visit set in `plants/queries.ts`). Plant tab headings go h3 → h2 to fix heading order.
- Checks: lint, typecheck, 78 Vitest, build, `make e2e` 6/6 (new: theme persists across reload, Back closes a sheet, a UI close leaves history clean, export zip, password change and back, viewer Back, Open plant then Back, delete then Back).
- Lighthouse 12 (built binary, logged in): mobile perf 73–82, a11y 100 (98 → 100 after the heading fix), best practices 100, SEO 100. Desktop /settings scores 100 in all four. The a11y score is also 100 in dark.
- Follow-ups: the server doesn't compress text (330 KiB could be saved on first load; the SW precache covers repeat loads). The e2e suite makes 10 login/password attempts against a per-IP burst of 10. A session-reuse fixture or a test-only limit would remove that margin risk. Route-level code splitting if first load matters.

### 2026-10-14 · Backend M5
- Done: pot/soil fields on plants (create/update/get/list/Today); `PUT /settings` (Today follows the new zone at once); `POST /auth/password` (`auth.ChangePassword`: login's rate limiter and constant-time verify; keeps this session, signs out the rest; the CLI's `set-password` shares `replacePassword`); `GET /storage`; `GET /export` (`internal/api/export_handler.go`); `internal/backup` (nightly `VACUUM INTO backups/grower-YYYY-MM-DD.db` on the writer, temp file + fsync + rename, keeps 14, checked at start and hourly, started and stopped by `serve`); `time/tzdata` embedded; `docs/backup.md` (linked from README and deploy.md).
- Export streams the zip from a custom strict response (`VisitExportDataResponse` writes straight to the client, like `loginSuccess`): no pipe or goroutine, and errors surface normally. Rows come from one read-only transaction; photos are stored (not deflated); a missing photo file is skipped with a warning. The write deadline is lifted for this response only.
- Rate limit: every password-change attempt (not only wrong ones) takes a token from the shared per-IP login bucket, like login. Short new passwords are rejected first and cost nothing.
- Problem details: 400 "Pick a time zone from the list.", "Your current password is wrong.", "Use at least 10 characters.", "Passwords must be 256 characters or fewer.", "Pot type must be plastic, terracotta, ceramic, fabric, self_watering or other.", "Pot size must be 0.5 to 48 inches.", "Soil must be 120 characters or fewer."; 429 "Too many attempts. Wait a minute and try again."
- The `jobs` table is still unused: the backup is a plain ticker because it needs no queue or retry state (the file name makes it idempotent). M6 push is the first `jobs` user.
- Checks: gen reproducible, build, vet, golangci-lint 0 issues, Go tests (export round trip, settings, password, storage, backup schedule/retention/restore, pot/soil validation). Smoke: `serve` writes the first backup on start and shuts down cleanly.
- Next: the owner runs the restore drill in docs/backup.md once deployed.

### 2026-10-14 · CP5: M4 integrated, v0.3.0
- Photos end to end: client-side resize + JPEG re-encode (strips EXIF/GPS), upload with dedupe, server thumbnails (small 256 / medium 1024, EXIF orientation), auth-checked `/media` with immutable caching, covers on cards/rows/hero, month timeline, viewer (cover, caption, delete).
- Lead fixes: duplicate-upload race returns the existing photo; leftover `.upload-*` temp dirs are cleaned at startup; e2e no longer depends on spec order.
- Owner feature backlog captured in PLAN §10.
- Verified: no codegen drift, golangci-lint 0 issues, Go tests, 56 Vitest, 5 Playwright e2e.
- Follow-ups: Android back should close the photo viewer (and sheets); swipe between photos; HEIC only if it shows up in practice.
- Next: M5 hardening (export, nightly backup + B2, settings, offline Today, a11y/perf pass), and pot/soil fields from §10 if cheap.

### 2026-10-14 · Frontend M4
- Done: `src/photos/`: `prepare.ts` (createImageBitmap with EXIF orientation, ≤2560 px, JPEG 0.85 via OffscreenCanvas or canvas; re-encoding strips EXIF/GPS; `UnsupportedImageError` for HEIC and other undecodable files), `queries.ts` (list, upload as multipart through openapi-fetch with the JSON Content-Type dropped, update, delete, set cover; writes refresh plants, photos and Today), `copy.ts` (month grouping, "Oct 12, 2026", upload error copy), PhotosTab (Add photo picker, multiple files uploaded one at a time with spinner tiles, month timeline in a 3-column grid, empty state, toasts), PhotoViewer (full-screen native `<dialog>`, medium image, date, caption, newer/older buttons and arrow keys, Set as cover / Cover photo, Edit caption sheet with caption + date, Delete with confirm). Design: `FileButton`; `PlantPhoto` takes `src` (cards, Today rows, detail hero show covers). Fixed: the card photo had the link's `.card` class through `styles[size]`.
- Checks: lint, typecheck, 56 Vitest, build, `make e2e` (5 tests, incl. upload → grid cover → viewer → delete).
- Notes: cover alt text is the plant name only on the hero; cards and rows use `alt=""` because the name is printed beside them. The viewer follows the theme (no forced dark). Android back leaves the plant page while the viewer is open.
- Follow-ups: swipe between photos; "View original"; a back-button history entry for the viewer if it feels wrong on the phone.

### 2026-10-14 · Backend M4
- Done: `internal/media` (store under `<data>/media/photos/<id>/`: untouched `original.jpg|png`, `small.jpg` 256 px, `medium.jpg` 1024 px, JPEG q80; EXIF orientation reader and the 8 transforms; Catmull-Rom scaling, never upscaling; 50 MP bomb guard via `DecodeConfig`; temp dir + rename). Queries `internal/db/queries/photos.sql`; handlers `internal/api/photos_handlers.go`; `GET /media/photos/{id}/{size}` in `internal/server/media.go`. New dependency: `golang.org/x/image`.
- Every Plant (list, get, create, update, archived, Today, set cover) carries `cover`; lists load all covers with one extra query. Upload: plant 404, 15 MB body cap (413), sha256 dedupe per plant (200 with the existing photo), first photo becomes cover in the insert's transaction, files removed if the insert fails. Deleting a cover hands it to the newest remaining photo in one transaction; deleting a plant removes its photo directories after commit.
- CSRF: multipart/form-data is accepted only on `POST /api/v1/plants/{id}/photos` (Origin/Sec-Fetch-Site still checked); that route requires multipart.
- Checks: gen reproducible, build, vet, golangci-lint 0 issues, Go tests.
- Follow-ups: leftover `media/.upload-*` dirs after a crash are not swept; two identical uploads racing get a 500 for the loser (UNIQUE), not a 200.

### 2026-10-14 · CP4: M3 integrated, v0.2.0
- The core loop works end to end: tasks per plant, `internal/schedule` (pure, table-tested incl. DST), Today with overdue/due/coming up, done with undo, snooze, skip, history.
- Lead fix: Today returns `task_count`, so "No care tasks yet" vs "All caught up" is correct for plants without tasks.
- Verified: no codegen drift, golangci-lint 0 issues, Go tests, 44 Vitest, 4 Playwright e2e.
- Follow-ups (not blocking): swipe/long-press on task rows; group Today by location; Back after deleting a plant shows "Plant not found"; plant task list loads events per task (fine at this scale).
- Next: M4 photos (upload with client-side resize + HEIC→JPEG, thumbnails, timeline, cover photo).

### 2026-10-14 · Frontend M3
- Done: design components TaskIcon (+ `taskName`), DoneCheck, TaskRow (brand-soft flash, then a 200ms collapse; reduced motion removes the row at once), Toast (`ToastProvider`/`useToast`, one toast, Undo, 5s); PlantPhoto `thumb` size; FilterChips option icons and `wrap`. `src/care/`: hooks (`queries.ts`: today, tasks, history, save/delete task, log event with optimistic removal from Today plus rollback, Undo), copy helpers (`copy.ts`: due lines, verbs, toasts, history), TaskActionsSheet (Done, Snooze presets or a picked date, Skip, Open plant), TaskSheet (kind chips, custom name, interval with presets, first due, anchor), CareTab (tasks, done check, History). Today (`/`) shows Overdue, Due today and a collapsible Coming up, with the heading taken from the server's date. Events send the device's `local_date`. `todayKey` is refreshed on plant and location writes.
- Checks: lint, typecheck, 44 Vitest, build and `make e2e` all pass (4 tests, including the core loop: add task, done, undo, snooze, done from Coming up, history).
- Notes: Today can't tell "no tasks at all" from "nothing due this week". With no plants it shows "No care tasks yet"; otherwise "All caught up / Nothing is due in the next week". Snooze presets that fall on or before the due date are hidden.
- Follow-ups: have Today return the next task beyond the week (or a task count) for the empty state; group Today by location; swipe or long-press on rows (rows open the sheet on tap for now).

### 2026-10-14 · Backend M3
- Done: `internal/schedule` (pure: civil `Date`, `Today`, `NextDue`, `LastDone`; table tests, a seeded property test, `Today` across DST in Chicago, London and Sydney). Queries `internal/db/queries/care.sql`; handlers `internal/api/{tasks,events}_handlers.go` and `today_handler.go`; integration tests `internal/server/{care,today}_test.go`. Checks pass: gen (no drift), build, vet, golangci-lint (0 issues), tests.
- Scope: no seasonal factor (M7) and no paused tasks yet. Every task response computes `next_due`/`last_done` from its non-voided events; nothing is stored.
- Today uses two queries (active tasks with plants, and all their non-voided events grouped in Go), so no N+1. The plant task list loads events per task (a handful).
- Behaviour: undoing an already-undone event returns the task (idempotent). `limit` on history is clamped to 1–200. Plant 404 on create task comes from the foreign key; validation runs before the task 404 on logging an event.
- Problem details: 400 "Kind must be water, fertilize, mist, repot, rotate or custom.", "Label is required for a custom task.", "Label must be 40 characters or fewer.", "Interval must be 1 to 365 days.", "Anchor must be from_completion or fixed.", "Kind must be done, skipped or snoozed.", "That date is too far from today.", "Pick a snooze date after today.", "Only a snooze takes a snooze date.", "Note must be 500 characters or fewer."; 404 "Plant not found.", "Task not found.", "Event not found."
- Next: frontend Today and Care tab against these endpoints; the M3 Playwright exit test.

### 2026-10-14 · CP3: M2 integrated
- Verified: no codegen drift, golangci-lint 0 issues, eslint + tsc clean, Go tests + 23 Vitest, 3 Playwright e2e (incl. the full locations/plants flow).
- Known minor issues: after deleting a plant, Back lands on its "Plant not found" page; location filter is client-side (fine at this scale).
- Next: M3 (care tasks, `internal/schedule`, Today). Write the schedule test table first (PLAN §5.4).

### 2026-10-14 · Frontend M2
- Done: design components Fab, FilterChips, PlantCard/PlantPhoto (initial placeholder until M4), BottomSheet (native `<dialog>`), SelectField, TextAreaField (shared `Field` wrapper with TextField), ConfirmButton (inline two-step delete), Tabs, ErrorText; Button `danger` variant; field errors use `--danger`; TopBar `leading` slot and optional title; AppShell `tabBar={false}` for full-screen forms. Hooks in `src/plants/queries.ts` and `src/locations/queries.ts` (writes refresh plants and locations). Screens: `/plants` (location chips in `?location=`, grid, FAB, Archived link), `/plants/archived`, `/plants/:id` (Care · Photos · Notes, restore), `/plants/new` and `/plants/:id/edit` (new location from a sheet, archive, delete), `/settings/locations` (sheet add/edit/delete, 409 name message).
- Checks: lint, typecheck, 23 Vitest, build, `make e2e` (3 tests, incl. the M2 flow) all pass.
- Notes: the location filter is client-side (the API has no `?location=`). Saves refetch the open plant once more than needed; harmless.
- Next: M3 care tasks (the Care tab and Today).

### 2026-10-14 · Backend M2
- Done: sqlc queries `locations.sql` and `plants.sql`, handlers `internal/api/{locations,plants}_handlers.go`, and integration tests `internal/server/{locations,plants}_test.go` (CRUD, plant_count, unplacing on delete, archive filter and timestamp, validation, 404, 401, CSRF on PUT/DELETE). New dependency: `github.com/oapi-codegen/runtime` (the generated code needs it). Checks pass: gen (no drift), build, vet, golangci-lint (0 issues), tests.
- Duplicate names and unknown `location_id` come from SQLite constraint errors (`db.IsUniqueViolation`, `db.IsForeignKeyViolation`), not pre-checks. Names are trimmed; species and notes are not. A bad `acquired_on` fails JSON decoding, so it gets the generic 400 detail.
- Problem details: 409 "A location with that name already exists.", 400 "That location does not exist.", "Name is required.", "Name must be 60 (or 80) characters or fewer.", 404 "Location not found." and "Plant not found."

### 2026-10-14 · CP2: M1 code complete, integrated
- Verified in the devshell: `make gen` (no drift), `make lint` (golangci-lint 0 issues, eslint, tsc), `make test` (Go + 16 Vitest), `make e2e` (2 Playwright tests vs the real binary), `make docker` (24 MB image; container user create, login and HEALTHCHECK all OK).
- CI needs golangci-lint ≥ 2.14 (built with Go 1.26); the workflow uses `latest`.
- Follow-ups: add a `danger` colour token for form errors (design system + `design/tokens.json`); switch the SW to injectManifest before M6; theme override in Settings (M5).
- Pushed: CI green on all jobs (go, web, e2e, docker); release workflow published the image; tagged v0.1.0.
- Next: the owner deploys to TrueNAS (see Owner action items). Then M2.

### 2026-10-14 · Backend M1
- Done: `internal/{buildinfo,config,db,auth,api,server}`, `cmd/grower` (`serve`, `migrate`, `user create|set-password [--password-stdin]`, `healthcheck`, `version`), `web/embed.go`, `.golangci.yml`. Migration 0001 (users, sessions, settings, jobs). Codegen is pinned as Go tool deps (`go tool oapi-codegen` v2.8.0, `go tool sqlc` v1.31.1 built with CGO off) and runs through `go generate ./...`. Argon2id hashing (m=64MiB, t=3, p=2, about 50 ms), SHA-256-hashed session tokens, a 90-day sliding expiry that refreshes daily, and per-IP plus global login rate limits. The CSRF check covers Origin, Sec-Fetch-Site and JSON content type. Also: a trusted-proxy XFF walk, security headers, SPA embed with cache headers and fallback, `/healthz`, and graceful shutdown. Tests are table-driven and run in under 3 s.
- Run: `printf 'pw-at-least-10\n' | GROWER_DATA_DIR=./data go run ./cmd/grower user create --username me --password-stdin`, then `make dev-api`.
- Deviations: `users` and `settings` also have `updated_at`; `settings.hemisphere` defaults to `north` and `digest_local_time` to `08:00`. Cookies are `grower_session` on http and `__Host-grower_session` on https (PLAN §7 said `__Host-session`). `go.mod` has `ignore ./web/node_modules` because an npm package ships .go files.
- Next: Dockerfile (distroless, `HEALTHCHECK ["/grower","healthcheck"]`), CI, deploy docs.

### 2026-10-14 · Frontend M1
- Done: `web/` app (Vite 8, React 19, strict TS, React Router 8 data mode, TanStack Query, openapi-fetch). `web/scripts/tokens.mjs` generates `src/design/tokens.css`. Components in `web/src/design/`: Button, IconButton, TextField, TopBar, BottomTabBar (a left rail from 900px), AppShell, EmptyState, Logo. Auth hooks (`useMe`, `useLogin`, `useLogout`) and route guards (`/login?next=`). Screens: `/login`, `/` (Today date heading + empty state), `/plants` (placeholder), `/settings` (account, log out, version, time zone). PWA through vite-plugin-pwa (generateSW, autoUpdate, no API caching). Icons come from `assets/brand` via `web/scripts/icons.sh`. Vitest covers Button, TextField, Login and dates (16 tests). There is a Playwright smoke test in `web/e2e` that runs against `bin/grower`.
- Deviations: generateSW for now. PLAN §1 says injectManifest, which push in M6 needs. TypeScript is pinned to 5.9.3 because of the peer ranges of typescript-eslint and openapi-typescript. `@playwright/test` is pinned to 1.63.0 to match nixpkgs. The design system has no error colour, so errors use ink text with an alert icon and words (a `danger` token is worth adding). Settings has no theme override yet, though the tokens support `data-theme`.
- Next: run `nix develop --command make e2e` once the backend's `user create --password-stdin` and `serve` land.

### 2026-10-14 · CP1: repo foundations
- Done: git repo, `.gitignore`, Go module `github.com/reilandeubank/grower`, `AGENTS.md` (+ `CLAUDE.md` symlink), `Makefile`, `api/openapi.yaml` (login/logout/me), `design/tokens.json`, leaf logo SVGs.
- Design system: https://claude.ai/artifact/E3aXVz7TBFPNxJqCmmMPHS (foundations only; components not yet published there).
- Next: backend and frontend scaffolds in parallel, then Docker/CI.
