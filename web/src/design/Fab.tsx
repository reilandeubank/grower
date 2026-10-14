import { Link } from "react-router";
import { Plus } from "lucide-react";
import styles from "./Fab.module.css";

export interface FabProps {
  to: string;
  /** The accessible name, e.g. "Add plant". */
  label: string;
}

/** The floating add button, in thumb reach above the tab bar. */
export function Fab({ to, label }: FabProps) {
  return (
    <Link to={to} className={styles.fab} aria-label={label}>
      <Plus size={24} strokeWidth={1.75} aria-hidden="true" />
    </Link>
  );
}
