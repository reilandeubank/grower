# grower

A self-hosted plant care app: see what your plants need today, tap done, keep a photo timeline. Single user, one container, SQLite, installable as a PWA.

- **Run it:** [docs/deploy.md](docs/deploy.md) (TrueNAS SCALE + Nginx Proxy Manager), [docs/update.md](docs/update.md), [docs/backup.md](docs/backup.md) (backups and restore)
- **Work on it:** [docs/development.md](docs/development.md), conventions in [AGENTS.md](AGENTS.md)
- **Design:** [PLAN.md](PLAN.md) (architecture and milestones), [design/README.md](design/README.md) (visual design rules)
- **Status:** [docs/PROGRESS.md](docs/PROGRESS.md)

Go server with an embedded React + TypeScript SPA, typed end to end from [api/openapi.yaml](api/openapi.yaml).
