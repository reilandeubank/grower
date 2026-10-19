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
  pot_type: null,
  pot_size_in: null,
  drainage: null,
  soil: "",
  cover: null,
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
  seasonal: true,
  moisture_check: true,
  water_at: 3,
  last_reading: null,
  drying_days: null,
  current_interval_days: 7,
};
const today = { date: "2026-10-14", task_count: 1, overdue: [], due: [{ task, plant, days_overdue: 0 }], upcoming: [] };
const event = {
  id: "e1",
  task_id: "t1",
  kind: "checked",
  local_date: "2026-10-14",
  snooze_until: null,
  reading: 6,
  note: "",
  occurred_at: "2026-10-14T15:00:00Z",
};

function setup(saved: { event: object; task: object }) {
  const posts: unknown[] = [];
  let current: object = today;
  mockApi({
    "GET /api/v1/me": () => json(200, me),
    "GET /api/v1/today": () => json(200, current),
    "POST /api/v1/tasks/t1/events": async (req) => {
      posts.push(await req.json());
      current = { ...today, due: [], upcoming: [{ task: saved.task, plant, days_overdue: -1 }] };
      return json(201, saved);
    },
  });
  renderApp("/");
  return posts;
}

async function openCheck() {
  await userEvent.click(await screen.findByRole("button", { name: "Check soil for Monstera" }));
  return screen.getByRole("dialog", { name: "Check soil · Monstera" });
}

describe("Check soil", () => {
  it("shows as a soil check on Today", async () => {
    setup({ event, task });
    const row = await screen.findByRole("button", { name: /^Monstera/ });
    expect(within(row).getByText("Check soil")).toBeTruthy();
    expect(within(row).getByText("Due today")).toBeTruthy();
  });

  it("waters on a dry reading, sending the reading with done", async () => {
    const posts = setup({ event: { ...event, kind: "done", reading: 2 }, task: { ...task, next_due: "2026-10-21" } });
    const sheet = await openCheck();
    expect(within(sheet).getByRole("button", { name: "Save reading" })).toHaveProperty("disabled", true);
    await userEvent.click(within(sheet).getByRole("radio", { name: "2, dry" }));
    expect(within(sheet).getByText("Dry enough to water.")).toBeTruthy();
    await userEvent.click(within(sheet).getByRole("button", { name: "Watered" }));

    expect(await screen.findByText("Watered Monstera.")).toBeTruthy();
    expect(posts).toEqual([expect.objectContaining({ kind: "done", reading: 2 })]);
  });

  it("can hold off on a dry reading", async () => {
    const posts = setup({ event: { ...event, reading: 3 }, task: { ...task, next_due: "2026-10-15" } });
    const sheet = await openCheck();
    await userEvent.click(within(sheet).getByRole("radio", { name: "3, dry" }));
    await userEvent.click(within(sheet).getByRole("button", { name: "Not now" }));
    expect(await screen.findByText("Saved reading 3 · Dry for Monstera. Next check Thu, Oct 15.")).toBeTruthy();
    expect(posts).toEqual([expect.objectContaining({ kind: "checked", reading: 3 })]);
  });

  it("saves a moist reading and names the next check from the server", async () => {
    const posts = setup({ event, task: { ...task, next_due: "2026-10-18", last_reading: { value: 6, on: "2026-10-14" } } });
    const sheet = await openCheck();
    await userEvent.click(within(sheet).getByRole("radio", { name: "6, moist" }));
    expect(within(sheet).getByText("Still moist — no water needed.")).toBeTruthy();
    expect(within(sheet).queryByRole("button", { name: "Watered" })).toBeNull();
    await userEvent.click(within(sheet).getByRole("button", { name: "Save reading" }));

    // Gone from Today at once; the toast offers Undo.
    await waitFor(() => expect(screen.queryByRole("button", { name: "Check soil for Monstera" })).toBeNull());
    const toast = await screen.findByText("Saved reading 6 · Moist for Monstera. Next check Sun, Oct 18.");
    expect(toast).toBeTruthy();
    expect(screen.getByRole("button", { name: "Undo" })).toBeTruthy();
    expect(posts).toEqual([expect.objectContaining({ kind: "checked", reading: 6 })]);
  });

  it("can water without a reading", async () => {
    const posts = setup({ event: { ...event, kind: "done", reading: null }, task });
    const sheet = await openCheck();
    await userEvent.click(within(sheet).getByRole("button", { name: "Watered without checking" }));
    expect(await screen.findByText("Watered Monstera.")).toBeTruthy();
    expect(posts).toEqual([expect.not.objectContaining({ reading: expect.anything() })]);
    expect(posts[0]).toMatchObject({ kind: "done" });
  });
});
