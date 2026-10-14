# grower

A self-hosted plant care app: see what your plants need today, tap done, keep a photo timeline, get a daily reminder on your phone. Single user, one container, SQLite, installable as a PWA. Reminders use web push, so the app must be served over HTTPS ([docs/deploy.md](docs/deploy.md)); their keys live in the data directory as `vapid.json`.

- **Run it:** [docs/deploy.md](docs/deploy.md) (TrueNAS SCALE + Nginx Proxy Manager), [docs/update.md](docs/update.md), [docs/backup.md](docs/backup.md) (backups and restore)
- **Work on it:** [docs/development.md](docs/development.md), conventions in [AGENTS.md](AGENTS.md)
- **Design:** [PLAN.md](PLAN.md) (architecture and milestones), [design/README.md](design/README.md) (visual design rules)
- **Status:** [docs/PROGRESS.md](docs/PROGRESS.md)

Go server with an embedded React + TypeScript SPA, typed end to end from [api/openapi.yaml](api/openapi.yaml).
