import { useEffect, useId, useRef, type ReactNode } from "react";
import { X } from "lucide-react";
import { useCloseOnBack } from "../lib/useCloseOnBack";
import { IconButton } from "./IconButton";
import styles from "./BottomSheet.module.css";

export interface BottomSheetProps {
  open: boolean;
  /** Called on Esc, a backdrop tap or the close button. The parent owns `open`. */
  onClose: () => void;
  title: string;
  /** The body: usually a short form with its own buttons. */
  children: ReactNode;
}

/**
 * A short form or choice over a scrim, on a native modal <dialog> (focus trap and Esc for free).
 * Android back closes it; to leave the page from inside it, navigate with `replace: true`.
 */
export function BottomSheet({ open, onClose, title, children }: BottomSheetProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  const titleId = useId();
  useCloseOnBack(open, onClose);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!open || !dialog) return;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    dialog.showModal();
    // Land on the first field rather than the close button.
    bodyRef.current?.querySelector<HTMLElement>("input:not([hidden]), select, textarea, button")?.focus();
    return () => {
      dialog.close();
      opener?.focus();
    };
  }, [open]);

  return (
    <dialog
      ref={dialogRef}
      className={styles.sheet}
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      // A click whose target is the dialog itself landed on the backdrop.
      onClick={(e) => e.target === e.currentTarget && onClose()}
    >
      {open && (
        <div className={styles.inner}>
          <div className={styles.header}>
            <h2 id={titleId} className="t-heading">
              {title}
            </h2>
            <IconButton aria-label="Close" icon={<X size={20} strokeWidth={1.75} />} onClick={onClose} />
          </div>
          <div ref={bodyRef} className={styles.body}>
            {children}
          </div>
        </div>
      )}
    </dialog>
  );
}
