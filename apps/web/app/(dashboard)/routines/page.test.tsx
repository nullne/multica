import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useAuthStore } from "@/features/auth";
import { useRoutineStore } from "@/features/routines";
import { useWorkspaceStore } from "@/features/workspace";

const mocks = vi.hoisted(() => ({
  api: {
    createRoutine: vi.fn(),
    updateRoutine: vi.fn(),
    listRoutines: vi.fn(),
    getRoutine: vi.fn(),
    listRoutineRuns: vi.fn(),
  },
  generateRoutineTriggerTokenDraft: vi.fn(),
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
  api: {
    ...mocks.api,
    getWorkspaceId: () => "ws-1",
  },
  generateRoutineTriggerTokenDraft: mocks.generateRoutineTriggerTokenDraft,
  regenerateRoutineTriggerToken: mocks.regenerateRoutineTriggerToken,
}));

vi.mock("sonner", () => ({
  toast: mocks.toast,
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => mocks.searchParams,
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("react-resizable-panels", async (importOriginal) => {
  const actual: object = await importOriginal();
  return {
    ...actual,
    useDefaultLayout: () => ({ defaultLayout: undefined, onLayoutChanged: () => {} }),
  };
});

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
      workspace: {
        id: "ws-1",
        name: "Workspace",
        slug: "workspace",
        description: null,
        context: null,
        settings: {},
        repos: [],
        issue_prefix: "MUL",
        github_connected: false,
        created_at: "2026-05-22T08:00:00Z",
        updated_at: "2026-05-22T08:00:00Z",
      },
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
    useRoutineStore.setState({ routines: [], loading: true, selectedId: null });
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
      github_auto_fix_enabled: false,
    });
    mocks.api.updateRoutine.mockResolvedValue({});
    mocks.generateRoutineTriggerTokenDraft.mockResolvedValue({
      draft_id: "draft-api",
      token_prefix: "sk-draft-token",
      token: "sk-draft-token-full",
    });
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
    await user.click(screen.getByRole("tab", { name: "Behavior" }));
    await user.click(screen.getByRole("switch", { name: "Auto-fix pull requests" }));
    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /Schedule/i }));
    await user.click(screen.getByRole("button", { name: /Create routine/i }));

    await waitFor(() => {
      expect(mocks.api.createRoutine).toHaveBeenCalledWith(
        expect.objectContaining({
          subscriber_ids: ["user-1", "user-2", "user-3", "user-4"],
          github_auto_fix_enabled: true,
        }),
      );
    });
  });

  it("renders the routines list by default", () => {
    mocks.searchParams = new URLSearchParams();
    render(<RoutinesPage />);

    expect(screen.getByRole("heading", { name: "Routines" })).toBeInTheDocument();
    expect(screen.getByText("Loading routines...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /New/i })).toBeInTheDocument();
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
        github_auto_fix_enabled: false,
      },
    ]);
    mocks.api.getRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Daily code review",
      instructions: "Review incoming work",
      priority: "medium",
      triggers: [{ id: "trigger-1", trigger_type: "schedule", schedule: "0 9 * * 1", timezone: "UTC", config: {} }],
      actions: [],
      subscriber_ids: [],
      label_ids: [],
      enabled: true,
      github_auto_fix_enabled: false,
    });
    const user = userEvent.setup();
    render(<RoutinesPage />);

    const row = await screen.findByRole("button", { name: /Daily code review/i });
    expect(row).toBeInTheDocument();
    expect(screen.getByText("1 trigger · Enabled")).toBeInTheDocument();
    expect(screen.queryByText("View")).not.toBeInTheDocument();

    await user.click(row);

    expect(await screen.findByText("Review incoming work")).toBeInTheDocument();
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
    expect(screen.queryByRole("button", { name: /New/i })).not.toBeInTheDocument();
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
    expect(screen.getAllByText("Labels").length).toBeGreaterThanOrEqual(1);
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
    expect(screen.getByText("Event")).toBeInTheDocument();
    const eventTypeTrigger = screen.getByRole("button", { name: "GitHub event type" });
    expect(eventTypeTrigger).toBeInTheDocument();
    await user.click(eventTypeTrigger);
    const eventSearch = await screen.findByRole("textbox", { name: /Search GitHub events/i });
    await user.type(eventSearch, "pull_request.closed");
    const closedItem = await screen.findByRole("menuitemradio", { name: "Closed" });
    await user.click(closedItem);
    await user.keyboard("{Escape}");
    expect(screen.getAllByText("Pull request closed").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Add a filter condition")).toBeInTheDocument();
    expect(screen.queryByText("GitHub labels")).not.toBeInTheDocument();

    await user.click(screen.getByText("Add a filter condition"));

    expect(screen.getByDisplayValue("Author")).toBeInTheDocument();
    expect(screen.getByText("is one of")).toBeInTheDocument();
    await user.selectOptions(screen.getByDisplayValue("Author"), "base_branch");
    expect(screen.getByDisplayValue("Base branch")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("main, releases/**")).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText("main, releases/**"), "main");

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
                event_types: ["github.pull_request.closed"],
                filters: [
                  {
                    field: "base_branch",
                    operator: "is one of",
                    value: "main",
                  },
                ],
              }),
            }),
          ]),
        }),
      );
    });
  });

  it("allows multiple triggers of the same type and saves each instance", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1");
    render(<RoutinesPage />);

    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /Schedule/i }));
    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /Schedule/i }));
    expect(screen.getAllByRole("button", { name: /Runs once on/ })).toHaveLength(2);

    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /GitHub event/i }));
    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /GitHub event/i }));
    expect(screen.getAllByRole("button", { name: /PR opened/ }).length).toBeGreaterThanOrEqual(2);

    await user.type(screen.getByLabelText(/Name/), "Multi trigger routine");
    await user.type(screen.getByLabelText("Instructions"), "Create work from multiple sources");
    await user.click(screen.getByRole("button", { name: "Select Test Agent" }));
    await user.click(screen.getByRole("button", { name: /Create routine/i }));

    await waitFor(() => {
      expect(mocks.api.createRoutine).toHaveBeenCalledWith(
        expect.objectContaining({
          triggers: [
            expect.objectContaining({ trigger_type: "schedule" }),
            expect.objectContaining({ trigger_type: "schedule" }),
            expect.objectContaining({
              trigger_type: "github",
              config: { event_types: ["github.pull_request.opened"] },
            }),
            expect.objectContaining({
              trigger_type: "github",
              config: { event_types: ["github.pull_request.opened"] },
            }),
          ],
        }),
      );
    });
  });

  it("groups triggers of the same type in the form", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1");
    render(<RoutinesPage />);

    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /GitHub event/i }));
    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /Schedule/i }));
    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /GitHub event/i }));
    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));

    const triggerButtons = screen
      .getAllByRole("button")
      .filter((button) =>
        /Runs once on|PR opened/.test(button.textContent ?? ""),
      )
      .map((button) => button.textContent ?? "");

    expect(triggerButtons).toHaveLength(3);
    expect(triggerButtons[0]).toMatch(/Runs once on/);
    expect(triggerButtons[1]).toMatch(/PR opened/);
    expect(triggerButtons[2]).toMatch(/PR opened/);
  });

  it("uses pull_request.closed plus a merged condition for the PR merged preset", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1");
    render(<RoutinesPage />);

    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /GitHub event/i }));
    await user.click(screen.getByRole("button", { name: "PR merged" }));

    expect(screen.getAllByText("Pull request closed").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByDisplayValue("Is merged")).toBeInTheDocument();
    expect(screen.getByDisplayValue("true")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /Search GitHub events/i })).not.toBeInTheDocument();
    expect(screen.queryByText("PR opened, PR merged")).not.toBeInTheDocument();

    await user.type(screen.getByLabelText(/Name/), "Merged PR routine");
    await user.type(screen.getByLabelText("Instructions"), "Create work when PRs merge");
    await user.click(screen.getByRole("button", { name: "Select Test Agent" }));
    await user.click(screen.getByRole("button", { name: /Create routine/i }));

    await waitFor(() => {
      expect(mocks.api.createRoutine).toHaveBeenCalledWith(
        expect.objectContaining({
          triggers: [
            expect.objectContaining({
              trigger_type: "github",
              config: {
                event_types: ["github.pull_request.closed"],
                filters: [
                  {
                    field: "is_merged",
                    operator: "equals",
                    value: "true",
                  },
                ],
              },
            }),
          ],
        }),
      );
    });
  });

  it("shows only filters supported by the selected GitHub event", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1");
    render(<RoutinesPage />);

    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /GitHub event/i }));

    await user.click(screen.getByRole("button", { name: "Add a filter condition" }));
    expect(screen.getByDisplayValue("Author")).toBeInTheDocument();
    expect(screen.getByText("Title")).toBeInTheDocument();
    expect(screen.getByText("Body")).toBeInTheDocument();
    expect(screen.getByText("Base branch")).toBeInTheDocument();
    expect(screen.getByText("Head branch")).toBeInTheDocument();
    expect(screen.getAllByText("Labels").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Is draft")).toBeInTheDocument();
    expect(screen.getByText("Is merged")).toBeInTheDocument();
    expect(screen.queryByText("Changed paths")).not.toBeInTheDocument();
    expect(screen.queryByText("Tags")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Issue opened" }));

    expect(screen.getByRole("button", { name: "Add a filter condition" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Add a filter condition" }));
    expect(screen.getByDisplayValue("Author")).toBeInTheDocument();
    expect(screen.getByText("Title")).toBeInTheDocument();
    expect(screen.getByText("Body")).toBeInTheDocument();
    expect(screen.getByText("State")).toBeInTheDocument();
    expect(screen.getAllByText("Labels").length).toBeGreaterThanOrEqual(1);
    expect(screen.queryByText("Base branch")).not.toBeInTheDocument();
    expect(screen.queryByText("Is merged")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Release published" }));

    expect(screen.getByRole("button", { name: "Add a filter condition" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Add a filter condition" }));
    expect(screen.getByDisplayValue("Tag name")).toBeInTheDocument();
    expect(screen.getByText("Target branch")).toBeInTheDocument();
    expect(screen.getByText("Release name")).toBeInTheDocument();
    expect(screen.getByText("Is draft")).toBeInTheDocument();
    expect(screen.getByText("Is prerelease")).toBeInTheDocument();
    expect(screen.queryByText("Author")).not.toBeInTheDocument();
  });

  it("searches events and replaces the selected GitHub event", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1");
    render(<RoutinesPage />);

    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /GitHub event/i }));
    await user.click(screen.getByRole("button", { name: "GitHub event type" }));

    const eventSearch = await screen.findByRole("textbox", { name: /Search GitHub events/i });
    await user.type(eventSearch, "publish");
    const published = await screen.findByRole("menuitemradio", { name: "Published" });
    await user.click(published);

    expect(screen.queryByRole("textbox", { name: /Search GitHub events/i })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "GitHub event type" }));
    const updatedEventSearch = await screen.findByRole("textbox", { name: /Search GitHub events/i });
    await user.type(updatedEventSearch, "created");
    await user.click(await screen.findByRole("menuitemradio", { name: "Created" }));

    expect(screen.getAllByText("Release created").length).toBeGreaterThanOrEqual(1);
    const presets = screen.getByRole("group", { name: "GitHub event presets" });
    expect(within(presets).getByRole("button", { name: "Release published" })).toHaveAttribute("aria-pressed", "false");
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

    // A manual non-preset selection via the dropdown lights up Custom
    await user.click(screen.getByRole("button", { name: "GitHub event type" }));
    const eventSearch = await screen.findByRole("textbox", { name: /Search GitHub events/i });
    await user.type(eventSearch, "ready");
    await user.click(await screen.findByRole("menuitemradio", { name: "Ready for review" }));
    await user.keyboard("{Escape}");
    expect(within(presets).getByRole("button", { name: "Custom" })).toHaveAttribute("aria-pressed", "true");
  });

  it("does not save an API trigger until a token is generated", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1");

    render(<RoutinesPage />);

    await user.click(screen.getByRole("button", { name: /Add another trigger/i }));
    await user.click(screen.getByRole("button", { name: /API/i }));
    await user.type(screen.getByLabelText(/Name/), "API routine");
    await user.type(screen.getByLabelText("Instructions"), "Create from API");
    await user.click(screen.getByRole("button", { name: "Select Test Agent" }));

    expect(screen.getByRole("button", { name: /Create routine/i })).toBeDisabled();
    expect(mocks.api.createRoutine).not.toHaveBeenCalled();
  });

  it("generates an API trigger token before create and saves the generated trigger", async () => {
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
    await user.click(screen.getByRole("button", { name: /Generate token/i }));

    expect(await screen.findByDisplayValue(/\/api\/webhook\/draft-api/)).toBeInTheDocument();
    expect(screen.getByDisplayValue("sk-draft-token...")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "API token generated" })).toBeInTheDocument();
    expect(screen.getByText("sk-draft-token-full")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Done" }));

    await user.type(screen.getByLabelText(/Name/), "API routine");
    await user.type(screen.getByLabelText("Instructions"), "Create from API");
    await user.click(screen.getByRole("button", { name: "Select Test Agent" }));
    await user.click(screen.getByRole("button", { name: /Create routine/i }));

    await waitFor(() => {
      expect(mocks.api.createRoutine).toHaveBeenCalledWith(
        expect.objectContaining({
          triggers: [
            expect.objectContaining({
              trigger_type: "api",
              id: "draft-api",
              token_draft_id: "draft-api",
            }),
          ],
        }),
      );
    });
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

    expect(await screen.findByDisplayValue(/\/api\/webhook\/trigger-api/)).toBeInTheDocument();
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
