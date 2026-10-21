import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";

const den = { id: "l1", name: "Den", light: null, windows: [], suggested_light: null, notes: "", plant_count: 1 };

describe("Locations", () => {
  it("adds windows, estimates the light as you tap, and saves them", async () => {
    let body: Record<string, unknown> = {};
    let items = [den];
    mockApi({
      "GET /api/v1/me": () => json(200, me),
      "GET /api/v1/locations": () => json(200, { items }),
      "PUT /api/v1/locations/l1": async (req) => {
        body = (await req.json()) as Record<string, unknown>;
        items = [{ ...den, windows: [{ direction: "e", count: 1 }], suggested_light: "bright_indirect" } as never];
        return json(200, items[0]);
      },
    });
    renderApp("/settings/locations");
    await userEvent.click(await screen.findByRole("button", { name: /Den/ }));
    const sheet = screen.getByRole("dialog", { name: "Edit location" });
    expect(within(sheet).getByRole("option", { name: "Not set" })).toBeTruthy();

    await userEvent.click(within(sheet).getByRole("button", { name: "East windows: 0" }));
    expect(within(sheet).getByText(/^Suggested light:/).textContent).toBe("Suggested light: Bright, indirect");
    expect(within(sheet).getByRole("option", { name: "Use the windows' estimate" })).toBeTruthy();

    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(body).toMatchObject({ light: null, windows: [{ direction: "e", count: 1 }] }));
    expect(await screen.findByText("1 E · Bright, indirect (est.) · 1 plant")).toBeTruthy();
  });
});
