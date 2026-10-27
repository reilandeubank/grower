# Contributing

Thanks for your interest.
grower is a personal project that I'm sharing as is, so please keep expectations modest: I'll read issues and pull requests, but I can't promise to respond quickly or to merge features I don't plan to use myself.

- **Bugs:** open an issue with what you did, what you expected and what happened, plus the `version` from `/healthz` and any relevant log lines.
- **Features:** open an issue to discuss before writing a large change.
  grower is deliberately single-household, mobile-first and light on dependencies; see [Non-goals](README.md#non-goals).
- **Security issues:** don't open a public issue; see [SECURITY.md](SECURITY.md).

Local setup is in [docs/development.md](docs/development.md), and the design reasoning (scheduling rules, data model, auth) is in [docs/architecture.md](docs/architecture.md).

## Layout

- `api/openapi.yaml`: the API contract.
  Source of truth for server interfaces and client types.
- `cmd/grower`: the binary (`serve`, `migrate`, `user create`, `import`, `healthcheck`).
- `internal/<pkg>`: one concept per package (`config`, `server`, `api`, `auth`, `db`, `schedule`, ...).
- `internal/db/migrations`: goose SQL migrations.
  `internal/db/queries`: sqlc input.
- `web/`: Vite + React + TypeScript SPA.
  `web/src/design/` holds tokens and UI components.
- `design/tokens.json`: design tokens.
  `web/src/design/tokens.css` is generated from it.

## Conventions

- **Contract first:** edit `api/openapi.yaml`, run `make gen`, then implement.
- **Never hand-edit generated code:** `internal/apigen`, `internal/db/dbgen`, `web/src/api/schema.d.ts`, `web/src/design/tokens.css`.
  CI fails if they're out of date.
- **Database changes** are a new migration file (never edit a released one) plus queries, then `make gen`.
- **Scheduling rules live only in `internal/schedule`:** pure, no `time.Now`, no DB.
  Add a test row for every behaviour change.
- **Dates:** care logic uses civil dates (YYYY-MM-DD); stored timestamps are UTC RFC 3339.
- **Handlers are thin:** parse, call the domain or db, map the result.
  No business logic in handlers or React components.
- **Errors:** wrap with `%w`, log once at the boundary with `slog`, return RFC 9457 problem JSON from the API.
- **Frontend:** use tokens and components from `web/src/design`.
  No raw hex colours, ad-hoc spacing or inline font sizes.
- **Copy** follows [design/README.md](design/README.md): sentence case, plain verbs, no emoji, status always has a word and an icon.
- **Dependencies:** few and mainstream.
  Open an issue before adding a new Go module or npm package.
- Keep files small (under ~300 lines) and names descriptive; comment the why, not the what.
- **Docs** use [semantic linefeeds](https://rhodesmill.org/brandon/2012/one-sentence-per-line/): one sentence per line, no hard wrapping.
- Never commit secrets.
  Configuration comes from `GROWER_*` environment variables.

## Before you open a pull request

`make lint test` must pass.
Run `make e2e` too if you touched login, Today or a task flow.
Keep each pull request to one change, and describe what it does and how you checked it.

## Releasing (maintainer)

Every push to `main` publishes the `:main` image.
To release:

1. Merge a pull request that adds the version's notes to [CHANGELOG.md](CHANGELOG.md).
2. On GitHub, **Releases → Draft a new release**.
   Create a new tag `vX.Y.Z` on `main`, title it `vX.Y.Z`, and paste that version's changelog section as the notes.
3. **Publish release.**
   The release workflow builds and publishes `:X.Y.Z`, `:X.Y` and `:latest` for amd64 and arm64.
   Saving as a draft publishes nothing.
