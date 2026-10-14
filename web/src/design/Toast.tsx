import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import styles from "./Toast.module.css";

const DISMISS_MS = 5000;
let seq = 0;

export interface ToastMessage {
  message: string;
  /** e.g. Undo. Pressing it also dismisses the toast. */
  action?: { label: string; onClick: () => void };
}

type ShowToast = (toast: ToastMessage) => void;

const ToastContext = createContext<ShowToast | null>(null);

/** Shows one toast at a time, above the tab bar; a new toast replaces the current one. */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [current, setCurrent] = useState<(ToastMessage & { id: number }) | null>(null);

  const show = useCallback<ShowToast>((toast) => setCurrent({ ...toast, id: ++seq }), []);

  useEffect(() => {
    if (!current) return;
    const id = window.setTimeout(() => setCurrent(null), DISMISS_MS);
    return () => window.clearTimeout(id);
  }, [current]);

  return (
    <ToastContext.Provider value={show}>
      {children}
      {/* The live region stays mounted so screen readers announce each new message. */}
      <div className={styles.region} role="status" aria-live="polite">
        {current && (
          <div key={current.id} className={styles.toast}>
            <p className={`t-body ${styles.message}`}>{current.message}</p>
            {current.action && (
              <button
                type="button"
                className={`t-body-strong ${styles.action}`}
                onClick={() => {
                  setCurrent(null);
                  current.action?.onClick();
                }}
              >
                {current.action.label}
              </button>
            )}
          </div>
        )}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ShowToast {
  const show = useContext(ToastContext);
  if (!show) throw new Error("useToast used outside ToastProvider");
  return show;
}
