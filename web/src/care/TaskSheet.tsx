import { useState, type FormEvent } from "react";
import { problemMessage, type Anchor, type CareTask, type CareTaskInput, type TaskKind } from "../api/client";
import {
  BottomSheet,
  Button,
  ConfirmButton,
  ErrorText,
  FilterChips,
  MoistureScale,
  Switch,
  TaskIcon,
  TextField,
  taskName,
} from "../design";
import { intervalLine, readingLine, seasonalHint } from "./copy";
import { useDeleteTask, useSaveTask } from "./queries";
import styles from "./TaskSheet.module.css";

const KINDS: TaskKind[] = ["water", "fertilize", "mist", "repot", "rotate", "custom", "check_in"];
// A new task's starting interval; check-ins are a monthly look, the rest start weekly.
const defaultInterval = (kind: TaskKind) => (kind === "check_in" ? 30 : 7);
const KIND_OPTIONS = KINDS.map((k) => ({ value: k, label: taskName(k), icon: <TaskIcon kind={k} size={16} /> }));
const INTERVAL_PRESETS = [3, 7, 14, 30].map((n) => ({ value: String(n), label: `${n} days` }));
const ANCHOR_OPTIONS = [
  { value: "from_completion", label: "Count from when it's done" },
  { value: "fixed", label: "Keep a fixed schedule" },
];
const WATER_MODES = [
  { value: "schedule", label: "On a schedule" },
  { value: "check", label: "Check the soil first" },
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
  // On by default for the kinds the seasons change, as on the server.
  const [seasonal, setSeasonal] = useState(task?.seasonal ?? seasonalHint(kind) !== undefined);
  // A soil check (docs/architecture.md) is a water task only; it keeps its own threshold.
  const [checkSoil, setCheckSoil] = useState(task?.moisture_check ?? false);
  const [waterAt, setWaterAt] = useState(task?.water_at ?? 3);
  const [errors, setErrors] = useState<{ label?: string; interval?: string; startOn?: string }>({});
  const save = useSaveTask(plantId);
  const remove = useDeleteTask(plantId);
  const days = parseInterval(intervalText);
  const check = kind === "water" && checkSoil;

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
      // Checks always count from the last watering.
      anchor: check ? "from_completion" : anchor,
      start_on: startOn,
      seasonal,
      moisture_check: check,
      water_at: waterAt,
    } satisfies CareTaskInput;
    save.mutate({ id: task?.id, body }, { onSuccess: onDone });
  };

  const failure = save.error ?? remove.error;
  const n = days ?? 7;
  const hint = seasonalHint(kind);

  return (
    <form className={styles.form} onSubmit={onSubmit} noValidate>
      <div className={styles.group}>
        <p className={`t-caption ${styles.label}`} aria-hidden="true">
          Kind
        </p>
        <FilterChips
          label="Kind"
          options={KIND_OPTIONS}
          value={kind}
          onChange={(v) => {
            const next = v as TaskKind;
            setKind(next);
            if (task) return;
            setSeasonal(seasonalHint(next) !== undefined);
            // Swap the default along with the kind, but keep a number the user typed.
            if (intervalText === String(defaultInterval(kind))) setIntervalText(String(defaultInterval(next)));
          }}
          wrap
        />
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
      {kind === "water" && (
        <div className={styles.group}>
          <p className={`t-caption ${styles.label}`} aria-hidden="true">
            How do you decide when to water?
          </p>
          <FilterChips
            label="How do you decide when to water?"
            options={WATER_MODES}
            value={checkSoil ? "check" : "schedule"}
            onChange={(v) => setCheckSoil(v === "check")}
            wrap
          />
        </div>
      )}
      <div className={styles.group}>
        <TextField
          label={check ? "Check about every" : "Days between"}
          suffix={check ? "days" : undefined}
          type="number"
          inputMode="numeric"
          min={1}
          max={365}
          value={intervalText}
          onChange={(e) => setIntervalText(e.target.value)}
          hint={
            check
              ? "Your first guess; grower learns from your readings."
              : days
                ? intervalLine(days)
                : "From 1 to 365 days."
          }
          error={errors.interval}
        />
        <FilterChips
          label="Common intervals"
          options={INTERVAL_PRESETS}
          value={intervalText}
          onChange={setIntervalText}
          wrap
        />
      </div>
      {check && (
        <div className={styles.group}>
          <p className={`t-caption ${styles.label} ${styles.split}`} aria-hidden="true">
            Water at or below
            <span className={styles.hint}>{readingLine(waterAt)}</span>
          </p>
          <MoistureScale label="Water at or below" value={waterAt} onChange={setWaterAt} max={9} />
        </div>
      )}
      <TextField
        label={check ? "First check" : "First due"}
        type="date"
        value={startOn}
        onChange={(e) => setStartOn(e.target.value)}
        error={errors.startOn}
      />
      {!check && (
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
      )}
      {hint && !check && (
        <Switch
          label="Adjust for seasons"
          description={anchor === "fixed" ? "Only for schedules counted from when it's done." : hint}
          checked={seasonal}
          disabled={anchor === "fixed"}
          onChange={setSeasonal}
        />
      )}
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
