import { useState, type FormEvent } from "react";
import { problemMessage, type Anchor, type CareTask, type TaskKind } from "../api/client";
import { BottomSheet, Button, ConfirmButton, ErrorText, FilterChips, TaskIcon, TextField, taskName } from "../design";
import { intervalLine } from "./copy";
import { useDeleteTask, useSaveTask } from "./queries";
import styles from "./TaskSheet.module.css";

const KINDS: TaskKind[] = ["water", "fertilize", "mist", "repot", "rotate", "custom"];
const KIND_OPTIONS = KINDS.map((k) => ({ value: k, label: taskName(k), icon: <TaskIcon kind={k} size={16} /> }));
const INTERVAL_PRESETS = [3, 7, 14, 30].map((n) => ({ value: String(n), label: `${n} days` }));
const ANCHOR_OPTIONS = [
  { value: "from_completion", label: "Count from when it's done" },
  { value: "fixed", label: "Keep a fixed schedule" },
];

export interface TaskSheetProps {
  open: boolean;
  plantId: string;
  /** Edit this task; without it the sheet adds a new one. */
  task?: CareTask | undefined;
  /** Today's date, the default first due date. */
  today: string;
  onClose: () => void;
}

export function TaskSheet({ open, plantId, task, today, onClose }: TaskSheetProps) {
  return (
    <BottomSheet open={open} onClose={onClose} title={task ? "Edit care task" : "Add care task"}>
      <TaskForm plantId={plantId} task={task} today={today} onDone={onClose} />
    </BottomSheet>
  );
}

/** Whole days from 1 to 365, or undefined. */
function parseInterval(value: string): number | undefined {
  const n = Number(value);
  return /^\d+$/.test(value.trim()) && n >= 1 && n <= 365 ? n : undefined;
}

// Mounted only while the sheet is open, so each opening starts fresh.
function TaskForm({ plantId, task, today, onDone }: Omit<TaskSheetProps, "open" | "onClose"> & { onDone: () => void }) {
  const [kind, setKind] = useState<TaskKind>(task?.kind ?? "water");
  const [label, setLabel] = useState(task?.label ?? "");
  const [intervalText, setIntervalText] = useState(String(task?.interval_days ?? 7));
  const [startOn, setStartOn] = useState(task?.start_on ?? today);
  const [anchor, setAnchor] = useState<Anchor>(task?.anchor ?? "from_completion");
  const [errors, setErrors] = useState<{ label?: string; interval?: string; startOn?: string }>({});
  const save = useSaveTask(plantId);
  const remove = useDeleteTask(plantId);
  const days = parseInterval(intervalText);

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const found = {
      label: kind === "custom" && !label.trim() ? "Give the task a name." : undefined,
      interval: days === undefined ? "Enter a number of days from 1 to 365." : undefined,
      startOn: startOn ? undefined : "Pick the first due date.",
    };
    setErrors(found);
    if (found.label || found.interval || found.startOn || days === undefined) return;
    const body = {
      kind,
      label: kind === "custom" ? label.trim() : "",
      interval_days: days,
      anchor,
      start_on: startOn,
    };
    save.mutate({ id: task?.id, body }, { onSuccess: onDone });
  };

  const failure = save.error ?? remove.error;
  const n = days ?? 7;

  return (
    <form className={styles.form} onSubmit={onSubmit} noValidate>
      <div className={styles.group}>
        <p className={`t-caption ${styles.label}`} aria-hidden="true">
          Kind
        </p>
        <FilterChips label="Kind" options={KIND_OPTIONS} value={kind} onChange={(v) => setKind(v as TaskKind)} wrap />
      </div>
      {kind === "custom" && (
        <TextField
          label="Name"
          value={label}
          maxLength={40}
          autoComplete="off"
          placeholder="e.g. Prune"
          onChange={(e) => setLabel(e.target.value)}
          error={errors.label}
        />
      )}
      <div className={styles.group}>
        <TextField
          label="Days between"
          type="number"
          inputMode="numeric"
          min={1}
          max={365}
          value={intervalText}
          onChange={(e) => setIntervalText(e.target.value)}
          hint={days ? intervalLine(days) : "From 1 to 365 days."}
          error={errors.interval}
        />
        <FilterChips label="Common intervals" options={INTERVAL_PRESETS} value={intervalText} onChange={setIntervalText} wrap />
      </div>
      <TextField
        label="First due"
        type="date"
        value={startOn}
        onChange={(e) => setStartOn(e.target.value)}
        error={errors.startOn}
      />
      <div className={styles.group}>
        <p className={`t-caption ${styles.label}`} aria-hidden="true">
          Schedule
        </p>
        <FilterChips
          label="Schedule"
          options={ANCHOR_OPTIONS}
          value={anchor}
          onChange={(v) => setAnchor(v as Anchor)}
          wrap
        />
        <p className={`t-caption ${styles.hint}`}>
          {anchor === "fixed"
            ? `Due on the same days, every ${n === 1 ? "day" : `${n} days`} from the first, even if it's done late.`
            : `The next one is due ${n === 1 ? "a day" : `${n} days`} after you mark it done.`}
        </p>
      </div>
      {failure && <ErrorText>{problemMessage(failure)}</ErrorText>}
      <div className={styles.actions}>
        <Button type="submit" fullWidth loading={save.isPending}>
          Save
        </Button>
        {task && (
          <ConfirmButton
            label="Delete task"
            question="Delete this task and its history?"
            loading={remove.isPending}
            onConfirm={() => remove.mutate(task.id, { onSuccess: onDone })}
          />
        )}
      </div>
    </form>
  );
}
