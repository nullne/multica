import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";

const mocks = vi.hoisted(() => ({
  api: {
    createRoutine: vi.fn(),
    updateRoutine: vi.fn(),
    listRoutines: vi.fn(),
    getRoutine: vi.fn(),
    listRoutineRuns: vi.fn(),
  },
  regenerateRoutineTriggerToken: vi.fn(),
  clipboardWriteText: vi.fn(),
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
  searchParams: new URLSearchParams(),
}));

vi.mock("@/features/issues/components/pickers", () => ({
  AssigneePicker: ({
    trigger,
    onUpdate,
  }: {
    trigger: React.ReactNode;
    onUpdate: (patch: Record<string, unknown>) => void;
  }) => (
    <div data-testid="issue-assignee-picker">
      {trigger}
      <button
        type="button"
        onClick={() =>
          onUpdate({
            assignee_type: "agent",
            assignee_id: "agent-1",
            dispatch_provider: "codex",
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

vi.mock("@/shared/api", () => ({
  api: mocks.api,
  regenerateRoutineTriggerToken: mocks.regenerateRoutineTriggerToken,
}));

vi.mock("sonner", () => ({
  toast: mocks.toast,
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => mocks.searchParams,
}));

import RoutinesPage from "./page";

describe("RoutinesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: mocks.clipboardWriteText.mockResolvedValue(undefined),
      },
    });
    useAuthStore.setState({
      user: {
        id: "user-1",
        name: "Dev User",
        email: "dev@example.com",
        avatar_url: null,
        created_at: "2026-05-22T08:00:00Z",
        updated_at: "2026-05-22T08:00:00Z",
      },
      isLoading: false,
    });
    useWorkspaceStore.setState({
      members: [
        {
          id: "member-1",
          workspace_id: "ws-1",
          user_id: "user-1",
          role: "owner",
          name: "Dev User",
          email: "dev@example.com",
          avatar_url: null,
          kind: "human",
          created_at: "2026-05-22T08:00:00Z",
        },
      ],
      agents: [],
    });
    mocks.api.listRoutines.mockResolvedValue([]);
    mocks.api.getRoutine.mockResolvedValue(null);
    mocks.api.listRoutineRuns.mockResolvedValue([]);
    mocks.api.createRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Daily review",
      triggers: [],
      actions: [],
      subscriber_ids: [],
      label_ids: [],
      enabled: true,
    });
    mocks.api.updateRoutine.mockResolvedValue({});
    mocks.regenerateRoutineTriggerToken.mockResolvedValue({
      trigger: {
        id: "trigger-api",
        routine_id: "routine-1",
        trigger_type: "api",
        source_type: "standard",
        token_prefix: "sk-new-token",
        timezone: "UTC",
        dedup_window_seconds: 600,
        successful_runs_count: 0,
        config: {},
        enabled: true,
        created_at: "2026-05-22T08:00:00Z",
        updated_at: "2026-05-22T08:00:00Z",
      },
      token: "sk-new-token-full",
    });
  });

  it("defaults subscribers to the current user and summarizes selected members", async () => {
    const user = userEvent.setup();
    useWorkspaceStore.setState({
      members: [
        {
          id: "member-1",
          workspace_id: "ws-1",
          user_id: "user-1",
          role: "owner",
          name: "Dev User",
          email: "dev@example.com",
          avatar_url: null,
          kind: "human",
          created_at: "2026-05-22T08:00:00Z",
        },
        {
          id: "member-2",
          workspace_id: "ws-1",
          user_id: "user-2",
          role: "member",
          name: "Alex Chen",
          email: "alex@example.com",
          avatar_url: null,
          kind: "human",
          created_at: "2026-05-22T08:00:00Z",
        },
        {
          id: "member-3",
          workspace_id: "ws-1",
          user_id: "user-3",
          role: "member",
          name: "Mina Park",
          email: "mina@example.com",
          avatar_url: null,
          kind: "human",
          created_at: "2026-05-22T08:00:00Z",
        },
        {
          id: "member-4",
          workspace_id: "ws-1",
          user_id: "user-4",
          role: "member",
          name: "Sam Lee",
          email: "sam@example.com",
          avatar_url: null,
          kind: "human",
          created_at: "2026-05-22T08:00:00Z",
        },
      ],
    });
    mocks.searchParams = new URLSearchParams("new=1");

    render(<RoutinesPage />);

    expect(screen.getByRole("button", { name: /Dev User/i })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Dev User/i }));
    await user.click(screen.getByRole("button", { name: /Alex Chen/i }));
    await user.click(screen.getByRole("button", { name: /Mina Park/i }));
    await user.click(screen.getByRole("button", { name: /Sam Lee/i }));

    expect(screen.getByRole("button", { name: /Dev User, Alex Chen, Mina Park \+1 more/i })).toBeInTheDocument();

    await user.type(screen.getByLabelText(/Name/), "Daily review");
    await user.type(screen.getByLabelText("Instructions"), "Review the code every day");
    await user.click(screen.getByRole("button", { name: "Select Test Agent" }));
    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /API/i }));
    await user.click(screen.getByRole("button", { name: /Create routine/i }));

    await waitFor(() => {
      expect(mocks.api.createRoutine).toHaveBeenCalledWith(
        expect.objectContaining({
          subscriber_ids: ["user-1", "user-2", "user-3", "user-4"],
        }),
      );
    });
  });

  it("renders the routines list by default", () => {
    mocks.searchParams = new URLSearchParams();
    render(<RoutinesPage />);

    expect(screen.getByRole("heading", { name: "Routines" })).toBeInTheDocument();
    expect(screen.getByText("Loading routines...")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /New routine/i })).toHaveAttribute("href", "/routines?new=1");
  });

  it("renders routines returned by the API", async () => {
    mocks.searchParams = new URLSearchParams();
    mocks.api.listRoutines.mockResolvedValue([
      {
        id: "routine-1",
        workspace_id: "ws-1",
        name: "Daily code review",
        triggers: [{ id: "trigger-1" }],
        actions: [],
        subscriber_ids: [],
        label_ids: [],
        enabled: true,
      },
    ]);
    render(<RoutinesPage />);

    expect(await screen.findByText("Daily code review")).toBeInTheDocument();
    expect(screen.getByText("1 trigger · Enabled")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Daily code review/i })).toHaveAttribute("href", "/routines/routine-1");
    expect(screen.queryByText("View")).not.toBeInTheDocument();
  });

  it("hides creation entry points for regular members", async () => {
    mocks.searchParams = new URLSearchParams();
    useWorkspaceStore.setState({
      members: [
        {
          id: "member-1",
          workspace_id: "ws-1",
          user_id: "user-1",
          role: "member",
          name: "Dev User",
          email: "dev@example.com",
          avatar_url: null,
          kind: "human",
          created_at: "2026-05-22T08:00:00Z",
        },
      ],
    });
    mocks.api.listRoutines.mockResolvedValue([]);

    render(<RoutinesPage />);

    await screen.findByText("No routines yet");
    expect(screen.queryByRole("link", { name: /New routine/i })).not.toBeInTheDocument();
    expect(screen.getByText("Ask an owner or admin to create routines for this workspace.")).toBeInTheDocument();
  });

  it("renders the routine issue template and trigger configuration surface", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1");
    render(<RoutinesPage />);

    expect(screen.getByTestId("routines-page-scroll")).toHaveClass("overflow-y-auto");
    expect(screen.getByRole("heading", { name: "New routine" })).toBeInTheDocument();
    expect(screen.getByLabelText(/Name/)).toBeInTheDocument();
    expect(screen.getByLabelText("Instructions")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Create routine/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Create routine/i })).toBeDisabled();
    expect(screen.queryByText("Save preview")).not.toBeInTheDocument();

    expect(screen.getByRole("heading", { name: "Issue template" })).toBeInTheDocument();
    expect(screen.getByText("Assignee")).toBeInTheDocument();
    expect(screen.getByText("Subscribers")).toBeInTheDocument();
    expect(screen.getByText("Labels")).toBeInTheDocument();
    expect(screen.getByTestId("issue-assignee-picker")).toBeInTheDocument();
    expect(screen.queryByText("Dispatch")).not.toBeInTheDocument();

    expect(screen.queryByText(/Runs once on/)).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Once" })).not.toBeInTheDocument();
    expect(screen.queryByText("Run on a recurring cron schedule or once at a future time")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /GitHub event/i })).not.toBeInTheDocument();
    const addTrigger = screen.getByRole("button", { name: /Add another trigger/i });

    await user.click(addTrigger);

    const scheduleOption = screen.getByRole("button", { name: /Schedule/i });
    const githubTrigger = screen.getByRole("button", { name: /GitHub event/i });
    expect(scheduleOption).toBeInTheDocument();
    expect(githubTrigger).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /API/i })).toBeInTheDocument();
    await user.click(scheduleOption);
    const scheduleTrigger = screen.getByRole("button", { name: /Runs once on/ });
    expect(scheduleTrigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getAllByText(/Runs once on/)).toHaveLength(1);
    expect(screen.getByRole("tab", { name: "Once" })).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Custom" }));
    expect(screen.getByDisplayValue("0 9 * * 1")).toBeInTheDocument();
    expect(screen.getByText("Format: minute hour day-of-month month day-of-week")).toBeInTheDocument();

    await user.click(addTrigger);
    expect(screen.getByRole("button", { name: /Runs on cron 0 9/ })).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByDisplayValue("0 9 * * 1")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /GitHub event/i }));

    expect(screen.getByRole("button", { name: /Runs on cron 0 9/ })).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByText("Run on a recurring cron schedule or once at a future time")).toBeInTheDocument();
    expect(screen.getByText("Event")).toBeInTheDocument();
    const eventTypeTrigger = screen.getByRole("button", { name: "GitHub event type" });
    expect(eventTypeTrigger).toBeInTheDocument();
    await user.click(eventTypeTrigger);
    const eventSearch = await screen.findByRole("textbox", { name: /Search GitHub events/i });
    await user.type(eventSearch, "merged");
    const mergedItem = await screen.findByRole("menuitemcheckbox", { name: "Merged" });
    expect(mergedItem).toHaveAttribute("aria-checked", "false");
    await user.click(mergedItem);
    await user.keyboard("{Escape}");
    expect(screen.getAllByText("PR opened, PR merged").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Add a filter condition")).toBeInTheDocument();
    expect(screen.queryByText("GitHub labels")).not.toBeInTheDocument();

    await user.click(screen.getByText("Add a filter condition"));

    expect(screen.getByDisplayValue("Author")).toBeInTheDocument();
    expect(screen.getByText("is one of")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Comma-separated values")).toBeInTheDocument();

    await user.type(screen.getByLabelText(/Name/), "Daily review");
    expect(screen.getByRole("button", { name: /Create routine/i })).toBeDisabled();
    await user.type(screen.getByLabelText("Instructions"), "Review the code every day");
    expect(screen.getByRole("button", { name: /Create routine/i })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Select Test Agent" }));
    expect(screen.getByRole("button", { name: /Create routine/i })).not.toBeDisabled();
    await user.click(screen.getByRole("button", { name: /Create routine/i }));

    await waitFor(() => {
      expect(mocks.api.createRoutine).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Daily review",
          instructions: "Review the code every day",
          assignee_type: "agent",
          assignee_id: "agent-1",
          triggers: expect.arrayContaining([
            expect.objectContaining({ trigger_type: "schedule", schedule: "0 9 * * 1" }),
            expect.objectContaining({
              trigger_type: "github",
              config: expect.objectContaining({
                event_types: expect.arrayContaining([
                  "github.pull_request.opened",
                  "github.pull_request.merged",
                ]),
              }),
            }),
          ]),
        }),
      );
    });
  });

  it("searches events and mixes selections across categories", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1");
    render(<RoutinesPage />);

    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /GitHub event/i }));
    await user.click(screen.getByRole("button", { name: "GitHub event type" }));

    const eventSearch = await screen.findByRole("textbox", { name: /Search GitHub events/i });
    await user.type(eventSearch, "publish");
    const published = await screen.findByRole("menuitemcheckbox", { name: "Published" });
    expect(published).toHaveAttribute("aria-checked", "false");
    await user.click(published);

    await user.clear(eventSearch);
    await user.type(eventSearch, "created");
    await user.click(await screen.findByRole("menuitemcheckbox", { name: "Created" }));
    await user.keyboard("{Escape}");

    // Default pull_request.opened + release.published + release.created
    expect(screen.getAllByText("3 events · 2 categories").length).toBeGreaterThanOrEqual(1);
  });

  it("offers event presets and falls back to Custom for manual selections", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1");
    render(<RoutinesPage />);

    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /GitHub event/i }));

    const presets = screen.getByRole("group", { name: "GitHub event presets" });
    // Default selection (pull_request.opened) matches the "PR opened" preset
    expect(within(presets).getByRole("button", { name: "PR opened" })).toHaveAttribute("aria-pressed", "true");
    expect(within(presets).getByRole("button", { name: "Custom" })).toHaveAttribute("aria-pressed", "false");

    // Picking a preset swaps the selection
    await user.click(within(presets).getByRole("button", { name: "Release published" }));
    expect(within(presets).getByRole("button", { name: "Release published" })).toHaveAttribute("aria-pressed", "true");
    expect(within(presets).getByRole("button", { name: "PR opened" })).toHaveAttribute("aria-pressed", "false");

    // A manual multi-event selection via the dropdown lights up Custom
    await user.click(screen.getByRole("button", { name: "GitHub event type" }));
    const eventSearch = await screen.findByRole("textbox", { name: /Search GitHub events/i });
    await user.type(eventSearch, "merged");
    await user.click(await screen.findByRole("menuitemcheckbox", { name: "Merged" }));
    await user.keyboard("{Escape}");
    expect(within(presets).getByRole("button", { name: "Custom" })).toHaveAttribute("aria-pressed", "true");
  });

  it("shows API trigger URL and generated token after create", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1");
    mocks.api.createRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "API routine",
      priority: "medium",
      assignee_id: "agent-1",
      assignee_type: "agent",
      triggers: [
        {
          id: "trigger-api",
          routine_id: "routine-1",
          trigger_type: "api",
          source_type: "standard",
          token_prefix: "sk-api-token",
          token: "sk-api-token-full",
          timezone: "UTC",
          dedup_window_seconds: 600,
          successful_runs_count: 0,
          config: {},
          enabled: true,
          created_at: "2026-05-22T08:00:00Z",
          updated_at: "2026-05-22T08:00:00Z",
        },
      ],
      actions: [],
      subscriber_ids: [],
      label_ids: [],
      enabled: true,
    });

    render(<RoutinesPage />);

    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /API/i }));

    expect(screen.getByText("API URL")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Save routine first/i })).toBeDisabled();

    await user.type(screen.getByLabelText(/Name/), "API routine");
    await user.type(screen.getByLabelText("Instructions"), "Create from API");
    await user.click(screen.getByRole("button", { name: "Select Test Agent" }));
    await user.click(screen.getByRole("button", { name: /Create routine/i }));

    expect(await screen.findByDisplayValue(/\/api\/routine-triggers\/trigger-api/)).toBeInTheDocument();
    expect(screen.getByDisplayValue("sk-api-token...")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "API token generated" })).toBeInTheDocument();
    expect(screen.getByText("sk-api-token-full")).toBeInTheDocument();
    expect(screen.getByText(/curl -X POST/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy token" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy curl request" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Done" }));
    expect(screen.getByRole("button", { name: /Regenerate token/i })).not.toBeDisabled();
  });

  it("shows existing API trigger prefix and regenerates the token in edit mode", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1&id=routine-1");
    mocks.api.getRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Existing API routine",
      instructions: "Existing instructions",
      priority: "high",
      assignee_id: "agent-1",
      assignee_type: "agent",
      triggers: [
        {
          id: "trigger-api",
          routine_id: "routine-1",
          trigger_type: "api",
          source_type: "standard",
          token_prefix: "sk-old-token",
          timezone: "UTC",
          dedup_window_seconds: 600,
          successful_runs_count: 0,
          config: {},
          enabled: true,
          created_at: "2026-05-22T08:00:00Z",
          updated_at: "2026-05-22T08:00:00Z",
        },
      ],
      actions: [],
      subscriber_ids: [],
      label_ids: [],
      enabled: true,
    });

    render(<RoutinesPage />);

    expect(await screen.findByDisplayValue(/\/api\/routine-triggers\/trigger-api/)).toBeInTheDocument();
    expect(screen.getByDisplayValue("sk-old-token...")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Regenerate token/i }));

    await waitFor(() => {
      expect(mocks.regenerateRoutineTriggerToken).toHaveBeenCalledWith("routine-1", "trigger-api");
    });
    expect(screen.getByDisplayValue("sk-new-token...")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "API token generated" })).toBeInTheDocument();
    expect(screen.getByText("sk-new-token-full")).toBeInTheDocument();
  });

  it("loads an existing routine in edit mode and updates it", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1&id=routine-1");
    mocks.api.getRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Existing routine",
      instructions: "Existing instructions",
      priority: "high",
      assignee_id: "agent-1",
      assignee_type: "agent",
      triggers: [{ id: "trigger-1", trigger_type: "schedule", schedule: "0 9 * * 1", timezone: "UTC", config: {} }],
      actions: [],
      subscriber_ids: [],
      label_ids: [],
      enabled: true,
    });

    render(<RoutinesPage />);

    expect(await screen.findByRole("heading", { name: "Edit routine" })).toBeInTheDocument();
    expect(screen.getByDisplayValue("Existing routine")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Update routine/i })).not.toBeDisabled();
    await user.clear(screen.getByLabelText(/Name/));
    await user.type(screen.getByLabelText(/Name/), "Updated routine");
    await user.click(screen.getByRole("button", { name: /Update routine/i }));

    await waitFor(() => {
      expect(mocks.api.updateRoutine).toHaveBeenCalledWith(
        "routine-1",
        expect.objectContaining({ name: "Updated routine" }),
      );
    });
  });
});
