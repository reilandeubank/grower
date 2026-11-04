import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";

const kitchen = { id: "l1", name: "Kitchen", light: "bright_indirect", windows: [], suggested_light: null, notes: "", plant_count: 0 };
const base = {
  "GET /api/v1/me": () => json(200, me),
  "GET /api/v1/locations": () => json(200, { items: [kitchen] }),
};

const calathea = {
  id: "p2",
  name: "Calathea",
  display_name: "Calathea",
  species: "",
  species_id: null,
  location_id: null,
  acquired_on: null,
  notes: "",
  archived: false,
  cover: null,
  created_at: "2026-10-14T12:00:00Z",
  pot_type: null,
  pot_size_in: null,
  drainage: null,
  soil: "",
  light: null,
  window_distance: null,
  grow_light_hours: null,
};

/** Opens the edit form for Calathea; PUTs land in `sent.body` and answer with `put`. */
function editCalathea(put: (body: Record<string, unknown>) => Response = (body) => json(200, { ...calathea, ...body })) {
  const sent: { body: Record<string, unknown> } = { body: {} };
  const fetchMock = mockApi({
    ...base,
    "GET /api/v1/plants/p2": () => json(200, { ...calathea, ...sent.body }),
    "GET /api/v1/plants/p2/tasks": () => json(200, { items: [] }),
    "GET /api/v1/plants/p2/events": () => json(200, { items: [] }),
    "PUT /api/v1/plants/p2": async (req) => {
      sent.body = (await req.json()) as Record<string, unknown>;
      return put(sent.body);
    },
  });
  renderApp("/plants/p2/edit");
  return { sent, fetchMock };
}

describe("Edit plant", () => {
  it("saves the pot and soil from their folded section, and the detail page sums them up", async () => {
    const { sent } = editCalathea();
    await userEvent.click(await screen.findByText("Pot and soil"));
    await userEvent.selectOptions(screen.getByLabelText("Pot type"), "Terracotta");
    await userEvent.type(screen.getByLabelText("Pot size"), "6.5");
    await userEvent.selectOptions(screen.getByLabelText("Drainage holes"), "Yes");
    await userEvent.type(screen.getByLabelText("Soil"), " Aroid mix ");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByText("6.5 in terracotta pot · drains · Aroid mix");
    expect(sent.body).toMatchObject({ pot_type: "terracotta", pot_size_in: 6.5, drainage: true, soil: "Aroid mix" });
  });

  it("saves the light group, including the light level override", async () => {
    const { sent } = editCalathea(() => json(400, { title: "Bad Request", status: 400, detail: "Stop here." }));
    await userEvent.selectOptions(await screen.findByLabelText("Distance from the window"), "Across the room");
    await userEvent.click(screen.getByRole("switch", { name: "Grow light" }));
    await userEvent.selectOptions(screen.getByLabelText("Light level (optional)"), "Medium light");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect((await screen.findByRole("alert")).textContent).toBe("Stop here.");
    expect(sent.body).toMatchObject({ window_distance: "across_room", grow_light_hours: 12, light: "medium" });
  });

  it("checks the pot size range", async () => {
    const { fetchMock } = editCalathea();
    await userEvent.click(await screen.findByText("Pot and soil"));
    await userEvent.type(screen.getByLabelText("Pot size"), "60");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByText("Use a size from 0.5 to 48 inches.")).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByLabelText("Pot size"));
    expect(fetchMock.mock.calls.some(([req]) => (req as Request).method === "PUT")).toBe(false);
  });
});
