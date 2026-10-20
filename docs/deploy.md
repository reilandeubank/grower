# Deploying grower on TrueNAS SCALE

grower runs as one container with one data directory. This guide puts it on a dedicated ZFS dataset, runs it as a TrueNAS custom app from `deploy/compose.yaml`, and exposes it through Nginx Proxy Manager (NPM) with TLS.

Assumes TrueNAS SCALE 24.10 or newer (Docker-based apps) and NPM already running.

## 1. Create the dataset

1. **Datasets → Add Dataset** under your apps pool, e.g. `tank/apps/grower`. Preset: _Apps_.
2. Set the owner to the `apps` user and group (UID/GID 568). The container runs as `568:568` and needs write access.
3. **Data Protection → Periodic Snapshot Tasks:** add hourly snapshots kept for 2 days and daily snapshots kept for 30 days on this dataset.

Everything grower stores lives here: `grower.db` (+ `-wal`/`-shm`), `media/`, `backups/` and `vapid.json` (the push notification keys, made on first start). For what is backed up and how to restore, see [backup.md](backup.md).

## 2. Let TrueNAS pull the image

The repo is private, so its GHCR package is private by default. Pick one:

- **Make the package public** (simplest; the image contains no secrets or personal data): GitHub → your profile → Packages → `grower` → Package settings → Change visibility → Public.
- **Keep it private:** create a classic GitHub personal access token with only `read:packages`. In TrueNAS, add it as a registry credential for `ghcr.io` (Apps → Configuration → Manage Container Image Registries, on recent versions). Username is your GitHub username, password is the token.

## 3. Install the app

1. Copy `deploy/compose.yaml` and edit every line marked `CHANGE`:
   - `image`: the version to run, e.g. `ghcr.io/reilandeubank/grower:v0.1.0` (or `:main` while testing).
   - `GROWER_BASE_URL`: the exact public URL, e.g. `https://grower.example.com`. Cookies and the CSRF check depend on it.
   - `GROWER_TZ`: your IANA time zone, e.g. `America/Chicago`.
   - The volume's host path: the dataset's mount path, e.g. `/mnt/tank/apps/grower`.
2. **Apps → Discover Apps → ⋮ → Install via YAML**, name it `grower`, paste the file, save.
3. Create your account (once). From a TrueNAS shell:

   ```sh
   sudo docker exec -it ix-grower-grower-1 /usr/local/bin/grower user create --username <you>
   ```

   You'll be asked for a password twice (10 characters minimum). There is no signup page.

## 4. Nginx Proxy Manager

Add a **Proxy Host**:

| Tab      | Setting                                                                                                                                                                                                              |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Details  | Domain: `grower.example.com`. Scheme `http`. Forward host: the TrueNAS IP, port `18080` (or, on a shared Docker network, the container name and port `8080`). Enable _Block Common Exploits_. Websockets not needed. |
| SSL      | Request a Let's Encrypt certificate. Enable _Force SSL_, _HTTP/2_ and _HSTS_.                                                                                                                                        |
| Advanced | `client_max_body_size 20m;` (photo uploads arrive in M4)                                                                                                                                                             |

NPM already sends `X-Forwarded-For` and `X-Forwarded-Proto`. grower trusts those headers **only** from addresses in `GROWER_TRUSTED_PROXIES`. To find the right value:

1. Open the site through NPM once.
2. Check the grower app logs. Each request line shows the peer address NPM connected from.
3. Set `GROWER_TRUSTED_PROXIES` to that address as a `/32`, or to the Docker network subnet it belongs to, and redeploy.

If the setting is wrong, everything still works, but login rate limiting treats every visitor as the proxy's IP.

## 5. Check it

- `https://grower.example.com/healthz` returns `{"status":"ok",...}`.
- Log in on your phone in Chrome, then **⋮ → Add to Home screen → Install**.
- Close and reopen the installed app: you should still be signed in. Sessions last 90 days from your last visit.
- In the installed app, **Settings → Notifications → Daily reminder** on, allow notifications, then **Send test notification**. On Android the reminder arrives even when the app is closed. Web push needs HTTPS, which the NPM proxy host already provides; over plain http the browser offers no push at all.

## Species data (optional)

The plant form can search online species databases for care defaults (watering interval, light, the moisture level to water at). Both providers are optional; without either, species still work by hand.

1. **Open Plantbook:** on open.plantbook.io, open **API keys** and generate a client ID and secret.
2. **Perenual:** copy your API key from perenual.com.
3. In TrueNAS, **Apps → grower → Edit**, and in the YAML under `environment:` uncomment and fill in the lines you need (see `deploy/compose.yaml`):

   ```yaml
   GROWER_OPENPLANTBOOK_CLIENT_ID: your-client-id
   GROWER_OPENPLANTBOOK_CLIENT_SECRET: your-client-secret
   GROWER_PERENUAL_API_KEY: your-key
   ```

4. Save. The startup log lists `species_providers` (names only; keys are never logged).

Perenual's free tier allows 100 calls a day. grower spends at most 90 a day (counted per UTC day, reset when the container restarts), caches each search for 24 hours, and only calls when you search or import. When the budget is spent, the search sheet says so and Open Plantbook still answers. Saved species are never refreshed automatically.

The free tier also only serves species ids 1–3000, so many popular houseplants (Monstera deliciosa among them) never appear in Perenual results; grower hides ids it can't fetch. Open Plantbook covers them, but gives light and soil-moisture ranges rather than a watering interval. Set the interval on the species yourself when that's the only source.

## Troubleshooting

- **Container restarts with a permission error:** the dataset isn't writable by 568:568. Fix the dataset ACL/owner.
- **Login returns 403:** `GROWER_BASE_URL` doesn't exactly match the URL in the browser (scheme, host and port).
- **Unhealthy container:** check logs. `grower healthcheck` calls `/healthz` inside the container.
- **No reminder arrives:** the app log says `daily reminder attempt failed` or `push subscription gone` when the push service refuses. If `vapid.json` was lost or replaced, every device must turn Daily reminder off and on again. The reminder is sent once a day at the chosen time (in the time zone from Settings) and only when something is due.
