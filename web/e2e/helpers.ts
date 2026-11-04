import type { Page } from "@playwright/test";

/** From Add plant's first step, skips the rest, which saves the plant (with no repot reminder). */
export async function skipToSave(page: Page) {
  await page.getByRole("button", { name: "Next" }).click();
  await page.getByRole("button", { name: "Skip", exact: true }).click();
  await page.getByRole("button", { name: "Skip", exact: true }).click();
  await page.getByRole("button", { name: "Skip and save" }).click();
}
