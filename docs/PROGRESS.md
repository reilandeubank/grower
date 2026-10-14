# Progress and checkpoints

The running log for picking work back up. Newest checkpoint first. Each checkpoint says what's done, what's in flight, and the exact next step.
Milestone definitions live in `PLAN.md` §2.

## Status board

| Milestone | Status |
|---|---|
| M1 Deployed skeleton + auth | in progress |
| M2 Plants and locations | not started |
| M3 Care tasks + scheduling + Today | not started |
| M4 Photos | not started |
| M5 MVP hardening, v1.0 | not started |

### M1 checklist
- [x] Repo, conventions (`AGENTS.md`), API contract for auth (`api/openapi.yaml`), logo (`assets/brand/`)
- [x] Design system foundations in Claude Design (tokens, README, cover)
- [ ] Backend: config, db + migrations, auth/sessions, API handlers, SPA embed, CLI, tests
- [ ] Frontend: Vite app, tokens.css, M1 components, login, app shell, Today empty state, PWA manifest + icons
- [ ] Dockerfile, compose file, CI (test) and release (GHCR) workflows
- [ ] Docs: deploy (TrueNAS + Nginx Proxy Manager), update, local dev
- [ ] First image published to GHCR
- [ ] Deployed on TrueNAS behind NPM; logged in from the installed PWA (needs owner)

## Owner action items
- (none yet)

## Checkpoints

### 2026-10-14 · CP1: repo foundations
- Done: git repo, `.gitignore`, Go module `github.com/reilandeubank/grower`, `AGENTS.md` (+ `CLAUDE.md` symlink), `Makefile`, `api/openapi.yaml` (login/logout/me), `design/tokens.json`, leaf logo SVGs.
- Design system: https://claude.ai/artifact/E3aXVz7TBFPNxJqCmmMPHS (foundations only; components not yet published there).
- Next: backend and frontend scaffolds in parallel, then Docker/CI.
