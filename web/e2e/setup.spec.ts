import { expect, test } from "@playwright/test";
import { SETUP_SERVER } from "../playwright.config";

// Runs against the second server, which starts with no account (see playwright.config.ts).
test.use({ baseURL: SETUP_SERVER.baseURL });

test("first run: create the account with the setup code, then log in with it", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/setup$/);
  await expect(
    page.getByRole("heading", { name: "Create your account" }),
  ).toBeVisible();

  // A wrong code is refused and nothing is created.
  await page.getByLabel("Setup code").fill("not-the-code");
  await page.getByLabel("Username").fill("first");
  await page.getByLabel("Password", { exact: true }).fill("first-password-123");
  await page.getByLabel("Confirm password").fill("first-password-123");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "setup code doesn't match",
  );

  // Codes ignore case and dashes.
  await page.getByLabel("Setup code").fill(SETUP_SERVER.code.toUpperCase());
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(
    /^[A-Z][a-z]+day, [A-Z][a-z]{2} \d{1,2}$/,
  );

  // Setup is gone for good: signed out, the app shows the login page, and the account works.
  await page
    .getByRole("navigation", { name: "Main" })
    .getByRole("link", { name: "Settings" })
    .click();
  await page.getByRole("button", { name: "Log out" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await page.goto("/setup");
  await expect(page).toHaveURL(/\/login$/);

  await page.getByLabel("Username").fill("first");
  await page.getByLabel("Password", { exact: true }).fill("first-password-123");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page).toHaveURL(/\/$/);
});
