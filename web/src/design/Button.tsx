import type { ComponentPropsWithRef, ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import styles from "./Button.module.css";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";

export interface ButtonProps extends ComponentPropsWithRef<"button"> {
  variant?: ButtonVariant;
  fullWidth?: boolean;
  /** Shows a spinner, keeps the label for screen readers and blocks presses. */
  loading?: boolean;
  icon?: ReactNode;
}

export function Button({
  variant = "primary",
  fullWidth = false,
  loading = false,
  icon,
  disabled,
  className,
  children,
  type = "button",
  ...rest
}: ButtonProps) {
  const classes = [styles.button, styles[variant], fullWidth && styles.full, loading && styles.loading, className]
    .filter(Boolean)
    .join(" ");
  return (
    <button
      {...rest}
      type={type}
      className={classes}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
    >
      {loading ? (
        <LoaderCircle className={styles.spinner} size={20} strokeWidth={1.75} aria-hidden="true" />
      ) : (
        icon
      )}
      <span className={styles.label}>{children}</span>
    </button>
  );
}
