import { useId, useState, type ReactNode } from "react";
import { useNavigate } from "react-router";
import { ChevronDown, CircleCheck, Clock, ListChecks } from "lucide-react";
import type { EventKind, Today as TodayData, TodayItem } from "../api/client";
import { useCurrentUser } from "../auth/useCurrentUser";
import { dueLine, dueWhen } from "../care/copy";
import { useLogEvent, useToday } from "../care/queries";
import { TaskActionsSheet } from "../care/TaskActionsSheet";
import { AppShell, Button, EmptyState, TaskRow, taskName } from "../design";
import { formatCivil, formatTodayHeading, greeting } from "../lib/dates";
import { useNow } from "../lib/useNow";
import { LoadError } from "./LoadError";
import { Pending } from "./Pending";
import styles from "./Today.module.css";

export function Today() {
  const me = useCurrentUser();
  const now = useNow();
  const tz = me.settings.timezone;
  const today = useToday();
  // The server's date is the source of truth; the device's stands in until it loads.
  const date = today.data?.date;

  return (
    <AppShell>
      <header className={styles.header}>
        <p className={`t-caption ${styles.greeting}`}>{greeting(now, tz)}</p>
        <h1 className="t-display">
          {date ? formatCivil(date, { weekday: "long", month: "short", day: "numeric" }) : formatTodayHeading(now, tz)}
        </h1>
      </header>
      {today.isPending ? (
        <Pending inline />
      ) : today.isError ? (
        <LoadError onRetry={() => void today.refetch()} />
      ) : (
        <TodayLists {...today.data} />
      )}
    </AppShell>
  );
}

function TodayLists({ date, task_count, overdue, due, upcoming }: TodayData) {
  const [selected, setSelected] = useState<TodayItem | null>(null);
  // Kept here, not in ComingUp: a done briefly empties the list, and it should stay open after.
  const [comingUpOpen, setComingUpOpen] = useState(false);
  const log = useLogEvent();

  const act = (item: TodayItem, kind: EventKind, snoozeUntil?: string) =>
    log.mutate({ task: item.task, plantName: item.plant.name, kind, snoozeUntil, today: date });

  const rows = (items: TodayItem[], quiet = false) => (
    <ul className={styles.list}>
      {items.map((item) => (
        <TaskRow
          key={item.task.id}
          plantName={item.plant.name}
          photo={item.plant.cover?.urls.small}
          kind={item.task.kind}
          label={item.task.label}
          due={dueLine(item.task.next_due, date)}
          overdue={item.task.next_due < date}
          quiet={quiet}
          onOpen={() => setSelected(item)}
          onComplete={() => act(item, "done")}
        />
      ))}
    </ul>
  );

  const next = upcoming[0];
  return (
    <div className={styles.page}>
      {overdue.length > 0 && (
        <Section
          title={
            <span className={styles.overdueTitle}>
              <Clock size={20} strokeWidth={1.75} aria-hidden="true" />
              Overdue
            </span>
          }
          count={overdue.length}
          overdue
        >
          {rows(overdue)}
        </Section>
      )}
      {due.length > 0 && (
        <Section title="Due today" count={due.length}>
          {rows(due)}
        </Section>
      )}
      {overdue.length + due.length === 0 &&
        (next ? (
          <EmptyState
            icon={<CircleCheck size={28} strokeWidth={1.75} />}
            title="All caught up"
            body={`Next: ${taskName(next.task.kind, next.task.label)} ${next.plant.name} ${dueWhen(next.task.next_due, date)}.`}
          />
        ) : log.isPending ? null : (
          // While a save is out, an empty list only means the optimistic removal; wait for the answer.
          <NothingThisWeek hasTasks={task_count > 0} />
        ))}
      {upcoming.length > 0 && (
        <ComingUp count={upcoming.length} open={comingUpOpen} onToggle={() => setComingUpOpen((o) => !o)}>
          {rows(upcoming, true)}
        </ComingUp>
      )}
      <TaskActionsSheet item={selected} today={date} onClose={() => setSelected(null)} onAction={act} />
    </div>
  );
}

function Section({
  title,
  count,
  overdue = false,
  children,
}: {
  title: ReactNode;
  count: number;
  overdue?: boolean;
  children: ReactNode;
}) {
  return (
    <section className={styles.section}>
      <h2 className={`t-heading ${styles.sectionTitle}`}>
        {title}
        <span className={`t-caption ${styles.count} ${overdue ? styles.countOverdue : ""}`}>{count}</span>
      </h2>
      {children}
    </section>
  );
}

/** Collapsed by default: Today is about what's due now. */
function ComingUp({
  count,
  open,
  onToggle,
  children,
}: {
  count: number;
  open: boolean;
  onToggle: () => void;
  children: ReactNode;
}) {
  const panelId = useId();
  return (
    <section className={styles.section}>
      <h2 className="t-heading">
        <button
          type="button"
          className={styles.toggle}
          aria-expanded={open}
          aria-controls={panelId}
          onClick={onToggle}
        >
          Coming up
          <span className={`t-caption ${styles.count}`}>{count}</span>
          <ChevronDown className={styles.chevron} size={20} strokeWidth={1.75} aria-hidden="true" />
        </button>
      </h2>
      <div id={panelId} hidden={!open}>
        {children}
      </div>
    </section>
  );
}

/** Nothing due this week: either no care tasks exist yet, or everything is done. */
function NothingThisWeek({ hasTasks }: { hasTasks: boolean }) {
  const navigate = useNavigate();
  const goToPlants = (
    <Button variant="secondary" onClick={() => navigate("/plants")}>
      Go to plants
    </Button>
  );
  return !hasTasks ? (
    <EmptyState
      icon={<ListChecks size={28} strokeWidth={1.75} />}
      title="No care tasks yet"
      body="Add a plant and its care schedule, and what needs doing today will show up here."
      action={goToPlants}
    />
  ) : (
    <EmptyState
      icon={<CircleCheck size={28} strokeWidth={1.75} />}
      title="All caught up"
      body="Nothing is due in the next week. Care schedules live on each plant's page."
      action={goToPlants}
    />
  );
}
