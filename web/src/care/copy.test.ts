import { describe, expect, it } from "vitest";
import { taskName } from "../design";
import { addDays, daysBetween } from "../lib/dates";
import { doneToast, doneVerb, dueLine, dueWhen, historyLine, intervalLine, skipToast, snoozeToast } from "./copy";

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
    expect(skipToast("fertilize", "", "Fern")).toBe("Skipped Fertilize for Fern.");
    expect(snoozeToast("2026-10-17", today)).toBe("Snoozed until Sat, Oct 17.");
  });

  it("reads intervals and history in words", () => {
    expect(intervalLine(1)).toBe("Every day");
    expect(intervalLine(7)).toBe("Every 7 days");
    const event = { id: "e1", task_id: "t1", note: "", occurred_at: "", local_date: "2026-10-12", snooze_until: null };
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
