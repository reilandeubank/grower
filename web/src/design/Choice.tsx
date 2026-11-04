import type { ReactNode } from "react";
import styles from "./Choice.module.css";

export interface ChoiceProps {
  title: string;
  detail?: string;
  icon?: ReactNode;
}

/**
 * A big tappable option's contents. Put it in an element with `choiceStyles.choice` (a button, a
 * link or a file input's label), and list several in a `choiceStyles.choices` list.
 */
export function Choice({ icon, title, detail }: ChoiceProps) {
  return (
    <>
      {icon && <span className={styles.icon}>{icon}</span>}
      <span className={styles.text}>
        <span className="t-body-strong">{title}</span>
        {detail && <span className={`t-caption ${styles.muted}`}>{detail}</span>}
      </span>
    </>
  );
}

export { styles as choiceStyles };
