import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useAuthStore } from "@/features/auth";
import { useActiveTaskStore } from "@/features/issues/stores/active-task-store";
import type { AgentTask } from "@/shared/types";

const mocks = vi.hoisted(() => ({
  api: {
    getRoutine: vi.fn(),
    listRoutineRuns: vi.fn(),
    triggerRoutine: vi.fn(),
    updateRoutine: vi.fn(),
    deleteRoutine: vi.fn(),
  },
  router: {
    push: vi.fn(),
  },
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock("@/shared/api", () => ({
  api: mocks.api,
}));

vi.mock("sonner", () => ({
  toast: mocks.toast,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => mocks.router,
}));

vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: (selector: (state: { workspace: { slug: string }; members: { user_id: string; name: string; role: string }[] }) => unknown) =>
    selector({
      workspace: { slug: "test" },
      members: [
        { user_id: "user-1", name: "Dev User", role: "owner" },
        { user_id: "user-2", name: "Alex Chen", role: "member" },
        { user_id: "user-3", name: "Mina Park", role: "member" },
        { user_id: "user-4", name: "Sam Lee", role: "member" },
      ],
    }),
  useActorName: () => ({
    getActorName: (type: string, id: string) => {
      if (type === "agent" && id === "agent-1") return "Review Bot";
      return "Unknown";
    },
    getActorInitials: () => "RB",
    getActorAvatarUrl: () => null,
  }),
}));

import { RoutineViewPage } from "../routine-view-page";

describe("RoutineDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
    useActiveTaskStore.setState({ tasks: new Map() });
    mocks.api.triggerRoutine.mockResolvedValue({ ran: 1 });
    mocks.api.updateRoutine.mockResolvedValue({});
    mocks.api.deleteRoutine.mockResolvedValue(undefined);
  });

  it("renders a routine overview, instructions, and trigger details", async () => {
    mocks.api.getRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Daily code review",
      instructions: "Review every deployment before it goes online.",
      priority: "medium",
      assignee_id: "agent-1",
      assignee_type: "agent",
      dispatch_provider: "codex",
      dispatch_daemon_id: "daemon-1",
      dispatch_daemon_label: null,
      triggers: [
        {
          id: "trigger-schedule",
          routine_id: "routine-1",
          trigger_type: "schedule",
          schedule: "0 9 * * 1",
          timezone: "UTC",
          token_prefix: "",
          dedup_window_seconds: 600,
          successful_runs_count: 0,
          config: {},
          enabled: true,
          created_at: "2026-05-22T08:00:00Z",
          updated_at: "2026-05-22T08:00:00Z",
        },
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
        {
          id: "trigger-github",
          routine_id: "routine-1",
          trigger_type: "github",
          source_type: "github",
          token_prefix: "",
          timezone: "UTC",
          dedup_window_seconds: 600,
          successful_runs_count: 0,
          config: { event_types: ["github.pull_request.opened"] },
          enabled: true,
          created_at: "2026-05-22T08:00:00Z",
          updated_at: "2026-05-22T08:00:00Z",
        },
      ],
      actions: [{ action_type: "create_issue", config: {}, enabled: true, position: 0 }],
      subscriber_ids: ["user-1", "user-2", "user-3", "user-4"],
      label_ids: [],
      enabled: true,
    });
    mocks.api.listRoutineRuns.mockResolvedValue([
      {
        id: "run-1",
        routine_id: "routine-1",
        trigger_id: "trigger-1",
        action_id: "action-1",
        event_type: "schedule",
        dedup_key: "",
        payload: {},
        status: "processed",
        issue_id: "issue-1",
        comment_id: null,
        error_message: null,
        created_at: "2026-05-22T08:00:00Z",
        issue: {
          id: "issue-1",
          workspace_id: "ws-1",
          number: 1,
          identifier: "TES-1",
          title: "Triggered issue",
          description: null,
          status: "todo",
          priority: "medium",
          assignee_type: null,
          assignee_id: null,
          verifier_agent_id: null,
          max_verification_rounds: null,
          creator_type: "member",
          creator_id: "user-1",
          parent_issue_id: null,
          acceptance_criteria: [],
          criteria_status: null,
          position: 0,
          due_date: null,
          dispatch_provider: null,
          dispatch_daemon_id: null,
          dispatch_daemon_label: null,
          labels: [],
          created_at: "2026-05-22T08:00:00Z",
          updated_at: "2026-05-22T08:00:00Z",
        },
      },
    ]);

    render(<RoutineViewPage routineID="routine-1" />);

    expect(await screen.findByText("Daily code review")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Daily code review" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Edit routine/i })).toHaveAttribute("href", "/routines?new=1&id=routine-1");
    expect(screen.getByText("Review every deployment before it goes online.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Pause routine/i })).toBeInTheDocument();
    expect(screen.getByText("Medium")).toBeInTheDocument();
    expect(screen.getByText("Review Bot")).toBeInTheDocument();
    expect(screen.queryByText(/agent-1/)).not.toBeInTheDocument();
    expect(screen.getByText("Dev User, Alex Chen, Mina Park +1 more")).toBeInTheDocument();
    expect(screen.queryByText("4")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Triggers" })).toBeInTheDocument();
    expect(screen.getByText("Schedule")).toBeInTheDocument();
    expect(screen.getByText("Cron 0 9 * * 1 · UTC")).toBeInTheDocument();
    expect(screen.getAllByText("API").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText(/\/api\/webhook\/trigger-api/)).toBeInTheDocument();
    expect(screen.getByText("GitHub")).toBeInTheDocument();
    expect(screen.getByText("github.pull_request.opened")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Runs" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Scheduled" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Manual" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "API" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Webhook" })).toBeInTheDocument();
    expect(screen.getByText("Triggered issue")).toBeInTheDocument();
    expect(screen.getByText(/TES-1/)).toBeInTheDocument();
  });

  it("toggles routine status from the detail page", async () => {
    const user = userEvent.setup();
    mocks.api.getRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Daily code review",
      instructions: "Review every deployment before it goes online.",
      priority: "medium",
      assignee_id: "agent-1",
      assignee_type: "agent",
      dispatch_provider: "codex",
      dispatch_daemon_id: "daemon-1",
      dispatch_daemon_label: null,
      triggers: [{
        id: "trigger-1",
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
      }],
      actions: [{ action_type: "create_issue", config: {}, enabled: true, position: 0 }],
      subscriber_ids: ["user-1"],
      label_ids: [],
      enabled: true,
    });
    mocks.api.updateRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Daily code review",
      enabled: false,
      triggers: [],
      actions: [],
      subscriber_ids: ["user-1"],
      label_ids: [],
    });
    mocks.api.listRoutineRuns.mockResolvedValue([]);

    render(<RoutineViewPage routineID="routine-1" />);

    await user.click(await screen.findByRole("button", { name: /Pause routine/i }));

    await waitFor(() => {
      expect(mocks.api.updateRoutine).toHaveBeenCalledWith(
        "routine-1",
        expect.objectContaining({ enabled: false }),
      );
    });
    expect(mocks.toast.success).toHaveBeenCalledWith("Routine paused");
  });

  it("filters runs by source and shows active agent work", async () => {
    const user = userEvent.setup();
    useActiveTaskStore.setState({
      tasks: new Map([
        ["issue-1", {
          id: "task-1",
          agent_id: "agent-1",
          runtime_id: "runtime-1",
          issue_id: "issue-1",
          status: "running",
          priority: 0,
          dispatched_at: null,
          started_at: "2026-05-22T08:00:00Z",
          completed_at: null,
          result: null,
          error: null,
          created_at: "2026-05-22T08:00:00Z",
        } satisfies AgentTask],
      ]),
    });
    mocks.api.getRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Daily code review",
      priority: "medium",
      assignee_id: null,
      assignee_type: null,
      triggers: [{ id: "trigger-1" }],
      actions: [],
      subscriber_ids: [],
      label_ids: [],
      enabled: true,
    });
    mocks.api.listRoutineRuns.mockResolvedValue([
      makeRun({ id: "run-schedule", event_type: "schedule", issue_id: "issue-1", title: "Scheduled issue" }),
      makeRun({ id: "run-manual", event_type: "manual", issue_id: "issue-2", title: "Manual issue" }),
      makeRun({ id: "run-api", event_type: "custom", issue_id: "issue-3", title: "API issue" }),
      makeRun({
        id: "run-webhook",
        event_type: "github.pull_request.opened",
        issue_id: "issue-4",
        title: "Webhook issue",
        payload: {
          action: "opened",
          repository: { full_name: "octocat/hello-world" },
          pull_request: { number: 42, title: "Add webhook details" },
        },
      }),
    ]);

    render(<RoutineViewPage routineID="routine-1" />);

    expect(await screen.findByText("Scheduled issue")).toBeInTheDocument();
    expect(screen.getByTitle("Agent is working")).toBeInTheDocument();
    expect(screen.getAllByText("Webhook").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText(/pull_request\.opened/)).toBeInTheDocument();

    await user.hover(screen.getAllByText("Webhook").at(-1)!);

    expect(await screen.findByText("GitHub webhook event")).toBeInTheDocument();
    expect(screen.getByText(/octocat\/hello-world/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Manual" }));

    expect(screen.queryByText("Scheduled issue")).not.toBeInTheDocument();
    expect(screen.getByText("Manual issue")).toBeInTheDocument();
    expect(screen.queryByText("API issue")).not.toBeInTheDocument();
    expect(screen.queryByText("Webhook issue")).not.toBeInTheDocument();
  });

  it("can manually trigger a routine and refresh runs", async () => {
    const user = userEvent.setup();
    mocks.api.getRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Daily code review",
      priority: "medium",
      assignee_id: null,
      assignee_type: null,
      triggers: [{ id: "trigger-1" }],
      actions: [],
      subscriber_ids: ["user-1"],
      label_ids: [],
      enabled: true,
    });
    mocks.api.listRoutineRuns
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([
        {
          id: "run-1",
          routine_id: "routine-1",
          trigger_id: "trigger-1",
          action_id: "action-1",
          event_type: "manual",
          dedup_key: "",
          payload: {},
          status: "processed",
          issue_id: "issue-1",
          comment_id: null,
          error_message: null,
          created_at: "2026-05-22T08:00:00Z",
          issue: {
            id: "issue-1",
            workspace_id: "ws-1",
            number: 1,
            identifier: "TES-1",
            title: "Manual issue",
            description: null,
            status: "todo",
            priority: "medium",
            assignee_type: null,
            assignee_id: null,
            verifier_agent_id: null,
            max_verification_rounds: null,
            creator_type: "member",
            creator_id: "user-1",
            parent_issue_id: null,
            acceptance_criteria: [],
            criteria_status: null,
            position: 0,
            due_date: null,
            dispatch_provider: null,
            dispatch_daemon_id: null,
            dispatch_daemon_label: null,
            labels: [],
            created_at: "2026-05-22T08:00:00Z",
            updated_at: "2026-05-22T08:00:00Z",
          },
        },
      ]);

    render(<RoutineViewPage routineID="routine-1" />);

    await user.click(await screen.findByRole("button", { name: /Run now/i }));

    await waitFor(() => {
      expect(mocks.api.triggerRoutine).toHaveBeenCalledWith("routine-1");
    });
    expect(await screen.findByText("Manual issue")).toBeInTheDocument();
    expect(mocks.toast.success).toHaveBeenCalledWith("Routine triggered");
  });

  it("renders regular members as read-only", async () => {
    useAuthStore.setState({
      user: {
        id: "user-2",
        name: "Alex Chen",
        email: "alex@example.com",
        avatar_url: null,
        created_at: "2026-05-22T08:00:00Z",
        updated_at: "2026-05-22T08:00:00Z",
      },
      isLoading: false,
    });
    mocks.api.getRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Daily code review",
      priority: "medium",
      assignee_id: null,
      assignee_type: null,
      triggers: [{ id: "trigger-1" }],
      actions: [],
      subscriber_ids: ["user-1"],
      label_ids: [],
      enabled: true,
    });
    mocks.api.listRoutineRuns.mockResolvedValue([]);

    render(<RoutineViewPage routineID="routine-1" />);

    expect(await screen.findByText("Daily code review")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Run now/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Edit routine/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Pause routine/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /More actions/i })).not.toBeInTheDocument();
    expect(screen.getByText("Read-only")).toBeInTheDocument();
  });

  it("deletes a routine after confirmation", async () => {
    const user = userEvent.setup();
    mocks.api.getRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Daily code review",
      priority: "medium",
      assignee_id: null,
      assignee_type: null,
      triggers: [{ id: "trigger-1" }],
      actions: [],
      subscriber_ids: ["user-1"],
      label_ids: [],
      enabled: true,
    });
    mocks.api.listRoutineRuns.mockResolvedValue([]);

    render(<RoutineViewPage routineID="routine-1" />);

    await user.click(await screen.findByRole("button", { name: /More actions/i }));
    await user.click(await screen.findByRole("menuitem", { name: /Delete/i }));
    await user.click(screen.getByRole("button", { name: "Delete routine" }));

    await waitFor(() => {
      expect(mocks.api.deleteRoutine).toHaveBeenCalledWith("routine-1");
    });
    expect(mocks.router.push).toHaveBeenCalledWith("/routines");
  });
});

function makeRun({
  id,
  event_type,
  issue_id,
  title,
  payload = {},
}: {
  id: string;
  event_type: string;
  issue_id: string;
  title: string;
  payload?: unknown;
}) {
  return {
    id,
    routine_id: "routine-1",
    trigger_id: "trigger-1",
    action_id: "action-1",
    event_type,
    dedup_key: "",
    payload,
    status: "processed",
    issue_id,
    comment_id: null,
    error_message: null,
    created_at: "2026-05-22T08:00:00Z",
    issue: {
      id: issue_id,
      workspace_id: "ws-1",
      number: 1,
      identifier: `TES-${issue_id.slice(-1)}`,
      title,
      description: null,
      status: "todo",
      priority: "medium",
      assignee_type: null,
      assignee_id: null,
      verifier_agent_id: null,
      max_verification_rounds: null,
      creator_type: "member",
      creator_id: "user-1",
      parent_issue_id: null,
      acceptance_criteria: [],
      criteria_status: null,
      position: 0,
      due_date: null,
      dispatch_provider: null,
      dispatch_daemon_id: null,
      dispatch_daemon_label: null,
      labels: [],
      created_at: "2026-05-22T08:00:00Z",
      updated_at: "2026-05-22T08:00:00Z",
    },
  };
}
