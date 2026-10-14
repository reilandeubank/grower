import { CloudOff } from "lucide-react";
import { Button, EmptyState } from "../design";
import styles from "./Unreachable.module.css";

/** Shown when /me fails for a reason other than being signed out (offline, server down). */
export function Unreachable({ onRetry }: { onRetry: () => void }) {
  return (
    <div className={styles.page}>
      <EmptyState
        as="h2"
        icon={<CloudOff size={28} strokeWidth={1.75} />}
        title="Can't reach grower"
        body="Check your connection, then try again."
        action={
          <Button variant="secondary" onClick={onRetry}>
            Try again
          </Button>
        }
      />
    </div>
  );
}
