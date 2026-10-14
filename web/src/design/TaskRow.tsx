import { useState } from "react";
import { Clock } from "lucide-react";
import { DoneCheck } from "./DoneCheck";
import { PlantPhoto } from "./PlantCard";
import { TaskIcon, taskName, type CareKind } from "./TaskIcon";
import styles from "./TaskRow.module.css";

// The brand-soft flash, then the collapse (design README: Motion).
const FLASH_MS = 120;
const COLLAPSE_MS = 200;

export interface TaskRowProps {
  plantName: string;
  /** The plant cover's small image URL. */
  photo?: string | undefined;
  kind: CareKind;
  label?: string;
  /** "Due today", "Overdue 3 days", "In 3 days"… */
  due: string;
  /** Paints the due line in `overdue` with the clock icon. */
  overdue?: boolean;
  /** Smaller and flatter, for Coming up. */
  quiet?: boolean;
  /** Opens the task's actions (snooze, skip, open plant). */
  onOpen: () => void;
  /** Called once the row has played its completion, so the caller can drop it. */
  onComplete: () => void;
}

function prefersReducedMotion(): boolean {
  // jsdom has no matchMedia; tests then skip the animation.
  return window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? true;
}

export function TaskRow({ plantName, photo, kind, label, due, overdue = false, quiet = false, onOpen, onComplete }: TaskRowProps) {
  const [completing, setCompleting] = useState(false);
  const name = taskName(kind, label);

  const complete = () => {
    if (prefersReducedMotion()) {
      onComplete();
      return;
    }
    setCompleting(true);
    // Not cleared on unmount: a tap that started completing a task should still record it.
    window.setTimeout(onComplete, FLASH_MS + COLLAPSE_MS);
  };

  return (
    <li className={`${styles.item} ${completing ? styles.collapsing : ""}`}>
      <div className={styles.clip}>
        <div className={`${styles.row} ${quiet ? styles.quiet : ""} ${completing ? styles.completing : ""}`}>
          <button type="button" className={styles.body} onClick={onOpen} aria-haspopup="dialog">
            <PlantPhoto name={plantName} src={photo} size="thumb" />
            <span className={styles.text}>
              <span className={`t-body-strong ${styles.clamp}`}>{plantName}</span>
              <span className={`t-caption ${styles.meta}`}>
                <TaskIcon kind={kind} size={16} />
                <span className={styles.clamp}>{name}</span>
                <span aria-hidden="true">·</span>
                <span className={`${styles.due} ${overdue ? styles.overdue : ""}`}>
                  {overdue && <Clock size={16} strokeWidth={1.75} aria-hidden="true" />}
                  {due}
                </span>
              </span>
            </span>
          </button>
          <DoneCheck aria-label={`Mark ${name} ${plantName} done`} checked={completing} onClick={complete} />
        </div>
      </div>
    </li>
  );
}
