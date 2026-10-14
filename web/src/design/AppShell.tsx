import type { ReactNode } from "react";
import { BottomTabBar } from "./BottomTabBar";
import styles from "./AppShell.module.css";

export interface AppShellProps {
  /** Usually a <TopBar>. Without one, the content gets the safe-area top inset itself. */
  topBar?: ReactNode;
  /** Full-screen pages (long forms) hide the tab bar so back simply leaves them. */
  tabBar?: boolean;
  children: ReactNode;
}

export function AppShell({ topBar, tabBar = true, children }: AppShellProps) {
  const contentClasses = [styles.content, !topBar && styles.noTopBar, !tabBar && styles.noTabBar].filter(Boolean);
  return (
    <div className={styles.shell}>
      <div className={`${styles.main} ${tabBar ? styles.withTabBar : ""}`}>
        {topBar}
        <main className={contentClasses.join(" ")}>{children}</main>
      </div>
      {tabBar && <BottomTabBar />}
    </div>
  );
}
