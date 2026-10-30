# Deploying grower

grower is one container with one data directory.
It must be served over **HTTPS** through a reverse proxy: browsers only allow web push (the daily reminder) and app installation on secure origins, and grower's session cookie is HTTPS-only outside local development.

This guide covers any Docker host first, then a full walkthrough for [TrueNAS SCALE with Nginx Proxy Manager](#truenas-scale-and-nginx-proxy-manager).
Images are published for `linux/amd64` and `linux/arm64` (a Raspberry Pi 4 or newer works).

## Docker Compose

1. Copy [`deploy/compose.yaml`](../deploy/compose.yaml) to a new directory and edit every value marked `CHANGE`:
   - `GROWER_BASE_URL`: the exact public URL, e.g. `https://grower.example.com`.
     Cookies and the CSRF check depend on it, so scheme, host and port must match what the browser shows.
   - `GROWER_TZ`: your [IANA time zone](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones), e.g. `Europe/Berlin`.
     It's the fallback for new users; each user can change it in Settings.
   - `user`: a UID:GID that owns the data directory.
     The image runs as non-root, so `mkdir data && sudo chown 1000:1000 data` (or your own `id -u`:`id -g`) before the first start.
   - `GROWER_TRUSTED_PROXIES`: see [Trusted proxies](#trusted-proxies).
2. Start it: `docker compose up -d`.
3. Once your [reverse proxy](#https-with-a-reverse-proxy) is set up, create your account (once).
   While grower has no account, it prints a setup code in its log at startup:

   ```sh
   docker compose logs grower | grep "setup code"
   ```

   Open `GROWER_BASE_URL` in a browser, enter the code, and choose a username and a password (10 characters minimum).
   You're signed in right away, and the setup page is gone for good.
   See [First account](#first-account) for the details and alternatives.

### HTTPS with a reverse proxy

Any reverse proxy that terminates TLS and forwards to port 8080 works.
With [Caddy](https://caddyserver.com/), which fetches certificates automatically, the whole config is:

```
grower.example.com {
	reverse_proxy localhost:8080
	request_body {
		max_size 20MB
	}
}
```

Whatever you use, allow request bodies of at least 20 MB for photo uploads and plant identification.

### Trusted proxies

grower trusts `X-Forwarded-For` and `X-Forwarded-Proto` **only** from addresses in `GROWER_TRUSTED_PROXIES` (comma-separated CIDRs).
To find the right value:

1. Open the site through your proxy once.
2. Check the logs (`docker compose logs grower`).
   Each request line shows the peer address the proxy connected from.
3. Set `GROWER_TRUSTED_PROXIES` to that address as a `/32`, or to the Docker network's subnet, and restart.

If it's wrong, everything still works, but login rate limiting treats every visitor as the proxy's IP.

## Check it

- `https://grower.example.com/healthz` returns `{"status":"ok",...}`.
- Log in on your phone, then install the app.
  In Chrome on Android (what grower is built and tested on): **⋮ → Add to Home screen → Install**.
  Safari on iOS (**Share → Add to Home Screen**) is untested.
- Close and reopen the installed app: you should still be signed in.
  Sessions last 90 days from your last visit.
- In the installed app, **Settings → Notifications → Daily reminder** on, allow notifications, then **Send test notification**.

Everything grower stores lives in the data directory: `grower.db` (+ `-wal`/`-shm`), `media/`, `backups/` and `vapid.json` (the push notification keys, made on first start).
Back it up: see [backup.md](backup.md).

## Species data (optional)

The plant form can search online species databases for care defaults (watering interval, light, the moisture level to water at), and quick add can suggest a species from a photo.
Every provider is optional; without them, species still work by hand.

1. **Open Plantbook:** on open.plantbook.io, open **API keys** and generate a client ID and secret.
2. **Perenual:** copy your API key from perenual.com.
3. **Pl@ntNet:** create a free account on my.plantnet.org and copy your API key (it identifies plants from a photo).
4. Uncomment and fill in the lines you need under `environment:` in your compose file, then restart:

   ```yaml
   GROWER_OPENPLANTBOOK_CLIENT_ID: your-client-id
   GROWER_OPENPLANTBOOK_CLIENT_SECRET: your-client-secret
   GROWER_PERENUAL_API_KEY: your-key
   GROWER_PLANTNET_API_KEY: your-key
   ```

5. The startup log lists `species_providers` (names only; keys are never logged) and `plant_identification` (true when Pl@ntNet is set up).

Perenual's free tier allows 100 calls a day.
grower spends at most 90 a day (counted per UTC day, reset when the container restarts), caches each search for 24 hours, and only calls when you search or import.
When the budget is spent, the search sheet says so and Open Plantbook still answers.
Saved species are never refreshed automatically.

The free tier also only serves species ids 1–3000, so many popular houseplants (Monstera deliciosa among them) never appear in Perenual results; grower hides ids it can't fetch.
Open Plantbook covers them, but gives light and soil-moisture ranges rather than a watering interval.
Set the interval on the species yourself when that's the only source.

Pl@ntNet's free tier allows 500 identifications a day.
grower spends at most 450 (counted per UTC day, reset when the container restarts) and only calls when you ask it to identify a photo.
The photo is forwarded and never stored by grower; Pl@ntNet's terms say submitted images aren't kept either.
Suggestions only fill in the name: care defaults still come from the providers above or from you.

## TrueNAS SCALE and Nginx Proxy Manager

A walkthrough for TrueNAS SCALE 24.10 or newer (Docker-based apps) with Nginx Proxy Manager (NPM) already running.

### 1. Create the dataset

1. **Datasets → Add Dataset** under your apps pool, e.g. `tank/apps/grower`.
   Preset: _Apps_.
2. Set the owner to the `apps` user and group (UID/GID 568).
3. **(Recommended) Data Protection → Periodic Snapshot Tasks:** add hourly snapshots kept for 2 days and daily snapshots kept for 30 days on this dataset.

### 2. Install the app

1. Copy `deploy/compose.yaml` and edit it as in [Docker Compose](#docker-compose), with these TrueNAS values:
   - `user: "568:568"` (the `apps` user that owns the dataset).
   - The volume's host path: the dataset's mount path, e.g. `/mnt/tank/apps/grower:/data`.
   - Change the published port if 8080 is taken, e.g. `"18080:8080"`.
   - Optionally add `pull_policy: always` if you track a moving tag like `:latest` ([update.md](update.md)).
2. **Apps → Discover Apps → ⋮ → Install via YAML**, name it `grower`, paste the file, save ([TrueNAS docs: custom app screens](https://www.truenas.com/docs/scale/25.04/scaleuireference/apps/installcustomappscreens/#add-custom-app-screen)).
3. Open the app's logs from **Apps → grower** and note the setup code in the line starting `No account yet`.
   You'll use it in [step 4](#4-create-your-account).

### 3. Add an NPM proxy host

| Tab      | Setting                                                                                                                                                                         |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Details  | Domain: `grower.example.com`. Scheme `http`. Forward host: the TrueNAS IP and the published port (or, on a shared Docker network, the container name and port `8080`). Enable _Block Common Exploits_. |
| SSL      | Request a Let's Encrypt certificate. Enable _Force SSL_, _HTTP/2_ and _HSTS_.                                                                                                   |
| Advanced | `client_max_body_size 20m;`                                                                                                                                                     |

NPM already sends `X-Forwarded-For` and `X-Forwarded-Proto`; set `GROWER_TRUSTED_PROXIES` as described in [Trusted proxies](#trusted-proxies) (Apps → grower → Edit).

### 4. Create your account

Open `https://grower.example.com`, enter the setup code from step 2, and choose a username and password.
If the app restarted in between (editing it restarts it, for example), the code has changed: check the logs again.

## First account

While grower has no account, every start prints a new setup code at warn level:

```
No account yet. Open https://grower.example.com and enter setup code K7Q2M-X9PB4
```

Whoever opens `GROWER_BASE_URL` first sees a **Create your account** page, which asks for that code, so only someone who can read the server's logs can claim the instance.
The code ignores case, spaces and dashes, is kept in memory only, and tries share the login rate limit.
Once the account exists, the page and its endpoint are gone for good.

- To choose the code yourself (from a compose secret, say), set `GROWER_SETUP_CODE` to 8 or more characters.
  grower then logs that a code is set but never the code itself.
- You can still create the account from the command line instead, and that's how to add more users later (everyone shares the same plants):

  ```sh
  docker compose exec grower /usr/local/bin/grower user create --username <you>
  ```

## Troubleshooting

- **Container restarts with a permission error:** the data directory isn't writable by the container's `user`.
  Fix its owner.
- **Login returns 403:** `GROWER_BASE_URL` doesn't exactly match the URL in the browser (scheme, host and port).
- **Unhealthy container:** check the logs.
  `grower healthcheck` calls `/healthz` inside the container.
- **No reminder arrives:** the log says `daily reminder attempt failed` or `push subscription gone` when the push service refuses.
  If `vapid.json` was lost or replaced, every device must turn Daily reminder off and on again.
  The reminder is sent once a day at the chosen time (in the time zone from Settings) and only when something is due.
  Over plain http the browser offers no push at all.
