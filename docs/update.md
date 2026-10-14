# Updating grower

Every push to `main` publishes `ghcr.io/reilandeubank/grower:main` and `:sha-<commit>`. A `vX.Y.Z` tag publishes `:X.Y.Z`, `:X.Y` and `:latest`.

Check what's running at `https://<your domain>/healthz`. The `version` field is the build, e.g. `v0.1.0-12-gabc1234` (12 commits after v0.1.0).

## Snapshot first

Datasets → the grower dataset → Create Snapshot (e.g. `pre-update-2026-10-14`). Database migrations run automatically on start and can't be undone without it.

## Tracking `:main` (current setup)

TrueNAS doesn't re-check a moving tag by itself. With `pull_policy: always` in the app's YAML (it's in `deploy/compose.yaml`), updating is:

1. Wait for the **Release image** workflow on GitHub to go green.
2. Apps → grower → **Edit → Save** (no changes needed). TrueNAS pulls the newest `:main` and recreates the container.

Without `pull_policy: always`, first re-pull the image (Apps → Configuration → Manage Container Images → Pull Image, or `sudo docker pull ghcr.io/reilandeubank/grower:main`), then Edit → Save.

## Pinned versions (once releases are tagged)

1. Tag a release on your laptop: `git tag v0.2.0 && git push origin v0.2.0`, and wait for the workflow.
2. Apps → grower → Edit, set `image:` to `ghcr.io/reilandeubank/grower:v0.2.0`, save.

Pinning makes rollback exact, which matters more once real data is in the app.

## Roll back

1. Edit the app back to the previous image (a `:sha-<commit>` or `vX.Y.Z` tag).
2. If the newer version ran a migration, stop the app and roll the dataset back to your snapshot before starting the older version. The binary refuses to start against a database newer than it understands, which is your signal that you need the snapshot.
