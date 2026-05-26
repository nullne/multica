import { defineConfig } from "@playwright/test";

const evidenceDir = process.env.ROUTINES_EVIDENCE_DIR;

export default defineConfig({
  testDir: "./e2e",
  outputDir: evidenceDir ? `${evidenceDir}/playwright-output` : "test-results",
  timeout: 60000,
  retries: 0,
  workers: process.env.CI ? 1 : undefined,
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN ?? "http://localhost:3000",
    headless: true,
    navigationTimeout: 30000,
    actionTimeout: 15000,
    screenshot: evidenceDir ? "on" : "off",
    video: evidenceDir ? "on" : "off",
    trace: evidenceDir ? "on" : "off",
    launchOptions: {
      args: process.env.CI ? ["--no-sandbox", "--no-proxy-server"] : [],
    },
  },
  projects: [
    {
      name: "chromium",
      use: { browserName: "chromium" },
    },
  ],
  // Don't auto-start servers — they must be running already
  // This avoids complexity and port conflicts during testing
});
