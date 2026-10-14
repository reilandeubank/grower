import { useState, type ComponentPropsWithRef } from "react";
import { Eye, EyeOff } from "lucide-react";
import { Field, fieldStyles as styles } from "./Field";

export interface TextFieldProps extends Omit<ComponentPropsWithRef<"input">, "id"> {
  label: string;
  hint?: string;
  /** Shown under the field with an icon; also marks the input invalid. */
  error?: string | undefined;
  id?: string;
}

export function TextField({ label, hint, error, id, type = "text", className, ...rest }: TextFieldProps) {
  const [revealed, setRevealed] = useState(false);
  const isPassword = type === "password";

  return (
    <Field label={label} hint={hint} error={error} id={id} className={className}>
      {(control) => (
        <>
          <input
            {...rest}
            {...control}
            type={isPassword && revealed ? "text" : type}
            className={`t-body ${styles.input} ${isPassword ? styles.withToggle : ""}`}
          />
          {isPassword && (
            <button
              type="button"
              className={styles.toggle}
              onClick={() => setRevealed((r) => !r)}
              aria-label={revealed ? "Hide password" : "Show password"}
              aria-controls={control.id}
            >
              {revealed ? (
                <EyeOff size={20} strokeWidth={1.75} aria-hidden="true" />
              ) : (
                <Eye size={20} strokeWidth={1.75} aria-hidden="true" />
              )}
            </button>
          )}
        </>
      )}
    </Field>
  );
}
