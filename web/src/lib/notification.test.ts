import { describe, expect, it, vi } from "vitest";
import {
  appPath,
  LATER_ACTION,
  remindLater,
  toNotification,
} from "./notification";

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

  it("offers to remind later when the server says when", () => {
    const n = toNotification(
      JSON.stringify({
        title: "1 plant needs care",
        body: "Water Monstera",
        url: "/",
        tag: "digest",
        date: "2026-03-01",
        later_time: "17:30",
      }),
    );
    expect(n.options.actions).toEqual([
      { action: LATER_ACTION, title: "Remind me at 5:30 PM" },
    ]);
    expect(n.options.data).toEqual({ url: "/", date: "2026-03-01" });
  });

  it("offers no later button without a readable date and time", () => {
    for (const extra of [
      {},
      { date: "2026-03-01" },
      { later_time: "17:30" },
      { date: "March 1", later_time: "17:30" },
      { date: "2026-03-01", later_time: "5pm" },
    ]) {
      const n = toNotification(
        JSON.stringify({ title: "1 plant needs care", ...extra }),
      );
      expect(n.options.actions).toBeUndefined();
      expect(n.options.data).toEqual({ url: "/" });
    }
  });

  it("falls back for empty or unreadable payloads", () => {
    for (const payload of [
      undefined,
      "",
      "not json",
      "null",
      "[1]",
      '{"title":3}',
    ]) {
      const n = toNotification(payload);
      expect(n.title).toBe("grower");
      expect(n.options.data).toEqual({ url: "/" });
    }
  });

  it("only opens paths on this site", () => {
    expect(appPath("/settings")).toBe("/settings");
    for (const url of [
      "https://evil.example/",
      "//evil.example/",
      "javascript:alert(1)",
      "",
      7,
      null,
    ]) {
      expect(appPath(url)).toBe("/");
    }
  });
});

describe("remindLater", () => {
  const reply = (status: number, body?: unknown) =>
    vi.fn(
      async () =>
        new Response(body === undefined ? null : JSON.stringify(body), {
          status,
        }),
    );

  it("asks the server and shows nothing when it worked", async () => {
    const fetchFn = reply(200, { remind_at: "2026-03-01T23:30:00Z" });
    expect(await remindLater("2026-03-01", fetchFn)).toBeNull();
    expect(fetchFn).toHaveBeenCalledWith("/api/v1/push/remind-later", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ date: "2026-03-01" }),
    });
  });

  it("says why when the server refuses", async () => {
    const n = await remindLater(
      "2026-03-01",
      reply(409, { detail: "Your later time has already passed today." }),
    );
    expect(n?.title).toBe("Couldn't move the reminder");
    expect(n?.options.body).toBe("Your later time has already passed today.");
    expect(n?.options.data).toEqual({ url: "/" });
  });

  it("asks to sign in when the session is gone", async () => {
    expect(
      (
        await remindLater(
          "2026-03-01",
          reply(401, { detail: "Sign in to continue." }),
        )
      )?.options.body,
    ).toBe("Open grower and sign in, then try again.");
  });

  it("explains a server or network failure", async () => {
    expect((await remindLater("2026-03-01", reply(500)))?.options.body).toBe(
      "Something went wrong. Open grower to see what's due.",
    );
    const offline = vi.fn(async () => {
      throw new TypeError("Failed to fetch");
    });
    expect((await remindLater("2026-03-01", offline))?.options.body).toBe(
      "Couldn't reach grower. Check your connection, then open grower to see what's due.",
    );
  });
});
