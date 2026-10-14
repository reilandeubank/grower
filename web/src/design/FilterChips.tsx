import type { ReactNode } from "react";
import { Check } from "lucide-react";
import styles from "./FilterChips.module.css";

export interface FilterChipsProps {
  /** Names the group for screen readers, e.g. "Filter by location". */
  label: string;
  /** An option's icon replaces the check mark: it shows whether or not the chip is selected. */
  options: { value: string; label: string; icon?: ReactNode }[];
  value: string;
  onChange: (value: string) => void;
  /** Wrap onto more lines instead of scrolling sideways (for choices inside a form). */
  wrap?: boolean;
}

/** A row of toggle chips where exactly one is selected. */
export function FilterChips({ label, options, value, onChange, wrap = false }: FilterChipsProps) {
  return (
    <div role="group" aria-label={label} className={`${styles.row} ${wrap ? styles.wrap : ""}`}>
      {options.map((o) => {
        const selected = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            className={styles.chip}
            aria-pressed={selected}
            onClick={() => onChange(o.value)}
          >
            <span className={`t-caption ${styles.pill}`}>
              {o.icon ?? (selected && <Check size={16} strokeWidth={1.75} aria-hidden="true" />)}
              {o.label}
            </span>
          </button>
        );
      })}
    </div>
  );
}
