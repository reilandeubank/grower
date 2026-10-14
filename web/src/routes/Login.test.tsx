import { describe, expect, it } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { json, me, mockApi, renderApp } from "../test/render";

async function fillAndSubmit(username: string, password: string) {
  await userEvent.type(await screen.findByLabelText("Username"), username);
  await userEvent.type(screen.getByLabelText("Password"), password);
  await userEvent.click(screen.getByRole("button", { name: "Log in" }));
}

describe("Login", () => {
  it("sends signed-out visitors to /login with a next param", async () => {
    mockApi({ "GET /api/v1/me": () => json(401, { title: "Unauthorized", status: 401 }) });
    const { router } = renderApp("/settings");
    await screen.findByRole("button", { name: "Log in" });
    expect(router.state.location.pathname).toBe("/login");
    expect(router.state.location.search).toBe("?next=%2Fsettings");
  });

  it("shows the mismatch message on 401", async () => {
    mockApi({
      "GET /api/v1/me": () => json(401, { title: "Unauthorized", status: 401 }),
      "POST /api/v1/auth/login": () => json(401, { title: "Unauthorized", status: 401 }),
    });
    renderApp("/login");
    await fillAndSubmit("fern", "wrong-password");
    expect((await screen.findByRole("alert")).textContent).toBe("That username and password don't match.");
  });

  it("shows the rate-limit message on 429", async () => {
    mockApi({
      "GET /api/v1/me": () => json(401, { title: "Unauthorized", status: 401 }),
      "POST /api/v1/auth/login": () => json(429, { title: "Too Many Requests", status: 429 }),
    });
    renderApp("/login");
    await fillAndSubmit("fern", "wrong-password");
    expect((await screen.findByRole("alert")).textContent).toBe("Too many attempts. Wait a minute and try again.");
  });

  it("asks for missing fields without calling the API", async () => {
    const fetchMock = mockApi({ "GET /api/v1/me": () => json(401, { title: "Unauthorized", status: 401 }) });
    renderApp("/login");
    await userEvent.click(await screen.findByRole("button", { name: "Log in" }));
    expect(screen.getByText("Enter your username.")).toBeTruthy();
    expect(screen.getByText("Enter your password.")).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledTimes(1); // only the initial /me
  });

  it("logs in and goes on to the next page", async () => {
    let body: unknown;
    mockApi({
      "GET /api/v1/me": () => json(401, { title: "Unauthorized", status: 401 }),
      "POST /api/v1/auth/login": async (req) => {
        body = await req.json();
        return json(200, me);
      },
    });
    const { router } = renderApp("/login?next=%2Fsettings");
    await fillAndSubmit(" fern ", "correct-horse");
    expect(await screen.findByRole("heading", { level: 1, name: "Settings" })).toBeTruthy();
    expect(router.state.location.pathname).toBe("/settings");
    expect(body).toEqual({ username: "fern", password: "correct-horse" });
  });

  it("lands on Today after logging in without a next param", async () => {
    mockApi({
      "GET /api/v1/me": () => json(401, { title: "Unauthorized", status: 401 }),
      "POST /api/v1/auth/login": () => json(200, me),
    });
    const { router } = renderApp("/login");
    await fillAndSubmit("fern", "correct-horse");
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
    expect(await screen.findByText("No care tasks yet")).toBeTruthy();
  });

  it("ignores an off-site next param", async () => {
    mockApi({
      "GET /api/v1/me": () => json(401, { title: "Unauthorized", status: 401 }),
      "POST /api/v1/auth/login": () => json(200, me),
    });
    const { router } = renderApp("/login?next=%2F%2Fevil.example");
    await fillAndSubmit("fern", "correct-horse");
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
  });
});
