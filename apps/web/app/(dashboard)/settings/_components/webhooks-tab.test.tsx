"use client";

import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";

const mockListWebhooks = vi.fn();
const mockListWebhookEvents = vi.fn();

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("@/shared/api", () => ({
  api: {
    listWebhooks: (...args: any[]) => mockListWebhooks(...args),
    listWebhookEvents: (...args: any[]) => mockListWebhookEvents(...args),
  },
}));

import { WebhooksTab } from "./webhooks-tab";

describe("WebhooksTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListWebhooks.mockResolvedValue([
      {
        webhook: {
          id: "wh-1",
          workspace_id: "ws-1",
          name: "Grafana Alerts",
          source_type: "oss-alert",
          token_prefix: "whk_123",
          status: "active",
          dedup_window_seconds: 600,
          bot_user_id: null,
          installation_id: null,
          created_by: "u-1",
          created_at: "2026-05-01T00:00:00Z",
          updated_at: "2026-05-01T00:00:00Z",
        },
        event_count: 1,
        actions: [],
      },
    ]);
    mockListWebhookEvents.mockResolvedValue([
      {
        id: "ev-1",
        webhook_id: "wh-1",
        dedup_key: "alert-42",
        payload: { alert: "HighCPU", value: 99 },
        status: "processed",
        issue_id: "iss-9",
        error_message: null,
        created_at: "2026-05-29T10:00:00Z",
      },
    ]);
  });

  it("renders received events with their webhook name and status", async () => {
    render(<WebhooksTab />);

    expect(await screen.findByText("Grafana Alerts")).toBeInTheDocument();
    expect(screen.getByText("oss-alert")).toBeInTheDocument();
    expect(screen.getByText("processed")).toBeInTheDocument();
  });

  it("links a processed event to its created issue", async () => {
    render(<WebhooksTab />);

    const link = await screen.findByRole("link");
    expect(link).toHaveAttribute("href", "/issues/iss-9");
  });

  it("expands a row to reveal the raw payload", async () => {
    render(<WebhooksTab />);

    const row = await screen.findByText("Grafana Alerts");
    expect(screen.queryByText(/HighCPU/)).not.toBeInTheDocument();

    fireEvent.click(row);

    await waitFor(() => {
      expect(screen.getByText(/HighCPU/)).toBeInTheDocument();
    });
  });

  it("shows an empty state when there are no events", async () => {
    mockListWebhookEvents.mockResolvedValue([]);
    render(<WebhooksTab />);

    expect(await screen.findByText(/no webhook events/i)).toBeInTheDocument();
  });
});
