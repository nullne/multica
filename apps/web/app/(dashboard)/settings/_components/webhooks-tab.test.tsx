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
const mockListWebhookActionEvents = vi.fn();
const mockCreateWebhook = vi.fn();
const mockUpdateWebhook = vi.fn();
const mockCreateWebhookAction = vi.fn();
const mockUpdateWebhookAction = vi.fn();

vi.mock("@/shared/api", () => ({
  api: {
    listWebhooks: (...args: any[]) => mockListWebhooks(...args),
    listDaemons: (...args: any[]) => mockListDaemons(...args),
    listBotUsers: (...args: any[]) => mockListBotUsers(...args),
    listWebhookAdapters: (...args: any[]) => mockListWebhookAdapters(...args),
    listWorkspaceWebhookEvents: (...args: any[]) => mockListWorkspaceWebhookEvents(...args),
    listWebhookEvents: (...args: any[]) => mockListWebhookEvents(...args),
    listWebhookActionEvents: (...args: any[]) => mockListWebhookActionEvents(...args),
    createWebhook: (...args: any[]) => mockCreateWebhook(...args),
    updateWebhook: (...args: any[]) => mockUpdateWebhook(...args),
    createWebhookAction: (...args: any[]) => mockCreateWebhookAction(...args),
    updateWebhookAction: (...args: any[]) => mockUpdateWebhookAction(...args),
  },
}));

// Mock the agent picker so the dialog tests don't pull in the real popover
// machinery — the picker emits agent + dispatch in one go to mirror its real
// confirmed-dispatch contract.
vi.mock("@/features/issues/components/pickers", () => ({
  AgentPicker: ({
    trigger,
    onChange,
  }: {
    trigger: React.ReactNode;
    onChange: (v: { agentId: string; daemonId: string; provider: string }) => void;
  }) => (
    <div>
      <div data-testid="agent-trigger">{trigger}</div>
      <button
        type="button"
        data-testid="confirm-agent"
        onClick={() =>
          onChange({ agentId: "agent-1", daemonId: "daemon-1", provider: "claude" })
        }
      >
        Confirm agent
      </button>
    </div>
  ),
  AgentDispatchTriggerContent: ({ value }: { value: { agentId: string | null } }) => (
    <span data-testid="dispatch-trigger-content">{value.agentId ?? "Select agent"}</span>
  ),
}));

function makeEvent(overrides: Partial<WebhookEvent> = {}): WebhookEvent {
  return {
    id: "evt-1",
    webhook_id: "wh-1",
    action_id: null,
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

async function expandEventHistory() {
  const btn = await screen.findByRole("button", { name: /event history/i });
  fireEvent.click(btn);
}

describe("WebhooksTab — event history", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListWebhooks.mockResolvedValue([]);
    mockListDaemons.mockResolvedValue([]);
    mockListBotUsers.mockResolvedValue([]);
    mockListWebhookAdapters.mockResolvedValue([]);
    mockListWorkspaceWebhookEvents.mockResolvedValue([]);
    mockListWebhookEvents.mockResolvedValue([]);
    mockListWebhookActionEvents.mockResolvedValue([]);
  });

  it("does not load workspace events on mount — Event History is collapsed by default", async () => {
    render(<WebhooksTab />);
    // Wait for the initial load to complete
    await waitFor(() => {
      expect(mockListWebhooks).toHaveBeenCalled();
    });
    expect(mockListWorkspaceWebhookEvents).not.toHaveBeenCalled();
  });

  it("shows 'No webhook events recorded yet' when there are no events after expanding", async () => {
    render(<WebhooksTab />);
    await expandEventHistory();
    await waitFor(() => {
      expect(screen.getByText("No webhook events recorded yet.")).toBeInTheDocument();
    });
  });

  it("renders a processed event in the global events section after expanding", async () => {
    const event = makeEvent({ status: "processed", payload: { title: "Deploy succeeded" } });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await expandEventHistory();
    await waitFor(() => {
      expect(screen.getByText("processed")).toBeInTheDocument();
      expect(screen.getByText("Deploy succeeded")).toBeInTheDocument();
    });
  });

  it("renders an error event with error message in the global events section after expanding", async () => {
    const event = makeEvent({
      status: "error",
      error_message: "action config missing agent_id",
    });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await expandEventHistory();
    await waitFor(() => {
      expect(screen.getByText("error")).toBeInTheDocument();
      expect(screen.getByText("action config missing agent_id")).toBeInTheDocument();
    });
  });

  it("renders a filtered event badge after expanding", async () => {
    const event = makeEvent({ status: "filtered" });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await expandEventHistory();
    await waitFor(() => {
      expect(screen.getByText("filtered")).toBeInTheDocument();
    });
  });

  it("renders a deduped event badge after expanding", async () => {
    const event = makeEvent({ status: "deduped", dedup_key: "alert-123" });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await expandEventHistory();
    await waitFor(() => {
      expect(screen.getByText("deduped")).toBeInTheDocument();
    });
  });

  it("shows unprocessed count badge when there are problem events after expanding", async () => {
    const events = [
      makeEvent({ id: "e1", status: "error" }),
      makeEvent({ id: "e2", status: "filtered" }),
      makeEvent({ id: "e3", status: "processed" }),
    ];
    mockListWorkspaceWebhookEvents.mockResolvedValue(events);

    render(<WebhooksTab />);
    await expandEventHistory();
    await waitFor(() => {
      expect(screen.getByText("2 unprocessed")).toBeInTheDocument();
    });
  });

  it("shows webhook name in the global events section after expanding", async () => {
    const wh = makeWebhookWithActions();
    const event = makeEvent({ webhook_id: "wh-1" });
    mockListWebhooks.mockResolvedValue([wh]);
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await expandEventHistory();
    await waitFor(() => {
      // Name appears in both the webhook card header and the global events row
      const names = screen.getAllByText("My Webhook");
      expect(names.length).toBeGreaterThanOrEqual(2);
    });
  });

  it("renders issue link when issue_id is present after expanding", async () => {
    const event = makeEvent({ status: "processed", issue_id: "issue-abc" });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await expandEventHistory();
    await waitFor(() => {
      const link = screen.getByRole("link", { name: /issue/i });
      expect(link).toHaveAttribute("href", "/w/test/issues/issue-abc");
    });
  });

  it("shows dedup_key in event row when present after expanding", async () => {
    const event = makeEvent({ dedup_key: "key-xyz" });
    mockListWorkspaceWebhookEvents.mockResolvedValue([event]);

    render(<WebhooksTab />);
    await expandEventHistory();
    await waitFor(() => {
      expect(screen.getByTitle("key-xyz")).toBeInTheDocument();
    });
  });
});

describe("WebhooksTab — per-action event history", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListWebhooks.mockResolvedValue([]);
    mockListDaemons.mockResolvedValue([]);
    mockListBotUsers.mockResolvedValue([]);
    mockListWebhookAdapters.mockResolvedValue([]);
    mockListWorkspaceWebhookEvents.mockResolvedValue([]);
    mockListWebhookEvents.mockResolvedValue([]);
    mockListWebhookActionEvents.mockResolvedValue([]);
  });

  async function expandWebhookCard() {
    const nameEl = await screen.findByText("My Webhook");
    const cardContainer = nameEl.closest("div.flex.items-center.gap-3");
    const expandBtn = cardContainer?.querySelector("button");
    if (!expandBtn) throw new Error("Could not find expand button");
    fireEvent.click(expandBtn);
  }

  it("shows per-action recent events toggle when webhook card is expanded", async () => {
    const wh = makeWebhookWithActions();
    wh.actions = [
      {
        id: "action-1",
        webhook_id: "wh-1",
        action_type: "create_issue",
        config: { agent_id: "agent-1", title_template: "", description_template: "", labels: [] },
        enabled: true,
        position: 0,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      },
    ];
    mockListWebhooks.mockResolvedValue([wh]);

    render(<WebhooksTab />);
    await expandWebhookCard();

    await waitFor(() => {
      expect(screen.getByText("Recent events")).toBeInTheDocument();
    });
  });

  it("loads and displays action events when per-action panel is expanded", async () => {
    const wh = makeWebhookWithActions();
    wh.actions = [
      {
        id: "action-1",
        webhook_id: "wh-1",
        action_type: "create_issue",
        config: { agent_id: "agent-1", title_template: "", description_template: "", labels: [] },
        enabled: true,
        position: 0,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      },
    ];
    const actionEvent = makeEvent({ id: "evt-action-1", action_id: "action-1", status: "processed", payload: { title: "Action alert" } });
    mockListWebhooks.mockResolvedValue([wh]);
    mockListWebhookActionEvents.mockResolvedValue([actionEvent]);

    render(<WebhooksTab />);
    await expandWebhookCard();

    const recentEventsBtn = await screen.findByText("Recent events");
    fireEvent.click(recentEventsBtn);

    await waitFor(() => {
      expect(mockListWebhookActionEvents).toHaveBeenCalledWith("wh-1", "action-1");
      expect(screen.getByText("Action alert")).toBeInTheDocument();
    });
  });

  it("shows 'No events for this action yet' when action has no events", async () => {
    const wh = makeWebhookWithActions();
    wh.actions = [
      {
        id: "action-1",
        webhook_id: "wh-1",
        action_type: "create_issue",
        config: { agent_id: "agent-1", title_template: "", description_template: "", labels: [] },
        enabled: true,
        position: 0,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      },
    ];
    mockListWebhooks.mockResolvedValue([wh]);
    mockListWebhookActionEvents.mockResolvedValue([]);

    render(<WebhooksTab />);
    await expandWebhookCard();

    const recentEventsBtn = await screen.findByText("Recent events");
    fireEvent.click(recentEventsBtn);

    await waitFor(() => {
      expect(screen.getByText("No events for this action yet")).toBeInTheDocument();
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
    mockListWebhookActionEvents.mockResolvedValue([]);
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

describe("WebhooksTab — confirmed agent dispatch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListWebhooks.mockResolvedValue([]);
    mockListDaemons.mockResolvedValue([]);
    mockListBotUsers.mockResolvedValue([]);
    mockListWebhookAdapters.mockResolvedValue([
      { source_type: "standard", description: "Standard", keys: [], example: "" },
    ]);
    mockListWorkspaceWebhookEvents.mockResolvedValue([]);
    mockListWebhookEvents.mockResolvedValue([]);
    mockListWebhookActionEvents.mockResolvedValue([]);
  });

  it("create dialog requires confirming an agent before saving", async () => {
    mockCreateWebhook.mockResolvedValue({
      webhook: { id: "wh-new", source_type: "standard" },
      token: "tok_xyz",
    });

    render(<WebhooksTab />);
    await waitFor(() => expect(mockListWebhooks).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: /create webhook/i }));
    fireEvent.change(screen.getByLabelText(/^Name$/), { target: { value: "Alerts" } });

    // Without confirming an agent, Create stays disabled — the form refuses
    // to submit a partial agent assignment.
    const createButton = screen.getByRole("button", { name: /^create$/i });
    expect(createButton).toBeDisabled();

    fireEvent.click(screen.getByTestId("confirm-agent"));
    expect(createButton).toBeEnabled();
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockCreateWebhook).toHaveBeenCalledWith(
        expect.objectContaining({
          agent_id: "agent-1",
          dispatch_provider: "claude",
          dispatch_daemon_id: "daemon-1",
        }),
      );
    });
  });

  it("edit dialog persists confirmed dispatch values onto existing actions", async () => {
    const wh = makeWebhookWithActions();
    wh.actions = [
      {
        id: "action-1",
        webhook_id: "wh-1",
        action_type: "create_issue",
        config: {
          agent_id: "agent-old",
          title_template: "",
          description_template: "",
          labels: [],
        },
        enabled: true,
        position: 0,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      },
    ];
    mockListWebhooks.mockResolvedValue([wh]);
    mockUpdateWebhook.mockResolvedValue(undefined);
    mockUpdateWebhookAction.mockResolvedValue(undefined);

    render(<WebhooksTab />);
    await waitFor(() => screen.getByText("My Webhook"));

    // The card header packs the expand chevron first, then the action
    // group (edit, toggle, regenerate, delete) — pencil-edit is index 1.
    const cardContainer = screen.getByText("My Webhook").closest("div.flex.items-center.gap-3");
    const editButton = cardContainer?.querySelectorAll("button")[1];
    if (!editButton) throw new Error("Edit button not found");
    fireEvent.click(editButton);

    await waitFor(() => screen.getByRole("heading", { name: /^edit webhook$/i }));

    // Confirm a fresh agent + dispatch via the picker, then save.
    fireEvent.click(screen.getByTestId("confirm-agent"));
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(mockUpdateWebhookAction).toHaveBeenCalledWith(
        "wh-1",
        "action-1",
        expect.objectContaining({
          config: expect.objectContaining({
            agent_id: "agent-1",
            dispatch_provider: "claude",
            dispatch_daemon_id: "daemon-1",
          }),
        }),
      );
    });
  });
});
