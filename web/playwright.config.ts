import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { defineConfig, devices } from "@playwright/test";

// Smoke tests run against the real binary (`make build` first) with a throwaway data dir.
// The config is evaluated in every worker; the env var makes them all share one dir.
process.env.GROWER_E2E_DATA_DIR ??= mkdtempSync(join(tmpdir(), "grower-e2e-"));
const dataDir = process.env.GROWER_E2E_DATA_DIR;

const port = 8099;
const baseURL = `http://localhost:${port}`;
export const E2E_USER = { username: "e2e", password: "e2e-password-123" };

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  globalTeardown: "./e2e/global-teardown.ts",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "mobile-chromium",
      use: {
        ...devices["Pixel 7"],
      },
    },
  ],
  webServer: {
    // Create the user before the server starts, in the same data dir, so ordering is guaranteed.
    command: `printf '%s\\n' "$E2E_PASSWORD" | ../bin/grower user create --username ${E2E_USER.username} --password-stdin && exec ../bin/grower serve`,
    url: `${baseURL}/healthz`,
    reuseExistingServer: false,
    timeout: 30_000,
    stdout: "pipe",
    stderr: "pipe",
    env: {
      GROWER_DATA_DIR: dataDir,
      GROWER_LISTEN: `:${port}`,
      GROWER_BASE_URL: baseURL,
      GROWER_LOG_FORMAT: "text",
      E2E_PASSWORD: E2E_USER.password,
    },
  },
});
