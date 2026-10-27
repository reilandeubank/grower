# grower

A self-hosted plant care app for your phone.
Open it, see what your plants need today, tap done.
grower keeps the schedule, a photo timeline and a growth journal for every plant, and sends one daily reminder when something is due.

It's a single container with SQLite, made to be installed as an app on your phone's home screen (a PWA).

<p align="center">
  <img src="https://github.com/user-attachments/assets/6515453c-d77a-46a2-8626-95d15fae8301" width="30%" alt="grower screenshot 1" />
  &nbsp;
  <img src="https://github.com/user-attachments/assets/744636fa-8fb0-416d-b794-bdbc5e82d5b3" width="30%" alt="grower screenshot 2" />
  &nbsp;
  <img src="https://github.com/user-attachments/assets/90131beb-fe45-46bf-80c2-9d9411975c7e" width="30%" alt="grower screenshot 3" />
</p>

## Features

- **Today:** overdue, due and coming-up care in one list.
  Tap to mark done, with undo; snooze or skip.
- **Care schedules** for watering, fertilizing, misting, repotting, rotating and your own tasks.
  They count from when you last did it, or follow a fixed calendar.
- **Seasonal adjustment:** watering and feeding intervals stretch in winter and tighten in summer, for either hemisphere.
- **Moisture-check watering:** log moisture meter readings instead of watering on a timer, and grower learns how fast each plant dries out.
- **Light:** describe each room's windows and each plant's spot (distance from the window, grow lights).
  grower estimates the light and flags plants that likely need more.
- **Photos and journal:** a photo timeline, growth measurements with a height chart, and check-in reminders.
- **Species data (optional):** care defaults from Open Plantbook and Perenual, and identification from a photo with Pl@ntNet.
  Each needs a free API key, and grower works fully without them.
- **Daily reminder** by web push, at a time you choose, with a button to send it again later in the day.
- **Your data stays yours:** nightly database backups, a documented export format, and an import for moving servers.
- Light and dark themes, offline read-only mode, accessible on small screens.

## Quick start

You need Docker and a domain with HTTPS through a reverse proxy.
Push notifications and app install only work on a secure origin.

```sh
mkdir grower && cd grower
curl -O https://raw.githubusercontent.com/reilandeubank/grower/main/deploy/compose.yaml
# edit every value marked CHANGE, then:
mkdir data && sudo chown 1000:1000 data
docker compose up -d
docker compose exec grower /usr/local/bin/grower user create --username you
```

Point your reverse proxy at port 8080, open your domain, log in and install the app from your browser's menu.
The full guide, including a Caddy example and a TrueNAS SCALE + Nginx Proxy Manager walkthrough, is in [docs/deploy.md](docs/deploy.md).

## Configuration

All configuration is environment variables:

| Variable | Default | Meaning |
|---|---|---|
| `GROWER_BASE_URL` | (required) | The exact public URL. `https` turns on secure `__Host-` cookies. |
| `GROWER_DATA_DIR` | `/data` | Database, photos, backups and push keys. |
| `GROWER_LISTEN` | `:8080` | Listen address. |
| `GROWER_TRUSTED_PROXIES` | (none) | Comma-separated CIDRs whose `X-Forwarded-*` headers are trusted. |
| `GROWER_TZ` | `UTC` | Time zone for newly created users (changeable in Settings). |
| `GROWER_LOG_FORMAT` | `json` | `json` or `text`. |
| `GROWER_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. |
| `GROWER_OPENPLANTBOOK_CLIENT_ID` | (none) | Open Plantbook OAuth2 client ID. Set with the secret, or neither. |
| `GROWER_OPENPLANTBOOK_CLIENT_SECRET` | (none) | Open Plantbook client secret. |
| `GROWER_PERENUAL_API_KEY` | (none) | Perenual API key. grower spends at most 90 calls a day. |
| `GROWER_PLANTNET_API_KEY` | (none) | Pl@ntNet API key for identifying plants from a photo. grower spends at most 450 a day. |

## Documentation

- **Running it:** [deploy](docs/deploy.md) · [update](docs/update.md) · [backup and restore](docs/backup.md) · [export format](docs/export-format.md) · [changelog](CHANGELOG.md)
- **Working on it:** [development](docs/development.md) · [architecture](docs/architecture.md) · [contributing](CONTRIBUTING.md) · [visual design rules](design/README.md)

The stack is a Go server with an embedded React + TypeScript SPA, typed end to end from [api/openapi.yaml](api/openapi.yaml).

## Non-goals

- **Multiple households or public signup.**
  One grower is one household.
  Extra users can be added from the command line, and they share all plants.
- **Native mobile apps.**
  The installable web app covers it.
- **Social features, a plant shop or anything that needs a server other than yours.**

## About this project

grower exists because I had a problem: I wanted the care loop of apps like Planta without the subscription, running on my own home server.
The existing FOSS solutions didn't really fit what I was looking for, so I put together grower.
I've decided to share it here in case it solves a problem for you, too.

### A note on how it was made

Most of the code in this repository was written by AI coding agents rather than typed by me.
I built this to solve a problem I had, and that was reason enough.
If you’d rather not interact with AI-written code, that’s a completely fair decision, and I wanted you to be able to make it with full information.

### No warranty

grower is provided **as is, without warranty of any kind** (see [LICENSE](LICENSE)).
It works well for my plants and my setup.
I can't promise it will work for yours, that it will never lose data, or that I'll keep maintaining it.
Keep backups ([docs/backup.md](docs/backup.md)).
Care schedules and light estimates are rules of thumb, not botanical advice: when your plant disagrees, trust the plant.

## License

[MIT](LICENSE) © Reiland Eubank
