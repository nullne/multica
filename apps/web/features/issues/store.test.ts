import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Issue } from "@/shared/types";

let currentWorkspaceId: string | null = null;
const mockListIssues = vi.fn();

vi.mock("@/shared/api", () => ({
  api: {
    getWorkspaceId: () => currentWorkspaceId,
    listIssues: (...args: unknown[]) => mockListIssues(...args),
  },
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn() },
}));

import { useIssueStore } from "./store";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function makeIssue(id: string, workspaceId: string): Issue {
  return {
    id,
    workspace_id: workspaceId,
    number: 1,
    identifier: "MUL-1",
    title: id,
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
    github_auto_fix_enabled: false,
    labels: [],
    created_at: "2026-06-01T00:00:00Z",
    updated_at: "2026-06-01T00:00:00Z",
  };
}

describe("useIssueStore workspace freshness", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentWorkspaceId = null;
    useIssueStore.setState({ issues: [], loading: true, activeIssueId: null });
  });

  it("does not let a stale workspace fetch overwrite current issues", async () => {
    const oldFetch = deferred<{ issues: Issue[] }>();
    currentWorkspaceId = "ws-old";
    mockListIssues.mockReturnValueOnce(oldFetch.promise);

    const oldPromise = useIssueStore.getState().fetch();
    await Promise.resolve();

    currentWorkspaceId = "ws-new";
    mockListIssues.mockResolvedValueOnce({
      issues: [makeIssue("new-issue", "ws-new")],
    });
    await useIssueStore.getState().fetch();

    oldFetch.resolve({ issues: [makeIssue("old-issue", "ws-old")] });
    await oldPromise;

    expect(useIssueStore.getState().issues.map((issue) => issue.id)).toEqual([
      "new-issue",
    ]);
  });
});
