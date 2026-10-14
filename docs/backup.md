# Backups and restore

Everything grower stores lives in its data directory: the dataset from [deploy.md](deploy.md), mounted at `/data` in the container. The paths below use `/mnt/tank/apps/grower`; use your dataset's mount path.

```
grower.db (+ -wal, -shm)       the live database
media/photos/<id>/             original photos and their thumbnails
backups/grower-YYYY-MM-DD.db   nightly consistent copies of the database
```

## What protects it

1. **ZFS snapshots** of the dataset (hourly and daily; [deploy.md §1](deploy.md#1-create-the-dataset)). Fast local undo for everything, including photos.
2. **Nightly database copies** in `backups/`, written by the app itself. Once a day after 03:00 in `GROWER_TZ` (and right away on a fresh install), grower runs `VACUUM INTO`, which gives a consistent, compact copy even while the app is in use. The newest 14 are kept. Each one logs `nightly backup written` with its size; a failure is logged and retried within the hour. Settings → Storage shows when the last one was written.
3. **Off-box copy to Google Drive:** a nightly TrueNAS Cloud Sync task copies the whole dataset (set up by the owner).

The live `grower.db` in a snapshot or the Drive copy may have been caught mid-write, and recent changes may sit only in `grower.db-wal`. A snapshot holds both files at the same instant, so it's crash-consistent and fine to roll back to. The Drive copy isn't atomic, so **restore the database from `backups/`, never from the Drive copy of `grower.db`**.

## Restore from a nightly copy

Use this after losing or damaging the database, or to go back to a known-good day. Changes made after that copy are lost.

1. Stop the app: Apps → grower → **Stop**.
2. In a TrueNAS shell, go to the data directory and keep the current files aside, just in case:

   ```sh
   cd /mnt/tank/apps/grower
   mkdir -p restore-aside && mv grower.db grower.db-wal grower.db-shm restore-aside/ 2>/dev/null
   ```

   (Restoring from Google Drive? First download the `backups/grower-YYYY-MM-DD.db` you want, and `media/` if photos were lost, into this directory.)
3. Copy the backup into place and make sure no old `-wal`/`-shm` files remain (they belong to the old database and would corrupt the restored one):

   ```sh
   cp backups/grower-YYYY-MM-DD.db grower.db
   rm -f grower.db-wal grower.db-shm
   chown 568:568 grower.db
   ```

4. Leave `media/` as it is. Photos added after the backup stay on disk without a row (harmless); photos deleted after it show as broken until you delete them again.
5. Start the app, log in and check Today and a plant's photos. Remove `restore-aside/` once you're happy.

If the backup came from an older grower version, the app migrates it on start. If it's from a newer version than the one running, update the app first.

## Restore from a ZFS snapshot instead

A snapshot brings back the database and photos together, as of the snapshot time. Best for "undo the last hour" or a bad update ([update.md](update.md#roll-back)).

- **Whole dataset:** stop the app, then Datasets → grower → Snapshots → the snapshot → **Rollback**. Rollback destroys newer snapshots, so clone first if you might need them. Start the app.
- **Single files:** every snapshot is readable under `/mnt/tank/apps/grower/.zfs/snapshot/<name>/`. With the app stopped, either take a `backups/` file from there and follow the steps above, or copy `grower.db` **together with** its `grower.db-wal` (if present) from the same snapshot over the live ones and delete `grower.db-shm`.

## Test it

Do a restore drill once after setting up, and after big changes: restore the newest nightly copy into a scratch directory and open it.

```sh
mkdir -p /tmp/grower-drill && cp /mnt/tank/apps/grower/backups/grower-YYYY-MM-DD.db /tmp/grower-drill/grower.db
sudo docker run --rm --user "$(id -u):$(id -g)" -v /tmp/grower-drill:/data ghcr.io/reilandeubank/grower:main migrate
```

`migrate` lists every migration as `applied` when the copy is a valid grower database.
