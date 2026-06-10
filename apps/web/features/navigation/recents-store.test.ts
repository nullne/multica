import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Issue } from "@/shared/types";

const listRecentIssuesMock = vi.fn();

vi.mock("@/shared/api", () => ({
  api: {
    listRecentIssues: (...args: unknown[]) => listRecentIssuesMock(...args),
  },
}));

import { useRecentsStore } from "./recents-store";

function makeIssue(overrides: Partial<Issue> & {
  id: string;
  workspace_id: string;
  updated_at: string;
}): Issue {
  return {
    number: 0,
    identifier: overrides.id,
    title: "Untitled",
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
    created_at: overrides.updated_at,
    ...overrides,
  };
}

describe("useRecentsStore", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useRecentsStore.getState().clear();
  });

  it("fetches each workspace exactly once and orders by recency", async () => {
    listRecentIssuesMock.mockImplementation(
      ({ workspace_id }: { workspace_id: string }) =>
        Promise.resolve({
          issues:
            workspace_id === "ws-1"
              ? [makeIssue({ id: "a", workspace_id: "ws-1", updated_at: "2026-01-01T00:00:00Z" })]
              : [makeIssue({ id: "b", workspace_id: "ws-2", updated_at: "2026-01-05T00:00:00Z" })],
          total: 1,
        }),
    );

    await useRecentsStore.getState().fetch(["ws-1", "ws-2"]);

    expect(listRecentIssuesMock).toHaveBeenCalledTimes(2);
    const ids = useRecentsStore.getState().issues.map((i) => i.id);
    expect(ids).toEqual(["b", "a"]);
  });

  it("dedupes a concurrent fetch for the same workspace set", async () => {
    listRecentIssuesMock.mockResolvedValue({ issues: [], total: 0 });

    const first = useRecentsStore.getState().fetch(["ws-1", "ws-2"]);
    const second = useRecentsStore.getState().fetch(["ws-1", "ws-2"]);
    await Promise.all([first, second]);

    expect(listRecentIssuesMock).toHaveBeenCalledTimes(2);
  });

  it("forwards the mine flag with the user id to the server", async () => {
    listRecentIssuesMock.mockResolvedValue({ issues: [], total: 0 });

    await useRecentsStore
      .getState()
      .fetch(["ws-1"], { mine: true, userId: "user-1" });

    expect(listRecentIssuesMock).toHaveBeenCalledWith(
      expect.objectContaining({ workspace_id: "ws-1", mine: true }),
    );
  });

  it("refetches when the mine flag toggles", async () => {
    listRecentIssuesMock.mockResolvedValue({ issues: [], total: 0 });

    await useRecentsStore.getState().fetch(["ws-1"], { userId: "user-1" });
    await useRecentsStore
      .getState()
      .fetch(["ws-1"], { mine: true, userId: "user-1" });

    expect(listRecentIssuesMock).toHaveBeenCalledTimes(2);
    expect(listRecentIssuesMock.mock.calls[0]?.[0]).not.toMatchObject({
      mine: true,
    });
    expect(listRecentIssuesMock.mock.calls[1]?.[0]).toMatchObject({
      mine: true,
    });
  });

  it("drops upserts that do not involve the user while in mine mode", async () => {
    listRecentIssuesMock.mockResolvedValue({
      issues: [
        makeIssue({
          id: "mine",
          workspace_id: "ws-1",
          updated_at: "2026-01-01T00:00:00Z",
          assignee_type: "member",
          assignee_id: "user-1",
        }),
      ],
      total: 1,
    });

    await useRecentsStore
      .getState()
      .fetch(["ws-1"], { mine: true, userId: "user-1" });

    useRecentsStore.getState().upsertIssue(
      makeIssue({
        id: "not-mine",
        workspace_id: "ws-1",
        updated_at: "2026-01-10T00:00:00Z",
        creator_id: "user-other",
        assignee_id: null,
      }),
    );

    expect(useRecentsStore.getState().issues.map((i) => i.id)).toEqual(["mine"]);
  });

  it("removes a previously-mine issue from the list when it gets reassigned away", async () => {
    listRecentIssuesMock.mockResolvedValue({
      issues: [
        makeIssue({
          id: "mine",
          workspace_id: "ws-1",
          updated_at: "2026-01-01T00:00:00Z",
          assignee_type: "member",
          assignee_id: "user-1",
        }),
      ],
      total: 1,
    });

    await useRecentsStore
      .getState()
      .fetch(["ws-1"], { mine: true, userId: "user-1" });

    useRecentsStore.getState().upsertIssue(
      makeIssue({
        id: "mine",
        workspace_id: "ws-1",
        updated_at: "2026-01-10T00:00:00Z",
        creator_type: "member",
        creator_id: "user-other",
        assignee_type: "member",
        assignee_id: "user-other",
      }),
    );

    expect(useRecentsStore.getState().issues).toEqual([]);
  });

  it("upserts an incoming issue update without refetching", async () => {
    listRecentIssuesMock.mockResolvedValue({
      issues: [
        makeIssue({ id: "a", workspace_id: "ws-1", updated_at: "2026-01-01T00:00:00Z", title: "Old" }),
      ],
      total: 1,
    });

    await useRecentsStore.getState().fetch(["ws-1"]);
    const callsAfterInitial = listRecentIssuesMock.mock.calls.length;

    useRecentsStore.getState().upsertIssue(
      makeIssue({ id: "a", workspace_id: "ws-1", updated_at: "2026-01-10T00:00:00Z", title: "New" }),
    );

    expect(listRecentIssuesMock).toHaveBeenCalledTimes(callsAfterInitial);
    expect(useRecentsStore.getState().issues[0]?.title).toBe("New");
  });

  it("ignores upserts for workspaces that are not loaded", async () => {
    listRecentIssuesMock.mockResolvedValue({ issues: [], total: 0 });
    await useRecentsStore.getState().fetch(["ws-1"]);

    useRecentsStore.getState().upsertIssue(
      makeIssue({ id: "stranger", workspace_id: "ws-other", updated_at: "2026-01-10T00:00:00Z" }),
    );

    expect(useRecentsStore.getState().issues).toEqual([]);
  });

  it("refresh re-runs the loaded fetch with the same mine and user", async () => {
    listRecentIssuesMock.mockResolvedValue({ issues: [], total: 0 });

    await useRecentsStore
      .getState()
      .fetch(["ws-1", "ws-2"], { mine: true, userId: "user-1" });
    const initialCalls = listRecentIssuesMock.mock.calls.length;

    await useRecentsStore.getState().refresh();

    expect(listRecentIssuesMock).toHaveBeenCalledTimes(initialCalls + 2);
    const refreshCalls = listRecentIssuesMock.mock.calls.slice(initialCalls);
    for (const call of refreshCalls) {
      expect(call[0]).toMatchObject({ mine: true });
    }
  });

  it("refresh is a no-op when nothing has been loaded yet", async () => {
    listRecentIssuesMock.mockResolvedValue({ issues: [], total: 0 });

    await useRecentsStore.getState().refresh();
    expect(listRecentIssuesMock).not.toHaveBeenCalled();
  });

  it("removes an issue on remove", async () => {
    listRecentIssuesMock.mockResolvedValue({
      issues: [makeIssue({ id: "a", workspace_id: "ws-1", updated_at: "2026-01-01T00:00:00Z" })],
      total: 1,
    });
    await useRecentsStore.getState().fetch(["ws-1"]);

    useRecentsStore.getState().removeIssue("a");
    expect(useRecentsStore.getState().issues).toEqual([]);
  });
});
