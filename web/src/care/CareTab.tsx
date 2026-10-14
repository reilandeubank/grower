import { useState } from "react";
import { CalendarClock, Clock, Plus } from "lucide-react";
import type { CareTask, Plant } from "../api/client";
import { Button, DoneCheck, EmptyState, TaskIcon, taskName } from "../design";
import { deviceToday, formatDay } from "../lib/dates";
import { LoadError } from "../routes/LoadError";
import { Pending } from "../routes/Pending";
import { dueLine, historyLine, intervalLine, lastDoneLine } from "./copy";
import { useLogEvent, usePlantHistory, usePlantTasks, useToday } from "./queries";
import { TaskSheet } from "./TaskSheet";
import styles from "./CareTab.module.css";

/** A plant's care tasks (tap to edit, check to complete) and its care history. */
export function CareTab({ plant }: { plant: Plant }) {
  const tasks = usePlantTasks(plant.id);
  const history = usePlantHistory(plant.id);
  // The server's date when Today has loaded it; the device's otherwise.
  const today = useToday().data?.date ?? deviceToday();
  const log = useLogEvent();
  const [sheet, setSheet] = useState<{ task?: CareTask } | null>(null);

  if (tasks.isPending) return <Pending inline />;
  if (tasks.isError) return <LoadError onRetry={() => void tasks.refetch()} />;

  const add = (
    <Button variant="secondary" icon={<Plus size={20} strokeWidth={1.75} aria-hidden="true" />} onClick={() => setSheet({})}>
      Add care task
    </Button>
  );

  return (
    <div className={styles.tab}>
      {tasks.data.length === 0 ? (
        <EmptyState
          as="h3"
          icon={<CalendarClock size={28} strokeWidth={1.75} />}
          title="No care schedule yet"
          body="Add a care task, like watering every 7 days, and it shows up on Today when it's due."
          action={add}
        />
      ) : (
        <>
          <ul className={styles.tasks}>
            {tasks.data.map((t) => {
              const name = taskName(t.kind, t.label);
              const overdue = t.next_due < today;
              const completing = log.isPending && log.variables.task.id === t.id;
              return (
                <li key={t.id} className={styles.task}>
                  <button type="button" className={styles.taskBody} onClick={() => setSheet({ task: t })} aria-haspopup="dialog">
                    <TaskIcon kind={t.kind} />
                    <span className={styles.text}>
                      <span className="t-body-strong">{name}</span>
                      <span className={`t-caption ${styles.muted}`}>
                        {intervalLine(t.interval_days)} ·{" "}
                        <span className={overdue ? styles.overdue : undefined}>
                          {overdue && <Clock size={16} strokeWidth={1.75} aria-hidden="true" />}
                          {dueLine(t.next_due, today)}
                        </span>
                      </span>
                      <span className={`t-caption ${styles.muted}`}>{lastDoneLine(t.last_done, today)}</span>
                    </span>
                  </button>
                  <DoneCheck
                    aria-label={`Mark ${name} ${plant.name} done`}
                    checked={completing}
                    onClick={() => log.mutate({ task: t, plantName: plant.name, kind: "done", today })}
                  />
                </li>
              );
            })}
          </ul>
          <div className={styles.add}>{add}</div>
        </>
      )}

      <section className={styles.history}>
        <h3 className="t-heading">History</h3>
        {history.isPending ? (
          <Pending inline />
        ) : history.isError ? (
          <LoadError onRetry={() => void history.refetch()} />
        ) : history.data.length === 0 ? (
          <p className={`t-body ${styles.muted}`}>No care logged yet.</p>
        ) : (
          <ul className={styles.events}>
            {history.data.map((entry) => (
              <li key={entry.event.id} className={`t-body ${styles.event}`}>
                <TaskIcon kind={entry.task_kind} />
                <span>
                  {historyLine(entry, today)}
                  <span className={styles.muted}> · {formatDay(entry.event.local_date, today)}</span>
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <TaskSheet
        open={sheet !== null}
        plantId={plant.id}
        task={sheet?.task}
        today={today}
        onClose={() => setSheet(null)}
      />
    </div>
  );
}
