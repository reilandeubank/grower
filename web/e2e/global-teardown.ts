import { rmSync } from "node:fs";

/** Removes the throwaway data dir created in playwright.config.ts. */
export default function globalTeardown() {
  const dir = process.env.GROWER_E2E_DATA_DIR;
  if (dir && dir.includes("grower-e2e-")) rmSync(dir, { recursive: true, force: true });
}
