# Backups and restore

Everything grower stores lives in its data directory, mounted at `/data` in the container ([deploy.md](deploy.md)).
The examples below use a TrueNAS dataset at `/mnt/tank/apps/grower` and the TrueNAS container name `ix-grower-grower-1`; substitute your own data directory and container (with Docker Compose, `docker compose exec grower ...`).

```
grower.db (+ -wal, -shm)       the live database
media/photos/<id>/             original photos and their thumbnails
backups/grower-YYYY-MM-DD.db   nightly consistent copies of the database
vapid.json                     the push notification key pair (keep it private)
```

`vapid.json` is covered by snapshots and off-box copies of the whole directory, but not by `backups/`.
Keep it when restoring: devices subscribed to the daily reminder are tied to its key, so losing it means each phone must turn Daily reminder off and on again in Settings.

## What protects it

1. **Filesystem snapshots** of the data directory, if your storage supports them (e.g. hourly and daily ZFS snapshots, [deploy.md](deploy.md#1-create-the-dataset)).
   Fast local undo for everything, including photos.
2. **Nightly database copies** in `backups/`, written by the app itself.
   Once a day after 03:00 in `GROWER_TZ` (and right away on a fresh install), grower runs `VACUUM INTO`, which gives a consistent, compact copy even while the app is in use.
   The newest 14 are kept.
   Each one logs `nightly backup written` with its size; a failure is logged and retried within the hour.
   Settings → Storage shows when the last one was written.
3. **An off-box copy you set up:** grower doesn't copy data off the machine itself.
   Sync the whole data directory somewhere else nightly, e.g. with a TrueNAS Cloud Sync task, rclone or restic.

The live `grower.db` in a snapshot or an off-box copy may have been caught mid-write, and recent changes may sit only in `grower.db-wal`.
A snapshot holds both files at the same instant, so it's crash-consistent and fine to roll back to.
A file-by-file copy isn't atomic, so **restore the database from `backups/`, never from an off-box copy of `grower.db`**.

## Restore from a nightly copy

Use this after losing or damaging the database, or to go back to a known-good day.
Changes made after that copy are lost.

1. Stop the app (`docker compose stop`, or Apps → grower → **Stop** on TrueNAS).
2. In a shell on the host, go to the data directory and keep the current files aside, just in case:

   ```sh
   cd /mnt/tank/apps/grower
   mkdir -p restore-aside && mv grower.db grower.db-wal grower.db-shm restore-aside/ 2>/dev/null
   ```

   (Restoring from an off-box copy? First download the `backups/grower-YYYY-MM-DD.db` you want, and `media/` if photos were lost, into this directory.)
3. Copy the backup into place and make sure no old `-wal`/`-shm` files remain (they belong to the old database and would corrupt the restored one):

   ```sh
   cp backups/grower-YYYY-MM-DD.db grower.db
   rm -f grower.db-wal grower.db-shm
   chown 568:568 grower.db   # the UID:GID the container runs as
   ```

4. Leave `media/` as it is.
   Photos added after the backup stay on disk without a row (harmless); photos deleted after it show as broken until you delete them again.
5. Start the app, log in and check Today and a plant's photos.
   Remove `restore-aside/` once you're happy.

If the backup came from an older grower version, the app migrates it on start.
If it's from a newer version than the one running, update the app first.

## Restore from a ZFS snapshot instead

A snapshot brings back the database and photos together, as of the snapshot time.
Best for "undo the last hour" or a bad update ([update.md](update.md#roll-back)).

- **Whole dataset:** stop the app, then Datasets → grower → Snapshots → the snapshot → **Rollback**.
  Rollback destroys newer snapshots, so clone first if you might need them.
  Start the app.
- **Single files:** every snapshot is readable under `/mnt/tank/apps/grower/.zfs/snapshot/<name>/`.
  With the app stopped, either take a `backups/` file from there and follow the steps above, or copy `grower.db` **together with** its `grower.db-wal` (if present) from the same snapshot over the live ones and delete `grower.db-shm`.

## Moving to a new server

Settings → Export data downloads a zip of everything ([export-format.md](export-format.md)).
`grower import` loads it into a new, empty grower.
It runs from the command line, because a big zip won't fit through the reverse proxy's upload limit.

1. Set up the new grower ([deploy.md](deploy.md)) and create your user, but don't add any plants: import only works into an empty grower.
2. Copy the zip into the container's data directory and import it:

   ```sh
   sudo docker cp grower-export-YYYY-MM-DD.zip ix-grower-grower-1:/data/
   sudo docker exec ix-grower-grower-1 /usr/local/bin/grower import /data/grower-export-YYYY-MM-DD.zip
   ```

   It prints what it loaded, e.g. `Imported 12 plants, 3 locations, 48 tasks, 512 events, 40 photos, 6 measurements, 4 species.` With several users, add `--username NAME`; that user gets the export's settings (time zone, reminder time, hemisphere).
3. Delete the zip from the dataset (the image has no shell or `rm`): `sudo rm /mnt/tank/apps/grower/grower-export-YYYY-MM-DD.zip`.

The import checks the whole file first (format, every reference, every photo's checksum) and then writes everything at once, so a failed import leaves the new grower empty; fix the cause and run it again.
Accounts, sessions and push subscriptions aren't in the export: sign in again and turn Daily reminder back on in Settings.

## Test it

Do a restore drill once after setting up, and after big changes: restore the newest nightly copy into a scratch directory and open it.

```sh
mkdir -p /tmp/grower-drill && cp /mnt/tank/apps/grower/backups/grower-YYYY-MM-DD.db /tmp/grower-drill/grower.db
sudo docker run --rm --user "$(id -u):$(id -g)" -v /tmp/grower-drill:/data ghcr.io/reilandeubank/grower:latest migrate
```

`migrate` lists every migration as `applied` when the copy is a valid grower database.
