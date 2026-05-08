import { describe, it, expect } from "vitest";
import type { AgentTask } from "@/shared/types/agent";
import { groupTaskRunsByTrigger, isActiveTask } from "./agent-live-card";

function makeTask(overrides: Partial<AgentTask>): AgentTask {
  return {
    id: "task-x",
    agent_id: "agent-1",
    runtime_id: "rt-1",
    issue_id: "issue-1",
    status: "completed",
    priority: 0,
    dispatched_at: null,
    started_at: null,
    completed_at: null,
    result: null,
    error: null,
    created_at: "2026-05-01T00:00:00Z",
    trigger_comment_id: null,
    ...overrides,
  };
}

describe("groupTaskRunsByTrigger", () => {
  it("places null-trigger runs in the issue bucket", () => {
    const tasks = [
      makeTask({ id: "t1", trigger_comment_id: null }),
      makeTask({ id: "t2", trigger_comment_id: undefined }),
    ];
    const { issueTriggered, byCommentId } = groupTaskRunsByTrigger(tasks);
    expect(issueTriggered.map((t) => t.id)).toEqual(["t1", "t2"]);
    expect(byCommentId.size).toBe(0);
  });

  it("buckets comment-triggered runs by trigger_comment_id", () => {
    const tasks = [
      makeTask({ id: "t1", trigger_comment_id: null, created_at: "2026-05-01T00:00:00Z" }),
      makeTask({ id: "t2", trigger_comment_id: "c-1", created_at: "2026-05-02T00:00:00Z" }),
      makeTask({ id: "t3", trigger_comment_id: "c-1", created_at: "2026-05-03T00:00:00Z" }),
      makeTask({ id: "t4", trigger_comment_id: "c-2", created_at: "2026-05-04T00:00:00Z" }),
    ];
    const { issueTriggered, byCommentId } = groupTaskRunsByTrigger(tasks);
    expect(issueTriggered.map((t) => t.id)).toEqual(["t1"]);
    expect(byCommentId.get("c-1")?.map((t) => t.id)).toEqual(["t2", "t3"]);
    expect(byCommentId.get("c-2")?.map((t) => t.id)).toEqual(["t4"]);
  });

  it("sorts each bucket chronologically (oldest first)", () => {
    const tasks = [
      makeTask({ id: "later", created_at: "2026-05-05T00:00:00Z" }),
      makeTask({ id: "earlier", created_at: "2026-05-01T00:00:00Z" }),
    ];
    const { issueTriggered } = groupTaskRunsByTrigger(tasks);
    expect(issueTriggered.map((t) => t.id)).toEqual(["earlier", "later"]);
  });
});

describe("isActiveTask", () => {
  it("returns true for queued/dispatched/running", () => {
    expect(isActiveTask(makeTask({ status: "queued" }))).toBe(true);
    expect(isActiveTask(makeTask({ status: "dispatched" }))).toBe(true);
    expect(isActiveTask(makeTask({ status: "running" }))).toBe(true);
  });

  it("returns false for terminal states", () => {
    expect(isActiveTask(makeTask({ status: "completed" }))).toBe(false);
    expect(isActiveTask(makeTask({ status: "failed" }))).toBe(false);
    expect(isActiveTask(makeTask({ status: "cancelled" }))).toBe(false);
  });
});
