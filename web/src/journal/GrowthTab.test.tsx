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
  growth: null,
  created_at: "2026-03-01T12:00:00Z",
};
const older = {
  id: "m1",
  plant_id: "p1",
  measured_on: "2026-03-02",
  height_in: 12,
  width_in: null,
  stage: "young",
  note: "",
  photo: null,
};
const newer = {
  id: "m2",
  plant_id: "p1",
  measured_on: "2026-10-01",
  height_in: 18,
  width_in: 14,
  stage: "mature",
  note: "Two new leaves",
  photo: null,
};

const baseApi = {
  "GET /api/v1/me": () => json(200, me),
  "GET /api/v1/locations": () => json(200, { items: [] }),
  "GET /api/v1/plants/p1/tasks": () => json(200, { items: [] }),
  "GET /api/v1/plants/p1/events": () => json(200, { items: [] }),
  "GET /api/v1/today": () => json(200, { date: "2026-10-21", task_count: 0, overdue: [], due: [], upcoming: [] }),
};

async function openGrowthTab() {
  renderApp("/plants/p1");
  await userEvent.click(await screen.findByRole("tab", { name: "Growth" }));
}

describe("Growth tab", () => {
  it("charts height and lists measurements, newest first", async () => {
    mockApi({
      ...baseApi,
      "GET /api/v1/plants/p1": () =>
        json(200, { ...plant, growth: { height_in: 18, width_in: 14, stage: "mature", measured_on: "2026-10-01" } }),
      "GET /api/v1/plants/p1/measurements": () => json(200, { items: [newer, older] }),
    });
    await openGrowthTab();
    expect(await screen.findByRole("img", { name: "Height grew from 12 in to 18 in since March" })).toBeTruthy();
    const items = screen.getAllByRole("listitem").filter((li) => within(li).queryByText(/tall/));
    expect(items.map((li) => li.textContent)).toEqual([
      "Oct 118 in tall · 14 in wide · MatureTwo new leaves",
      "Mar 212 in tall · Young",
    ]);
    // The plant's meta line has the latest size.
    expect(screen.getByText("18 in tall · Mature")).toBeTruthy();
  });

  it("leaves the chart out with fewer than two heights", async () => {
    mockApi({
      ...baseApi,
      "GET /api/v1/plants/p1": () => json(200, plant),
      "GET /api/v1/plants/p1/measurements": () => json(200, { items: [older] }),
    });
    await openGrowthTab();
    expect(await screen.findByText("12 in tall · Young")).toBeTruthy();
    expect(screen.queryByRole("img", { name: /^Height/ })).toBeNull();
  });

  it("adds a first measurement, asking for something to save", async () => {
    let items: unknown[] = [];
    let posted: unknown;
    mockApi({
      ...baseApi,
      "GET /api/v1/plants/p1": () => json(200, plant),
      "GET /api/v1/plants/p1/measurements": () => json(200, { items }),
      "POST /api/v1/plants/p1/measurements": async (req) => {
        posted = await req.json();
        items = [{ ...older, measured_on: "2026-10-21", height_in: 15, stage: null }];
        return json(201, items[0]);
      },
    });
    await openGrowthTab();
    expect(await screen.findByText("No measurements yet")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Add measurement" }));
    const sheet = screen.getByRole("dialog", { name: "Add measurement" });
    expect(within(sheet).getByLabelText("Date")).toHaveProperty("value", "2026-10-21");

    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
    expect(within(sheet).getByText("Add a size, stage, note or photo.")).toBeTruthy();
    expect(posted).toBeUndefined();

    await userEvent.type(within(sheet).getByLabelText("Height"), "15");
    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Measurement added.")).toBeTruthy();
    expect(posted).toEqual({ measured_on: "2026-10-21", height_in: 15, width_in: null, stage: null, note: "", photo_id: null });
    expect(await screen.findByText("15 in tall")).toBeTruthy();
  });

  it("edits and deletes a measurement", async () => {
    let items: unknown[] = [older];
    let put: unknown;
    mockApi({
      ...baseApi,
      "GET /api/v1/plants/p1": () => json(200, plant),
      "GET /api/v1/plants/p1/measurements": () => json(200, { items }),
      "PUT /api/v1/measurements/m1": async (req) => {
        put = await req.json();
        items = [{ ...older, height_in: 12.5 }];
        return json(200, items[0]);
      },
      "DELETE /api/v1/measurements/m1": () => {
        items = [];
        return new Response(null, { status: 204 });
      },
    });
    await openGrowthTab();
    await userEvent.click(await screen.findByRole("button", { name: /12 in tall · Young/ }));
    let sheet = screen.getByRole("dialog", { name: "Edit measurement" });
    const height = within(sheet).getByLabelText("Height");
    expect(height).toHaveProperty("value", "12");
    await userEvent.clear(height);
    await userEvent.type(height, "12.5");
    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
    expect(await screen.findByText("12.5 in tall · Young")).toBeTruthy();
    expect(put).toMatchObject({ measured_on: "2026-03-02", height_in: 12.5, stage: "young" });

    await userEvent.click(screen.getByRole("button", { name: /12.5 in tall · Young/ }));
    sheet = screen.getByRole("dialog", { name: "Edit measurement" });
    await userEvent.click(within(sheet).getByRole("button", { name: "Delete measurement" }));
    await userEvent.click(within(sheet).getByRole("button", { name: "Delete" }));
    expect(await screen.findByText("Measurement deleted.")).toBeTruthy();
    await waitFor(() => expect(screen.getByText("No measurements yet")).toBeTruthy());
  });
});
