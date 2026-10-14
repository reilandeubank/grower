import { Download } from "lucide-react";
import { usePlants } from "../plants/queries";
import { lastBackupLine, storageSummary } from "./copy";
import { useStorage } from "./queries";
import styles from "../routes/Settings.module.css";

/** Export, and how much space grower uses. */
export function DataSection() {
  const storage = useStorage();
  const plants = usePlants();

  return (
    <div className={styles.card}>
      {/* A plain same-origin link: the session cookie rides along and the browser saves the zip. */}
      <a href="/api/v1/export" download className={`${styles.row} ${styles.action}`}>
        <span className={styles.rowText}>
          <span className="t-body">Export data</span>{" "}
          <span className={`t-caption ${styles.muted} ${styles.block}`}>
            A zip of your plants, care history, settings and every photo.
          </span>
        </span>
        <Download size={20} strokeWidth={1.75} aria-hidden="true" className={styles.muted} />
      </a>
      <div className={styles.row}>
        <div className={styles.rowText}>
          <p className="t-body">Storage</p>
          <p className={`t-caption ${styles.muted}`}>
            {storage.isSuccess
              ? storageSummary(storage.data, plants.data?.length)
              : storage.isError
                ? "Couldn't load storage use."
                : "Loading…"}
          </p>
        </div>
      </div>
      {storage.isSuccess && (
        <div className={styles.row}>
          <p className="t-body">Last backup</p>
          <p className={`t-caption ${styles.muted}`}>{lastBackupLine(storage.data.last_backup_at)}</p>
        </div>
      )}
    </div>
  );
}
