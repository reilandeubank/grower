# grower web

The React SPA for grower (Vite + React 19 + TypeScript, TanStack Query, React Router in data mode).
`make build` embeds `dist/` into the Go binary via `web/embed.go`.

## Scripts

| Script | What it does |
|---|---|
| `npm run dev` | Vite on :5173, proxying `/api` and `/healthz` to the Go server on :8080 (`make dev` runs both). |
| `npm run build` | Type-checks, builds to `dist/` and restores `dist/.gitkeep`. |
| `npm run gen` | `gen:api` (types from `../api/openapi.yaml` into `src/api/schema.d.ts`) and `gen:tokens`. |
| `npm run gen:tokens` | Writes `src/design/tokens.css` from `../design/tokens.json`. |
| `npm run icons` | Regenerates `public/icons/*` and `public/favicon.svg` from `../assets/brand` (ImageMagick). |
| `npm run lint` / `typecheck` / `test` | ESLint, `tsc -b`, Vitest (jsdom). |
| `npm run e2e` | Playwright smoke tests against `../bin/grower` (run `make build` first, or `make e2e`). |

## Design tokens

`design/tokens.json` is the source of truth for design tokens; `scripts/tokens.mjs` turns it into CSS custom properties (`--surface`, `--space-4`, ...), font stacks (`--font-display`, `--font-sans`) and one class per type style (`.t-display` ... `.t-label`).
Light values sit on `:root`; dark values apply via `prefers-color-scheme` unless `<html data-theme="light">`, or always with `data-theme="dark"`.
Never edit `tokens.css` or `schema.d.ts` by hand.
Components live in `src/design/` (import from `src/design`), and screens use only those components and tokens.

## E2E locally

The e2e config starts `../bin/grower serve` on :8099 with a throwaway data dir, creating user `e2e` first.
On NixOS, npm-downloaded browsers don't run: use the flake devshell, which provides matching browsers (keep `@playwright/test` at the nixpkgs `playwright-driver` version, currently 1.63.0):

```sh
nix develop --command make e2e
```
