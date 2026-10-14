import { describe, expect, it } from "vitest";
import { appPath, toNotification } from "./notification";

describe("toNotification", () => {
  it("shows the server's digest", () => {
    const payload = JSON.stringify({
      title: "2 plants need care",
      body: "Water Monstera and Mist Fern",
      url: "/",
      tag: "digest",
    });
    expect(toNotification(payload)).toEqual({
      title: "2 plants need care",
      options: {
        body: "Water Monstera and Mist Fern",
        tag: "digest",
        icon: "/icons/icon-192.png",
        badge: "/icons/badge-96.png",
        data: { url: "/" },
      },
    });
  });

  it("falls back for empty or unreadable payloads", () => {
    for (const payload of [undefined, "", "not json", "null", "[1]", '{"title":3}']) {
      const n = toNotification(payload);
      expect(n.title).toBe("grower");
      expect(n.options.data).toEqual({ url: "/" });
    }
  });

  it("only opens paths on this site", () => {
    expect(appPath("/settings")).toBe("/settings");
    for (const url of ["https://evil.example/", "//evil.example/", "javascript:alert(1)", "", 7, null]) {
      expect(appPath(url)).toBe("/");
    }
  });
});
