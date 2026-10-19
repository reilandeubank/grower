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
  pot_type: null,
  pot_size_in: null,
  drainage: null,
  soil: "",
  cover: null,
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
      { kind: "fertilize", label: "", interval_days: 14, anchor: "fixed", start_on: "2026-10-14", seasonal: true, moisture_check: false, water_at: 3 },
    ]);
  });

  it("adjusts for seasons by default for the kinds the seasons change", async () => {
    const posts = setup();
    const sheet = await openSheet();
    const seasons = () => within(sheet).queryByRole("switch", { name: /Adjust for seasons/ }) as HTMLInputElement | null;
    expect(seasons()?.checked).toBe(true);
    expect(within(sheet).getByText("Waters less often in winter and a little more in summer.")).toBeTruthy();

    await userEvent.click(within(sheet).getByRole("button", { name: "Rotate" }));
    expect(seasons()).toBeNull();
    await userEvent.click(within(sheet).getByRole("button", { name: "Fertilize" }));
    expect(seasons()?.checked).toBe(true);
    expect(within(sheet).getByText("Feeds less often from fall to early spring.")).toBeTruthy();

    await userEvent.click(within(sheet).getByRole("button", { name: "Keep a fixed schedule" }));
    expect(seasons()?.disabled).toBe(true);
    expect(within(sheet).getByText("Only for schedules counted from when it's done.")).toBeTruthy();
    await userEvent.click(within(sheet).getByRole("button", { name: "Count from when it's done" }));
    await userEvent.click(seasons()!);
    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(posts).toEqual([
      { kind: "fertilize", label: "", interval_days: 7, anchor: "from_completion", start_on: "2026-10-14", seasonal: false, moisture_check: false, water_at: 3 },
    ]);
  });

  it("turns a water task into a soil check, and back", async () => {
    const posts = setup();
    const sheet = await openSheet();
    const mode = () => within(sheet).getByRole("group", { name: "How do you decide when to water?" });
    await userEvent.click(within(mode()).getByRole("button", { name: "Check the soil first" }));

    // The interval is a first guess, the threshold appears, and the schedule options go.
    expect(within(sheet).getByLabelText("Check about every")).toBeTruthy();
    expect(within(sheet).getByText("Your first guess; grower learns from your readings.")).toBeTruthy();
    expect(within(sheet).getByText("3 · Dry")).toBeTruthy();
    expect(within(sheet).queryByRole("switch", { name: /Adjust for seasons/ })).toBeNull();
    expect(within(sheet).queryByRole("button", { name: "Keep a fixed schedule" })).toBeNull();
    const threshold = within(sheet).getByRole("radiogroup", { name: "Water at or below" });
    expect(within(threshold).getByRole("radio", { name: "3, dry" }).getAttribute("aria-checked")).toBe("true");
    expect((within(threshold).getByRole("radio", { name: "10, wet" }) as HTMLButtonElement).disabled).toBe(true);
    await userEvent.click(within(threshold).getByRole("radio", { name: "4, moist" }));
    expect(within(sheet).getByText("4 · Moist")).toBeTruthy();

    // Back to a schedule restores its controls.
    await userEvent.click(within(mode()).getByRole("button", { name: "On a schedule" }));
    expect(within(sheet).getByLabelText("Days between")).toBeTruthy();
    expect(within(sheet).getByRole("switch", { name: /Adjust for seasons/ })).toBeTruthy();
    // A fixed schedule picked before switching doesn't carry into a check.
    await userEvent.click(within(sheet).getByRole("button", { name: "Keep a fixed schedule" }));
    await userEvent.click(within(mode()).getByRole("button", { name: "Check the soil first" }));

    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(posts).toEqual([
      {
        kind: "water",
        label: "",
        interval_days: 7,
        anchor: "from_completion",
        start_on: "2026-10-14",
        seasonal: true,
        moisture_check: true,
        water_at: 4,
      },
    ]);
  });

  it("only offers soil checks for water", async () => {
    setup();
    const sheet = await openSheet();
    expect(within(sheet).getByRole("group", { name: "How do you decide when to water?" })).toBeTruthy();
    await userEvent.click(within(sheet).getByRole("button", { name: "Mist" }));
    expect(within(sheet).queryByRole("group", { name: "How do you decide when to water?" })).toBeNull();
  });
});
