import { expect, test } from "@playwright/test";
import { clientIP, E2E_USER } from "../playwright.config";

test.use(clientIP(4));

// Runs after smoke.spec.ts (which expects a fresh, empty Today), in the same data dir.
test("the core loop: add a task, do it, undo, snooze, see history", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Username").fill(E2E_USER.username);
  await page.getByLabel("Password", { exact: true }).fill(E2E_USER.password);
  await page.getByRole("button", { name: "Log in" }).click();
  const nav = page.getByRole("navigation", { name: "Main" });

  // A plant with a Water task every 7 days, first due today (the form's defaults).
  await nav.getByRole("link", { name: "Plants" }).click();
  await page.getByRole("link", { name: "Add plant" }).click();
  await page.getByLabel("Name").fill("Calathea");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Calathea" })).toBeVisible();
  await page.getByRole("button", { name: "Add care task" }).click();
  const sheet = page.getByRole("dialog", { name: "Add care task" });
  await expect(sheet.getByText("Every 7 days")).toBeVisible();
  // Seasons would make the due dates depend on the month the test runs in.
  await sheet.getByRole("switch", { name: /Adjust for seasons/ }).uncheck();
  await sheet.getByRole("button", { name: "Save" }).click();
  await expect(sheet).toBeHidden();
  await expect(page.getByText("Every 7 days · Due today")).toBeVisible();

  // On Today, under Due today.
  await nav.getByRole("link", { name: "Today" }).click();
  const dueToday = page.locator("section", { has: page.getByRole("heading", { name: /^Due today/ }) });
  const check = page.getByRole("button", { name: "Mark Water Calathea done" });
  await expect(dueToday.getByRole("button", { name: "Mark Water Calathea done" })).toBeVisible();

  // Done, then Undo puts it back.
  await check.click();
  await expect(check).toBeHidden();
  const toast = page.getByRole("status").filter({ hasText: "Watered Calathea." });
  await expect(toast).toBeVisible();
  await toast.getByRole("button", { name: "Undo" }).click();
  await expect(dueToday.getByRole("button", { name: "Mark Water Calathea done" })).toBeVisible();

  // Snooze to tomorrow from the row's sheet.
  await page.getByRole("button", { name: /^Calathea/ }).click();
  const actions = page.getByRole("dialog", { name: "Water · Calathea" });
  await actions.getByRole("button", { name: "Snooze" }).click();
  await actions.getByRole("button", { name: /^Tomorrow/ }).click();
  await expect(page.getByRole("status").filter({ hasText: "Snoozed until" })).toBeVisible();
  await expect(page.getByText("Next: Water Calathea tomorrow.")).toBeVisible();

  // Done from Coming up: the next one is a week out.
  await page.getByRole("button", { name: /^Coming up/ }).click();
  await expect(page.getByRole("button", { name: /^Calathea.*Tomorrow/ })).toBeVisible();
  await check.click();
  await expect(page.getByRole("status").filter({ hasText: "Watered Calathea." })).toBeVisible();
  await expect(page.getByRole("button", { name: /^Calathea.*In 7 days/ })).toBeVisible();

  // The plant's history has it in words.
  await page.getByRole("button", { name: /^Calathea/ }).click();
  await page.getByRole("dialog", { name: "Water · Calathea" }).getByRole("button", { name: "Open plant" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Calathea" })).toBeVisible();
  const history = page.locator("section", { has: page.getByRole("heading", { name: "History" }) });
  await expect(history.getByText(/^Watered/)).toBeVisible();
  await expect(history.getByText(/^Snoozed Water until/)).toBeVisible();

  // "Open plant" took the sheet's history entry, so Back returns to Today.
  await page.goBack();
  await expect(page).toHaveURL(/\/$/);
});
