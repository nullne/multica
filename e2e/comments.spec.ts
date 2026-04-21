import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Comments", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await api.createIssue("E2E Comment Test " + Date.now());
    await loginAsDefault(page, api);
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("can navigate to issue detail and see comment editor", async ({ page }) => {
    // Wait for issues to load and click first one.
    const issueLink = page.locator('a[href^="/issues/"]').first();
    await expect(issueLink).toBeVisible({ timeout: 10000 });
    await issueLink.click();
    await page.waitForURL(/\/issues\/[\w-]+/);

    // Properties panel renders on the issue detail page.
    await expect(page.locator("text=Properties").first()).toBeVisible({ timeout: 10000 });

    // The rich-text comment editor is present (placeholder rendered as a
    // ProseMirror prompt — match the data-placeholder attribute).
    await expect(
      page.locator('[data-placeholder="Leave a comment..."]'),
    ).toBeVisible({ timeout: 10000 });
  });
});
