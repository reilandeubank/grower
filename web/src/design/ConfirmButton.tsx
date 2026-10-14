import { useEffect, useId, useRef, useState } from "react";
import { CircleAlert, Trash2 } from "lucide-react";
import { Button } from "./Button";
import styles from "./ConfirmButton.module.css";

export interface ConfirmButtonProps {
  /** The first-step button, e.g. "Delete plant". */
  label: string;
  /** Asked in place, e.g. "Delete plant? This can't be undone." */
  question: string;
  /** The confirming button; defaults to "Delete". */
  confirmLabel?: string;
  loading?: boolean;
  onConfirm: () => void;
}

/** A destructive action that asks for confirmation in place (no browser dialogs). */
export function ConfirmButton({ label, question, confirmLabel = "Delete", loading, onConfirm }: ConfirmButtonProps) {
  const [asking, setAsking] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const cancelRef = useRef<HTMLButtonElement>(null);
  const wasAsking = useRef(false);
  const questionId = useId();

  // Move focus into the question, and back to the trigger on cancel.
  useEffect(() => {
    if (asking) cancelRef.current?.focus();
    else if (wasAsking.current) triggerRef.current?.focus();
    wasAsking.current = asking;
  }, [asking]);

  const trash = <Trash2 size={20} strokeWidth={1.75} aria-hidden="true" />;

  if (!asking) {
    return (
      <Button ref={triggerRef} variant="danger" fullWidth icon={trash} onClick={() => setAsking(true)}>
        {label}
      </Button>
    );
  }

  return (
    <div className={styles.confirm} role="group" aria-labelledby={questionId}>
      <p id={questionId} className={`t-body ${styles.question}`}>
        <CircleAlert size={20} strokeWidth={1.75} aria-hidden="true" />
        {question}
      </p>
      <div className={styles.buttons}>
        <Button ref={cancelRef} variant="secondary" onClick={() => setAsking(false)} disabled={loading}>
          Cancel
        </Button>
        <Button variant="danger" icon={trash} loading={loading} onClick={onConfirm}>
          {confirmLabel}
        </Button>
      </div>
    </div>
  );
}
