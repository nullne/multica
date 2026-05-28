import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { createStore } from "zustand/vanilla";
import type { Issue } from "@/shared/types";
import type { AgentTask } from "@/shared/types/agent";

import { useActiveTaskStore } from "@/features/issues/stores/active-task-store";
import { useWorkspaceStore } from "@/features/workspace/store";
import { ViewStoreProvider } from "@/features/issues/stores/view-store-context";
import {
  viewStoreSlice,
  type IssueViewState,
  type CardProperties,
} from "@/features/issues/stores/view-store";
import { BoardCardContent } from "./board-card";

function makeIssue(overrides: Partial<Issue> = {}): Issue {
  return {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "MUL-1",
    title: "Test issue",
    description: null,
    status: "in_progress",
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
    ...overrides,
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

function renderWithViewStore(
  issue: Issue,
  cardProperties: Partial<CardProperties> = {},
) {
  const store = createStore<IssueViewState>()((set) => ({
    ...viewStoreSlice(set),
    cardProperties: {
      priority: true,
      description: true,
      assignee: true,
      dueDate: true,
      labels: true,
      ...cardProperties,
    },
  }));
  return render(
    <ViewStoreProvider store={store}>
      <BoardCardContent issue={issue} />
    </ViewStoreProvider>,
  );
}

describe("BoardCardContent", () => {
  beforeEach(() => {
    useActiveTaskStore.setState({ tasks: new Map() });
    useWorkspaceStore.setState({ workspace: null, members: [], agents: [] });
  });

  it("shows the running ring even when assignee is hidden", () => {
    useActiveTaskStore.setState({
      tasks: new Map([["issue-1", makeTask("issue-1")]]),
    });
    renderWithViewStore(makeIssue(), { assignee: false });
    expect(screen.getByTitle("Agent is working")).toBeInTheDocument();
  });

  it("shows the running ring even when every optional card property is hidden", () => {
    useActiveTaskStore.setState({
      tasks: new Map([["issue-1", makeTask("issue-1")]]),
    });
    renderWithViewStore(makeIssue(), {
      priority: false,
      description: false,
      assignee: false,
      dueDate: false,
      labels: false,
    });
    expect(screen.getByTitle("Agent is working")).toBeInTheDocument();
  });

  it("does not render the ring when no agent task is active", () => {
    renderWithViewStore(makeIssue());
    expect(screen.queryByTitle("Agent is working")).not.toBeInTheDocument();
  });
});
