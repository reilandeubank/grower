import { CircleAlert } from "lucide-react";
import styles from "./ErrorText.module.css";

/** A failed action, announced as it appears: danger colour, alert icon and words. */
export function ErrorText({ children }: { children: string }) {
  return (
    <p className={`t-caption ${styles.error}`} role="alert">
      <CircleAlert size={16} strokeWidth={1.75} aria-hidden="true" />
      {children}
    </p>
  );
}
