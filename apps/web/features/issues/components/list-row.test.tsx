import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type { Issue } from "@/shared/types";
import type { AgentTask } from "@/shared/types/agent";

import { useIssueStore } from "@/features/issues/store";
import { useIssueSelectionStore } from "@/features/issues/stores/selection-store";
import { useActiveTaskStore } from "@/features/issues/stores/active-task-store";
import { useWorkspaceStore } from "@/features/workspace/store";
import { ListRow } from "./list-row";

function makeIssue(id: string): Issue {
  return {
    id,
    workspace_id: "ws-1",
    number: 1,
    identifier: "MUL-1",
    title: `Issue ${id}`,
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
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
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
    created_at: "2026-05-01T00:00:00Z",
    trigger_comment_id: null,
  };
}

describe("ListRow", () => {
  beforeEach(() => {
    useIssueStore.setState({ issues: [], loading: false, activeIssueId: null });
    useIssueSelectionStore.setState({ selectedIds: new Set() });
    useActiveTaskStore.setState({ tasks: new Map() });
    useWorkspaceStore.setState({
      workspace: {
        id: "ws-1",
        name: "WS",
        slug: "ws-1",
        description: null,
        context: null,
        settings: {},
        repos: [],
        issue_prefix: "MUL",
        github_connected: false,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      },
    });
  });

  it("does not mark the row as active when no issue is opened", () => {
    const { container } = render(<ListRow issue={makeIssue("issue-1")} />);
    const row = container.firstElementChild!;
    expect(row.getAttribute("aria-current")).toBeNull();
    expect(row.getAttribute("data-active")).toBeNull();
  });

  it("marks the row as active when the store says this issue is the opened one", () => {
    useIssueStore.setState({ activeIssueId: "issue-1" });
    const { container } = render(<ListRow issue={makeIssue("issue-1")} />);
    const row = container.firstElementChild!;
    expect(row.getAttribute("aria-current")).toBe("true");
    expect(row.getAttribute("data-active")).toBe("true");
  });

  it("does not mark unrelated rows as active", () => {
    useIssueStore.setState({ activeIssueId: "issue-other" });
    const { container } = render(<ListRow issue={makeIssue("issue-1")} />);
    const row = container.firstElementChild!;
    expect(row.getAttribute("aria-current")).toBeNull();
  });

  it("renders the running ring when the issue has an active agent task", () => {
    useActiveTaskStore.setState({
      tasks: new Map([["issue-1", makeTask("issue-1")]]),
    });
    render(<ListRow issue={makeIssue("issue-1")} />);
    expect(screen.getByTitle("Agent is working")).toBeInTheDocument();
  });
});
