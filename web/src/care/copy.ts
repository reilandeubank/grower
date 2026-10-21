// Care copy, per the design README: relative dates when close, plain verbs, no guilt.
import type { CareTask, HistoryEntry, TaskKind } from "../api/client";
import { moistureZone, taskName, zoneWord } from "../design";
import { daysBetween, formatDay, formatWeekdayDay } from "../lib/dates";

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? "" : "s"}`;

/** "Overdue 3 days", "Due today", "Tomorrow", "In 3 days", then "Fri, Oct 24". */
export function dueLine(due: string, today: string): string {
  const days = daysBetween(today, due);
  if (days < 0) return `Overdue ${plural(-days, "day")}`;
  if (days === 0) return "Due today";
  if (days === 1) return "Tomorrow";
  if (days <= 7) return `In ${days} days`;
  return formatWeekdayDay(due, today);
}

/** For mid-sentence use, as in "Next: Water Fern tomorrow". Only for dates from today on. */
export function dueWhen(due: string, today: string): string {
  const days = daysBetween(today, due);
  if (days <= 0) return "today";
  if (days === 1) return "tomorrow";
  if (days <= 7) return `in ${days} days`;
  return `on ${formatWeekdayDay(due, today)}`;
}

export function intervalLine(days: number): string {
  return days === 1 ? "Every day" : `Every ${days} days`;
}

/** "Every 7 days", or "Every 7 days · 10 days this season" while seasons adjust it. */
export function taskIntervalLine(task: Pick<CareTask, "interval_days" | "current_interval_days">): string {
  const line = intervalLine(task.interval_days);
  const now = task.current_interval_days;
  return now === task.interval_days ? line : `${line} · ${plural(now, "day")} this season`;
}

const SEASONAL_HINTS: Partial<Record<TaskKind, string>> = {
  water: "Waters less often in winter and a little more in summer.",
  mist: "Mists less often in winter and a little more in summer.",
  fertilize: "Feeds less often from fall to early spring.",
};

/** What seasonal adjustment does for a kind, or undefined for kinds it doesn't change. */
export function seasonalHint(kind: TaskKind): string | undefined {
  return SEASONAL_HINTS[kind];
}

export function lastDoneLine(lastDone: string | null, today: string): string {
  return lastDone ? `Last done ${formatDay(lastDone, today)}` : "Not done yet";
}

const VERBS: Record<Exclude<TaskKind, "custom">, string> = {
  water: "Watered",
  fertilize: "Fertilized",
  mist: "Misted",
  repot: "Repotted",
  rotate: "Rotated",
  check_in: "Checked in",
};

/** "Watered", "Fertilized", … or "Done: Prune" for a custom task. */
export function doneVerb(kind: TaskKind, label = ""): string {
  return kind === "custom" ? `Done: ${taskName(kind, label)}` : VERBS[kind];
}

export function doneToast(kind: TaskKind, label: string, plant: string): string {
  if (kind === "custom") return `${doneVerb(kind, label)} for ${plant}.`;
  return kind === "check_in" ? `Checked in on ${plant}.` : `${doneVerb(kind)} ${plant}.`;
}

export function skipToast(kind: TaskKind, label: string, plant: string): string {
  return `Skipped ${taskName(kind, label)} for ${plant}.`;
}

export function snoozeToast(until: string, today: string): string {
  return `Snoozed until ${formatWeekdayDay(until, today)}.`;
}

/**
 * A history row's words: "Watered", "Skipped Fertilize", "Snoozed Water until Oct 17",
 * "Checked soil · 6 Moist", "Watered · read 2 Dry".
 */
export function historyLine({ event, task_kind, task_label }: HistoryEntry, today: string): string {
  const read = event.reading === null ? "" : `${event.reading} ${zoneWord(event.reading)}`;
  if (event.kind === "checked") return read ? `Checked soil · ${read}` : "Checked soil";
  const name = taskName(task_kind, task_label);
  if (event.kind === "skipped") return `Skipped ${name}`;
  if (event.kind === "snoozed") {
    return event.snooze_until ? `Snoozed ${name} until ${formatDay(event.snooze_until, today)}` : `Snoozed ${name}`;
  }
  return read ? `${doneVerb(task_kind, task_label)} · read ${read}` : doneVerb(task_kind, task_label);
}

// Moisture checks (PLAN §5.5).

/** "6 · Moist". */
export function readingLine(reading: number): string {
  return `${reading} · ${zoneWord(reading)}`;
}

/** Whether a reading says to water now, and the words for it. Mirrors the server: water at or below the threshold. */
export function verdict(reading: number, waterAt: number): { water: boolean; message: string } {
  if (reading <= waterAt) return { water: true, message: "Dry enough to water." };
  return { water: false, message: `Still ${moistureZone(reading) === "wet" ? "wet" : "moist"} — no water needed.` };
}

/** "Saved reading 6 · Moist for Monstera. Next check Fri, Oct 24." */
export function checkedToast(reading: number, plant: string, nextDue: string, today: string): string {
  return `Saved reading ${readingLine(reading)} for ${plant}. Next check ${formatWeekdayDay(nextDue, today)}.`;
}

/** "Usually dries out in 9 days" once learned, else the first guess. */
export function dryingLine({ drying_days, interval_days }: Pick<CareTask, "drying_days" | "interval_days">): string {
  if (drying_days !== null) return `Usually dries out in ${plural(drying_days, "day")}`;
  const every = interval_days === 1 ? "day" : `${interval_days} days`;
  return `Check about every ${every} · learning from your readings`;
}

/** "Last reading 6 · Moist, Oct 18". */
export function lastReadingLine(last: CareTask["last_reading"], today: string): string {
  return last ? `Last reading ${readingLine(last.value)}, ${formatDay(last.on, today)}` : "No readings yet";
}

/** "Water Fern", "Check soil for Fern", "Check in on Fern": a task named with its plant. */
export function taskWithPlant(task: Pick<CareTask, "kind" | "label" | "moisture_check">, plant: string): string {
  if (task.moisture_check) return `Check soil for ${plant}`;
  if (task.kind === "check_in") return `Check in on ${plant}`;
  return `${taskName(task.kind, task.label)} ${plant}`;
}
