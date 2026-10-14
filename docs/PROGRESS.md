# Progress and checkpoints

The running log for picking work back up. Newest checkpoint first. Each checkpoint says what's done, what's in flight, and the exact next step.
Milestone definitions live in `PLAN.md` §2.

## Status board

| Milestone | Status |
|---|---|
| M1 Deployed skeleton + auth | done (deployed 2026-10-14) |
| M2 Plants and locations | done (pending deploy) |
| M3 Care tasks + scheduling + Today | not started |
| M4 Photos | not started |
| M5 MVP hardening, v1.0 | not started |

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
- After M2 is released: update the TrueNAS app (image `:main`, or tag a version) and try adding plants on the phone.

## Checkpoints

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
