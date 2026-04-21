import { test, expect } from "@playwright/test";
import { loginAsDefault } from "./helpers";

const sidebarLink = (href: string) =>
  `[data-slot="sidebar"] a[href="${href}"]`;

test.describe("Navigation", () => {
  test.beforeEach(async ({ page }) => {
    await loginAsDefault(page);
  });

  test("sidebar navigation works", async ({ page }) => {
    await page.locator(sidebarLink("/inbox")).first().click();
    await page.waitForURL("**/inbox");
    await expect(page).toHaveURL(/\/inbox/);

    await page.locator(sidebarLink("/agents")).first().click();
    await page.waitForURL("**/agents");
    await expect(page).toHaveURL(/\/agents/);

    await page.locator(sidebarLink("/issues")).first().click();
    await page.waitForURL((url) => url.pathname === "/issues");
    await expect(page).toHaveURL(/\/issues$/);
  });

  test("settings page is reachable from sidebar", async ({ page }) => {
    await page.locator(sidebarLink("/settings")).first().click();
    await page.waitForURL("**/settings");
    await expect(page.locator('[role="tablist"]').first()).toBeVisible({ timeout: 10000 });
  });

  test("agents page renders", async ({ page }) => {
    await page.locator(sidebarLink("/agents")).first().click();
    await page.waitForURL("**/agents");
    await expect(page.locator("main").first()).toBeVisible();
  });
});
