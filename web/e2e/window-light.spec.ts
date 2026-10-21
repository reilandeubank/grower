import { expect, test } from "@playwright/test";
import { clientIP, E2E_USER } from "../playwright.config";

test.use(clientIP(7));

// The user is in the northern hemisphere (the default). An east window gives bright, indirect light;
// across the room is two steps dimmer (low); a 12 h grow light lifts it back to bright, indirect.
test("light: windows estimate a room's light, distance dims it, a grow light lifts it", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Username").fill(E2E_USER.username);
  await page.getByLabel("Password", { exact: true }).fill(E2E_USER.password);
  await page.getByRole("button", { name: "Log in" }).click();
  const nav = page.getByRole("navigation", { name: "Main" });

  // A new location, then edited to add one east window.
  await nav.getByRole("link", { name: "Settings" }).click();
  await page.getByRole("link", { name: "Locations" }).click();
  await page.getByRole("button", { name: "Add location" }).click();
  const add = page.getByRole("dialog", { name: "Add location" });
  await add.getByLabel("Name").fill("Study");
  await add.getByRole("button", { name: "Save" }).click();
  await expect(add).toBeHidden();
  const row = page.getByRole("button", { name: /Study/ });
  await row.click();
  const edit = page.getByRole("dialog", { name: "Edit location" });
  await edit.getByRole("button", { name: "East windows: 0", exact: true }).click();
  const east = edit.getByRole("button", { name: "East windows: 1", exact: true });
  await expect(east).toHaveAttribute("aria-pressed", "true");
  await expect(edit.getByText("Suggested light: Bright, indirect")).toBeVisible();
  await edit.getByRole("button", { name: "Save" }).click();
  await expect(edit).toBeHidden();
  await expect(row).toContainText("1 E · Bright, indirect (est.) · No plants");

  // A plant there, across the room.
  await nav.getByRole("link", { name: "Plants" }).click();
  await page.getByRole("link", { name: "Add plant" }).click();
  await page.getByLabel("Name").fill("Peace lily");
  await page.getByLabel("Location").selectOption("Study");
  await page.getByLabel("Distance from the window").selectOption("Across the room");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Peace lily" })).toBeVisible();
  await expect(page.getByText("Low light · across the room")).toBeVisible();

  // A grow light, 12 hours a day by default.
  await page.getByRole("button", { name: "Edit" }).click();
  await page.getByRole("switch", { name: "Grow light" }).check();
  await expect(page.getByLabel("Hours a day")).toHaveValue("12");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Bright, indirect light · grow light 12 h")).toBeVisible();
});
