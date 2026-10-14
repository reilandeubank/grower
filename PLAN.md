# grower: Implementation Plan

Derived from `design-brief.md` (2026-10-14). Sections follow brief §11. Owner decisions are recorded in §9 and folded into the sections below.

---

## 1. Stack recommendation: Option A (Go + embedded React SPA)

| Layer        | Choice                                                                                                                     | Why                                                                                                                                                                                                         |
| ------------ | -------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Language     | Go 1.25+                                                                                                                   | Owner preference; single static binary; in-process scheduler is natural (goroutine + ticker), unlike Next.js's request-scoped model.                                                                        |
| HTTP         | stdlib `net/http` (1.22+ method/path routing)                                                                              | No router dependency needed.                                                                                                                                                                                |
| API contract | **Spec-first** `api/openapi.yaml` → `oapi-codegen` (strict server interfaces)                                              | The spec is the source of truth for both sides; the compiler fails if a handler drifts from it.                                                                                                             |
| DB driver    | `modernc.org/sqlite` (pure Go, no CGO)                                                                                     | Brief constraint; trivial cross-compile and distroless image.                                                                                                                                               |
| Queries      | `sqlc`                                                                                                                     | Typed queries from plain SQL; SQL stays portable to Postgres (sqlc supports both).                                                                                                                          |
| Migrations   | `goose` with SQL files embedded via `embed.FS`, run at startup                                                             | Real, versioned migrations from day one; plain SQL is agent-friendly.                                                                                                                                       |
| Frontend     | React 19 + Vite + TypeScript, TanStack Query, React Router                                                                 | Owner already knows React (via Next.js); largest ecosystem for agent-written code. SvelteKit is viable, but swapping frameworks _and_ backend language at once adds learning load for no core-loop benefit. |
| API client   | `openapi-typescript` + `openapi-fetch`                                                                                     | Generated types, ~6 kB runtime, no codegen'd classes.                                                                                                                                                       |
| Styling      | CSS custom-property tokens (from the design system, §8) + CSS Modules; Radix primitives only for dialog/sheet/popover a11y | Avoids a default-template look and keeps deps few.                                                                                                                                                          |
| PWA          | `vite-plugin-pwa` (injectManifest mode, hand-written SW for push)                                                          | Standard, but leaves push/notification-click logic in our code.                                                                                                                                             |
| Push         | `github.com/SherClockHolmes/webpush-go`                                                                                    | VAPID web push, pure Go.                                                                                                                                                                                    |
| Images       | stdlib `image/jpeg` + `golang.org/x/image/draw` + small EXIF-orientation reader                                            | Pure Go. **Note:** no pure-Go WebP/AVIF _encoder_, so thumbnails are JPEG.                                                                                                                                  |
| Logging      | `log/slog` JSON handler                                                                                                    | Stdlib structured logging.                                                                                                                                                                                  |

**Objections and corrections to the brief**

1. **Snooze conflicts with "derive from the log."** Make snooze and skip _events_ in the same log (`kind = done | skipped | snoozed`), so next-due stays a pure function of (task config, events, today). §5 has the details.
2. **Store civil dates alongside UTC.** "Store UTC, evaluate local day" is right for timestamps. But care is day-granular, and recomputing a local date from UTC after a timezone change (e.g. travel) silently moves history. Each event also records its `local_date` at write time, and scheduling works only in civil dates. That removes DST from the math entirely.
3. **Pull export into the MVP.** The brief lists export as phase 2 but also says to define the format early. JSON export costs about a day once the schema exists, so it ships in M5. Import stays in phase 2.
4. **Thumbnails will be JPEG, not WebP**, to keep CGO out (see the table above). Android camera HEIC uploads are converted to JPEG in the browser before upload, so the server only ever handles JPEG/PNG.
5. **Species data sources are shakier than the brief implies** (checked 2026-10-14):
   - **Trefle**: its homepage says it is "being restored", and its about page has long called it beta with schema changes possible. Treat it as unreliable.
   - **Perenual**: owner vetted the ToS; the free tier is **100 calls/day**. An API key is in hand.
   - **Open Plantbook**: needs an account and API key (OAuth2 client credentials supported). Data is community-sourced. It has useful numeric ranges (light/moisture/temperature) and is popular with Home Assistant users. An account is in hand.
   - **Pl@ntNet** (identification, not care data): the official terms give 500 free identifications/day and say submitted images aren't retained. That's the best fit for "quick-add via photo."

   Recommendation: design `species` as a local, user-editable table with a `source` column and a pluggable `SpeciesProvider` interface. Ship M8 with both providers: Open Plantbook first, Perenual as a fallback. Perenual's 100 calls/day means lookups are on-demand only (never bulk prefetch), results are cached indefinitely, and the refresh job stays within a daily budget. The app works fully with zero providers configured.

6. **The Docker healthcheck needs a binary subcommand.** Distroless images have no curl, so `grower healthcheck` hits `/healthz` locally.
7. **Backups:** a ZFS snapshot of a live WAL-mode SQLite DB is crash-consistent, which is fine. Off-box copies should still come from a **consistent dump**: a nightly in-app job runs `VACUUM INTO data/backups/grower-YYYYMMDD.db` and keeps N copies, and the off-box sync copies `backups/` + `media/`.
8. **Design-system scope risk:** limit the Claude Design pass to tokens plus about 12 components (§8). Don't do full-app mockups before the core loop works.

---

## 2. Milestones (vertical slices, each one deployed)

Every milestone ends with: CI green, image published, update applied on TrueNAS, smoke-tested on the phone.

### M1: Deployed skeleton + auth (pipeline first)

- Repo scaffold (§6), `AGENTS.md`, Makefile, sqlc/goose/oapi-codegen wiring.
- Go server: `/healthz`, `/api/v1/me`, static SPA embed with SPA fallback routing, slog request logging, graceful shutdown.
- Config via env (`GROWER_DATA_DIR`, `GROWER_BASE_URL`, `GROWER_TRUSTED_PROXIES`, `GROWER_TZ`). Later secrets (`GROWER_OPENPLANTBOOK_CLIENT_ID/SECRET`, `GROWER_PERENUAL_API_KEY`) are set in the TrueNAS app env and never committed.
- Migration 0001: `users`, `sessions`, `settings`, `jobs`.
- Auth: first-run setup (create owner via `grower user create` CLI, so there's no public setup page), login/logout, sessions (§4).
- SPA: login screen + empty Today shell, manifest, icons, installable.
- CI: lint, test, build `linux/amd64`, push to GHCR (private package) with `sha-` and semver tags.
- Deploy: TrueNAS dataset, compose file, Nginx Proxy Manager proxy host with a Let's Encrypt cert (§7). Because the image is private, TrueNAS needs `docker login ghcr.io` with a read-only PAT (`read:packages`). Write `docs/deploy.md` and `docs/update.md`.
- **Exit:** log in from the installed PWA over the public URL, and a session survives a container restart.

### M2: Plants and locations

- Migration 0002: `locations`, `plants`.
- CRUD API + screens: plant list (grid, by location), plant detail (stub tabs), add/edit forms.
- First design-system components land here (§8).

### M3: Care tasks + scheduling + Today (the core loop, MVP heart)

- `internal/schedule` pure package with a full test table (§5) **before** wiring it up.
- Migration 0003: `care_tasks`, `care_events`.
- API: task CRUD, `POST /tasks/{id}/events` (done/skip/snooze), `GET /today`.
- Today screen: overdue + due grouped by location, one-tap done (optimistic, with undo toast), snooze sheet (1d / 3d / 1w / pick date).
- Plant detail → Care tab: tasks + history.
- **Exit:** a Playwright E2E test runs create plant → add watering task → appears in Today → done → disappears → reappears on the correct day (clock injected).

### M4: Photos

- Migration 0004: `photos`; `media/` layout (§3).
- Upload (multipart, size cap, client-side resize to ~2560px max + HEIC → JPEG in the browser), server-side EXIF orientation, thumbnails at 256/1024 px, served with long-cache immutable headers.
- Timeline tab on plant detail; set cover photo; camera capture via `<input capture>`.

### M5: MVP hardening, release v1.0

- JSON export (`GET /export` streams a zip: `export.json` + `media/`) with a documented, versioned format (`docs/export-format.md`).
- Nightly `VACUUM INTO` backup job; create the B2 bucket and off-box sync, and document it with a tested restore (§7).
- Settings screen (timezone, digest time placeholder, password change).
- Offline: SW caches app shell + last Today response (read-only offline).
- Accessibility + performance pass (Lighthouse PWA ≥ 90 on mid-range Android).

### Phase 2 (each one is its own milestone, in this order)

- **M6 Web push:** VAPID keys auto-generated into `data/`, `push_subscriptions`, **daily digest only** at the user's local time ("3 plants need care"; tapping opens Today). Per-task reminders are out of scope.
- **M7 Seasonal adjustment:** per-task-type monthly multipliers + hemisphere (§5.4).
- **M8 Species:** `species` table, provider interface with Open Plantbook + Perenual adapters, search-as-you-type on the plant form (debounced, local cache first), "apply care defaults" to create tasks; cached data with `fetched_at` and a budget-limited refresh job.
- **M9 Import** (round-trips M5 export) **+ quick-add via photo** with optional Pl@ntNet identification.
- **Later:** health notes, propagation, ICS feed (`/calendar.ics?token=`), Home Assistant (REST sensor of due count, or MQTT).

---

## 3. Schema and migrations

**Approach**

- `internal/db/migrations/NNNN_name.sql` (goose, up/down), embedded, applied on boot inside a transaction. The app refuses to start if the DB is ahead of the binary.
- `internal/db/queries/*.sql` → `sqlc` → `internal/db/dbgen` (generated, committed).
- Portability rules for a future Postgres move: no SQLite-only functions in queries; IDs are text ULIDs generated in Go; timestamps are `TEXT` RFC 3339 UTC (converted in Go); booleans are `INTEGER 0/1` behind generated types. A move means a parallel `migrations/postgres` set plus a sqlc engine switch.
- Pragmas on open: `journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000`, `synchronous=NORMAL`. A single writer connection plus a read pool.

**Data directory**

```
/data
  grower.db (+ -wal, -shm)
  media/originals/{plant_id}/{photo_id}.jpg
  media/thumbs/{photo_id}_{256|1024}.jpg
  backups/grower-YYYYMMDD.db
  vapid.json            (M6)
```

**Tables** (all have `id TEXT PK` ULID, `created_at`, `updated_at` unless noted)

```sql
users(id, username UNIQUE, password_hash, created_at)
sessions(id_hash PK, user_id FK, created_at, last_seen_at, expires_at, user_agent)
settings(user_id PK FK, timezone TEXT /*IANA*/, hemisphere TEXT, digest_local_time TEXT /*HH:MM*/)

locations(id, name, light_level TEXT /*low|medium|bright_indirect|direct*/, notes, sort_order)
plants(id, name, species_text, species_id FK NULL /*M8*/, location_id FK NULL,
       acquired_on TEXT /*YYYY-MM-DD*/, notes, cover_photo_id FK NULL, archived_at NULL)

care_tasks(id, plant_id FK, kind TEXT /*water|fertilize|mist|repot|rotate|custom*/,
           custom_label NULL, interval_days INTEGER CHECK(>0),
           anchor TEXT /*from_completion|fixed*/, start_on TEXT /*YYYY-MM-DD*/,
           seasonal INTEGER DEFAULT 0, paused_at NULL, archived_at NULL)

care_events(id, task_id FK, plant_id FK /*denorm for history*/,
            kind TEXT /*done|skipped|snoozed*/,
            occurred_at TEXT /*UTC*/, local_date TEXT /*YYYY-MM-DD at write time*/,
            snooze_until TEXT NULL /*YYYY-MM-DD, kind=snoozed only*/,
            note NULL, created_at)        -- append-only; "undo" = soft-delete via voided_at
  INDEX(task_id, local_date)

photos(id, plant_id FK, taken_at TEXT, width, height, bytes, sha256 UNIQUE, caption NULL)

jobs(id, kind, dedupe_key UNIQUE, run_after TEXT, status /*pending|running|done|failed*/,
     attempts, last_error, payload JSON, locked_until)

-- phase 2
push_subscriptions(id, user_id FK, endpoint UNIQUE, p256dh, auth, created_at, last_ok_at, failed_count)
species(id, scientific_name UNIQUE, common_names JSON, source, source_ref,
        water_interval_days NULL, fertilize_interval_days NULL, light TEXT NULL,
        raw JSON, fetched_at, user_edited INTEGER)
seasonal_factors(user_id, task_kind, month INTEGER 1-12, factor REAL, PK(user_id,task_kind,month))
```

`care_events` has one exception to strict append-only: `voided_at` supports the undo toast and mistaken taps without breaking the "derive from the log" rule. Voided rows are ignored by the scheduler.

---

## 4. API outline and screen map

All under `/api/v1`, JSON, cookie auth. `api/openapi.yaml` is canonical; this is a summary.

```
POST   /auth/login            {username,password} → sets cookie
POST   /auth/logout
GET    /me                    user + settings
PATCH  /settings

GET    /today?date=YYYY-MM-DD → {date, overdue[], due[], upcoming[] (next 7d)}
         item: {task, plant{id,name,thumb}, location, due_on, days_overdue}

GET/POST         /locations        PATCH/DELETE /locations/{id}
GET/POST         /plants           GET/PATCH/DELETE /plants/{id}   (?location=&archived=)
GET/POST         /plants/{id}/tasks   PATCH/DELETE /tasks/{id}
POST             /tasks/{id}/events   {kind, snooze_until?, note?, local_date?}
DELETE           /events/{id}         (void, for undo)
GET              /plants/{id}/events  (history, paginated)
GET/POST         /plants/{id}/photos  (multipart)   PATCH/DELETE /photos/{id}
GET    /media/{path}          static, auth-checked, immutable caching

GET    /export                zip stream
GET    /healthz               {status, db, version}  (no auth)

-- phase 2
GET    /push/vapid-public-key ; POST/DELETE /push/subscriptions
GET    /species?q= ; POST /species/{id}/apply/{plant_id}
POST   /import
```

`local_date` on event POST: the client sends its own local date, the server validates it against the configured timezone (±1 day tolerance), and the server timezone is used if it's absent. This keeps a tap at 23:59 from being recorded as tomorrow.

**Screens** (bottom tab bar, mobile-first)

1. **Today** (default route `/`): date header, overdue section (warm accent), due today, collapsed "Coming up". Each row has a plant thumb, name, task icon, a ✓ button, and a swipe or long-press for snooze/skip. An empty state celebrates being done.
2. **Plants** `/plants`: grid of cover photos, location filter chips, FAB "Add plant".
3. **Plant detail** `/plants/:id`: hero photo, name/species/location, tabs **Care** (tasks + next due + history) · **Photos** (timeline) · **Notes**.
4. **Add/Edit plant** (full-screen sheet), **Add/Edit task** (bottom sheet, interval presets).
5. **Locations** (inside Plants or Settings): list + light level.
6. **Settings** `/settings`: timezone, digest time, notifications (M6), export, password, about/version.
7. **Login**.

---

## 5. Scheduling design (`internal/schedule`, pure, no I/O)

### 5.1 Core types

```go
type Date = civil date (year, month, day)  // e.g. cloud.google.com/go/civil-style; or own 30-line type
type Task  struct { Interval int; Anchor AnchorMode; StartOn Date; Paused bool; Seasonal bool; Kind string }
type Event struct { Kind EventKind; LocalDate Date; SnoozeUntil *Date; Voided bool }
type Season interface { Factor(kind string, on Date) float64 } // identity until M7

func NextDue(t Task, events []Event, s Season) (due Date, ok bool)
func Status(due Date, today Date) (overdueDays int, isDue bool)
func Today(now time.Time, loc *time.Location) Date   // the only tz-aware function
```

### 5.2 Rules

1. Ignore voided events. Sort by `LocalDate`, then insertion order.
2. **Base:** the last `done` or `skipped` event's `LocalDate` (`last`). If none, `due = StartOn`.
3. **Anchor `from_completion`** (default, Planta-like): `due = last + effectiveInterval(last)`.
   **Anchor `fixed`**: `due = smallest StartOn + k·interval that is > last`. It doesn't drift when you're late, which suits fertilizing on a calendar.
4. **Snooze:** if the latest non-voided event is `snoozed` and newer than `last`, `due = max(due, SnoozeUntil)`. A later done/skip clears it naturally. Snooze never changes the anchor.
5. **Paused** tasks have no due date (`ok=false`).
6. **Missed days:** no catch-up and no stacking. A task is overdue by `today - due` days and shows once. Completing it while overdue under `from_completion` restarts from the completion day.
7. **Due today** = `due <= today`. **Overdue** = `due < today`.
8. **Effective interval** = `max(1, round(Interval × Season.Factor(kind, last)))`. The factor is chosen by the month of `last`, the date the clock started from, so the due date is stable and doesn't oscillate across a season boundary.

### 5.3 Time zones and DST

- Only `Today()` touches `time.Location`. Everything else is `Date` arithmetic (`AddDays` via `time.Date(y,m,d+n,12,0,0,0,time.UTC)`), so a 23- or 25-hour day can't shift a due date.
- Events store `local_date` at write time, so changing the configured timezone later never rewrites history.
- The digest job's fire time is computed per day as `time.Date(y,m,d,hh,mm,0,0,loc)`. For times that fall in a DST gap (e.g. 02:30 on spring-forward), Go normalizes forward, which is acceptable and tested. For a fall-back duplicate hour, `dedupe_key = digest:{date}` ensures a single send.

### 5.4 Test table (must exist before M3 handlers)

Table-driven tests plus a property test (`testing/quick` or `rapid`):

- No events → `StartOn`; StartOn in the future → not due.
- Done on time / early / late, for both anchor modes.
- Fixed anchor with multiple missed periods → next future slot, single occurrence.
- Snooze before due, snooze while overdue, snooze then done, snooze into the past (rejected at API).
- Skip vs. done equivalence for scheduling.
- Voided last event → falls back to the previous one.
- Two done events on the same day (idempotent).
- `Today()` at 23:59 / 00:01 local, across DST forward/back in `America/Chicago` and `Europe/London`, and southern-hemisphere `Australia/Sydney`.
- Timezone change between events.
- Leap day, year boundary, interval = 1.
- Seasonal factor at a month boundary, factor < 1 rounding to ≥ 1.
- Property: `NextDue` is deterministic, `due > last` for `from_completion`, and adding a voided event never changes the result.

---

## 6. Repo layout, conventions, tests, CI/CD

```
/
├── AGENTS.md                 conventions (below), symlinked as CLAUDE.md
├── Makefile                  gen, test, lint, dev, build, e2e
├── api/openapi.yaml          canonical contract
├── cmd/grower/main.go      serve | migrate | user create | healthcheck
├── internal/
│   ├── config/               env parsing
│   ├── server/               router, middleware (auth, proxy, logging, recover)
│   ├── api/                  handlers implementing generated strict interface
│   ├── apigen/               oapi-codegen output (generated)
│   ├── db/                   sqlc output (generated) + Open()/pragmas
│   ├── schedule/             pure scheduling logic
│   ├── auth/                 hashing, sessions, rate limit
│   ├── media/                storage, thumbnails, exif
│   ├── jobs/                 runner + job kinds (digest, backup, species refresh)
│   ├── push/                 (M6)
│   ├── species/              provider interface + adapters (M8)
│   └── export/
├── internal/db/migrations/   NNNN_*.sql (embedded)
├── internal/db/queries/      sqlc input → internal/db/dbgen
├── web/                      Vite app
│   ├── src/api/schema.d.ts   generated
│   ├── src/design/           tokens.css + components (§8)
│   ├── src/routes/ …
│   └── e2e/                  Playwright
├── web/embed.go              //go:embed dist
├── deploy/compose.yaml, deploy/nginx-proxy-manager.md
├── docs/ deploy.md update.md backup.md export-format.md adr/
└── .github/workflows/ ci.yml release.yml
```

### `AGENTS.md` contents (draft)

```md
# Conventions for agents

- Contract first: edit api/openapi.yaml → `make gen` → implement. Never hand-edit internal/apigen, internal/db, web/src/api/schema.d.ts.
- DB changes = new migration file (never edit a merged one) + queries/\*.sql + `make gen`.
- Scheduling rules live ONLY in internal/schedule (pure, no time.Now, no DB). Add a test row for every behaviour change.
- Dates: care logic uses civil dates (YYYY-MM-DD); timestamps are UTC RFC3339. Never compute "today" outside schedule.Today.
- Handlers are thin: parse → call domain/db → map. No business logic in handlers or React components.
- Errors: wrap with %w; log once at the boundary with slog; API errors use the Problem schema.
- Frontend: use design tokens/components from web/src/design; no raw hex colors or ad-hoc spacing.
- Deps: ask before adding any new dependency (Go or npm).
- Before finishing: `make lint test` must pass; run `make e2e` if you touched Today or task flows.
- Small files (<300 lines), one concept per package, names over comments.
```

### Test strategy

| Layer           | Tool                                                                                                     | Scope                                                                          | Speed target      |
| --------------- | -------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | ----------------- |
| Unit            | `go test`                                                                                                | `schedule` (exhaustive), auth hashing, media orientation/thumbs, export format | < 2 s             |
| API integration | `go test` + `httptest` + temp-dir SQLite with migrations applied, fake clock                             | every endpoint's happy path + authz + validation; Today correctness            | < 10 s            |
| Frontend unit   | Vitest + Testing Library                                                                                 | only non-trivial logic (snooze presets, optimistic updates)                    | < 5 s             |
| E2E             | Playwright (mobile Chromium viewport) against the real binary with a seeded temp DB and `GROWER_FAKE_NOW` | login, core loop, photo upload, export                                         | < 60 s, 3–5 tests |

### CI/CD (GitHub Actions)

- **ci.yml** (PR + main): `make gen` then `git diff --exit-code` (generated code is current), `golangci-lint`, `go test ./...`, `npm ci && npm run lint typecheck test build`, Playwright e2e, and a `docker build`.
- **release.yml** (push to main + tags `v*`): build the multi-stage image (node build → go build `CGO_ENABLED=0` → `gcr.io/distroless/static:nonroot`), push `ghcr.io/<owner>/grower:{sha-xxxx, main, vX.Y.Z, latest-on-tag}`, and attach SBOM/provenance (buildx default).
- **Update path on TrueNAS** (`docs/update.md`): pin the image tag in compose → bump tag → `docker compose pull && up -d` (or edit the custom app in the UI). Migrations run on boot; a ZFS snapshot is taken before an upgrade (a documented manual step, or a periodic snapshot task). Rollback = previous tag + snapshot rollback if a migration ran. Renovate/Dependabot for deps is optional.

---

## 7. Auth, proxy, operations details

- **Passwords:** argon2id (`golang.org/x/crypto/argon2`). Login rate limiting is per IP + global (token bucket, in memory), with a constant-time response.
- **Sessions:** 32-byte random token, stored as SHA-256 in `sessions`. Cookie `__Host-session`, `HttpOnly; Secure; SameSite=Lax; Path=/`. **Sliding 90-day expiry** (refreshed at most once a day), so the installed PWA effectively stays logged in. Logout and "log out other devices" are in Settings.
- **CSRF:** SameSite=Lax + mutating endpoints require `Content-Type: application/json` or a custom `X-Requested-With` header, with an Origin check against `GROWER_BASE_URL`.
- **Proxy:** honour `X-Forwarded-For/Proto` only from `GROWER_TRUSTED_PROXIES` CIDRs. `GROWER_BASE_URL` is authoritative for absolute URLs and cookie security. Add security headers (CSP, HSTS left to the proxy).
- **Nginx Proxy Manager:** NPM sends `X-Forwarded-For`, `X-Forwarded-Proto` and `X-Real-IP` by default. Put the app and NPM on a shared Docker network, set `GROWER_TRUSTED_PROXIES` to that network's subnet, and don't publish the app's port on the host. On the proxy host, enable Force SSL + HSTS and raise `client_max_body_size` (Advanced tab, e.g. `20m`) for photo uploads.
- **Auth scope:** local accounts only for v1. Forward-auth (Authentik/Authelia header) is deferred. Auth middleware sits behind one `Authenticator` interface so a header mode can be added later without touching handlers.
- **Second household user:** the `users` table already supports it. All data is shared, with no per-row ownership (per brief).
- **Jobs runner:** ticker every 30 s. Claims rows with `UPDATE … SET status='running', locked_until=now+5m WHERE id=(SELECT … WHERE status='pending' AND run_after<=now LIMIT 1) RETURNING`. Producers insert with `dedupe_key` (`INSERT … ON CONFLICT DO NOTHING`), and failures back off exponentially. After a restart, expired locks are reclaimed. **Push is sent before marking done**, so a crash means at most one duplicate digest. That's the accepted trade-off, and at-least-once beats silently dropping a reminder.
- **Backups:** ZFS dataset `tank/apps/grower` with hourly/daily snapshot tasks. A nightly `VACUUM INTO` job (keeps 14). Off-box to **Backblaze B2** via a TrueNAS Cloud Sync task (push, remote encryption on) of `backups/` + `media/`, with a B2 lifecycle rule keeping old versions for 30 days. Set up in M5 and documented in `docs/backup.md` with a quarterly restore drill. `restic` is the fallback if you later want deduplicated snapshots.
- **TrueNAS specifics:** run the container as the dataset owner's UID:GID (e.g. `568:568` apps user), mount the dataset at `/data`, and attach it to the existing proxy's network.
- **Photo growth:** keep originals (client-downscaled to ≤2560px, which is ~0.5–1 MB each) + thumbs. Kept indefinitely. Show storage usage in Settings and revisit if it grows large.

---

## 8. Visual design system via Claude Design

**Status (2026-10-14):** foundations are published at https://claude.ai/artifact/E3aXVz7TBFPNxJqCmmMPHS (private). It has light and dark colour tokens including care-type accents, a type scale (Bricolage Grotesque + Atkinson Hyperlegible Next, self-hosted in the app), spacing, radius, shadow and size tokens, the README brand book and the cover. Next: the component previews, in the order the README lists.

Plan:

1. **When:** start in parallel with M1. Tokens must exist before M2's first screens, so UI work doesn't settle on defaults.
2. **Brief to Claude Design:** a mobile-first, app-like, calm botanical feel that avoids generic "green leaf SaaS". Light and dark themes. Tap targets ≥ 44px. One-thumb reach for primary actions. Overdue states are distinguishable without colour alone.
3. **Deliverables:**
   - **Tokens:** colour (neutral ramp, brand, care-type accents for water/fertilize/etc., semantic success/warn/overdue), type scale (one variable font, e.g. a humanist sans), spacing 4-pt scale, radii, elevation, motion durations/easings.
   - **Components (~12):** Button (primary/secondary/ghost/icon), FAB, TaskRow (with done/snooze states + swipe affordance), PlantCard, Chip/FilterChip, BottomTabBar, TopBar, BottomSheet, TextField/Select/DatePicker field, Toast (with Undo), EmptyState, PhotoTimelineItem, Avatar/Thumb.
   - **Key screens** as reference compositions only: Today (with overdue / all-done states), Plant detail, Add task sheet.
4. **Into code:** export tokens as `web/src/design/tokens.css` (CSS custom properties, `prefers-color-scheme` + `[data-theme]`) and implement the components as React + CSS Modules in `web/src/design/`. If we want the local library and the Claude Design project kept in sync afterwards, use the `/design-sync` workflow (component-by-component, never a wholesale replace).
5. **Guardrail:** design work beyond these deliverables waits until after M3 ships.

---

## 9. Owner decisions (resolved 2026-10-14)

| #   | Topic                 | Decision                                                                             |
| --- | --------------------- | ------------------------------------------------------------------------------------ |
| 1   | Frontend framework    | React                                                                                |
| 2   | Default task anchor   | `from_completion`                                                                    |
| 3   | Reverse proxy         | Nginx Proxy Manager                                                                  |
| 4   | Auth                  | Local accounts only for v1; forward-auth deferred (no Authentik running)            |
| 5   | HEIC                  | Convert to JPEG in the browser                                                       |
| 6   | Photo retention       | Keep downscaled originals indefinitely                                               |
| 7   | Off-box backup        | Backblaze B2 (not set up yet; set up in M5)                                          |
| 8   | Species providers     | Open Plantbook (account created) + Perenual (ToS vetted, 100 calls/day, key in hand) |
| 9   | Notifications         | Daily digest only                                                                    |
| 10  | Image architecture    | `linux/amd64` only                                                                   |
| 11  | Repo hosting          | GitHub, private for now; may open later, so keep secrets out of history from day one |
| 12  | Claude Design         | Start now, alongside M1                                                              |

---

### Sources consulted for §1.5

- Trefle: https://trefle.io/ , https://trefle.io/about
- Open Plantbook SDK (auth details): https://openplantbook-sdk-py.readthedocs.io/en/latest/source/readme.html
- Pl@ntNet API terms & pricing: https://my.plantnet.org/terms_of_use , https://my.plantnet.org/pricing
- Perenual: terms and free-tier limit (100 calls/day) verified by the owner.
