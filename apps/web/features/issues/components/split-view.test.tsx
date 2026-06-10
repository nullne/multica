import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Issue } from "@/shared/types";

let mockSearchParams = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useSearchParams: () => mockSearchParams,
}));

vi.mock("react-resizable-panels", async (importOriginal) => {
  const actual: object = await importOriginal();
  return {
    ...actual,
    useDefaultLayout: () => ({ defaultLayout: undefined, onLayoutChanged: () => {} }),
  };
});

vi.mock("./issue-detail", () => ({
  IssueDetail: ({ issueId }: { issueId: string }) => (
    <div data-testid="issue-detail">{issueId}</div>
  ),
}));

import { useActiveTaskStore } from "@/features/issues/stores/active-task-store";
import type { AgentTask } from "@/shared/types/agent";
import { SplitView } from "./split-view";

function makeIssue(id: string, identifier: string, title: string): Issue {
  return {
    id,
    workspace_id: "ws-1",
    number: 1,
    identifier,
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

describe("SplitView", () => {
  beforeEach(() => {
    mockSearchParams = new URLSearchParams();
    useActiveTaskStore.setState({ tasks: new Map() });
  });

  it("marks the row matching the URL issue param as active", () => {
    mockSearchParams = new URLSearchParams("issue=issue-2");
    const issues = [
      makeIssue("issue-1", "MUL-1", "First"),
      makeIssue("issue-2", "MUL-2", "Second"),
    ];

    render(<SplitView issues={issues} />);

    const activeRow = screen.getByRole("button", { name: /Second/ });
    expect(activeRow).toHaveAttribute("aria-current", "true");
    expect(activeRow).toHaveAttribute("data-active");

    const inactiveRow = screen.getByRole("button", { name: /First/ });
    expect(inactiveRow).not.toHaveAttribute("aria-current");
    expect(inactiveRow).not.toHaveAttribute("data-active");
  });

  it("switches the active row when a different issue is clicked", async () => {
    const issues = [
      makeIssue("issue-1", "MUL-1", "First"),
      makeIssue("issue-2", "MUL-2", "Second"),
    ];
    const user = userEvent.setup();
    render(<SplitView issues={issues} />);

    await user.click(screen.getByRole("button", { name: /Second/ }));

    expect(screen.getByRole("button", { name: /Second/ })).toHaveAttribute(
      "aria-current",
      "true",
    );
    expect(screen.getByRole("button", { name: /First/ })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("renders a running ring for issues with an active agent task", () => {
    useActiveTaskStore.setState({
      tasks: new Map([["issue-1", makeTask("issue-1")]]),
    });
    const issues = [
      makeIssue("issue-1", "MUL-1", "First"),
      makeIssue("issue-2", "MUL-2", "Second"),
    ];

    render(<SplitView issues={issues} />);

    const rings = screen.getAllByTitle("Agent is working");
    expect(rings).toHaveLength(1);
    expect(
      screen.getByRole("button", { name: /First/ }).contains(rings[0]!),
    ).toBe(true);
  });
});
