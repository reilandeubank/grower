import type { ReactNode } from "react";
import styles from "./Switch.module.css";

export interface SwitchProps {
  label: string;
  /** A line under the label: what the switch does, or why it can't be used. */
  description?: ReactNode;
  checked: boolean;
  disabled?: boolean;
  /** While a change is in flight: the switch stays put and is marked busy. */
  busy?: boolean;
  onChange: (checked: boolean) => void;
  className?: string;
}

/** An on/off setting as a full-width row. A native checkbox with role="switch". */
export function Switch({
  label,
  description,
  checked,
  disabled = false,
  busy = false,
  onChange,
  className,
}: SwitchProps) {
  return (
    <label className={[styles.row, className].filter(Boolean).join(" ")} aria-busy={busy || undefined}>
      <span className={styles.text}>
        <span className="t-body">{label}</span>
        {description && <span className={`t-caption ${styles.description}`}>{description}</span>}
      </span>
      <input
        type="checkbox"
        role="switch"
        className={styles.switch}
        checked={checked}
        disabled={disabled || busy}
        onChange={(e) => onChange(e.target.checked)}
      />
    </label>
  );
}
