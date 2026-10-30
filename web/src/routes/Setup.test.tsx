import { describe, expect, it } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";
import { validateSetup } from "./Setup";

const signedOut = () => json(401, { title: "Unauthorized", status: 401 });
const needed = (yes: boolean) => () => json(200, { needed: yes });

async function fillAndSubmit(values: {
  code?: string;
  username?: string;
  password?: string;
  confirm?: string;
}) {
  const field = async (label: string, value: string | undefined) => {
    if (value)
      await userEvent.type(
        await screen.findByLabelText(label, { exact: true }),
        value,
      );
  };
  await field("Setup code", values.code);
  await field("Username", values.username);
  await field("Password", values.password);
  await field("Confirm password", values.confirm);
  await userEvent.click(screen.getByRole("button", { name: "Create account" }));
}

describe("validateSetup", () => {
  it.each([
    [
      {
        code: "",
        username: "fern",
        password: "correct-horse",
        confirm: "correct-horse",
      },
      { code: "Enter the setup code from the server log." },
    ],
    [
      {
        code: "K7Q2M",
        username: "two words",
        password: "correct-horse",
        confirm: "correct-horse",
      },
      { username: "Use a username with no spaces." },
    ],
    [
      { code: "K7Q2M", username: "fern", password: "short", confirm: "short" },
      { password: "Use at least 10 characters." },
    ],
    [
      {
        code: "K7Q2M",
        username: "fern",
        password: "correct-horse",
        confirm: "correct-hors",
      },
      { confirm: "The passwords don't match." },
    ],
    [
      {
        code: "K7Q2M",
        username: " fern ",
        password: "correct-horse",
        confirm: "correct-horse",
      },
      {},
    ],
  ])("%o", (values, want) => {
    expect(validateSetup(values)).toEqual(want);
  });
});

describe("Setup", () => {
  it("sends a fresh install to /setup instead of /login", async () => {
    mockApi({ "GET /api/v1/me": signedOut, "GET /api/v1/setup": needed(true) });
    const { router } = renderApp("/plants");
    await screen.findByRole("heading", { name: "Create your account" });
    expect(router.state.location.pathname).toBe("/setup");
  });

  it("sends /login to /setup while setup is needed", async () => {
    mockApi({ "GET /api/v1/me": signedOut, "GET /api/v1/setup": needed(true) });
    const { router } = renderApp("/login");
    await screen.findByRole("heading", { name: "Create your account" });
    expect(router.state.location.pathname).toBe("/setup");
  });

  it("sends /setup to /login once an account exists", async () => {
    mockApi({
      "GET /api/v1/me": signedOut,
      "GET /api/v1/setup": needed(false),
    });
    const { router } = renderApp("/setup");
    await screen.findByRole("button", { name: "Log in" });
    expect(router.state.location.pathname).toBe("/login");
  });

  it("checks the form without calling the API", async () => {
    const fetchMock = mockApi({
      "GET /api/v1/me": signedOut,
      "GET /api/v1/setup": needed(true),
    });
    renderApp("/setup");
    await fillAndSubmit({
      username: "fern",
      password: "correct-horse",
      confirm: "correct-horse",
    });
    expect(
      screen.getByText("Enter the setup code from the server log."),
    ).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledTimes(2); // only /me and /setup
  });

  it("creates the account and lands on Today", async () => {
    let body: Record<string, unknown> = {};
    mockApi({
      "GET /api/v1/me": signedOut,
      "GET /api/v1/setup": needed(true),
      "POST /api/v1/setup": async (req) => {
        body = (await req.json()) as Record<string, unknown>;
        return json(200, me);
      },
      "GET /api/v1/today": () =>
        json(200, { date: "2026-10-14", overdue: [], due: [], upcoming: [] }),
      "GET /api/v1/plants": () => json(200, { items: [] }),
    });
    const { router } = renderApp("/setup");
    await fillAndSubmit({
      code: " k7q2m-x9pb4 ",
      username: " fern ",
      password: "correct-horse",
      confirm: "correct-horse",
    });
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
    expect(await screen.findByText("No care tasks yet")).toBeTruthy();
    expect(body).toMatchObject({
      code: "k7q2m-x9pb4",
      username: "fern",
      password: "correct-horse",
    });
    expect(typeof body.timezone).toBe("string");
  });

  it("shows the server's message for a wrong code", async () => {
    const detail =
      "That setup code doesn't match. Check the server log for the current one.";
    mockApi({
      "GET /api/v1/me": signedOut,
      "GET /api/v1/setup": needed(true),
      "POST /api/v1/setup": () =>
        json(403, { title: "Forbidden", status: 403, detail }),
    });
    renderApp("/setup");
    await fillAndSubmit({
      code: "WRONG",
      username: "fern",
      password: "correct-horse",
      confirm: "correct-horse",
    });
    expect((await screen.findByRole("alert")).textContent).toBe(detail);
  });

  it("offers the login page when someone else finished setup first", async () => {
    mockApi({
      "GET /api/v1/me": signedOut,
      "GET /api/v1/setup": needed(true),
      "POST /api/v1/setup": () =>
        json(409, {
          title: "Conflict",
          status: 409,
          detail: "This grower already has an account. Log in instead.",
        }),
    });
    const { router } = renderApp("/setup");
    await fillAndSubmit({
      code: "K7Q2M",
      username: "fern",
      password: "correct-horse",
      confirm: "correct-horse",
    });
    await userEvent.click(
      await screen.findByRole("link", { name: "Go to log in" }),
    );
    await screen.findByRole("button", { name: "Log in" });
    expect(router.state.location.pathname).toBe("/login");
  });
});
