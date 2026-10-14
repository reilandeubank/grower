import { useId, type ReactNode } from "react";
import styles from "./SegmentedControl.module.css";

export interface SegmentedControlProps<T extends string> {
  /** Shown above the control and names the group. */
  label: string;
  options: { value: T; label: string; icon?: ReactNode }[];
  value: T;
  onChange: (value: T) => void;
}

/** One choice of a few, side by side. Native radios: arrow keys and screen readers work as usual. */
export function SegmentedControl<T extends string>({ label, options, value, onChange }: SegmentedControlProps<T>) {
  const name = useId();
  return (
    <fieldset className={styles.fieldset}>
      <legend className={`t-caption ${styles.legend}`}>{label}</legend>
      <div className={styles.track}>
        {options.map((o) => (
          <label key={o.value} className={`t-caption ${styles.option}`}>
            <input
              type="radio"
              className={styles.input}
              name={name}
              value={o.value}
              checked={o.value === value}
              onChange={() => onChange(o.value)}
            />
            {o.icon}
            {o.label}
          </label>
        ))}
      </div>
    </fieldset>
  );
}
