"use client";

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import type { WebhookEvent, WebhookWithActions } from "@/shared/types";

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/settings",
}));

const mockMembers = [
  {
    id: "mem-1",
    workspace_id: "ws-1",
    user_id: "user-alice",
    role: "member" as const,
    created_at: "2026-01-01T00:00:00Z",
    name: "Alice",
    email: "alice@example.com",
    avatar_url: null,
    kind: "human" as const,
  },
];

// Mock workspace feature
vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: Object.assign(
    (selector?: any) => {
      const state = {
        workspace: { id: "ws-1", name: "Test", slug: "test" },
        agents: [],
        members: mockMembers,
      };
      return selector ? selector(state) : state;
    },
    {
      getState: () => ({
        workspace: { id: "ws-1", name: "Test", slug: "test" },
        agents: [],
        members: mockMembers,
      }),
    },
  ),
}));

// Mock labels feature
vi.mock("@/features/labels", () => ({
  useLabelStore: (selector?: any) => {
    const state = { labels: [] };
    return selector ? selector(state) : state;
  },
}));

// Mock sonner toast
vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

// Mock api
const mockListWebhooks = vi.fn();
const mockListDaemons = vi.fn();
const mockListBotUsers = vi.fn();
const mockListWebhookAdapters = vi.fn();
const mockListWorkspaceWebhookEvents = vi.fn();
const mockListWebhookEvents = vi.fn();

vi.mock("@/shared/api", () => ({
  api: {
    listWebhooks: (...args: any[]) => mockListWebhooks(...args),
    listDaemons: (...args: any[]) => mockListDaemons(...args),
    listBotUsers: (...args: any[]) => mockListBotUsers(...args),
    listWebhookAdapters: (...args: any[]) => mockListWebhookAdapters(...args),
    listWorkspaceWebhookEvents: (...args: any[]) => mockListWorkspaceWebhookEvents(...args),
    listWebhookEvents: (...args: any[]) => mockListWebhookEvents(...args),
  },
}));

function makeEvent(overrides: Partial<WebhookEvent> = {}): WebhookEvent {
  return {
    id: "evt-1",
    webhook_id: "wh-1",
    dedup_key: "",
    payload: { title: "Test alert" },
    status: "processed",
    issue_id: null,
    error_message: null,
    created_at: "2026-04-01T10:00:00Z",
    ...overrides,
  };
}

function makeWebhookWithActions(overrides: Partial<WebhookWithActions["webhook"]> = {}): WebhookWithActions {
  return {
    webhook: {
      id: "wh-1",
      workspace_id: "ws-1",
      name: "My Webhook",
      source_type: "standard",
      token_prefix: "tok_",
      status: "active",
      dedup_window_seconds: 300,
      bot_user_id: null,
      installation_id: null,
      created_by: "user-1",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      ...overrides,
    },
    event_count: 0,
    actions: [],
  };
}

import { WebhooksTab } from "./webhooks-tab";

describe("WebhooksTab — event history", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListWebhooks.mockResolvedValue([]);
    mockListDaemons.mockResolvedValue([]);
    mockListBotUsers.mockResolvedValue([]);
    mockListWebhookAdapters.mockResolvedValue([]);
    mockListWorkspaceWebhookEvents.mockResolvedValue([]);
    mockListWebhookEvents.mockResolvedValue([]);
  });

  it("shows 'No webhook events recorded yet' when there are no events", async () => {
    render(<WebhooksTab />);
    await waitFor(() => {
      expect(screen.getByText("No webhook events recorded yet.")).toBeInTheDocument();
    });
  });

  it("renders a processed event in the global events section", async () => {
    const event = makeEvent({ status: "processed", payload: { title: "Deploy succeeded" } });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await waitFor(() => {
      expect(screen.getByText("processed")).toBeInTheDocument();
      expect(screen.getByText("Deploy succeeded")).toBeInTheDocument();
    });
  });

  it("renders an error event with error message in the global events section", async () => {
    const event = makeEvent({
      status: "error",
      error_message: "action config missing agent_id",
    });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await waitFor(() => {
      expect(screen.getByText("error")).toBeInTheDocument();
      expect(screen.getByText("action config missing agent_id")).toBeInTheDocument();
    });
  });

  it("renders a filtered event badge", async () => {
    const event = makeEvent({ status: "filtered" });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await waitFor(() => {
      expect(screen.getByText("filtered")).toBeInTheDocument();
    });
  });

  it("renders a deduped event badge", async () => {
    const event = makeEvent({ status: "deduped", dedup_key: "alert-123" });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await waitFor(() => {
      expect(screen.getByText("deduped")).toBeInTheDocument();
    });
  });

  it("shows unprocessed count badge when there are problem events", async () => {
    const events = [
      makeEvent({ id: "e1", status: "error" }),
      makeEvent({ id: "e2", status: "filtered" }),
      makeEvent({ id: "e3", status: "processed" }),
    ];
    mockListWorkspaceWebhookEvents.mockResolvedValue(events);

    render(<WebhooksTab />);
    await waitFor(() => {
      expect(screen.getByText("2 unprocessed")).toBeInTheDocument();
    });
  });

  it("shows webhook name in the global events section", async () => {
    const wh = makeWebhookWithActions();
    const event = makeEvent({ webhook_id: "wh-1" });
    mockListWebhooks.mockResolvedValue([wh]);
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await waitFor(() => {
      // Name appears in both the webhook card header and the global events row
      const names = screen.getAllByText("My Webhook");
      expect(names.length).toBeGreaterThanOrEqual(2);
    });
  });

  it("renders issue link when issue_id is present", async () => {
    const event = makeEvent({ status: "processed", issue_id: "issue-abc" });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await waitFor(() => {
      const link = screen.getByRole("link", { name: /issue/i });
      expect(link).toHaveAttribute("href", "/issues/issue-abc");
    });
  });

  it("shows dedup_key in event row when present", async () => {
    const event = makeEvent({ dedup_key: "key-xyz" });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await waitFor(() => {
      expect(screen.getByTitle("key-xyz")).toBeInTheDocument();
    });
  });
});

describe("WebhooksTab — subscriber config display", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListWebhooks.mockResolvedValue([]);
    mockListDaemons.mockResolvedValue([]);
    mockListBotUsers.mockResolvedValue([]);
    mockListWebhookAdapters.mockResolvedValue([]);
    mockListWorkspaceWebhookEvents.mockResolvedValue([]);
    mockListWebhookEvents.mockResolvedValue([]);
  });

  it("shows subscriber names in action summary when subscriber_ids are configured", async () => {
    const wh = makeWebhookWithActions();
    wh.actions = [
      {
        id: "action-1",
        webhook_id: "wh-1",
        action_type: "create_issue",
        config: {
          agent_id: "agent-1",
          title_template: "",
          description_template: "",
          labels: [],
          subscriber_ids: ["user-alice"],
        },
        enabled: true,
        position: 0,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      },
    ];
    mockListWebhooks.mockResolvedValue([wh]);
    render(<WebhooksTab />);

    // Wait for the webhook card to render, then click the expand button.
    // The card's expand button is inside the card div, before the action buttons
    // (edit, toggle, regenerate, delete). It's the first button that comes right
    // before the webhook name text.
    await waitFor(() => screen.getByText("My Webhook"));
    const webhookNameEl = screen.getByText("My Webhook");
    // The expand button is the first button inside the card container
    const cardContainer = webhookNameEl.closest("div.flex.items-center.gap-3");
    const expandBtn = cardContainer?.querySelector("button");
    if (!expandBtn) throw new Error("Could not find expand button");
    fireEvent.click(expandBtn);

    await waitFor(() => {
      expect(screen.getByText("Alice")).toBeInTheDocument();
    });
  });
});
