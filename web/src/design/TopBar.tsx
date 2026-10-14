import type { ReactNode } from "react";
import styles from "./TopBar.module.css";

export interface TopBarProps {
  title: string;
  /** Icon buttons or a ghost button, aligned right. */
  actions?: ReactNode;
}

export function TopBar({ title, actions }: TopBarProps) {
  return (
    <header className={styles.topBar}>
      <div className={styles.inner}>
        <h1 className={`t-title ${styles.title}`}>{title}</h1>
        {actions && <div className={styles.actions}>{actions}</div>}
      </div>
    </header>
  );
}
