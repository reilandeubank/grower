import { describe, expect, it } from "vitest";
import { formatTodayHeading, greeting } from "./dates";

describe("dates", () => {
  // 2026-10-15T03:30Z is still Oct 14 in Chicago, but Oct 15 in Berlin.
  const instant = new Date("2026-10-15T03:30:00Z");

  it("formats the Today heading in the user's zone", () => {
    expect(formatTodayHeading(instant, "America/Chicago")).toBe("Wednesday, Oct 14");
    expect(formatTodayHeading(instant, "Europe/Berlin")).toBe("Thursday, Oct 15");
  });

  it("falls back to the device zone for an unknown zone", () => {
    expect(() => formatTodayHeading(instant, "Mars/Olympus")).not.toThrow();
  });

  it("greets by the hour in the user's zone", () => {
    expect(greeting(instant, "America/Chicago")).toBe("Good evening");
    expect(greeting(new Date("2026-10-14T14:00:00Z"), "America/Chicago")).toBe("Good morning");
    expect(greeting(new Date("2026-10-14T20:00:00Z"), "America/Chicago")).toBe("Good afternoon");
  });
});
