import type { ReactNode } from "react";
import styles from "./EmptyState.module.css";

export interface EmptyStateProps {
  icon: ReactNode;
  title: string;
  body: string;
  action?: ReactNode;
  /** Heading level for the title, so it fits the screen's outline. */
  as?: "h2" | "h3";
}

export function EmptyState({ icon, title, body, action, as: Heading = "h2" }: EmptyStateProps) {
  return (
    <section className={styles.empty}>
      <div className={styles.icon} aria-hidden="true">
        {icon}
      </div>
      <Heading className={`t-heading ${styles.title}`}>{title}</Heading>
      <p className={`t-body ${styles.body}`}>{body}</p>
      {action && <div className={styles.action}>{action}</div>}
    </section>
  );
}
