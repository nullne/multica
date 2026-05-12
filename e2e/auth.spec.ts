import { test, expect } from "@playwright/test";
import { loginAsDefault } from "./helpers";

test.describe("Authentication", () => {
  test("login page renders correctly", async ({ page }) => {
    await page.goto("/login");

    await expect(page.locator("text=Multica").first()).toBeVisible();
    // Firebase email link sign-in form (no DEV_LOGIN_EMAIL configured).
    await expect(page.locator('input[placeholder="you@example.com"]')).toBeVisible();
    await expect(page.locator("button", { hasText: "Email me a sign-in link" })).toBeVisible();
    await expect(page.locator("button", { hasText: "Continue with Google" })).toBeVisible();
  });

  test("login and redirect to /issues", async ({ page }) => {
    await loginAsDefault(page);

    await expect(page).toHaveURL(/\/issues/);
    // Dashboard is rendered (shadcn sidebar visible).
    await expect(page.locator('[data-slot="sidebar"]').first()).toBeVisible();
  });

  test("unauthenticated user is redirected away from dashboard", async ({ page }) => {
    await page.goto("/login");
    await page.evaluate(() => {
      localStorage.removeItem("multica_token");
      localStorage.removeItem("multica_workspace_id");
    });

    await page.goto("/issues");
    // Dashboard layout pushes unauthenticated users back to / (landing).
    await page.waitForURL((url) => !url.pathname.startsWith("/issues"), {
      timeout: 10000,
    });
  });

  test("logout clears auth and exits dashboard", async ({ page }) => {
    await loginAsDefault(page);

    // Open the user menu in the sidebar footer.
    await page.getByRole("button", { name: /e2e@multica\.ai/i }).click();

    // Click "Log out" in the user menu.
    await page.locator("text=Log out").click();

    // After logout the dashboard layout pushes us to / (landing).
    await page.waitForURL((url) => !url.pathname.startsWith("/issues"), {
      timeout: 10000,
    });
  });
});
