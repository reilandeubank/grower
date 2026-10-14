import type { TextareaHTMLAttributes } from "react";
import { Field, fieldStyles as styles } from "./Field";

export interface TextAreaFieldProps extends Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, "id"> {
  label: string;
  hint?: string;
  error?: string | undefined;
  id?: string;
}

export function TextAreaField({ label, hint, error, id, className, rows = 4, ...rest }: TextAreaFieldProps) {
  return (
    <Field label={label} hint={hint} error={error} id={id} className={className}>
      {(control) => <textarea {...rest} {...control} rows={rows} className={`t-body ${styles.input} ${styles.textarea}`} />}
    </Field>
  );
}
