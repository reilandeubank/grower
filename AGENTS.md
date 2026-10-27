# grower: notes for coding agents

grower is a self-hosted, single-household plant care PWA: a Go server with an embedded React SPA and SQLite.

Before changing anything, read [CONTRIBUTING.md](CONTRIBUTING.md) for the layout and conventions, and [docs/architecture.md](docs/architecture.md) for the design (scheduling rules, data model, auth).
The conventions there are rules, not suggestions.
In particular:

- Contract first: edit `api/openapi.yaml`, run `make gen`, then implement.
  Never hand-edit generated code.
- Scheduling rules live only in `internal/schedule` (pure, no `time.Now`, no DB), with a test row for every behaviour change.
- Ask before adding a new Go module or npm package.
- Markdown uses semantic linefeeds: one sentence per line.
- Never commit secrets or anyone's personal setup (domains, IPs, paths).

## Commands
- `make gen`: regenerate API server/client code, sqlc queries and design tokens.
- `make test`: Go and frontend unit/integration tests.
  `make e2e`: Playwright smoke tests.
- `make lint`: go vet, golangci-lint (if installed), eslint and tsc.
- `make dev`: API on :8080 and Vite on :5173 with a proxy.
  `make build`: web build, then a single binary in `bin/`.
- Before finishing: `make lint test` must pass.
  Run `make e2e` if you touched login or Today.
