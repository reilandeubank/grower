import { ListChecks } from "lucide-react";
import { useCurrentUser } from "../auth/useCurrentUser";
import { AppShell, EmptyState } from "../design";
import { formatTodayHeading, greeting } from "../lib/dates";
import { useNow } from "../lib/useNow";
import styles from "./Today.module.css";

export function Today() {
  const me = useCurrentUser();
  const now = useNow();
  const tz = me.settings.timezone;

  return (
    <AppShell>
      <header className={styles.header}>
        <p className={`t-caption ${styles.greeting}`}>{greeting(now, tz)}</p>
        <h1 className="t-display">{formatTodayHeading(now, tz)}</h1>
      </header>
      <EmptyState
        icon={<ListChecks size={28} strokeWidth={1.75} />}
        title="No care tasks yet"
        body="Add a plant and its care schedule, and what needs doing today will show up here."
      />
    </AppShell>
  );
}
