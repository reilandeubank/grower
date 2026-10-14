import { describe, expect, it } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";

const kitchen = { id: "l1", name: "Kitchen", light: "bright_indirect", notes: "", plant_count: 0 };
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
      location_id: "l1",
      acquired_on: null,
      notes: "",
      archived: false,
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
    expect(body).toEqual({ name: "Monstera", species: "", location_id: "l1", acquired_on: null, notes: "" });
    expect(await screen.findByRole("heading", { level: 1, name: "Monstera" })).toBeTruthy();
  });
});
