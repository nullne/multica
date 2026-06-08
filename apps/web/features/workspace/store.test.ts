import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Workspace } from "@/shared/types";

const mockSetWorkspaceId = vi.fn();
const mockListMembers = vi.fn();
const mockListAgents = vi.fn();
const mockListSkills = vi.fn();
const mockIssueFetch = vi.fn();
const mockInboxFetch = vi.fn();
const mockLabelFetch = vi.fn();
const mockSetRuntimes = vi.fn();
const mockRuntimeReset = vi.fn();
const mockSetIssues = vi.fn();
const mockSetActiveIssue = vi.fn();
const mockSetInboxItems = vi.fn();
const mockSetLabels = vi.fn();
const mockSetActiveTasks = vi.fn();
const mockClearIssueSelection = vi.fn();
const mockRoutineReset = vi.fn();

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
  },
}));

vi.mock("@/shared/api", () => ({
  api: {
    setWorkspaceId: (...args: unknown[]) => mockSetWorkspaceId(...args),
    listMembers: (...args: unknown[]) => mockListMembers(...args),
    listAgents: (...args: unknown[]) => mockListAgents(...args),
    listSkills: (...args: unknown[]) => mockListSkills(...args),
  },
}));

vi.mock("@/features/issues", () => ({
  useIssueStore: {
    getState: () => ({
      fetch: mockIssueFetch,
      setIssues: mockSetIssues,
      setActiveIssue: mockSetActiveIssue,
    }),
  },
}));

vi.mock("@/features/inbox", () => ({
  useInboxStore: {
    getState: () => ({
      fetch: mockInboxFetch,
      setItems: mockSetInboxItems,
    }),
  },
}));

vi.mock("@/features/labels", () => ({
  useLabelStore: {
    getState: () => ({
      fetch: mockLabelFetch,
      setLabels: mockSetLabels,
    }),
  },
}));

vi.mock("@/features/runtimes", () => ({
  useRuntimeStore: {
    getState: () => ({
      setRuntimes: mockSetRuntimes,
      reset: mockRuntimeReset,
    }),
  },
}));

vi.mock("@/features/routines", () => ({
  useRoutineStore: {
    getState: () => ({
      reset: mockRoutineReset,
    }),
  },
}));

vi.mock("@/features/issues/stores/active-task-store", () => ({
  useActiveTaskStore: {
    getState: () => ({
      setTasks: mockSetActiveTasks,
    }),
  },
}));

vi.mock("@/features/issues/stores/selection-store", () => ({
  useIssueSelectionStore: {
    getState: () => ({
      clear: mockClearIssueSelection,
    }),
  },
}));

import { useWorkspaceStore } from "./store";

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function makeWorkspace(id: string): Workspace {
  return {
    id,
    name: `Workspace ${id}`,
    slug: id,
    description: null,
    context: null,
    settings: {},
    repos: [],
    issue_prefix: "MUL",
    github_connected: false,
    created_at: "2026-05-06T00:00:00Z",
    updated_at: "2026-05-06T00:00:00Z",
  };
}

async function flushPromises() {
  await Promise.resolve();
  await Promise.resolve();
}

describe("useWorkspaceStore hydrateWorkspace", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    useWorkspaceStore.setState({
      workspace: null,
      workspaces: [],
      members: [],
      agents: [],
      skills: [],
    });
    mockIssueFetch.mockResolvedValue(undefined);
    mockInboxFetch.mockResolvedValue(undefined);
    mockLabelFetch.mockResolvedValue(undefined);
  });

  it("returns after selecting the workspace without waiting for background data", async () => {
    const members = createDeferred<Array<{ id: string }>>();
    mockListMembers.mockReturnValueOnce(members.promise);
    mockListAgents.mockResolvedValueOnce([{ id: "agent-1" }]);
    mockListSkills.mockResolvedValueOnce([{ id: "skill-1" }]);

    const workspace = makeWorkspace("ws-1");
    const hydratePromise = useWorkspaceStore.getState().hydrateWorkspace([workspace]);

    const status = await Promise.race([
      hydratePromise.then(() => "resolved"),
      new Promise<string>((resolve) => setTimeout(() => resolve("timed-out"), 0)),
    ]);

    expect(status).toBe("resolved");
    expect(useWorkspaceStore.getState().workspace?.id).toBe("ws-1");
    expect(useWorkspaceStore.getState().members).toEqual([]);
    expect(mockIssueFetch).toHaveBeenCalledOnce();
    expect(mockInboxFetch).toHaveBeenCalledOnce();
    expect(mockLabelFetch).toHaveBeenCalledOnce();

    members.resolve([{ id: "member-1" }]);
    await flushPromises();

    expect(useWorkspaceStore.getState().members).toEqual([{ id: "member-1" }]);
    expect(useWorkspaceStore.getState().agents).toEqual([{ id: "agent-1" }]);
    expect(useWorkspaceStore.getState().skills).toEqual([{ id: "skill-1" }]);
  });

  it("ignores stale background results from a previously selected workspace", async () => {
    const oldMembers = createDeferred<Array<{ id: string }>>();
    mockListMembers.mockReturnValueOnce(oldMembers.promise);
    mockListAgents.mockResolvedValueOnce([{ id: "agent-old" }]);
    mockListSkills.mockResolvedValueOnce([{ id: "skill-old" }]);

    await useWorkspaceStore.getState().hydrateWorkspace([makeWorkspace("ws-old")]);

    mockListMembers.mockResolvedValueOnce([{ id: "member-new" }]);
    mockListAgents.mockResolvedValueOnce([{ id: "agent-new" }]);
    mockListSkills.mockResolvedValueOnce([{ id: "skill-new" }]);

    await useWorkspaceStore.getState().hydrateWorkspace([makeWorkspace("ws-new")]);
    await flushPromises();

    oldMembers.resolve([{ id: "member-old" }]);
    await flushPromises();

    expect(useWorkspaceStore.getState().workspace?.id).toBe("ws-new");
    expect(useWorkspaceStore.getState().members).toEqual([{ id: "member-new" }]);
    expect(useWorkspaceStore.getState().agents).toEqual([{ id: "agent-new" }]);
    expect(useWorkspaceStore.getState().skills).toEqual([{ id: "skill-new" }]);
  });
});

describe("useWorkspaceStore switchWorkspace", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    useWorkspaceStore.setState({
      workspace: makeWorkspace("ws-old"),
      workspaces: [makeWorkspace("ws-old"), makeWorkspace("ws-new")],
      members: [{ id: "member-old" }] as never,
      agents: [{ id: "agent-old" }] as never,
      skills: [{ id: "skill-old" }] as never,
    });
    mockIssueFetch.mockResolvedValue(undefined);
    mockInboxFetch.mockResolvedValue(undefined);
    mockLabelFetch.mockResolvedValue(undefined);
    mockListMembers.mockResolvedValue([]);
    mockListAgents.mockResolvedValue([]);
    mockListSkills.mockResolvedValue([]);
  });

  it("clears workspace-scoped stores before hydrating the next workspace", async () => {
    await useWorkspaceStore.getState().switchWorkspace("ws-new");

    expect(mockSetIssues).toHaveBeenCalledWith([]);
    expect(mockSetActiveIssue).toHaveBeenCalledWith(null);
    expect(mockSetInboxItems).toHaveBeenCalledWith([]);
    expect(mockSetLabels).toHaveBeenCalledWith([]);
    expect(mockRuntimeReset).toHaveBeenCalledOnce();
    expect(mockRoutineReset).toHaveBeenCalledOnce();
    expect(mockSetActiveTasks).toHaveBeenCalledWith([]);
    expect(mockClearIssueSelection).toHaveBeenCalledOnce();
    expect(useWorkspaceStore.getState().workspace?.id).toBe("ws-new");
  });
});
