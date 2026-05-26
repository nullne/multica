"use client";

import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";

const mockGetProviderConfig = vi.fn();
const mockUpdateProviderConfig = vi.fn();
const mockValidateProviderAPIKey = vi.fn();

vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: Object.assign(
    (selector?: any) => {
      const state = {
        workspace: { id: "ws-1", name: "Test", slug: "test" },
        members: [],
      };
      return selector ? selector(state) : state;
    },
    {
      getState: () => ({
        workspace: { id: "ws-1", name: "Test", slug: "test" },
        members: [],
      }),
    },
  ),
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock("@/shared/api", () => ({
  api: {
    getProviderConfig: (...args: any[]) => mockGetProviderConfig(...args),
    updateProviderConfig: (...args: any[]) => mockUpdateProviderConfig(...args),
    validateProviderAPIKey: (...args: any[]) => mockValidateProviderAPIKey(...args),
  },
}));

import { ProvidersTab } from "./providers-tab";

describe("ProvidersTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetProviderConfig.mockResolvedValue({
      providers: {
        codex: {
          enabled: true,
          api_key: "sk-test",
          target_version: "1.0.0",
        },
      },
    });
    mockValidateProviderAPIKey.mockResolvedValue({
      provider: "codex",
      status: "valid",
      message: "API key is valid.",
    });
  });

  it("validates a provider key and displays the result", async () => {
    render(<ProvidersTab />);

    const validateButton = await screen.findByRole("button", { name: "Validate" });
    fireEvent.click(validateButton);

    await waitFor(() => {
      expect(mockValidateProviderAPIKey).toHaveBeenCalledWith("ws-1", "codex", "sk-test");
    });
    expect(await screen.findByText("API key is valid.")).toBeInTheDocument();
  });

  it("sends a redacted saved key so the backend can validate the stored secret", async () => {
    mockGetProviderConfig.mockResolvedValue({
      providers: {
        codex: {
          enabled: true,
          api_key: "******1234",
          target_version: "1.0.0",
        },
      },
    });

    render(<ProvidersTab />);

    fireEvent.click(await screen.findByRole("button", { name: "Validate" }));

    await waitFor(() => {
      expect(mockValidateProviderAPIKey).toHaveBeenCalledWith("ws-1", "codex", "******1234");
    });
  });
});
