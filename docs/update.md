# Updating grower

Every push to `main` publishes `ghcr.io/reilandeubank/grower:main` and `:sha-<commit>`. A `vX.Y.Z` tag publishes `:X.Y.Z`, `:X.Y` and `:latest`. The server should run a pinned version tag.

## Release (on your laptop)

```sh
git tag v0.2.0 && git push origin v0.2.0
```

Wait for the **Release image** workflow to go green in GitHub Actions.

## Update (on TrueNAS)

1. **Snapshot first.** Datasets → `tank/apps/grower` → Create Snapshot (name it `pre-v0.2.0`). Database migrations run automatically on start and can't be undone without it.
2. Apps → grower → Edit, change the `image:` tag to the new version, save. TrueNAS pulls the image and restarts the container.
3. Check `https://<your domain>/healthz` shows the new version, and open the app.

## Roll back

1. Edit the app back to the previous image tag.
2. If the new version ran a migration (the release notes say so), stop the app and roll the dataset back to the `pre-…` snapshot before starting the old version. The binary refuses to start against a database newer than it understands, which is your signal that you need the snapshot.
