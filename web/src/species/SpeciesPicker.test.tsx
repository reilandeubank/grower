import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { SpeciesLookup } from "../api/client";
import { json, me, mockApi, renderApp } from "../test/render";
import { monstera, plant } from "./fixtures";

const ficus = { ...monstera, id: "s2", scientific_name: "Ficus lyrata", common_name: "Fiddle-leaf fig" };
const candidate = { source: "openplantbook", common_name: "", saved_id: null } as const;

/** Mocks the form's API; returns what the plant was saved with and every species request. */
function setup(opts: { lookup?: SpeciesLookup; edit?: boolean } = {}) {
  const sent: { plant?: Record<string, unknown>; lookups: string[]; imports: unknown[] } = { lookups: [], imports: [] };
  const saved = { ...plant, id: "p9" };
  const fetchMock = mockApi({
    "GET /api/v1/me": () => json(200, me),
    "GET /api/v1/locations": () => json(200, { items: [] }),
    "GET /api/v1/plants/p1": () => json(200, plant),
    "GET /api/v1/species": (req) => {
      const q = new URL(req.url).searchParams.get("q")?.toLowerCase() ?? "";
      return json(200, { items: [monstera].filter((s) => s.scientific_name.toLowerCase().includes(q)) });
    },
    "GET /api/v1/species/lookup": (req) => {
      sent.lookups.push(new URL(req.url).searchParams.get("q") ?? "");
      return json(200, opts.lookup ?? { providers: [], items: [] });
    },
    "POST /api/v1/species/import": async (req) => {
      sent.imports.push(await req.json());
      return json(200, ficus);
    },
    "POST /api/v1/species": async (req) => {
      const body = (await req.json()) as { scientific_name: string };
      return json(201, { ...ficus, id: "s3", source: "manual", scientific_name: body.scientific_name });
    },
    "POST /api/v1/plants": async (req) => {
      sent.plant = (await req.json()) as Record<string, unknown>;
      return json(201, saved);
    },
    "PUT /api/v1/plants/p1": async (req) => {
      sent.plant = (await req.json()) as Record<string, unknown>;
      return json(200, plant);
    },
    "GET /api/v1/plants/p9": () => json(200, saved),
    "GET /api/v1/species/s1": () => json(200, monstera),
  });
  renderApp(opts.edit ? "/plants/p1/edit" : "/plants/new");
  return { sent, fetchMock };
}

const combobox = () => screen.findByRole("combobox", { name: "Species" });

async function save() {
  if (!(screen.getByLabelText("Name") as HTMLInputElement).value) {
    await userEvent.type(screen.getByLabelText("Name"), "Plant");
  }
  await userEvent.click(screen.getByRole("button", { name: "Save" }));
}

describe("Species picker", () => {
  it("suggests saved species as you type and links the one you pick", async () => {
    const { sent } = setup();
    await userEvent.type(await combobox(), "mons");
    await userEvent.click(await screen.findByRole("option", { name: /Monstera deliciosa.*Swiss cheese plant/ }));
    expect((await combobox()).getAttribute("aria-expanded")).toBe("false");
    expect(((await combobox()) as HTMLInputElement).value).toBe("Monstera deliciosa");
    expect(screen.getByText("Linked to a saved species")).toBeTruthy();
    await save();
    await waitFor(() => expect(sent.plant).toMatchObject({ species: "Monstera deliciosa", species_id: "s1" }));
    expect(sent.lookups).toEqual([]);
  });

  it("searches online only from its row, then imports and links a result", async () => {
    const { sent } = setup({
      lookup: {
        providers: [
          { source: "openplantbook", ok: true },
          { source: "perenual", ok: false, detail: "Daily limit reached." },
        ],
        items: [
          { ...candidate, ref: "ficus lyrata", scientific_name: "Ficus lyrata", common_name: "Fiddle-leaf fig" },
          { ...candidate, ref: "ficus elastica", scientific_name: "Ficus elastica", saved_id: "s7" },
        ],
      },
    });
    await userEvent.type(await combobox(), "ficus");
    const search = await screen.findByRole("option", { name: "Search online for “ficus”" });
    expect(sent.lookups).toEqual([]);

    await userEvent.click(search);
    const listbox = screen.getByRole("listbox");
    const elastica = await within(listbox).findByRole("option", { name: /Ficus elastica/ });
    expect(within(elastica).getByText("Open Plantbook")).toBeTruthy();
    expect(within(elastica).getByText("Saved")).toBeTruthy();
    expect(within(listbox).getByText("Perenual didn't answer. Daily limit reached.")).toBeTruthy();
    expect(within(listbox).queryByRole("option", { name: /Search online/ })).toBeNull();
    expect(sent.lookups).toEqual(["ficus"]);

    await userEvent.click(within(listbox).getByRole("option", { name: /Ficus lyrata/ }));
    await waitFor(() => expect(screen.getByRole("combobox", { name: "Species" })).toHaveProperty("value", "Ficus lyrata"));
    expect(sent.imports).toEqual([{ source: "openplantbook", ref: "ficus lyrata" }]);
    await save();
    await waitFor(() => expect(sent.plant).toMatchObject({ species: "Ficus lyrata", species_id: "s2" }));
  });

  it("says when online lookup isn't set up", async () => {
    setup({ lookup: { providers: [], items: [] } });
    await userEvent.type(await combobox(), "ficus");
    await userEvent.click(await screen.findByRole("option", { name: /Search online/ }));
    expect(
      await screen.findByText("Online lookup isn't set up. Ask whoever runs grower to add API keys."),
    ).toBeTruthy();
  });

  it("says when no provider found anything", async () => {
    setup({ lookup: { providers: [{ source: "perenual", ok: true }], items: [] } });
    await userEvent.type(await combobox(), "zzz");
    await userEvent.click(await screen.findByRole("option", { name: /Search online/ }));
    expect(await screen.findByText("No matches online.")).toBeTruthy();
  });

  it("saves a typed name as a new species and links it", async () => {
    const { sent } = setup();
    await userEvent.type(await combobox(), "Pilea peperomioides");
    await userEvent.click(await screen.findByRole("option", { name: "Save as a new species “Pilea peperomioides”" }));
    await waitFor(() => expect(screen.getByText("Linked to a saved species")).toBeTruthy());
    await save();
    await waitFor(() => expect(sent.plant).toMatchObject({ species: "Pilea peperomioides", species_id: "s3" }));
  });

  it("unlinks when the text is cleared, keeping what you type instead", async () => {
    const { sent } = setup({ edit: true });
    const input = await combobox();
    await waitFor(() => expect((input as HTMLInputElement).value).toBe("Monstera deliciosa"));
    expect(screen.getByText("Linked to a saved species")).toBeTruthy();
    await userEvent.clear(input);
    await userEvent.type(input, "My mystery plant");
    await userEvent.keyboard("{Escape}");
    expect(screen.getByText("Optional, like Monstera deliciosa")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(sent.plant).toMatchObject({ species: "My mystery plant", species_id: null }));
  });

  it("works from the keyboard: arrows, Enter and Escape", async () => {
    setup();
    const input = await combobox();
    await userEvent.type(input, "mons");
    const first = await screen.findByRole("option", { name: /Monstera deliciosa/ });
    await userEvent.keyboard("{ArrowDown}");
    expect(input.getAttribute("aria-activedescendant")).toBe(first.id);
    expect(first.getAttribute("aria-selected")).toBe("true");
    // Up from the first wraps to the last row, the online search.
    await userEvent.keyboard("{ArrowUp}");
    expect(document.getElementById(input.getAttribute("aria-activedescendant") ?? "")?.textContent).toBe(
      "Search online for “mons”",
    );
    await userEvent.keyboard("{ArrowDown}{Enter}");
    expect((input as HTMLInputElement).value).toBe("Monstera deliciosa");
    expect(screen.queryByRole("listbox")).toBeNull();

    await userEvent.type(input, "x");
    expect(await screen.findByRole("listbox")).toBeTruthy();
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(input.getAttribute("aria-expanded")).toBe("false");
  });
});
