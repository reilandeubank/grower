import { describe, expect, it } from "vitest";
import type { Photo } from "../api/client";
import { ApiError } from "../api/client";
import { formatPhotoDate, groupByMonth, uploadErrorMessage, withLocalDate } from "./copy";
import { UnsupportedImageError } from "./prepare";

const photo = (id: string, taken_at: string): Photo => ({
  id,
  plant_id: "p1",
  taken_at,
  caption: "",
  width: 100,
  height: 100,
  urls: { small: "", medium: "", original: "" },
});

describe("groupByMonth", () => {
  it("groups newest-first photos by month, keeping their order", () => {
    const photos = [
      photo("a", "2026-10-12T15:00:00Z"),
      photo("b", "2026-10-01T15:00:00Z"),
      photo("c", "2026-09-30T15:00:00Z"),
      photo("d", "2025-09-02T15:00:00Z"),
    ];
    expect(groupByMonth(photos, "America/Chicago").map((m) => [m.label, m.photos.map((p) => p.id)])).toEqual([
      ["October 2026", ["a", "b"]],
      ["September 2026", ["c"]],
      ["September 2025", ["d"]],
    ]);
  });

  it("uses the given zone for the month boundary", () => {
    // 03:00Z on Nov 1 is still Oct 31 in Chicago.
    const photos = [photo("a", "2026-11-01T03:00:00Z")];
    expect(groupByMonth(photos, "America/Chicago")[0]?.label).toBe("October 2026");
    expect(groupByMonth(photos, "Europe/Berlin")[0]?.label).toBe("November 2026");
  });

  it("returns nothing for no photos", () => {
    expect(groupByMonth([])).toEqual([]);
  });
});

describe("photo copy", () => {
  it("formats the date", () => {
    expect(formatPhotoDate("2026-10-12T15:00:00Z", "America/Chicago")).toBe("Oct 12, 2026");
  });

  it("moves a photo to another day, keeping its local time", () => {
    const moved = new Date(withLocalDate("2026-10-12T15:30:00Z", "2026-09-03"));
    const original = new Date("2026-10-12T15:30:00Z");
    expect([moved.getFullYear(), moved.getMonth(), moved.getDate()]).toEqual([2026, 8, 3]);
    expect([moved.getHours(), moved.getMinutes()]).toEqual([original.getHours(), original.getMinutes()]);
  });

  it("explains upload failures", () => {
    expect(uploadErrorMessage(new UnsupportedImageError())).toBe(
      "This photo's format isn't supported. Try a JPEG, or set your camera to save JPEG.",
    );
    const tooBig = new ApiError(413, { title: "Too large", status: 413, detail: "Photos can be up to 15 MB." });
    expect(uploadErrorMessage(tooBig)).toBe("Photos can be up to 15 MB.");
    const rejected = new ApiError(400, { title: "Bad Request", status: 400, detail: "Upload a JPEG or PNG." });
    expect(uploadErrorMessage(rejected)).toBe("Upload a JPEG or PNG.");
  });
});
