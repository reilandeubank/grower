import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { CareTask, Plant } from "../api/client";
import { json, me, mockApi, renderApp } from "../test/render";
import { monstera, plant, waterTask } from "./fixtures";

function setup(opts: { plant?: Plant; tasks?: CareTask[]; apply?: () => Response } = {}) {
  const sent = { species: undefined as unknown };
  let tasks = opts.tasks ?? [];
  mockApi({
    "GET /api/v1/me": () => json(200, me),
    "GET /api/v1/locations": () => json(200, { items: [] }),
    "GET /api/v1/plants/p1": () => json(200, opts.plant ?? plant),
    "GET /api/v1/plants/p1/tasks": () => json(200, { items: tasks }),
    "GET /api/v1/plants/p1/events": () => json(200, { items: [] }),
    "GET /api/v1/species/s1": () => json(200, monstera),
    "PUT /api/v1/species/s1": async (req) => {
      sent.species = await req.json();
      return json(200, { ...monstera, ...(sent.species as object), user_edited: true });
    },
    "POST /api/v1/plants/p1/apply-species": () => {
      if (opts.apply) return opts.apply();
      tasks = [{ ...waterTask, interval_days: 7 }];
      return json(200, { items: tasks });
    },
  });
  renderApp("/plants/p1");
  return sent;
}

const card = () => screen.findByRole("region", { name: "Care guide" });

describe("Care guide", () => {
  it("shows the linked species' defaults and where they came from", async () => {
    setup();
    const guide = await card();
    expect(within(guide).getByText("Monstera deliciosa")).toBeTruthy();
    expect(within(guide).getByText("· Swiss cheese plant")).toBeTruthy();
    expect(within(guide).getByText("Water about every 7 days")).toBeTruthy();
    expect(within(guide).getByText("Bright, indirect light")).toBeTruthy();
    expect(within(guide).getByText("Water at 3 · Dry")).toBeTruthy();
    expect(within(guide).getByText("From Perenual")).toBeTruthy();
  });

  it("isn't shown without a linked species", async () => {
    setup({ plant: { ...plant, species_id: null } });
    await screen.findByRole("heading", { level: 1, name: "Monty" });
    expect(screen.getByText("Monstera deliciosa")).toBeTruthy();
    expect(screen.queryByRole("region", { name: "Care guide" })).toBeNull();
  });

  it("adds a water task and says so", async () => {
    setup();
    await screen.findByRole("heading", { name: "No care schedule yet" });
    await userEvent.click(within(await card()).getByRole("button", { name: "Apply to care schedule" }));
    expect(await screen.findByText("Water task added.")).toBeTruthy();
    expect(await screen.findByText(/^Every 7 days/)).toBeTruthy();
  });

  it("names the new interval when it updates an existing task", async () => {
    setup({ tasks: [{ ...waterTask, interval_days: 3, current_interval_days: 3 }] });
    await screen.findByText(/^Every 3 days/);
    await userEvent.click(within(await card()).getByRole("button", { name: "Apply to care schedule" }));
    expect(await screen.findByText("Watering set to every 7 days.")).toBeTruthy();
  });

  it("shows the server's reason when there's nothing to apply", async () => {
    setup({
      apply: () => json(400, { title: "Bad Request", status: 400, detail: "This species has no care defaults yet." }),
    });
    await userEvent.click(within(await card()).getByRole("button", { name: "Apply to care schedule" }));
    expect((await screen.findByRole("alert")).textContent).toBe("This species has no care defaults yet.");
  });

  it("edits the defaults in a sheet", async () => {
    const sent = setup();
    await userEvent.click(within(await card()).getByRole("button", { name: "Edit care guide" }));
    const sheet = screen.getByRole("dialog", { name: "Edit care guide" });
    await userEvent.clear(within(sheet).getByLabelText("Water every"));
    await userEvent.type(within(sheet).getByLabelText("Water every"), "5");
    await userEvent.selectOptions(within(sheet).getByLabelText("Light"), "Medium light");
    await userEvent.click(within(sheet).getByRole("button", { name: "Clear" }));
    expect(within(sheet).getByText("Water at, for soil checks").textContent).toBe("Water at, for soil checksNot set");
    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(sent.species).toEqual({
      scientific_name: "Monstera deliciosa",
      common_name: "Swiss cheese plant",
      water_interval_days: 5,
      light: "medium",
      water_at: null,
      notes: "",
    });
    const guide = await card();
    expect(within(guide).getByText("Water about every 5 days")).toBeTruthy();
    expect(within(guide).getByText("Edited by you")).toBeTruthy();
  });
});
