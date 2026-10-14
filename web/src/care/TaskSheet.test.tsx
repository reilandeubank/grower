import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";

const plant = {
  id: "p1",
  name: "Fern",
  species: "",
  location_id: null,
  acquired_on: null,
  notes: "",
  archived: false,
  created_at: "2026-10-01T12:00:00Z",
};

function setup() {
  const posts: unknown[] = [];
  mockApi({
    "GET /api/v1/me": () => json(200, me),
    "GET /api/v1/locations": () => json(200, { items: [] }),
    "GET /api/v1/plants/p1": () => json(200, plant),
    "GET /api/v1/plants/p1/tasks": () => json(200, { items: [] }),
    "GET /api/v1/plants/p1/events": () => json(200, { items: [] }),
    "GET /api/v1/today": () => json(200, { date: "2026-10-14", overdue: [], due: [], upcoming: [] }),
    "POST /api/v1/plants/p1/tasks": async (req) => {
      posts.push(await req.json());
      return json(201, { id: "t1", plant_id: "p1", next_due: "2026-10-14", last_done: null, ...(posts.at(-1) as object) });
    },
  });
  renderApp("/plants/p1");
  return posts;
}

async function openSheet() {
  await userEvent.click(await screen.findByRole("button", { name: "Add care task" }));
  return screen.getByRole("dialog", { name: "Add care task" });
}

describe("Task form", () => {
  it("needs a name for a custom task", async () => {
    const posts = setup();
    const sheet = await openSheet();
    expect(within(sheet).queryByLabelText("Name")).toBeNull();
    await userEvent.click(within(sheet).getByRole("button", { name: "Custom" }));
    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
    expect(within(sheet).getByText("Give the task a name.")).toBeTruthy();
    expect(posts).toHaveLength(0);
  });

  it("keeps the interval from 1 to 365 days", async () => {
    const posts = setup();
    const sheet = await openSheet();
    const days = within(sheet).getByLabelText("Days between");
    for (const value of ["0", "366", "2.5"]) {
      await userEvent.clear(days);
      await userEvent.type(days, value);
      await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
      expect(within(sheet).getByText("Enter a number of days from 1 to 365.")).toBeTruthy();
    }
    expect(posts).toHaveLength(0);
  });

  it("adds a task with the chosen schedule", async () => {
    const posts = setup();
    const sheet = await openSheet();
    await userEvent.click(within(sheet).getByRole("button", { name: "Fertilize" }));
    await userEvent.click(within(sheet).getByRole("button", { name: "14 days" }));
    expect(within(sheet).getByText("Every 14 days")).toBeTruthy();
    await userEvent.click(within(sheet).getByRole("button", { name: "Keep a fixed schedule" }));
    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(posts).toEqual([
      { kind: "fertilize", label: "", interval_days: 14, anchor: "fixed", start_on: "2026-10-14" },
    ]);
  });
});
