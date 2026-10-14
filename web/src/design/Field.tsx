import { useId, type ReactNode } from "react";
import { CircleAlert } from "lucide-react";
import styles from "./Field.module.css";

/** Props a field hands to its control so the label, hint and error are wired up. */
export interface ControlProps {
  id: string;
  "aria-invalid": true | undefined;
  "aria-describedby": string | undefined;
}

export interface FieldProps {
  label: string;
  hint?: string | undefined;
  /** Shown under the control with an icon; also marks the control invalid. */
  error?: string | undefined;
  id?: string | undefined;
  className?: string | undefined;
  children: (control: ControlProps) => ReactNode;
}

/** Label, hint and error around one control. TextField, SelectField and TextAreaField build on it. */
export function Field({ label, hint, error, id, className, children }: FieldProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  const hintId = `${inputId}-hint`;
  const errorId = `${inputId}-error`;
  const describedBy = [hint && hintId, error && errorId].filter(Boolean).join(" ") || undefined;

  return (
    <div className={[styles.field, className].filter(Boolean).join(" ")}>
      <label htmlFor={inputId} className={`t-caption ${styles.label}`}>
        {label}
      </label>
      <div className={styles.control}>
        {children({ id: inputId, "aria-invalid": error ? true : undefined, "aria-describedby": describedBy })}
      </div>
      {hint && (
        <p id={hintId} className={`t-caption ${styles.hint}`}>
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className={`t-caption ${styles.error}`}>
          <CircleAlert size={16} strokeWidth={1.75} aria-hidden="true" />
          {error}
        </p>
      )}
    </div>
  );
}

export { styles as fieldStyles };
