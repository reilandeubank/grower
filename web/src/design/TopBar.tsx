import type { ReactNode } from "react";
import styles from "./TopBar.module.css";

export interface TopBarProps {
  /** The screen's h1. Leave it out when the page sets its own (plant detail). */
  title?: string;
  /** A back or close icon button, aligned left. */
  leading?: ReactNode;
  /** Icon buttons or a ghost button, aligned right. */
  actions?: ReactNode;
}

export function TopBar({ title, leading, actions }: TopBarProps) {
  return (
    <header className={styles.topBar}>
      <div className={styles.inner}>
        {leading && <div className={styles.leading}>{leading}</div>}
        {title ? <h1 className={`t-title ${styles.title}`}>{title}</h1> : <div className={styles.title} />}
        {actions && <div className={styles.actions}>{actions}</div>}
      </div>
    </header>
  );
}
