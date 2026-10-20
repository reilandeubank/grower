import { expect, test } from "@playwright/test";
import { clientIP, E2E_USER } from "../playwright.config";

test.use(clientIP(6));

// Runs after today.spec.ts (which expects nothing else due today), in the same data dir.
// The e2e server has no provider keys, so this covers the manual path only.
test("a species by hand: save it from the plant form, set its watering, apply it", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Username").fill(E2E_USER.username);
  await page.getByLabel("Password", { exact: true }).fill(E2E_USER.password);
  await page.getByRole("button", { name: "Log in" }).click();
  const nav = page.getByRole("navigation", { name: "Main" });

  // A new plant whose typed species is saved as a new one and linked.
  await nav.getByRole("link", { name: "Plants" }).click();
  await page.getByRole("link", { name: "Add plant" }).click();
  await page.getByLabel("Name").fill("Pothos");
  await page.getByRole("combobox", { name: "Species" }).fill("Epipremnum aureum");
  await page.getByRole("option", { name: "Save as a new species “Epipremnum aureum”" }).click();
  await expect(page.getByText("Linked to a saved species")).toBeVisible();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Pothos" })).toBeVisible();

  // Its care guide starts empty; give it a watering interval.
  const guide = page.getByRole("region", { name: "Care guide" });
  await expect(guide.getByText("Epipremnum aureum")).toBeVisible();
  await expect(guide.getByText("Added by you")).toBeVisible();
  await guide.getByRole("button", { name: "Edit care guide" }).click();
  const sheet = page.getByRole("dialog", { name: "Edit care guide" });
  await sheet.getByLabel("Water every").fill("5");
  await sheet.getByRole("button", { name: "Save" }).click();
  await expect(sheet).toBeHidden();
  await expect(guide.getByText("Water about every 5 days")).toBeVisible();

  // Applying it adds a water task every 5 days on the Care tab.
  await guide.getByRole("button", { name: "Apply to care schedule" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Water task added." })).toBeVisible();
  await expect(page.getByText(/^Every 5 days/)).toBeVisible();
});
