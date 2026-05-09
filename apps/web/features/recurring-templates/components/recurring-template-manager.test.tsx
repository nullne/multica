import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { mockUser, mockWorkspace, mockMembers } from "@/test/helpers";
import type { RecurringTemplate } from "@/shared/types";

// vi.mock factories are hoisted above top-level `const` declarations, so any
// values they reference must be defined via vi.hoisted.
const mocks = vi.hoisted(() => ({
  api: {
    listRecurringTemplates: vi.fn(),
    createRecurringTemplate: vi.fn(),
    updateRecurringTemplate: vi.fn(),
    deleteRecurringTemplate: vi.fn(),
  },
}));

vi.mock("@/features/auth", () => ({
  useAuthStore: (selector: (s: unknown) => unknown) =>
    selector({ user: mockUser, isLoading: false }),
}));

const workspaceState = {
  workspace: mockWorkspace,
  workspaces: [mockWorkspace],
  members: mockMembers,
  agents: [
    {
      id: "agent-1",
      name: "Test Agent",
      providers: ["claude"],
      default_daemon_id: null as string | null,
    },
  ],
};
vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: Object.assign(
    (selector: (s: unknown) => unknown) => selector(workspaceState),
    { getState: () => workspaceState },
  ),
  useActorName: () => ({
    getMemberName: (id: string) => (id === "user-1" ? "Test User" : "Unknown"),
    getAgentName: () => "Unknown Agent",
    getActorName: (type: string, id: string) => {
      if (type === "member" && id === "user-1") return "Test User";
      return "Unknown";
    },
    getActorInitials: () => "TU",
  }),
}));

// Mock the assignee picker so it doesn't pull in heavy issue UI for tests.
// Mirror the real picker contract: agent picks emit assignee + dispatch in
// one patch (the in-popover confirmation step is what wires that up live).
vi.mock("@/features/issues/components/pickers", () => ({
  AssigneePicker: ({
    trigger,
    onUpdate,
  }: {
    trigger: React.ReactNode;
    onUpdate: (u: Record<string, unknown>) => void;
  }) => (
    <div>
      <div data-testid="assignee-trigger">{trigger}</div>
      <button
        type="button"
        data-testid="select-member"
        onClick={() =>
          onUpdate({
            assignee_type: "member",
            assignee_id: "user-1",
            dispatch_provider: null,
            dispatch_daemon_id: null,
            dispatch_daemon_label: null,
          })
        }
      >
        Select Test User
      </button>
      <button
        type="button"
        data-testid="select-agent"
        onClick={() =>
          onUpdate({
            assignee_type: "agent",
            assignee_id: "agent-1",
            dispatch_provider: "claude",
            dispatch_daemon_id: "daemon-1",
            dispatch_daemon_label: null,
          })
        }
      >
        Select Test Agent
      </button>
    </div>
  ),
}));

vi.mock("@/features/runtimes", () => ({
  useRuntimeStore: Object.assign(
    (selector: (s: unknown) => unknown) => {
      const state = {
        daemons: [
          { id: "daemon-1", daemon_id: "d1", device_name: "Test Daemon", status: "online" },
        ],
        runtimes: [
          { id: "rt-1", daemon_ref: "daemon-1", provider: "claude", auth_status: "ready" },
        ],
        fetchAll: vi.fn(),
      };
      return selector(state);
    },
    {
      getState: () => ({
        daemons: [
          { id: "daemon-1", daemon_id: "d1", device_name: "Test Daemon", status: "online" },
        ],
        runtimes: [
          { id: "rt-1", daemon_ref: "daemon-1", provider: "claude", auth_status: "ready" },
        ],
        fetchAll: vi.fn(),
      }),
    },
  ),
}));

vi.mock("@/components/common/actor-avatar", () => ({
  ActorAvatar: ({ actorId }: { actorId: string }) => (
    <span data-testid={`avatar-${actorId}`} />
  ),
}));

vi.mock("@/shared/api", () => ({
  api: mocks.api,
}));

vi.mock("@/features/recurring-templates/store", async () => {
  const { create } = await import("zustand");
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const store = create<any>((set: unknown, get: unknown) => {
    void set;
    void get;
    return {
      templates: [],
      loading: false,
      includeInactive: false,
      fetch: vi.fn(async () => {
        const res = await mocks.api.listRecurringTemplates({
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          includeInactive: (store.getState() as any).includeInactive,
        });
        store.setState({ templates: res, loading: false });
      }),
      setIncludeInactive: vi.fn(async (value: boolean) => {
        store.setState({ includeInactive: value });
        const res = await mocks.api.listRecurringTemplates({ includeInactive: value });
        store.setState({ templates: res });
      }),
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      setTemplates: (t: any) => store.setState({ templates: t }),
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      addTemplate: (t: any) =>
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        store.setState((s: any) => ({ templates: [...s.templates, t] })),
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      updateTemplate: (id: string, u: any) =>
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        store.setState((s: any) => ({
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          templates: s.templates.map((t: any) => (t.id === id ? { ...t, ...u } : t)),
        })),
      removeTemplate: (id: string) =>
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        store.setState((s: any) => ({
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          templates: s.templates.filter((t: any) => t.id !== id),
        })),
    };
  });
  return {
    // Return the zustand hook directly so React subscribes to state changes
    // and re-renders when setState fires.
    useRecurringTemplateStore: store,
  };
});

// Import after mocks so the manager picks up the mocked store.
import { RecurringTemplateManager } from "./recurring-template-manager";
import { useRecurringTemplateStore } from "@/features/recurring-templates/store";

const mockTemplate: RecurringTemplate = {
  id: "tpl-1",
  workspace_id: "ws-1",
  title: "Weekly Review",
  description: "Review items",
  priority: "medium",
  schedule: "0 9 * * 1",
  timezone: "UTC",
  enabled: true,
  successful_runs_count: 0,
  next_run_at: "2026-05-11T09:00:00Z",
  created_by_id: "user-1",
  created_by_type: "member",
  created_at: "2026-05-01T00:00:00Z",
  updated_at: "2026-05-01T00:00:00Z",
  subscriber_ids: [],
};

beforeEach(() => {
  vi.clearAllMocks();
  workspaceState.agents[0] = {
    id: "agent-1",
    name: "Test Agent",
    providers: ["claude"],
    default_daemon_id: null,
  };
  mocks.api.listRecurringTemplates.mockResolvedValue([mockTemplate]);
  // Reset store with the mock template seeded so list-render tests don't
  // depend on the manager's mount-time fetch resolving first.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (useRecurringTemplateStore as any).setState({
    templates: [mockTemplate],
    loading: false,
    includeInactive: false,
  });
});

// Find the row's hover-only action toolbar (the div containing pencil/trash).
function getRowActionButtons(rowEl: Element): HTMLButtonElement[] {
  const groups = rowEl.querySelectorAll("div.flex");
  const actionGroup = Array.from(groups).find((g) => {
    const cls = g.getAttribute("class") ?? "";
    return cls.includes("group-hover:opacity-100");
  });
  if (!actionGroup) throw new Error("hover action toolbar not found");
  return Array.from(actionGroup.querySelectorAll("button"));
}

// Find the per-row enable/disable Switch (excludes the page-level "Show inactive" switch).
function getRowSwitch(rowEl: Element): HTMLElement {
  const sw = rowEl.querySelector('[role="switch"]');
  if (!sw) throw new Error("row switch not found");
  return sw as HTMLElement;
}

describe("RecurringTemplateManager", () => {
  it("renders the header and new template button for admins", () => {
    render(<RecurringTemplateManager />);
    expect(screen.getByText("Recurring Templates")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /new template/i })).toBeInTheDocument();
  });

  it("shows existing templates with schedule and next run", () => {
    render(<RecurringTemplateManager />);
    expect(screen.getByText("Weekly Review")).toBeInTheDocument();
    expect(screen.getByText("0 9 * * 1")).toBeInTheDocument();
    expect(screen.getByText(/Next:/)).toBeInTheDocument();
  });

  it("shows successful run count without max for unlimited templates", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (useRecurringTemplateStore as any).setState({
      templates: [{ ...mockTemplate, successful_runs_count: 7 }],
    });
    render(<RecurringTemplateManager />);
    expect(screen.getByText(/^Runs:\s*7$/)).toBeInTheDocument();
  });

  it("shows progress for templates with max_runs configured", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (useRecurringTemplateStore as any).setState({
      templates: [{ ...mockTemplate, max_runs: 5, successful_runs_count: 2 }],
    });
    render(<RecurringTemplateManager />);
    expect(screen.getByText(/^Runs:\s*2\s*\/\s*5$/)).toBeInTheDocument();
  });

  it("marks templates that reached max_runs as completed", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (useRecurringTemplateStore as any).setState({
      templates: [
        {
          ...mockTemplate,
          max_runs: 1,
          successful_runs_count: 1,
          next_run_at: undefined,
        },
      ],
    });
    render(<RecurringTemplateManager />);
    expect(screen.getByText(/^completed$/i)).toBeInTheDocument();
    // Completed templates must not render the "enabled" badge.
    expect(screen.queryByText(/^enabled$/i)).not.toBeInTheDocument();
  });

  it("opens create dialog when New template is clicked", async () => {
    const user = userEvent.setup();
    render(<RecurringTemplateManager />);
    await user.click(screen.getByRole("button", { name: /new template/i }));
    expect(screen.getByText("New recurring template")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Weekly standup issue")).toBeInTheDocument();
  });

  it("includes assignee in createRecurringTemplate payload", async () => {
    const newTemplate: RecurringTemplate = {
      ...mockTemplate,
      id: "tpl-2",
      title: "Daily Standup",
      schedule: "0 9 * * *",
      assignee_type: "member",
      assignee_id: "user-1",
    };
    mocks.api.createRecurringTemplate.mockResolvedValueOnce(newTemplate);

    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    await user.click(screen.getByRole("button", { name: /new template/i }));
    await user.type(screen.getByPlaceholderText("Weekly standup issue"), "Daily Standup");
    await user.type(screen.getByPlaceholderText("0 9 * * 1"), "0 9 * * *");
    await user.click(screen.getByTestId("select-member"));
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    await waitFor(() => {
      expect(mocks.api.createRecurringTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "Daily Standup",
          schedule: "0 9 * * *",
          assignee_type: "member",
          assignee_id: "user-1",
        }),
      );
    });
  });

  it("includes max_runs when set in create payload", async () => {
    mocks.api.createRecurringTemplate.mockResolvedValueOnce({
      ...mockTemplate,
      id: "tpl-onetime",
      max_runs: 1,
    });

    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    await user.click(screen.getByRole("button", { name: /new template/i }));
    await user.type(screen.getByPlaceholderText("Weekly standup issue"), "One-time job");
    await user.type(screen.getByPlaceholderText("0 9 * * 1"), "0 9 * * *");
    await user.type(screen.getByPlaceholderText("Blank = unlimited"), "1");
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    await waitFor(() => {
      expect(mocks.api.createRecurringTemplate).toHaveBeenCalledWith(
        expect.objectContaining({ max_runs: 1 }),
      );
    });
  });

  it("omits max_runs from the create payload when left blank", async () => {
    mocks.api.createRecurringTemplate.mockResolvedValueOnce(mockTemplate);

    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    await user.click(screen.getByRole("button", { name: /new template/i }));
    await user.type(screen.getByPlaceholderText("Weekly standup issue"), "Forever");
    await user.type(screen.getByPlaceholderText("0 9 * * 1"), "0 9 * * *");
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    await waitFor(() => {
      expect(mocks.api.createRecurringTemplate).toHaveBeenCalled();
    });
    const payload = mocks.api.createRecurringTemplate.mock.calls[0]?.[0] ?? {};
    expect(payload).not.toHaveProperty("max_runs");
  });

  it("sends explicit nulls for cleared fields when updating", async () => {
    const seededTemplate: RecurringTemplate = {
      ...mockTemplate,
      assignee_type: "member",
      assignee_id: "user-1",
      due_date_offset_hours: 48,
      max_runs: 5,
    };
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (useRecurringTemplateStore as any).setState({
      templates: [seededTemplate],
      loading: false,
    });
    mocks.api.updateRecurringTemplate.mockResolvedValueOnce({
      ...seededTemplate,
      assignee_type: undefined,
      assignee_id: undefined,
      description: undefined,
      due_date_offset_hours: undefined,
      max_runs: undefined,
    });

    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    const row = screen.getByText("Weekly Review").closest(".group");
    if (!row) throw new Error("row not found");
    const buttons = getRowActionButtons(row);
    const pencil = buttons[0];
    if (!pencil) throw new Error("edit button not found");
    await act(async () => {
      fireEvent.click(pencil);
    });

    // Clear description, due-date offset, and max_runs, then save.
    const description = screen.getByPlaceholderText(
      "Optional description for issues created from this template",
    );
    await user.clear(description);
    const offset = screen.getByPlaceholderText("Optional, e.g. 72 for 3 days");
    await user.clear(offset);
    const maxRuns = screen.getByPlaceholderText("Blank = unlimited");
    await user.clear(maxRuns);
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(mocks.api.updateRecurringTemplate).toHaveBeenCalledWith(
        "tpl-1",
        expect.objectContaining({
          description: null,
          due_date_offset_hours: null,
          max_runs: null,
        }),
      );
    });
  });

  it("includes selected subscribers in the create payload", async () => {
    mocks.api.createRecurringTemplate.mockResolvedValueOnce({
      ...mockTemplate,
      id: "tpl-subs",
      subscriber_ids: ["user-1"],
    });

    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    await user.click(screen.getByRole("button", { name: /new template/i }));
    await user.type(screen.getByPlaceholderText("Weekly standup issue"), "Subscribed Job");
    await user.type(screen.getByPlaceholderText("0 9 * * 1"), "0 9 * * *");

    // Open subscribers picker and select Test User. There's a "Select Test User"
    // button rendered by the assignee mock, so scope this to the popover.
    await user.click(screen.getByText("No subscribers"));
    const popover = await screen.findByPlaceholderText("Filter members...");
    const popoverContent = popover.closest("[role=dialog]") ?? popover.parentElement!.parentElement!;
    const memberButton = within(popoverContent as HTMLElement).getByRole("button", { name: /^test user$/i });
    await user.click(memberButton);
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    await waitFor(() => {
      expect(mocks.api.createRecurringTemplate).toHaveBeenCalledWith(
        expect.objectContaining({ subscriber_ids: ["user-1"] }),
      );
    });
  });

  it("persists dispatch values emitted by the assignee picker for agent assignees", async () => {
    mocks.api.createRecurringTemplate.mockResolvedValueOnce({
      ...mockTemplate,
      id: "tpl-agent",
      assignee_type: "agent",
      assignee_id: "agent-1",
      dispatch_provider: "claude",
      dispatch_daemon_id: "daemon-1",
    });

    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    await user.click(screen.getByRole("button", { name: /new template/i }));
    await user.type(screen.getByPlaceholderText("Weekly standup issue"), "Agent Job");
    await user.type(screen.getByPlaceholderText("0 9 * * 1"), "0 9 * * *");
    // The mocked AssigneePicker emits dispatch values alongside the assignee
    // patch — mirroring the real picker's confirmation flow.
    await user.click(screen.getByTestId("select-agent"));
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    await waitFor(() => {
      expect(mocks.api.createRecurringTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          assignee_type: "agent",
          assignee_id: "agent-1",
          dispatch_daemon_id: "daemon-1",
          dispatch_provider: "claude",
        }),
      );
    });
  });

  it("clears dispatch values when switching from agent to member", async () => {
    mocks.api.createRecurringTemplate.mockResolvedValueOnce({
      ...mockTemplate,
      id: "tpl-member-after-agent",
      assignee_type: "member",
      assignee_id: "user-1",
    });

    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    await user.click(screen.getByRole("button", { name: /new template/i }));
    await user.type(screen.getByPlaceholderText("Weekly standup issue"), "Switch Job");
    await user.type(screen.getByPlaceholderText("0 9 * * 1"), "0 9 * * *");
    await user.click(screen.getByTestId("select-agent"));
    await user.click(screen.getByTestId("select-member"));
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    await waitFor(() => {
      expect(mocks.api.createRecurringTemplate).toHaveBeenCalled();
    });
    const payload = mocks.api.createRecurringTemplate.mock.calls[0]?.[0] ?? {};
    expect(payload).toMatchObject({
      assignee_type: "member",
      assignee_id: "user-1",
    });
    // Member assignees never carry a dispatch override — keep the create
    // payload free of stale agent-only fields.
    expect(payload).not.toHaveProperty("dispatch_provider");
    expect(payload).not.toHaveProperty("dispatch_daemon_id");
  });

  it("calls deleteRecurringTemplate after confirmation", async () => {
    mocks.api.deleteRecurringTemplate.mockResolvedValueOnce(undefined);
    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    const row = screen.getByText("Weekly Review").closest(".group");
    if (!row) throw new Error("row not found");
    const buttons = getRowActionButtons(row);
    const trash = buttons[buttons.length - 1];
    if (!trash) throw new Error("delete button not found");
    await act(async () => {
      fireEvent.click(trash);
    });

    await waitFor(() =>
      expect(screen.getByText(/delete recurring template/i)).toBeInTheDocument(),
    );

    await user.click(screen.getByRole("button", { name: /^delete$/i }));
    await waitFor(() => {
      expect(mocks.api.deleteRecurringTemplate).toHaveBeenCalledWith("tpl-1");
    });
  });

  it("toggles enabled state via switch", async () => {
    const updatedTemplate = { ...mockTemplate, enabled: false };
    mocks.api.updateRecurringTemplate.mockResolvedValueOnce(updatedTemplate);
    mocks.api.listRecurringTemplates.mockResolvedValue([updatedTemplate]);

    render(<RecurringTemplateManager />);

    const row = screen.getByText("Weekly Review").closest(".group");
    if (!row) throw new Error("row not found");
    const rowSwitch = getRowSwitch(row);
    await act(async () => {
      fireEvent.click(rowSwitch);
    });

    await waitFor(() => {
      expect(mocks.api.updateRecurringTemplate).toHaveBeenCalledWith("tpl-1", {
        enabled: false,
      });
    });
  });

  it("requests inactive templates when 'Show inactive' is toggled on", async () => {
    const inactive: RecurringTemplate = {
      ...mockTemplate,
      id: "tpl-disabled",
      title: "Old Job",
      enabled: false,
    };
    mocks.api.listRecurringTemplates.mockImplementation(
      async (opts?: { includeInactive?: boolean }) =>
        opts?.includeInactive ? [mockTemplate, inactive] : [mockTemplate],
    );

    render(<RecurringTemplateManager />);

    // Wait for the mount-time fetch to settle so its promise resolution does
    // not race with the toggle's refetch and clobber state out from under it.
    await waitFor(() => {
      expect(mocks.api.listRecurringTemplates).toHaveBeenCalledWith({
        includeInactive: false,
      });
    });

    const showInactiveLabel = screen.getByText(/show inactive/i);
    const labelEl = showInactiveLabel.closest("label");
    if (!labelEl) throw new Error("show inactive label not found");
    const toggle = within(labelEl).getByRole("switch");
    const user = userEvent.setup();
    await user.click(toggle);

    await waitFor(() => {
      expect(mocks.api.listRecurringTemplates).toHaveBeenCalledWith({
        includeInactive: true,
      });
    });
    await waitFor(() => {
      expect(screen.getByText("Old Job")).toBeInTheDocument();
    });
  });
});
