# Updating grower

Images are published to `ghcr.io/reilandeubank/grower`:

| Tag | What it is |
|---|---|
| `:X.Y.Z` (e.g. `:1.7.0`) | One exact release. Recommended: updates happen only when you change it, and rolling back is exact. |
| `:X.Y`, `:latest` | The newest release in that line, or overall. |
| `:main`, `:sha-<commit>` | Every commit on `main`. Unreleased; may break. |

Release notes are in [CHANGELOG.md](../CHANGELOG.md) and on the GitHub releases page.
Check what's running at `https://<your domain>/healthz`: the `version` field is the build, e.g. `v1.7.0`.

## Back up first

Database migrations run automatically on start and can't be undone without a backup.
Before updating, take a filesystem snapshot of the data directory (e.g. a ZFS snapshot), or stop the container and copy the directory.
See [backup.md](backup.md).

## Update

**Pinned version:** change the `image:` tag in your compose file, then `docker compose pull && docker compose up -d`.
On TrueNAS: Apps → grower → Edit, change `image:`, save.

**Moving tag (`:latest`):** `docker compose pull && docker compose up -d`.
TrueNAS doesn't re-check a moving tag by itself; add `pull_policy: always` to the service, and then Apps → grower → **Edit → Save** (no changes needed) fetches the newest image.

## Roll back

1. Set the compose file back to the previous version's tag.
2. If the newer version ran a migration, stop the app and restore your pre-update backup or snapshot before starting the older version.
   The binary refuses to start against a database newer than it understands, which is your signal that you need the backup.
