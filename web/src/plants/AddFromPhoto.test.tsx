import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";
import { preparePhoto } from "../photos/prepare";

// jsdom can't decode images; the resize itself is covered by prepare.test.ts and e2e.
vi.mock("../photos/prepare", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../photos/prepare")>()),
  preparePhoto: vi.fn(),
}));

const plant = {
  id: "p1",
  name: "Swiss cheese plant",
  species: "",
  species_id: null,
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

const monstera = { scientific_name: "Monstera deliciosa", common_name: "swiss cheese plant", score: 0.82, saved_id: "s1" };
const philodendron = { scientific_name: "Philodendron hederaceum", common_name: "Heartleaf philodendron", score: 0.41, saved_id: null };
const ficus = { scientific_name: "Ficus lyrata", common_name: "", score: 0.09, saved_id: null };
const suggestions = { configured: true, detail: "", candidates: [monstera, philodendron, ficus] };
const PRIVACY = "The photo is sent to Pl@ntNet to identify it; grower doesn't keep a copy there.";

type Handlers = Parameters<typeof mockApi>[0];

/** Mocks the API and records each write, in order, with its JSON body (multipart bodies as null). */
function setup(identify: (organ: string) => unknown, extra: Handlers = {}) {
  // Node can't read back a body built from jsdom's FormData, so watch what goes into it.
  const append = vi.spyOn(FormData.prototype, "append");
  const organs: string[] = [];
  const writes: { call: string; body: unknown }[] = [];
  const record = (call: string, response: () => Response) => async (req: Request) => {
    const multipart = req.headers.get("Content-Type")?.startsWith("multipart/");
    writes.push({ call, body: multipart ? null : await req.json() });
    return response();
  };
  mockApi({
    "GET /api/v1/me": () => json(200, me),
    "GET /api/v1/locations": () => json(200, { items: [] }),
    "GET /api/v1/plants": () => json(200, { items: [plant] }),
    "GET /api/v1/plants/p1": () => json(200, plant),
    "GET /api/v1/plants/p1/tasks": () => json(200, { items: [] }),
    "GET /api/v1/plants/p1/events": () => json(200, { items: [] }),
    "GET /api/v1/species": () => json(200, { items: [] }),
    "GET /api/v1/today": () => json(200, { date: "2026-10-14", task_count: 0, overdue: [], due: [], upcoming: [] }),
    "POST /api/v1/identify": () => {
      const organ = append.mock.calls.filter(([key]) => key === "organ").at(-1)?.[1];
      organs.push(String(organ));
      return json(200, identify(String(organ)));
    },
    "POST /api/v1/species": record("POST /species", () =>
      json(201, { id: "s9", scientific_name: philodendron.scientific_name, common_name: philodendron.common_name }),
    ),
    "POST /api/v1/plants": record("POST /plants", () => json(201, plant)),
    "POST /api/v1/plants/p1/photos": record("POST /photos", () => json(201, { id: "ph1" })),
    ...extra,
  });
  return { append, organs, writes };
}

async function addFromPhoto() {
  const app = renderApp("/plants");
  await userEvent.click(await screen.findByRole("button", { name: "Add plant" }));
  await userEvent.upload(screen.getByLabelText(/Add from photo/), new File(["raw"], "IMG_0001.jpg", { type: "image/jpeg" }));
  return app;
}

async function save(router: ReturnType<typeof renderApp>["router"]) {
  await userEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(router.state.location.pathname).toBe("/plants/p1"));
}

describe("Add from photo", () => {
  beforeEach(() => {
    vi.mocked(preparePhoto).mockResolvedValue({
      blob: new Blob(["jpeg"], { type: "image/jpeg" }),
      takenAt: "2026-10-12T15:00:00.000Z",
    });
    URL.createObjectURL = vi.fn(() => "blob:photo");
    URL.revokeObjectURL = vi.fn();
  });
  afterEach(() => vi.restoreAllMocks());

  it("skips identification when it isn't set up, then saves the plant and uploads the photo", async () => {
    const { append, organs, writes } = setup(() => ({ configured: false, detail: "", candidates: [] }));
    const { router } = await addFromPhoto();

    expect(await screen.findByRole("figure", { name: "Cover photo" })).toBeTruthy();
    expect(organs).toEqual(["auto"]);
    expect(append).toHaveBeenCalledWith("file", expect.any(Blob), "photo.jpg");
    expect(screen.queryByText(PRIVACY)).toBeNull();
    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("");

    await userEvent.type(screen.getByLabelText("Name"), "Monstera");
    await save(router);
    expect(writes.map((w) => w.call)).toEqual(["POST /plants", "POST /photos"]);
    expect(writes[0]?.body).toMatchObject({ name: "Monstera", species: "", species_id: null });
    expect(append).toHaveBeenCalledWith("taken_at", "2026-10-12T15:00:00.000Z");
  });

  it("lists suggestions in words, and asks again for another organ", async () => {
    const { organs } = setup((organ) =>
      organ === "flower" ? { configured: true, detail: "No match for this photo.", candidates: [] } : suggestions,
    );
    await addFromPhoto();

    expect(await screen.findByText("Likely · 82%")).toBeTruthy();
    expect(screen.getByText("Possible · 41%")).toBeTruthy();
    expect(screen.getByText("Unlikely · 9%")).toBeTruthy();
    expect(screen.getAllByText("Saved")).toHaveLength(1);
    // A candidate without a common name goes by its scientific name.
    expect(screen.getByRole("button", { name: /^Ficus lyrata/ })).toBeTruthy();
    expect(screen.getByText(PRIVACY)).toBeTruthy();

    await userEvent.click(screen.getByRole("button", { name: "Flower" }));
    expect(await screen.findByText("No match for this photo.")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Continue without a species" })).toBeTruthy();

    // Back to "Not sure" reuses its answer rather than asking again.
    await userEvent.click(screen.getByRole("button", { name: "Not sure" }));
    expect(await screen.findByText("Likely · 82%")).toBeTruthy();
    expect(organs).toEqual(["auto", "flower"]);
  });

  it("prefills a saved species and links it", async () => {
    const { writes } = setup(() => suggestions);
    const { router } = await addFromPhoto();
    await userEvent.click(await screen.findByRole("button", { name: /^swiss cheese plant/ }));

    expect((await screen.findByLabelText<HTMLInputElement>("Name")).value).toBe("Swiss cheese plant");
    expect(screen.getByRole<HTMLInputElement>("combobox", { name: "Species" }).value).toBe("Monstera deliciosa");
    expect(screen.getByText("Linked to a saved species")).toBeTruthy();
    expect(screen.getByRole("figure", { name: "Cover photo" })).toBeTruthy();

    await save(router);
    expect(writes.map((w) => w.call)).toEqual(["POST /plants", "POST /photos"]);
    expect(writes[0]?.body).toMatchObject({ species: "Monstera deliciosa", species_id: "s1" });
  });

  it("creates the species on save when the choice isn't saved yet", async () => {
    const { writes } = setup(() => suggestions);
    const { router } = await addFromPhoto();
    await userEvent.click(await screen.findByRole("button", { name: /^Heartleaf philodendron/ }));

    await save(router);
    expect(writes.map((w) => w.call)).toEqual(["POST /species", "POST /plants", "POST /photos"]);
    expect(writes[0]?.body).toEqual({
      scientific_name: "Philodendron hederaceum",
      common_name: "Heartleaf philodendron",
      notes: "",
    });
    expect(writes[1]?.body).toMatchObject({ name: "Heartleaf philodendron", species_id: "s9" });
  });

  it("goes on without a species, and without the photo once it's removed", async () => {
    const { writes } = setup(() => suggestions);
    const { router } = await addFromPhoto();
    await userEvent.click(await screen.findByRole("button", { name: "None of these" }));

    expect((await screen.findByLabelText<HTMLInputElement>("Name")).value).toBe("");
    expect(screen.getByRole<HTMLInputElement>("combobox", { name: "Species" }).value).toBe("");
    await userEvent.click(screen.getByRole("button", { name: "Remove photo" }));
    expect(screen.queryByRole("figure", { name: "Cover photo" })).toBeNull();

    await userEvent.type(screen.getByLabelText("Name"), "Fern");
    await save(router);
    expect(writes.map((w) => w.call)).toEqual(["POST /plants"]);
  });

  it("opens the plant even when its photo fails, and says so", async () => {
    setup(
      () => ({ configured: false, detail: "", candidates: [] }),
      { "POST /api/v1/plants/p1/photos": () => json(500, { title: "Internal Server Error", status: 500 }) },
    );
    const { router } = await addFromPhoto();
    await userEvent.type(await screen.findByLabelText("Name"), "Monstera");
    await save(router);
    expect(await screen.findByText("Added Swiss cheese plant, but the photo didn't upload. Add it from Photos.")).toBeTruthy();
  });
});
