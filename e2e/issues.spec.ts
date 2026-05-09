import { test, expect } from "@playwright/test";
import { loginAsDefault, createTestApi } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Issues", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await loginAsDefault(page, api);
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("issues page loads with breadcrumb", async ({ page }) => {
    // Issues breadcrumb in the page header (hidden on mobile but our default
    // viewport is desktop-sized).
    await expect(page.locator("text=Issues").first()).toBeVisible();
    // Sidebar (shadcn) rendered.
    await expect(page.locator('[data-slot="sidebar"]').first()).toBeVisible();
  });

  test("can create an issue via the API and see it in the list", async ({ page }) => {
    const title = "E2E Created " + Date.now();
    const issue = await api.createIssue(title);

    await page.reload();
    await expect(page.locator(`text=${title}`).first()).toBeVisible({ timeout: 10000 });

    // Drill into detail page.
    await page.locator(`a[href$="/issues/${issue.id}"]`).first().click();
    await page.waitForURL(/\/issues\/[\w-]+/);
    await expect(page.locator("text=Properties").first()).toBeVisible({ timeout: 10000 });
  });
});
