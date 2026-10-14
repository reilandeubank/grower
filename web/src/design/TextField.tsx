import { useId, useState, type InputHTMLAttributes } from "react";
import { CircleAlert, Eye, EyeOff } from "lucide-react";
import styles from "./TextField.module.css";

export interface TextFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "id"> {
  label: string;
  hint?: string;
  /** Shown under the field with an icon; also marks the input invalid. */
  error?: string | undefined;
  id?: string;
}

export function TextField({ label, hint, error, id, type = "text", className, ...rest }: TextFieldProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  const hintId = `${inputId}-hint`;
  const errorId = `${inputId}-error`;
  const [revealed, setRevealed] = useState(false);
  const isPassword = type === "password";
  const describedBy = [hint && hintId, error && errorId].filter(Boolean).join(" ") || undefined;

  return (
    <div className={[styles.field, className].filter(Boolean).join(" ")}>
      <label htmlFor={inputId} className={`t-caption ${styles.label}`}>
        {label}
      </label>
      <div className={styles.control}>
        <input
          {...rest}
          id={inputId}
          type={isPassword && revealed ? "text" : type}
          className={`t-body ${styles.input} ${isPassword ? styles.withToggle : ""}`}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
        />
        {isPassword && (
          <button
            type="button"
            className={styles.toggle}
            onClick={() => setRevealed((r) => !r)}
            aria-label={revealed ? "Hide password" : "Show password"}
            aria-controls={inputId}
          >
            {revealed ? (
              <EyeOff size={20} strokeWidth={1.75} aria-hidden="true" />
            ) : (
              <Eye size={20} strokeWidth={1.75} aria-hidden="true" />
            )}
          </button>
        )}
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
