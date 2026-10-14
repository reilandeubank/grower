import { useId, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import styles from "./Tabs.module.css";

export interface TabsProps {
  /** Names the tab list for screen readers. */
  label: string;
  tabs: { label: string; panel: ReactNode }[];
}

/** Segmented tabs. Arrow keys move between tabs, as the ARIA tabs pattern expects. */
export function Tabs({ label, tabs }: TabsProps) {
  const [selected, setSelected] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);
  const baseId = useId();

  const onKeyDown = (e: KeyboardEvent) => {
    const step = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
    if (!step) return;
    const next = (selected + step + tabs.length) % tabs.length;
    setSelected(next);
    listRef.current?.querySelectorAll<HTMLElement>('[role="tab"]')[next]?.focus();
  };

  return (
    <div className={styles.tabs}>
      <div ref={listRef} role="tablist" aria-label={label} className={styles.list} onKeyDown={onKeyDown}>
        {tabs.map((t, i) => (
          <button
            key={t.label}
            type="button"
            role="tab"
            id={`${baseId}-tab-${i}`}
            aria-selected={i === selected}
            aria-controls={`${baseId}-panel`}
            tabIndex={i === selected ? 0 : -1}
            className={`t-caption ${styles.tab}`}
            onClick={() => setSelected(i)}
          >
            {t.label}
          </button>
        ))}
      </div>
      <div role="tabpanel" id={`${baseId}-panel`} aria-labelledby={`${baseId}-tab-${selected}`}>
        {tabs[selected]?.panel}
      </div>
    </div>
  );
}
