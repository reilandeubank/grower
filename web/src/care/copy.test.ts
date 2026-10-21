import { describe, expect, it } from "vitest";
import { taskName } from "../design";
import { addDays, daysBetween } from "../lib/dates";
import {
  checkedToast,
  doneToast,
  taskWithPlant,
  doneVerb,
  dryingLine,
  dueLine,
  dueWhen,
  historyLine,
  intervalLine,
  lastReadingLine,
  readingLine,
  skipToast,
  snoozeToast,
  taskIntervalLine,
  verdict,
} from "./copy";

const today = "2026-10-14"; // a Wednesday

describe("dueLine", () => {
  it.each([
    ["2026-10-11", "Overdue 3 days"],
    ["2026-10-13", "Overdue 1 day"],
    ["2026-10-14", "Due today"],
    ["2026-10-15", "Tomorrow"],
    ["2026-10-17", "In 3 days"],
    ["2026-10-21", "In 7 days"],
    ["2026-10-23", "Fri, Oct 23"],
    ["2027-01-08", "Fri, Jan 8, 2027"],
  ])("%s reads %s", (due, line) => expect(dueLine(due, today)).toBe(line));

  it("counts across a month and a DST change without drifting", () => {
    expect(dueLine("2026-11-02", "2026-10-31")).toBe("In 2 days");
    expect(daysBetween("2026-03-07", "2026-03-09")).toBe(2);
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
  });

  it("has a mid-sentence form", () => {
    expect(dueWhen("2026-10-15", today)).toBe("tomorrow");
    expect(dueWhen("2026-10-19", today)).toBe("in 5 days");
    expect(dueWhen("2026-10-30", today)).toBe("on Fri, Oct 30");
  });
});

describe("task names and verbs", () => {
  it("names each kind, and custom tasks by their label", () => {
    expect(taskName("water")).toBe("Water");
    expect(taskName("rotate")).toBe("Rotate");
    expect(taskName("custom", " Prune ")).toBe("Prune");
    expect(taskName("custom", "")).toBe("Custom");
  });

  it("uses a past-tense verb per kind", () => {
    expect(doneVerb("water")).toBe("Watered");
    expect(doneVerb("fertilize")).toBe("Fertilized");
    expect(doneVerb("mist")).toBe("Misted");
    expect(doneVerb("repot")).toBe("Repotted");
    expect(doneVerb("rotate")).toBe("Rotated");
    expect(doneVerb("custom", "Prune")).toBe("Done: Prune");
  });

  it("writes the toasts", () => {
    expect(doneToast("water", "", "Monstera")).toBe("Watered Monstera.");
    expect(doneToast("custom", "Prune", "Monstera")).toBe("Done: Prune for Monstera.");
    expect(doneToast("check_in", "", "Monstera")).toBe("Checked in on Monstera.");
    expect(taskWithPlant({ kind: "check_in", label: "", moisture_check: false }, "Fern")).toBe("Check in on Fern");
    expect(taskWithPlant({ kind: "water", label: "", moisture_check: true }, "Fern")).toBe("Check soil for Fern");
    expect(taskWithPlant({ kind: "mist", label: "", moisture_check: false }, "Fern")).toBe("Mist Fern");
    expect(skipToast("fertilize", "", "Fern")).toBe("Skipped Fertilize for Fern.");
    expect(snoozeToast("2026-10-17", today)).toBe("Snoozed until Sat, Oct 17.");
  });

  it("reads intervals and history in words", () => {
    expect(intervalLine(1)).toBe("Every day");
    expect(intervalLine(7)).toBe("Every 7 days");
    expect(taskIntervalLine({ interval_days: 7, current_interval_days: 7 })).toBe("Every 7 days");
    expect(taskIntervalLine({ interval_days: 7, current_interval_days: 11 })).toBe("Every 7 days · 11 days this season");
    expect(taskIntervalLine({ interval_days: 1, current_interval_days: 1 })).toBe("Every day");
    expect(taskIntervalLine({ interval_days: 2, current_interval_days: 1 })).toBe("Every 2 days · 1 day this season");
    const event = { id: "e1", task_id: "t1", note: "", occurred_at: "", local_date: "2026-10-12", snooze_until: null, reading: null };
    expect(historyLine({ event: { ...event, kind: "done" }, task_kind: "water", task_label: "" }, today)).toBe(
      "Watered",
    );
    expect(historyLine({ event: { ...event, kind: "skipped" }, task_kind: "mist", task_label: "" }, today)).toBe(
      "Skipped Mist",
    );
    expect(
      historyLine(
        { event: { ...event, kind: "snoozed", snooze_until: "2026-10-17" }, task_kind: "water", task_label: "" },
        today,
      ),
    ).toBe("Snoozed Water until Oct 17");
  });
});

describe("moisture checks", () => {
  it.each([
    [1, 3, true, "Dry enough to water."],
    [3, 3, true, "Dry enough to water."],
    [4, 3, false, "Still moist — no water needed."],
    [7, 3, false, "Still moist — no water needed."],
    [8, 3, false, "Still wet — no water needed."],
    [10, 3, false, "Still wet — no water needed."],
    [3, 4, true, "Dry enough to water."],
    [4, 4, true, "Dry enough to water."],
    [7, 4, false, "Still moist — no water needed."],
    [8, 4, false, "Still wet — no water needed."],
  ])("reading %i with water at %i: water %s", (reading, waterAt, water, message) => {
    expect(verdict(reading, waterAt)).toEqual({ water, message });
  });

  it("names readings with their zone", () => {
    expect([1, 3, 4, 7, 8, 10].map(readingLine)).toEqual([
      "1 · Dry",
      "3 · Dry",
      "4 · Moist",
      "7 · Moist",
      "8 · Wet",
      "10 · Wet",
    ]);
    expect(checkedToast(6, "Monstera", "2026-10-23", today)).toBe(
      "Saved reading 6 · Moist for Monstera. Next check Fri, Oct 23.",
    );
    expect(lastReadingLine({ value: 6, on: "2026-10-12" }, today)).toBe("Last reading 6 · Moist, Oct 12");
    expect(lastReadingLine(null, today)).toBe("No readings yet");
  });

  it("says what it has learned about drying", () => {
    expect(dryingLine({ drying_days: 9, interval_days: 7 })).toBe("Usually dries out in 9 days");
    expect(dryingLine({ drying_days: 1, interval_days: 7 })).toBe("Usually dries out in 1 day");
    expect(dryingLine({ drying_days: null, interval_days: 7 })).toBe(
      "Check about every 7 days · learning from your readings",
    );
  });

  it("shows readings in history", () => {
    const event = { id: "e1", task_id: "t1", note: "", occurred_at: "", local_date: "2026-10-12", snooze_until: null };
    const entry = (kind: "checked" | "done", reading: number | null) => ({
      event: { ...event, kind, reading },
      task_kind: "water" as const,
      task_label: "",
    });
    expect(historyLine(entry("checked", 6), today)).toBe("Checked soil · 6 Moist");
    expect(historyLine(entry("done", 2), today)).toBe("Watered · read 2 Dry");
    expect(historyLine(entry("done", null), today)).toBe("Watered");
  });
});
