import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import type { Issue } from "@/shared/types";
import type { AgentTask } from "@/shared/types/agent";

const listRecentIssuesMock = vi.fn();
const listActiveTasksMock = vi.fn();
const listIssuesMock = vi.fn();
const listMembersMock = vi.fn();
const listAgentsMock = vi.fn();
const listSkillsMock = vi.fn();
const listInboxMock = vi.fn();
const listLabelsMock = vi.fn();
const getActiveTaskForIssueMock = vi.fn();

vi.mock("@/shared/api", () => ({
  api: {
    getWorkspaceId: () => null,
    listRecentIssues: (...args: unknown[]) => listRecentIssuesMock(...args),
    listActiveTasks: (...args: unknown[]) => listActiveTasksMock(...args),
    listIssues: (...args: unknown[]) => listIssuesMock(...args),
    listMembers: (...args: unknown[]) => listMembersMock(...args),
    listAgents: (...args: unknown[]) => listAgentsMock(...args),
    listSkills: (...args: unknown[]) => listSkillsMock(...args),
    listInbox: (...args: unknown[]) => listInboxMock(...args),
    listLabels: (...args: unknown[]) => listLabelsMock(...args),
    getActiveTaskForIssue: (...args: unknown[]) => getActiveTaskForIssueMock(...args),
  },
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), info: vi.fn() },
}));

import { useRecentsStore } from "@/features/navigation/recents-store";
import { useIssueStore } from "@/features/issues";
import { useActiveTaskStore } from "@/features/issues/stores/active-task-store";
import { useWorkspaceStore } from "@/features/workspace";
import { useRealtimeSync } from "./use-realtime-sync";

interface FakeWSClient {
  onAny: ReturnType<typeof vi.fn>;
  on: ReturnType<typeof vi.fn>;
  onReconnect: ReturnType<typeof vi.fn>;
  emit: (type: string, payload: unknown) => void;
  triggerReconnect: () => Promise<void>;
}

function makeFakeWS(): FakeWSClient {
  let reconnectCb: (() => void | Promise<void>) | null = null;
  const handlers = new Map<string, Array<(payload: unknown) => void>>();
  return {
    onAny: vi.fn(() => () => {}),
    on: vi.fn((type: string, cb: (payload: unknown) => void) => {
      const current = handlers.get(type) ?? [];
      current.push(cb);
      handlers.set(type, current);
      return () => {
        handlers.set(
          type,
          (handlers.get(type) ?? []).filter((handler) => handler !== cb),
        );
      };
    }),
    onReconnect: vi.fn((cb: () => void | Promise<void>) => {
      reconnectCb = cb;
      return () => {
        if (reconnectCb === cb) reconnectCb = null;
      };
    }),
    emit: (type, payload) => {
      for (const handler of handlers.get(type) ?? []) handler(payload);
    },
    triggerReconnect: async () => {
      if (reconnectCb) await reconnectCb();
    },
  };
}

function makeIssue(id: string, workspaceId: string, title = id): Issue {
  return {
    id,
    workspace_id: workspaceId,
    number: 1,
    identifier: "MUL-1",
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
    dispatch_after: null,
    dispatch_provider: null,
    dispatch_daemon_id: null,
    dispatch_daemon_label: null,
    github_auto_fix_enabled: false,
    labels: [],
    created_at: "2026-06-01T00:00:00Z",
    updated_at: "2026-06-01T00:00:00Z",
  };
}

function makeTask(issueId: string): AgentTask {
  return {
    id: `task-${issueId}`,
    agent_id: "agent-1",
    runtime_id: "rt-1",
    issue_id: issueId,
    status: "running",
    priority: 0,
    dispatched_at: null,
    started_at: null,
    completed_at: null,
    result: null,
    error: null,
    created_at: "2026-06-01T00:00:00Z",
    trigger_comment_id: null,
  };
}

function makeWorkspace(id: string) {
  return {
    id,
    name: id,
    slug: id,
    description: null,
    context: null,
    settings: {},
    repos: [],
    issue_prefix: "MUL",
    github_connected: false,
    created_at: "2026-06-01T00:00:00Z",
    updated_at: "2026-06-01T00:00:00Z",
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

beforeEach(() => {
  vi.clearAllMocks();
  useRecentsStore.getState().clear();
  useIssueStore.setState({ issues: [], loading: true, activeIssueId: null });
  useActiveTaskStore.setState({ tasks: new Map() });
  useWorkspaceStore.setState({ workspace: null });
  // The hook fires off many parallel refetches on reconnect — give every
  // one a safe noop default so the awaited Promise.all settles.
  listRecentIssuesMock.mockResolvedValue({ issues: [], total: 0 });
  listActiveTasksMock.mockResolvedValue([]);
  listIssuesMock.mockResolvedValue({ issues: [], total: 0 });
  listMembersMock.mockResolvedValue([]);
  listAgentsMock.mockResolvedValue([]);
  listSkillsMock.mockResolvedValue([]);
  listInboxMock.mockResolvedValue([]);
  listLabelsMock.mockResolvedValue([]);
  getActiveTaskForIssueMock.mockResolvedValue({ task: null });
});

describe("useRealtimeSync reconnect", () => {
  it("refetches the Recents store after a WS reconnect", async () => {
    // Seed the recents store as if the sidebar had already loaded data,
    // so refresh() has a workspace set + mine flag to replay.
    await useRecentsStore
      .getState()
      .fetch(["ws-1", "ws-2"], { mine: true, userId: "user-1" });
    const callsBeforeReconnect = listRecentIssuesMock.mock.calls.length;

    const ws = makeFakeWS();
    renderHook(() => useRealtimeSync(ws as unknown as Parameters<typeof useRealtimeSync>[0]));

    await ws.triggerReconnect();

    await waitFor(() => {
      expect(listRecentIssuesMock.mock.calls.length).toBeGreaterThan(
        callsBeforeReconnect,
      );
    });
    // The reconnect refetch must reuse the loaded mine flag, otherwise a
    // user who toggled "Only my issues" would silently get unfiltered data
    // back after a reconnect.
    const reconnectCalls = listRecentIssuesMock.mock.calls.slice(
      callsBeforeReconnect,
    );
    expect(reconnectCalls).toHaveLength(2);
    for (const call of reconnectCalls) {
      expect(call[0]).toMatchObject({ mine: true });
    }
  });

  it("does not call the Recents endpoint on reconnect if nothing was loaded", async () => {
    const ws = makeFakeWS();
    renderHook(() => useRealtimeSync(ws as unknown as Parameters<typeof useRealtimeSync>[0]));

    await ws.triggerReconnect();

    expect(listRecentIssuesMock).not.toHaveBeenCalled();
  });
});

describe("useRealtimeSync workspace filtering", () => {
  it("ignores issue updates from another workspace", () => {
    useWorkspaceStore.setState({
      workspace: {
        id: "ws-active",
        name: "Active",
        slug: "active",
        description: null,
        context: null,
        settings: {},
        repos: [],
        issue_prefix: "ACT",
        github_connected: false,
        created_at: "2026-06-01T00:00:00Z",
        updated_at: "2026-06-01T00:00:00Z",
      },
    });
    useIssueStore.setState({
      issues: [makeIssue("issue-1", "ws-active", "Current title")],
      loading: false,
      activeIssueId: null,
    });

    const ws = makeFakeWS();
    renderHook(() => useRealtimeSync(ws as unknown as Parameters<typeof useRealtimeSync>[0]));

    ws.emit("issue:updated", {
      issue: makeIssue("issue-1", "ws-other", "Stale title"),
    });

    expect(useIssueStore.getState().issues[0]?.title).toBe("Current title");
  });

  it("ignores stale initial active-task fetches after workspace changes", async () => {
    const activeTasks = deferred<AgentTask[]>();
    listActiveTasksMock.mockReturnValueOnce(activeTasks.promise);
    useWorkspaceStore.setState({ workspace: makeWorkspace("ws-old") });

    const ws = makeFakeWS();
    renderHook(() => useRealtimeSync(ws as unknown as Parameters<typeof useRealtimeSync>[0]));
    await waitFor(() => expect(listActiveTasksMock).toHaveBeenCalled());

    useWorkspaceStore.setState({ workspace: makeWorkspace("ws-new") });
    activeTasks.resolve([makeTask("issue-old")]);
    await Promise.resolve();

    expect(useActiveTaskStore.getState().tasks.size).toBe(0);
  });

  it("ignores stale task dispatch lookups after workspace changes", async () => {
    const activeTask = deferred<{ task: AgentTask | null }>();
    getActiveTaskForIssueMock.mockReturnValueOnce(activeTask.promise);
    useWorkspaceStore.setState({ workspace: makeWorkspace("ws-old") });

    const ws = makeFakeWS();
    renderHook(() => useRealtimeSync(ws as unknown as Parameters<typeof useRealtimeSync>[0]));

    ws.emit("task:dispatch", { task_id: "task-1", issue_id: "issue-old" });
    await waitFor(() => expect(getActiveTaskForIssueMock).toHaveBeenCalledWith("issue-old"));

    useWorkspaceStore.setState({ workspace: makeWorkspace("ws-new") });
    activeTask.resolve({ task: makeTask("issue-old") });
    await Promise.resolve();

    expect(useActiveTaskStore.getState().tasks.size).toBe(0);
  });
});
