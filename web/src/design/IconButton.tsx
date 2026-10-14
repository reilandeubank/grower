import type { ButtonHTMLAttributes, ReactNode } from "react";
import styles from "./IconButton.module.css";

export interface IconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label"> {
  /** Required: an icon alone has no accessible name. */
  "aria-label": string;
  icon: ReactNode;
}

export function IconButton({ icon, className, type = "button", ...rest }: IconButtonProps) {
  return (
    <button {...rest} type={type} className={[styles.iconButton, className].filter(Boolean).join(" ")}>
      {icon}
    </button>
  );
}
