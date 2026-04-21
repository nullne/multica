import { test, expect } from "@playwright/test";
import { loginAsDefault } from "./helpers";

test.describe("Settings", () => {
  test("settings page loads with workspace tab visible", async ({ page }) => {
    await loginAsDefault(page);

    await page.locator('[data-slot="sidebar"] a[href="/settings"]').first().click();
    await page.waitForURL("**/settings");

    // Settings uses a vertical Tabs layout. The tablist + at least one tab
    // trigger should be visible.
    await expect(page.locator('[role="tablist"]').first()).toBeVisible({ timeout: 10000 });
    await expect(page.locator('[role="tab"]', { hasText: "General" }).first()).toBeVisible();
  });
});
