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
    id: overrides.id,
    workspace_id: overrides.workspace_id,
    number: 0,
    identifier: overrides.id,
    title: overrides.title ?? "Untitled",
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
    created_at: overrides.updated_at,
    updated_at: overrides.updated_at,
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
