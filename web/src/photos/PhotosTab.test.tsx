import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";
import { preparePhoto, UnsupportedImageError } from "./prepare";

// jsdom can't decode images; the resize itself is covered by prepare.test.ts and e2e.
vi.mock("./prepare", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./prepare")>()),
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
  created_at: "2026-10-01T12:00:00Z",
};
const photo = {
  id: "ph1",
  plant_id: "p1",
  taken_at: "2026-10-12T15:00:00Z",
  caption: "New leaf",
  width: 2560,
  height: 1920,
  urls: { small: "/media/photos/ph1/small", medium: "/media/photos/ph1/medium", original: "/media/photos/ph1/original" },
};

const baseApi = {
  "GET /api/v1/me": () => json(200, me),
  "GET /api/v1/locations": () => json(200, { items: [] }),
  "GET /api/v1/plants/p1/tasks": () => json(200, { items: [] }),
  "GET /api/v1/plants/p1/events": () => json(200, { items: [] }),
  "GET /api/v1/today": () => json(200, { date: "2026-10-14", task_count: 0, overdue: [], due: [], upcoming: [] }),
};

async function openPhotosTab() {
  renderApp("/plants/p1");
  await userEvent.click(await screen.findByRole("tab", { name: "Photos" }));
}

describe("Photos tab", () => {
  beforeEach(() => {
    vi.mocked(preparePhoto).mockResolvedValue({
      blob: new Blob(["jpeg"], { type: "image/jpeg" }),
      takenAt: "2026-10-12T15:00:00.000Z",
    });
  });
  afterEach(() => vi.restoreAllMocks());

  it("uploads a photo, shows it while it uploads, then in the timeline", async () => {
    let photos: unknown[] = [];
    let finishUpload: (r: Response) => void = () => {};
    let upload: Request | undefined;
    // Node can't read back a body built from jsdom's FormData, so watch what goes into it.
    const append = vi.spyOn(FormData.prototype, "append");
    mockApi({
      ...baseApi,
      "GET /api/v1/plants/p1": () => json(200, photos.length ? { ...plant, cover: photo } : plant),
      "GET /api/v1/plants/p1/photos": () => json(200, { items: photos }),
      "POST /api/v1/plants/p1/photos": (req) => {
        upload = req;
        return new Promise<Response>((resolve) => (finishUpload = resolve));
      },
    });
    await openPhotosTab();
    expect(await screen.findByText("No photos yet")).toBeTruthy();

    const file = new File(["raw"], "IMG_0001.jpg", { type: "image/jpeg" });
    await userEvent.upload(screen.getByLabelText("Add photo"), file);
    expect(await screen.findByText("Uploading photo")).toBeTruthy();
    expect(preparePhoto).toHaveBeenCalledWith(file);
    await waitFor(() => expect(upload).toBeDefined());
    // The browser picks the multipart boundary; the client must not force JSON.
    expect(upload?.headers.get("Content-Type")).toMatch(/^multipart\/form-data; boundary=/);
    expect(append).toHaveBeenCalledWith("file", expect.any(Blob), "photo.jpg");
    expect(append).toHaveBeenCalledWith("taken_at", "2026-10-12T15:00:00.000Z");

    photos = [photo];
    finishUpload(json(201, photo));
    expect(await screen.findByText("Photo added.")).toBeTruthy();
    expect(await screen.findByRole("heading", { name: "October 2026" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Photo from Oct 12, 2026" })).toBeTruthy();
    expect(screen.queryByText("Uploading photo")).toBeNull();
    // The plant now has its cover in the hero.
    await waitFor(() => expect(screen.getByRole("img", { name: "Monstera" }).getAttribute("src")).toBe(photo.urls.medium));
  });

  it("explains a format the browser can't read", async () => {
    vi.mocked(preparePhoto).mockRejectedValue(new UnsupportedImageError());
    mockApi({
      ...baseApi,
      "GET /api/v1/plants/p1": () => json(200, plant),
      "GET /api/v1/plants/p1/photos": () => json(200, { items: [] }),
    });
    await openPhotosTab();
    await userEvent.upload(await screen.findByLabelText("Add photo"), new File(["heic"], "IMG.HEIC", { type: "image/heic" }));
    expect(
      await screen.findByText("This photo's format isn't supported. Try a JPEG, or set your camera to save JPEG."),
    ).toBeTruthy();
    expect(screen.getByText("No photos yet")).toBeTruthy();
  });

  it("opens a photo, sets it as the cover, then deletes it", async () => {
    let photos: unknown[] = [photo];
    let cover: unknown = null;
    mockApi({
      ...baseApi,
      "GET /api/v1/plants/p1": () => json(200, { ...plant, cover }),
      "GET /api/v1/plants/p1/photos": () => json(200, { items: photos }),
      "PUT /api/v1/plants/p1/cover": async (req) => {
        expect(await req.json()).toEqual({ photo_id: "ph1" });
        cover = photo;
        return json(200, { ...plant, cover });
      },
      "DELETE /api/v1/photos/ph1": () => {
        photos = [];
        cover = null;
        return new Response(null, { status: 204 });
      },
    });
    await openPhotosTab();
    await userEvent.click(await screen.findByRole("button", { name: "Photo from Oct 12, 2026" }));
    const viewer = screen.getByRole("dialog", { name: "Photo" });
    expect(within(viewer).getByText("New leaf")).toBeTruthy();
    expect(within(viewer).getByText("Oct 12, 2026")).toBeTruthy();

    await userEvent.click(within(viewer).getByRole("button", { name: "Set as cover" }));
    const isCover = await within(viewer).findByRole("button", { name: "Cover photo" });
    expect((isCover as HTMLButtonElement).disabled).toBe(true);

    await userEvent.click(within(viewer).getByRole("button", { name: "Delete photo" }));
    await userEvent.click(within(viewer).getByRole("button", { name: "Delete" }));
    expect(await screen.findByText("Photo deleted.")).toBeTruthy();
    expect(await screen.findByText("No photos yet")).toBeTruthy();
  });
});
