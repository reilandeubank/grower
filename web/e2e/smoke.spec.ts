import { expect, test } from "@playwright/test";
import { E2E_USER } from "../playwright.config";

test("log in, stay signed in across a reload, log out", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/);

  await page.getByLabel("Username").fill(E2E_USER.username);
  await page.getByLabel("Password", { exact: true }).fill(E2E_USER.password);
  await page.getByRole("button", { name: "Log in" }).click();

  await expect(page).toHaveURL(/\/$/);
  const heading = page.getByRole("heading", { level: 1 });
  // e.g. "Wednesday, Oct 14"
  await expect(heading).toHaveText(/^[A-Z][a-z]+day, [A-Z][a-z]{2} \d{1,2}$/);
  await expect(page.getByText("No care tasks yet")).toBeVisible();

  await page.reload();
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page).toHaveURL(/\/$/);

  await page.getByRole("navigation", { name: "Main" }).getByRole("link", { name: "Settings" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Settings" })).toBeVisible();
  await expect(page.getByText(E2E_USER.username, { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "Log out" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole("button", { name: "Log in" })).toBeVisible();

  // The session is really gone: the app sends us back to log in.
  await page.goto("/settings");
  await expect(page).toHaveURL(/\/login\?next=%2Fsettings$/);
});

test("shows the mismatch message for a wrong password", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Username").fill(E2E_USER.username);
  await page.getByLabel("Password", { exact: true }).fill("not-the-password");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page.getByRole("alert")).toHaveText("That username and password don't match.");
});
