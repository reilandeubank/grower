import { Check } from "lucide-react";
import styles from "./DoneCheck.module.css";

export interface DoneCheckProps {
  /** e.g. "Mark Water Monstera done". */
  "aria-label": string;
  /** Filled while the task is being completed. */
  checked?: boolean;
  disabled?: boolean;
  onClick: () => void;
}

/** The round done check: a full 48px tap target around a smaller visible circle. */
export function DoneCheck({ "aria-label": label, checked = false, disabled, onClick }: DoneCheckProps) {
  return (
    <button
      type="button"
      className={styles.check}
      aria-label={label}
      data-checked={checked || undefined}
      disabled={disabled || checked}
      onClick={onClick}
    >
      <span className={styles.circle}>
        <Check size={20} strokeWidth={1.75} aria-hidden="true" />
      </span>
    </button>
  );
}
