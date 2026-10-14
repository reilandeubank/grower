import { afterEach, describe, expect, it, vi } from "vitest";
import { applyTheme, readTheme, saveTheme, THEME_KEY } from "./theme";

function addMeta(media: string, content: string) {
  const meta = document.createElement("meta");
  meta.name = "theme-color";
  meta.media = media;
  meta.content = content;
  document.head.append(meta);
  return meta;
}

afterEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset.theme;
  document.documentElement.style.removeProperty("--surface");
  document.head.querySelectorAll('meta[name="theme-color"]').forEach((m) => m.remove());
});

describe("theme", () => {
  it("follows the system until a theme is saved", () => {
    expect(readTheme()).toBe("system");
    saveTheme("dark");
    expect(localStorage.getItem(THEME_KEY)).toBe("dark");
    expect(readTheme()).toBe("dark");
    expect(document.documentElement.dataset.theme).toBe("dark");

    saveTheme("system");
    expect(localStorage.getItem(THEME_KEY)).toBeNull();
    expect(document.documentElement.dataset.theme).toBeUndefined();
  });

  it("ignores junk in storage", () => {
    localStorage.setItem(THEME_KEY, "purple");
    expect(readTheme()).toBe("system");
  });

  it("still applies the theme when storage is blocked", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("blocked", "SecurityError");
    });
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("blocked", "SecurityError");
    });
    expect(() => saveTheme("light")).not.toThrow();
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(readTheme()).toBe("system");
  });

  it("points theme-color at the page, and restores the system colours", () => {
    const light = addMeta("(prefers-color-scheme: light)", "#f4f6f1");
    const dark = addMeta("(prefers-color-scheme: dark)", "#121713");
    // jsdom doesn't apply tokens.css; stand in for the dark surface.
    document.documentElement.style.setProperty("--surface", "#121713");
    applyTheme("dark");
    expect([light.content, dark.content]).toEqual(["#121713", "#121713"]);
    applyTheme("system");
    expect([light.content, dark.content]).toEqual(["#f4f6f1", "#121713"]);
  });
});
