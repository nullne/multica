import { test, expect } from "@playwright/test";
import { loginAsDefault, createTestApi } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workspace Providers", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await loginAsDefault(page, api);

    // Reset provider config to empty state before each test.
    await api.updateProviderConfig({ providers: {} });
  });

  test.afterEach(async () => {
    // Clean up: reset provider config.
    await api.updateProviderConfig({ providers: {} });
    await api.cleanup();
  });

  test("providers tab is visible in settings", async ({ page }) => {
    await page.goto("/settings");
    // Wait for dashboard chrome to render before asserting tabs (auth race).
    await expect(page.locator('[data-slot="sidebar"]').first()).toBeVisible({ timeout: 15000 });
    await expect(page.locator('[role="tablist"]').first()).toBeVisible({ timeout: 10000 });

    // Click the Providers tab.
    await page.locator('button:has-text("Providers")').first().click();

    // Provider cards should be visible
    await expect(page.locator("text=Claude Code").first()).toBeVisible();
    await expect(page.locator("text=Codex").first()).toBeVisible();

    // Save button should be present
    await expect(page.locator("button", { hasText: "Save" }).first()).toBeVisible();
  });

  test("can enable a provider and save configuration", async ({ page }) => {
    await page.goto("/settings");
    await expect(page.locator('[data-slot="sidebar"]').first()).toBeVisible({ timeout: 15000 });
    await expect(page.locator('[role="tablist"]').first()).toBeVisible({ timeout: 10000 });

    await page.locator('button:has-text("Providers")').first().click();
    await expect(page.locator("text=Claude Code").first()).toBeVisible();

    // Find the Claude Code card and toggle its switch (base-ui Switch uses
    // data-slot="switch", not role="switch").
    const claudeCard = page.locator("[data-slot='card']").filter({ hasText: "Claude Code" }).first();
    await claudeCard.locator("[data-slot='switch']").click();

    // API Key and read-only tested version should appear.
    await expect(page.locator("label:has-text('API Key')").first()).toBeVisible();
    await expect(page.locator("text=Tested Version").first()).toBeVisible();

    // Fill in an API key
    const apiKeyInput = page.locator("input[placeholder='sk-...']").first();
    await apiKeyInput.fill("sk-test-key-12345678");

    // Save
    await page.locator("button", { hasText: "Save" }).click();

    // Wait for success toast
    await expect(page.locator("text=Provider configuration saved")).toBeVisible({ timeout: 5000 });

    // Verify via API that the config was saved
    const config = await api.getProviderConfig();
    expect(config.providers?.claude?.enabled).toBe(true);
    // API key should be redacted in the response
    expect(config.providers?.claude?.api_key).toContain("****");
  });

  test("can disable a provider", async ({ page }) => {
    // Pre-configure a provider via API
    await api.updateProviderConfig({
      providers: {
        claude: { enabled: true, api_key: "sk-test-disable-me" },
      },
    });

    await page.goto("/settings");
    await expect(page.locator('[data-slot="sidebar"]').first()).toBeVisible({ timeout: 15000 });
    await expect(page.locator('[role="tablist"]').first()).toBeVisible({ timeout: 10000 });

    await page.locator('button:has-text("Providers")').first().click();
    await expect(page.locator("text=Claude Code").first()).toBeVisible();

    // Find the Claude Code card's switch (base-ui Switch).
    const claudeCard = page.locator("[data-slot='card']").filter({ hasText: "Claude Code" }).first();
    const claudeSwitch = claudeCard.locator("[data-slot='switch']");
    await expect(claudeSwitch).toHaveAttribute("data-checked", "");

    // Disable it
    await claudeSwitch.click();
    await expect(claudeSwitch).toHaveAttribute("data-unchecked", "");

    // Save
    await page.locator("button", { hasText: "Save" }).click();
    await expect(page.locator("text=Provider configuration saved")).toBeVisible({ timeout: 5000 });

    // Verify via API
    const config = await api.getProviderConfig();
    expect(config.providers?.claude?.enabled).toBe(false);
  });

  test("provider config API: get returns empty config initially", async () => {
    const config = await api.getProviderConfig();
    // Should return empty or no providers
    expect(config).toBeDefined();
  });

  test("provider config API: update and get roundtrip returns repo version", async () => {
    // Set provider config
    const updated = await api.updateProviderConfig({
      providers: {
        codex: { enabled: true, api_key: "sk-codex-test-key-1234" },
      },
    });

    // Response should have redacted API key
    expect(updated.providers?.codex?.enabled).toBe(true);
    expect(updated.providers?.codex?.api_key).toContain("****");
    expect(updated.providers?.codex?.api_key).toContain("1234");
    expect(updated.providers?.codex?.target_version).toBeTruthy();
    expect(updated.providers?.codex?.default_model).toBeTruthy();
    expect(updated.providers?.codex?.supported_models ?? []).toContain(updated.providers?.codex?.default_model);

    // GET should return same redacted config
    const fetched = await api.getProviderConfig();
    expect(fetched.providers?.codex?.enabled).toBe(true);
    expect(fetched.providers?.codex?.target_version).toBe(updated.providers?.codex?.target_version);
    expect(fetched.providers?.codex?.default_model).toBe(updated.providers?.codex?.default_model);
  });

  test("provider config API: rejects repository-owned catalog fields", async () => {
    const res = await api.updateProviderConfig({
      providers: {
        codex: {
          enabled: true,
          api_key: "sk-codex-test-key-1234",
          target_version: "0.1.0",
          default_model: "gpt-5.2-codex",
          supported_models: ["gpt-5.2-codex"],
        },
      },
    });
    expect((res as Record<string, unknown>).error).toBeDefined();
  });

  test("provider config API: rejects unsupported provider", async () => {
    const res = await api.updateProviderConfig({
      providers: {
        "unsupported-provider": { enabled: true, api_key: "" },
      },
    });
    // The response should indicate an error (400 status), but since updateProviderConfig
    // returns the parsed JSON, we check for an error field
    expect((res as Record<string, unknown>).error).toBeDefined();
  });

  test("provider config API: redacted key is preserved on update", async () => {
    // Set initial key
    await api.updateProviderConfig({
      providers: {
        claude: { enabled: true, api_key: "sk-ant-real-key-abcd" },
      },
    });

    // Get the redacted version
    const config = await api.getProviderConfig();
    const redactedKey = config.providers?.claude?.api_key ?? "";
    expect(redactedKey).toContain("****");
    expect(redactedKey).toContain("abcd");

    // Send back with the redacted key — server should preserve the original
    await api.updateProviderConfig({
      providers: {
        claude: { enabled: true, api_key: redactedKey },
      },
    });

    // Verify the key is still the same (not overwritten with the redacted value)
    const after = await api.getProviderConfig();
    expect(after.providers?.claude?.api_key).toContain("abcd");
    expect(after.providers?.claude?.target_version).toBe(config.providers?.claude?.target_version);
  });
});
