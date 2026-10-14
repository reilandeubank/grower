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

test("locations and plants: add, filter, edit, archive", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Username").fill(E2E_USER.username);
  await page.getByLabel("Password", { exact: true }).fill(E2E_USER.password);
  await page.getByRole("button", { name: "Log in" }).click();
  const nav = page.getByRole("navigation", { name: "Main" });

  // A location, from Settings.
  await nav.getByRole("link", { name: "Settings" }).click();
  await page.getByRole("link", { name: "Locations" }).click();
  await page.getByRole("button", { name: "Add location" }).click();
  const sheet = page.getByRole("dialog", { name: "Add location" });
  await sheet.getByLabel("Name").fill("Kitchen");
  await sheet.getByLabel("Light").selectOption("Bright, indirect");
  await sheet.getByRole("button", { name: "Save" }).click();
  await expect(sheet).toBeHidden();
  await expect(page.getByRole("button", { name: /Kitchen/ })).toContainText("Bright, indirect · No plants");

  // Two plants: one in the kitchen, one without a location.
  const addPlant = async (name: string, location?: string) => {
    await nav.getByRole("link", { name: "Plants" }).click();
    await page.getByRole("link", { name: "Add plant" }).click();
    await page.getByLabel("Name").fill(name);
    if (location) await page.getByLabel("Location").selectOption(location);
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
  };
  await addPlant("Monstera", "Kitchen");
  await addPlant("Fern");

  await nav.getByRole("link", { name: "Plants" }).click();
  const monstera = page.getByRole("link", { name: /^Monstera/ });
  const fern = page.getByRole("link", { name: /^Fern/ });
  await expect(monstera).toBeVisible();
  await expect(fern).toBeVisible();
  await page.getByRole("button", { name: "Kitchen" }).click();
  await expect(fern).toBeHidden();
  await expect(page).toHaveURL(/\?location=/);

  // Edit the species, then archive.
  await monstera.click();
  await page.getByRole("button", { name: "Edit" }).click();
  await page.getByLabel("Species").fill("Monstera deliciosa");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Monstera deliciosa")).toBeVisible();
  await page.getByRole("button", { name: "Edit" }).click();
  await page.getByRole("button", { name: "Archive plant" }).click();
  await expect(page.getByText("Archived", { exact: true })).toBeVisible();

  await nav.getByRole("link", { name: "Plants" }).click();
  await expect(fern).toBeVisible();
  await expect(monstera).toBeHidden();
  await page.getByRole("link", { name: "Archived (1)" }).click();
  await expect(page.getByRole("link", { name: /^Monstera/ })).toBeVisible();
});
