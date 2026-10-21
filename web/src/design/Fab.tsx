import { Plus } from "lucide-react";
import styles from "./Fab.module.css";

export interface FabProps {
  /** The accessible name, e.g. "Add plant". */
  label: string;
  /** Opens the add sheet. */
  onClick: () => void;
}

/** The floating add button, in thumb reach above the tab bar. */
export function Fab({ label, onClick }: FabProps) {
  return (
    <button type="button" className={styles.fab} aria-label={label} aria-haspopup="dialog" onClick={onClick}>
      <Plus size={24} strokeWidth={1.75} aria-hidden="true" />
    </button>
  );
}
