import { describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";
import { tokenLine } from "./tokens";

const base = {
  "GET /api/v1/me": () => json(200, me),
  "GET /api/v1/storage": () =>
    json(200, { photo_count: 0, photo_bytes: 0, database_bytes: 1000, last_backup_at: null }),
  "GET /api/v1/plants": () => json(200, { items: [] }),
};

const token = {
  id: "t1",
  label: "Home Assistant",
  scope: "read",
  created_at: "2026-10-12T15:00:00Z",
  last_used_at: null as string | null,
};

describe("tokenLine", () => {
  const now = new Date(2026, 10, 5, 12);
  it("says when a token was made and last used", () => {
    expect(tokenLine({ created_at: new Date(2026, 9, 12, 9).toISOString(), last_used_at: null }, now)).toBe(
      "Created Oct 12 · Never used",
    );
    expect(
      tokenLine({ created_at: new Date(2026, 10, 4, 9).toISOString(), last_used_at: new Date(2026, 10, 5, 8).toISOString() }, now),
    ).toBe("Created yesterday · Last used today");
  });
});

describe("API tokens in Settings", () => {
  it("creates a token and shows its secret once", async () => {
    let tokens: (typeof token)[] = [];
    const created = vi.fn(async (req: Request) => {
      const body = (await req.json()) as { label: string };
      const t = { ...token, label: body.label, created_at: new Date().toISOString() };
      tokens = [t];
      return json(201, { token: t, secret: "grower_s3cret" });
    });
    mockApi({
      ...base,
      "GET /api/v1/api-tokens": () => json(200, { items: tokens }),
      "POST /api/v1/api-tokens": created,
    });
    renderApp("/settings");

    await userEvent.click(await screen.findByRole("button", { name: "Create API token" }));
    const sheet = await screen.findByRole("dialog", { name: "Create API token" });
    const name = within(sheet).getByRole("textbox", { name: /Name/ });
    expect((name as HTMLInputElement).value).toBe("Home Assistant");

    await userEvent.clear(name);
    await userEvent.click(within(sheet).getByRole("button", { name: "Create token" }));
    expect(await within(sheet).findByText(/Name the token/)).toBeTruthy();
    expect(created).not.toHaveBeenCalled();

    await userEvent.type(name, " Kitchen tablet ");
    await userEvent.click(within(sheet).getByRole("button", { name: "Create token" }));
    const secret = (await within(sheet).findByRole("textbox", { name: "Token" })) as HTMLInputElement;
    expect(secret.value).toBe("grower_s3cret");
    expect(secret.readOnly).toBe(true);
    expect(created).toHaveBeenCalledOnce();
    expect((within(sheet).getByRole("textbox", { name: /Address/ }) as HTMLInputElement).value).toMatch(
      /\/api\/v1\/integrations\/home-assistant$/,
    );

    await userEvent.click(within(sheet).getByRole("button", { name: "Done" }));
    await waitFor(() => expect(screen.queryByText("grower_s3cret")).toBeNull());
    expect(await screen.findByRole("button", { name: /Kitchen tablet/ })).toBeTruthy();
    expect(screen.queryByDisplayValue("grower_s3cret")).toBeNull();
  });

  it("revokes a token after asking", async () => {
    let tokens = [token];
    const revoked = vi.fn(() => {
      tokens = [];
      return new Response(null, { status: 204 });
    });
    mockApi({
      ...base,
      "GET /api/v1/api-tokens": () => json(200, { items: tokens }),
      "DELETE /api/v1/api-tokens/t1": revoked,
    });
    renderApp("/settings");

    await userEvent.click(await screen.findByRole("button", { name: /Home Assistant/ }));
    const sheet = await screen.findByRole("dialog", { name: "Home Assistant" });
    expect(within(sheet).getByText(/Never used/)).toBeTruthy();
    await userEvent.click(within(sheet).getByRole("button", { name: "Revoke token" }));
    expect(revoked).not.toHaveBeenCalled();
    await userEvent.click(within(sheet).getByRole("button", { name: "Revoke" }));

    expect(await screen.findByText("Token revoked.")).toBeTruthy();
    expect(revoked).toHaveBeenCalledOnce();
    await waitFor(() => expect(screen.queryByRole("button", { name: /Home Assistant/ })).toBeNull());
  });
});
