import { describe, expect, it } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";

const kitchen = { id: "l1", name: "Kitchen", light: "bright_indirect", windows: [], suggested_light: null, notes: "", plant_count: 0 };
const base = {
  "GET /api/v1/me": () => json(200, me),
  "GET /api/v1/locations": () => json(200, { items: [kitchen] }),
};

describe("Plant form", () => {
  it("asks for a name without calling the API", async () => {
    const fetchMock = mockApi(base);
    renderApp("/plants/new");
    await userEvent.click(await screen.findByRole("button", { name: "Save" }));
    expect(screen.getByText("Give your plant a name.")).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByLabelText("Name"));
    expect(fetchMock.mock.calls.some(([req]) => (req as Request).method === "POST")).toBe(false);
  });

  it("shows the server's problem detail on a 400", async () => {
    mockApi({
      ...base,
      "POST /api/v1/plants": () =>
        json(400, { title: "Bad Request", status: 400, detail: "Acquired can't be in the future." }),
    });
    renderApp("/plants/new");
    await userEvent.type(await screen.findByLabelText("Name"), "Monstera");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect((await screen.findByRole("alert")).textContent).toBe("Acquired can't be in the future.");
  });

  it("creates the plant with trimmed fields and opens its detail page", async () => {
    let body: unknown;
    const plant = {
      id: "p1",
      name: "Monstera",
      species: "",
      species_id: null,
      location_id: "l1",
      acquired_on: null,
      notes: "",
      archived: false,
      pot_type: null,
      pot_size_in: null,
      drainage: null,
      soil: "",
      created_at: "2026-10-14T12:00:00Z",
    };
    mockApi({
      ...base,
      "POST /api/v1/plants": async (req) => {
        body = await req.json();
        return json(201, plant);
      },
      "GET /api/v1/plants/p1": () => json(200, plant),
    });
    const { router } = renderApp("/plants/new");
    await userEvent.type(await screen.findByLabelText("Name"), "  Monstera ");
    await userEvent.selectOptions(screen.getByLabelText("Location"), "Kitchen");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(router.state.location.pathname).toBe("/plants/p1"));
    expect(body).toEqual({
      name: "Monstera",
      species: "",
      species_id: null,
      location_id: "l1",
      acquired_on: null,
      notes: "",
      pot_type: null,
      pot_size_in: null,
      drainage: null,
      soil: "",
      light: null,
      window_distance: null,
      grow_light_hours: null,
    });
    expect(await screen.findByRole("heading", { level: 1, name: "Monstera" })).toBeTruthy();
  });

  it("saves the pot and soil, and the detail page sums them up", async () => {
    let body: Record<string, unknown> = {};
    const plant = {
      id: "p2",
      name: "Calathea",
      species: "",
      location_id: null,
      acquired_on: null,
      notes: "",
      archived: false,
      cover: null,
      created_at: "2026-10-14T12:00:00Z",
      pot_type: "terracotta",
      pot_size_in: 6.5,
      drainage: true,
      soil: "Aroid mix",
    };
    mockApi({
      ...base,
      "POST /api/v1/plants": async (req) => {
        body = (await req.json()) as Record<string, unknown>;
        return json(201, plant);
      },
      "GET /api/v1/plants/p2": () => json(200, plant),
      "GET /api/v1/plants/p2/tasks": () => json(200, { items: [] }),
      "GET /api/v1/plants/p2/events": () => json(200, { items: [] }),
    });
    renderApp("/plants/new");
    await userEvent.type(await screen.findByLabelText("Name"), "Calathea");
    await userEvent.selectOptions(screen.getByLabelText("Pot type"), "Terracotta");
    await userEvent.type(screen.getByLabelText("Pot size"), "6.5");
    await userEvent.selectOptions(screen.getByLabelText("Drainage holes"), "Yes");
    await userEvent.type(screen.getByLabelText("Soil"), " Aroid mix ");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByRole("heading", { level: 1, name: "Calathea" });
    expect(body).toMatchObject({ pot_type: "terracotta", pot_size_in: 6.5, drainage: true, soil: "Aroid mix" });
    expect(screen.getByText("6.5 in terracotta pot · drains · Aroid mix")).toBeTruthy();
  });

  it("saves the light group, with a grow light at 12 hours unless changed", async () => {
    let body: Record<string, unknown> = {};
    mockApi({
      ...base,
      "POST /api/v1/plants": async (req) => {
        body = (await req.json()) as Record<string, unknown>;
        return json(400, { title: "Bad Request", status: 400, detail: "Stop here." });
      },
    });
    renderApp("/plants/new");
    await userEvent.type(await screen.findByLabelText("Name"), "Fern");
    await userEvent.selectOptions(screen.getByLabelText("Distance from the window"), "Across the room");
    expect(screen.queryByLabelText("Hours a day")).toBeNull();
    await userEvent.click(screen.getByRole("switch", { name: "Grow light" }));
    expect((screen.getByLabelText("Hours a day") as HTMLInputElement).value).toBe("12");
    await userEvent.selectOptions(screen.getByLabelText("Light level (optional)"), "Medium light");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByRole("alert");
    expect(body).toMatchObject({ window_distance: "across_room", grow_light_hours: 12, light: "medium" });
  });

  it("checks the grow light hours", async () => {
    const fetchMock = mockApi(base);
    renderApp("/plants/new");
    await userEvent.type(await screen.findByLabelText("Name"), "Fern");
    await userEvent.click(screen.getByRole("switch", { name: "Grow light" }));
    await userEvent.clear(screen.getByLabelText("Hours a day"));
    await userEvent.type(screen.getByLabelText("Hours a day"), "25");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByText("Use whole hours from 1 to 24.")).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByLabelText("Hours a day"));
    expect(fetchMock.mock.calls.some(([req]) => (req as Request).method === "POST")).toBe(false);
  });

  it("checks the pot size range", async () => {
    const fetchMock = mockApi(base);
    renderApp("/plants/new");
    await userEvent.type(await screen.findByLabelText("Name"), "Fern");
    await userEvent.type(screen.getByLabelText("Pot size"), "60");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByText("Use a size from 0.5 to 48 inches.")).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByLabelText("Pot size"));
    expect(fetchMock.mock.calls.some(([req]) => (req as Request).method === "POST")).toBe(false);
  });
});
