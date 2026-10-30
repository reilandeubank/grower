import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { defineConfig, devices } from "@playwright/test";

// Smoke tests run against the real binary (`make build` first) with a throwaway data dir.
// The config is evaluated in every worker; the env var makes them all share one dir.
process.env.GROWER_E2E_DATA_DIR ??= mkdtempSync(join(tmpdir(), "grower-e2e-"));
const dataDir = process.env.GROWER_E2E_DATA_DIR;
// A second server starts with no account, for the first-run setup spec.
process.env.GROWER_E2E_SETUP_DATA_DIR ??= mkdtempSync(join(tmpdir(), "grower-e2e-"));
const setupDataDir = process.env.GROWER_E2E_SETUP_DATA_DIR;

const port = 8099;
// The server and the browser must agree on what day it is (the app sends the device's local date with
// each event and the server checks it against the user's zone). Pin both, whatever the machine's zone is.
const timeZone = "America/Chicago";
const baseURL = `http://localhost:${port}`;
export const E2E_USER = { username: "e2e", password: "e2e-password-123" };

const setupPort = 8098;
/** The fresh server for setup.spec.ts, and the code it accepts (set, so the spec needn't read the log). */
export const SETUP_SERVER = {
  baseURL: `http://localhost:${setupPort}`,
  code: "e2e-setup-code",
};

/** Headers that make a spec look like its own visitor (n = 1–254), with its own login rate limit. */
export const clientIP = (n: number) => ({
  extraHTTPHeaders: { "X-Forwarded-For": `198.51.100.${n}` },
});

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
        timezoneId: timeZone,
      },
    },
  ],
  webServer: [
    {
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
        GROWER_TZ: timeZone,
        // Trust loopback as a proxy so each spec can send its own X-Forwarded-For (see clientIP):
        // specs get separate login rate-limit buckets, as distinct visitors would.
        GROWER_TRUSTED_PROXIES: "127.0.0.1/32,::1/128",
        E2E_PASSWORD: E2E_USER.password,
      },
    },
    {
      // No `user create`: this one starts empty, as a fresh install does.
      command: "exec ../bin/grower serve",
      url: `${SETUP_SERVER.baseURL}/healthz`,
      reuseExistingServer: false,
      timeout: 30_000,
      stdout: "pipe",
      stderr: "pipe",
      env: {
        GROWER_DATA_DIR: setupDataDir,
        GROWER_LISTEN: `:${setupPort}`,
        GROWER_BASE_URL: SETUP_SERVER.baseURL,
        GROWER_LOG_FORMAT: "text",
        GROWER_TZ: timeZone,
      },
    },
  ],
});
