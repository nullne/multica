import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

const mocks = vi.hoisted(() => ({
  api: {
    getRoutine: vi.fn(),
    listRoutineRuns: vi.fn(),
  },
  toast: {
    error: vi.fn(),
  },
}));

vi.mock("@/shared/api", () => ({
  api: mocks.api,
}));

vi.mock("sonner", () => ({
  toast: mocks.toast,
}));

import { RoutineViewPage } from "../routine-view-page";

describe("RoutineDetailPage", () => {
  it("renders a routine overview and triggered issues", async () => {
    mocks.api.getRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Daily code review",
      priority: "medium",
      assignee_id: null,
      assignee_type: null,
      triggers: [{ id: "trigger-1" }, { id: "trigger-2" }],
      actions: [],
      subscriber_ids: ["user-1"],
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

    expect(await screen.findByRole("heading", { name: "Daily code review" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Edit routine/i })).toHaveAttribute("href", "/routines?new=1&id=routine-1");
    expect(screen.getByText("Triggered issues")).toBeInTheDocument();
    expect(screen.getByText("Triggered issue")).toBeInTheDocument();
    expect(screen.getByText(/TES-1/)).toBeInTheDocument();
  });
});
