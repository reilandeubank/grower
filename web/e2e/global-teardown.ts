import { rmSync } from "node:fs";

/** Removes the throwaway data dirs created in playwright.config.ts. */
export default function globalTeardown() {
  for (const dir of [process.env.GROWER_E2E_DATA_DIR, process.env.GROWER_E2E_SETUP_DATA_DIR]) {
    if (dir && dir.includes("grower-e2e-")) rmSync(dir, { recursive: true, force: true });
  }
}
