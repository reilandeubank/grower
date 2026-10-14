/// <reference lib="dom" />
import { readFileSync } from "node:fs";
import { expect, test, type Page } from "@playwright/test";
import { clientIP, E2E_USER } from "../playwright.config";

test.use(clientIP(2));

async function logIn(page: Page, password = E2E_USER.password) {
  await page.goto("/login");
  await page.getByLabel("Username").fill(E2E_USER.username);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page.getByRole("navigation", { name: "Main" })).toBeVisible();
}

async function openSettings(page: Page) {
  await page.getByRole("navigation", { name: "Main" }).getByRole("link", { name: "Settings" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Settings" })).toBeVisible();
}

// One test, so one login and one password round trip.
test("settings: theme, Android back on sheets, export, password", async ({ page }) => {
  await logIn(page);
  await openSettings(page);

  // The theme applies at once and survives a reload.
  const html = page.locator("html");

  await page.getByRole("radio", { name: "Dark" }).check();
  await expect(html).toHaveAttribute("data-theme", "dark");
  await page.reload();
  await expect(html).toHaveAttribute("data-theme", "dark");
  await expect(page.getByRole("radio", { name: "Dark" })).toBeChecked();

  await page.getByRole("radio", { name: "System" }).check();
  await expect(html).not.toHaveAttribute("data-theme", /.*/);

  // Back closes the sheet and stays on Settings.
  await page.getByRole("button", { name: "Change password" }).click();
  const sheet = page.getByRole("dialog", { name: "Change password" });
  await expect(sheet).toBeVisible();
  await page.goBack();
  await expect(sheet).toBeHidden();
  await expect(page).toHaveURL(/\/settings$/);

  // Closing from the UI leaves no extra entry: the next Back goes to Today.
  await page.getByRole("button", { name: /^Time zone/ }).click();
  await page.getByRole("dialog", { name: "Time zone" }).getByRole("button", { name: "Close" }).click();
  await expect(page.getByRole("dialog", { name: "Time zone" })).toBeHidden();
  // The sheet pops its entry a moment after closing; a person can't press Back that fast.
  await expect.poll(() => page.evaluate(() => (window.history.state as { overlay?: number }).overlay)).toBeUndefined();
  await page.goBack();
  await expect(page).toHaveURL(/\/$/);

  // Export downloads a zip.
  await openSettings(page);
  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("link", { name: /^Export data/ }).click(),
  ]);
  expect(download.suggestedFilename()).toMatch(/^grower-export-\d{4}-\d{2}-\d{2}\.zip$/);
  const zip = readFileSync(await download.path());
  expect(zip.length).toBeGreaterThan(0);
  expect(zip.subarray(0, 2).toString()).toBe("PK");

  // Change the shared user's password, then change it back so later specs still log in.
  const newPassword = "e2e-changed-password-456";
  const change = async (from: string, to: string) => {
    await openSettings(page);
    await page.getByRole("button", { name: "Change password" }).click();
    const sheet = page.getByRole("dialog", { name: "Change password" });
    await sheet.getByLabel("Current password", { exact: true }).fill(from);
    await sheet.getByLabel("New password", { exact: true }).fill(to);
    await sheet.getByLabel("Confirm new password", { exact: true }).fill(to);
    await sheet.getByRole("button", { name: "Change password" }).click();
    await expect(page.getByRole("status").filter({ hasText: "Password changed." })).toBeVisible();
    await expect(sheet).toBeHidden();
  };

  await change(E2E_USER.password, newPassword);
  await page.getByRole("button", { name: "Log out" }).click();

  await page.getByLabel("Username").fill(E2E_USER.username);
  await page.getByLabel("Password", { exact: true }).fill(E2E_USER.password);
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page.getByRole("alert")).toHaveText("That username and password don't match.");

  await logIn(page, newPassword);
  await change(newPassword, E2E_USER.password);
});
