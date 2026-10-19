import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";

const base = {
  "GET /api/v1/me": () => json(200, me),
  "GET /api/v1/storage": () =>
    json(200, { photo_count: 0, photo_bytes: 0, database_bytes: 1000, last_backup_at: null }),
  "GET /api/v1/plants": () => json(200, { items: [] }),
};

const endpoint = "https://push.example/device-1";

/** A browser with push: the permission it would grant, and a device subscription we can inspect. */
function stubPush({
  permission = "default" as NotificationPermission,
  grant = "granted" as NotificationPermission,
  subscribed = false,
} = {}) {
  let subscription: PushSubscription | null = null;
  const unsubscribe = vi.fn(async () => {
    subscription = null;
    return true;
  });
  const make = () =>
    ({
      endpoint,
      unsubscribe,
      toJSON: () => ({ endpoint, keys: { p256dh: "p256dh-key", auth: "auth-secret" } }),
    }) as unknown as PushSubscription;
  if (subscribed) subscription = make();
  const subscribe = vi.fn(async () => (subscription = make()));
  const registration = { pushManager: { getSubscription: async () => subscription, subscribe } };
  const Notification = { permission, requestPermission: vi.fn(async () => (Notification.permission = grant)) };
  vi.stubGlobal("Notification", Notification);
  vi.stubGlobal("PushManager", class {});
  Object.defineProperty(navigator, "serviceWorker", {
    configurable: true,
    value: { getRegistration: async () => registration },
  });
  return { subscribe, unsubscribe, Notification };
}

afterEach(() => {
  Reflect.deleteProperty(navigator, "serviceWorker");
});

const reminder = () => screen.findByRole("switch", { name: /Daily reminder/ });

describe("Notifications settings", () => {
  it("says so when the browser can't show notifications", async () => {
    mockApi(base);
    renderApp("/settings");
    expect(await screen.findByText("This browser can't show notifications.")).toBeTruthy();
    expect(((await reminder()) as HTMLInputElement).disabled).toBe(true);
  });

  it("explains how to unblock notifications", async () => {
    stubPush({ permission: "denied" });
    mockApi(base);
    renderApp("/settings");
    expect(
      await screen.findByText("Notifications are blocked for grower. Allow them in your browser's site settings."),
    ).toBeTruthy();
    expect(((await reminder()) as HTMLInputElement).disabled).toBe(true);
  });

  it("turns the reminder on, sends a test, and turns it off", async () => {
    const push = stubPush();
    const calls: string[] = [];
    let saved: unknown;
    mockApi({
      ...base,
      "GET /api/v1/push/vapid-key": () =>
        json(200, {
          public_key: "BEl62iUYgUivxIkv69yViEuiBIa-Ib9-SkvMeAtA3LFgDzkrxZJjSgSnfckjBJuBkr3qBUYIHBQFLXYp5Nksh8U",
        }),
      "POST /api/v1/push/subscriptions": async (req) => {
        saved = await req.json();
        return new Response(null, { status: 204 });
      },
      "DELETE /api/v1/push/subscriptions": async (req) => {
        calls.push(`delete ${(await req.json()).endpoint}`);
        return new Response(null, { status: 204 });
      },
      "POST /api/v1/push/test": () => {
        calls.push("test");
        return new Response(null, { status: 204 });
      },
    });
    renderApp("/settings");
    const toggle = (await reminder()) as HTMLInputElement;
    await waitFor(() => expect(toggle.disabled).toBe(false));
    expect(toggle.checked).toBe(false);
    expect(screen.queryByRole("button", { name: "Send test notification" })).toBeNull();

    await userEvent.click(toggle);
    await waitFor(() => expect(toggle.checked).toBe(true));
    expect(push.Notification.requestPermission).toHaveBeenCalled();
    expect(push.subscribe).toHaveBeenCalledWith(expect.objectContaining({ userVisibleOnly: true }));
    expect(saved).toEqual({ endpoint, keys: { p256dh: "p256dh-key", auth: "auth-secret" } });

    await userEvent.click(screen.getByRole("button", { name: "Send test notification" }));
    expect(await screen.findByText("Test notification sent.")).toBeTruthy();

    await userEvent.click(toggle);
    await waitFor(() => expect(toggle.checked).toBe(false));
    expect(calls).toEqual(["test", `delete ${endpoint}`]);
    expect(push.unsubscribe).toHaveBeenCalled();
  });

  it("stays off when permission is refused", async () => {
    const push = stubPush({ grant: "denied" });
    mockApi(base);
    renderApp("/settings");
    const toggle = (await reminder()) as HTMLInputElement;
    await waitFor(() => expect(toggle.disabled).toBe(false));
    await userEvent.click(toggle);
    expect(
      await screen.findByText("Notifications are blocked for grower. Allow them in your browser's site settings."),
    ).toBeTruthy();
    expect(toggle.checked).toBe(false);
    expect(push.subscribe).not.toHaveBeenCalled();
  });

  it("undoes the device subscription when the server refuses it", async () => {
    const push = stubPush();
    mockApi({
      ...base,
      "GET /api/v1/push/vapid-key": () => json(200, { public_key: "AAAA" }),
      "POST /api/v1/push/subscriptions": () =>
        json(400, { title: "Bad Request", status: 400, detail: "Endpoint must be an https URL." }),
    });
    renderApp("/settings");
    const toggle = (await reminder()) as HTMLInputElement;
    await waitFor(() => expect(toggle.disabled).toBe(false));
    await userEvent.click(toggle);
    expect(
      await screen.findByText("Couldn't turn on the reminder. Check your connection, then try again."),
    ).toBeTruthy();
    expect(push.unsubscribe).toHaveBeenCalled();
    expect(toggle.checked).toBe(false);
  });

  it("shows an existing subscription as on and saves the reminder time", async () => {
    stubPush({ permission: "granted", subscribed: true });
    let saved: unknown;
    mockApi({
      ...base,
      "PUT /api/v1/settings": async (req) => {
        saved = await req.json();
        return json(200, saved);
      },
    });
    renderApp("/settings");
    await waitFor(async () => expect(((await reminder()) as HTMLInputElement).checked).toBe(true));
    expect(screen.getByRole("button", { name: "Send test notification" })).toBeTruthy();

    const time = screen.getByLabelText("Reminder time") as HTMLInputElement;
    expect(time.value).toBe("08:00");
    // A native time input changes in one step, so set it the way the picker does.
    await userEvent.clear(time);
    await userEvent.type(time, "07:30");
    await waitFor(() => expect(saved).toEqual({ timezone: "America/Chicago", digest_time: "07:30", hemisphere: "north" }));
  });
});
