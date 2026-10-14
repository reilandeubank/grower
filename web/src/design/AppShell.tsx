import type { ReactNode } from "react";
import { BottomTabBar } from "./BottomTabBar";
import styles from "./AppShell.module.css";

export interface AppShellProps {
  /** Usually a <TopBar>. Without one, the content gets the safe-area top inset itself. */
  topBar?: ReactNode;
  children: ReactNode;
}

export function AppShell({ topBar, children }: AppShellProps) {
  return (
    <div className={styles.shell}>
      <div className={styles.main}>
        {topBar}
        <main className={`${styles.content} ${topBar ? "" : styles.noTopBar}`}>{children}</main>
      </div>
      <BottomTabBar />
    </div>
  );
}
