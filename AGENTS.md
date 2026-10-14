# grower: conventions for agents

grower is a self-hosted, single-user plant care PWA: a Go server with an embedded React SPA and SQLite.
Read `PLAN.md` for the design and `docs/PROGRESS.md` for where work stands. Update PROGRESS.md when you finish a task.

## Layout
- `api/openapi.yaml`: the API contract. Source of truth for server interfaces and client types.
- `cmd/grower`: the binary (`serve`, `migrate`, `user create`, `healthcheck`).
- `internal/<pkg>`: one concept per package (`config`, `server`, `api`, `auth`, `db`, `schedule`, ...).
- `internal/db/migrations`: goose SQL migrations. `internal/db/queries`: sqlc input.
- `web/`: Vite + React + TypeScript SPA. `web/src/design/` holds tokens and UI components.
- `design/tokens.json`: design tokens, mirrored from the Claude Design system. `web/src/design/tokens.css` is generated from it.

## Rules
- Contract first: edit `api/openapi.yaml`, run `make gen`, then implement.
- Never hand-edit generated code: `internal/apigen`, `internal/db/dbgen`, `web/src/api/schema.d.ts`, `web/src/design/tokens.css`.
- DB changes are a new migration file (never edit a merged one) plus queries, then `make gen`.
- Scheduling rules live only in `internal/schedule`: pure, no `time.Now`, no DB. Add a test row for every behaviour change.
- Dates: care logic uses civil dates (YYYY-MM-DD); stored timestamps are UTC RFC 3339.
- Handlers are thin: parse, call the domain or db, map the result. No business logic in handlers or React components.
- Errors: wrap with `%w`, log once at the boundary with `slog`, return RFC 9457 problem JSON from the API.
- Frontend: use tokens and components from `web/src/design`. No raw hex colours, ad-hoc spacing, or inline font sizes.
- Copy follows the design README: sentence case, plain verbs, no emoji, status always has a word and an icon.
- Dependencies: few and mainstream. Ask before adding a new Go module or npm package.
- Keep files small (under ~300 lines) and names descriptive; comment the why, not the what.
- Never commit secrets. Config comes from `GROWER_*` environment variables.

## Commands
- `make gen`: regenerate API server/client code, sqlc queries and design tokens.
- `make test`: Go and frontend unit/integration tests. `make e2e`: Playwright smoke tests.
- `make lint`: go vet, golangci-lint (if installed), eslint and tsc.
- `make dev`: API on :8080 and Vite on :5173 with a proxy. `make build`: web build, then a single binary in `bin/`.
- Before finishing: `make lint test` must pass. Run `make e2e` if you touched login or Today.
