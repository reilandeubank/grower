import type { ReactNode } from "react";
import type { ButtonVariant } from "./Button";
import styles from "./Button.module.css";

export interface FileButtonProps {
  children: ReactNode;
  icon?: ReactNode;
  variant?: ButtonVariant;
  /** e.g. "image/*", which on Android offers the camera as well as the gallery. */
  accept?: string;
  multiple?: boolean;
  onFiles: (files: File[]) => void;
}

/** A file picker that looks like a Button: a label wrapping a hidden file input. */
export function FileButton({ children, icon, variant = "secondary", accept, multiple, onFiles }: FileButtonProps) {
  return (
    <label className={`${styles.button} ${styles[variant]} ${styles.file}`}>
      {icon}
      <span className={styles.label}>{children}</span>
      <input
        type="file"
        className="visually-hidden"
        accept={accept}
        multiple={multiple}
        onChange={(e) => {
          const files = Array.from(e.target.files ?? []);
          // Clear it so picking the same file again still fires a change.
          e.target.value = "";
          if (files.length) onFiles(files);
        }}
      />
    </label>
  );
}
