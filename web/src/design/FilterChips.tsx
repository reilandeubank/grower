import { Check } from "lucide-react";
import styles from "./FilterChips.module.css";

export interface FilterChipsProps {
  /** Names the group for screen readers, e.g. "Filter by location". */
  label: string;
  options: { value: string; label: string }[];
  value: string;
  onChange: (value: string) => void;
}

/** A horizontally scrolling row of toggle chips where exactly one is selected. */
export function FilterChips({ label, options, value, onChange }: FilterChipsProps) {
  return (
    <div role="group" aria-label={label} className={styles.row}>
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
              {selected && <Check size={16} strokeWidth={1.75} aria-hidden="true" />}
              {o.label}
            </span>
          </button>
        );
      })}
    </div>
  );
}
