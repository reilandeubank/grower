import { useEffect, useState } from "react";
import styles from "./Pending.module.css";

/** Shows nothing for the first 300ms so fast loads don't flash, then a quiet indicator. */
export function Pending() {
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    const id = window.setTimeout(() => setVisible(true), 300);
    return () => window.clearTimeout(id);
  }, []);
  return (
    <div className={styles.pending} role="status" aria-live="polite">
      {visible && (
        <>
          <span className={styles.dots} aria-hidden="true">
            <span />
            <span />
            <span />
          </span>
          <span className="visually-hidden">Loading</span>
        </>
      )}
    </div>
  );
}
