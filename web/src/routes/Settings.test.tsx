import { describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { validatePassword } from "../settings/PasswordSheet";
import { json, me, mockApi, renderApp } from "../test/render";

const storage = { photo_count: 48, photo_bytes: 63_000_000, database_bytes: 1_200_000, last_backup_at: null };
const base = {
  "GET /api/v1/me": () => json(200, me),
  "GET /api/v1/storage": () => json(200, storage),
  "GET /api/v1/plants": () => json(200, { items: [] }),
};

function deviceZone(zone: string) {
  const real = Intl.DateTimeFormat.prototype.resolvedOptions;
  vi.spyOn(Intl.DateTimeFormat.prototype, "resolvedOptions").mockImplementation(function (this: Intl.DateTimeFormat) {
    return { ...real.call(this), timeZone: zone };
  });
}

describe("Settings", () => {
  it("shows storage use, the export link and no backup yet", async () => {
    mockApi(base);
    renderApp("/settings");
    expect(await screen.findByText("0 plants · 48 photos (63 MB) · database 1.2 MB")).toBeTruthy();
    expect(screen.getByText("No backup yet")).toBeTruthy();
    const link = screen.getByRole("link", { name: /Export data/ });
    expect(link.getAttribute("href")).toBe("/api/v1/export");
    expect(link.hasAttribute("download")).toBe(true);
  });

  it("changes the time zone by search or from the device", async () => {
    deviceZone("Europe/London");
    let saved: unknown;
    mockApi({
      ...base,
      "PUT /api/v1/settings": async (req) => {
        saved = await req.json();
        return json(200, saved);
      },
    });
    renderApp("/settings");
    await userEvent.click(await screen.findByRole("button", { name: "Time zone America/Chicago" }));
    const sheet = screen.getByRole("dialog", { name: "Time zone" });
    // Role queries over all ~400 zones are slow in jsdom, so narrow the list first.
    const search = within(sheet).getByLabelText("Search");
    await userEvent.click(search);
    await userEvent.paste("chicago");
    expect(within(sheet).getByRole("button", { name: "America/Chicago" }).getAttribute("aria-current")).toBe("true");
    expect(within(sheet).getByRole("button", { name: "Use this device's time zone (Europe/London)" })).toBeTruthy();

    await userEvent.clear(search);
    await userEvent.paste("new york");
    const list = within(sheet).getByRole("list", { name: "Time zones" });
    expect(within(list).getAllByRole("button").map((b) => b.textContent)).toEqual(["America/New York"]);
    await userEvent.click(within(list).getByRole("button", { name: "America/New York" }));

    await waitFor(() => expect(saved).toEqual({ timezone: "America/New_York" }));
    expect(await screen.findByRole("button", { name: "Time zone America/New York" })).toBeTruthy();
    expect(screen.getByText("Time zone set to America/New York.")).toBeTruthy();
  });

  it("offers no device suggestion when the device already matches", async () => {
    deviceZone("America/Chicago");
    mockApi(base);
    renderApp("/settings");
    await userEvent.click(await screen.findByRole("button", { name: /^Time zone/ }));
    expect(screen.queryByRole("button", { name: /Use this device's time zone/ })).toBeNull();
  });

  it("changes the password and shows the server's reason when it refuses", async () => {
    let calls = 0;
    let body: unknown;
    mockApi({
      ...base,
      "POST /api/v1/auth/password": async (req) => {
        body = await req.json();
        return ++calls === 1
          ? json(400, { title: "Bad Request", status: 400, detail: "Your current password is wrong." })
          : new Response(null, { status: 204 });
      },
    });
    renderApp("/settings");
    await userEvent.click(await screen.findByRole("button", { name: "Change password" }));
    const sheet = screen.getByRole("dialog", { name: "Change password" });
    await userEvent.type(within(sheet).getByLabelText("Current password"), "old-password-1");
    await userEvent.type(within(sheet).getByLabelText("New password"), "new-password-22");
    await userEvent.type(within(sheet).getByLabelText("Confirm new password"), "new-password-22");
    await userEvent.click(within(sheet).getByRole("button", { name: "Change password" }));
    expect((await within(sheet).findByRole("alert")).textContent).toBe("Your current password is wrong.");

    await userEvent.click(within(sheet).getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("Password changed. Other devices were signed out.")).toBeTruthy();
    expect(screen.queryByRole("dialog", { name: "Change password" })).toBeNull();
    expect(body).toEqual({ current_password: "old-password-1", new_password: "new-password-22" });
  });
});

describe("validatePassword", () => {
  it("needs the current password, 10 characters, and a matching confirmation", () => {
    expect(validatePassword({ current: "", next: "short", confirm: "" })).toEqual({
      current: "Enter your current password.",
      next: "Use at least 10 characters.",
    });
    expect(validatePassword({ current: "x", next: "long-enough-1", confirm: "long-enough-2" })).toEqual({
      confirm: "The passwords don't match.",
    });
    expect(validatePassword({ current: "x", next: "long-enough-1", confirm: "long-enough-1" })).toEqual({});
  });
});
