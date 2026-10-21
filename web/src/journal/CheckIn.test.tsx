import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";
import { preparePhoto } from "../photos/prepare";

// jsdom can't decode images; prepare.test.ts and e2e cover the resize.
vi.mock("../photos/prepare", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../photos/prepare")>()),
  preparePhoto: vi.fn(),
}));

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
  growth: { height_in: 10, width_in: null, stage: "young", measured_on: "2026-09-14" },
  created_at: "2026-10-01T12:00:00Z",
};
const task = {
  id: "t1",
  plant_id: "p1",
  kind: "check_in",
  label: "",
  interval_days: 30,
  anchor: "from_completion",
  start_on: "2026-10-14",
  next_due: "2026-10-14",
  last_done: null,
  seasonal: false,
  moisture_check: false,
  water_at: 3,
  last_reading: null,
  drying_days: null,
  current_interval_days: 30,
};
const today = { date: "2026-10-14", task_count: 1, overdue: [], due: [{ task, plant, days_overdue: 0 }], upcoming: [] };
const photo = {
  id: "ph1",
  plant_id: "p1",
  taken_at: "2026-10-14T15:00:00Z",
  caption: "",
  width: 2560,
  height: 1920,
  urls: { small: "/media/photos/ph1/small", medium: "/media/photos/ph1/medium", original: "/media/photos/ph1/original" },
};
const measurement = {
  id: "m1",
  plant_id: "p1",
  measured_on: "2026-10-14",
  height_in: 12,
  width_in: null,
  stage: "young",
  note: "",
  photo,
};
const event = {
  id: "e1",
  task_id: "t1",
  kind: "done",
  local_date: "2026-10-14",
  snooze_until: null,
  reading: null,
  note: "",
  occurred_at: "2026-10-14T15:00:00Z",
};

type Step = { call: string; body?: unknown };

/** Records each write in order; `fail` makes the named call answer 500 (once, then succeed). */
function setup(fail: string[] = []) {
  const steps: Step[] = [];
  const failing = new Set(fail);
  const answer = (call: string, ok: () => Response) => {
    if (failing.delete(call)) return json(500, { title: "Server error", status: 500 });
    return ok();
  };
  mockApi({
    "GET /api/v1/me": () => json(200, me),
    "GET /api/v1/today": () => json(200, today),
    "POST /api/v1/plants/p1/photos": () => {
      steps.push({ call: "upload" });
      return answer("upload", () => json(201, photo));
    },
    "POST /api/v1/plants/p1/measurements": async (req) => {
      steps.push({ call: "create", body: await req.json() });
      return answer("create", () => json(201, measurement));
    },
    "PUT /api/v1/measurements/m1": async (req) => {
      steps.push({ call: "update", body: await req.json() });
      return json(200, measurement);
    },
    "POST /api/v1/tasks/t1/events": async (req) => {
      steps.push({ call: "done", body: await req.json() });
      return answer("done", () => json(201, { event, task: { ...task, next_due: "2026-11-13" } }));
    },
  });
  renderApp("/");
  return steps;
}

async function openCheckIn() {
  await userEvent.click(await screen.findByRole("button", { name: "Check in on Monstera" }));
  return screen.getByRole("dialog", { name: "Check in · Monstera" });
}

describe("Check in", () => {
  beforeEach(() => {
    vi.mocked(preparePhoto).mockResolvedValue({
      blob: new Blob(["jpeg"], { type: "image/jpeg" }),
      takenAt: "2026-10-14T15:00:00.000Z",
    });
  });
  afterEach(() => vi.restoreAllMocks());

  it("shows on Today as a check-in", async () => {
    setup();
    const row = await screen.findByRole("button", { name: /^Monstera/ });
    expect(within(row).getByText("Check in")).toBeTruthy();
  });

  it("uploads the photo, saves the measurement with it, then logs the task", async () => {
    const steps = setup();
    const sheet = await openCheckIn();
    // The stage starts at the plant's latest.
    expect(within(sheet).getByRole("radio", { name: "Young" })).toHaveProperty("checked", true);
    await userEvent.upload(within(sheet).getByLabelText("Add photo"), new File(["raw"], "a.jpg", { type: "image/jpeg" }));
    expect(within(sheet).getByRole("button", { name: "Remove photo" })).toBeTruthy();
    await userEvent.type(within(sheet).getByLabelText("Height"), "12");
    await userEvent.click(within(sheet).getByRole("button", { name: "Save check-in" }));

    expect(await screen.findByText("Checked in on Monstera.")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Undo" })).toBeTruthy();
    expect(steps.map((s) => s.call)).toEqual(["upload", "create", "done"]);
    expect(steps[1]?.body).toEqual({ height_in: 12, width_in: null, stage: "young", note: "", photo_id: "ph1" });
    expect(steps[2]?.body).toMatchObject({ kind: "done" });
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Check in · Monstera" })).toBeNull());
  });

  it("only logs the task when nothing was filled in", async () => {
    const steps = setup();
    const sheet = await openCheckIn();
    await userEvent.click(within(sheet).getByRole("button", { name: "Save check-in" }));
    expect(await screen.findByText("Checked in on Monstera.")).toBeTruthy();
    expect(steps.map((s) => s.call)).toEqual(["done"]);
  });

  it("stops at a failed upload, saving nothing, and keeps the sheet open", async () => {
    const steps = setup(["upload"]);
    const sheet = await openCheckIn();
    await userEvent.upload(within(sheet).getByLabelText("Add photo"), new File(["raw"], "a.jpg", { type: "image/jpeg" }));
    await userEvent.type(within(sheet).getByLabelText("Height"), "12");
    await userEvent.click(within(sheet).getByRole("button", { name: "Save check-in" }));

    expect(await within(sheet).findByText(/^The photo didn't upload, so nothing was saved\./)).toBeTruthy();
    expect(steps.map((s) => s.call)).toEqual(["upload"]);
    expect(screen.getByRole("dialog", { name: "Check in · Monstera" })).toBeTruthy();

    // Trying again goes all the way through.
    await userEvent.click(within(sheet).getByRole("button", { name: "Save check-in" }));
    expect(await screen.findByText("Checked in on Monstera.")).toBeTruthy();
    expect(steps.map((s) => s.call)).toEqual(["upload", "upload", "create", "done"]);
  });

  it("says when the measurement saved but the task didn't, and retries without duplicating", async () => {
    const steps = setup(["done"]);
    const sheet = await openCheckIn();
    await userEvent.upload(within(sheet).getByLabelText("Add photo"), new File(["raw"], "a.jpg", { type: "image/jpeg" }));
    await userEvent.type(within(sheet).getByLabelText("Height"), "12");
    await userEvent.click(within(sheet).getByRole("button", { name: "Save check-in" }));

    expect(
      await within(sheet).findByText("Saved to the growth log, but the check-in isn't marked done yet. Try again."),
    ).toBeTruthy();
    await userEvent.click(within(sheet).getByRole("button", { name: "Save check-in" }));
    expect(await screen.findByText("Checked in on Monstera.")).toBeTruthy();
    // The photo isn't sent twice, and the measurement is updated rather than created again.
    expect(steps.map((s) => s.call)).toEqual(["upload", "create", "done", "update", "done"]);
  });

  it("explains a size it can't read", async () => {
    const steps = setup();
    const sheet = await openCheckIn();
    await userEvent.type(within(sheet).getByLabelText("Height"), "tall");
    await userEvent.click(within(sheet).getByRole("button", { name: "Save check-in" }));
    expect(within(sheet).getByText("Enter inches from 0.1 to 600, like 12.5.")).toBeTruthy();
    expect(steps).toEqual([]);
  });
});
