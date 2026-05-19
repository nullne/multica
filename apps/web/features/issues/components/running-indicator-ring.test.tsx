import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type { AgentTask } from "@/shared/types/agent";
import { useActiveTaskStore } from "@/features/issues/stores/active-task-store";
import { RunningIndicatorRing } from "./running-indicator-ring";

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

describe("RunningIndicatorRing", () => {
  beforeEach(() => {
    useActiveTaskStore.setState({ tasks: new Map() });
  });

  it("renders only the child icon when no agent task is active", () => {
    render(
      <RunningIndicatorRing issueId="issue-1">
        <span data-testid="icon">P</span>
      </RunningIndicatorRing>,
    );

    expect(screen.getByTestId("icon")).toBeInTheDocument();
    expect(screen.queryByTitle("Agent is working")).not.toBeInTheDocument();
  });

  it("renders a spinning ring when the issue has an active agent task", () => {
    useActiveTaskStore.setState({
      tasks: new Map([["issue-1", makeTask("issue-1")]]),
    });

    render(
      <RunningIndicatorRing issueId="issue-1">
        <span data-testid="icon">P</span>
      </RunningIndicatorRing>,
    );

    expect(screen.getByTestId("icon")).toBeInTheDocument();
    const ring = screen.getByTitle("Agent is working");
    expect(ring).toBeInTheDocument();
    expect(ring.className).toContain("animate-spin");
  });

  it("ignores tasks on other issues", () => {
    useActiveTaskStore.setState({
      tasks: new Map([["other-issue", makeTask("other-issue")]]),
    });

    render(
      <RunningIndicatorRing issueId="issue-1">
        <span data-testid="icon">P</span>
      </RunningIndicatorRing>,
    );

    expect(screen.queryByTitle("Agent is working")).not.toBeInTheDocument();
  });
});
