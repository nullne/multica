import { test, expect } from "@playwright/test";
import { loginAsDefault, createTestApi, openWorkspaceMenu } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workspace switch from issue detail", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await loginAsDefault(page, api);
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("switching workspace from workspace-aware issue URL keeps the new workspace active", async ({
    page,
  }) => {
    // Get primary workspace info before we touch workspaceId.
    const primaryWsId = api.getWorkspaceId()!;
    const allWorkspaces = await api.getWorkspaces();
    const primaryWs = allWorkspaces.find((ws) => ws.id === primaryWsId)!;

    // Create an issue in the primary workspace.
    const issue = await api.createIssue("Workspace Switch Test " + Date.now());

    // Ensure a second workspace exists (this updates api.workspaceId to it).
    const secondWs = await api.ensureWorkspace("E2E Workspace 2", "e2e-workspace-2");
    // Restore primary workspace context for any further api calls.
    api.setWorkspaceId(primaryWsId);

    // Navigate directly to the workspace-aware issue URL.
    await page.goto(`/w/${primaryWs.slug}/issues/${issue.id}`);
    await page.waitForURL(`**/w/${primaryWs.slug}/issues/${issue.id}`, { timeout: 10000 });
    await expect(page.locator("text=Properties").first()).toBeVisible({ timeout: 10000 });

    // Switch to the second workspace via the sidebar switcher.
    await openWorkspaceMenu(page);
    const menuItem = page.locator(`[role="menu"]`).getByText(secondWs.name).first();
    await menuItem.waitFor({ state: "visible", timeout: 5000 });
    await menuItem.click();

    // Allow time for any unintended revert to happen.
    await page.waitForTimeout(800);

    // The second workspace should remain active — not reverted to primary.
    const sidebar = page.locator('[data-slot="sidebar"]').first();
    await expect(sidebar).toContainText(secondWs.name, { timeout: 5000 });
    await expect(sidebar).not.toContainText(primaryWs.name);
  });

  test("switching workspace from legacy issue URL keeps the new workspace active", async ({
    page,
  }) => {
    // Get primary workspace info.
    const primaryWsId = api.getWorkspaceId()!;
    const allWorkspaces = await api.getWorkspaces();
    const primaryWs = allWorkspaces.find((ws) => ws.id === primaryWsId)!;

    // Create an issue in the primary workspace.
    const issue = await api.createIssue("Legacy Route Switch Test " + Date.now());

    // Ensure a second workspace exists.
    const secondWs = await api.ensureWorkspace("E2E Workspace 2", "e2e-workspace-2");
    api.setWorkspaceId(primaryWsId);

    // Navigate via the legacy /issues/:id route.
    await page.goto(`/issues/${issue.id}`);
    // Legacy route may redirect to workspace-aware URL — wait for either form.
    await page.waitForURL(
      (url) =>
        url.pathname === `/issues/${issue.id}` ||
        url.pathname === `/w/${primaryWs.slug}/issues/${issue.id}`,
      { timeout: 10000 },
    );
    await expect(page.locator("text=Properties").first()).toBeVisible({ timeout: 10000 });

    // Switch to the second workspace.
    await openWorkspaceMenu(page);
    const menuItem = page.locator(`[role="menu"]`).getByText(secondWs.name).first();
    await menuItem.waitFor({ state: "visible", timeout: 5000 });
    await menuItem.click();

    // Allow time for any unintended revert.
    await page.waitForTimeout(800);

    // The second workspace should remain active.
    const sidebar = page.locator('[data-slot="sidebar"]').first();
    await expect(sidebar).toContainText(secondWs.name, { timeout: 5000 });
    await expect(sidebar).not.toContainText(primaryWs.name);
  });
});
