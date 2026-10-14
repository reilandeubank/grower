import type { SelectHTMLAttributes } from "react";
import { ChevronDown } from "lucide-react";
import { Field, fieldStyles as styles } from "./Field";

export interface SelectFieldProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, "id"> {
  label: string;
  hint?: string;
  error?: string | undefined;
  id?: string;
  options: { value: string; label: string }[];
}

/** A native select, so phones show their own picker. */
export function SelectField({ label, hint, error, id, className, options, ...rest }: SelectFieldProps) {
  return (
    <Field label={label} hint={hint} error={error} id={id} className={className}>
      {(control) => (
        <>
          <select {...rest} {...control} className={`t-body ${styles.input} ${styles.select}`}>
            {options.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
          <ChevronDown className={styles.chevron} size={20} strokeWidth={1.75} aria-hidden="true" />
        </>
      )}
    </Field>
  );
}
