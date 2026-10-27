# Local development

**With Nix (recommended):** `direnv allow` once, and the flake devshell provides Go, Node 24, golangci-lint, ImageMagick and Playwright browsers that run on NixOS.
Without direnv: `nix develop`.

**Without Nix:** install Go (version in `go.mod`) and Node 24, then `npx --prefix web playwright install chromium` for e2e.
Docker is only needed for the image build.

```sh
make web-install                                   # npm ci in web/
GROWER_DATA_DIR=./data go run ./cmd/grower user create --username dev
make dev                                           # API :8080 + Vite :5173
```

Open http://localhost:5173.
Vite proxies `/api` and `/healthz` to the Go server.
In dev, `GROWER_BASE_URL` is `http://localhost:5173`, so the cookie is non-secure `grower_session`.

| Command | What it does |
|---|---|
| `make gen` | Regenerate Go API code (oapi-codegen), sqlc queries, TS client types and `tokens.css` |
| `make lint` / `make test` | Static checks / unit + integration tests |
| `make build` | Build the SPA, then `bin/grower` with it embedded |
| `make e2e` | Build, then run the Playwright smoke tests against the real binary |
| `make docker` | Build the production image locally as `grower:dev` |

## Changing things

- **API:** edit `api/openapi.yaml` → `make gen` → implement the new method on the strict server in `internal/api`, and use the typed client in `web/src/api`.
- **Database:** add `internal/db/migrations/NNNN_name.sql` and queries in `internal/db/queries/` → `make gen`.
- **Design tokens:** edit `design/tokens.json` (the source of truth for colours, type and spacing) → `make gen` rewrites `web/src/design/tokens.css`.
  Visual and copy rules are in [design/README.md](../design/README.md).

## Environment variables

See [Configuration](../README.md#configuration) in the README.
`make dev` sets `GROWER_DATA_DIR=./data`, `GROWER_BASE_URL=http://localhost:5173` and `GROWER_LOG_FORMAT=text`.

## Design notes

[architecture.md](architecture.md) explains the stack, the data model and the scheduling rules.
Conventions for changes are in [CONTRIBUTING.md](../CONTRIBUTING.md).
