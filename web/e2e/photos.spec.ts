/// <reference lib="dom" />
// The DOM lib is for the code that runs in the page via evaluate().
import { expect, test, type Page } from "@playwright/test";
import { clientIP, E2E_USER } from "../playwright.config";

test.use(clientIP(1));

/** A small JPEG drawn in the browser, so no binary fixture lives in the repo. */
async function makeJpeg(page: Page): Promise<Buffer> {
  const dataUrl = await page.evaluate(() => {
    const canvas = document.createElement("canvas");
    canvas.width = 320;
    canvas.height = 240;
    const ctx = canvas.getContext("2d")!;
    ctx.fillStyle = "seagreen";
    ctx.fillRect(0, 0, 320, 240);
    ctx.fillStyle = "gold";
    ctx.fillRect(100, 60, 120, 120);
    return canvas.toDataURL("image/jpeg", 0.8);
  });
  return Buffer.from(dataUrl.split(",")[1] ?? "", "base64");
}

test("photos: upload, cover on the grid, view, delete", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Username").fill(E2E_USER.username);
  await page.getByLabel("Password", { exact: true }).fill(E2E_USER.password);
  await page.getByRole("button", { name: "Log in" }).click();
  const nav = page.getByRole("navigation", { name: "Main" });

  await nav.getByRole("link", { name: "Plants" }).click();
  await page.getByRole("link", { name: "Add plant" }).click();
  await page.getByLabel("Name").fill("Pothos");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Pothos" })).toBeVisible();

  // Upload from the Photos tab.
  await page.getByRole("tab", { name: "Photos" }).click();
  await expect(page.getByText("No photos yet")).toBeVisible();
  await page.getByLabel("Add photo").setInputFiles({ name: "pothos.jpg", mimeType: "image/jpeg", buffer: await makeJpeg(page) });
  await expect(page.getByRole("status").filter({ hasText: "Photo added." })).toBeVisible();
  const thumb = page.getByRole("button", { name: /^Photo from / });
  await expect(thumb).toBeVisible();
  // The first photo becomes the cover, and the hero shows it.
  await expect(page.getByRole("img", { name: "Pothos" })).toBeVisible();

  // On the grid card, served from /media with the session cookie.
  await nav.getByRole("link", { name: "Plants" }).click();
  const cover = page.getByRole("link", { name: /^Pothos/ }).locator("img");
  await expect(cover).toHaveAttribute("src", /^\/media\/photos\/.+\/small$/);
  await expect.poll(() => cover.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);

  // View, then delete.
  await page.getByRole("link", { name: /^Pothos/ }).click();
  await page.getByRole("tab", { name: "Photos" }).click();
  // Android back closes the viewer and stays on the plant.
  await thumb.click();
  await expect(page.getByRole("dialog", { name: "Photo" })).toBeVisible();
  await page.goBack();
  await expect(page.getByRole("dialog", { name: "Photo" })).toBeHidden();
  await expect(page.getByRole("heading", { level: 1, name: "Pothos" })).toBeVisible();

  await thumb.click();
  const viewer = page.getByRole("dialog", { name: "Photo" });
  await expect(viewer.getByRole("button", { name: "Cover photo" })).toBeDisabled();
  await viewer.getByRole("button", { name: "Delete photo" }).click();
  await viewer.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(viewer).toBeHidden();
  await expect(page.getByText("No photos yet")).toBeVisible();
  await expect(page.getByRole("img", { name: "Pothos" })).toBeHidden();
});
