import { describe, expect, it } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { addDays, deviceToday } from "../lib/dates";
import { json, me, mockApi, renderApp, skipToSave } from "../test/render";

const kitchen = { id: "l1", name: "Kitchen", light: "bright_indirect", windows: [], suggested_light: null, notes: "", plant_count: 0 };
const plant = {
  id: "p1",
  name: "Monstera",
  display_name: "Monstera",
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

type Handlers = Parameters<typeof mockApi>[0];

/** Mocks the API and records each write's call and JSON body, in order. */
function setup(extra: Handlers = {}) {
  const writes: { call: string; body: unknown }[] = [];
  const record = (call: string, response: () => Response) => async (req: Request) => {
    writes.push({ call, body: await req.json() });
    return response();
  };
  const fetchMock = mockApi({
    "GET /api/v1/me": () => json(200, me),
    "GET /api/v1/locations": () => json(200, { items: [kitchen] }),
    "GET /api/v1/plants": () => json(200, { items: [plant] }),
    "GET /api/v1/plants/p1": () => json(200, plant),
    "POST /api/v1/plants": record("POST /plants", () => json(201, plant)),
    "POST /api/v1/plants/p1/tasks": record("POST /tasks", () => json(201, { id: "t1" })),
    ...extra,
  });
  return { writes, fetchMock };
}

/** Opens Add plant over the Plants list, so history has somewhere to go back to, and names the plant. */
async function openAdd() {
  const app = renderApp("/plants");
  await screen.findByRole("button", { name: "Add plant" });
  await app.router.navigate("/plants/new");
  await userEvent.type(await screen.findByLabelText("Name"), "  Monstera ");
  return app;
}

describe("Add plant", () => {
  it("asks for a name or a species before going on", async () => {
    const { fetchMock } = setup();
    renderApp("/plants/new");
    await userEvent.click(await screen.findByRole("button", { name: "Next" }));
    expect(screen.getByText("Give your plant a name or a species.")).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByLabelText("Name"));
    expect(fetchMock.mock.calls.some(([req]) => (req as Request).method === "POST")).toBe(false);
  });

  it("asks one question a screen, then saves the plant with a repot reminder", async () => {
    const { writes } = setup();
    const { router } = await openAdd();
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    await userEvent.click(await screen.findByRole("button", { name: /^Kitchen/ }));
    await userEvent.click(await screen.findByRole("button", { name: "Across the room" }));
    await userEvent.click(screen.getByRole("switch", { name: "Grow light" }));
    expect((screen.getByLabelText("Hours a day") as HTMLInputElement).value).toBe("12");
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    await userEvent.click(await screen.findByRole("button", { name: "A few months ago" }));

    await waitFor(() => expect(router.state.location.pathname).toBe("/plants/p1"));
    expect(writes).toEqual([
      {
        call: "POST /plants",
        body: {
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
          window_distance: "across_room",
          grow_light_hours: 12,
        },
      },
      {
        call: "POST /tasks",
        body: expect.objectContaining({ kind: "repot", interval_days: 365, start_on: addDays(deviceToday(), 245) }),
      },
    ]);
    expect(await screen.findByRole("heading", { level: 1, name: "Monstera" })).toBeTruthy();
    // The steps are gone from history: Back from the plant returns to the list.
    await router.navigate(-1);
    await waitFor(() => expect(router.state.location.pathname).toBe("/plants"));
  });

  it("goes back a step with the answers kept", async () => {
    setup();
    await openAdd();
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    await userEvent.click(await screen.findByRole("button", { name: "Back" }));
    expect((await screen.findByLabelText<HTMLInputElement>("Name")).value).toBe("  Monstera ");
  });

  it("checks the grow light hours", async () => {
    setup();
    await openAdd();
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    await userEvent.click(await screen.findByRole("button", { name: "Skip" }));
    await userEvent.click(await screen.findByRole("switch", { name: "Grow light" }));
    await userEvent.clear(screen.getByLabelText("Hours a day"));
    await userEvent.type(screen.getByLabelText("Hours a day"), "25");
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByText("Use whole hours from 1 to 24.")).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByLabelText("Hours a day"));
  });

  it("shows the server's problem detail on a 400", async () => {
    setup({
      "POST /api/v1/plants": () => json(400, { title: "Bad Request", status: 400, detail: "Location not found." }),
    });
    await openAdd();
    await skipToSave();
    expect((await screen.findByRole("alert")).textContent).toBe("Location not found.");
  });

  it("opens the plant even when its repot reminder fails, and says so", async () => {
    const { writes } = setup({ "POST /api/v1/plants/p1/tasks": () => json(500, { title: "Error", status: 500 }) });
    const { router } = await openAdd();
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    await userEvent.click(await screen.findByRole("button", { name: "Skip" }));
    await userEvent.click(await screen.findByRole("button", { name: "Skip" }));
    await userEvent.click(await screen.findByRole("button", { name: "Never, it's in its nursery pot" }));
    await waitFor(() => expect(router.state.location.pathname).toBe("/plants/p1"));
    expect(writes.map((w) => w.call)).toEqual(["POST /plants"]);
    expect(await screen.findByText("Added Monstera, but the repot reminder didn't save. Add it from Care.")).toBeTruthy();
  });
});
