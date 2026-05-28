import { type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const DEFAULT_E2E_NAME = "E2E User";
const DEFAULT_E2E_EMAIL = "e2e@multica.ai";
const DEFAULT_E2E_WORKSPACE = "e2e-workspace";

/**
 * Log in as the default E2E user and ensure the workspace exists first.
 * Authenticates via the Firebase login endpoint, then injects the token into
 * localStorage so the browser session is authenticated.
 *
 * If an existing TestApiClient is provided, its token is reused to avoid
 * repeated auth setup.
 */
export async function loginAsDefault(page: Page, existingApi?: TestApiClient) {
  const api = existingApi ?? new TestApiClient();
  if (!existingApi) {
    await api.login(DEFAULT_E2E_EMAIL, DEFAULT_E2E_NAME);
    await api.ensureWorkspace("E2E Workspace", DEFAULT_E2E_WORKSPACE);
  }

  const token = api.getToken();
  await page.goto("/login");
  await page.evaluate(({ token: t, workspaceId }) => {
    localStorage.setItem("multica_token", t);
    if (workspaceId) localStorage.setItem("multica_workspace_id", workspaceId);
  }, { token, workspaceId: api.getWorkspaceId() });
  await page.goto("/issues");
  await page.waitForURL("**/issues", { timeout: 10000 });
  // Ensure auth init settled so subsequent hard-navigations don't race with
  // the "redirect to /" branch in the dashboard layout.
  await page.locator('[data-slot="sidebar"]').first().waitFor({ state: "visible", timeout: 15000 });
}

/**
 * Create a TestApiClient logged in as the default E2E user.
 * Call api.cleanup() in afterEach to remove test data created during the test.
 */
export async function createTestApi(): Promise<TestApiClient> {
  const api = new TestApiClient();
  await api.login(DEFAULT_E2E_EMAIL, DEFAULT_E2E_NAME);
  await api.ensureWorkspace("E2E Workspace", DEFAULT_E2E_WORKSPACE);
  return api;
}

export async function openWorkspaceMenu(page: Page) {
  // The sidebar is rendered as a `<div data-slot="sidebar">` (shadcn ui).
  // The first button inside is the workspace switcher dropdown trigger.
  await page.locator('[data-slot="sidebar"] button').first().click();
  // Wait for the dropdown menu (Radix DropdownMenu) to mount.
  await page.locator('[role="menu"]').first().waitFor({ state: "visible" });
}
