import { useId, useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { AlarmClockOff, ArrowLeft, Check, Clock, SkipForward, Sprout } from "lucide-react";
import type { EventKind, TodayItem } from "../api/client";
import { BottomSheet, Button, TaskIcon, TextField, taskName } from "../design";
import { addDays, daysBetween, formatWeekdayDay } from "../lib/dates";
import { dueLine } from "./copy";
import styles from "./TaskActionsSheet.module.css";

export interface TaskActionsSheetProps {
  /** The task to act on; the sheet is open while it's set. */
  item: TodayItem | null;
  /** Today's date from the server. */
  today: string;
  onClose: () => void;
  onAction: (item: TodayItem, kind: EventKind, snoozeUntil?: string) => void;
}

const icon = (Icon: typeof Check) => <Icon size={20} strokeWidth={1.75} aria-hidden="true" />;

/** Done, snooze, skip or open the plant, for one task row. */
export function TaskActionsSheet({ item, today, onClose, onAction }: TaskActionsSheetProps) {
  const title = item ? `${taskName(item.task.kind, item.task.label)} · ${item.plant.name}` : "";
  return (
    <BottomSheet open={item !== null} onClose={onClose} title={title}>
      {item && <Actions key={item.task.id} item={item} today={today} onClose={onClose} onAction={onAction} />}
    </BottomSheet>
  );
}

function Actions({ item, today, onClose, onAction }: TaskActionsSheetProps & { item: TodayItem }) {
  const [snoozing, setSnoozing] = useState(false);
  const navigate = useNavigate();
  const act = (kind: EventKind, until?: string) => {
    onClose();
    onAction(item, kind, until);
  };

  if (snoozing) {
    return (
      <SnoozeOptions item={item} today={today} onBack={() => setSnoozing(false)} onSnooze={(d) => act("snoozed", d)} />
    );
  }

  const overdue = item.task.next_due < today;
  return (
    <>
      <p className={`t-caption ${styles.due} ${overdue ? styles.overdue : ""}`}>
        {overdue ? <Clock size={16} strokeWidth={1.75} aria-hidden="true" /> : <TaskIcon kind={item.task.kind} size={16} />}
        {dueLine(item.task.next_due, today)}
      </p>
      <div className={styles.buttons}>
        <Button fullWidth icon={icon(Check)} onClick={() => act("done")}>
          Done
        </Button>
        <Button variant="secondary" fullWidth icon={icon(AlarmClockOff)} onClick={() => setSnoozing(true)}>
          Snooze
        </Button>
        <div className={styles.withNote}>
          <Button variant="secondary" fullWidth icon={icon(SkipForward)} onClick={() => act("skipped")}>
            Skip this time
          </Button>
          <p className={`t-caption ${styles.note}`}>Skipping counts as done for the schedule.</p>
        </div>
        <Button
          variant="ghost"
          fullWidth
          icon={icon(Sprout)}
          onClick={() => {
            onClose();
            navigate(`/plants/${item.plant.id}`);
          }}
        >
          Open plant
        </Button>
      </div>
    </>
  );
}

function SnoozeOptions({
  item,
  today,
  onBack,
  onSnooze,
}: {
  item: TodayItem;
  today: string;
  onBack: () => void;
  onSnooze: (until: string) => void;
}) {
  // Snoozing to a day on or before the due date changes nothing, so start after both.
  const earliest = addDays(item.task.next_due > today ? item.task.next_due : today, 1);
  const presets = [
    { label: "Tomorrow", days: 1 },
    { label: "In 3 days", days: 3 },
    { label: "In a week", days: 7 },
  ]
    .map((p) => ({ ...p, date: addDays(today, p.days) }))
    .filter((p) => p.date >= earliest);
  const [picked, setPicked] = useState(earliest);
  const [error, setError] = useState<string>();
  const headingId = useId();

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!picked || daysBetween(earliest, picked) < 0) {
      setError(`Pick a date from ${formatWeekdayDay(earliest, today)} on.`);
      return;
    }
    onSnooze(picked);
  };

  return (
    <>
      <p id={headingId} className={`t-caption ${styles.heading}`}>
        Snooze until
      </p>
      <div className={styles.buttons} role="group" aria-labelledby={headingId}>
        {presets.map((p, i) => (
          <Button
            key={p.days}
            variant="secondary"
            fullWidth
            className={styles.preset}
            // The view swapped under the focused Snooze button; land on the first choice.
            autoFocus={i === 0}
            onClick={() => onSnooze(p.date)}
          >
            {p.label}
            <span className={`t-caption ${styles.presetDate}`}>{formatWeekdayDay(p.date, today)}</span>
          </Button>
        ))}
      </div>
      <form className={styles.pick} onSubmit={onSubmit} noValidate>
        <TextField
          label="Pick a date"
          type="date"
          min={earliest}
          value={picked}
          onChange={(e) => {
            setPicked(e.target.value);
            setError(undefined);
          }}
          error={error}
        />
        <Button type="submit" variant="secondary" icon={icon(AlarmClockOff)}>
          Snooze
        </Button>
      </form>
      <Button variant="ghost" icon={icon(ArrowLeft)} onClick={onBack}>
        Back
      </Button>
    </>
  );
}
