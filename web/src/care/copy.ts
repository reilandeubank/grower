// Care copy, per the design README: relative dates when close, plain verbs, no guilt.
import type { HistoryEntry, TaskKind } from "../api/client";
import { taskName } from "../design";
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

export function lastDoneLine(lastDone: string | null, today: string): string {
  return lastDone ? `Last done ${formatDay(lastDone, today)}` : "Not done yet";
}

const VERBS: Record<Exclude<TaskKind, "custom">, string> = {
  water: "Watered",
  fertilize: "Fertilized",
  mist: "Misted",
  repot: "Repotted",
  rotate: "Rotated",
};

/** "Watered", "Fertilized", … or "Done: Prune" for a custom task. */
export function doneVerb(kind: TaskKind, label = ""): string {
  return kind === "custom" ? `Done: ${taskName(kind, label)}` : VERBS[kind];
}

export function doneToast(kind: TaskKind, label: string, plant: string): string {
  return kind === "custom" ? `${doneVerb(kind, label)} for ${plant}.` : `${doneVerb(kind)} ${plant}.`;
}

export function skipToast(kind: TaskKind, label: string, plant: string): string {
  return `Skipped ${taskName(kind, label)} for ${plant}.`;
}

export function snoozeToast(until: string, today: string): string {
  return `Snoozed until ${formatWeekdayDay(until, today)}.`;
}

/** A history row's words: "Watered", "Skipped Fertilize", "Snoozed Water until Oct 17". */
export function historyLine({ event, task_kind, task_label }: HistoryEntry, today: string): string {
  const name = taskName(task_kind, task_label);
  if (event.kind === "skipped") return `Skipped ${name}`;
  if (event.kind === "snoozed") {
    return event.snooze_until ? `Snoozed ${name} until ${formatDay(event.snooze_until, today)}` : `Snoozed ${name}`;
  }
  return doneVerb(task_kind, task_label);
}
