import { expect, test } from "@playwright/test";
import { clientIP, E2E_USER } from "../playwright.config";

test.use(clientIP(8));

// Runs after today.spec.ts (which expects nothing else due today), in the same data dir.
test("journal: a monthly check-in logs a size, then the Growth tab charts it", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Username").fill(E2E_USER.username);
  await page.getByLabel("Password", { exact: true }).fill(E2E_USER.password);
  await page.getByRole("button", { name: "Log in" }).click();
  const nav = page.getByRole("navigation", { name: "Main" });

  // A plant with a check-in every 30 days, starting today.
  await nav.getByRole("link", { name: "Plants" }).click();
  await page.getByRole("button", { name: "Add plant" }).click();
  await page.getByRole("link", { name: "Enter details" }).click();
  await page.getByLabel("Name").fill("Calathea");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Calathea" })).toBeVisible();
  await page.getByRole("button", { name: "Add care task" }).click();
  const sheet = page.getByRole("dialog", { name: "Add care task" });
  await sheet.getByRole("button", { name: "Check in" }).click();
  await expect(sheet.getByLabel("Days between")).toHaveValue("30");
  await expect(sheet.getByRole("switch", { name: /Adjust for seasons/ })).toBeHidden();
  await sheet.getByRole("button", { name: "Save" }).click();
  await expect(sheet).toBeHidden();
  await expect(page.getByText(/^Every 30 days · Due today/)).toBeVisible();

  // On Today as "Check in"; the round button opens the check-in.
  await nav.getByRole("link", { name: "Today" }).click();
  const dueToday = page.locator("section", { has: page.getByRole("heading", { name: /^Due today/ }) });
  await expect(dueToday.getByRole("button", { name: /^Calathea.*Check in.*Due today/ })).toBeVisible();
  await dueToday.getByRole("button", { name: "Check in on Calathea" }).click();
  const checkIn = page.getByRole("dialog", { name: "Check in · Calathea" });
  await checkIn.getByLabel("Height").fill("12");
  await checkIn.getByRole("radio", { name: "Young" }).check();
  await checkIn.getByRole("button", { name: "Save check-in" }).click();
  await expect(checkIn).toBeHidden();
  await expect(page.getByRole("status").filter({ hasText: "Checked in on Calathea." })).toBeVisible();
  await expect(dueToday.getByRole("button", { name: "Check in on Calathea" })).toBeHidden();

  // The plant shows its size in the meta line and on the Growth tab.
  await nav.getByRole("link", { name: "Plants" }).click();
  await page.getByRole("link", { name: /^Calathea/ }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Calathea" })).toBeVisible();
  await expect(page.getByText("12 in tall · Young", { exact: true })).toBeVisible();
  await page.getByRole("tab", { name: "Growth" }).click();
  const panel = page.getByRole("tabpanel");
  await expect(panel.getByRole("button", { name: /12 in tall · Young/ })).toBeVisible();
  await expect(panel.getByRole("img", { name: /^Height/ })).toBeHidden();

  // A second height makes a chart.
  await panel.getByRole("button", { name: "Add measurement" }).click();
  const add = page.getByRole("dialog", { name: "Add measurement" });
  await add.getByLabel("Height").fill("15");
  await add.getByRole("button", { name: "Save" }).click();
  await expect(add).toBeHidden();
  await expect(panel.getByRole("img", { name: /^Height grew from 12 in to 15 in since / })).toBeVisible();
  await expect(panel.getByRole("button", { name: /15 in tall · Young/ })).toBeVisible();
  // The meta line follows the latest measurement.
  await expect(page.getByRole("paragraph").filter({ hasText: /^15 in tall · Young$/ })).toBeVisible();
});
