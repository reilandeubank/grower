import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";

const plant = {
  id: "p1",
  name: "Monstera",
  species: "",
  location_id: null,
  acquired_on: null,
  notes: "",
  archived: false,
  created_at: "2026-10-01T12:00:00Z",
};
const task = {
  id: "t1",
  plant_id: "p1",
  kind: "water",
  label: "",
  interval_days: 7,
  anchor: "from_completion",
  start_on: "2026-10-14",
  next_due: "2026-10-14",
  last_done: null,
};
const dueToday = { date: "2026-10-14", overdue: [], due: [{ task, plant, days_overdue: 0 }], upcoming: [] };
const allDone = {
  ...dueToday,
  due: [],
  upcoming: [{ task: { ...task, next_due: "2026-10-21" }, plant, days_overdue: -7 }],
};
const event = {
  id: "e1",
  task_id: "t1",
  kind: "done",
  local_date: "2026-10-14",
  snooze_until: null,
  note: "",
  occurred_at: "2026-10-14T15:00:00Z",
};

describe("Today", () => {
  it("removes a task at once on done, then offers Undo", async () => {
    let today: unknown = dueToday;
    let finishPost: (r: Response) => void = () => {};
    let posted: unknown;
    let undone = false;
    mockApi({
      "GET /api/v1/me": () => json(200, me),
      "GET /api/v1/today": () => json(200, today),
      "POST /api/v1/tasks/t1/events": async (req) => {
        posted = await req.json();
        return new Promise<Response>((resolve) => (finishPost = resolve));
      },
      "DELETE /api/v1/events/e1": () => {
        undone = true;
        today = dueToday;
        return json(200, task);
      },
    });
    renderApp("/");
    expect(await screen.findByRole("heading", { level: 1, name: "Wednesday, Oct 14" })).toBeTruthy();
    const section = (await screen.findByRole("heading", { name: /Due today/ })).closest("section")!;
    await userEvent.click(within(section).getByRole("button", { name: "Mark Water Monstera done" }));

    // Gone before the server answers.
    await waitFor(() => expect(screen.queryByRole("button", { name: "Mark Water Monstera done" })).toBeNull());
    expect(posted).toMatchObject({ kind: "done", local_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/) });

    today = allDone;
    finishPost(json(201, { event, task: { ...task, next_due: "2026-10-21", last_done: "2026-10-14" } }));
    expect(await screen.findByText("Watered Monstera.")).toBeTruthy();
    expect(await screen.findByText("All caught up")).toBeTruthy();
    expect(screen.getByText("Next: Water Monstera in 7 days.")).toBeTruthy();

    await userEvent.click(screen.getByRole("button", { name: "Undo" }));
    await waitFor(() => expect(undone).toBe(true));
    expect(await screen.findByRole("button", { name: "Mark Water Monstera done" })).toBeTruthy();
    expect(screen.queryByText("Watered Monstera.")).toBeNull();
  });

  it("puts the task back if the save fails", async () => {
    mockApi({
      "GET /api/v1/me": () => json(200, me),
      "GET /api/v1/today": () => json(200, dueToday),
      "POST /api/v1/tasks/t1/events": () => json(500, { title: "Internal Server Error", status: 500 }),
    });
    renderApp("/");
    await userEvent.click(await screen.findByRole("button", { name: "Mark Water Monstera done" }));
    expect(await screen.findByText(/Couldn't save that/)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Mark Water Monstera done" })).toBeTruthy();
  });

  it("snoozes from the task's sheet", async () => {
    let posted: unknown;
    mockApi({
      "GET /api/v1/me": () => json(200, me),
      "GET /api/v1/today": () => json(200, dueToday),
      "POST /api/v1/tasks/t1/events": async (req) => {
        posted = await req.json();
        return json(201, { event: { ...event, kind: "snoozed", snooze_until: "2026-10-17" }, task });
      },
    });
    renderApp("/");
    await userEvent.click(await screen.findByRole("button", { name: /^Monstera/ }));
    const sheet = screen.getByRole("dialog", { name: "Water · Monstera" });
    await userEvent.click(within(sheet).getByRole("button", { name: "Snooze" }));
    await userEvent.click(within(sheet).getByRole("button", { name: /In 3 days/ }));
    expect(await screen.findByText("Snoozed until Sat, Oct 17.")).toBeTruthy();
    expect(posted).toMatchObject({ kind: "snoozed", snooze_until: "2026-10-17" });
  });

  it("shows the first-run state with no plants", async () => {
    mockApi({
      "GET /api/v1/me": () => json(200, me),
      "GET /api/v1/today": () => json(200, { date: "2026-10-14", overdue: [], due: [], upcoming: [] }),
      "GET /api/v1/plants": () => json(200, { items: [] }),
    });
    renderApp("/");
    expect(await screen.findByText("No care tasks yet")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Go to plants" })).toBeTruthy();
  });
});
