import { expect, test } from "@playwright/test";
import { clientIP, E2E_USER } from "../playwright.config";

test.use(clientIP(5));

// Runs last (smoke.spec.ts expects a fresh, empty Today), in the same data dir.
test("a soil check: add one, read the meter, see when to check next", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Username").fill(E2E_USER.username);
  await page.getByLabel("Password", { exact: true }).fill(E2E_USER.password);
  await page.getByRole("button", { name: "Log in" }).click();
  const nav = page.getByRole("navigation", { name: "Main" });

  // A plant whose water task checks the soil first, about every 7 days, first check today.
  await nav.getByRole("link", { name: "Plants" }).click();
  await page.getByRole("link", { name: "Add plant" }).click();
  await page.getByLabel("Name").fill("Ficus");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Ficus" })).toBeVisible();
  await page.getByRole("button", { name: "Add care task" }).click();
  const sheet = page.getByRole("dialog", { name: "Add care task" });
  await sheet.getByRole("button", { name: "Check the soil first" }).click();
  await expect(sheet.getByLabel("Check about every")).toHaveValue("7");
  await expect(sheet.getByText("3 · Dry")).toBeVisible();
  await sheet.getByRole("button", { name: "Save" }).click();
  await expect(sheet).toBeHidden();
  await expect(page.getByText("Check about every 7 days · learning from your readings")).toBeVisible();

  // On Today as "Check soil", due today.
  await nav.getByRole("link", { name: "Today" }).click();
  const dueToday = page.locator("section", { has: page.getByRole("heading", { name: /^Due today/ }) });
  await expect(dueToday.getByRole("button", { name: /^Ficus.*Check soil.*Due today/ })).toBeVisible();

  // A moist reading: no water, and the toast says when to check again.
  await dueToday.getByRole("button", { name: "Check soil for Ficus" }).click();
  const check = page.getByRole("dialog", { name: "Check soil · Ficus" });
  await check.getByRole("radio", { name: "6, moist" }).click();
  await expect(check.getByText("Still moist — no water needed.")).toBeVisible();
  await check.getByRole("button", { name: "Save reading" }).click();
  await expect(check).toBeHidden();
  await expect(
    page.getByRole("status").filter({ hasText: /^Saved reading 6 · Moist for Ficus\. Next check \w{3}, \w{3} \d+\./ }),
  ).toBeVisible();
  await expect(dueToday.getByRole("button", { name: "Check soil for Ficus" })).toBeHidden();

  // Coming up has the next check; the plant's Care tab has the reading.
  await page.getByRole("button", { name: /^Coming up/ }).click();
  await page.getByRole("button", { name: /^Ficus.*Check soil/ }).click();
  const next = page.getByRole("dialog", { name: "Check soil · Ficus" });
  await next.getByRole("button", { name: "Snooze or skip" }).click();
  await next.getByRole("button", { name: "Open plant" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Ficus" })).toBeVisible();
  await expect(page.getByText(/^Last reading 6 · Moist, \w{3} \d+$/)).toBeVisible();
  const history = page.locator("section", { has: page.getByRole("heading", { name: "History" }) });
  await expect(history.getByText(/^Checked soil · 6 Moist/)).toBeVisible();
});
